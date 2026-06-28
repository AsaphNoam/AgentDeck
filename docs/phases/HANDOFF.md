# AgentDeck — Implementation Handoff

**Live state. Read this first, every session. Update it after every change.**
Protocol: [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) (Claude Code or Codex, whichever the human runs).
Keep this lean — apply the condensation rules (workflow §5); old detail lives in git, not here.

---

## Current position

- **Active phase:** 2 — State manager, SSE bus, dashboard card grid — **not started**
- **Active subphase:** start at Phase 2's first subphase (read `tech/phase-2-*-techspec.md` → `## Subphase plan`)
- **Spec:** Phase 2 techspec (TBD path under `tech/`)
- **Last GREEN checkpoint:** `go build ./...` + `go test ./...` (and `-race`) pass @ `impl/phase-1`; real-CLI
  acceptance PASSED against `claude-code-acp` v0.16.2 (Phase 1 complete)
- **Branch:** `impl/phase-1` (Phase 1 work; do not commit to `main`). Start Phase 2 on a new branch.

---

## Phase status

- [x] Phase 0 — Foundation (data model, file store, server & CLI skeleton) ✅
- [x] Phase 1 — Core loop (ACP chat runtime, launch, streaming chat) ✅ — verified against real `claude-code-acp` v0.16.2
- [ ] Phase 2 — State manager, SSE bus, dashboard card grid — **next**, start here
- [ ] Phase 3 — Config CRUD & onboarding
- [ ] Phase 4 — Persistence: archive, search, resume, file/command tracking
- [ ] Phase 5 — Coordination: MCP messaging, nudger, budgets, notifications
- [ ] Phase 6 — Flexibility: terminal runtime, switch-runtime, task groups
- [ ] Phase 7 — Polish: activity map

Build order: `0 → 1 → 2 → {3, 4, 5} → 6 → 7` (3/4/5 are independent after 2).

---

## Active subphase detail

> The ONLY place granular steps live. Phase 1 is complete and collapsed (workflow §5).
> **Phase 2 has not started** — read its techspec's `## Subphase plan`, expand the first
> subphase's steps here, branch off `main` (or continue a fresh `impl/phase-2`), and begin.

_(Phase 2 subphases go here once its techspec is opened.)_

---

## Decisions & notes (durable contracts from Phase 1)

- **Normalized `Event` is the cross-phase contract.** `internal/runtime`: `event.go` (envelope +
  `*Data` payloads), `acpmap.go` (the ONLY place ACP wire shapes are decoded — §12.1 isolation rule).
  Phase 2 streams these `Event`s as `new_message` payloads; the interim SSE `data:` object is already
  byte-identical to what Phase 2 wraps. Permanent fields: `agent_id,seq,type,ts,data` (append-only).
- **`Registry` is the server's entry to runtimes** (`Launch`/`SendPrompt`/`Cancel`/`Stop`/`Permission`/
  `Subscribe`/`Shutdown`; dispatch by `agent.interface`; `Chat()` + `ChatRuntime.SetCommand` inject the
  adapter binary). `chat.go` owns `agentState` per agent (process group, transport, hub, status writes);
  `permission.go` is the withhold-the-response gate; `reconcile.go::ReconcileStale` cleans stale rows on start.
- **Status vocabulary (§4.4)** is the dashboard contract Phase 2 reads: `state ∈
  {busy,idle,waiting_input,done,error}`, `last_trace ∈ {SessionStart,UserPromptSubmit,PreToolUse:*,
  PostToolUse:*,PermissionRequest:*,PermissionResolved,Stop,Cancelled,Error}`.
- **REST surface (server pkg):** `POST /api/sessions` (launch), `GET /api/sessions/{id}`,
  `POST .../{prompt,cancel,stop,permission}`, `GET .../events` (interim SSE). Session routes use the §7.7
  nested error envelope via `writeAPIError`. `server.New` takes a `*runtime.Registry`. CLI launch
  (`internal/cli/launch.go`) just POSTs to `/api/sessions` (CLI≡modal parity).
- **fakeacp** (`internal/runtime/testdata/fakeacp`) is the deterministic test adapter — under `testdata/`
  so `go build ./...` skips it; build explicitly with `go build -o /dev/null ./internal/runtime/testdata/fakeacp`.
