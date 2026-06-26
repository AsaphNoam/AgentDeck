# Phase 1 — Core Loop — Implementation Tech Spec

**Mirrors:** `docs/phases/phase-1-core-loop.md` (phase PRD)
**Master PRD:** `agent-dashboard-prd.md` (source of truth — §4.1 runtime abstraction, §4.4 hooks, §4.5 storage, F3, F4)
**Builds on:** Phase 0 (config file store + `state.db` + `127.0.0.1` server skeleton + CLI)
**Status:** ready to implement
**Audience:** the engineer implementing Phase 1. This document is intended to be complete enough to implement with essentially no further design decisions. Where the master PRD leaves something open (§9), this spec pins a concrete decision — see §12. The rationale behind the storage split, the hook token, and the in-process MCP server is recorded in [`docs/architecture-decisions.md`](../../architecture-decisions.md).

---

## 1. Overview & scope recap

### 1.1 What this phase delivers

The vertical spine: **one** Claude Code agent, launched via REST or CLI, wrapped through the **ACP chat runtime** over stdio, accepting a prompt and streaming a normalized transcript (assistant text, tool calls, tool results, diffs, permission prompts, turn end) back to the launching client over an interim per-agent SSE stream. Permission requests **gate execution** until the client approves or denies. The agent's status row in `state.db` transitions `idle → busy → idle` across each turn.

This is the highest-risk phase because two things must be gotten exactly right and are hard to fake convincingly later:

1. **The ACP wire format** (JSON-RPC over NDJSON on stdio) and its mapping to our normalized transcript events.
2. **Permission gating** — a tool call that requires permission must actually pause the agent until a decision is relayed back over the same ACP channel.

### 1.2 In scope

- `Runtime` Go interface + a `Registry` that dispatches by `agent.interface`.
- **Chat runtime** implementation: process-group spawn of the Claude Code ACP CLI, ACP JSON-RPC framing over stdio, normalization of the ACP stream into AgentDeck transcript events.
- Config composition at launch (`project.cwd` + `project.context_prompt` + `role.system_prompt` + `backend/model`; backend `env` then per-model `env` override).
- Launch flow: `POST /api/sessions` and the `agentdeck role@project ...` CLI form (CLI calls the same REST endpoint). Launch inserts identity/running/status rows into `state.db`, mints a per-launch hook token, and registers the in-process Go MCP messaging server with the agent.
- `prompt`, `cancel`, `stop`, and `permission` REST endpoints.
- Interim per-agent SSE: `GET /api/sessions/{id}/events`, with a payload envelope **forward-compatible** with Phase 2's multiplexed `new_message` bus.
- Status updates written to `state.db` across the turn lifecycle.
- One backend end-to-end: Claude Code (`type: "claude-acp"`).
- A **fake ACP CLI** for deterministic tests.

### 1.3 Out of scope (and explicitly stubbed)

| Item | Status this phase | Lands in |
|------|-------------------|----------|
| Dashboard / chat UI | Out — verify via `curl` + an SSE client / tiny test HTML page | Phase 2 |
| `POST /api/hook` ingest endpoint + multiplexed `/api/events` SSE bus | Out — interim per-agent stream only; the hook token is minted now (§6.1) but its ingest endpoint lands in Phase 2 | Phase 2 |
| **Codex backend** (`codex-acp`) | **Stubbed** — interface designed for it; registry/composition recognize it but `Start` returns `not_implemented` | later |
| **Terminal runtime** (`interface: "terminal"`) | **Stubbed** — registry returns `not_implemented` | Phase 6 |
| `Runtime.Resume` | **Stubbed** — returns `not_implemented`; identity plumbing in place | Phase 4 |
| `Runtime.CheckMessages` | **Stubbed** — returns `not_implemented` | Phase 5 |
| MCP messaging server **tools** (`list_agents`/`send_message`/`check_messages`) | Out — this phase **registers** the in-process server with the agent; the tool handlers are implemented in Phase 5 | Phase 5 |
| Hook ingest for terminal agents | Out — chat runtime derives status from the ACP stream directly (master PRD §4.4: chat agents derive status from the stream; terminal agents POST hooks) | Phase 2 |
| Persistence of transcript to `sessions/{id}/` | Out — transcript is in-memory + streamed only; FTS5 indexing is later | Phase 4 |
| Launch queueing / concurrency limits | Out — single agent assumed; see §12 | later |

> The registry, the `Runtime` interface, and the transcript event shapes are designed now so that the stubbed pieces slot in without breaking callers.

---

## 2. Technology choices

All server-side; Go 1.22+, single binary (Phase 0 constraint). Prefer the standard library; pull a dependency only where the stdlib is genuinely insufficient.

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Child-process management | `os/exec` (stdlib) + `syscall` for process groups | No external dep needed. `exec.Cmd` gives us `Stdin`/`Stdout`/`Stderr` pipes. We need the process **group** so cancel/stop signal the whole tree (see §2.1). |
| Process group / signalling | `syscall.SysProcAttr{Setpgid: true}` on spawn; signal `-pgid` | Standard POSIX approach, works on macOS + Linux (our only platforms). Lets us `kill(-pgid, SIGTERM)` to take down the CLI and any children it forked. |
| State persistence | SQLite `state.db` via the Phase 0 store package; the Go server is the **sole writer** | Machine state (identity, running registry, live status) lives in `state.db` (master PRD §4.5). Launch and the chat runtime write rows; no other process writes the DB, which is what makes single-file SQLite safe here. Config objects (roles/projects/backends) are read from the config file store. |
| stdio JSON-RPC / NDJSON parsing | `bufio.Scanner` (line framing) + `encoding/json` (per-line decode) | ACP frames one JSON object per line (NDJSON). `bufio.Scanner` with an enlarged buffer reads line-delimited frames; `encoding/json` decodes each. Avoid `json.Decoder.Decode` streaming from the pipe directly because we want explicit line boundaries for resync after a malformed frame, and we want to log the raw line on parse failure. |
| Outbound JSON-RPC writes | `encoding/json` Marshal + a mutex-guarded write to the child's stdin, newline-terminated | One writer goroutine / serialized writes so concurrent `SendPrompt`/`permission` responses never interleave a half-written frame on stdin. |
| SSE on the Go side | stdlib `net/http` + manual SSE framing (`text/event-stream`, `data: ...\n\n`, flush per event) | SSE is trivial to emit from `net/http`; a library adds nothing. We control the exact wire shape, which matters for Phase 2 forward-compat. Use `http.Flusher` to flush each event. Set `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`. |
| Per-agent event fan-out | An in-process `Hub` of Go channels (one buffered chan per subscriber) | The interim stream needs one or more subscribers per agent. A small hub with bounded buffered channels and drop-oldest matches Phase 2's bus semantics so the code generalizes cleanly. |
| Scanner buffer size | `bufio.Scanner.Buffer` raised to 8 MiB max token | ACP `tool_result`/`diff` frames can be large (full file patches). The default 64 KiB line cap would truncate them. |
| In-process MCP messaging server | Official Go MCP SDK (`modelcontextprotocol/go-sdk`), stdio transport | The agent-to-agent messaging server runs **inside the Go binary** (master PRD §4.5). This phase registers it with the agent at launch (§6.4); its tool handlers land in Phase 5. Hosting it in-process means the tools become direct reads/writes of `state.db` with no serialization boundary. |
| UUID / ids | reuse Phase 0's `agent_id` generator; JSON-RPC request ids = monotonic `int64` per process; hook tokens via `crypto/rand` | No new dep beyond the MCP SDK. |

