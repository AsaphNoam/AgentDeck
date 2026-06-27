package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// feedTransport runs a transport whose read side is the given NDJSON text and
// whose write side is discarded. Returns the transport and a func to wait for
// the read loop to finish.
func feedTransport(t *testing.T, ndjson string, onNote func(string, json.RawMessage), onReq func(*IncomingRequest)) (*Transport, func()) {
	t.Helper()
	tr := NewTransport(io.Discard, onNote, onReq)
	done := make(chan struct{})
	go func() {
		_ = tr.Run(strings.NewReader(ndjson))
		close(done)
	}()
	return tr, func() {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("transport read loop did not finish")
		}
	}
}

// TestTransportBigFrame feeds a single notification whose params exceed 64 KiB,
// locking in the 8 MiB scanner buffer (techspec §2). A default bufio cap would
// drop this frame with bufio.ErrTooLong.
func TestTransportBigFrame(t *testing.T) {
	big := strings.Repeat("x", 100*1024) // 100 KiB > default 64 KiB cap
	frame := mustFrame(t, rpcMessage{
		JSONRPC: jsonrpcVersion,
		Method:  "session/update",
		Params:  json.RawMessage(`{"text":"` + big + `"}`),
	})

	var got json.RawMessage
	var mu sync.Mutex
	_, wait := feedTransport(t, frame+"\n", func(method string, params json.RawMessage) {
		mu.Lock()
		got = params
		mu.Unlock()
	}, nil)
	wait()

	mu.Lock()
	defer mu.Unlock()
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("unmarshal big params: %v", err)
	}
	if len(payload.Text) != len(big) {
		t.Fatalf("big frame truncated: got %d bytes, want %d", len(payload.Text), len(big))
	}
}

// TestTransportMalformedThenValid feeds a bad line followed by a valid
// notification, asserting resync: the bad line is skipped and the good frame
// still dispatches (techspec §8.3).
func TestTransportMalformedThenValid(t *testing.T) {
	good := mustFrame(t, rpcMessage{
		JSONRPC: jsonrpcVersion,
		Method:  "session/update",
		Params:  json.RawMessage(`{"ok":true}`),
	})
	ndjson := "{this is not valid json}\n" + good + "\n"

	var methods []string
	var mu sync.Mutex
	_, wait := feedTransport(t, ndjson, func(method string, _ json.RawMessage) {
		mu.Lock()
		methods = append(methods, method)
		mu.Unlock()
	}, nil)
	wait()

	mu.Lock()
	defer mu.Unlock()
	if len(methods) != 1 || methods[0] != "session/update" {
		t.Fatalf("resync failed: got notifications %v, want [session/update]", methods)
	}
}

// TestTransportCallResponse exercises request/response correlation: a Call
// blocks until a matching-id response arrives.
func TestTransportCallResponse(t *testing.T) {
	// Use an io.Pipe so the test can hand-write the response after seeing the
	// request id on the write side.
	reqR, reqW := io.Pipe()   // we read what the transport writes
	respR, respW := io.Pipe() // we write responses the transport reads

	tr := NewTransport(reqW, nil, nil)
	go tr.Run(respR)

	// Reader goroutine: parse the outgoing request, reply with its id.
	go func() {
		sc := bufio.NewScanner(reqR)
		sc.Buffer(make([]byte, 0, 64*1024), maxFrameBytes)
		for sc.Scan() {
			var m rpcMessage
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil || m.ID == nil {
				continue
			}
			resp := mustFrameBytes(rpcMessage{
				JSONRPC: jsonrpcVersion, ID: m.ID,
				Result: json.RawMessage(`{"sessionId":"fake-sess-1"}`),
			})
			respW.Write(append(resp, '\n'))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := tr.Call(ctx, "session/new", map[string]any{"cwd": "/tmp"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var out struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out.SessionID != "fake-sess-1" {
		t.Fatalf("sessionId = %q, want fake-sess-1", out.SessionID)
	}
}

// TestTransportIncomingRequestRespond asserts a peer→us request reaches the
// handler and Respond writes a reply (the withhold-then-respond mechanism that
// powers permission gating in 1.4).
func TestTransportIncomingRequestRespond(t *testing.T) {
	var out strings.Builder
	tr := NewTransport(&lockedWriter{w: &out}, nil, nil)

	var captured *IncomingRequest
	var mu sync.Mutex
	tr.onRequest = func(req *IncomingRequest) {
		mu.Lock()
		captured = req
		mu.Unlock()
	}

	reqFrame := mustFrame(t, rpcMessage{
		JSONRPC: jsonrpcVersion, ID: int64Ptr(7),
		Method: "session/request_permission",
		Params: json.RawMessage(`{"toolCall":{"toolCallId":"tc_1"}}`),
	})
	_, wait := feedTransportWith(t, tr, reqFrame+"\n")
	wait()

	mu.Lock()
	req := captured
	mu.Unlock()
	if req == nil {
		t.Fatal("incoming request not delivered to handler")
	}
	if req.Method != "session/request_permission" || req.ID != 7 {
		t.Fatalf("unexpected request: id=%d method=%q", req.ID, req.Method)
	}
	if err := req.Respond(map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "o1"}}); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	// A second response is rejected.
	if err := req.Respond(map[string]any{}); err == nil {
		t.Fatal("second Respond should error")
	}
	if !strings.Contains(out.String(), `"optionId":"o1"`) {
		t.Fatalf("response not written: %q", out.String())
	}
}

// --- helpers ---

func mustFrame(t *testing.T, m rpcMessage) string {
	t.Helper()
	return string(mustFrameBytes(m))
}

func mustFrameBytes(m rpcMessage) []byte {
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return b
}

func int64Ptr(v int64) *int64 { return &v }

// feedTransportWith runs an already-constructed transport against ndjson.
func feedTransportWith(t *testing.T, tr *Transport, ndjson string) (*Transport, func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		_ = tr.Run(strings.NewReader(ndjson))
		close(done)
	}()
	return tr, func() {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("transport read loop did not finish")
		}
	}
}

// lockedWriter serializes concurrent writes for tests that read the output.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
