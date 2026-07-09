# AgentDeck — Implementation Handoff

**Live state. Read this first, every session. Update it after every change.**
Protocol: [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) (Claude Code or Codex, whichever the human runs).
Keep this lean — apply the condensation rules (workflow §5); old detail lives in git, not here.

---

## Current position

- **Active phase:** 7 — Additional features: OpenHands & OpenCode backends (Phase 6 complete ✅)
- **Active subphase:** 7.4 (next) — GATED live acceptance, **blocked on human** (needs `opencode`+`openhands` CLIs + provider keys); 7.1–7.3 done ✅. All fakeacp/UI paths green.
- **Spec:** [`tech/phase-7-additional-features-techspec.md`](tech/phase-7-additional-features-techspec.md) (PRD: [`phase-7-additional-features.md`](phase-7-additional-features.md))
- **Last GREEN checkpoint:** phase 7.3 (UI plumbing): `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`, `cd ui && npm run test` (78) + `npm run build` + embed.
- **Branch:** working branch `claude/work-phase-ngp3b7` (this session's designated branch); commit here.

---

## Phase status

- [x] Phase 0 — Foundation (data model, file store, server & CLI skeleton) ✅
- [x] Phase 1 — Core loop (ACP chat runtime, launch, streaming chat) ✅ — verified against real `claude-code-acp` v0.16.2
- [x] Phase 2 — State manager, SSE bus, dashboard card grid ✅
- [x] Phase 3 — Config CRUD & onboarding ✅
- [x] Phase 4 — Persistence: archive, search, resume, file/command tracking ✅
- [x] Phase 5 — Coordination: MCP messaging, nudger, budgets, notifications ✅
- [x] Phase 6 — Flexibility: terminal runtime, switch-runtime, task groups, drivers (xterm/tmux/iterm2) ✅
- [ ] Phase 7 — Additional features: OpenHands & OpenCode backends — **7.1–7.3 ✅** (adapters, config, terminal gates, yolo/credchecks/switch matrix, UI); **7.4 GATED** (live acceptance, blocked on human credentials). PRD [`phase-7-additional-features.md`](phase-7-additional-features.md), spec [`tech/phase-7-additional-features-techspec.md`](tech/phase-7-additional-features-techspec.md)

Build order: `0 → 1 → 2 → {3, 4, 5} → 6 → 7` (3/4/5 are independent after 2).

---

## Active subphase detail

> The ONLY place granular steps live.

**Phases 0–4 complete ✅** (all subphases green; details in git history & Phase status above).

**Phase 5 complete ✅.** MCP messaging server, message store/tools, per-agent registration, nudger, per-turn budgets, janitor, notification SSE, config-backed notification mutes, Web Notification/in-app toast client, message badges/outbound pulse, and read-only inbox endpoint are all green. Details live in git history (`5.1`–`5.4`) and changelog.

**Phase 6 complete ✅.** Backend adapter + Codex chat (6.1), hook scripts/registration + interface gate (6.2),
terminal runtime behind the `TerminalDriver` seam with xterm/PTY + tmux + iTerm2 drivers and the PTY↔WS bridge
(6.3/6.7), same-backend switch-runtime (6.4), backend-swap history primer (6.5), task groups + endpoints + UI (6.6),
and driver-selection plumbing (`driver` field on launch/switch → per-agent runtime dispatch, `422
terminal_unavailable` for unavailable drivers) are all green. `GET /api/capabilities` advertises xterm/tmux/iterm2.
Details in git history (`6.1`–`6.7`) and changelog.

**Phase 7 — in progress.** OpenHands & OpenCode chat backends through the existing ACP runtime (no runtime changes).

- **7.1 ✅ (adapters + config + terminal gates).** `opencodeACP`/`openhandsACP` adapters (`internal/backend/adapter.go`);
  new optional `ExtraEnvProvider` interface consumed in `chat.go::spawnCmd` (OpenHands sets `LLM_MODEL`); seed backends
  `opencode`/`openhands` + widened type-union comment; `terminalSupported` helper in `server/terminal.go` replacing the
  three `codex-acp` literals in launch/switch/resume composers. Tests: `TestNewBackendAdapters`,
  `TestOpenHandsExtraEnvCarriesModel`, `TestOpenCodeChatE2E`, `TestOpenHandsChatE2E`, `TestNewBackendTerminalRejected`.

- **7.2 ✅ (permissions, credchecks, switch matrix, PUT validation).** OpenCode skip=true → `ExtraEnv` injects
  `OPENCODE_CONFIG_CONTENT` yolo config; OpenHands skip relies on the shared runtime auto-approve gate (CLI-side arm
  GATED — see Autonomous decisions). `credcheck/opencode.go` + `openhands.go` (installed-binary + auth.json/provider-key
  or LLM_API_KEY/settings.json → ok, else skipped). Widened `knownBackendTypes` to the four-value union (rejects unknown
  at PUT; also lets the new seeds validate). Tests: `TestSkipPermissionsEnvOpenCode`, `TestOpenCodeProber`,
  `TestOpenHandsProber`, `TestValidateBackendsConfig_NewBackendTypesAccepted`, `TestSwitchClaudeToOpenCodePrimer`.

- **7.3 ✅ (UI plumbing).** `schemas/backends.ts` exports `backendTypeSchema`/`BackendType` (four-value enum, single
  source); new `lib/backendTypes.ts` (`BACKEND_TYPE_LABELS`, `BACKEND_TYPE_OPTIONS`, `terminalSupported`). BackendStep
  and BackendsEditor render type options from the enum+labels; BackendStep shows `LLM_API_KEY`/`LLM_BASE_URL` inputs
  for `openhands-acp`. NewAgentModal disables/hides Terminal for non-claude backends and resets a stale terminal
  selection to chat. Embedded dist refreshed. Tests (+4): editor four-option, BackendStep openhands fields, modal
  terminal disable + reset-to-chat.

- **7.4 — GATED, blocked on human.** Live acceptance against real `opencode`/`openhands` CLIs (handshake, one streamed
  turn, permission round-trip, stop, resume-or-primer verdict, `mcpServers` honor verdict). See "Blocked on human".
  Everything is fakeacp/UI-green; when credentials arrive, add `//go:build acceptance` tests and flip the GATED adapter
  one-liners per the verdicts (§2.1 resume, §2.3 yolo arms, §2.4 MCP). Nothing in 7.4 may regress the fake paths.

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

- **GATED (7.4): live OpenCode & OpenHands acceptance.** Phases 7.1–7.3 wired both backends end-to-end through the
  chat runtime and proved launch→prompt→stream→stop→resume against **fakeacp**, plus yolo env, credchecks, cross-backend
  primer switch, and the full UI. What's gated (needs the real CLIs + provider keys, same class as the Phase 1/Codex
  acceptances): confirm (1) `opencode acp` / `openhands acp` speak the same ACP handshake the runtime expects; (2)
  native `session/load` resume works (else the adapters' `ResolveResumeID` should return `""` → primer floor, a one-line
  flip); (3) OpenCode `OPENCODE_CONFIG_CONTENT` yolo keys are correct, and pick OpenHands' skip arm (currently the
  shared auto-approve gate; confirm/adopt the CLI's ACP always-approve mode or flag — see Autonomous decisions); (4)
  each CLI honors HTTP `mcpServers` entries via `session/new` (else document, don't build, the fallback). **To do
  (human):** install `opencode` + `openhands`, set provider keys, launch a chat agent of each, run a turn + permission
  round-trip + stop + resume; note any deviation so the adapter one-liners are flipped. Does not block anything else —
  the phase ships with gates documented if credentials never arrive (Codex precedent).

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

**Source:** fifth full top-to-bottom review (2026-07-04) — focused mixed-model reviews for
foundation/config/state/persistence, runtime/switching, UI/SSE/dashboard, and
coordination/notifications/shutdown, followed by a holistic higher-intelligence synthesis and
main-agent verification. Baseline in this sandbox: `cd ui && npm run test` and `cd ui && npm run
build` pass; `env GOCACHE=/Users/mcnoam/Projects/AgentDeck/.gocache go build ./...` exits 0 but
prints a sandbox-denied Go module stat-cache write outside the repo; `env
GOCACHE=/Users/mcnoam/Projects/AgentDeck/.gocache go test ./...` and `go test -tags sqlite_fts5
./...` are partially blocked here because several tests call `httptest.NewServer`, which fails to
bind `tcp6 [::1]:0` under this sandbox. The prior cancel-escalation BLOCKING finding is fixed in
current code (`permission.go` captures `turnSeq`); the permission-resolution race remains open.

### BLOCKING

### ADVISORY

- **ADVISORY — archive `matched_in` can go empty on mixed metadata+transcript hits.**
  `internal/archive/archive.go:207-219`: `matchedIn` only returns `metadata` or `transcript` when *all* query terms are
  contained inside one field. A normal query that spans both fields, such as one token in the agent name and one token in
  the transcript, still returns a valid FTS hit but `matched_in` comes back empty, so the archive UI cannot explain the
  result and the API shape is misleading. Fix: compute field coverage per token/column, or mark any result whose terms are
  split across metadata and transcript as matching both; test: query a session whose name matches one term and transcript
  matches another, and assert `matched_in` is non-empty.
- **ADVISORY — New Agent modal does not follow later default-backend changes.**
  `ui/src/features/launch/NewAgentModal.tsx:30-76`: `backendId` initializes once and only fills
  when empty, so an open modal can keep a stale backend after Settings changes the default. Fix:
  track whether the current selection was auto-derived and resync on default changes until the user
  explicitly selects a backend.
- **ADVISORY — hook-only file/command activity never bumps session recency.**
  `internal/index/indexer.go:392-448`: `CaptureHookFile`/`CaptureHookCommand` refresh rollup
  counts but not `sessions.updated_at` or `last_seq`; terminal-only activity can stay buried in
  archive ordering and look idle until another turn boundary. Fix: touch the session row from hook
  capture; test: hook file/command activity moves the session to the top of `/api/archive`.
- **ADVISORY — live Files/Commands tabs are one-shot snapshots.**
  `ui/src/components/chat/FilesTab.tsx:48-56` and
  `ui/src/components/chat/CommandsTab.tsx:35-43` fetch only on mount; if the agent keeps editing or
  running commands while the tab is open, the list stays frozen until remount. Fix: refetch on
  relevant SSE/transcript activity or poll while visible; test: add a tracked row after mount and
  assert the visible tab updates.
- **ADVISORY — unread badges stay stale after message read/delete/expiry.**
  `internal/messaging/tools.go:182-230`, `internal/server/messaging_loops.go:91-106`, and
  `internal/server/server.go:114-129`: `send_message` publishes a state update, but
  `check_messages` and janitor cleanup mutate read/delete state without touching the affected
  agent, so `unread_messages` can remain nonzero until unrelated activity. Fix: publish/touch after
  read/delete/expiry; test: reading or expiring messages immediately emits `unread_messages:0`.
- **ADVISORY — nudger cooldown state survives stop/relaunch by agent_id.**
  `internal/server/messaging_loops.go:12-26,40-87`: in-memory nudge state is keyed only by stable
  `agent_id`, so a fresh launch can inherit stale `inFlight`/`lastNudgeAt` and miss a wake for up
  to the cooldown. Fix: key the cache by launch generation/started_at or clear it when the running
  row changes; test: stop/relaunch with pending mail still nudges promptly.
- **ADVISORY — user's own chat prompts are never persisted; history reads one-sided on every
  revisit.** No user-prompt `EventType` (`internal/runtime/event.go`); the Composer's `user_text`
  is client-local; every ChatPanel mount / gap-refetch / archive view drops it; typed text is
  unsearchable in FTS. Formally in-spec (phase-2 techspec resolved this client-side), but it is the
  most frequently user-visible defect found — recommend before Phase 7: emit+persist a `user_text`
  event in `SendPrompt` (and nudge turns).
- **ADVISORY — crash-path teardown lacks a launch-generation guard (root of a reproducible ~2%
  test flake).** `teardownAgentRegistration` is keyed by agent_id only (`launch.go:441`, exit hook
  `server.go:150`) — a late crash teardown for launch N deletes launch N+1's hook-settings/MCP
  file/token (switch re-registration window, `switch.go:147-180`).
  `TestSwitchRuntimeKeepsTargetRegistration` fails ~6/300 under `-race -count=300` (switch_test's
  `cat` + `--settings` ExtraArgs dies instantly, racing the assertions). Fix: generation/epoch tag
  on artifacts (exit hook no-ops on mismatch) + a flag-tolerant long-lived test command.
- **ADVISORY — StopAll ignores ctx; stop grace is serial 5s per agent; the tmux path always sleeps
  the full 5s** (`internal/runtime/permission.go:210-220`, `chat.go:977-984`,
  `terminal/terminal.go:396-399`) — multi-agent shutdown overshoots every timeout → SIGKILL +
  possible orphaned process groups.
- **ADVISORY — reconcile sweep stomps switched-to-terminal agents' status detail with stale
  pre-switch chat text.** `internal/server/reconcile.go` derives previews from `transcript.ndjson`
  with no interface check; `ApplyStaleCorrection` discards `RunningEntry.Interface`
  (`state/manager.go:176-244`). Self-heals on the next hook. Fix: skip the preview when
  `interface != "chat"`.
- **ADVISORY — the nudger has no retry cap or backoff** (`messaging_loops.go:40-89`): any
  recipient that can't drain unread mail is re-nudged every ~62s indefinitely (bounded only by the
  mail TTL). Cap per (agent, oldest-unread) or back off exponentially.