> **Buffer enlargement is load-bearing.** A single oversized ACP frame silently dropped by a 64 KiB scanner cap is one of the most likely "it works in tests, fails on a real edit" bugs. The fake CLI test suite (§10) must emit a frame > 64 KiB to lock this in.

### 2.1 Why a process group

Claude Code (and Codex later) may spawn subprocesses (shells for tool calls, language servers). `Cancel` interrupts the current turn; `Stop` must terminate everything we started. Signalling only the direct child can orphan its children. With `Setpgid: true` the child becomes a group leader; we `syscall.Kill(-pgid, sig)` to hit the whole group. The `pgid` equals the child's pid (it is the leader), and that pid is what we record in the running row in `state.db`.

---

## 3. The Runtime interface

### 3.1 Definition

```go
package runtime

import (
	"context"

	"agentdeck/internal/store" // Phase 0 store: config file objects + state.db rows
)

// LaunchSpec is the fully-composed input to Start. The launch flow (§6) builds
// this from agent identity + project + role + backend/model so the Runtime needs
// no further lookups.
type LaunchSpec struct {
	Agent        store.Agent       // stable identity (agent_id, role, project, backend, model, interface)
	Cwd          string            // resolved absolute working dir (project.cwd, ~-expanded)
	AddDirs      []string          // project.add_dirs, ~-expanded
	SystemPrompt string            // composed: context_prompt + role.system_prompt (see §6.2)
	BackendType  string            // "claude-acp" | "codex-acp"
	ModelID      string            // provider model id, e.g. "claude-sonnet-4-6"
	Env          []string          // composed env layering (backend env then per-model override), "K=V"
	SkipPerms    bool              // effective skip_permissions after role/global resolution (§5, §12)
	HookToken    string            // per-launch one-time token passed to the agent's hooks (§6.4)
	MCPServers   []MCPServerSpec   // messaging MCP server registration (§6.4); one entry this phase
	ExtraArgs    []string          // reserved (e.g. extra adapter flags) — empty this phase
}

// MCPServerSpec is one stdio MCP server the agent should connect to. This phase
// carries exactly one: the in-process Go messaging server (§6.4).
type MCPServerSpec struct {
	Name    string   // "agentdeck-messaging"
	Command string   // path to invoke; for the in-process server this re-execs the
	                 // AgentDeck binary in a hidden "mcp-stdio" mode that pipes to the hub
	Args    []string // includes the hook token / agent_id so the server scopes to this agent
	Env     []string // "K=V"
}

// Handle is the live, in-memory representation of a started runtime. Returned by
// Start and held by the Registry keyed by agent_id. Not persisted.
type Handle struct {
	AgentID   string
	Pid       int    // == pgid; written to the running row in state.db
	SessionID string // ephemeral CLI session id, written to the running row in state.db
	// internal: process handle, stdin writer, event hub, pending-permission map, cancel fn
}

// Event is the normalized transcript event emitted to subscribers (§4.2).
type Event struct {
	AgentID string          `json:"agent_id"`
	Seq     int64           `json:"seq"`  // monotonic per agent, starts at 1
	Type    string          `json:"type"` // see EventType constants
	Data    json.RawMessage `json:"data"` // type-specific payload (§4.2)
	Ts      string          `json:"ts"`   // RFC3339 UTC
}

// Runtime is the interface the server programs against (master PRD §4.1).
// One implementation this phase: ChatRuntime. TerminalRuntime is a stub.
type Runtime interface {
	// Start spawns the CLI, performs the ACP initialize handshake, records the
	// ephemeral session id, inserts the running + initial status rows in state.db,
	// and returns a Handle. Idempotent guard: erroring if a Handle already exists
	// for the agent is the caller's (Registry) responsibility.
	Start(ctx context.Context, spec LaunchSpec) (*Handle, error)

	// SendPrompt submits one user turn. Non-blocking: it writes the prompt frame
	// and returns; transcript events stream asynchronously via the agent's hub.
	// Errors if the agent is not started or a turn is already in flight (§12: no queue).
	SendPrompt(ctx context.Context, agentID, text string) error

	// Cancel interrupts the in-progress turn (ACP cancel; see §8.4). Safe to call
	// when idle (no-op). Does not stop the process.
	Cancel(ctx context.Context, agentID string) error

	// Stop terminates the process group, removes the running row from state.db,
	// sets the status row's state. Idempotent.
	Stop(ctx context.Context, agentID string) error

	// Resume re-attaches to a persisted session. STUB this phase: returns
	// ErrNotImplemented. Signature fixed now for Phase 4.
	Resume(ctx context.Context, spec LaunchSpec, sessionID string) (*Handle, error)

	// CheckMessages wakes an idle agent to drain its mailbox. STUB this phase:
	// returns ErrNotImplemented. Signature fixed now for Phase 5.
	CheckMessages(ctx context.Context, pid int) error

	// Permission relays an approve/deny decision back over ACP for a pending
	// permission request (§5). Errors if no such pending request.
	Permission(ctx context.Context, agentID, toolCallID, decision string) error

	// Subscribe returns a channel of normalized events for an agent and an
	// unsubscribe func. Used by the interim SSE handler (§7). Buffered, drop-oldest.
	Subscribe(agentID string) (<-chan Event, func(), error)
}
```

> `Permission` and `Subscribe` are not in the master PRD's "minimum" method list (§4.1 lists `Start/SendPrompt/Cancel/Stop/Resume/CheckMessages`). They are required to make gating and streaming work and are additive — they do not change the PRD's minimum set, which all remain present.

### 3.2 Registry

```go
type Registry struct {
	mu       sync.Mutex
	handles  map[string]*Handle      // agent_id -> live handle
	byIface  map[string]Runtime      // "chat" -> ChatRuntime, "terminal" -> stub
	store    *store.Store            // config file objects + state.db
}

func NewRegistry(s *store.Store) *Registry {
	r := &Registry{handles: map[string]*Handle{}, byIface: map[string]Runtime{}, store: s}
	r.byIface["chat"] = NewChatRuntime(s, /*hub*/)
	r.byIface["terminal"] = notImplementedRuntime{name: "terminal"}
	return r
}

// runtimeFor dispatches by agent.interface. Backend (claude vs codex) is handled
// inside ChatRuntime.Start via spec.BackendType.
func (r *Registry) runtimeFor(iface string) (Runtime, error) {
	rt, ok := r.byIface[iface]
	if !ok {
		return nil, fmt.Errorf("%w: interface %q", ErrNotImplemented, iface)
	}
	return rt, nil
}
```

