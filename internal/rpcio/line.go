package rpcio

import (
	"bufio"
	"fmt"
	"io"
)

const (
	MaxFrame  = 8 << 20
	MaxChild  = 4 << 20
	MaxHeader = 8 << 10
)

func ReadLine(r *bufio.Reader, max int) ([]byte, error) {
	if r == nil {
		return nil, io.EOF
	}
	if max <= 0 {
		max = MaxFrame
	}
	var out []byte
	for {
		part, err := r.ReadSlice('\n')
		if len(out)+len(part) > max {
			return nil, fmt.Errorf("frame too large")
		}
		out = append(out, part...)
		switch {
		case err == nil:
			return out, nil
		case err == bufio.ErrBufferFull:
			continue
		case err == io.EOF:
			if len(out) == 0 {
				return nil, io.EOF
			}
			return out, io.EOF
		default:
			return nil, err
		}
	}
}
