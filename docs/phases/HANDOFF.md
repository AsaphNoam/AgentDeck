# AgentDeck — Implementation Handoff

**Live state. Read this first, every session. Update it after every change.**
Protocol: [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) (Claude Code or Codex, whichever the human runs).
Keep this lean — apply the condensation rules (workflow §5); old detail lives in git, not here.

---

## Current position

- **Active phase:** 6 — Flexibility: terminal runtime, switch-runtime, task groups
- **Active subphase:** 6.7 (next, optional) — iTerm2/AppleScript driver
- **Spec:** [`tech/phase-6-flexibility-techspec.md`](tech/phase-6-flexibility-techspec.md) (PRD: [`phase-6-flexibility.md`](phase-6-flexibility.md)); subphase plan at §"Subphase plan"
- **Last GREEN checkpoint:** review fix (terminal-tab binary input) @ `main`: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`, `cd ui && npm test`, `cd ui && npm run build`.
- **Branch:** `main` — **trunk-based: all work commits directly to `main`, no per-phase branches, no PRs** (workflow §6). Don't push to origin unless asked.

---

## Phase status

- [x] Phase 0 — Foundation (data model, file store, server & CLI skeleton) ✅
- [x] Phase 1 — Core loop (ACP chat runtime, launch, streaming chat) ✅ — verified against real `claude-code-acp` v0.16.2
- [x] Phase 2 — State manager, SSE bus, dashboard card grid ✅
- [x] Phase 3 — Config CRUD & onboarding ✅
- [x] Phase 4 — Persistence: archive, search, resume, file/command tracking ✅
- [x] Phase 5 — Coordination: MCP messaging, nudger, budgets, notifications ✅
- [ ] Phase 6 — Flexibility: terminal runtime, switch-runtime, task groups
- [ ] Phase 7 — Polish: activity map

Build order: `0 → 1 → 2 → {3, 4, 5} → 6 → 7` (3/4/5 are independent after 2).

---

## Active subphase detail

> The ONLY place granular steps live.

**Phases 0–4 complete ✅** (all subphases green; details in git history & Phase status above).

**Phase 5 complete ✅.** MCP messaging server, message store/tools, per-agent registration, nudger, per-turn budgets, janitor, notification SSE, config-backed notification mutes, Web Notification/in-app toast client, message badges/outbound pulse, and read-only inbox endpoint are all green. Details live in git history (`5.1`–`5.4`) and changelog.

**Subphase 6.1 ✅ — hook ingest hardened + backend adapter + Codex (chat).** `internal/backend/adapter.go`
(`BackendAdapter` for `claude-acp`/`codex-acp`: binary, env-strip keys, `ResolveResumeID`, `CanSwitchModelOnResume`,
`HookMap`/`UnsupportedHookEvents`); chat runtime resolves spawn binary/env-strip per adapter (codex now runs through
the chat runtime); `/api/hook` accepts the terminal lifecycle events + 401-on-stale-token. Details in changelog.

**Subphase 6.2 ✅ — hook scripts + registration + interface gate.** New `internal/hooks`: embedded `_post.sh`
(jq-encoded `curl POST /api/hook`, interface gate) + 5 event wrappers, `Install(home)` (rewritten on dashboard
startup), `ClaudeSettings`/`WriteAgentSettings`. Launch + resume inject `AGENTDECK_*` env and write a per-agent
settings file; `BackendAdapter.HookLaunchArgs` (claude `--settings <path>`, codex gated). The `--settings`
passthrough is gated behind `AGENTDECK_HOOK_REGISTRATION=1` (default off) so real launches aren't regressed. Details
in changelog.

**Subphase 6.3 ✅ — terminal runtime (xterm/PTY default + tmux).** New `internal/runtime/terminal`: `Runtime`
(`Start/SendPrompt/Cancel/Stop/Resume/CheckMessages/Permission/Subscribe/Transcript`) behind the `TerminalDriver`
seam (`StartTab/WriteText/ReadTTY/CloseTab/RevealTab`); xterm/PTY driver (`creack/pty`, Setsid+Setctty, pgid signal)
+ tmux driver (new-session/send-keys/display-message). PTY↔WS bridge at `GET /api/sessions/{id}/terminal/ws`
(`coder/websocket`; binary frames↔master, JSON `{cols,rows}`→`pty.Setsize`). `terminal.Probe()` + `GET
/api/capabilities`. Running row gained `driver`/`driver_ids` (state migration v6). Registry gets the real terminal
runtime via `SetTerminalRuntime` (subpackage→avoids import cycle); status flows from hooks only (runtime writes the
race-guarded initial idle + a `done` on Stop). Details in changelog + Autonomous decisions.

**Subphase 6.4 ✅ — switch-runtime: same-backend (interface/model swap).** `POST /api/sessions/{id}/switch-runtime`
(`internal/server/switch.go`): per-agent switch lock (`Server.switching` set → `409 switch_in_progress`); merge target
over current (`400 no_change` if identical, `400 invalid_field` for bad interface); validate→cancel-and-wait→
`registry.Stop`→cleanup old MCP/hook→persist new identity (`WriteAgent`, agent_id UNCHANGED)→`registry.Resume` (dispatch
by new interface). `resolveResumeId` via the adapter (same-backend→prev native id; `CanSwitchModelOnResume` gate);
chat↔terminal works. Rollback on Resume-after-Stop failure re-launches the previous identity (`500
switch_failed_rolled_back`; double-fault → status `error` + `500 switch_failed`). New switch-runtime error codes added to
`runtime/errors.go` (`no_change`/`invalid_field`→400, `switch_in_progress`/`agent_not_running`→409,
`terminal_unavailable`→422, `switch_failed*`→500). Details in changelog + Autonomous decisions.

**Subphase 6.5 ✅ — switch-runtime: backend-swap history primer.** Cross-backend and non-native-resumable model swaps now
route to `history_handoff:"primer"`: no native resume id, bounded transcript primer appended to this launch's
`SystemPrompt` only, `switch.primer_token_budget` default 8k, tail N=6 turns, summary fallback to local truncation, and
`backend_switch` transcript marker. Claude→Codex fake-backend integration proves marker + new Codex runtime prompt.
Details in changelog + Autonomous decisions.

**Subphase 6.6 ✅ — task groups + remaining endpoints + UI.** Added identity/group endpoints, bounded group release,
liveness pruning, layout group-collapse persistence, grouped card sections with Release group, functional Move-to-group
and switch-runtime context actions, terminal badges/reveal link, terminal tab attached to the PTY WebSocket, terminal
launch option via capabilities, and refreshed embedded UI. Details in changelog + Autonomous decisions.

**Subphase 6.7 — next to implement (optional)** (iTerm2/AppleScript driver; techspec §2.2, §3.6, task 6):
- [ ] iTerm2 `TerminalDriver` implementation via `osascript`.
- [ ] AppleScript templates rendered with `text/template` for create-tab, set-appearance, write-text.
- [ ] Escaping + shell-quote helper with tests for quotes/backslashes/newlines/argv shell-quoting.
- [ ] Capability probe wiring; explicit unavailable `driver:"iterm2"` returns `422 terminal_unavailable` with reason.
- **Checkpoint:** `go build ./...` + `go test ./...` + `go test -tags sqlite_fts5 ./...` (Go-only unless UI driver picker changes).
- **Resume note:** xterm/tmux drivers and capabilities are green. 6.7 is fully skippable; if skipped, roll Phase 6 complete and proceed to Phase 7.

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

- **GATED (not blocking 6.1): live two-CLI MCP registration confirmation.** Subphase 5.1 proved the
  in-process HTTP streamable MCP transport works (round-trips a `ping` via the go-sdk client, both
  directly and through the real dashboard mux). What can't be done without credentials: confirming that
  the **real Claude Code and Codex CLIs** each accept the transport-(A) HTTP MCP entry (vs. needing the
  transport-(B) stdio `agentdeck mcp` subcommand). This is a credentialed acceptance, same class as the
  Phase 1 real-CLI run. **To do (human, ~30min):** launch the dashboard, register an HTTP MCP server
  entry (`type:"http"`, `url:http://127.0.0.1:{port}/mcp`, header `X-AgentDeck-Token`) with each CLI and
  confirm a `ping` tool call round-trips; if a CLI rejects HTTP, note it so 5.3's `RegisterMessagingMCP`
  emits the stdio entry for that backend. This does **not** block 5.2/5.3 — they proceed targeting HTTP
  with the stdio fallback ready. Subphase 5.3 currently emits HTTP MCP entries for both backends pending this verdict.

