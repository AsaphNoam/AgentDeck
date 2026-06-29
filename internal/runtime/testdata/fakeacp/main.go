// Command fakeacp is a deterministic stand-in for the real claude-code-acp ACP
// adapter, used by ChatRuntime integration tests (techspec §10.2). It speaks
// JSON-RPC over NDJSON on stdin/stdout: it answers `initialize` and
// `session/new`, and on `session/prompt` it replays the scenario named by the
// FAKEACP_SCENARIO env var, emitting `session/update` notifications and then a
// prompt result.
//
// Permission scenarios send a `session/request_permission` request back to the
// client and block until the client replies (or cancels) — exactly the gating
// pause under test. The sentinel file (FAKEACP_SENTINEL) is created iff the tool
// is approved, giving tests a side effect that proves the tool "ran".
//
// It is intentionally standalone (no internal imports) so it builds and behaves
// like an external CLI.
package main

import (
	"bufio"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const sessionID = "fake-sess-1"

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var (
	out     = bufio.NewWriter(os.Stdout)
	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   = map[int64]chan rpcMessage{}
	reqSeq    atomic.Int64

	cancelOnce sync.Once
	cancelCh   = make(chan struct{})
)

func main() {
	reqSeq.Store(1000)
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		// A response to one of our (server→client) requests.
		if msg.ID != nil && msg.Method == "" {
			routeResponse(*msg.ID, msg)
			continue
		}
		handle(&msg)
	}
}

func handle(msg *rpcMessage) {
	if msg.ID == nil { // notification
		if msg.Method == "session/cancel" {
			cancelOnce.Do(func() { close(cancelCh) })
		}
		return
	}
	switch msg.Method {
	case "initialize":
		ver := 1
		if v := os.Getenv("FAKEACP_PROTO_VERSION"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				ver = n
			}
		}
		respond(*msg.ID, map[string]any{"protocolVersion": ver, "agentCapabilities": map[string]any{}})
	case "session/new":
		respond(*msg.ID, map[string]any{"sessionId": sessionID})
	case "session/load":
		// If asked, dump the raw load params so tests can assert that the
		// fresh MCP registration is carried on the load path (not just new).
		if dump := os.Getenv("FAKEACP_LOAD_DUMP"); dump != "" {
			_ = os.WriteFile(dump, msg.Params, 0o600)
		}
		// Simulate successful load by returning a distinct resumed session ID.
		respond(*msg.ID, map[string]any{"sessionId": "fake-sess-loaded"})
	case "session/prompt":
		id := *msg.ID
		// Run the scenario asynchronously so the read loop keeps handling the
		// client's permission reply / cancel while the scenario blocks.
		go func() {
			stop := runScenario(os.Getenv("FAKEACP_SCENARIO"))
			respond(id, map[string]any{
				"stopReason": stop,
				"usage":      map[string]any{"used": 4200, "window": 200000},
			})
		}()
	default:
		respondErr(*msg.ID, -32601, "method not found: "+msg.Method)
	}
}

// runScenario replays a named sequence and returns the prompt stopReason.
func runScenario(name string) string {
	switch name {
	case "", "stream_text":
		for _, chunk := range []string{"Sure, ", "I'll ", "do that."} {
			emitChunk(chunk)
			time.Sleep(time.Millisecond)
		}
		return "end_turn"

	case "tool_flow":
		emitUpdate(map[string]any{
			"sessionUpdate": "tool_call", "toolCallId": "tc_1",
			"title": "Edit main.go", "kind": "edit", "status": "in_progress",
			"rawInput": map[string]any{"path": "main.go"},
		})
		emitUpdate(map[string]any{
			"sessionUpdate": "tool_call_update", "toolCallId": "tc_1", "status": "completed",
			"content": []any{map[string]any{"type": "diff", "path": "main.go", "oldText": "a", "newText": "b"}},
		})
		return "end_turn"

	case "big_frame":
		big := strings.Repeat("x", 100*1024)
		emitUpdate(map[string]any{
			"sessionUpdate": "tool_call_update", "toolCallId": "tc_big", "status": "completed",
			"content": []any{map[string]any{"type": "text", "text": big}},
		})
		return "end_turn"

	case "malformed_then_valid":
		writeRaw("{ this is not valid json }")
		emitChunk("recovered")
		return "end_turn"

	case "permission", "permission_approve", "permission_deny", "permission_timeout":
		return permissionScenario()

	case "crash_midturn":
		emitChunk("about to crash")
		_ = out.Flush()
		os.Exit(1)
		return "" // unreachable

	default:
		emitChunk("unknown scenario: " + name)
		return "end_turn"
	}
}

