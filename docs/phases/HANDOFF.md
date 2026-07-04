# AgentDeck — Implementation Handoff

**Live state. Read this first, every session. Update it after every change.**
Protocol: [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) (Claude Code or Codex, whichever the human runs).
Keep this lean — apply the condensation rules (workflow §5); old detail lives in git, not here.

---

## Current position

- **Active phase:** 6 — Flexibility: terminal runtime, switch-runtime, task groups
- **Active subphase:** 6.7 (next, optional) — iTerm2/AppleScript driver
- **Spec:** [`tech/phase-6-flexibility-techspec.md`](tech/phase-6-flexibility-techspec.md) (PRD: [`phase-6-flexibility.md`](phase-6-flexibility.md)); subphase plan at §"Subphase plan"
- **Last GREEN checkpoint:** review fix (switch primer one-shot, finding 6): `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`.
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
- [ ] Phase 7 — Future phase: candidate-driven post-core work

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
- **Resume note:** xterm/tmux drivers and capabilities are green. 6.7 is fully skippable; if skipped, roll Phase 6 complete and pick the Phase 7 candidate from `phase-7-feature-candidates.md`.

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

**Source:** fourth full top-to-bottom review (2026-07-04) — segmented simpler-model reviews for
phases 0–6.6 (foundation/config/state, runtime, SSE/dashboard, onboarding/config, persistence,
coordination, terminal/switch/UI) followed by a holistic simpler-model synthesis and main-agent
verification. Baseline: `go test ./...`, `go test -tags sqlite_fts5 ./...`, `cd ui && npm run
test`, and `cd ui && npm run build` all pass. `go build ./...` exits 0 but prints a sandbox-denied
Go module stat-cache write outside the repo. The prior cancel-escalation BLOCKING finding is fixed
in current code (`permission.go` captures `turnSeq`); the permission-resolution race remains open.

### BLOCKING

- **BLOCKING — Permission() ignores whether its resolution won the race: fabricated
  `resolved:true` + conflicting transcript events.** `internal/runtime/permission.go:67-95`
  discards `resolvePending`'s bool and unconditionally returns success, emits
  `EvPermissionResolved` with its own decision, and sets busy; `onPermissionTimeout` (98-114) has
  the same TOCTOU + unconditional emit — racing approve/cancel/timeout can record two conflicting
  resolutions for one tool call while the ACP peer saw only one. Trigger: double-click approve then
  cancel, or approve racing the timeout. Fix: check the bool; the loser returns already-resolved
  and emits nothing; test: race `Permission` vs `Cancel`, assert exactly one resolved event
  matching the decision that reached fakeacp.
- **BLOCKING — SSE auto-reconnect can kill a healthy stream and preserve stale snapshot rows.**
  `ui/src/api/sse.ts:18-35,50-55,113-121`: `lastPing`, `hydrationIds`, and `lastAgentSeq` reset
  only on `connect()`/first hydration, not on the browser's automatic `EventSource.onopen`
  reconnect. A normal dropped connection can reopen on the same `EventSource` after the stale
  25s ping window, then the watchdog closes it before its first new ping; if the drop happened
  mid-hydration, stale IDs from the partial snapshot are unioned into the next snapshot and deleted
  agents can survive indefinitely. Fix: treat every `onopen` after an error as a fresh hydration
  generation/liveness boundary; tests: fake auto-reconnect after >25s and reconnect before
  `hydrated`, asserting the stream survives and removed agents are pruned.
- **BLOCKING — Onboarding backend validation can overwrite seeded backends with placeholder
  model data.** `ui/src/features/onboarding/steps/BackendStep.tsx:22-35,40-76,143-145`: the
  Validate button is enabled while `useBackends()` is still undefined; a fast click composes from
  `{}` with `modelKey="default"` and `modelStr=""`, then `PUT`s a replacement backend document
  containing the literal placeholder model. This defeats the merge-preserve review fix and can
  destroy the seeded model map on a fresh install. Fix: disable validation until the backend query
  has loaded/prefilled, or compose only from the fetched document; test: delay `GET /api/backends`,
  click immediately, and assert no placeholder payload is sent.
- **BLOCKING — Terminal backend swaps report `history_handoff:"primer"` but drop the primer.**
  `internal/server/switch.go:160-172` stores the bounded history primer in
  `spec.RuntimeSystemPrompt`, but `internal/runtime/terminal/terminal.go:537-565` builds terminal
  launches from argv/env only and never consumes `RuntimeSystemPrompt`/`StartSystemPrompt()`.
  Cross-backend or non-native-resumable switches into terminal therefore lose the continuity the
  API claims was handed off. Fix: thread the one-shot primer into the terminal launch path or reject
  primer-required terminal switches; test: switch into terminal with `history_handoff:"primer"` and
  assert the launched backend receives the primer payload.