- **GATED (not blocking 6.2): live Codex (codex-acp) chat acceptance.** 6.1 wired `codex-acp` end-to-end through the
  chat runtime and proved launch→prompt→stream→stop→native-resume against **fakeacp** (the codex adapter supplies the
  binary/env/resume). What's gated: a real `codex-acp` CLI + OpenAI credentials to confirm the live handshake, model
  arg, and native resume. Same class as the Phase 1 real-CLI run. **To do (human):** install `codex-acp`, set
  `CODEX_HOME`/`OPENAI_API_KEY`, launch a Codex chat agent, run a turn, stop, resume; if the live hook event names
  differ from Claude's, note them so 6.2's registration + `codexACP.HookMap()` are corrected.

## Review findings (from the last review — BLOCKING and ADVISORY)

> Written by the review agent (workflow §8), one bullet per finding tagged with its severity
> (`BLOCKING` / `ADVISORY`). Consumed by the fix agent (`/fix-review`, workflow §9), which validates
> each is actually true, then **deletes the bullet** once it's fixed-and-green or dismissed as a
> validated false positive — recording the outcome in the changelog + its end-of-turn summary (§5).
> **This section holds only OPEN findings** — no resolved/dismissed graveyard.
> Blocking items must be fixed before the next phase starts; advisory items when convenient.

- **ADVISORY — terminal panel is a line-box, not xterm.js, and never sends resize.** Task 13 calls for an xterm.js panel; [`TerminalTab.tsx`](../../ui/src/components/chat/TerminalTab.tsx) is a hand-rolled `<pre>` + input that renders raw bytes (ANSI escapes shown literally) and never sends `{cols,rows}`, so the PTY stays at its default size and output wrapping can be wrong. Functional for basic output once the input fix lands, but not the specified terminal experience. Fix: integrate xterm.js (its `onData` → binary frame, `onResize` → `{cols,rows}` text frame) when convenient.

## Autonomous decisions (please review)

> Resolved without stopping; the human should still see them. Remove once acknowledged (workflow §3, §5).

- **NEW (6.6): switch-runtime and move-to-group UI use compact browser prompts/context-menu actions, not a custom in-app dialog/picker yet.**
  The spec asks for a switch-runtime dialog and Move-to-group picker. I implemented the functional API-backed controls through
  the existing card context menu (`window.prompt` for interface/backend/model and group) to keep 6.6 shippable without adding
  a new modal subsystem. **Tradeoff:** the workflow is usable but less polished and lacks capability-gated model/driver dropdowns.
  **To reverse/fix:** replace the prompt flow with a dedicated React dialog backed by `/api/backends` + `/api/capabilities`, and a
  group picker populated from current agent groups.
- **NEW (6.6): liveness pruning marks disappeared processes `done` / `Stop`, not `error`.** §9 says the liveness sweep prunes
  stale rows when a process is gone; it does not pin the resulting badge. I chose `done` with detail `process exited` so a normal
  terminal close reads as stopped rather than a failure. **To reverse:** set status `error`/`Error` (like startup stale reconcile)
  if the human wants unexpected process disappearance to be noisy.
- **NEW (6.5, GATED): target-backend summary is an injectable seam with local truncation fallback by default, not a live CLI call yet.**
  §5.3 calls for a one-shot target-model summary before launch. Without credentialed Claude/Codex CLI surfaces and a confirmed
  non-interactive invocation form, I added `Server.primerSummarizer` as the one-shot seam and made the production default return
  an error so primer synthesis degrades to bounded local truncation (as the spec allows) instead of blocking a switch. Tests inject
  a deterministic summarizer and cover success + failure. **To reverse/fix:** once live CLI surfaces are confirmed, implement
  `defaultPrimerSummarizer` with the chosen `--print`/ACP one-turn invocation and keep the fallback on failure.
