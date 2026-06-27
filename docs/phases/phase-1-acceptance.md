# Phase 1 — Manual acceptance recipe (real `claude-code-acp`)

This is the credential-gated, human-run verification for Phase 1 (techspec §10.1, Appendix A).
The automated suite (`go test ./...`) proves everything against the **fake** ACP CLI and stays
green without credentials. This recipe drives the **real** adapter end-to-end.

## Prerequisites

1. **Install the pinned adapter** (Node 18+ required):
   ```bash
   INSTALL_ACP=1 ./install.sh
   # or directly:
   npm install -g @zed-industries/claude-code-acp@<pinned>   # see CLAUDE_ACP_VERSION in install.sh
   ```
2. **Log in** so the adapter has Claude credentials (per the adapter's own auth flow).
3. Confirm it is on PATH: `which claude-code-acp`.

## Option A — the gated Go acceptance test

```bash
# Uses the real adapter; skips automatically if claude-code-acp is not on PATH.
go test -tags acceptance ./internal/runtime -run TestRealCLIAcceptance -v
# Override the binary if needed:
ACP_CMD=/path/to/claude-code-acp go test -tags acceptance ./internal/runtime -run TestRealCLIAcceptance -v
```

Asserts: handshake succeeds, the prompt streams **incremental** `assistant_text`, a `turn_end`
arrives, and the status row returns to `idle`. If the adapter gates a tool, the test approves it.

## Option B — curl + SSE by hand

Start the dashboard, then drive the REST surface. The agent must target a **project whose `cwd`
exists** (the adapter is spawned there).

```bash
# 1. Start the server (binds 127.0.0.1:4317 by default).
agentdeck dashboard start --detach

# 2. Launch an agent (CLI form — identical to the modal's POST /api/sessions).
agentdeck implementer@my-app
#   → launched Atlas (a_XXXXXX) implementer@my-app
AGENT=a_XXXXXX        # copy the printed agent_id
PORT=4317

# 3. In a second terminal, attach to the interim event stream BEFORE prompting.
curl -N "http://127.0.0.1:${PORT}/api/sessions/${AGENT}/events"
#   You should immediately see an `event: state_update` replay, then live frames:
#   event: message
#   data: {"agent_id":"a_XXXXXX","seq":1,"type":"assistant_text","ts":"...","data":{"delta":"..."}}

# 4. Back in the first terminal, send a prompt.
curl -s -XPOST "http://127.0.0.1:${PORT}/api/sessions/${AGENT}/prompt" \
  -H 'Content-Type: application/json' -d '{"text":"Reply with exactly one word: pong"}'
#   → {"accepted":true,"agent_id":"a_XXXXXX"}
#   Watch the SSE terminal stream assistant_text deltas then a turn_end.

# 5. If a tool needs permission, the stream shows:
#   event: message
#   data: {... "type":"permission_request", "data":{"tool_call_id":"tc_..","name":".."}}
#   Approve (or deny) it:
curl -s -XPOST "http://127.0.0.1:${PORT}/api/sessions/${AGENT}/permission" \
  -H 'Content-Type: application/json' -d '{"tool_call_id":"tc_..","decision":"approve"}'

# 6. Cancel an in-flight turn (interrupts; does NOT kill the process):
curl -s -XPOST "http://127.0.0.1:${PORT}/api/sessions/${AGENT}/cancel"

# 7. Stop the agent (terminates the process group, removes the running row):
curl -s -XPOST "http://127.0.0.1:${PORT}/api/sessions/${AGENT}/stop"

# 8. Inspect final state:
curl -s "http://127.0.0.1:${PORT}/api/sessions/${AGENT}" | jq .
```

## Appendix A checklist (techspec)

| Acceptance | How to confirm here |
|------------|---------------------|
| CLI and REST produce identical agent | step 2 (`agentdeck role@project` POSTs the same body) |
| Prompt streams incrementally | step 4 — multiple `assistant_text` deltas, not one block |
| Permission gates execution; deny prevents tool | step 5 — deny, then confirm the tool's side effect did not happen |
| Tool calls/results/diffs with args/patches in stream | `tool_call` / `tool_result` / `diff` frames during a tool turn |
| Cancel interrupts; Stop kills group + removes running row | steps 6–7, then step 8 shows `running` absent |
| Status row idle→busy→idle incl. `context_pct` | `GET /api/sessions/{id}` `status.state` + `status.context_pct` |

## If the real wire shapes differ from §12.1

All ACP decoding is isolated in [`internal/runtime/acpmap.go`](../../internal/runtime/acpmap.go).
Fix any drift (field names, content-block shapes, usage location) **there only** — the rest of the
system consumes normalized `Event`s and needs no changes (§12.1 isolation rule).
