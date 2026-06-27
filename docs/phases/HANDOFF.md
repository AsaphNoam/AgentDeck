# AgentDeck — Implementation Handoff

**Live state. Read this first, every session. Update it after every change.**
Protocol: [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) (Claude Code or Codex, whichever the human runs).
Keep this lean — apply the condensation rules (workflow §5); old detail lives in git, not here.

---

## Current position

- **Active phase:** 1 — Core loop (ACP chat runtime, launch, streaming chat) · F4, F3(min)
- **Active subphase:** 1.3 — `ChatRuntime.Start` + ACP→normalized mapping + hub/Subscribe (stream a turn)
- **Spec:** [`tech/phase-1-core-loop-techspec.md`](tech/phase-1-core-loop-techspec.md) → `## Subphase plan`
- **Last GREEN checkpoint:** `go build ./...` + `go test ./...` pass @ `impl/phase-1` (1.2 complete)
- **Branch:** `impl/phase-1` (do not commit to `main`)

---

## Phase status

- [x] Phase 0 — Foundation (data model, file store, server & CLI skeleton) ✅
- [ ] Phase 1 — Core loop (ACP chat runtime, launch, streaming chat) — **not started**, start at 1.1
- [ ] Phase 2 — State manager, SSE bus, dashboard card grid
- [ ] Phase 3 — Config CRUD & onboarding
- [ ] Phase 4 — Persistence: archive, search, resume, file/command tracking
- [ ] Phase 5 — Coordination: MCP messaging, nudger, budgets, notifications
- [ ] Phase 6 — Flexibility: terminal runtime, switch-runtime, task groups
- [ ] Phase 7 — Polish: activity map

Build order: `0 → 1 → 2 → {3, 4, 5} → 6 → 7` (3/4/5 are independent after 2).

---

## Active subphase detail

> The ONLY place granular steps live. When this subphase is fully green, collapse it
> (mark `1.1 ✅` on the phase line above) and expand the next subphase here.

### Phase 1 subphases (from the tech spec — tick as each goes green)
- [x] **1.1** Foundations: sentinels, event types, `Runtime` interface + Registry skeleton ✅
- [x] **1.2** Fake ACP CLI + JSON-RPC stdio transport (deterministic test harness) ✅
- [ ] **1.3** `ChatRuntime.Start` + ACP→normalized mapping + hub/Subscribe (stream a turn end-to-end) ← **active**
- [ ] **1.4** Permission gating (withhold response + timeout + skip_permissions) and Cancel/Stop
- [ ] **1.5** Launch flow, composition, REST + interim SSE, CLI parity
- [ ] **1.6** Real-CLI acceptance (credential-gated) + manual verification

**Active step (1.3):** implement `ChatRuntime.Start` (process-group spawn `Setpgid`, `initialize` +
`session/new` handshake, capture `sessionId`, insert running+status rows), `acpmap.go` (ACP
`session/update` → normalized `Event` with per-agent monotonic `Seq`, ALL ACP decoding isolated here),
the in-process `Hub`+`Subscribe` (bounded buffered chans, drop-oldest), and `SendPrompt` driving
`session/prompt`→`turn_end` + §4.4 status transitions. Read §4.1, §4.3, §4.4, §2.1. Add a `tool_flow`
scenario to fakeacp. Done-when: `go test ./internal/runtime` green: `stream_text` yields multiple
`assistant_text` then `turn_end`; `tool_flow` yields correlated `tool_call`+`tool_result`+`diff`;
status row `idle→busy→idle` with `context_pct` written. Leave `session/request_permission` unhandled (1.4).

> ⚠️ **1.6 is a known STOP point** — it needs real `claude-code-acp` credentials. When you reach it,
> if you don't have a logged-in CLI, record it under "Blocked on human" and stop rather than fake it.

---

## Decisions & notes

- Phase 0 substrate is in place: `internal/{config,state,store,server,cli,version}`, seed data, `127.0.0.1` bind, GET-only routes.
- Launch (`role@project`) is currently a stub: [`internal/cli/launch_stub.go`](../../internal/cli/launch_stub.go) — Phase 1 replaces it with the real runtime.
- `internal/runtime` created in 1.1: `errors.go` (sentinel + APIError/code vocab §7.7), `event.go`
  (Event envelope + `*Data` payloads), `runtime.go` (Runtime iface, LaunchSpec, MCPServerSpec, Handle),
  `registry.go` (byIface dispatch + terminal stub), `chat.go` (ChatRuntime stub). All methods return
  `ErrNotImplemented` until later subphases.
- 1.2 added: `jsonrpc.go` (rpcMessage union + `kind()` classifier), `transport.go` (`Transport`:
  8 MiB scanner, serialized writer, `Call`/`Notify`, request/response correlation map, `IncomingRequest`
  with withhold-then-`Respond` for permission gating), `testdata/fakeacp/main.go` (standalone fake ACP
  CLI: scenarios `stream_text`, `big_frame`, `malformed_then_valid`). fakeacp is under `testdata/` so
  `go build ./...` skips it — build explicitly: `go build -o /dev/null ./internal/runtime/testdata/fakeacp`.

## Blocked on human

_(empty — nothing blocking. Add items here per workflow §3, then stop.)_

## Autonomous decisions (please review)

> Calls an agent had to make itself because a directive was ambiguous or the spec had a gap —
> resolved without stopping, but the human should see them. Each entry: what was unclear, what was
> chosen and why, how to reverse it. Agents also surface these in their end-of-turn summary. Remove
> an entry once the human has acknowledged it (workflow §3, §5).

- **2026-06-27 — Tech spec says `internal/store` (`store.Store`/`store.Agent`); Phase 0 actually built
  `internal/state` (`state.Store`/`state.Agent`).** The runtime package imports `internal/state`
  throughout (`LaunchSpec.Agent state.Agent`, `Registry.store *state.Store`). The spec's `store` is just
  the older name for the same Phase-0 package; no behavior change. **To reverse:** rename only if Phase 0
  is ever split into separate config-`store` and state packages — then update the runtime imports.

## Changelog

_(most recent first; keep ~10, older history is in git)_

- 2026-06-27 — **1.2 green.** Added JSON-RPC stdio transport (8 MiB scanner, serialized writer, Call/Notify,
  correlation map, IncomingRequest withhold/Respond) + standalone fakeacp CLI (stream_text/big_frame/
  malformed_then_valid). Tests: >64 KiB frame, malformed-then-valid resync, Call/response, incoming-request reply.
- 2026-06-27 — **1.1 green.** Created `internal/runtime`: sentinel + APIError/code vocab, Event envelope +
  payload structs, Runtime interface, Registry dispatch + terminal/ChatRuntime stubs. Tests: payload JSON
  round-trips, code→status map, dispatch table. `go build ./...` + `go test ./...` green.
- 2026-06-27 — Handoff + workflow created. Phase 0 confirmed complete (build + tests green). Phase 1 ready to start at 1.1.