`ErrNotImplemented` is a sentinel (`errors.New("not implemented")`) the API layer maps to HTTP `501` (§7.7).

### 3.3 Real vs stubbed this phase

| Method | chat (claude-acp) | chat (codex-acp) | terminal |
|--------|-------------------|------------------|----------|
| `Start` | **real** | stub → `501` | stub → `501` |
| `SendPrompt` | **real** | — | — |
| `Cancel` | **real** | — | — |
| `Stop` | **real** | — | — |
| `Permission` | **real** | — | — |
| `Subscribe` | **real** | — | — |
| `Resume` | stub → `501` | stub | stub |
| `CheckMessages` | stub → `501` | stub | stub |

`ChatRuntime.Start` checks `spec.BackendType`: `claude-acp` proceeds; anything else returns `ErrNotImplemented`.

---

## 4. Chat runtime design

### 4.1 Process spawn & ACP handshake

On `Start(spec)`:

1. **Resolve the CLI invocation** for `claude-acp` (§12 pins the binary + flags):
   - Binary: `claude-code-acp` (the ACP adapter for Claude Code; see §12.1 for version pin and fallback).
   - Args: ACP runs as a long-lived JSON-RPC server over stdio — no per-prompt args. cwd, model, system prompt, add-dirs, and MCP servers are passed **in the ACP `session/new` params**, not as CLI flags, because the ACP session model is what carries them. (Flags that the adapter also accepts, e.g. `--model`, are set redundantly where supported; the authoritative path is `session/new`.)
2. **Spawn** with `exec.CommandContext`:
   ```go
   cmd := exec.Command(bin, args...)
   cmd.Dir = spec.Cwd
   cmd.Env = spec.Env
   cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
   stdin, _ := cmd.StdinPipe()
   stdout, _ := cmd.StdoutPipe()
   stderr, _ := cmd.StderrPipe()
   cmd.Start()
   pgid := cmd.Process.Pid // group leader
   ```
3. **Start reader goroutines:** one `bufio.Scanner` over stdout (NDJSON frames → `dispatch`), one over stderr (log lines, captured into a ring buffer for diagnostics; surfaced on crash as an `error` event).
4. **ACP `initialize` handshake** (JSON-RPC request, client→server):
   - Send `initialize` with `protocolVersion` and client capabilities (we advertise that we handle `session/request_permission` and `fs/*` is delegated to the agent).
   - Await the `initialize` result (protocol version negotiation). On version mismatch beyond our pinned range → fail Start with a clear error.
5. **`session/new`** request with params:
   ```jsonc
   {
     "cwd": "<spec.Cwd absolute>",
     "mcpServers": [                   // the in-process messaging server (§6.4)
       {
         "name": "agentdeck-messaging",
         "command": "<spec.MCPServers[0].Command>",
         "args": ["<spec.MCPServers[0].Args...>"],
         "env": { /* spec.MCPServers[0].Env */ }
       }
     ],
     // model + system prompt + add_dirs passed per adapter convention:
     "model": "<spec.ModelID>",
     "systemPrompt": "<spec.SystemPrompt>",
     "additionalDirectories": ["<spec.AddDirs...>"]
   }
   ```
   The result carries the **ephemeral `sessionId`** → store as `Handle.SessionID`.
6. **Persist:** insert the running row (`pid=pgid`, `session_id`, `interface:"chat"`, `started_at`) and the initial status row (`state:"idle"`, `detail:"ready"`, `context_pct:0`) in `state.db`. The server is the sole writer.
7. Register the `Handle` in the registry, return.

> **Where composed config goes.** cwd → `cmd.Dir` *and* `session/new.cwd`. system prompt → `session/new.systemPrompt`. model → `session/new.model`. add_dirs → `session/new.additionalDirectories`. env → `cmd.Env`. messaging MCP server → `session/new.mcpServers`. This is the single composition point (§6).

### 4.2 Normalized internal transcript events

These are AgentDeck's own event types — the contract Phase 2's chat panel and Phase 4's persistence consume. They are **independent of the ACP wire shape** so that swapping/adding a backend (Codex) does not change anything downstream. The envelope is the `Event` struct (§3.1); `Data` is one of the following per `Type`.

`EventType` constants:

```go
const (
	EvAssistantText     = "assistant_text"
	EvToolCall          = "tool_call"
	EvToolResult        = "tool_result"
	EvDiff              = "diff"
	EvPermissionRequest = "permission_request"
	EvTurnEnd           = "turn_end"
	EvError             = "error"
)
```

Payload shapes (`Event.Data`):

```go
// assistant_text — a streamed markdown delta (NOT cumulative; append on the client)
type AssistantTextData struct {
	Delta string `json:"delta"`
}

// tool_call — the agent intends to / begins to run a tool
type ToolCallData struct {
	ToolCallID string          `json:"tool_call_id"` // stable id used to correlate result + permission
	Name       string          `json:"name"`         // e.g. "Edit", "Bash", "Read"
	Title      string          `json:"title"`        // human label from ACP if present, else Name
	Args       json.RawMessage `json:"args"`         // raw tool arguments object
	Status     string          `json:"status"`       // "pending" | "in_progress"
}

// tool_result — outcome of a tool call
type ToolResultData struct {
	ToolCallID string          `json:"tool_call_id"`
	Status     string          `json:"status"` // "completed" | "failed"
	Content    json.RawMessage `json:"content"` // textual/structured result content
	Error      string          `json:"error,omitempty"`
}

// diff — a file edit expressed as a patch (often arrives within a tool_call/update)
type DiffData struct {
	ToolCallID string `json:"tool_call_id"`
	Path       string `json:"path"`         // absolute or cwd-relative file path
	OldText    string `json:"old_text"`     // may be empty for new files
	NewText    string `json:"new_text"`
	Patch      string `json:"patch"`        // unified diff if the adapter provides one, else derived
}

// permission_request — execution is PAUSED awaiting approve/deny (§5)
type PermissionRequestData struct {
	ToolCallID string   `json:"tool_call_id"`
	Name       string   `json:"name"`     // tool requiring permission
	Reason     string   `json:"reason"`   // why permission is needed
	Args       json.RawMessage `json:"args"`
	Options    []PermOption `json:"options"` // ACP-offered options; we map to approve/deny (§5.3)
	AutoApproved bool   `json:"auto_approved"` // true when skip_permissions bypassed the gate (§5.2)
	ExpiresAt  string   `json:"expires_at"` // RFC3339; after this we auto-deny (§5.4)
}
type PermOption struct {
	OptionID string `json:"option_id"`
	Label    string `json:"label"`
	Kind     string `json:"kind"` // "allow_once" | "allow_always" | "reject_once" | "reject_always"
}

// turn_end — the assistant turn completed (success or stopped)
type TurnEndData struct {
	StopReason string  `json:"stop_reason"` // "end_turn" | "cancelled" | "max_tokens" | "error"
	ContextPct float64 `json:"context_pct"` // 0..1 if reported, else last-known
}

// error — runtime/protocol/process error surfaced to the client
type ErrorData struct {
	Scope   string `json:"scope"`   // "protocol" | "process" | "tool" | "internal"
	Message string `json:"message"`
	Fatal   bool   `json:"fatal"`   // true => session is dead, Stop has been performed
}
```

