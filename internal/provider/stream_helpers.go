package provider

import (
	"bufio"
	"context"
	"io"
)

// NewSSEScanner returns a scanner configured for SSE payload sizes that are
// commonly larger than bufio's default token limit.
func NewSSEScanner(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return s
}

// SendStreamChunk delivers a stream chunk unless the context is canceled.
func SendStreamChunk(ctx context.Context, chunks chan<- StreamChunk, sc StreamChunk) bool {
	select {
	case <-ctx.Done():
		return false
	case chunks <- sc:
		return true
	}
}