- The **real-CLI acceptance** is gated behind `//go:build acceptance` (5 tests: stream, permission
  deny/approve, cancel, stop); run with `go test -tags acceptance ./internal/runtime -run TestRealCLI -v`
  (needs `claude-code-acp` + a logged-in Claude account). Recipe + Appendix A: [`phase-1-acceptance.md`](phase-1-acceptance.md).

## Blocked on human

_(empty — the 1.6 credentialed acceptance ran GREEN against `claude-code-acp` v0.16.2. Nothing blocking.)_

## Review findings (BLOCKING items from the last review)

> Written by the review agent (workflow §8). Remove an entry once fixed and verified green.

- ✅ **RESOLVED — crash teardown left registry ownership stale (Phase 0/1, BLOCKING).** On an ACP crash,
  `ChatRuntime.onTransportClosed` removed its handle but `Registry.rtByAgent` kept claiming the agent, so a
  relaunch/resume was rejected with `ErrAlreadyStarted` (and control ops hit `ErrNoHandle`) until a manual
  `Stop`. Fixed: `ChatRuntime` now carries an `onExit` callback, wired by `NewRegistry` to `Registry.forget`,
  invoked from `onTransportClosed` before the `turn_end` emit. New `TestRegistryForgetsAgentAfterCrash`
  (`registry_crash_test.go`) drives a crash through the registry and asserts ownership is dropped + relaunch
  is no longer blocked. Green incl. `-race`.
- ✅ **RESOLVED — idle cancel reported a no-op as success (Phase 1, advisory).** `Runtime.Cancel` now returns
  `(cancelled bool, error)`; `ChatRuntime.Cancel` reports `true` only when a turn or pending permission was
  actually interrupted, and `POST /api/sessions/{id}/cancel` returns `{cancelled:false}` for an idle no-op.
  `TestCancelDuringPendingPermission` extended to assert the idle no-op case.
- 📝 **RECORDED — future-phase Codex findings (Phases 2/3/4).** The remaining review items target code not
  yet written, so they are recorded as a new **`## 0. Codex review findings`** section at the top of each
  affected techspec, to be resolved when that phase is built: Phase 2 — `new_message` double-nesting (BLOCKING)
  + snapshot/subscribe race + missing optimistic-user-bubble renderer; Phase 3 — 409 detail propagation +
  default role/project preselection; Phase 4 — indexer FTS wipe-on-restart (BLOCKING) + missing
  `sqlite_fts5` build tag in `Makefile`/`install.sh` (BLOCKING, cross-project) + fresh MCP registration on
  resume.
