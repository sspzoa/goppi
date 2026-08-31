package lsp

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

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type incoming struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcErr         `json:"error"`
}

type Conn struct {
	in       *bufio.Reader
	out      io.Writer
	mu       sync.Mutex
	id       atomic.Int64
	wait     map[int64]chan incoming
	err      error
	done     chan struct{}
	onNotify func(method string, params json.RawMessage)
	reply    func(method string) any
}

func newConn(r io.Reader, w io.Writer) *Conn {
	c := &Conn{
		in:   bufio.NewReader(r),
		out:  w,
		wait: map[int64]chan incoming{},
		done: make(chan struct{}),
	}
	go c.readLoop()
	return c
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
	ch := make(chan incoming, 1)
	c.mu.Lock()
	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		return nil, err
	}
	c.wait[id] = ch
	c.mu.Unlock()
	if err := c.write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.wait, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-c.done:
		return nil, c.closedErr()
	case resp, ok := <-ch:
		if !ok {
			return nil, c.closedErr()
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("lsp %s: %s", method, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

func (c *Conn) notify(method string, params any) error {
	return c.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (c *Conn) closedErr() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err == nil {
		return io.EOF
	}
	return c.err
}

func (c *Conn) write(v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := fmt.Fprintf(c.out, "Content-Length: %d\r\n\r\n", len(raw)); err != nil {
		return err
	}
	_, err = c.out.Write(raw)
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
			c.wait = map[int64]chan incoming{}
			c.mu.Unlock()
			return
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		var msg incoming
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Method != "" && hasJSONID(msg.ID) {
			result := any(nil)
			if c.reply != nil {
				result = c.reply(msg.Method)
			}
			_ = c.write(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(msg.ID), "result": result})
			continue
		}
		if msg.Method != "" {
			if c.onNotify != nil {
				c.onNotify(msg.Method, msg.Params)
			}
			continue
		}
		id, ok := parseID(msg.ID)
		if !ok {
			continue
		}
		c.mu.Lock()
		ch, found := c.wait[id]
		if found {
			delete(c.wait, id)
		}
		c.mu.Unlock()
		if found {
			ch <- msg
		}
	}
}

func hasJSONID(id json.RawMessage) bool {
	s := strings.TrimSpace(string(id))
	return s != "" && s != "null"
}

func parseID(id json.RawMessage) (int64, bool) {
	var n int64
	if err := json.Unmarshal(id, &n); err != nil {
		return 0, false
	}
	return n, true
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
				return nil, fmt.Errorf("lsp: missing Content-Length")
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
				return nil, fmt.Errorf("lsp: bad Content-Length")
			}
		}
	}
}
