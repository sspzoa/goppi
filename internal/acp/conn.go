package acp

import (
	"bufio"
	"bytes"
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
	in   *bufio.Reader
	out  io.Writer
	mu   sync.Mutex
	id   atomic.Int64
	wait map[int64]chan incoming
}

func newConn(r io.Reader, w io.Writer) *Conn {
	return &Conn{in: bufio.NewReader(r), out: w, wait: map[int64]chan incoming{}}
}

func (c *Conn) read() (incoming, error) {
	raw, err := readFrame(c.in)
	if err != nil {
		return incoming{}, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return incoming{}, nil
	}
	var msg incoming
	if err := json.Unmarshal(raw, &msg); err != nil {
		return incoming{}, err
	}
	if msg.Method == "" {
		id, ok := parseID(msg.ID)
		if ok {
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
	return msg, nil
}

func (c *Conn) reply(id json.RawMessage, result any) error {
	return c.write(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": result})
}

func (c *Conn) replyErr(id json.RawMessage, code int, message string) error {
	return c.write(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"error":   map[string]any{"code": code, "message": message},
	})
}

func (c *Conn) notify(method string, params any) error {
	return c.write(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

func (c *Conn) call(method string, params any) (json.RawMessage, error) {
	id := c.id.Add(1)
	ch := make(chan incoming, 1)
	c.mu.Lock()
	c.wait[id] = ch
	c.mu.Unlock()
	if err := c.write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	resp := <-ch
	if resp.Error != nil {
		return nil, fmt.Errorf("acp %s: %s", method, resp.Error.Message)
	}
	return resp.Result, nil
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
			return rpcio.ReadLine(r, rpcio.MaxFrame)
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
				return nil, fmt.Errorf("acp: missing Content-Length")
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
			if err != nil || n < 0 || n > 8<<20 {
				return nil, fmt.Errorf("acp: bad Content-Length")
			}
		}
	}
}