- **BLOCKING — Codex terminal launches register no hooks, so hooks-only terminal status never
  advances.** `internal/backend/adapter.go:161-165` returns nil `HookLaunchArgs` for Codex, while
  `internal/server/launch.go:229-238` says terminal hook registration is required because hooks are
  the sole status producer and `internal/runtime/terminal/terminal.go:167-205` only writes the
  initial idle row. The UI exposes `backend=codex` + `interface=terminal`, so a normal Codex
  terminal launch can succeed but stay stuck at the initial status. Fix: implement Codex terminal
  hook registration or reject the unsupported combination with 422; test: Codex terminal launch
  must either receive hook args or fail before launch.

### ADVISORY

- **ADVISORY — launch parser accepts missing value operands as empty strings.**
  `internal/cli/launch.go:62-85`: `val()` returns `""` when a value flag is last or followed by
  another flag, so `agentdeck impl@proj --resume` silently falls through to a fresh launch instead
  of failing fast. Fix: make value-taking flags require a non-flag operand; test:
  `parseLaunch([]string{"impl@p", "--resume"})` returns an error.
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
- **ADVISORY — graceful shutdown never completes while a dashboard tab is open → every stop ends
  in SIGKILL.** SSE handlers never idle (`internal/server/sse.go:47-61`), `srv.Shutdown` doesn't
  cancel request contexts, and the bus is never closed on shutdown (`server.go:198-211`) → 5s
  timeout, `dashboard stop` prints "did not exit gracefully" and SIGKILLs. Also makes the
  crash-truncated-transcript BLOCKING's precondition routine. Fix: close bus subscribers on
  shutdown (or cancelable BaseContext).
- **ADVISORY — StopAll ignores ctx; stop grace is serial 5s per agent; the tmux path always sleeps
  the full 5s** (`internal/runtime/permission.go:210-220`, `chat.go:977-984`,
  `terminal/terminal.go:396-399`) — multi-agent shutdown overshoots every timeout → SIGKILL +
  possible orphaned process groups.
- **ADVISORY — reconcile sweep stomps switched-to-terminal agents' status detail with stale
  pre-switch chat text.** `internal/server/reconcile.go` derives previews from `transcript.ndjson`
  with no interface check; `ApplyStaleCorrection` discards `RunningEntry.Interface`
  (`state/manager.go:176-244`). Self-heals on the next hook. Fix: skip the preview when
  `interface != "chat"`.
- **ADVISORY — deleting a role silently flips archived agents to the GLOBAL `skip_permissions` on
  resume.** The delete in-use guard checks only running agents (`config_handlers.go:215-231`);
  `resolveSkipForRole` (`launch.go:298-303`) falls back to the global default when the role is
  missing — a deliberately-cautious (skip=false) role deleted during cleanup makes a later resumed
  agent auto-approve. Safety-relevant. Fix: include archived references in the 409 guard, or fall
  back to `skip=false`.
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
- **ADVISORY — the inbox endpoint returns the OLDEST N when the mailbox exceeds `limit`**
  (`server/sessions.go:76-83`: ASC LIMIT then reverse). Latent (inbox UI unbuilt). Use
  `ORDER BY created_at DESC LIMIT`.
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
- **ADVISORY — tmux driver is implemented+tested but unselectable, while `/api/capabilities`
  advertises `tmux:true`** (no `driver` field in launch/switch API or UI; `DriverAvailable`'s 422
  is unreachable). Wire a driver field or stop advertising. Related: `config.terminal.max_tabs` /
  `429 terminal_tab_limit` (techspec §9) is entirely unimplemented and untracked — implement or
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
- **ADVISORY — d6a18cb residue: `state/migrate.go:39` is a missed `rows.Err()` sibling** (the
  schema_migrations scan still checks `rows.Close()`); and `latestKnownMigration=6` is
  hand-maintained with no guard test tying it to the migrations slice (a future v7 without the
  bump self-bricks). Add the one-line guard test.
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
- **NEW (review fix): skip_permissions/add_dirs are RE-RESOLVED from current role/project config on resume+switch,
  not persisted into the frozen session snapshot.** The BLOCKING findings suggested persisting `add_dirs` into
  `SessionSnapshot` + the `sessions` table (+ a migration). I chose to re-resolve both `skip_permissions` (via
  `resolveSkipForRole`) and `add_dirs` (via `resolveAddDirs`) from the current role/project config instead. **Why a
  judgment call:** (1) the finding itself mandates "resume must re-read the role" for skip — re-reading the project for
  add_dirs is the consistent analog; (2) it avoids a schema migration + session write-path changes (lower risk, smaller
  blast radius); (3) it picks up config edits made after launch. **Tradeoff:** it diverges from the strict "frozen
  snapshot" philosophy — `cwd`/`system_prompt` are still frozen, but skip/add_dirs now track the live config, so editing
  a project's `add_dirs` between launch and resume changes the resumed agent's dirs. **To reverse:** add an `add_dirs`
  column to `sessions` + `SessionSnapshot`/`SessionMetaData`, populate it at session creation, and read `snap.AddDirs`
  (and a persisted skip flag) in the composers instead of the resolvers.
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
