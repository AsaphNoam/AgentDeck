# AgentDeck — Implementation Handoff
**Live state. Read this first, every session. Update it after every change.**
Protocol: [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) (Claude Code or Codex, whichever the human runs).
Keep this lean — apply the condensation rules (workflow §5); old detail lives in git, not here.
Human-facing session state lives in [`BRIEFS.md`](BRIEFS.md); agents do not read old briefs to resume.

---

## Current position

- **Active phase:** 6 — Flexibility: terminal runtime, switch-runtime, task groups
- **Active subphase:** 6.7 (next, optional) — iTerm2/AppleScript driver
- **Spec:** [`tech/phase-6-flexibility-techspec.md`](tech/phase-6-flexibility-techspec.md) (PRD: [`phase-6-flexibility.md`](phase-6-flexibility.md)); subphase plan at §"Subphase plan"
- **Last GREEN checkpoint:** `0f7e680` — review fix (advisory batch: inbox newest-N + CLI operand validation); build and both test variants passed.
- **Last code review:** `8667fe2` (the parent reviewed by `d12bbb6`; next review covers later GREEN checkpoints).
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
- [ ] Phase 7 — Additional features: OpenHands & OpenCode backends — candidate selected 2026-07-07; PRD [`phase-7-additional-features.md`](phase-7-additional-features.md), spec [`tech/phase-7-additional-features-techspec.md`](tech/phase-7-additional-features-techspec.md); not started

Build order: `0 → 1 → 2 → {3, 4, 5} → 6 → 7` (3/4/5 are independent after 2).

---

## Active subphase detail

> The ONLY place granular steps live.

**Subphase 6.7 — next to implement (optional)** (iTerm2/AppleScript driver; techspec §2.2, §3.6, task 6):
- [ ] iTerm2 `TerminalDriver` implementation via `osascript`.
- [ ] AppleScript templates rendered with `text/template` for create-tab, set-appearance, write-text.
- [ ] Escaping + shell-quote helper with tests for quotes/backslashes/newlines/argv shell-quoting.
- [ ] Capability probe wiring; explicit unavailable `driver:"iterm2"` returns `422 terminal_unavailable` with reason.
- **Checkpoint:** `go build ./...` + `go test ./...` + `go test -tags sqlite_fts5 ./...` (Go-only unless UI driver picker changes).
- **Resume note:** xterm/tmux drivers and capabilities are green. 6.7 is fully skippable; if skipped, roll Phase 6 complete and start Phase 7 (selected candidate: OpenHands & OpenCode backends — [`tech/phase-7-additional-features-techspec.md`](tech/phase-7-additional-features-techspec.md), subphase plan §7).

---

## Decisions awaiting review

> Only unresolved HUMAN choices and PEER choices awaiting independent review live here (workflow §3).
> HUMAN items repeat in every new brief until explicitly acknowledged; silence is not consent.

- **HUMAN — Terminal support boundary.** Claude terminal
  launches receive model/directories/system-prompt flags, but that live CLI mapping is not credential-tested;
  Codex terminal launches are rejected, and terminal agents cannot receive agent-to-agent messages. This
  avoids statusless or endlessly nudged agents at the cost of advertised combinations. Reverse by verifying
  each CLI's hook/flag/MCP surfaces, then wiring the adapter-specific paths before lifting the gates.
- **HUMAN — HTTP-only agent messaging.** The in-process
  messaging server is mounted over local HTTP; the planned stdio proxy was removed because it never shipped.
  A CLI that rejects HTTP registration cannot use messaging until a working proxy is implemented.
- **HUMAN — Immediate/prompt-based UI.** Clone launches
  immediately with no confirmation; runtime/group changes use browser prompts; a disappeared process becomes
  `done` rather than `error`; and an invalid seeded project is explained only after launch fails. Reverse by
  adding the dedicated dialogs/confirmation and stricter preflight/error semantics.
- **HUMAN — Runtime-switch fallbacks.** Cross-backend
  context defaults to local transcript truncation instead of a live target-model summary; cancellation polls
  status for a hardcoded five seconds; the live identity updates before the archived session snapshot; and a
  stopped identity returns a new `409 agent_not_running`. These are user/API-visible interoperability choices.
- **HUMAN — Unbounded transcript indexing.** Full-text indexing keeps the
  whole transcript in memory and rewrites it at turn boundaries so old content remains searchable. Very long
  sessions can become expensive; a chunked index would reverse the trade-off without dropping search data.
- **HUMAN — API/model compatibility.** Older endpoints still use a different error-envelope
  shape, and the current Agent Client Protocol adapter can ignore AgentDeck's requested model in favor of its
  own model identifiers. Standardize the API envelope and map model IDs before promising those contracts.

## Acceptance gates (not blockers)

- Confirm real Claude Code and Codex accept the local HTTP MCP registration and can call `ping`; if either
  rejects it, implement the documented stdio proxy before claiming messaging compatibility for that CLI.
- Run real Codex chat launch, turn, stop, and resume with credentials; reconcile model/resume/hook behavior.
- Run real Claude terminal launch/switch with the composed flags and hooks; reconcile any CLI flag mismatch.

These gates require credentials but do not block subphase 6.7 or Phase 7. They must be cleared before a
release claims the affected live-CLI compatibility.

## Blocked on human

None. This section holds only genuine STOP conditions, never nonblocking acceptance work.

## Review findings (from the last review — BLOCKING and ADVISORY)

> Written by the review agent (workflow §8), one bullet per finding tagged with its severity
> (`BLOCKING` / `ADVISORY`). Consumed by the fix agent (`/fix-review`, workflow §9), which validates
> each is actually true, then **deletes the bullet** once it's fixed-and-green or dismissed as a
> validated false positive — recording the outcome in the changelog + its human brief (§5, §7).
> **This section holds only OPEN findings** — no resolved/dismissed graveyard.
> Blocking items must be fixed before the next phase starts; advisory items when convenient.

### Review through `8667fe2` — 2026-07-04 (legacy batch)

Subsequent fix-review sessions removed resolved and dismissed bullets. The list below is the complete
remaining open set; every surviving item is ADVISORY.

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

## Changelog

_(most recent first; keep ~10, older history is in git)_

- 2026-07-10 — **workflow review: low-attention briefs and deterministic routing.** Added the bounded
  human brief contract and usability-review role; split HUMAN from PEER decisions; made reviews persist
  all findings and state commits; repaired cold-resume/path references; thinned and synchronized role
  skills; condensed this handoff without removing any open finding.
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
  terminal runtime. Fix (hybrid, see Decisions awaiting review): claude terminal now passes `--model`/`--add-dir`/
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
  violating techspec §12.4's frozen-snapshot rule. This **reverses a prior decision** that chose
  re-resolution (historical rationale is in git). Migration v7 adds `sessions.skip_permissions`/`add_dirs`; the values
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
- _(Older checkpoint detail lives in git.)_