### 4.3 ACP → normalized mapping

ACP (the version pinned in §12.1) drives the agent turn primarily through **`session/update` notifications** (server→client) and one **`session/request_permission` request** (server→client, expects a result). The mapping:

| ACP message | ACP `update.sessionUpdate` (or method) | → normalized event |
|-------------|----------------------------------------|--------------------|
| `session/update` | `agent_message_chunk` (text content delta) | `assistant_text` (`delta` = chunk text) |
| `session/update` | `agent_thought_chunk` | *dropped* this phase (reasoning not surfaced) |
| `session/update` | `tool_call` (new tool call, status `pending`/`in_progress`) | `tool_call` |
| `session/update` | `tool_call_update` (status → `completed`/`failed`, content) | `tool_result` (+ `diff` if it carries a `diff`/`content[].type=="diff"`) |
| `session/update` | `tool_call`/`tool_call_update` whose content includes a `diff` block | `diff` (one per file changed) |
| `session/update` | `plan` / other update kinds | *dropped* this phase |
| `session/request_permission` (request) | — | `permission_request`; the runtime **holds the JSON-RPC response** until a decision arrives (§5) |
| `session/prompt` result (response to our request) | `stopReason` | `turn_end` (map `stopReason`) |
| Any JSON-RPC `error` / parse failure / process exit | — | `error` |

**Correlation:** ACP `tool_call.toolCallId` (or `id`) is propagated verbatim into our `ToolCallID` and reused for `tool_result`, `diff`, and `permission_request`, so the client can stitch them. If the adapter omits an id on a permission request, synthesize one and keep a map from ACP request-id → our `ToolCallID` so the response routes back correctly.

**Context percentage:** if `session/update` or the prompt result carries token-usage / context info, compute `context_pct = used / window`, cache it on the handle, emit it on `turn_end`, and write it to the status row in `state.db`. If the adapter does not report usage, leave `context_pct` at last-known (initial `0`) and document that it may be `0` until usage data is available (§12.1).

**Sequence numbers:** every emitted `Event` gets a per-agent monotonic `Seq` (starts at 1). This lets Phase 2/4 detect gaps and order events; the interim SSE includes it as the SSE `id:` field.

### 4.4 Status updates across the turn lifecycle

The agent's status row in `state.db` (master PRD §3.1 / §4.5 shape) is updated transactionally at these transitions. Chat agents derive status from the ACP stream (the `POST /api/hook` ingest path, added in Phase 2, carries lifecycle for terminal agents instead).

| Moment | `state` | `detail` | `last_trace` | `busy_since` | `context_pct` |
|--------|---------|----------|--------------|--------------|---------------|
| After `Start` | `idle` | `"ready"` | `"SessionStart"` | cleared (`""`/null) | `0` |
| `SendPrompt` accepted | `busy` | `"thinking"` | `"UserPromptSubmit"` | now (RFC3339) | last-known |
| `tool_call` seen | `busy` | `"Running <Name>"` | `"PreToolUse: <Name>"` | unchanged | last-known |
| `tool_result` seen | `busy` | `"<Name> done"` | `"PostToolUse: <Name>"` | unchanged | last-known |
| `permission_request` seen | `waiting_input` | `"Permission: <Name>"` | `"PermissionRequest: <Name>"` | unchanged | last-known |
| decision relayed | `busy` | `"thinking"` | `"PermissionResolved"` | unchanged | last-known |
| `turn_end` (end_turn) | `idle` | last assistant snippet (≤120 chars) | `"Stop"` | cleared | from turn_end |
| `turn_end` (cancelled) | `idle` | `"cancelled"` | `"Cancelled"` | cleared | last-known |
| fatal `error` | `error` | error message (≤120 chars) | `"Error"` | cleared | last-known |
| `Stop` | `done` (running row deleted) | — | — | cleared | last-known |

> Debouncing status writes is **not** needed this phase (single agent, low write rate; each write is a single-row `UPDATE`). Phase 2's status reads come from `state.db`; the `last_trace` vocabulary (`PreToolUse`, `PostToolUse`, `Stop`, …) deliberately mirrors the master PRD §4.4 hook trace names so terminal-runtime hook output (POSTed in Phase 2) and chat-runtime stream output land in identical status rows downstream.

---

## 5. Permission gating

The acceptance-critical path. A tool call requiring permission must **block the agent** until the launching client decides.

### 5.1 Flow

```
agent turn running
      │
      ▼
ACP server→client:  session/request_permission { sessionId, toolCall{id,name,args}, options[] }
      │
ChatRuntime:
  1. record pending: pendingPerms[toolCallID] = { rpcRequestID, optionsByKind, timer }
  2. set status state=waiting_input (state.db)
  3. emit normalized permission_request event (with ExpiresAt = now + timeout)
  4. DO NOT respond to the JSON-RPC request yet  ← this is what pauses the agent
      │
      ▼  (out of band)
client → POST /api/sessions/{id}/permission { tool_call_id, decision: "approve"|"deny" }
      │
ChatRuntime.Permission:
  5. look up pending by toolCallID; if absent → 409
  6. choose the ACP optionId for the decision (§5.3)
  7. send JSON-RPC result for the held request: { outcome: "selected", optionId }
  8. cancel the timer, delete pending, status state=busy (state.db)
      │
      ▼
agent resumes: runs the tool (approve) or skips/aborts it (deny) and continues the turn
```

The pause is achieved by **withholding the JSON-RPC response** to `session/request_permission`. ACP servers block the tool on that response, so no separate "pause" command is needed — not replying *is* the pause.

### 5.2 skip_permissions bypass

If `spec.SkipPerms == true` (resolved per §12.2), the runtime auto-approves: on `session/request_permission` it immediately replies with the `allow_once` option and emits the `permission_request` + an immediate synthetic resolution, **without** entering `waiting_input`. (We still emit the `permission_request` event so the transcript records that a permission was granted; it carries `AutoApproved: true`.)

