package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// maxFrameBytes is the scanner's max token size. ACP tool_result/diff frames can
// be large (full file patches); the default 64 KiB bufio cap would truncate them
// (techspec §2 — "buffer enlargement is load-bearing").
const maxFrameBytes = 8 * 1024 * 1024 // 8 MiB

// errTransportClosed is returned to pending Calls when the read loop ends.
var errTransportClosed = errors.New("runtime: transport closed")

// IncomingRequest is a peer→us JSON-RPC request (e.g. session/request_permission)
// that expects a reply. The handler may hold it and Respond later — withholding
// the response is exactly how permission gating pauses the agent (techspec §5.1).
type IncomingRequest struct {
	ID     int64
	Method string
	Params json.RawMessage

	t        *Transport
	mu       sync.Mutex
	answered bool
}

// Respond sends a success result for this request. Safe to call once; a second
// call is a no-op error.
func (r *IncomingRequest) Respond(result any) error {
	return r.reply(result, nil)
}

// RespondError sends a JSON-RPC error for this request.
func (r *IncomingRequest) RespondError(code int, message string) error {
	return r.reply(nil, &rpcError{Code: code, Message: message})
}

func (r *IncomingRequest) reply(result any, rerr *rpcError) error {
	r.mu.Lock()
	if r.answered {
		r.mu.Unlock()
		return fmt.Errorf("runtime: request %d already answered", r.ID)
	}
	r.answered = true
	r.mu.Unlock()

	msg := rpcMessage{JSONRPC: jsonrpcVersion, ID: &r.ID}
	if rerr != nil {
		msg.Error = rerr
	} else {
		raw, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("runtime: marshal response: %w", err)
		}
		msg.Result = raw
	}
	return r.t.writeMessage(msg)
}

// Transport is one JSON-RPC-over-NDJSON stdio channel to a child process. It
// owns a serialized writer to the child's stdin and a read loop over its stdout.
// Outbound writes are mutex-guarded so concurrent Calls/Notifies/Responds never
// interleave a half-written frame (techspec §2, §8.1).
type Transport struct {
	w   io.Writer
	wmu sync.Mutex // serializes all writes to w

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan rpcResult
	closed  bool

	onNotification func(method string, params json.RawMessage)
	onRequest      func(req *IncomingRequest)

	done chan struct{} // closed when the read loop exits
	err  error         // read-loop terminal error (EOF or scanner error)
}

type rpcResult struct {
	result json.RawMessage
	// err is `error` (not `*rpcError`) so shutdown can deliver the
	// errTransportClosed sentinel itself — the read side uses errors.Is to
	// distinguish a crash/stop from a genuine RPC error.
	err error
}

// NewTransport builds a transport writing to w. Handlers may be nil (frames of
// that kind are then logged and dropped). Call Run with the child's stdout to
// start the read loop.
func NewTransport(w io.Writer, onNotification func(string, json.RawMessage), onRequest func(*IncomingRequest)) *Transport {
	return &Transport{
		w:              w,
		pending:        map[int64]chan rpcResult{},
		onNotification: onNotification,
		onRequest:      onRequest,
		done:           make(chan struct{}),
	}
}

// Run reads NDJSON frames from r until EOF or a fatal scanner error, dispatching
// each. It blocks; run it in a goroutine. On exit it fails all pending Calls.
// The returned error is the terminal read error (nil on clean EOF).
func (t *Transport) Run(r io.Reader) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxFrameBytes)

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			// Malformed frame: log truncated raw, resync on the next line
			// boundary — do not kill the session for one bad frame (§8.3).
			slog.Warn("runtime: skip malformed frame", "err", err, "raw", truncate(line, 256))
			continue
		}
		t.dispatch(&msg)
	}

	err := sc.Err()
	t.shutdown(err)
	return err
}

