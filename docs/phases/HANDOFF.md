# AgentDeck — Implementation Handoff

**Live state. Read this first, every session. Update it after every change.**
Protocol: [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) (Claude Code or Codex, whichever the human runs).
Keep this lean — apply the condensation rules (workflow §5); old detail lives in git, not here.

---

## Current position

- **Active phase:** 1 — Core loop (ACP chat runtime, launch, streaming chat) · F4, F3(min)
- **Active subphase:** 1.6 — Real-CLI acceptance (credential-gated) + manual verification
- **Spec:** [`tech/phase-1-core-loop-techspec.md`](tech/phase-1-core-loop-techspec.md) → `## Subphase plan`
- **Last GREEN checkpoint:** `go build ./...` + `go test ./...` (and `-race`) pass @ `impl/phase-1` (1.5 complete)
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
- [x] **1.4** Permission gating (withhold response + timeout + skip_permissions) and Cancel ✅
- [x] **1.5** Launch flow, composition, REST + interim SSE, CLI parity ✅
- [~] **1.6** Real-CLI acceptance — **code/docs done; credentialed run BLOCKED on human** (see below)

**1.6 status:** the credential-independent deliverables are complete and committed:
- Gated acceptance test [`internal/runtime/acceptance_test.go`](../../internal/runtime/acceptance_test.go)
  behind `//go:build acceptance` — excluded from default `go test ./...` (CI stays green); run with
  `go test -tags acceptance ./internal/runtime -run TestRealCLIAcceptance -v` (skips if adapter absent).
- Adapter pin in [`install.sh`](../../install.sh) (`CLAUDE_ACP_PKG`/`CLAUDE_ACP_VERSION`, optional
  `INSTALL_ACP=1` install step).
- curl+SSE recipe + Appendix A checklist: [`phase-1-acceptance.md`](phase-1-acceptance.md).

The **actual run against the real adapter** can't be done in this environment — see "Blocked on human".
Once a human runs it green, mark `1.6 ✅`, collapse Phase 1 to one line in "Phase status", and delete
this subphase breakdown (workflow §5).

> ⚠️ **1.6 is a known STOP point** — it needs real `claude-code-acp` credentials. When you reach it,
> if you don't have a logged-in CLI, record it under "Blocked on human" and stop rather than fake it.

---

## Decisions & notes

- Phase 0 substrate is in place: `internal/{config,state,store,server,cli,version}`, seed data, `127.0.0.1` bind, GET-only routes.
- Launch (`role@project`) is real as of 1.5: [`internal/cli/launch.go`](../../internal/cli/launch.go) POSTs to
  `/api/sessions`. `launch_stub.go` now only holds `isLaunchArg`.
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
  writes, working `Stop`). `command` field is injectable (tests point it at fakeacp). `c.command`
  defaults to `claude-code-acp`.
- 1.4 added: `permission.go` (`onRequest` withhold-the-response gate, `Permission` relay, 180s
  `permissionTimeout` auto-deny, `skip_permissions` auto-approve, `Cancel` via `session/cancel` notify +
  pending resolution, `StopAll`), crash handling in `onTransportClosed` (`error{fatal}` + delete running +
  status `error`, §8.2), `reconcile.go` (`ReconcileStale` for stale running rows on startup). fakeacp
  rewritten with concurrent request/response routing + sentinel-file trick; scenarios `permission*` and
  `crash_midturn`. The full `Runtime` interface is now real for the claude-acp chat path.