// permissionScenario requests permission for a tool and blocks until the client
// decides or cancels. On approval it creates the sentinel file.
func permissionScenario() string {
	id := reqSeq.Add(1)
	ch := registerPending(id)
	sendRequest(id, "session/request_permission", map[string]any{
		"sessionId": sessionID,
		"reason":    "run a shell command",
		"toolCall": map[string]any{
			"toolCallId": "tc_p", "title": "Run ls", "kind": "execute",
			"rawInput": map[string]any{"command": "ls"},
		},
		"options": []any{
			map[string]any{"optionId": "opt_allow", "name": "Allow", "kind": "allow_once"},
			map[string]any{"optionId": "opt_reject", "name": "Reject", "kind": "reject_once"},
		},
	})

	select {
	case resp := <-ch:
		switch outcome, optID := parseOutcome(resp.Result); outcome {
		case "selected":
			if optID == "opt_allow" {
				writeSentinel()
			}
			return "end_turn"
		default: // "cancelled"
			return "cancelled"
		}
	case <-cancelCh:
		return "cancelled"
	}
}

func parseOutcome(result json.RawMessage) (string, string) {
	var r struct {
		Outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		} `json:"outcome"`
	}
	_ = json.Unmarshal(result, &r)
	return r.Outcome.Outcome, r.Outcome.OptionID
}

func writeSentinel() {
	if path := os.Getenv("FAKEACP_SENTINEL"); path != "" {
		_ = os.WriteFile(path, []byte("ran"), 0o644)
	}
}

// --- request/response plumbing ---

func registerPending(id int64) chan rpcMessage {
	ch := make(chan rpcMessage, 1)
	pendingMu.Lock()
	pending[id] = ch
	pendingMu.Unlock()
	return ch
}

func routeResponse(id int64, msg rpcMessage) {
	pendingMu.Lock()
	ch, ok := pending[id]
	if ok {
		delete(pending, id)
	}
	pendingMu.Unlock()
	if ok {
		ch <- msg
	}
}

func sendRequest(id int64, method string, params map[string]any) {
	raw, _ := json.Marshal(params)
	writeMessage(rpcMessage{JSONRPC: "2.0", ID: &id, Method: method, Params: raw})
}

func emitChunk(text string) {
	emitUpdate(map[string]any{
		"sessionUpdate": "agent_message_chunk",
		"content":       map[string]any{"type": "text", "text": text},
	})
}

func emitUpdate(update map[string]any) {
	raw, _ := json.Marshal(map[string]any{"sessionId": sessionID, "update": update})
	writeMessage(rpcMessage{JSONRPC: "2.0", Method: "session/update", Params: raw})
}

func respond(id int64, result any) {
	raw, _ := json.Marshal(result)
	writeMessage(rpcMessage{JSONRPC: "2.0", ID: &id, Result: raw})
}

func respondErr(id int64, code int, message string) {
	writeMessage(rpcMessage{JSONRPC: "2.0", ID: &id, Error: &rpcError{Code: code, Message: message}})
}

func writeMessage(msg rpcMessage) {
	b, _ := json.Marshal(msg)
	writeRaw(string(b))
}

func writeRaw(s string) {
	writeMu.Lock()
	defer writeMu.Unlock()
	_, _ = out.WriteString(s)
	_ = out.WriteByte('\n')
	_ = out.Flush()
}
