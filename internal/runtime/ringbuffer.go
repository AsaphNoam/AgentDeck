package runtime

import (
	"io"
	"sync"
)

// ringBuffer keeps the last N bytes written to it, discarding older bytes. It
// captures a child's stderr tail for diagnostics surfaced on crash (techspec
// §4.1 step 3, §8.2). Safe for concurrent Tail while copyFrom drains.
type ringBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func newRingBuffer(max int) *ringBuffer {
	return &ringBuffer{max: max}
}

// copyFrom drains r into the ring until EOF. Intended to run in a goroutine.
func (r *ringBuffer) copyFrom(rd io.Reader) {
	chunk := make([]byte, 4096)
	for {
		n, err := rd.Read(chunk)
		if n > 0 {
			r.write(chunk[:n])
		}
		if err != nil {
			return
		}
	}
}

func (r *ringBuffer) write(p []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = append(r.buf, p...)
	if len(r.buf) > r.max {
		r.buf = r.buf[len(r.buf)-r.max:]
	}
}

// Tail returns the most recent bytes as a string.
func (r *ringBuffer) Tail() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.buf)
}