- **ADVISORY — notification edge detection is racy: duplicate or missed done/waiting_input
  notifications.** `Manager.Touch` skips `writeMu` (`manager.go:82-84`); `PublishStateUpdate`
  reads prev + writes snapshot under separate lock acquisitions (`bus.go:124-145`); the
  message-insert sink touching the recipient races its own turn-end touch → double "finished"
  toasts or a card stuck busy. Fix: read-prev + set-snapshot + publish under one lock; Touch takes
  `writeMu`.
- **ADVISORY — terminal nudge injects mid-typing.** `terminal/terminal.go:199-205` writes
  text+`\n` straight to the PTY without the §5.2 pre-injection idle re-check chat does — can
  submit a mangled half-typed command. Re-check status just before `WriteText`.
- **ADVISORY — `budget_exceeded` notifies on every over-limit retry, not first breach**
  (`state/messages.go:398-422` re-marks breached unconditionally; `messaging/tools.go:143,202`
  fire the sink each time). Gate on the prior breached flag.
- **ADVISORY — Settings editors discard structured validation errors.** Roles/Projects/Backends
  `onError` shows `String(e)` → "Error: HTTP 400" though the 400 body names the offending field
  (`ui/src/api/config.ts` `.body` unread outside the DELETE-409 handlers). Same class as the fixed
  NewAgentModal gap — generalize it.
- **ADVISORY — SSE client: notification mutes are silently ignored on deep links** (`sse.ts:97-105`
  reads config via passive `getQueryData`, populated only on `/` and `/settings` routes) — prefetch
  config in `main.tsx`. **And transcript refetches race with no ordering token** (gap-refetch,
  ChatPanel mount, reconnect refetch → last-to-resolve wins, transcript can regress until the next
  append). Add a per-agent request token or max-seq compare before `setTranscript`.
- **ADVISORY — archive search UI hardcodes limit 50 / offset 0** (`ArchivePage.tsx:72`) while
  displaying the true total; matches past 50 are unreachable. Add pagination.