- **NEW (review fix): archive resume now resolves identity (interface/backend/model) from the LIVE `agents`
  row, not the frozen `sessions` snapshot.** The terminal-resume BLOCKING fix required this: after a
  chat→terminal switch the snapshot's `interface` stays `"chat"` (no terminal `turn_end` ever refreshes it),
  while the agents row correctly reads `"terminal"` — so the prior snapshot-sourced resume would relaunch the
  wrong runtime. `handleResume` (`internal/server/resume.go`) now reads `agent.Backend/Model/Interface` (the
  identity switch-runtime keeps current); cwd/system_prompt/last_session_id still come from the frozen
  snapshot, and the optional override fields still win. **Why a judgment call:** Phase 4 originally resumed
  purely from the frozen snapshot; trusting the live identity row is the minimal correct source for a switched
  agent and is equivalent for never-switched agents (agents row == snapshot identity). **To reverse:** read
  `snap.Backend/Model/Interface` again — but then a switched-then-stopped agent resumes under its pre-switch
  interface.
- **NEW (6.4): switch-runtime cancel-then-wait is best-effort (poll status≠busy up to 5s), not a true `turn_end` await.**
  §9 says wait up to `config.switch.cancel_timeout_ms` for `turn_end`. I poll the status row leaving `busy` rather than
  subscribing to the runtime hub for the `turn_end` event (simpler, no subscription lifecycle in the handler); the
  streamed events are already persisted, so a lost in-flight tool result is acceptable (§9). The timeout is a hardcoded
  5s const (`switchCancelTimeout`) — `config.switch.cancel_timeout_ms` plumbing is deferred. **To reverse:** subscribe to
  `registry.Subscribe(id)` and block on a `turn_end` event; add the config field.
- **NEW (6.4): switch error codes added to the §7.7 vocabulary with 400/409 statuses.** The spec's §8.1 uses distinct
  code strings (`no_change`, `invalid_field`, `switch_in_progress`, `terminal_unavailable`, `switch_failed*`,
  `agent_not_running`) with 400/409 statuses the existing vocab lacked (it only had 422/404/409/501/502/500). I added the
  code constants + `statusForCode` cases (incl. the first **400** mappings in the project). The not-found case still uses
  the existing `not_found` (404) code string rather than §8.1's `agent_not_found`, for consistency with every other
  session route. **To reverse:** drop the constants/cases; map switch validation onto the generic `validation` (422).
- **NEW (6.4): a not-running agent → `409 agent_not_running` (a code §8.1 doesn't list).** §8.1's listed errors assume a
  live agent; it has no "not running" case. Rather than 404 (the identity exists) I return a new `agent_not_running`
  (409). **To reverse:** fold into `conflict`/`not_found` if preferred.
- **NEW (6.4): switch persists new identity to the `agents` row only; the `sessions` snapshot refreshes on next
  turn_end.** `composeSwitchSpec` reads cwd/system_prompt from the frozen `sessions` snapshot (like resume) and overrides
  backend/model/interface; the durable snapshot's interface/backend/model columns are updated by the indexer on the next
  turn_end, not synchronously in the handler. Archive-resume between the switch and the next turn would see the old
  snapshot identity. **To reverse:** add a `state` writer that updates the snapshot's interface/backend/model in the
  switch handler.

- **NEW (6.3): terminal runtime registered via `Registry.SetTerminalRuntime` (setter), not constructed in `NewRegistry`.**
  The terminal runtime lives in `internal/runtime/terminal`, which imports `internal/runtime` for the `Runtime`
  interface + `Event`/`LaunchSpec`/`Handle`/`Hub` — so `runtime.NewRegistry` can't construct it without an import
  cycle. The server (which imports both) builds it and calls `registry.SetTerminalRuntime(term)`, which swaps out the
  `notImplementedRuntime` stub and wires `onExit`/`StopAll` via interface assertions (`exitNotifier`/`stopAller`). The
  spec named the package `internal/runtime/terminal` (§3), so I kept the subpackage and broke the cycle with the setter
  rather than moving the runtime into package `runtime`. **To reverse:** move the terminal runtime into package
  `runtime` and construct it directly in `NewRegistry` (drops the setter, no import cycle but a fatter package).
- **NEW (6.3, GATED): terminal runtime launches the *interactive* CLI via a hardcoded `interactiveBinary` map +
  `--resume <id>`, both unverified against a live CLI.** Unlike chat (which spawns the ACP adapter `claude-code-acp`),
  terminal runs the real CLI under a PTY (per the 6.2 decision). The backend adapter only models the *ACP* binary, so
  the terminal runtime maps `claude-acp→"claude"`, `codex-acp→"codex"` and uses claude's `--resume <id>` resume form —
  none confirmed against a credentialed CLI (same gate class as the Phase 1 real-CLI / Codex acceptances). Tests use
  `SetCommand("cat")` to avoid needing a real CLI. **To reverse/fix:** add an `InteractiveBinary()`/resume-args method to
  `BackendAdapter` and resolve from there once the live CLI surfaces are known. Codex's resume is `CODEX_HOME`-based, not
  `--resume` — refine when verified.
- **NEW (6.3): two new deps — `github.com/creack/pty` (PTY) + `github.com/coder/websocket` (WS bridge).** Both pure-Go,
  no transitive C. creack/pty backs the xterm driver; coder/websocket backs `/api/sessions/{id}/terminal/ws`
  (accepted with `InsecureSkipVerify` since the server is loopback-only, so the same-machine UI origin is trusted). **To
  reverse:** only by dropping the terminal PTY/WS feature.
- **NEW (6.3): `running.driver_ids` is a JSON-object TEXT column (migration v6), `RunningEntry.DriverIDs map[string]string`.**
  Added alongside `driver TEXT`. Chat agents write empty (`""`/`{}`→nil map, omitted from API JSON). The manager's hook
  "running"/SessionStart paths don't touch the driver columns (ON CONFLICT preserves them). **To reverse:** none sensible —
  6.3 needs it; existing local DBs auto-migrate (no real data lost).
- **NEW (6.3): terminal `Permission` returns `ErrNotImplemented`; `Subscribe` returns an empty hub; `Transcript` returns nil.**
  Terminal has no ACP permission-relay channel (an approval surfaces as `waiting_input` via hooks and the user answers in
  the terminal); terminal *content* flows over the PTY WebSocket, not as normalized `Event`s, so the hub stays empty until
  Stop closes it. **To reverse:** if a terminal driver ever exposes a structured event stream, populate the hub from it.

- **NEW (review fix, supersedes the 6.2 env-flag gate): CLI hook-registration `--settings` passthrough is now gated
  by INTERFACE, not by `AGENTDECK_HOOK_REGISTRATION`.** The launch composer always injects the `AGENTDECK_*` env and
  writes the per-agent settings file; whether it adds the CLI flag (`claude --settings <path>`) now depends on the
  agent's interface: **terminal → ON by default** (the 6.3 terminal runtime runs the *real* interactive CLI under a
  PTY — not `claude-code-acp` — where `--settings` is a known-good flag and hooks are the only status producer);
  **chat → still gated behind `AGENTDECK_HOOK_REGISTRATION=1`** (chat runs through `claude-code-acp`, whose
  `--settings` forwarding is unverified, AND doesn't need registration — the runtime owns chat status and `_post.sh`
  self-suppresses). This resolved the review's BLOCKING finding without regressing the green chat path. **Why this is
  a judgment call:** I chose interface-gating over either flipping the env-flag default (would risk the chat path) or
  building the `.claude/settings.json` project-injection fallback (writes into the user's project dir, can clobber
  user settings). **To reverse:** restore the unconditional `AGENTDECK_HOOK_REGISTRATION` gate in
  `composeHookRegistration`. Codex's `HookLaunchArgs` still returns nil (its hook surface is gated regardless).
