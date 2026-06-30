package terminal

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
)

// fakeWS is a scripted wsConn: read() returns queued frames then io.EOF;
// writeBinary records every outbound frame.
type fakeWS struct {
	reads []wsFrame
	ri    int

	mu     sync.Mutex
	writes [][]byte
	closed bool
}

type wsFrame struct {
	isText bool
	data   []byte
}

func (f *fakeWS) read(ctx context.Context) (bool, []byte, error) {
	if f.ri >= len(f.reads) {
		return false, nil, io.EOF
	}
	fr := f.reads[f.ri]
	f.ri++
	return fr.isText, fr.data, nil
}

func (f *fakeWS) writeBinary(ctx context.Context, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes = append(f.writes, append([]byte(nil), data...))
	return nil
}

func (f *fakeWS) close() error {
	f.mu.Lock()
	f.closed = true
	f.mu.Unlock()
	return nil
}

// fakePTY is a scripted PTYConn: Read drains queued chunks then io.EOF; Write
// accumulates keystrokes; Resize records the last requested size.
type fakePTY struct {
	mu       sync.Mutex
	written  bytes.Buffer
	readQ    [][]byte
	ri       int
	rzCalled bool
	rzRows   uint16
	rzCols   uint16
	closed   bool
}

func (p *fakePTY) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ri >= len(p.readQ) {
		return 0, io.EOF
	}
	chunk := p.readQ[p.ri]
	p.ri++
	return copy(b, chunk), nil
}

func (p *fakePTY) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.written.Write(b)
}

func (p *fakePTY) Close() error {
	p.mu.Lock()
	p.closed = true
	p.mu.Unlock()
	return nil
}

func (p *fakePTY) Resize(rows, cols uint16) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rzCalled, p.rzRows, p.rzCols = true, rows, cols
	return nil
}

// A binary client frame is written verbatim to the PTY master (keystrokes).
func TestPumpClientToPTYKeystroke(t *testing.T) {
	ws := &fakeWS{reads: []wsFrame{{isText: false, data: []byte("ls -la\r")}}}
	p := &fakePTY{}
	if err := pumpClientToPTY(context.Background(), ws, p); err != io.EOF {
		t.Fatalf("pump err = %v, want io.EOF", err)
	}
	if got := p.written.String(); got != "ls -la\r" {
		t.Fatalf("pty got %q, want %q", got, "ls -la\r")
	}
}

// A text client frame carrying {cols,rows} resizes the PTY, not written as input.
func TestPumpClientToPTYResize(t *testing.T) {
	ws := &fakeWS{reads: []wsFrame{{isText: true, data: []byte(`{"cols":120,"rows":40}`)}}}
	p := &fakePTY{}
	if err := pumpClientToPTY(context.Background(), ws, p); err != io.EOF {
		t.Fatalf("pump err = %v, want io.EOF", err)
	}
	if !p.rzCalled || p.rzRows != 40 || p.rzCols != 120 {
		t.Fatalf("resize = (called=%v rows=%d cols=%d), want (true 40 120)", p.rzCalled, p.rzRows, p.rzCols)
	}
	if p.written.Len() != 0 {
		t.Fatalf("resize frame must not be written to pty, got %q", p.written.String())
	}
}

// PTY output is streamed to the browser as binary frames.
func TestPumpPTYToClientOutput(t *testing.T) {
	p := &fakePTY{readQ: [][]byte{[]byte("hello "), []byte("world")}}
	ws := &fakeWS{}
	if err := pumpPTYToClient(context.Background(), ws, p); err != nil {
		t.Fatalf("pump err = %v, want nil on EOF", err)
	}
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if len(ws.writes) != 2 {
		t.Fatalf("frames = %d, want 2", len(ws.writes))
	}
	if string(ws.writes[0]) != "hello " || string(ws.writes[1]) != "world" {
		t.Fatalf("frames = %q, %q", ws.writes[0], ws.writes[1])
	}
}