Where supported by the adapter, we also pass a session-level "bypass permissions"/"accept edits" mode in `session/new` so the agent does not even ask — but the runtime-side auto-approve above is the authoritative guarantee in case the adapter still asks.

### 5.3 Mapping decision → ACP option

ACP offers an `options` array; each has a `kind`. We map:

- `decision == "approve"` → prefer the option with `kind == "allow_once"`; fall back to `allow_always` if `allow_once` absent.
- `decision == "deny"` → prefer `reject_once`; fall back to `reject_always`.
- If none of the expected kinds exist, respond with `outcome: "cancelled"` and emit an `error` event noting the adapter offered no usable option.

Phase 1 exposes only binary approve/deny over REST; "always" variants are a Phase-3 policy concern. We deliberately use the **once** variants so a Phase-1 approval never silently grants standing permission.

### 5.4 Timeout & deny semantics

- **Timeout:** default **180s** (`PERMISSION_TIMEOUT`, configurable). On expiry the runtime auto-**denies** (sends `reject_once`), emits a `permission_request`-resolved `error` (`scope:"tool"`, message `"permission timed out"`), and returns status to `busy`/then the agent typically ends the turn. Rationale: a hung prompt must not pin an agent in `waiting_input` forever.
- **Deny semantics:** denying relays `reject_once` to ACP. The agent receives the rejection and **must not run the tool**; it typically continues the turn (e.g. explains it could not proceed) or ends it. The acceptance test asserts the tool's side effect (e.g. the file write) **did not happen** after a deny.
- **Concurrent requests:** ACP issues permission requests serially within a turn (one tool at a time). The pending map is keyed by `toolCallID` to be safe if that assumption ever breaks; a `POST .../permission` for an unknown id → `409 conflict`.

---

## 6. Launch flow & config composition

### 6.1 Order of operations (`POST /api/sessions` and CLI)

1. Validate `role`, `project`, `backend`, `model`, `interface` against the config file store; unknown → `422` (§7.7).
2. Resolve effective `skip_permissions` (§12.2).
3. Generate `agent_id` (Phase 0 generator), auto-suggest `name` if absent (§6.3).
4. Insert the identity row in `state.db` (stable identity).
5. Mint the **per-launch hook token** (`crypto/rand`, stored on the identity/launch record so the hook ingest endpoint can validate it in Phase 2) and build the messaging MCP server registration (§6.4).
6. Build `LaunchSpec` via composition (§6.2), including `HookToken` and `MCPServers`.
7. `Registry` → `runtimeFor(interface)` → `Start(spec)`.
8. On success, the runtime has already inserted the running row + initial status row in `state.db` (§4.1 step 6). On failure, **roll back**: delete the identity row, ensure no running/status rows remain, return `502`/`501` (§7.7).
9. Return `201` with `{ agent, running, status }`.

> Identity is inserted **before** Start so a launch that crashes mid-handshake still has a stable id for diagnostics; the rollback removes it only if Start fails outright. This preserves the stable-`agent_id`-vs-ephemeral-`session_id` invariant (load-bearing concept): the agent_id is minted once, the session_id comes from the CLI on every Start/Resume.

### 6.2 Config composition

```
LaunchSpec.Cwd          = expand(project.cwd)                         // ~ and env expanded, absolute
LaunchSpec.AddDirs      = map(expand, project.add_dirs)
LaunchSpec.SystemPrompt = join("\n\n", [ project.context_prompt,      // project context first
                                         role.system_prompt ])        // then role persona
LaunchSpec.BackendType  = backends[backend].type                      // "claude-acp"
LaunchSpec.ModelID      = backends[backend].models[model].model       // e.g. "claude-sonnet-4-6"
LaunchSpec.Env          = composeEnv(os.Environ(),
                                     backends[backend].env,           // backend-level layer
                                     backends[backend].models[model].env) // per-model OVERRIDE
LaunchSpec.SkipPerms    = resolveSkip(config, role)                   // §12.2
LaunchSpec.HookToken    = mintToken()                                 // per-launch, §6.4
LaunchSpec.MCPServers   = []MCPServerSpec{ messagingServer(agent_id, hookToken) } // §6.4
```

- **Config objects come from the file store.** `role`, `project`, and `backends` are read from the config file store (`roles/`, `projects/`, `backends.json`, `config.json`). Only machine state (identity/running/status) is in `state.db`.
- **System prompt order is fixed:** `project.context_prompt` then `role.system_prompt`, separated by a blank line. Empty components are skipped (no leading/trailing blank lines).
- **Env layering is fixed:** start from the server process env, overlay backend `env`, then overlay per-model `env` (per-model wins on key collision), per master PRD §3.4. `composeEnv` returns a deduped `[]string` of `K=V`.
- Composition happens **only** in the launch flow; the runtime receives a finished `LaunchSpec` and does no further lookups. This keeps the master-PRD invariant "edits affect future launches only" — a running agent's spec is frozen.

### 6.3 Name auto-suggestion

If `name` omitted: pick a stable, friendly name. Algorithm: a curated wordlist (e.g. `["Atlas","Nova","Echo",…]`); choose the first not currently used by a live agent (query the running rows in `state.db`); if all used, append a numeric suffix. Deterministic given the current live set (testable).

### 6.4 Hook token & MCP server registration

Two things are wired into every launch so later phases slot in without re-plumbing:

- **Per-launch hook token.** Launch mints a one-time token (`crypto/rand`) and passes it to the agent's hooks via `LaunchSpec.HookToken` (surfaced to the agent's hook environment / config so a hook can later `POST /api/hook` with the token in a header). The server records the token against the agent so the Phase 2 ingest endpoint can validate it and reject spoofed status from other local processes. This phase mints and plumbs the token; the `POST /api/hook` endpoint itself lands in Phase 2.
- **In-process Go MCP messaging server registration.** Launch builds an `MCPServerSpec` for the agent-to-agent messaging server and passes it in `session/new.mcpServers` (§4.1 step 5). The server is hosted **inside the AgentDeck Go binary** (official Go MCP SDK over stdio); the registration entry invokes the binary in a hidden stdio MCP mode scoped to this `agent_id`/token, so the messaging tools read/write `state.db` in-process. This phase performs the **registration only** — the tool handlers (`list_agents` / `send_message` / `check_messages`) are implemented in Phase 5. Registering now keeps the launch composition identical between phases.

### 6.5 CLI parsing

`agentdeck <role>@<project> [--backend B] [--model M] [--interface chat|terminal] [--name N] [--group G]`

- Parse the `role@project` positional (split on the **last** `@`; both sides required, else usage error).
- Defaults: `--backend` → `backends.default` backend; `--model` → that backend's `default_model`; `--interface` → `chat`; `--name` → omitted (server auto-suggests).
- The CLI then **POSTs to `http://127.0.0.1:{port}/api/sessions`** with exactly `{role, project, backend, model, interface, name?, group?}` — it does not launch the runtime itself. This guarantees CLI and modal produce an identical agent (acceptance criterion). If the server is not running, print a hint to run `agentdeck dashboard start`.
- Reuses Phase 0's reserved `agentdeck <role>@<project>` stub slot.