- **NEW (6.2): hook scripts require `jq` + `curl` on PATH (POSIX `sh`).** Per techspec §2.3 these are documented
  prereqs (no python3/node at runtime). `_post.sh`'s interface gate runs before `jq`/`curl`, so a chat agent
  self-suppresses even without them; a terminal agent needs both to POST. No fallback is provided. **To reverse:**
  add a curl-less POST path (e.g. a tiny `agentdeck hook-post` subcommand) if a target host lacks them.
- **NEW (6.1): terminal-CLI `Stop` hook does NOT clear the running row.** The subphase line said "running-row
  refresh/clear on SessionStart/Stop", but Claude Code's `Stop` hook fires at the **end of each turn**, not on CLI
  exit (§4.2 footnote ties the clear to "CLI exit", a separate signal). Clearing on every `Stop` would unregister a
  live idle terminal agent. So `SessionStart` refreshes the running row's `session_id`/`tty`; `Stop` only applies
  idle/done status. The running-row clear stays with the runtime's `Stop`, the explicit internal `stopped` event, and
  the 6.6 liveness sweep. **To reverse:** if a real terminal CLI emits `Stop` only on exit, add a running-row delete
  to the `Stop` case in `manager.go::ApplyHook`.
- **NEW (6.1): `/api/hook` token errors realigned to §8.6 on the status path — 401 `bad_token`, 404 `agent_not_found`.**
  Was 403 `forbidden` / 404 `not_found`. The subphase requires "stale token → 401". The file_edit/command **tracking**
  path (Phase 4) is untouched (still 403 `forbidden`). Updated `TestHookValidationErrors` expectations accordingly.
  **To reverse:** restore the prior codes in `hook.go` (status switch) — but §8.6 mandates these.
- **NEW (6.1): Codex `HookMap` mirrors Claude's lifecycle keys — GATED, unverified against a live codex-acp.** Same
  class as the Phase 1 real-CLI / Phase 5 two-CLI gates: without codex-acp credentials I can't confirm Codex's real
  hook event names. I targeted the five Claude keys (`SessionStart`…`Stop`); any Codex rejects in 6.2 move that event
  into `UnsupportedHookEvents` and the terminal runtime backfills it from ACP. The Codex chat e2e (launch→prompt→
  stream→stop→native-resume) is proven against **fakeacp**, not a real codex-acp CLI — the credentialed live Codex run
  remains gated (see Blocked on human). **To reverse:** edit `codexACP.HookMap()` once the live surface is known.
- **NEW (5.4): notification edge detection lives in `internal/bus`, not `state.Manager`.** The tech spec phrases this as a state-manager extension, but the bus already owns the prior `AgentStateUpdate` snapshot needed to edge-detect `done`/`waiting_input` without adding another state cache. `state.Manager` still recomputes `unread_messages`; `bus.PublishStateUpdate` emits `notification` on transitions, and `bus.PublishRuntimeEvent` emits `permission_required`. **To reverse:** move the previous-state cache and notification publishing into `state.Manager` and have the bus only transport events.
- **NEW (5.3): HTTP MCP entries emitted for both `claude-acp` and `codex-acp` while live CLI verdict remains gated.** The spec's Task 1 wants a per-CLI HTTP-vs-stdio decision, but the credentialed live confirmation is still blocked on the human. I chose the already-proven in-process HTTP transport for both backends and left the stdio fallback branch in `registerMessagingMCP` for a future verdict. **To reverse:** change `usesHTTPMessagingMCP(backendType)` for any backend that rejects HTTP and implement/enable the `agentdeck mcp` proxy path.
- **NEW (5.3): direct MCP calls without a runtime turn use implicit turn `t_000000000000`.** Runtime-owned turns still reset real `t_` counters at user/nudge turn boundaries. The implicit row exists so direct MCP tests/manual calls have deterministic budget accounting instead of bypassing the loop cap or failing before a runtime turn. **To reverse:** make `CurrentTurnBudget`/`ConsumeTurnBudget` return an error when no runtime-created row exists and require tests/manual callers to reset one first.
- **NEW (5.1): `go` directive bumped `1.22 → 1.25.0`.** `go get github.com/modelcontextprotocol/go-sdk`
  auto-raised the directive to the SDK's minimum (1.25.0); local toolchain is go1.25.5, all builds/tests
  green. Forced, not chosen — the v1.x SDK the spec mandates requires it. **To reverse:** only by dropping
  the SDK, which the phase can't do. No action expected; flagging because a toolchain-floor bump is a
  durable repo change.
