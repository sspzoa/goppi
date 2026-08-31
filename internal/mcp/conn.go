package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sspzoa/goppi/internal/rpcio"
)

type rpcReq struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcErr         `json:"error"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type Conn struct {
	in   *bufio.Reader
	out  io.Writer
	mu   sync.Mutex
	id   atomic.Int64
	wait map[int64]chan rpcResp
	err  error
	done chan struct{}
}

func NewConn(r io.Reader, w io.Writer) *Conn {
	c := &Conn{
		in:   bufio.NewReader(r),
		out:  w,
		wait: map[int64]chan rpcResp{},
		done: make(chan struct{}),
	}
	go c.readLoop()
	return c
}

func (c *Conn) Initialize(ctx context.Context, clientName, version string) error {
	_, err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": clientName, "version": version},
	})
	if err != nil {
		return err
	}
	return c.notify("notifications/initialized", map[string]any{})
}

func (c *Conn) ListTools(ctx context.Context) ([]Tool, error) {
	raw, err := c.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var out struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Tools, nil
}

func (c *Conn) Call(ctx context.Context, name string, args json.RawMessage) (string, error) {
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	raw, err := c.call(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": json.RawMessage(args),
	})
	if err != nil {
		return "", err
	}
	var out struct {
		IsError bool `json:"isError"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return string(raw), nil
	}
	var b strings.Builder
	for _, part := range out.Content {
		if part.Text != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(part.Text)
		}
	}
	text := b.String()
	if text == "" {
		text = strings.TrimSpace(string(raw))
	}
	if out.IsError {
		return "", fmt.Errorf("mcp: %s", text)
	}
	return text, nil
}

func (c *Conn) Close() {
	c.mu.Lock()
	if c.err == nil {
		c.err = io.EOF
	}
	c.mu.Unlock()
}

func (c *Conn) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.id.Add(1)
	ch := make(chan rpcResp, 1)
	c.mu.Lock()
	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		return nil, err
	}
	c.wait[id] = ch
	c.mu.Unlock()
	if err := c.write(rpcReq{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.wait, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-c.done:
		c.mu.Lock()
		err := c.err
		c.mu.Unlock()
		if err == nil {
			err = io.EOF
		}
		return nil, err
	case resp, ok := <-ch:
		if !ok {
			c.mu.Lock()
			err := c.err
			c.mu.Unlock()
			if err == nil {
				err = io.EOF
			}
			return nil, err
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("mcp %s: %s", method, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (c *Conn) notify(method string, params any) error {
	return c.write(rpcReq{JSONRPC: "2.0", Method: method, Params: params})
}

func (c *Conn) write(v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return writeFrame(c.out, raw)
}

func writeFrame(w io.Writer, raw []byte) error {
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(raw)); err != nil {
		return err
	}
	_, err := w.Write(raw)
	return err
}

func (c *Conn) readLoop() {
	defer close(c.done)
	for {
		raw, err := readFrame(c.in)
		if err != nil {
			c.mu.Lock()
			c.err = err
			for _, ch := range c.wait {
				close(ch)
			}
			c.wait = map[int64]chan rpcResp{}
			c.mu.Unlock()
			return
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var probe struct {
			Method string `json:"method"`
			ID     int64  `json:"id"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			continue
		}
		if probe.Method != "" && probe.ID == 0 {
			continue // notification from server
		}
		var resp rpcResp
		if err := json.Unmarshal(raw, &resp); err != nil {
			continue
		}
		c.mu.Lock()
		ch, ok := c.wait[resp.ID]
		if ok {
			delete(c.wait, resp.ID)
		}
		c.mu.Unlock()
		if ok {
			ch <- resp
		}
	}
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	for {
		lead, err := r.Peek(1)
		if err != nil {
			return nil, err
		}
		if lead[0] == '\n' || lead[0] == '\r' {
			if _, err := r.ReadByte(); err != nil {
				return nil, err
			}
			continue
		}
		if lead[0] == '{' {
			return rpcio.ReadLine(r, rpcio.MaxChild)
		}
		break
	}
	var n int
	for {
		raw, err := rpcio.ReadLine(r, rpcio.MaxHeader)
		if err != nil {
			return nil, err
		}
		line := strings.TrimSpace(string(raw))
		if line == "" {
			if n <= 0 {
				return nil, fmt.Errorf("mcp: missing Content-Length")
			}
			buf := make([]byte, n)
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, err
			}
			return buf, nil
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(key), "Content-Length") {
			n, err = strconv.Atoi(strings.TrimSpace(val))
			if err != nil || n < 0 || n > 4<<20 {
				return nil, fmt.Errorf("mcp: bad Content-Length")
			}
		}
	}
}