- **ADVISORY — terminal `driver` field is wired at the API/runtime level but not in the UI, and
  `config.terminal.max_tabs` is unimplemented.** 6.7 added the `driver` field to launch + switch
  (validated → `422 terminal_unavailable`, `DriverAvailable`'s 422 now reachable) and per-agent
  runtime dispatch, so tmux/iterm2 are selectable via the API. Still open: the New-Agent modal /
  switch dialog have no driver picker (UI always sends the default), and `config.terminal.max_tabs`
  / `429 terminal_tab_limit` (techspec §9) is entirely unimplemented and untracked — implement or
  record as a deviation.
- **ADVISORY — liveness/identity checks trust bare PIDs.** The pidfile (`cli/pidfile.go:83-95`)
  and the running-row sweeps (`server/reconcile.go:202-207`, `runtime/reconcile.go:43-50`) use
  `kill(pid,0)` with no start-time//proc-comm/nonce corroboration → PID reuse can block `start`,
  mis-target `stop`, or keep dead rows alive. Same primitive gap in both places; compounds with
  the Stop-orphan BLOCKING.
- **ADVISORY — `start --detach` residue from aa6f99c:** concurrent double-invocation TOCTOU
  remains (no flock/O_EXCL; `removePidfile` never verifies the pidfile names its own PID — a
  losing child can delete the winner's live pidfile), and the 300ms confirm grace is measured from
  spawn, not bind (slow setup → parent prints "started", child dies after). The re-exec/grace/
  confirm paths are untested.
- **ADVISORY — `emit()` delivery order can invert seq.** `chat.go:704-732`: seq assigned under
  lock, persist/hub/sink run after unlock; five concurrent emitter classes exist → NDJSON + SSE
  can carry locally non-monotonic seq (in-memory transcript stays ordered). Widen the critical
  section or serialize dispatch per agent.
- **ADVISORY — the reconcile watcher re-reads and re-parses EVERY session's ENTIRE transcript on
  EVERY `sessions/` fsnotify write, with no debounce** (`server/reconcile.go`) — O(all
  transcripts) work per streamed append during active multi-agent sessions.
- **ADVISORY — `PUT /api/backends` cred checks run sequentially, 6s timeout each**
  (`config_handlers.go:476-485`; UI Save blocks on it) — Settings Save can hang 6s×N with one
  unreachable backend. Parallelize.
- **ADVISORY — every chat permission prompt double-notifies** (`permission.go:61-62`:
  waiting_input status edge + permission_required event always fire together → two stacked
  toasts; muting one type doesn't suppress its twin). Collapse or make one type authoritative.
- **ADVISORY — docs/install drift for a fresh user:** README quickstart omits that `install.sh`
  defaults `INSTALL_ACP=0` (a fresh install cannot launch a chat agent until the adapter is
  installed) and never lists `jq`/`curl` (required by terminal hooks, which are ON by default for
  terminal agents); `MAP.md` still says the messaging MCP is stdio (shipped transport is HTTP
  `/mcp`); `architecture-flow.md`'s diagram shows terminal→bus event parity that doesn't exist.
- **ADVISORY — Files and Commands tabs can show the wrong agent after a quick switch.**
  `ui/src/components/chat/FilesTab.tsx:48-56` and `ui/src/components/chat/CommandsTab.tsx:35-43`
  reuse one `mountedRef` across `agentId` changes, so a slower request from the previous agent can
  land after the new effect has flipped the flag back to `true` and overwrite the current tab with
  stale rows. Fix: tie each fetch to the requested `agentId` or cancel it with an `AbortController`;
  test: start loading agent A, switch to B before A resolves, and assert A's late response does not
  replace B's list.
- **ADVISORY — Release group failures are silent.** `ui/src/components/grid/CardGrid.tsx:88-94`
  fires `releaseGroup()` without a catch or toast, so a 500/409 leaves the user with no indication
  that the group stop did not happen. That breaks the normal task-group workflow because the button
  appears to succeed even when nothing changed. Fix: await the call and surface the server error
  through the existing toast path; test: mock a rejected release and assert the UI shows an error.
- **ADVISORY — New Agent drafts never reset between launches.** `ui/src/features/launch/NewAgentModal.tsx:35-43`
  and `ui/src/features/launch/useSuggestedName.ts:17-28` keep the last modal's local form values
  and `dirtyRef` alive across close/reopen, so a canceled or completed launch reopens with stale
  role/project/backend/model/interface/name state instead of current defaults and a fresh name
  suggestion. Fix: reset the draft on `open` transitions or remount the dialog per launch; test:
  edit the name/role, close the modal, reopen, and expect the current default suggestion.

### Cross-project observations (2026-07-04 holistic pass — guidance, not findings)

1. **The repeat failure mode is stale state crossing a boundary without reset or republish.** Confirmed
   instances span SSE reconnect/hydration, permission races, unread badges after read/delete, nudger
   relaunch cooldown, and terminal switch handoff. Fix direction: make reconnect/relaunch/read/switch
   boundaries explicit generation changes, and require every durable state mutation that affects cards to
   publish/touch the affected agent immediately.
2. **Terminal remains vulnerable where it depends on chat-shaped contracts.** Recent fixes made PTY
   fan-out, persistence, and stop behavior much better, but switch primer delivery and Codex hook
   registration still assume chat/ACP mechanisms that terminal does not actually use. Treat terminal as a
   first-class runtime in launch composition, hook registration, status production, and capability gating.
3. **Config/UI writes need a loaded-source precondition.** The BackendStep blocker is the sharpest current
   example: whole-document writes composed from pre-query placeholders can destroy seeded state. Settings
   and onboarding components should either block until the authoritative query has loaded or merge on the
   server against the persisted document.
4. **Biggest real-world acceptance risk remains gated live-CLI behavior.** Codex terminal hooks, Codex chat
   live acceptance, Claude/Codex HTTP MCP registration, interactive resume forms, and optional driver
   support should be burned down before Phase 7 feature work expands the surface.

## Autonomous decisions (please review)

> Resolved without stopping; the human should still see them. Remove once acknowledged (workflow §3, §5).

- **NEW (7.1): the terminal-rejection gate was ADDED to the resume composer, which previously had none.** The spec
  (§4) says `terminalSupported` must guard all three composers (launch/switch/resume), but before 7.1 only launch and
  switch rejected codex-terminal — `composeResumeSpec` had no such gate. So `terminalSupported` in resume is a genuine
  behavior addition: a resume whose override sets `interface:terminal` on a non-claude backend now returns `422
  terminal_unavailable` instead of composing a statusless spec. Spec-directed, but flagging because it closes a hole
  that wasn't previously gated. **To reverse:** drop the gate at the top of `composeResumeSpec` (re-opens the hole).
- **NEW (7.1): `ExtraEnv` takes primitives `(modelID string, skipPerms bool)`, not a `LaunchSpec`.** The spec sketches
  it as `ExtraEnv(spec)`, but `internal/backend` must not import `internal/runtime` (the runtime imports backend — a
  cycle), so the optional `ExtraEnvProvider` interface takes the two launch fields it needs. `chat.go::spawnCmd` passes
  `spec.ModelID, spec.SkipPerms`. **To reverse:** move the adapter into package `runtime` and pass the whole spec.
- **NEW (7.2): OpenHands skip=true (yolo) is honored by the shared runtime auto-approve gate, NOT a CLI-side injection.**
  §2.3 sketches two arms (ACP session-mode at `session/new`, or a CLI approval flag). The session-mode arm needs a change
  to the shared `sessionNewParams` — forbidden by §1's "no runtime changes" rule, and claude's path doesn't select a
  mode (the §2.3 conditional that gates that arm is false). The CLI flag arm needs the unverified `openhands` ACP
  always-approve flag AND a spec-aware `LaunchArgs`. Meanwhile the runtime permission gate (`permission.go`) already
  auto-approves every request when `SkipPerms` is true, backend-agnostic — so OpenHands yolo is functionally correct
  today. `openhandsACP.ExtraEnv` documents this; the CLI-side arm is GATED to 7.4. **Tradeoff vs. OpenCode:** OpenCode
  yolo is pushed into the CLI via `OPENCODE_CONFIG_CONTENT` (env, per spec) so it never even raises requests; OpenHands
  round-trips each request through the gate. **To reverse:** once 7.4 confirms the flag/mode, add it (flag arm via a
  spec-aware LaunchArgs, or mode arm only if the "no runtime changes" rule is relaxed).
- **NEW (7.2): PUT type-union rejection reuses the existing `unknown_backend_type` validation error, not a new
  `invalid_field`.** §3 says "400 invalid_field", but `ValidateBackendsConfig` already rejects unknown types with code
  `unknown_backend_type` (the established, tested pattern). I widened `knownBackendTypes` to the four-value union and
  kept the existing code/mechanism rather than adding a parallel `invalid_field` path. This also fixes a latent bug: the
  new seeds would otherwise fail their own validation. **To reverse:** rename the code string if `invalid_field` is
  required for the UI.
- **NEW (7.1): OpenCode/OpenHands each seeded with one model key `sonnet-4-5` → `anthropic/claude-sonnet-4-5`.** The
  spec names the default model value but not the map key; I used a short key. OpenHands seeds empty `LLM_API_KEY`/
  `LLM_BASE_URL` env so Settings shows the fields. Neither changes the default backend (`claude`). **To reverse:**
  rename the key or add more seeded models.

- **NEW (6.7): went beyond the checkpoint's minimum — full driver-selection plumbing, not just the 422 gate.**
  6.7's "done when" only strictly required an explicit unavailable `driver:"iterm2"` request to return `422`. I also
  plumbed the `driver` field through launch + switch into per-agent runtime dispatch (`runtime.LaunchSpec.Driver`;
  `Runtime.driverFor`; each `termAgent` carries the driver that launched it, so `WriteText`/`CloseTab` dispatch
  correctly). **Why a judgment call:** accepting a `driver` field but ignoring it (always launching xterm) would be a
  new silent lie — exactly the standing advisory that tmux was "advertised but unselectable." Doing it properly makes an
  accepted non-xterm driver actually launch, and closes the API/runtime half of that advisory as a side effect.
  **Tradeoff:** tmux is now launchable via the public API (previously only via the `SetDriver` test override), and the
  runtime holds a driver per agent instead of one global driver. The UI still sends no driver (no picker) and
  `config.terminal.max_tabs` is still unimplemented — both remain in the trimmed advisory. **To reverse:** revert to a
  single `r.driver` and validate-only in the server (drop `spec.Driver` threading + `driverFor`).
- **NEW (6.7, GATED): iTerm2 AppleScript templates/verbs are unverified against a live iTerm2.** The escaping
  (`escapeAppleScript`: `\`→`\\`, `"`→`\"`, newlines→`" & return & "`), the shell-quote layer (`shellJoin`/`shellQuote`,
  reused from tmux), the color scaling (0–255→0–65535), template rendering, and the capability gating ARE tested. The
  live AppleScript semantics (`create window`, `session id` addressing, `write text`, `close`) are NOT — same
  credential-gated class as every other Phase 6 terminal-CLI behavior (real CLI login, Codex hooks, `--settings`).
  `osascript` runs via `osascript -` (script over stdin) with a 4s timeout + stderr capture. On non-macOS the driver is
  never invoked (the `422` gate rejects it first). **To reverse/refine:** adjust the three templates in
  `applescript.go` once verified on a real macOS + iTerm2 host.
- **NEW (6.7): iTerm2 background color is left unset (title only).** `TabSpec.Color` is populated from nothing —
  `runtime.LaunchSpec` carries no project accent color, so `set-appearance` sets only the session name and skips
  `background color` (driver.go already documents "zero value means unset"). Threading `project.color` through the
  LaunchSpec was out of 6.7 scope. **To reverse/complete:** add the project color to `LaunchSpec`→`TabSpec.Color`; the
  scaling + the color-gated template branch already handle the rest. Also minor: switch **rollback** re-launches the
  previous identity under the default (xterm) driver, since the prior driver isn't persisted in the session snapshot —
  correct for the only shipped default, revisit if a non-xterm driver must survive a rollback.

- **NEW (review fix, finding 4 — terminal LaunchSpec contract): hybrid fix — HONOR claude terminal fields, REJECT
  codex terminal with 422.** The finding offered two arms (honor via adapter, or 422-reject); I split by backend
  because the techspec (line 140) requires terminal launches to succeed and switch requests carry a model, so
  full-reject was not viable. (1) **claude terminal**: `terminal/launchArgv` now appends the documented Claude Code
  CLI flags `--model`, `--add-dir` (per dir), `--append-system-prompt` (from `StartSystemPrompt()`, so the switch
  primer rides along) — GATED-labeled like the existing `interactiveBinary`/`--resume`/`--settings` (real flags,
  unverified against a live login). (2) **codex terminal**: rejected at BOTH launch (`composeLaunch`) and switch
  (`validateSwitchTarget`, which runs before any teardown) with `422 terminal_unavailable` — codex's interactive CLI
  has no verified hook-registration path (status never flows) and its flags differ from claude's, so landing one
  produces a statusless agent that drops the spec. (3) **messaging MCP**: deliberately NOT wired into the terminal
  CLI — terminal agents are already non-messageable (`ResolveRecipient` excludes them, prior decision), so wiring
  send-only tools would be inconsistent and rests on the unverified `--mcp-config` flag; the in-process registration
  is left as-is (idle, teardown-symmetric). **Why judgment calls:** the arm split, the exact CLI flags, and rejecting
  a codex-terminal combo the spec shows as an example (line 352) are all decisions the finding left open. **Tradeoffs:**
  claude-terminal flags are unverified against a live CLI (same gate class as all Phase 6 terminal CLI behavior); codex
  terminal is now unavailable until its live hook/flag surface is confirmed. **To reverse:** drop the codex-terminal
  guards + implement codex's verified hook registration and CLI flags; or move the claude flag mapping behind an
  adapter method once the live CLI surface is known. Tests: `TestLaunchArgvHonorsComposedSpec`, `TestCodexTerminalRejected`.

- **NEW (review fix, findings 8+9): terminal PTY hub design choices the findings left open.** (1) **Scrollback ring
  = 256 KiB** per agent (a documented constant) — bounds per-agent memory while covering a screenful+context on
  attach. (2) **Slow-subscriber overflow = drop-and-close that one subscriber** (per-sub buffer 256 chunks), NOT
  drop-oldest: a mid-stream byte drop would corrupt the xterm render, so a clean cut + browser reconnect (fresh
  scrollback replay) is safer, and it keeps the always-on reader non-blocking (load-bearing for Finding 9). (3) **UI
  default-tab** for terminal agents implemented as a pure `initialTab(tabParam, interface)` lazy `useState` initializer
  PLUS a one-shot `useRef`-guarded effect that applies the terminal default once the agent hydrates over SSE (the agent
  often isn't in the store at mount); it never overrides an explicit `?tab=` or a manual switch. **To reverse:** tune
  the two constants; switch overflow to drop-oldest; or gate the terminal default differently.
- **NEW (review fix, finding 4): terminal agents are made NON-messageable (excluded from `ResolveRecipient`),
  rather than wiring full terminal messaging.** The finding offered two arms: (a) pass `--mcp-config` to the
  terminal CLI so it gets the `check_messages` tool, or (b) exclude terminal agents from recipient resolution until
  wired. I chose (b): a terminal agent now resolves to `ErrRecipientNotFound` by every address form, which
  definitively stops the paid-turn nudge loop without depending on the unverified `--mcp-config` CLI flag (same gated
  class as `--settings`) or adding a nudger give-up. **Tradeoff:** terminal agents can no longer receive messages from
  other agents at all (they still appear in `list_agents`, just can't be mailed) — a real feature reduction vs. wiring
  arm (a). **Why a judgment call:** arm (a) is the fuller feature but rests on a credential-gated CLI flag AND still
  needs a nudge give-up (a separate open advisory) to be safe; arm (b) is the robust BLOCKING-closer. **To reverse:**
  drop the terminal filter in `ResolveRecipient`, pass `--mcp-config` in terminal `launchArgv` (gated), and add the
  nudger give-up (advisory) so a terminal agent that can't drain mail isn't nudged forever.
- **NEW (review fix): Clone is a direct clone-launch, not a prefilled NewAgentModal.** The advisory offered two arms —
  (a) open `NewAgentModal` prefilled from the agent's role/project/backend/model, or (b) retitle/de-scope. I did neither
  literally: I wired Clone to POST `/api/sessions` directly with the source agent's config (role/project/backend/model/
  interface/group), server auto-suggesting the name — a functional Clone matching the existing prompt/direct-API context
  menu style (consistent with the 6.6 decision that context-menu actions use direct API calls, not a modal subsystem).
  **Why a judgment call:** arm (a) would need new modal props (`initialBackend`/`initialModel`/`initialInterface`) +
  open-state plumbing from the context menu; the direct launch is smaller, immediate, and clones with zero extra clicks.
  **Tradeoff:** no pre-launch review/edit of the cloned config (it launches on click); a mistaken clone must be stopped.
  **To reverse/fix:** add the prefill props to `NewAgentModal` and open it from the context menu instead of launching
  directly, if a confirm-before-launch step is wanted.

- **NEW (review fix): removed the (dead, unimplemented) stdio-MCP fallback scaffolding.** The 5.3 decision left a
  stdio branch in `registerMessagingMCP` behind a constant-true `usesHTTPMessagingMCP`, as a placeholder for the gated
  live two-CLI HTTP-vs-stdio verdict. The dead-code review flagged it; I removed it (branch + function + the now-unused
  `backendType` param) because it was unreachable AND non-functional — the `agentdeck mcp` stdio subcommand it pointed
  at doesn't exist, so it would fail at runtime if ever hit. **Why a judgment call:** it deletes intentional gated
  scaffolding rather than leaving it. The gate itself remains open in "Blocked on human" (live CLI HTTP acceptance).
  **To reverse:** if a real CLI rejects HTTP, re-add a stdio branch AND implement the `agentdeck mcp` proxy subcommand.
- **NEW (review fix, REVERSES the prior "re-resolve" decision): skip_permissions/add_dirs are now FROZEN into the
  session snapshot at launch and read back verbatim on resume+switch.** A prior fix agent had deliberately chosen to
  re-resolve both from the *current* role/project config (via `resolveSkipForRole`/`resolveAddDirs`), documented here as
  a pending-review autonomous decision. The fifth review re-flagged that as a BLOCKING spec violation, and it is: the
  techspec (§12.4, and repeated at spec lines 305/326/460/488) plus the master-PRD invariant say resume must reproduce
  the *frozen composed config*, with only env **values** (§8.7) and MCP servers as documented exceptions — skip/add_dirs
  are not exempt. Re-resolution was also the root of the (now-moot) "delete-a-role flips skip on resume" safety advisory.
  So I reversed it: migration v7 adds `sessions.skip_permissions`/`add_dirs`; `SessionMetaData`+`runtimeMeta` carry them;
  `UpsertSessionMeta` persists them; `SessionSnapshot`/`ReadSession`/`ListInactiveSessions` read them; the resume/switch
  composers now use `snap.SkipPermissions`/`snap.AddDirs`; the dead `resolveSkipForRole`/`resolveAddDirs` were removed.
  Pre-v7 rows default to skip=0 (fail closed — never auto-approve) and no extra dirs. **Why still a judgment call:** it
  overrides another agent's documented decision (the review is the tiebreaker, and the spec is unambiguous). **To
  reverse:** re-introduce the resolvers and call them in the composers — but that re-violates §12.4.
- **NEW (review fix): adopted xterm.js for the terminal panel — two new UI deps (`@xterm/xterm`, `@xterm/addon-fit`).**
  The advisory asked for the spec's task-13 xterm.js panel (replacing the hand-rolled `<pre>` + input). I integrated the
  real emulator: `TerminalTab` now mounts `Terminal` + `FitAddon`, pipes `onData`→binary frame and `onResize`/fit→`{cols,rows}`
  text frame, and writes PTY bytes via `term.write`. **Why a judgment call:** it adds two runtime dependencies and grows the
  bundle (the build already warns >500 kB); I judged that acceptable since it's the specified terminal experience and resolves
  the never-sent-resize gap. The component test mocks the xterm modules (xterm needs canvas measurement jsdom lacks) and drives
  `onData`/`onResize` to assert the binary-keystroke / text-resize contract. **To reverse:** restore the line-box `<pre>` panel
  and drop the two deps — but then ANSI renders literally and the PTY size is never set.
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

- 2026-07-09 — **phase 7.3: OpenCode/OpenHands UI plumbing — green.** `schemas/backends.ts` now exports
  `backendTypeSchema`/`BackendType` (four-value enum — the single source of the union); new `lib/backendTypes.ts`
  (`BACKEND_TYPE_LABELS`, `BACKEND_TYPE_OPTIONS`, `terminalSupported`) replaces the per-component type ternaries.
  BackendStep + BackendsEditor render options from the enum+labels; BackendStep adds `LLM_API_KEY`/`LLM_BASE_URL`
  fields for `openhands-acp`. NewAgentModal gates the Terminal interface on `terminalSupported(backend.type)` (only
  claude) in addition to host capability, and resets a stale terminal selection to chat on backend change. Embedded
  dist refreshed (index.html). Tests (+4, 78 total): editor four-option dropdown, BackendStep openhands fields, modal
  terminal-disable + reset-to-chat. Green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`,
  `cd ui && npm run test` + `npm run build`.

- 2026-07-09 — **phase 7.2: OpenCode/OpenHands permissions + credchecks + switch matrix — green.** OpenCode skip=true
  → `opencodeACP.ExtraEnv` injects `OPENCODE_CONFIG_CONTENT` yolo config (env-only, torn down with the process);
  OpenHands skip is honored by the shared runtime auto-approve gate (CLI-side arm GATED — see Autonomous decisions).
  New `credcheck/opencode.go` + `openhands.go` (best-effort: installed binary + auth.json/provider-key or
  LLM_API_KEY/settings.json → ok, else skipped; never "failed"), registered in the prober map; shared `lookPath`/
  `homeDir`/`fileExists`/`hasProviderAPIKey` helpers in `env.go`. Widened `config.knownBackendTypes` to the four-value
  union (rejects unknown types at PUT /api/backends AND lets the new seeds validate). Tests:
  `TestSkipPermissionsEnvOpenCode` (spawned-env), `TestOpenCodeProber`/`TestOpenHandsProber` (fs/env-faked),
  `TestValidateBackendsConfig_NewBackendTypesAccepted`, `TestSwitchClaudeToOpenCodePrimer`. Green: `go build ./...`,
  `go test ./...`, `go test -tags sqlite_fts5 ./...`.

- 2026-07-09 — **phase 7.1: OpenCode/OpenHands adapters + config + terminal gates — green.** Added `opencodeACP`
  and `openhandsACP` to `internal/backend/adapter.go` (both `<bin> acp`, hookless → chat status from the ACP stream,
  native `session/load` resume attempt with primer floor; GATED for live CLI). New optional `ExtraEnvProvider`
  interface, applied in `chat.go::spawnCmd` after `StripEnvKeys` — OpenHands sets `LLM_MODEL` (model rides env; also in
  its StripEnvKeys so a shell value can't leak). Seeded `opencode`/`openhands` backends (`config/seed.go`), widened the
  `Backend.Type` union comment. `terminalSupported` helper (`server/terminal.go`) replaces the three `codex-acp`
  literals in the launch/switch/resume composers — and adds the previously-missing gate to `composeResumeSpec`. Tests:
  `TestNewBackendAdapters`, `TestOpenHandsExtraEnvCarriesModel`, `TestOpenCodeChatE2E`, `TestOpenHandsChatE2E`,
  `TestNewBackendTerminalRejected`. Green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`,
  `go test -race ./internal/runtime`.

- 2026-07-09 — **phase 6.7: iTerm2/AppleScript driver + driver-selection plumbing — green (Phase 6 complete).**
  New `internal/runtime/terminal/iterm2.go` (`iterm2Driver` via `osascript -`, 4s timeout + stderr capture, best-effort
  title/color) + `applescript.go` (three `text/template` templates; mandatory `escapeAppleScript` + reused `shellJoin`
  shell-quote layer; 0–255→0–65535 color scale). `probeITerm2` now advertises the driver on macOS+iTerm. Wired a
  `driver` field through launch (`launchRequest`) + switch (`switchRuntimeRequest`) → `runtime.LaunchSpec.Driver` →
  per-agent runtime dispatch (`Runtime.driverFor`, `termAgent.driver`), with `validateTerminalDriver` returning `422
  terminal_unavailable` + reason for any unavailable driver (closes the API/runtime half of the "tmux unselectable"
  advisory). Tests: `applescript_test.go` (escaping quotes/backslashes/newlines, argv shell-quoting, color scale,
  injection-defeat, template rendering), `TestTerminalDriverUnavailableRejected` (launch+switch 422+reason), updated
  `TestProbeXtermAlwaysAvailable` (GOOS-guarded iterm2 contract). Green: `go build ./...`, `go test ./...`, `go test
  -tags sqlite_fts5 ./...`, `go test -race ./internal/runtime/terminal`. GATED: live AppleScript semantics unverified
  (see Autonomous decisions).

- 2026-07-07 — **review fix: advisory batch (inbox newest-N + CLI operand validation) — green.** Two ADVISORY,
  both confirmed real. (1) Invariant §7: the inbox endpoint returned the OLDEST N when the mailbox exceeded
  `limit` (`ListMessages` did `ORDER BY created_at ASC LIMIT`, then the handler reversed). Switched `ListMessages`
  to `ORDER BY created_at DESC, message_id DESC` (newest N) and dropped the handler's now-redundant reversal —
  the endpoint still presents newest-first and truncation now keeps recent mail. Test: `TestListMessagesOrderingAndLimit`
  now asserts the newest N with the oldest dropped. (2) `internal/cli/launch.go`: value-taking flags (`--resume`,
  `--model`, …) took `""` when given last or before another flag, so `impl@proj --resume` silently fell through to
  a fresh launch; they now require a non-flag operand or error. Test: `TestParseLaunchErrors` missing-operand cases.
  Green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`.
- 2026-07-07 — **review fix: graceful shutdown ends open SSE streams — green.** BLOCKING, confirmed real
  (invariant §9 — liveness/lifecycle primitives are weaker than they look; `http.Server.Shutdown` waits for
  in-flight requests but never cancels their contexts). The `/api/events` SSE handler blocks on `<-ctx.Done()`,
  so a single open dashboard tab held `Server.Start` for the full `shutdownTimeout` (5s) and then the CLI fell
  back to an ungraceful kill. Gave the `http.Server` a cancelable `BaseContext` and cancel it just before
  `srv.Shutdown`, so every in-flight request context (incl. SSE) is Done and the handlers return immediately.
  Regression: `TestStartShutsDownWithOpenSSEClient` (verified: 4.1s timeout-fail without the cancel, 0.1s with;
  `-race` clean). Green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`.
- 2026-07-07 — **review fix: terminal honors the composed LaunchSpec; codex terminal rejected — green.** BLOCKING,
  confirmed real (invariant §6 — a new runtime must join the LaunchSpec contract + capability honesty). The terminal
  runtime's `launchArgv` built the CLI invocation from argv/env only, silently dropping the composed model, add_dirs,
  and system prompt/primer. `composeLaunch` composes them correctly (shared §2 helper); the gap was purely in the
  terminal runtime. Fix (hybrid, see Autonomous decisions): claude terminal now passes `--model`/`--add-dir`/
  `--append-system-prompt`; codex terminal is rejected at launch + switch with `422 terminal_unavailable` (no verified
  hook/flag path — also resolves the "codex terminal status has no registration path" half); messaging MCP stays
  intentionally unwired (terminal is non-messageable). Tests: `TestLaunchArgvHonorsComposedSpec`,
  `TestCodexTerminalRejected`. Green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`.
- 2026-07-07 — **review fix: SSE onopen is a fresh hydration generation — green.** BLOCKING, confirmed real
  (invariant §1 — reset connection-scoped state in `onopen`, not the constructor). `ui/src/api/sse.ts` reset
  `lastPing`/`hydrationIds`/`lastAgentSeq` only when `!hydrating`, so the browser's automatic `EventSource`
  reconnect (fires `onopen` again on the same object; the server re-sends a full snapshot + `hydrated` every
  connection) inherited stale state: a drop mid-hydration unioned the partial snapshot's IDs into the next
  `hydrateComplete` so a server-deleted agent survived forever, and a stale `lastPing` let the watchdog reap the
  freshly-reopened stream before its first ping. Now every `onopen` unconditionally resets liveness + starts a
  new hydration generation; removed the now-dead `hydrating` field. Regression: `sse.test.ts` "resets the
  hydration generation on auto-reconnect so deleted agents are pruned" (verified failing before the fix). Green:
  `go build ./...`, `go test ./...`, `-tags sqlite_fts5`, UI 74/74 + `npm run build`, embedded dist refreshed.
- 2026-07-07 — **review fix: freeze skip_permissions/add_dirs in the session snapshot — green.** BLOCKING,
  confirmed real (invariant §3 — persisted fields must not be re-derived from live config; §2 — resume/switch
  compose through the frozen snapshot). Resume/switch re-resolved `SkipPerms`/`AddDirs` from the *current*
  role/project, so a config edit after launch silently changed a resumed agent's permission policy or dirs —
  violating techspec §12.4's frozen-snapshot rule. This **reverses a prior autonomous decision** that chose
  re-resolution (see Autonomous decisions). Migration v7 adds `sessions.skip_permissions`/`add_dirs`; the values
  flow launch → `SessionMetaData`/`runtimeMeta` → `UpsertSessionMeta` → `SessionSnapshot`; the composers read
  `snap.*`; removed the dead `resolveSkipForRole`/`resolveAddDirs`. Also closed two advisories in passing:
  the "delete-a-role flips skip on resume" safety advisory (moot once skip is frozen) and the `migrate.go`
  `rows.Err()`/hand-maintained `latestKnownMigration` residue (added the `rows.Err()` check; derived the
  migration floor from the slice so it can't drift). Regression: `TestResumeAndSwitchUseFrozenSkipAndAddDirs`
  (verified failing when the composer reads live config). Green: `go build ./...`, `go test ./...`,
  `go test -tags sqlite_fts5 ./...`.
- 2026-07-07 — **review fix: reindex preserves the final partial turn — green.** BLOCKING, confirmed real
  (invariant §7 — the read-path repair losing the final partial turn, already listed there). `reindexAgent`
  (`internal/index/reindex.go`) flushed each completed turn but only ran the post-loop flush when NO `turn_end`
  was ever seen (`!sawTurnEnd`), so a transcript with turn 1 completed + turn 2 crash-truncated left turn 2's
  assistant text only in the in-memory buffer — dropped from `sessions_fts`. Replaced the `sawTurnEnd` gate with
  a `pendingFlush` dirty flag (set on every event, cleared after each `OnTurnEnd`) so a final flush also fires
  when a completed turn is followed by a partial one. Regression: `TestReindexPreservesFinalPartialTurn`
  (verified failing before the fix). Green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`.
- 2026-07-04 — **review fix: onboarding backend validation race (finding 3) — green.** BLOCKING,
  confirmed real: the onboarding Validate button stayed enabled while `/api/backends` was still
  loading, so an immediate click could still compose from placeholder state before the seeded
  backend document arrived. `BackendStep` now gates validation on the backend query being loaded,
  reuses the loaded seeded backend identity in the submit path, and adds a delayed-load regression
  test proving the button stays disabled and no premature PUT is sent before prefill completes.
  Regression coverage: `BackendStep.test.tsx` delayed-load case + existing merge-preserve case.
  Green: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`, `cd ui && npm run build`.
- 2026-07-04 — **review fix: permission resolution race (finding 1) — green.** BLOCKING,
  confirmed real: `Permission()` and `onPermissionTimeout()` each loaded the pending request before
  deleting it, so concurrent approve/deny/cancel/timeout paths could both believe they won and emit
  conflicting transcript state. Fixed by making "take the pending request" the atomic step
  (`takePending`), resolving through the claimed request only, restoring the pending entry on invalid
  decisions, and surfacing `ErrPermissionAlreadyResolved` as a 409 instead of fabricating
  `resolved:true`. Regression coverage: `TestTakePendingSingleWinner`,
  `TestTakePendingReportsAlreadyResolved`, and server mapping coverage in
  `TestPermissionErrorAlreadyResolved`. Green: `go build ./...`, `go test ./...`,
  `go test -tags sqlite_fts5 ./...`.
- 2026-07-03 — **review fix: switch primer one-shot (finding 6) — green.** BLOCKING, confirmed real (reproduced: the
  frozen snapshot absorbed the primer; a second cross-backend switch stacked another). Decoupled "prompt fed to the
  backend process this launch" from "prompt persisted to the frozen snapshot": new `LaunchSpec.RuntimeSystemPrompt`
  (one-shot, NOT persisted) + `LaunchSpec.StartSystemPrompt()`; `session/new` + `session/load` params now use
  `StartSystemPrompt()`, while `runtimeMeta`/`UpsertSessionMeta` still snapshot the pristine `SystemPrompt`. The
  switch backend-swap path sets `spec.RuntimeSystemPrompt = join(SystemPrompt, primer)` instead of mutating
  `spec.SystemPrompt`, so `sessions.system_prompt` stays pre-primer and successive switches prime from the clean base
  (no stacking). Test: `TestSwitchRuntimePrimerKeepsFrozenSystemPrompt` (primer switch → snapshot unchanged; second
  switch → still no primer). Green: `go build/test`, `-tags sqlite_fts5`.
- 2026-07-03 — **review fix: onboarding BackendStep merge-preserve (finding 10) — green.** BLOCKING, confirmed real
  (reproduced: an untouched submit persisted `models:{default:{model:"default"}}`, wiping the seeded models —
  every later launch on that backend then fails). `ui/src/features/onboarding/steps/BackendStep.tsx` now pre-fills
  `modelKey`/`modelName`/`modelStr` from the seeded backend's default model (a `useEffect` keyed on the loaded
  config), and on submit MERGES `...(seeded.models)` under the one edited key instead of replacing the map (also
  preserves the seeded `name`/`env`). An untouched submit round-trips the real seeded models, never the "default"
  placeholder. Test: `BackendStep.test.tsx` — seeded two-model backend, untouched Validate&Continue, asserts the PUT
  payload retains both seeded models and no `model:"default"`. Green: UI 72/72 + `npm run build` + dist refreshed.
- 2026-07-03 — **review fix: terminal PTY broadcast hub (findings 8+9, + fd-leak advisory) — green.** BLOCKING×2,
  both confirmed real, ONE architectural fix. New `internal/runtime/terminal/ptyhub.go` `ptyHub`: a per-agent
  broadcast hub with a single always-on reader goroutine that drains the PTY master from launch (not on WS attach)
  into a bounded 256 KiB scrollback ring and fans out to N subscribers via NON-blocking sends — replaces the per-WS
  `dup()` (deleted `dupPTYMaster`/`ptyMaster`). Fixes Finding 9 (unobserved agent no longer stalls on a full tty
  buffer: reader always drains; a slow subscriber is dropped-and-closed, never blocks the reader) and Finding 8
  (all viewers now see IDENTICAL bytes — subscribe snapshots scrollback + registers under one lock, no split/dup).
  `Bridge` now returns a `hubConn` subscriber (scrollback-then-live Read; Write/Resize→shared master; Close only
  unsubscribes). Hub lifecycle wired into Start/Resume (+failure teardown), Stop, and the crash watcher alongside
  the existing `closePersistence`; Stop still keeps the sessions row. UI: `ChatPanel` now defaults the active tab to
  Terminal for terminal-interface agents (`initialTab` pure fn + hydration effect). Also fixed the advisory PTY-fd
  leak (`server/terminal.go` closes the conn on the `websocket.Accept` error path). Tests:
  `TestPTYHubBroadcastsIdenticalBytesToAllSubscribers`, `TestPTYHubDrainsWithNoSubscriberThenReplaysScrollback`,
  `TestPTYHubCloseUnblocksReaderAndSubscribers`, `TestPTYHubDropsSlowSubscriber`, 3 `ChatPanel` initialTab cases.
  Green: `go build/test`, `-tags sqlite_fts5`, UI 71/71 + `npm run build`, embedded dist refreshed.
- 2026-07-03 — **review fix: terminal lifecycle trio (findings 7/4/5) — green.** BLOCKING×3, all confirmed real.
  (7) terminal-origin agents now get a `sessions` row + transcript: `terminal.Runtime.SetPersistence`/`openPersistence`
  mirror the chat runtime (wired in `server.go` via a `transcript.Open` adapter + indexer); `Start`/`Resume` open
  persistence before `WriteRunning` (teardown on failure), `Stop` KEEPS the archive row (only `DeleteRunning`), and the
  crash watcher + failure paths close the writer — terminal agents are now archive-visible and resumable. Exported
  `runtime.NewSessionMeta`. (5) Stop-on-unowned-agent no longer silently orphans a live process: both `!ok` branches
  (`chat.go`, `terminal.go`) call `reconcileOrphanStop` — read the running row, and if the recorded pgid is alive
  SIGTERM→(5s grace)→SIGKILL `-pid` before `DeleteRunning` (kill, not 404, since reconcile never re-adopts live PIDs).
  Exported `runtime.PidAlive`. (4) terminal nudge-loop closed at the choke point: `ResolveRecipient` now excludes
  `interface=="terminal"` agents (added `Interface` to `LiveAgent`/`LiveAgents` query) so a terminal agent is never
  handed mail and the nudger never targets it (terminal agents stay visible in `list_agents`, just unmailable until
  full terminal messaging is wired). Tests: `TestTerminalStartCreatesSessionRowSurvivingStop`,
  `TestChatStopKillsOrphanedLiveProcess`, `TestTerminalStopKillsOrphanedLiveProcess`,
  `TestResolveRecipientExcludesTerminalAgents`. Green: `go build/test`, `-tags sqlite_fts5`.
- 2026-07-03 — **review fix: transcript durability trio (findings 1–3) — green.** BLOCKING×3, all confirmed real.
  (1) `transcript/reader.go`: `readAll` replaced the 8 MiB `bufio.Scanner` (which aborted the whole file on
  `ErrTooLong`) with a `bufio.Reader`+`readLine` loop that SKIPS an oversized record and stays aligned to the
  next — a big diff/tool_result no longer 500s `/transcript` or bricks resume/reindex. (2) `transcript/writer.go`:
  new `truncateToLastNewline` runs in `Open` before the O_APPEND, removing a torn trailing partial line so the
  next `Append` can't byte-fuse onto it (well-formed logs untouched). (3) `index/reindex.go`: per-agent isolation
  via `reindexAgent` + `errors.Join` — a single unreadable transcript is skipped-and-reported, the good agents
  stay reindexed (was: wipe-all-then-abort). Tests: `TestReadAllSkipsOversizedRecord`,
  `TestReadAllOversizedTrailingRecordNoAbort`, `TestReaderRecoversMaxSeqPastOversized`,
  `TestOpenTruncatesTornTrailingLine`, `TestOpenLeavesWellFormedLogUntouched`, `TestReindexIsolatesBadAgent`.
  Green: `go build/test`, `-tags sqlite_fts5`. (Note: with 1+3 in place, in-content corruption is now always
  tolerated per techspec §8.1; the only remaining reindex-abort trigger is an I/O-level failure, which isolation contains.)
- 2026-07-02 — **review fix: Clone context-menu action wired (was a dead "Available in Phase 3" stub) — green.**
  ADVISORY: `CardContextMenu`'s Clone button was permanently `disabled` with a stale Phase-3 tooltip though Phase 3
  shipped the launch flow. Wired it to `launchAgent(...)` (new `api/client` helper) — a clone launches a new agent with
  the source's role/project/backend/model/interface/group and lets the server auto-suggest a name; failures surface a
  "Clone failed" toast. Tests: two new `CardContextMenu` tests (correct POST payload + error toast). UI 68/68,
  `npm run build`, embedded dist refreshed. See Autonomous decisions (direct clone-launch vs. prefilled modal).
- 2026-07-02 — **review fix: every launch/resume failure path routes through `teardownAgentRegistration` — green.**
  ADVISORY: (1) `handleLaunch`'s `WriteAgent`-failure path returned without cleaning the token/MCP/hook-settings that
  `composeLaunch` had already created; (2) `handleResume`'s Resume-failure path cleaned token+MCP but leaked the
  hook-settings file `composeResumeSpec` wrote. Both now call `teardownAgentRegistration` (all three artifacts); the
  launch Start-failure path was unified onto it too. Test: `TestResumeFailureRemovesHookSettings` (verified failing
  before the fix). Green (Go-only).
- 2026-07-02 — **review fix: list scans check `rows.Err()` not `rows.Close()` — green.** `ListInactiveSessions`
  (`state/session.go`), `queryTrackedFiles`/`queryTrackedCommands` (`server/files_commands.go`) ended their scan loops
  with `rows.Close()` (already deferred), which does not surface a mid-iteration error — a failed row scan would return
  a silently truncated list as success (archive resume matching / Files & Commands tabs could miss rows). All three now
  check `rows.Err()`. No bespoke test (a mid-iteration scan fault isn't deterministically forceable; happy-path list
  tests cover the return). Green (Go-only).
- 2026-07-02 — **review fix: `dashboard start --detach` refuses when a live server holds the pidfile — green.**
  BLOCKING: the detach parent re-exec'd its daemon child *before* the already-running liveness check (which the child
  also skips), so `start --detach` against a live server overwrote the live pidfile with the doomed child's PID; the
  child then died on `address already in use` and its `defer removePidfile` deleted the pidfile entirely, leaving
  stop/open/reindex reporting "not running" while the original kept running. `startDetached` now runs the same
  `readPidfile`+`processAlive` refusal before spawning, and confirms the child is still alive (300ms grace) before
  printing "started". Test: `TestStartDetachedRefusesWhenAlreadyRunning`. Green (Go-only).
- 2026-07-02 — **review fix: reconcile sweep derives a bounded assistant-text preview, no more raw JSON in status.detail — green.**
  BLOCKING: the 2.2 stale-correction sweep passed `lastNonEmptyLine(transcript.jsonl)` — a raw NDJSON event envelope
  (Phase 4's transcript is JSON, not plain text) — as the status `detail`, so any live agent idle ≥30s got its card
  preview overwritten with `{"agent_id":...,"type":"turn_end",...}` (unbounded, multi-MB for a big tool_result). Replaced
  it with `lastAssistantPreview`: parse tail events, concatenate the last turn's `assistant_text` deltas, clip to 120
  runes (§6.4 last-output-line); no assistant text → return "" so the existing detail is preserved. `ApplyStaleCorrection`
  now preserves the prior `last_trace` (was the out-of-§4.4-vocab `"ReconcileSweep"`) and clamps detail at the boundary.
  Tests: `TestReconcileSessionsOnceDoesNotWriteRawJSON`, `…PreservesDetailWithoutAssistantText`, rewritten
  `…AppliesStaleCorrection` (NDJSON). Green plain + `-tags sqlite_fts5`.
- 2026-07-02 — **review fix: Cancel escalates to SIGINT + FTS fallback upgrade; matched_in dismissed — green.** (1)
  `ChatRuntime.Cancel` now arms a grace-then-SIGINT escalation (`SetCancelGrace`, default 3s) so a peer that ignores
  `session/cancel` is reaped instead of staying busy until a hard Stop (techspec §8.4). New fakeacp `ignore_cancel`
  scenario; test `TestCancelEscalatesToSIGINT`. (2) `ensureSessionsFTS` detects a stale plain `sessions_fts` (from a
  prior non-FTS5 build) and, when FTS5 is now available, drops+recreates it as the virtual table (content repopulates on
  reindex) instead of staying degraded forever; tagged test `TestEnsureSessionsFTSUpgradesFallback`. (3) DISMISSED the
  `matched_in`-on-diacritics advisory — cosmetic, correct fix needs a new `x/text` dep. Green plain + `-tags sqlite_fts5`.
- 2026-07-01 — **review fix: reindex refuses a live server + switch rollback covers the pre-resume window — green.** (1)
  `agentdeck reindex` now hard-errors when the server is running (was only a warning) — it opens its own writer and wipes
  the index, violating the sole-writer invariant; a stale pidfile is tolerated via the signal-0 liveness probe. Test:
  `TestServerRunningDetectsLiveProcess`. (2) `handleSwitchRuntime` now routes `composeSwitchSpec`/`buildHistoryPrimer`/
  `WriteAgent` failures (which occur AFTER the old runtime is stopped+cleaned) through `rollbackSwitch`, not a bare
  error — so a failure there no longer leaves the agent dead with no running row (guarded by the existing
  `TestSwitchRuntimeRollbackOnResumeFailure`; the three post-teardown failures aren't deterministically forceable in a
  test). Green plain + `-tags sqlite_fts5` (Go-only).
- 2026-07-01 — **review fix: SSE atomic snapshot+subscribe (no dropped state_update) — green.** The `/api/events`
  handler took the bus snapshot and subscribed under two separate locks, so a `state_update` published in the gap was
  lost and a card could show stale state until the next update. Added `Bus.SubscribeWithSnapshot()` (snapshot + register
  under one write lock) and switched `handleEvents` to it. Test: `TestSubscribeWithSnapshotReturnsSnapshotAndLiveChannel`.
  Green plain + `-tags sqlite_fts5` (Go-only).
- 2026-07-01 — **review fix: runtime — transport-closed sentinel + tool_call_update terminal-only — green.** (1)
  `transport.shutdown` now delivers the `errTransportClosed` sentinel itself (widened `rpcResult.err` to `error` with a
  typed-nil guard at the deliver site) so chat.go's `errors.Is(err, errTransportClosed)` matches — a crash mid-turn no
  longer risks a spurious `error{protocol}` + second `turn_end`. (2) `acpmap` only emits a `tool_result` on a terminal
  `tool_call_update` status (completed/failed), not on in-progress/status-less updates (diff blocks still stream). Tests:
  `TestTransportCallErrTransportClosedOnShutdown`, `TestMapToolCallUpdateOnlyTerminalStatusEmitsResult`,
  `TestMapToolCallUpdateEmitsDiffOnInProgress`. Green plain + `-tags sqlite_fts5` (Go-only).
- 2026-07-01 — **review fix: UI advisory batch — green.** Five UI advisories: FilesTab "Diff" now works —
  `TranscriptView` emits `data-seq`, `ChatPanel` tabs are controlled so `FilesTab.onReveal` switches to the transcript
  tab then scrolls to the event; `sse.ts` seq-gap refetch is gated on the OPEN agent and no longer double-appends the
  gap event (was refetching any agent + clobbering live appends); `CardContextMenu` rename/stop now `.catch → pushError`
  like their siblings; removed the dead `launchDefaultAgent` export; `NotificationCenter` gives each toast its own 6s
  timer (per-toast `<Toast>` component) so a new toast no longer restarts older ones. Tests: FilesTab onReveal,
  CardContextMenu rename+stop error toasts, NotificationCenter independent timers, sse open-agent-only gap refetch.
  Embedded UI dist refreshed. Green: `go build/test`, `npm test` (66), `npm run build`.
- _(older entries — the 2026-07-01 review-fix batch (dead-code + session/load model/systemPrompt, durability + Makefile,
  crash-path registration teardown, WS-bridge PTY dup, terminal turn-budget reset, skip_permissions/add_dirs on
  resume/switch), the 6.x subphases (6.2–6.6) + their review fixes, 6.1, 5.4 — all live in git history.)_