- **NEW (5.1): `/mcp` registered for explicit `POST`/`GET`/`DELETE`, not method-agnostic.** A bare
  method-agnostic `mux.Handle("/mcp", …)` panics — Go 1.22 mux rejects it as conflicting with the
  existing `OPTIONS /` CORS route ("matches more methods but more specific path"). I registered the three
  methods the streamable transport actually uses. **To reverse/extend:** if a future transport needs more
  verbs on `/mcp`, add them explicitly (don't go method-agnostic while `OPTIONS /` exists).
- **NEW (5.2): Phase-0 placeholder `messages` table + its CRUD were REPLACED, not extended.** Migration v5
  drops+recreates `messages` with the §4.1 shape (TEXT `message_id` PK vs the old INTEGER autoincrement) and
  **removes the agent FK / `ON DELETE CASCADE`** (mail must outlive a stopped/deleted agent until the janitor —
  §4.3). The old `state.Message` type and `WriteMessage`/`ReadMessage`/`DeleteMessage`/`ListMessages(to)` are
  gone, replaced by the §3.2 API. The spec contradicted shipped Phase-0 code here; I treated the Phase-0 table
  as the placeholder it was. **Test impact (flagged):** `TestDeleteAgentCascades` now asserts a message
  *survives* its deleted sender (was: cascaded away); migration-count asserts 5 not 4. **To reverse:** none
  sensible — Phase 5 needs this schema. Existing local DBs auto-migrate (the placeholder table held no real data).
- **NEW (5.2): `InsertMessage` returns `(string, error)`, not the spec's `error`.** §3.2 lists
  `InsertMessage(m Message) error`, but §4.1 also requires the server to mint `message_id` with collision-retry.
  I put that minting in `InsertMessage` and return the id (the `send_message` handler needs it for its response).
  **To reverse:** move id-minting into the handler and restore the `error`-only signature.
- **NEW (5.2): tool results are JSON-in-TextContent with `IsError`, `Out`=`any` (no output schema).** The spec's
  success and error payloads have different shapes; rather than fight the typed-output inference I marshal each
  payload to a single text content and set `IsError` on errors (matching §3.3–§3.5 "content[0].text = JSON"). The
  go-sdk still validates *input* schemas strictly (extra args are rejected before the handler — relevant when
  testing). **To reverse:** define typed `Out` structs per tool and use structured content.
- **NEW (5.1): spike kept, not throwaway; `messaging.New` already takes `*state.Store`.** The spec allows
  throwaway-or-keep; I built `internal/messaging` as the keep-able foundation 5.2 extends (the `ping` tool
  is the only throwaway part — 5.2 replaces it with the three real tools). `New(store, log)` takes the
  store now (the ping tool ignores it) to avoid a constructor-signature churn next subphase. The existing
  `launch.go::messagingServer` stdio stub is left untouched and will be **superseded** by 5.3's
  `RegisterMessagingMCP`. **To reverse:** none needed; it's additive.

- **NEW (review fix): seeded-`my-app`-cwd advisory addressed only by surfacing the failure, not by
  pre-launch validation.** The advisory offered two arms: (a) steer users to set a real project before
  launch, or (b) surface the launch failure more directly. I did (b) — `NewAgentModal` now shows the
  server's `error.message` (e.g. "project cwd does not exist") instead of "HTTP 502" — because it's
  bounded and clearly correct. I did **not** do (a): adding pre-launch cwd validation or changing the
  `cwd_not_found` onboarding gate is a design decision the spec explicitly permits as-is, so it's left
  for the human. The seed still points `my-app` → `~/Projects/my-app`. **To take arm (a):** add a
  pre-launch existence check (server 422 or modal-side warning) and/or promote `cwd_not_found` to a hard
  gate. Deleted the finding bullet since the actionable part is fixed.
- **NEW (review fix): archive FTS now indexes the COMPLETE transcript — unbounded buffer chosen over a
  segment model.** The 1 MiB cap was data-loss (older phrases unsearchable), so I removed it. The
  reviewer offered two fixes: (a) index complete content, or (b) a bounded-but-specified segment model.
  I took (a) because it's minimal and zero-risk to the existing single-row `sessions_fts` schema and the
  archive search/COUNT/snippet query — a segment model would need a schema migration (FTS5 can't
  `ALTER ADD COLUMN`, so a drop+recreate) and dedupe/aggregation across multiple rows per agent.
  **Tradeoff:** the per-agent in-memory `content` buffer now grows with the session, and each `turn_end`
  flush rewrites the full FTS row (DELETE+INSERT) → O(n) per turn, ~O(n²) cumulative over one very long
  session. Fine for normal personal use (transcripts of a few MiB); a multi-tens-of-MiB single session
  would get costly. **To reverse / harden later:** implement the segment model (bounded chunk rows per
  agent, append-only, rewrite only the active chunk; archive query groups by `agent_id`, best snippet
  per agent). Guard test: `TestIndexerFTSLongTranscript`.

- **`internal/store` (spec) → `internal/state` (Phase 0 reality).** The runtime imports `internal/state`
  throughout; the spec's `store` is the older name for the same package. No behavior change.
- **`Stop` implemented in 1.3** (spec slots it in 1.4) for test teardown — matches §8.5 exactly; no reversal needed.
- **Tool `Name` ← ACP `kind`** (fallback `title`, then `"tool"`); §4.3 didn't pin the field. Isolated in
  `acpmap.go::toolName`. Verified against the real adapter (turn streamed cleanly).
- **RESOLVED in 2.2: hook token persisted in `running.hook_token`.** `Server.hookTokens` still exists as
  Phase 1 launch scaffolding but hook validation now reads the live `running` row, not the map.
- **Two error-envelope shapes coexist** — new session routes use the §7.7 nested shape; Phase-0 GET routes
  keep flat `{"error":"msg"}` (not migrated, to avoid breaking Phase-0 tests). Migrate later if §7.7 is meant
  to be truly project-wide.
- **`messagingServer.Command = os.Executable()`** with `["mcp-stdio","--agent",ID,"--token",T]` —
  registration-only; the `mcp-stdio` subcommand lands in Phase 5.
- **NEW (4.6): `Server` stores a shared `*index.Indexer` field.** The registry's persistence path and the hook capture both use the same indexer instance so the in-memory FTS content accumulator is shared. To reverse: create a second indexer for hook capture only (no harm beyond a second seed per agent per process).
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
- **Phase 2.1 manager contract:** `state.Manager` wraps the existing Phase 0 `Store`; it does not replace
  typed CRUD. It emits `AgentStateUpdate` through `StatePublisher`, now implemented by `internal/bus`.
  `status.updated_at` is migration v2, `running.hook_token` is migration v3, and `Store.WriteStatus` stamps
  `updated_at` when callers omit it.
- **Phase 2.1 transcript mirror kept generic.** The spec asked for transcript types in `internal/state/types.go`
  but Phase 1's concrete normalized event shapes already live in `internal/runtime/event.go`. I added only
  `state.TranscriptEvent {Kind, Data}` as a storage/UI-facing mirror to avoid duplicating runtime structs.
  To reverse: replace it with concrete state-owned transcript structs when 2.4/2.6 needs them.
- **Phase 2.3 kept runtime Hub internally.** The HTTP route `GET /api/sessions/{id}/events` is deleted and
  transcript deltas now publish as bus `new_message`, but `Runtime.Subscribe`/per-agent `Hub` still exist for
  runtime tests and local internal compatibility. To reverse: remove the hub API once no tests/internal callers need it.
- **Phase 2.4 replaced the walkthrough UI source.** The repo had a product-demo React app, not the dashboard shell
  scaffold described by the spec. I replaced `ui/src` with the Phase 2 shell/stores/SSE foundation and refreshed
  `internal/server/ui/dist`. To reverse: recover the demo from git history, but it is no longer the Phase 2 target UI.
- **Phase 4.1 writer API takes optional metadata.** The tech spec pseudo-signature said `Open(home, agentID)` but also
  requires the writer to create the first `session_meta` record. I implemented `transcript.Open(home, agentID, meta)`
  so runtime wiring can pass the frozen launch snapshot at creation; `nil` skips meta for tests/recovery cases. To
  reverse: split this into `Open` + explicit `AppendSessionMeta` before 4.3 runtime wiring.
- **Phase 4.2 no-tag FTS fallback.** The Phase 4 spec requires SQLite FTS5 with `-tags sqlite_fts5`, but the canonical
  workflow still requires ordinary `go build ./...` and `go test ./...` to pass. Migration v4 creates a real FTS5
  virtual table when the tag is enabled and a schema-compatible plain `sessions_fts` table otherwise. Tagged builds/tests
  cover real `MATCH`; no-tag builds keep state.Open usable. To reverse: make all checkpoints/builds always pass
  `-tags sqlite_fts5` and remove the fallback branch in `ensureSessionsFTS`.
- **Phase 4.3 adds full `system_prompt` to new `session_meta` records.** The DB schema requires exact `system_prompt`,
  but 4.1 initially only modeled `system_prompt_sha`. Runtime wiring now writes `system_prompt` into `session_meta`
  and `sessions.system_prompt`; reindex of any older raw log without that field leaves the DB column empty. To reverse:
  remove `SessionMetaData.SystemPrompt` and require runtime Start to upsert the DB snapshot out-of-band.

## Changelog

_(most recent first; keep ~10, older history is in git)_

- 2026-06-30 — **review fix: switch-runtime / move-to-group failures now surface a toast — green.** ADVISORY:
  `CardContextMenu` fired `void switchRuntime(...)` / `void updateAgentIdentity(...)` with no `.catch`, so any
  failure (the common `400 no_change`, `409 switch_in_progress`, `422`, rollback `500`) was invisible. Added a
  `pushError(title, body?)` action to `uiStore` (new `"error"` toast type) and `.catch` → `pushError` on both
  context-menu actions; also taught `client.ts::json()` to extract the §7.7 nested `error.message` so the toast
  shows the real reason instead of a bare status line. Test `CardContextMenu.test.tsx` (new) asserts a failing
  switch-runtime/move-to-group yields an `"error"` toast carrying the server message. Embedded UI dist refreshed.
  Checkpoint green: `go build ./...`, `cd ui && npm test`, `cd ui && npm run build`.
- 2026-06-30 — **review fix: terminal-tab input reaches the PTY (binary frame) — green.** BLOCKING:
  `TerminalTab.tsx`'s `send()` sent `ws.send(`${input}\n`)` — a string, transmitted as a WebSocket
  *text* frame, which the PTY↔WS bridge routes to resize and drops (only binary frames reach the PTY
  master), so the headless xterm/PTY driver's only input surface was inert. Now sends
  `ws.send(new TextEncoder().encode(input + "\n"))`. Test `TerminalTab.test.tsx` (new) asserts Send and
  Enter each emit a non-string ArrayBuffer view decoding to `"<cmd>\n"`. Embedded UI dist refreshed.
  Checkpoint green: `go build ./...`, `cd ui && npm test`, `cd ui && npm run build`.
- 2026-06-29 — **6.6 green — task groups + remaining endpoints + UI.** Backend: `POST /api/sessions/{id}/identity`
  edits name/group and emits `state_update`; `POST /api/groups/{group}/release` stops group members with a bounded worker
  pool and returns per-agent results; existing rename now returns the §8.2 shape; layout schema/API persists
  `groups[name].collapsed`; dashboard state includes terminal `tty`/`driver`; reconciliation loop prunes stale running rows.
  UI: grouped card sections with persisted collapse + aggregate state summary + Release group; context-menu Move-to-group,
  Switch runtime, and Reveal terminal actions; terminal badge on cards; `backend_switch` transcript divider; terminal tab
  attaches to `/api/sessions/{id}/terminal/ws`; new-agent modal enables terminal via `/api/capabilities`; embedded UI dist
  refreshed. Tests: new server coverage for identity, reserved group, release group, stale-running prune; existing UI tests
  updated for terminal availability. Checkpoint green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`,
  `cd ui && npm test`, `cd ui && npm run build`. See Autonomous decisions for the MVP prompt-based UI controls + liveness badge.
- 2026-06-29 — **6.5 green — switch-runtime: backend-swap history primer.** Removed the cross-backend `501` guard:
  `handleSwitchRuntime` now routes cross-backend swaps and same-backend model swaps with `CanSwitchModelOnResume=false`
  through `history_handoff:"primer"` (empty native resume id). New `internal/server/primer.go` reads
  `sessions/{agent_id}/transcript.ndjson`, synthesizes a bounded primer (older-turn summary + last N=6 verbatim turns),
  appends it to this launch's `SystemPrompt` only, honors `config.json switch.primer_token_budget` (default 8k), and falls
  back to local truncation if the one-shot target-model summary seam fails. New `runtime.EvBackendSwitch`/
  `BackendSwitchData`; switch appends `{type:"backend_switch", from, to, at}` after target resume succeeds. Tests: primer budget,
  summarizer success + fallback, marker append, Claude→Codex backend swap (handoff=primer, marker present, identity switched,
  new Codex fake session accepts prompt). Green both tag modes (Go-only). See Autonomous decisions for the gated live-summary seam.
- 2026-06-29 — **review fix: switch-runtime keeps the target registration + terminal archive resume — green.**
  (1) BLOCKING: `handleSwitchRuntime` (`internal/server/switch.go`) cleaned the OLD MCP/hook artifacts (keyed by
  the unchanged `agent_id`) AFTER `composeSwitchSpec` had already registered the fresh target token + rewritten
  the per-agent hook settings file — so it revoked the new MCP token, deleted the `--settings` file the resume
  needs, and orphaned the old token (its cleanup closure was overwritten). Reordered to validate (new pure
  `validateSwitchTarget` — no side effects) → stop old → cleanup OLD → `composeSwitchSpec` (register fresh) →
  resume. Test `TestSwitchRuntimeKeepsTargetRegistration` (chat→terminal: hook settings file present + MCP token
  still `Lookup`-able after the 200). (2) BLOCKING: removed the stale `501 "terminal resume not implemented"`
  guard in `handleResume`; resume now resolves interface/backend/model from the live `agents` row (not the frozen
  snapshot, which stays `chat` after a switch). Test `TestResumeTerminalAgent` (chat→switch terminal→stop→resume
  → terminal running row with tty/driver). See Autonomous decisions for the identity-source judgment call. Green
  both tag modes (Go-only).
- 2026-06-29 — **6.4 green — switch-runtime: same-backend (interface/model swap).** New `internal/server/switch.go`:
  `POST /api/sessions/{id}/switch-runtime {interface?, backend?, model?}`. Per-agent switch lock (`Server.switching`
  set; concurrent → `409 switch_in_progress`). Flow: merge target over current (`400 no_change`/`400 invalid_field`) →
  validate target (terminal driver via `terminal.Probe().DriverAvailable`; backend/model exist) → adapter
  `ResolveResumeID(prev.SessionID, true)` (native, same-backend) gated by `CanSwitchModelOnResume` →
  `cancelAndWait` (poll status≠busy ≤5s) → `registry.Stop` + cleanup old MCP/hook → `WriteAgent(target)` (agent_id
  UNCHANGED) → `registry.Resume` (dispatches by new interface). Rollback on Resume-after-Stop failure re-launches the
  previous identity (`500 switch_failed_rolled_back`); double-fault sets status `error` + `500 switch_failed`.
  Cross-backend swap guarded `501` (history primer → 6.5). New switch error codes in `runtime/errors.go` (first 400
  mappings). Tests: model-swap (agent_id stable, new session_id, identity persisted, handoff=native_resume), chat→terminal
  (running row terminal/xterm/tty), no_change (400), rollback (500 rolled_back + chat restored). Green both tag modes (Go-only).
- 2026-06-29 — **6.3 green — terminal runtime (xterm/PTY default + tmux).** New `internal/runtime/terminal` package
  implementing `runtime.Runtime` behind a `TerminalDriver` seam (`StartTab/WriteText/ReadTTY/CloseTab/RevealTab`):
  xterm/PTY driver (`creack/pty`; opens a PTY, child as session leader via Setsid+Setctty so pid==pgid, records the slave
  tty, reaps via one Wait closing an `exited` chan) + tmux driver (`new-session -d`/`send-keys`/`display-message`/
  `kill-session`). PTY↔WebSocket bridge at `GET /api/sessions/{id}/terminal/ws` (`coder/websocket`): binary frames↔PTY
  master, JSON `{cols,rows}` text frames→`pty.Setsize`; pump logic unit-tested against fakes. `terminal.Probe()` +
  `GET /api/capabilities` (xterm always available + default; tmux if on PATH; iterm2 reported unavailable w/ reason until
  6.7). State migration v6 adds `running.driver`/`driver_ids`; `RunningEntry` carries them. Registry swaps the terminal
  stub for the real runtime via `SetTerminalRuntime` (subpackage→avoids import cycle; wires onExit/StopAll). Status flows
  from hooks only — the runtime writes just the race-guarded initial idle (§3.1 step 7) and a `done` on Stop; Cancel
  SIGINTs the pgid, Stop SIGTERM→grace→SIGKILL then CloseTab + clears the running row. Tests: bridge pumps (keystroke→
  master, output→frame, resize→Setsize), capabilities (endpoint + probe), terminal launch records tty/driver + idle→busy
  →idle via `Manager.ApplyHook`, WS route 404s unknown agent. Green both tag modes (Go-only; no `ui/` change). New deps:
  `creack/pty`, `coder/websocket`. Interactive-CLI binary map + `--resume` are GATED (see Autonomous decisions).
- 2026-06-29 — **review fix: hook registration enabled by default for terminal + per-agent settings cleanup — green.**
  (1) BLOCKING: `composeHookRegistration` no longer disables registration in the default path — it now gates by
  *interface*: terminal agents get the `--settings` launch args by default (the terminal runtime runs the real CLI
  under a PTY where the flag is known-good and hooks are the only status producer), while chat stays gated behind
  `AGENTDECK_HOOK_REGISTRATION=1` (claude-code-acp flag-forwarding unverified; chat doesn't need hook registration).
  This unblocks 6.3 terminal status without regressing the green chat path (see Autonomous decisions for the judgment
  call). Test `TestComposeHookRegistrationTerminalDefault` (terminal → `--settings <path>` with no env flag); existing
  `TestComposeHookRegistration` keeps chat default-off + chat self-suppression covered by the hooks interface-gate
  test. (2) ADVISORY: per-agent `{home}/hooks/agents/{id}.json` is now deleted on stop, launch-rollback, and shutdown
  (new `hooks.RemoveAgentSettings`/`RemoveAllAgentSettings` + `Server.cleanupHookSettings`, mirroring
  `cleanupMessagingMCP`). Test `TestStopRemovesHookSettings` (file present at launch, gone after stop). Green both tag modes.
- 2026-06-29 — **6.2 green — hook scripts + registration + interface gate.** New `internal/hooks` package: embedded
  POSIX-`sh` script set — `_post.sh` (jq-encoded body → `curl POST /api/hook`, with the `AGENTDECK_INTERFACE=chat`
  self-suppression gate for runtime-owned events) + `session-start/user-prompt-submit/pre-tool-use/post-tool-use/stop.sh`
  wrappers; `Install(home)` atomically (re)writes them to `{home}/hooks` on dashboard startup; `ClaudeSettings` +
  `WriteAgentSettings` compose a per-agent Claude hooks settings file from the adapter `HookMap`. Launch + resume now
  inject `AGENTDECK_HOOK_URL/TOKEN/AGENT_ID/INTERFACE` env and write the settings file; new
  `BackendAdapter.HookLaunchArgs` (claude `--settings <path>`, codex nil/gated) feeds `LaunchSpec.ExtraArgs`, appended
  to the spawn argv. The `--settings` activation is gated behind `AGENTDECK_HOOK_REGISTRATION=1` (default off) so real
  launches aren't regressed by an unverified flag (see Autonomous decisions). Tests: hooks install/executability,
  `ClaudeSettings` shape, hermetic interface-gate (shimmed curl+jq: chat→no POST, terminal→POST); server hookEnv +
  composeHookRegistration; adapter `HookLaunchArgs`. Green both tag modes (Go-only).
- 2026-06-29 — **6.1 green — hook ingest hardened + backend adapter + Codex (chat).** New `internal/backend/adapter.go`:
  `BackendAdapter` for `claude-acp`/`codex-acp` carrying `Binary`/`LaunchArgs`/`StripEnvKeys`/`ResolveResumeID`/
  `CanSwitchModelOnResume`/`HookMap`/`UnsupportedHookEvents`. `ChatRuntime` now resolves the spawn binary + env-strip
  per adapter (claude strips `CLAUDECODE`; codex strips nothing) instead of hardcoding claude — **codex-acp now runs
  through the chat runtime** (gate accepts known backends, rejects unknown with `ErrNotImplemented`). `/api/hook`
  accepts the terminal lifecycle events (`SessionStart`/`UserPromptSubmit`/`PreToolUse`/`PostToolUse`/`Stop`):
  SessionStart refreshes the running row `session_id`/`tty` (new `HookPayload.TTY`); the rest are pure status
  producers; **Stop does not clear the running row** (per-turn — see Autonomous decisions). Status-path token errors
  realigned to §8.6 (`401 bad_token`, `404 agent_not_found`). Per-model env was already layered in `composeLaunch`
  (model env overrides backend env). Tests: `backend` adapter units; runtime backend-gate + Codex chat e2e
  (launch→prompt→stream→stop→native-resume vs fakeacp); server hook-lifecycle ingest (SessionStart refresh, PreToolUse
  busy+publish, Stop keeps running row, stale→401). Green both tag modes; live codex-acp run gated (Blocked on human).
- 2026-06-29 — **review fix: budget_exceeded toast names the agent + dismissed recipient-badge false positive — green.** New `bus.PublishBudgetExceeded` routes breaches through `notificationPayload` (the existing `budget_exceeded` case) using the agent's snapshot, so the toast carries `agent_name`/`address`/named title instead of the old inline generic payload; `SetBudgetExceededSink` now uses it. Tests: `TestPublishBudgetExceededNamesAgent` + `…FallsBackToAgentID`. **Dismissed** the "recipient unread badge doesn't update live" advisory as a false positive: the message-inserted sink calls `stateMgr.Touch(toAgentID)`, and `Touch`→`recomputeAndPublish` already `PublishStateUpdate`s the recipient with the recomputed `unread_messages` (the inline `SetSnapshot` was merely redundant — `PublishStateUpdate` already sets the snapshot). Dropped that redundant `SetSnapshot`; guard test `TestTouchRecipientPublishesUnread`. Green both tag modes.
- 2026-06-29 — **review fix: turn budget single-row-per-agent — green (also fixes unbounded growth).** `ResetTurnBudget` now deletes the agent's other `turn_budget` rows in-tx so at most one row survives per agent. Fixes the restart+resume blocker: `turnSeq` resets to 0 on a fresh process, so a resumed agent re-emitted low `turn_id`s while prior-session rows kept the highest rowids — `currentBudgetTx`'s `ORDER BY rowid DESC` read a stale/breached row and could block `send_message`/`check_messages`. One row per agent also caps `turn_budget`'s formerly unbounded growth (resolves that advisory too). Test `TestResetTurnBudgetReusesSingleRow` simulates the restart, asserts the freshly-reset `t_…01` is read (0 used, not the stale `t_…02`), and that exactly one row remains. Green both tag modes.
- 2026-06-29 — **5.4 green / Phase 5 COMPLETE — notifications + dashboard message indicators.** `AgentState` now includes `unread_messages` and `last_sent_at`; message sends touch recipient/sender state for unread badges and outbound pulse; bus emits edge-triggered `notification` SSE for done/waiting_input/permission_required plus the existing budget_exceeded path. `config.json` gained `notifications.desktop_enabled` + per-type mutes via existing `GET/PUT /api/config`; UI consumes notification SSE, sends hidden-tab desktop notifications when permitted, visible-tab toasts otherwise, and adds Settings notification toggles. Added read-only `GET /api/sessions/{id}/messages`. Embedded UI refreshed. Tests: Go notification/indicator/config/inbox coverage; UI mute + hidden desktop notification + settings toggle. Checkpoint green: Go standard/tagged build+tests, `cd ui && npm test`, `cd ui && npm run build`.
