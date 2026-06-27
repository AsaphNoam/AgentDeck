// Command fakeacp is a deterministic stand-in for the real claude-code-acp ACP
// adapter, used by ChatRuntime integration tests (techspec §10.2). It speaks
// JSON-RPC over NDJSON on stdin/stdout: it answers `initialize` and
// `session/new`, and on `session/prompt` it replays the scenario named by the
// FAKEACP_SCENARIO env var, emitting `session/update` notifications and then a
// prompt result.
//
// It is intentionally standalone (no internal imports) so it builds and behaves
// like an external CLI. Scenarios grow across subphases; 1.2 ships stream_text,
// big_frame, and malformed_then_valid. Permission/tool/crash scenarios are added
// in 1.3–1.4.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

var out = bufio.NewWriter(os.Stdout)

func main() {
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue // ignore malformed input
		}
		handle(&msg)
	}
}

func handle(msg *rpcMessage) {
	// Notifications (no id) — e.g. session/cancel. Nothing to reply.
	if msg.ID == nil {
		return
	}
	switch msg.Method {
	case "initialize":
		respond(*msg.ID, map[string]any{
			"protocolVersion":   1,
			"agentCapabilities": map[string]any{},
		})
	case "session/new":
		respond(*msg.ID, map[string]any{"sessionId": sessionID})
	case "session/prompt":
		runScenario(os.Getenv("FAKEACP_SCENARIO"))
		respond(*msg.ID, map[string]any{"stopReason": "end_turn"})
	default:
		respondErr(*msg.ID, -32601, "method not found: "+msg.Method)
	}
}

// runScenario replays a named NDJSON sequence of session/update notifications.
func runScenario(name string) {
	switch name {
	case "", "stream_text":
		for _, chunk := range []string{"Sure, ", "I'll ", "do that."} {
			emitUpdate(map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content":       map[string]any{"type": "text", "text": chunk},
			})
			time.Sleep(time.Millisecond)
		}
	case "big_frame":
		// A single tool_call_update whose content exceeds 64 KiB, locking in the
		// transport's enlarged scanner buffer end-to-end (techspec §2, §10.2).
		big := strings.Repeat("x", 100*1024)
		emitUpdate(map[string]any{
			"sessionUpdate": "tool_call_update",
			"toolCallId":    "tc_big",
			"status":        "completed",
			"content":       []any{map[string]any{"type": "text", "text": big}},
		})
	case "malformed_then_valid":
		// Emit a deliberately broken line, then a valid chunk; the runtime must
		// resync and surface only the valid frame (techspec §8.3).
		writeRaw("{ this is not valid json }")
		emitUpdate(map[string]any{
			"sessionUpdate": "agent_message_chunk",
			"content":       map[string]any{"type": "text", "text": "recovered"},
		})
	default:
		// Unknown scenario: behave like stream_text with a single marker chunk.
		emitUpdate(map[string]any{
			"sessionUpdate": "agent_message_chunk",
			"content":       map[string]any{"type": "text", "text": "unknown scenario: " + name},
		})
	}
}

func emitUpdate(update map[string]any) {
	params := map[string]any{"sessionId": sessionID, "update": update}
	raw, _ := json.Marshal(params)
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
	fmt.Fprint(out, s, "\n")
	out.Flush()
}