---

## 7. API contracts

Base: `http://127.0.0.1:{port}/api`. All bodies JSON (`Content-Type: application/json`). All error responses use the shape in §7.7.

### 7.1 POST /api/sessions — launch

Request:
```json
{ "role": "implementer", "project": "my-app", "backend": "claude",
  "model": "sonnet-4-6", "interface": "chat", "name": "Atlas", "group": "auth-migration" }
```
`name` and `group` optional; `backend`/`model`/`interface` optional and default per §6.5.

Response `201 Created`:
```json
{
  "agent": {
    "agent_id": "a_8f3c12", "name": "Atlas", "role": "implementer",
    "project": "my-app", "backend": "claude", "model": "sonnet-4-6",
    "interface": "chat", "created_at": "2026-06-22T10:00:00Z", "group": "auth-migration"
  },
  "running": {
    "agent_id": "a_8f3c12", "pid": 48213, "session_id": "claude-sess-xyz",
    "interface": "chat", "started_at": "2026-06-22T10:00:01Z"
  },
  "status": {
    "agent_id": "a_8f3c12", "state": "idle", "detail": "ready",
    "last_trace": "SessionStart", "busy_since": "", "context_pct": 0
  }
}
```
Errors: `422` (unknown role/project/backend/model or bad interface), `501` (interface/backend not implemented — terminal/codex), `502` (runtime failed to start, e.g. CLI not found / handshake failed), `500`.

### 7.2 GET /api/sessions/{id} — detail

Response `200`: same `{ agent, running, status }` shape, reading the identity/running/status rows from `state.db` (running omitted/null if stopped). `404` if no identity row for `{id}`.

### 7.3 POST /api/sessions/{id}/prompt

Request: `{ "text": "Add a null check to parseUser()" }`
Response `202 Accepted`: `{ "accepted": true, "agent_id": "a_8f3c12" }` — the turn streams over the events endpoint.
Errors: `404` (unknown agent), `409` (agent not started / a turn already in flight — single-agent, no queue, §12.3), `422` (empty text).

### 7.4 POST /api/sessions/{id}/cancel

Request: empty body. Response `202`: `{ "cancelled": true }`. Idempotent: cancelling an idle agent returns `202` with `{"cancelled": false}`. `404` unknown agent.

### 7.5 POST /api/sessions/{id}/stop

Request: empty body. Response `200`: `{ "stopped": true }`. Terminates the process group, deletes the running row from `state.db`, and updates the status row. **Decision: delete the running row, set the status row's `state="done"`, and keep the status row so the archive/UI can show a final state.** Idempotent. `404` unknown agent.

### 7.6 POST /api/sessions/{id}/permission

Request: `{ "tool_call_id": "tc_42", "decision": "approve" }` (`decision ∈ {"approve","deny"}`).
Response `200`: `{ "resolved": true, "tool_call_id": "tc_42", "decision": "approve" }`.
Errors: `404` (unknown agent), `409` (no pending permission for that `tool_call_id`, or already resolved/timed-out), `422` (bad decision value).

### 7.7 Error shape & status-code policy

```json
{ "error": { "code": "not_implemented", "message": "interface \"terminal\" is not implemented in this phase", "details": {} } }
```
`code` vocabulary: `validation` (422), `not_found` (404), `conflict` (409), `not_implemented` (501), `runtime_start_failed` (502), `internal` (500). `ErrNotImplemented` → `501`/`not_implemented`.

### 7.8 GET /api/sessions/{id}/events — interim per-agent SSE

`Content-Type: text/event-stream`. On connect, the handler **subscribes** to the agent's hub (`Runtime.Subscribe`) and streams every `Event` as it is produced. It also replays the current status row (read from `state.db`) as a synthetic `state_update`-shaped event so a late-joining client has context (full transcript replay is Phase 4; this phase streams from connect-time, documented).

Wire format per event — **forward-compatible with Phase 2's multiplexed `new_message`**:

```
id: 7
event: message
data: {"agent_id":"a_8f3c12","seq":7,"type":"assistant_text","ts":"2026-06-22T10:04:12Z","data":{"delta":"Sure, I'll "}}

```

- The `data:` JSON is exactly the `Event` struct (§3.1). Phase 2 wraps this same object as the `payload` of a `new_message` SSE event on the multiplexed bus — so a Phase-2 client can reuse the identical parser. The `agent_id` is in-payload precisely so the multiplexed bus can route without a separate channel.
- SSE `event:` name is `message` for transcript events and `ping` for keepalives:
  ```
  event: ping
  data: {"ts":"2026-06-22T10:04:20Z"}

  ```
- Keepalive every **10s** (matches Phase 2 §4.3).
- SSE `id:` = `Event.Seq`, enabling `Last-Event-ID` driven gap detection later.
- Backpressure: per-subscriber buffered channel (cap e.g. 256), **drop-oldest** on overflow (matches Phase 2). On the dropped event, set a `dropped` flag the client can detect via a seq gap.

---

## 8. Concurrency, edge cases & error handling

### 8.1 Goroutine model per agent

- 1 stdout reader (NDJSON → dispatch), 1 stderr reader (ring buffer), 1 serialized stdin writer (mutex or single-writer channel). Dispatch runs on the stdout goroutine and emits to the hub; the hub fans out to subscriber channels. JSON-RPC request/response correlation via a `map[int64]chan json.RawMessage` keyed by request id, guarded by a mutex.
- A single `sync.Mutex` (or per-handle lock) guards: pending-permission map, in-flight-turn flag, pending-rpc map, last-known context_pct. Keep critical sections tiny.

### 8.2 CLI crash mid-turn

- stdout EOF or `cmd.Wait()` returns → emit `error{scope:"process", message:<stderr tail>, fatal:true}` and a `turn_end{stop_reason:"error"}`, set the status row `state="error"`, delete the running row from `state.db`, mark the handle dead. Any held permission request is abandoned (its `POST .../permission` later → `409`). Subscribers see the error then the stream is closed.

### 8.3 Broken / malformed stdio

- A line that fails JSON decode → log the raw line (truncated), emit nothing, **continue scanning** (resync on the next line boundary). Do not kill the session for one bad frame. If the scanner itself errors (e.g. token > 8 MiB cap, pipe closed) → treat as §8.2 process failure.
- A JSON-RPC response to an unknown request id → log and ignore.

### 8.4 Cancel during a tool call / during a pending permission