- ✅ **RESOLVED — full real-adapter Appendix A coverage added & PASSED.** The gated acceptance suite
  (`internal/runtime/acceptance_test.go`, `//go:build acceptance`) now has five real-CLI tests, all green
  against `claude-code-acp` v0.16.2 (`go test -tags acceptance ./internal/runtime -run TestRealCLI -v`):
  `TestRealCLIAcceptance` (incremental stream + turn_end + idle), `TestRealCLIPermissionDeny` (real gate;
  denied tool's side effect never happens), `TestRealCLIPermissionApprove` (approved tool runs),
  `TestRealCLICancel` (cancel interrupts an in-flight turn → idle), `TestRealCLIStop` (terminates the
  process group + removes the running row + status `done`). Confirmed real option kinds are
  `allow_once`/`reject_once`/`allow_always` — `selectOption` (§5.3) maps approve/deny correctly.

## Autonomous decisions (please review)

> Resolved without stopping; the human should still see them. Remove once acknowledged (workflow §3, §5).

- **`internal/store` (spec) → `internal/state` (Phase 0 reality).** The runtime imports `internal/state`
  throughout; the spec's `store` is the older name for the same package. No behavior change.
- **`Stop` implemented in 1.3** (spec slots it in 1.4) for test teardown — matches §8.5 exactly; no reversal needed.
- **Tool `Name` ← ACP `kind`** (fallback `title`, then `"tool"`); §4.3 didn't pin the field. Isolated in
  `acpmap.go::toolName`. Verified against the real adapter (turn streamed cleanly).
- **Hook token stored in-memory** (`Server.hookTokens`) — no `state.db` column yet. **Phase 2 should
  persist it** (it owns `POST /api/hook` ingest) and drop the in-memory map.
- **Two error-envelope shapes coexist** — new session routes use the §7.7 nested shape; Phase-0 GET routes
  keep flat `{"error":"msg"}` (not migrated, to avoid breaking Phase-0 tests). Migrate later if §7.7 is meant
  to be truly project-wide.
- **`messagingServer.Command = os.Executable()`** with `["mcp-stdio","--agent",ID,"--token",T]` —
  registration-only; the `mcp-stdio` subcommand lands in Phase 5.
- **NEW: runtime strips `CLAUDECODE` from the spawned adapter's env** (`chat.go::stripEnv`). The real
  `claude-code-acp` refuses a "nested" session when `CLAUDECODE` is set (true when AgentDeck is launched
  from a Claude Code terminal). AgentDeck spawns independent agents, so the nested guard must never apply.
  Discovered during the 1.6 run. **To reverse:** drop the strip if it ever causes surprise; production
  (standalone server) is unaffected since `CLAUDECODE` isn't set there.
- **RESOLVED: `CLAUDE_ACP_VERSION` pinned to `0.16.2`** (was an unverified `0.4.1` placeholder; corrected
  via `npm view` to the real latest-stable, against which acceptance passed).
- **Wire-shape note (no fix needed):** the real adapter's `session/new` ignores our `model` param and
  exposes its own modelIds (`default`/`sonnet`/`haiku`/`opus`) + permission `modes`
  (incl. `bypassPermissions`/`acceptEdits`). Phase 1 doesn't assert the model, so this is fine; a future
  phase wanting real model/mode selection should map our model→adapter modelId in `acpmap.go`/`sessionNewParams`.

## Changelog

_(most recent first; keep ~10, older history is in git)_

- 2026-06-28 — **Codex review pass (branch `claude/codex-issue-review-jhrf6m`).** Fixed two implemented-code
  issues with tests (build + `go test ./...` + `-race` green): crash-teardown registry-ownership leak
  (BLOCKING — `chat.go`/`registry.go`, new `registry_crash_test.go`) and idle-cancel no-op reporting
  (`Runtime.Cancel`→`(bool,error)`, `sessions.go`). Recorded the remaining future-phase findings into
  `## 0` sections of the Phase 2/3/4 techspecs (see Review findings above).
- 2026-06-28 — **Review fix: full Appendix A real-adapter coverage.** Added 4 gated tests
  (permission deny/approve, cancel, stop) alongside the stream test — all 5 PASS against
  `claude-code-acp` v0.16.2. Real option kinds confirmed (`allow_once`/`reject_once`/`allow_always`).
  Resolves the BLOCKING review finding. Default suite untouched (tests tagged off).
- 2026-06-28 — **Phase 1 COMPLETE.** Real-CLI acceptance PASSED against `claude-code-acp` v0.16.2:
  handshake + incremental stream + turn_end + idle. Fixed: runtime strips `CLAUDECODE` from the spawned
  adapter env (adapter refuses nested sessions); `install.sh` pin corrected `0.4.1`→`0.16.2`.
- 2026-06-27 — **1.6 code/docs.** Gated `acceptance` build-tag test + `install.sh` adapter pin +
  `phase-1-acceptance.md` curl/SSE recipe.
- 2026-06-27 — **1.5 green** (incl. `-race`). Launch composition + REST (`POST /api/sessions`, detail,
  prompt/cancel/stop/permission) + interim SSE + CLI launch. Tests: composeEnv/joinSystemPrompt/resolveSkip
  units, CLI parseLaunch + parity, full HTTP integration (launch→SSE→prompt→permission_request→approve→
  sentinel→turn_end), §7.7 validation/404 envelopes. Replaced the Phase-0 CLI launch stub.
- 2026-06-27 — **1.4 green** (incl. `-race`). Permission gating (withhold/approve/deny/timeout/skip),
  Cancel, crash handling, stale-row reconcile. Tests: approve→sentinel, deny→no sentinel, timeout auto-deny,
  skip auto-approve, unknown-tool 409, cancel-during-pending, crash (fatal err + running row deleted), reconcile.
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
