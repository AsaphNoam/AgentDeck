package runtime

import "encoding/json"

// JSON-RPC 2.0 message plumbing for the ACP stdio protocol. One JSON object per
// line (NDJSON). A frame is a request (id + method), a response (id, no method),
// or a notification (method, no id). See techspec §2, §8.1.

const jsonrpcVersion = "2.0"

// rpcMessage is the union of all three JSON-RPC frame shapes. The presence of
// ID and Method disambiguates: id+method = request, id only = response,
// method only = notification.
type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError is the JSON-RPC error object.
type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Error implements error so a peer error can be returned from Call.
func (e *rpcError) Error() string { return e.Message }

// kind classifies a decoded frame.
type frameKind int

const (
	frameUnknown      frameKind = iota
	frameRequest                // id + method (peer→us request, e.g. session/request_permission)
	frameResponse               // id, no method (reply to one of our Calls)
	frameNotification           // method, no id (e.g. session/update)
)

func (m *rpcMessage) kind() frameKind {
	switch {
	case m.ID != nil && m.Method != "":
		return frameRequest
	case m.ID != nil && m.Method == "":
		return frameResponse
	case m.ID == nil && m.Method != "":
		return frameNotification
	default:
		return frameUnknown
	}
}