- `Cancel` sends the ACP cancellation (`session/cancel` notification for the session). If a permission request is pending, the runtime first **resolves it as `cancelled`** (frees the agent), then sends cancel. The agent should emit `turn_end{stop_reason:"cancelled"}`; the runtime maps that to `idle`. If the agent does not honor cancel within a grace window (e.g. 5s), escalate to `SIGINT` to the process group; still not stopped → that's a `Stop` decision, not `Cancel` (Cancel never kills the process).

### 8.5 Orphaned process groups

- `Stop` sends `SIGTERM` to `-pgid`, waits up to `STOP_GRACE` (default 5s) on `cmd.Wait()`, then `SIGKILL` to `-pgid`. Always delete the running row from `state.db` even if the kill races (the pid may already be gone). On server shutdown (SIGINT/SIGTERM to the Go server), iterate all live handles and `Stop` them so no orphaned CLI groups survive the server.
- On server **start**, scan the running rows in `state.db` for stale entries (pid not alive) and reconcile: delete stale running rows and set their status rows to `error`/`done`. (Full resume is Phase 4; this is just cleanup so a crashed prior run doesn't leave ghost cards in Phase 2.)

### 8.6 Double-start / unknown agent

- `Start` for an agent_id already in the registry → `409` (caller-level guard in the API layer / registry). `SendPrompt`/`Cancel`/`Stop`/`Permission` for an agent with no live handle → `404` (no handle) or `409` (handle exists but wrong state), per §7.

---

## 9. Implementation task breakdown

Ordered; each step is small and independently testable. Steps 1–3 build the fake-CLI test harness *first* so the risky protocol code is TDD'd.

1. **Sentinels & error mapping.** `ErrNotImplemented` etc.; HTTP error helper (§7.7).
2. **Event types & envelope.** `Event`, `EventType` constants, the `*Data` payload structs (§4.2). Pure data + JSON round-trip tests.
3. **Fake ACP CLI** (`testdata/fakeacp`, a Go program). Reads JSON-RPC on stdin, emits scripted NDJSON sequences on stdout, driven by an env var naming a scenario file (§10.2). Implement before the real runtime so streaming/permission can be tested deterministically.
4. **JSON-RPC stdio transport.** Frame reader (`bufio.Scanner`, enlarged buffer), serialized writer, request/response correlation map, notification dispatch hook. Unit test against the fake CLI.
5. **ChatRuntime.Start.** Spawn (process group), handshake (`initialize` + `session/new` incl. the messaging MCP server registration), capture `sessionId`, insert running + initial status rows in `state.db`. Test: handle returned, rows written, pid is a group leader.
6. **ACP→normalized mapping + hub/Subscribe.** Implement `dispatch` for each `session/update` kind → emit `Event`s with seq; the in-process hub with drop-oldest. Test full streaming scenario via fake CLI.
7. **SendPrompt + turn lifecycle status writes.** Drive `session/prompt`, map result to `turn_end`, write status-row transitions in `state.db` (§4.4). Test idle→busy→idle + context_pct.
8. **Permission gating.** Hold the `session/request_permission` response; pending map; `Permission` relay; timeout auto-deny; `skip_permissions` auto-approve. Test approve runs tool, deny prevents it, timeout denies (§10.3).
9. **Cancel & Stop.** ACP cancel (+pending-permission resolution), process-group SIGTERM/SIGKILL, running-row deletion, shutdown reconciliation. Test orphan cleanup.
10. **Registry.** Dispatch by interface; stubs for terminal/codex/Resume/CheckMessages returning `ErrNotImplemented`.
11. **Launch flow + composition.** `composeEnv`, system-prompt join, hook-token mint, messaging-MCP-server registration, `LaunchSpec` builder, name auto-suggest, rollback on failure. Unit-test composition exhaustively.
12. **REST endpoints.** `POST /sessions`, `GET /sessions/{id}`, `prompt`, `cancel`, `stop`, `permission` (§7). Handler tests with the fake CLI.
13. **Interim SSE endpoint.** `GET /sessions/{id}/events`: subscribe, stream `Event`s with `id:`/`event:`, keepalive, status replay (§7.8). Test with an HTTP SSE client reading a scripted turn.
14. **CLI launch path.** Parse `role@project` + flags; POST to the REST endpoint (§6.5). Test parity with REST (acceptance).
15. **Wiring + manual verification.** A tiny static test page or documented `curl` + SSE recipe to drive a real `claude-code-acp` (§12.1). Confirm the acceptance checklist end-to-end against the real CLI.

---

## 10. Testing strategy

### 10.1 Layers

- **Unit:** event JSON round-trips; `composeEnv` layering; system-prompt join; hook-token mint/uniqueness; CLI arg parsing; status-transition table; decision→ACP-option mapping.
- **Transport:** JSON-RPC framing against a raw NDJSON fixture incl. a > 64 KiB frame (locks in §2 buffer enlargement) and a malformed line (locks in §8.3 resync).
- **Runtime integration (fake CLI):** the bulk of confidence. Deterministic, no real Claude.
- **Acceptance (real CLI), manual + gated:** one scripted run against `claude-code-acp` behind a build tag / env flag so CI without credentials still passes. Verifies the §12.1 wire assumptions against reality.

### 10.2 Fake ACP CLI

A standalone Go binary (`testdata/fakeacp/main.go`) that the test harness points `ChatRuntime` at via the CLI-path indirection (the binary path is injectable for tests). It:

- Responds to `initialize` and `session/new` (returns a fixed `sessionId`; accepts the `mcpServers` registration entry without needing to honor it).
- On `session/prompt`, replays a **scenario** (a list of NDJSON frames with optional inter-frame sleeps) named by `FAKEACP_SCENARIO`.
- Scenarios to ship:
  - `stream_text` — several `agent_message_chunk` updates then a prompt result `stopReason:"end_turn"`. Asserts incremental `assistant_text` (multiple events, not one) + `turn_end`.
  - `tool_flow` — `tool_call` → `tool_call_update(completed)` with a `diff` block. Asserts `tool_call` + `tool_result` + `diff` correlated by id.
  - `permission_approve` — emits `session/request_permission`, **blocks** until it receives the JSON-RPC result, then writes a sentinel file iff approved, then continues + `end_turn`. Test approves via `POST .../permission` and asserts the sentinel exists.
  - `permission_deny` — same but asserts the sentinel does **not** exist after deny.
  - `permission_timeout` — emits the request and never self-resolves; test waits past a shortened `PERMISSION_TIMEOUT` and asserts auto-deny (no sentinel) + an `error` event.
  - `big_frame` — a single `tool_call_update` whose content exceeds 64 KiB.
  - `crash_midturn` — emits one chunk then `os.Exit(1)`; asserts `error{fatal:true}` + status row `state="error"` + running row deleted.
  - `malformed_then_valid` — a bad line followed by a good frame; asserts resync.

The sentinel-file trick gives a side-effect that proves "the tool actually ran / did not run," which is exactly the F3 acceptance criterion for gating.

### 10.3 Permission gating tests (acceptance-critical, called out)

Drive the full HTTP path: `POST /sessions` (fake) → `POST .../prompt` → consume `/events` until `permission_request` → `POST .../permission` → assert (a) the status row's `state` was `waiting_input` while pending, (b) the side-effect sentinel matches the decision, (c) the turn resumes and ends. Repeat for approve / deny / timeout.

### 10.4 Concurrency / shutdown tests

- Cancel during `permission_pending` → request resolved as cancelled + `turn_end:"cancelled"`.
- Server-shutdown reconciliation: start (fake), kill the process out from under the runtime, assert `error` + running row removed.
- Stale running-row cleanup on server start.

---

## 11. Interfaces produced for later phases

These are the load-bearing contracts later phases depend on; freezing them now is a goal of this phase.

1. **Normalized transcript `Event`** (§3.1) + payload structs (§4.2). **Phase 2** streams these as `new_message` payloads on the multiplexed bus (the interim SSE `data:` object is byte-identical to what Phase 2 wraps). **Phase 4** persists this exact stream to `sessions/{id}/`. Stability requirements: `agent_id`, `seq`, `type`, `ts`, `data` are permanent fields; new `type`s may be added; existing payload fields are append-only.
2. **`Runtime` interface + `Registry`** (§3). **Phase 6** adds the real `TerminalRuntime` (same interface) and implements `Resume` (also reused by Phase 4) and `switch-runtime` (re-`Start` on the same `agent_id`). **Phase 5** implements `CheckMessages` and the messaging MCP tool handlers behind the registration this phase already passes in `session/new.mcpServers`. `LaunchSpec.MCPServers`/`ExtraArgs` are the seams for that.
3. **Status semantics** (§4.4): the status row's field set, the `state` enum (`busy|idle|waiting_input|done|error`), and the `last_trace` vocabulary (`SessionStart|UserPromptSubmit|PreToolUse:*|PostToolUse:*|PermissionRequest:*|Stop|Cancelled|Error`). **Phase 2** reads dashboard state from these rows; the vocabulary matching the §4.4 hook trace names means terminal (hook-driven, ingested via `POST /api/hook`) and chat (stream-driven) agents produce indistinguishable status rows.
4. **Per-launch hook token** (§6.4): minted at launch and recorded against the agent. **Phase 2** adds the `POST /api/hook` ingest endpoint that validates this token before applying a terminal agent's status update.
5. **REST surface** (§7): `POST /sessions`, `GET /sessions/{id}`, `prompt`, `cancel`, `stop`, `permission`. **Phase 3** adds the modal over `POST /sessions`; **Phase 2** chat panel consumes `prompt`/`cancel`/`permission`. Error shape (§7.7) is the project-wide convention.
6. **Stable-id invariant** wired (§6.1): `agent_id` minted once, `session_id` per Start/Resume — the basis for F7 switch-runtime and F9 resume.

---

## 12. Resolved decisions (master PRD §9 / phase §7 open questions)

### 12.1 Pin the ACP target & document assumed shapes

- **Target adapter:** `claude-code-acp` (Zed's ACP adapter for Claude Code), pinned to a **specific released version recorded in `install.sh`/lockfile** (e.g. `@zed-industries/claude-code-acp@<pinned>`). The Go code targets the ACP **protocol version** that adapter negotiates; the `initialize` handshake asserts the negotiated version is within `[MIN_ACP, MAX_ACP]` and fails Start clearly otherwise.
- **Assumed wire shapes** (the contract our mapping in §4.3 codes against; the real-CLI acceptance test in §10.1 verifies them, and any drift is fixed in one place — the mapping function):
  - Turn driven by `session/prompt` (client request) whose **result** carries `stopReason`.
  - Streaming via `session/update` notifications with `sessionUpdate` discriminator: `agent_message_chunk` (text), `tool_call`, `tool_call_update`, `agent_thought_chunk`, `plan`.
  - `tool_call`/`tool_call_update` carry `toolCallId`, `title`, `kind`, `status`, and `content[]` where a content item may be a `diff` (with `path`, `oldText`, `newText`).
  - Permission via `session/request_permission` **request** (server→client) with `toolCall` + `options[]` (`optionId`, `name`, `kind ∈ {allow_once, allow_always, reject_once, reject_always}`); client replies with `{outcome:{outcome:"selected", optionId}}` or `{outcome:{outcome:"cancelled"}}`.
  - Cancellation via `session/cancel` notification.
  - MCP servers passed in `session/new.mcpServers` (`name`, `command`, `args`, `env`) as standard stdio MCP server entries.
  - **Context usage:** if not present in `session/update`/result, `context_pct` stays `0` (documented limitation; refined when a usage field is confirmed).
- **Isolation rule:** all ACP-specific decoding lives in one `acpmap.go`; the rest of the system sees only normalized `Event`s. This is what lets Codex be added later and the version pin be bumped with a localized blast radius.

### 12.2 Permission granularity

- Phase 1 honors **`skip_permissions`** (effective value) **and per-call prompting**. Nothing finer.
- **Resolution order** for effective skip: `role.skip_permissions` if non-null, else `config.skip_permissions` (global). (Per master PRD §3.2: role `null` = inherit global.) Computed in the launch flow into `LaunchSpec.SkipPerms`.
- `true` → auto-approve every request (§5.2), no `waiting_input`. `false` → every request gates via `POST .../permission`. Per-role / per-tool policy is deferred to Phase 3.

### 12.3 Concurrency / queueing

- **Single agent assumption for the loop's correctness, no launch queue.** Multiple agents *can* be started (the registry holds many handles), but this phase does not impose or test resource limits or queue launches.
- **One in-flight turn per agent:** `SendPrompt` while a turn is running → `409` (no per-agent prompt queue this phase). The client serializes prompts. Per-agent turn queueing and global concurrency limits are explicitly deferred.

### 12.4 Other §9 items (noted, out of scope here)

Message-loop budgets (Phase 5), cross-platform terminal fallback (Phase 6) are out of scope and are not constrained by Phase 1 decisions.

---

## Appendix A — Acceptance checklist mapping

| PRD acceptance (phase §6) | Covered by |
|---------------------------|-----------|
| CLI and REST produce identical agent | §6.5 (CLI→same REST), §10 task 14 |
| Prompt streams incrementally | §4.3 `assistant_text` deltas, §10.2 `stream_text` |
| Permission gates execution; deny prevents tool | §5, §10.3 (sentinel) |
| Tool calls/results/diffs with args/patches in stream | §4.2/§4.3, §10.2 `tool_flow` |
| Cancel interrupts; Stop kills group + removes running row | §8.4, §8.5, §10.4 |
| Status row idle→busy→idle incl. `context_pct` | §4.4, §10.2 step 7 |