// dispatch routes one decoded frame by kind.
func (t *Transport) dispatch(msg *rpcMessage) {
	switch msg.kind() {
	case frameNotification:
		if t.onNotification != nil {
			t.onNotification(msg.Method, msg.Params)
		}
	case frameRequest:
		if t.onRequest != nil {
			t.onRequest(&IncomingRequest{ID: *msg.ID, Method: msg.Method, Params: msg.Params, t: t})
		} else {
			slog.Warn("runtime: no handler for incoming request", "method", msg.Method, "id", *msg.ID)
		}
	case frameResponse:
		// Guard the typed-nil trap: a nil *rpcError stored in an `error` field
		// would read as non-nil. Only set err on a genuine RPC error object.
		res := rpcResult{result: msg.Result}
		if msg.Error != nil {
			res.err = msg.Error
		}
		t.deliver(*msg.ID, res)
	default:
		slog.Warn("runtime: unknown frame shape", "raw", truncate(rawOf(msg), 256))
	}
}

// deliver routes a response to the waiting Call, if any.
func (t *Transport) deliver(id int64, res rpcResult) {
	t.mu.Lock()
	ch, ok := t.pending[id]
	if ok {
		delete(t.pending, id)
	}
	t.mu.Unlock()
	if !ok {
		slog.Warn("runtime: response for unknown request id", "id", id)
		return
	}
	ch <- res
}

// shutdown closes done once and fails every pending Call.
func (t *Transport) shutdown(err error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	t.err = err
	pending := t.pending
	t.pending = map[int64]chan rpcResult{}
	t.mu.Unlock()

	for _, ch := range pending {
		ch <- rpcResult{err: errTransportClosed}
	}
	close(t.done)
}

// Call sends a request and blocks until the response, ctx cancellation, or the
// transport closing. Returns the raw result or the peer/transport error.
func (t *Transport) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	raw, err := marshalParams(params)
	if err != nil {
		return nil, err
	}

	ch := make(chan rpcResult, 1)
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, errTransportClosed
	}
	t.nextID++
	id := t.nextID
	t.pending[id] = ch
	t.mu.Unlock()

	msg := rpcMessage{JSONRPC: jsonrpcVersion, ID: &id, Method: method, Params: raw}
	if err := t.writeMessage(msg); err != nil {
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
		return nil, err
	}

	select {
	case <-ctx.Done():
		t.mu.Lock()
		delete(t.pending, id)
		t.mu.Unlock()
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		return res.result, nil
	}
}

// Notify sends a notification (no id, no response expected).
func (t *Transport) Notify(method string, params any) error {
	raw, err := marshalParams(params)
	if err != nil {
		return err
	}
	return t.writeMessage(rpcMessage{JSONRPC: jsonrpcVersion, Method: method, Params: raw})
}

// writeMessage serializes one frame and writes it newline-terminated under the
// write mutex so frames never interleave on stdin.
func (t *Transport) writeMessage(msg rpcMessage) error {
	msg.JSONRPC = jsonrpcVersion
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("runtime: marshal frame: %w", err)
	}
	b = append(b, '\n')

	t.wmu.Lock()
	defer t.wmu.Unlock()
	if _, err := t.w.Write(b); err != nil {
		return fmt.Errorf("runtime: write frame: %w", err)
	}
	return nil
}

// Done returns a channel closed when the read loop exits.
func (t *Transport) Done() <-chan struct{} { return t.done }

// Err returns the read loop's terminal error after Done is closed (nil on EOF).
func (t *Transport) Err() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}

// marshalParams renders params to a RawMessage, treating nil as absent.
func marshalParams(params any) (json.RawMessage, error) {
	if params == nil {
		return nil, nil
	}
	if raw, ok := params.(json.RawMessage); ok {
		return raw, nil
	}
	b, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("runtime: marshal params: %w", err)
	}
	return b, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}

func rawOf(msg *rpcMessage) []byte {
	b, _ := json.Marshal(msg)
	return b
}