- 1.5 added: `Registry` exported surface (`Launch`/`SendPrompt`/`Cancel`/`Stop`/`Permission`/`Subscribe`/
  `Shutdown`, dispatch by interface + double-start guard + `rtByAgent` routing; `Chat()` accessor;
  `ChatRuntime.SetCommand` to inject the adapter/fake binary). Server: `launch.go` (composition: `composeEnv`,
  `joinSystemPrompt`, `resolveSkip`, `suggestName` wordlist, `mintHookToken`, `messagingServer`,
  `LaunchSpec` builder + rollback), `sessions.go` (prompt/cancel/stop/permission), `sse.go` (interim
  events stream), `apierror.go` (§7.7 nested envelope). Routes: `POST /api/sessions`,
  `GET/POST /api/sessions/{id}[/prompt|cancel|stop|permission|events]`. `server.New` now takes a
  `*runtime.Registry`. `statusRecorder` got a `Flush()` passthrough (SSE needs `http.Flusher`); CORS
  allows POST. CLI: `launch.go` (`parseLaunch` + POST to `/api/sessions`) replaced the Phase-0 stub.
  Dashboard start wires `ReconcileStale` + the registry; shutdown calls `registry.Shutdown` (StopAll).

## Blocked on human

- **1.6 credentialed acceptance run.** `claude-code-acp` is **not installed** on this machine and there
  are **no Claude credentials** for it, so the real-adapter acceptance (techspec §10.1, Appendix A) cannot
  be executed here. Everything else in Phase 1 is built, tested (against the fake CLI), and green.
  **To unblock:** on a machine with a logged-in Claude account, run `INSTALL_ACP=1 ./install.sh` (or
  `npm i -g @zed-industries/claude-code-acp@<pinned>`), then either run the gated test
  (`go test -tags acceptance ./internal/runtime -run TestRealCLIAcceptance -v`) or follow
  [`phase-1-acceptance.md`](phase-1-acceptance.md). If the real wire shapes differ from §12.1, fix only
  `acpmap.go`. **Also verify the pinned `CLAUDE_ACP_VERSION` in `install.sh`** — see autonomous decision below.

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
- **2026-06-27 — Hook token stored in-memory, not persisted.** §6.4 says "record the token against the
  agent" but there is no `state.db` column for it yet (Phase 2 owns the `POST /api/hook` ingest). Stored
  in a `Server.hookTokens` in-memory map keyed by agent_id. **To reverse / complete:** Phase 2 should add
  a persistent column (e.g. on `running` or a `launch_tokens` table) and have launch write it there; then
  drop the in-memory map.
- **2026-06-27 — Two error-envelope shapes coexist.** New Phase-1 session routes use the §7.7 nested
  `{"error":{"code","message","details"}}`; the Phase-0 GET routes (`/api/roles`,`/api/sessions` list, …)
  keep their flat `{"error":"msg"}`. I did **not** migrate the old routes to avoid breaking Phase-0
  tests/clients. **To reverse:** if §7.7 is meant to be truly project-wide, migrate the Phase-0 handlers
  to `writeAPIError` and update their tests — out of scope for Phase 1.
- **2026-06-27 — `messagingServer.Command = os.Executable()`** with args `["mcp-stdio","--agent",ID,
  "--token",T]`. §6.4 says the registration "re-execs the AgentDeck binary in a hidden mcp-stdio mode";
  the `mcp-stdio` subcommand/handler does not exist yet (Phase 5). The fake CLI ignores `mcpServers`, so
  this is registration-only as specified. **To reverse:** Phase 5 adds the real `mcp-stdio` command.
- **2026-06-27 — `CLAUDE_ACP_VERSION=0.4.1` in `install.sh` is an UNVERIFIED placeholder.** The exact
  released version of `@zed-industries/claude-code-acp` could not be confirmed offline (no npm access /
  no adapter installed). The pin is a best guess. **To resolve:** check `npm view
  @zed-industries/claude-code-acp version`, set the real latest-stable pin, and confirm the negotiated ACP
  protocol version is within the runtime's expectations during the 1.6 credentialed run.

## Changelog

_(most recent first; keep ~10, older history is in git)_

- 2026-06-27 — **1.6 code/docs done; credentialed run blocked.** Gated `acceptance` build-tag test +
  `install.sh` adapter pin + `phase-1-acceptance.md` curl/SSE recipe. Default suite green (test excluded);
  compiles under `-tags acceptance`. Real-adapter run needs credentials → Blocked on human.
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
