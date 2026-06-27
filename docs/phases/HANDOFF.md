# AgentDeck — Implementation Handoff

**Live state. Read this first, every session. Update it after every change.**
Protocol: [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) (Claude Code or Codex, whichever the human runs).
Keep this lean — apply the condensation rules (workflow §5); old detail lives in git, not here.

---

## Current position

- **Active phase:** 1 — Core loop (ACP chat runtime, launch, streaming chat) · F4, F3(min)
- **Active subphase:** 1.4 — Permission gating (withhold response + timeout + skip_permissions) and Cancel
- **Spec:** [`tech/phase-1-core-loop-techspec.md`](tech/phase-1-core-loop-techspec.md) → `## Subphase plan`
- **Last GREEN checkpoint:** `go build ./...` + `go test ./...` (and `-race`) pass @ `impl/phase-1` (1.3 complete)
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
- [x] **1.3** `ChatRuntime.Start` + ACP→normalized mapping + hub/Subscribe (stream a turn end-to-end) ✅
- [ ] **1.4** Permission gating (withhold response + timeout + skip_permissions) and Cancel ← **active**
- [ ] **1.5** Launch flow, composition, REST + interim SSE, CLI parity
- [ ] **1.6** Real-CLI acceptance (credential-gated) + manual verification

**Active step (1.4):** wire `onRequest` in `ChatRuntime.Start` (currently `nil`) to handle
`session/request_permission`: emit `permission_request` (status `waiting_input`), keep the
`IncomingRequest` in a pending map keyed by `toolCallID`, and **withhold** its response (the pause).
`Permission(agentID, toolCallID, decision)` maps approve→`allow_once`/deny→`reject_once` (§5.3) and
calls `req.Respond({outcome:{outcome:"selected",optionId}})`. Add 180s timeout auto-deny
(`PERMISSION_TIMEOUT` env, shortenable in tests) and the `skip_permissions` auto-approve path
(`AutoApproved:true`, no `waiting_input`, §5.2). Implement `Cancel` (ACP `session/cancel` notify;
resolve any pending permission as cancelled first, §8.4) and crash handling in `onTransportClosed`
(`error{fatal:true}` + delete running row + status `error`, §8.2). Add fakeacp scenarios
`permission_approve`/`permission_deny`/`permission_timeout`/`crash_midturn` (sentinel-file trick §10.2).
Done-when: `go test ./internal/runtime` green for those + cancel-during-pending-permission.
NOTE: `Stop` (process-group SIGTERM→SIGKILL, delete running row, status `done`) was already
implemented in 1.3 for test teardown — 1.4 only adds start-time/shutdown stale-row reconciliation.

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
- 1.3 added: `hub.go` (per-agent fan-out, drop-oldest, cap 256), `acpmap.go` (ALL ACP decoding —
  `mapSessionUpdate`/`mapPromptResult`), `ringbuffer.go` (stderr tail), real `chat.go` (`ChatRuntime`
  with `agentState` per agent: process-group spawn, handshake, hub, async `SendPrompt` turn, §4.4 status
  writes, working `Stop`). `command` field is injectable (tests point it at fakeacp). Permission
  `onRequest` is still `nil` (1.4). `c.command` defaults to `claude-code-acp`.

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
- **2026-06-27 — `Stop` implemented in 1.3 instead of 1.4.** The spec slots `Stop` under 1.4, but the 1.3
  fake-CLI tests need to tear down the spawned process. Implemented the full §8.5 Stop (SIGTERM→grace→
  SIGKILL to `-pgid`, delete running row, status→`done`) in 1.3. 1.4 now only adds stale-row
  reconciliation + Cancel + permission. **To reverse:** none needed; it matches the §8.5 spec exactly.
- **2026-06-27 — Tool `Name` derived from ACP `kind` (fallback `title`).** The §4.3 mapping table maps
  `tool_call`→`tool_call` but doesn't pin which ACP field becomes the normalized `Name`. Chose ACP
  `kind` (e.g. `edit`) as the most stable discriminator, falling back to `title` then `"tool"`. Isolated
  in `acpmap.go::toolName`. **To reverse:** if the real adapter (1.6) surfaces a cleaner tool name field,
  change only `toolName` — blast radius is one function (§12.1 isolation rule).

## Changelog

_(most recent first; keep ~10, older history is in git)_

- 2026-06-27 — **1.3 green.** Real `ChatRuntime`: process-group spawn + ACP handshake, isolated
  `acpmap.go`, per-agent `Hub` (drop-oldest), async `SendPrompt` streaming a turn end-to-end, §4.4 status
  writes, working `Stop`. Tests (incl. `-race`): `stream_text` (multi-delta + monotonic seq + context_pct),
  `tool_flow` (correlated call/result/diff), idle→busy→idle. fakeacp gained `tool_flow` + usage in result.
- 2026-06-27 — **1.2 green.** Added JSON-RPC stdio transport (8 MiB scanner, serialized writer, Call/Notify,
  correlation map, IncomingRequest withhold/Respond) + standalone fakeacp CLI (stream_text/big_frame/
  malformed_then_valid). Tests: >64 KiB frame, malformed-then-valid resync, Call/response, incoming-request reply.
- 2026-06-27 — **1.1 green.** Created `internal/runtime`: sentinel + APIError/code vocab, Event envelope +
  payload structs, Runtime interface, Registry dispatch + terminal/ChatRuntime stubs. Tests: payload JSON
  round-trips, code→status map, dispatch table. `go build ./...` + `go test ./...` green.
- 2026-06-27 — Handoff + workflow created. Phase 0 confirmed complete (build + tests green). Phase 1 ready to start at 1.1.
