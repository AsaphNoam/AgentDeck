# AgentDeck invariants — bugs this repo already paid for

Every class below cost at least one review cycle; most recurred in two or more subsystems before
being named. The evidence base is multiple full top-to-bottom reviews plus the repo's `review fix:`
commit history. Treat these as **load-bearing rules, not suggestions**: a diff that violates one is
wrong until proven otherwise, and a review finding that matches one is almost certainly real.

The hot-spot areas: launch/resume/switch composition, `internal/runtime` concurrency,
`internal/state`/`internal/index` persistence, terminal/PTY, UI forms over seeded config.

How the loop uses this file:
- **/work-phase** — before building in a hot-spot area, read the matching class; new interfaces
  must complete the §6 contract checklist.
- **/review-phase** — sweep the diff against every class; tag each finding with its class number.
- **/fix-review** — note the class in the changelog line; if a fix reveals a genuinely new class
  (or a new canonical pattern), append it here. Keep this file curated — merge near-duplicates,
  don't let it become a graveyard.

---

## 1. Crossing a boundary must reset or republish derived state

**Rule:** whenever execution crosses a lifecycle boundary — reconnect, relaunch, resume, switch,
agent-tab change — every piece of state *derived from the old side* must be explicitly reset or
republished. Nothing "just stays valid."

Paid for by:
- `ui/src/api/sse.ts` — `lastPing`/`hydrationIds`/`lastAgentSeq` reset only on first connect, not
  on `EventSource` auto-reconnect: the watchdog reaped fresh connections with a stale clock and
  stale snapshot rows survived indefinitely.
- `internal/server/messaging_loops.go` — nudger cooldown keyed by `agent_id` alone survived
  stop/relaunch; `check_messages`/janitor mutated read state without republishing the agent, so
  unread badges went stale.
- `internal/server/reconcile.go` — stale-sweep overwrote a switched-to-terminal agent's preview
  with pre-switch chat text.
- `ui/src/components/.../FilesTab.tsx`, `CommandsTab.tsx` — one-shot fetch-on-mount snapshots that
  also answer-raced across agent switches (fixed with a per-agent request token).

**Canonical patterns:** reset connection-scoped state in `onopen`, not the constructor; republish
the affected agent after any read/delete mutation; per-agent request tokens for async UI fetches.

## 2. Parallel paths that build "the same thing" must share one helper

**Rule:** two code paths constructing the same logical artifact (a LaunchSpec, a session params
struct, a pidfile guard, a validation) WILL drift. Extract the shared helper the moment the second
path appears, and route both through it.

Paid for by:
- launch vs resume vs switch each rebuilding LaunchSpec: `SkipPerms`/`AddDirs` dropped on
  resume/switch (`internal/server/resume.go`, `switch.go`); terminal launch silently dropped
  model/system-prompt/add_dirs/MCP registration.
- `session/new` vs `session/load` params in `internal/runtime/chat.go`: `load` omitted
  `cwd`/`mcpServers`, then later `model`/`systemPrompt` — resumed agents silently lost registration
  or kept the old model.
- foreground vs `--detach` in `internal/cli/dashboard.go`: only the foreground path had the
  live-pidfile refusal, so a doomed detached child clobbered a live server's pidfile.
- POST-only slug validation: PUT/DELETE role/project handlers in
  `internal/server/config_handlers.go` took path ids unvalidated (path traversal, BLOCKING).

**Canonical helpers:** `composeLaunch`, `resolveSkipForRole`, `resolveAddDirs`
(`internal/server/launch.go`); keep `sessionNewParams`/`sessionLoadParams` in lockstep;
`config.ValidSlug` on **every** verb of every path-keyed resource.

Corollary: permission-relevant re-resolution **fails closed** — on a role-read error, refuse, never
fall back to the permissive global default (`resolveSkipForRole`).

## 3. Persisted fields never receive one-shot data; forms merge, never replace

**Rule:** a field that is both sent to the runtime AND persisted must never have transient data
concatenated into it — one-shot additions need a separate field the persistence path is
structurally blind to. Symmetrically: a UI form pre-populated from seeded server config must
merge-preserve the seeded collection on submit, never write back its partial on-screen view.

Paid for by:
- `internal/server/switch.go` — backend-switch primer concatenated into `spec.SystemPrompt`, which
  is also persisted into the frozen `sessions.system_prompt`: the primer stacked on every
  subsequent switch. Fix: a runtime-only suffix field the DB write never sees.
- `ui/src/features/onboarding/steps/BackendStep.tsx` — twice: an untouched "Continue" wholesale-PUT
  a single synthesized backend over the seeded multi-model map; then the Validate button raced the
  initial `/api/backends` fetch and clobbered it again. Fix: merge-preserve + gate the handler on
  query readiness (`isLoading`).

## 4. Create/teardown symmetry: one teardown function, generation-scoped, on every exit path

**Rule:** every artifact created at registration (hook token, MCP session/file, hook-settings file,
DB row) is torn down by exactly one shared function, invoked from **every** exit path — stop,
switch, failed launch, failed resume, and unsolicited crash. Teardown is scoped to a launch
generation, not just an agent id. Teardown of the OLD strictly precedes registration of the NEW
under the same key, and every failure branch after that point routes through the same rollback.

Paid for by:
- `handleLaunch`/`handleResume` failure paths hand-rolled different cleanup subsets, leaking
  tokens/MCP/hook-settings → unified `teardownAgentRegistration` (`internal/server/launch.go`).
- crash path (`chat.onExit`) only called `registry.forget`, leaving a crashed agent's identity
  spoofable → `Registry.SetExitHook`/`handleAgentExit`.
- teardown keyed by `agent_id` alone let a late crash-teardown of launch N delete launch N+1's
  artifacts during a switch window (a reproducible flake, pinned by
  `TestSwitchRuntimeKeepsTargetRegistration`).
- `handleSwitchRuntime` cleaned OLD artifacts *after* NEW registration (wiping the fresh token),
  and its rollback covered only the final failure branch.
- Stop on an agent the registry didn't own silently deleted the DB row and orphaned the live
  process → `runtime.ReapOrphan`: confirm PID liveness and signal before clearing state ("not
  owned" ≠ "not running"). Also: 404 means "no identity row", never "not currently running" —
  lifecycle verbs are idempotent for known entities.

## 5. Check-then-act needs an atomic claim or a generation token

**Rule:** any "if pending then resolve" or "if active then fire" across goroutines must either
claim atomically under one critical section (take-and-delete) or capture a generation counter and
recheck it at fire time. Only the winner of the race may emit side-effect events. Snapshot+subscribe
is one lock acquisition, never two.

Paid for by:
- `internal/runtime/permission.go` — approve/deny/timeout raced on `pending[toolCallID]` under
  separate locks; losers still emitted `EvPermissionResolved` → `takePending` +
  `ErrPermissionAlreadyResolved`.
- cancel SIGINT escalation rechecked a `turnActive` bool, so a cancel armed for turn A could SIGINT
  turn B started inside the grace window → capture and recheck `turnSeq`.
- `internal/bus/bus.go` + `internal/server/sse.go` — `Snapshot()` then `Subscribe()` as two locked
  calls lost any event published in the gap → `SubscribeWithSnapshot`.
- notification edge detection (`Manager.Touch` skipping `writeMu`; read-prev/write-snapshot under
  separate locks) double-fired or missed done/waiting_input toasts.
- `internal/state/messages.go` — "current turn budget" read via `ORDER BY rowid DESC` picked a
  stale row after resume reset the in-process `turnSeq`: prune all other rows for the key in the
  same transaction as the reset; never trust an in-process counter across restarts.

## 6. A new interface/runtime must join every existing contract (checklist)

**Rule:** the terminal runtime shipped as a second-class citizen and produced the single largest
concentration of BLOCKING findings (6 findings, one review). Any new interface, runtime, or driver
must explicitly walk this checklist — silence on any line is a bug, not a default:

- [ ] **Persistence:** gets a `sessions` row → visible in archive, resumable, survives Stop.
- [ ] **LaunchSpec:** honors the full composed spec (model, system prompt, add_dirs, MCP
      registration) — via the shared resolvers of §2.
- [ ] **Fan-out/drain:** output readable by N viewers and drained when *zero* viewers are attached
      (a full kernel tty queue stalled an unobserved CLI indefinitely). Pattern:
      `internal/runtime/terminal/ptyhub.go` — one always-on reader per PTY, bounded scrollback
      ring, non-blocking fan-out that drops slow subscribers. Never `dup()` a shared fd per viewer
      (splits the stream), and never let a transient view's teardown close a long-lived fd it
      doesn't solely own (a WS unmount once SIGHUP'd the live CLI).
- [ ] **Messaging:** either it can drain its mailbox (`check_messages`) or `ResolveRecipient`
      excludes it — an undrainable nudge loop once burned thousands of paid turns.
- [ ] **Turn boundaries:** its turn signal is identified and wired into every per-turn reset
      (`ResetTurnBudget` via `terminalTurnID`) — not lumped into a generic status hook.
- [ ] **Reconcile:** the sweep knows its shape (no chat-shaped preview stomping, §1).
- [ ] **Hooks/status:** feature flags scoped to the actual risk surface, not blanket-applied
      (`AGENTDECK_HOOK_REGISTRATION` once muted terminal agents entirely).
- [ ] **Capabilities honesty:** never advertise what no API/UI surface can select (`tmux:true`
      with no selector shipped).
- [ ] **Teardown:** joins §4's single teardown on every exit path.

## 7. Read paths must not swallow errors or amplify damage

**Rule:** iteration and repair code treats each record/entity as independently failable: check the
real error signal, skip the bad record, continue, and report — never abort the whole stream or
wipe-then-fail.

Paid for by (this one recurred **four times** as the same literal mistake):
- `rows.Err()` unchecked after scan loops — `ListInactiveSessions` (`internal/state/session.go`),
  `queryTrackedFiles`/`queryTrackedCommands` (`internal/server/files_commands.go`), and again as
  residue in `internal/state/migrate.go`. `rows.Close()` is cleanup; **`rows.Err()` is the only
  iteration-failure signal.** A mid-iteration failure otherwise silently truncates the list.
- `bufio.Scanner` in the transcript reader aborted the *entire* transcript on one oversized
  (>8 MiB) line (`ErrTooLong`) — could 500 `/transcript` and block resume permanently. Skip the
  record, keep the stream.
- `agentdeck reindex` wiped ALL agents' index up front, then aborted wholesale on the first
  unreadable transcript — the repair tool left the archive worse than before. Per-entity isolation.
- reindex flush logic lost the final partial turn when a transcript had one finished turn plus a
  crash mid-later-turn.
- FTS content buffer capped at 1 MiB silently dropped older transcript text; "newest N" implemented
  as `ORDER BY created_at ASC LIMIT N` returned *oldest* N (subquery DESC + re-sort).

## 8. User-facing surfaces get parsed, bounded, in-vocabulary data — and errors always surface

**Rule:** anything rendered to the human (card previews, `status.detail`, toasts) is parsed,
human-meaningful, length-clamped at the write boundary, and drawn from the declared vocabulary.
Every mutating UI action surfaces failure; every notification funnels through the one payload
builder.

Paid for by:
- `internal/server/reconcile.go` wrote the raw NDJSON transcript line (unbounded JSON envelope)
  into `status.detail`, corrupting every idle card preview each ~30s sweep → parse tail events,
  clip to ~120 runes (`lastAssistantPreview`); never overwrite vocabulary fields (`last_trace`)
  with out-of-vocabulary values.
- bare `void switchRuntime(...)`/`void rename(...)` in `ui/src/components/grid/CardContextMenu.tsx`
  swallowed structured server errors → extract the `{error:{code,message}}` envelope in
  `ui/src/api/client.ts`, `.catch → pushError` on every mutation.
- `budget_exceeded` payload built inline instead of via the shared `notificationPayload()` builder.
- config-derived caches invalidated in the same handler that writes the config
  (`handlePutBackends` once left `onboardingCache` stale); DELETE UIs implement the 409
  confirm+force-retry loop the API contract requires.

## 9. Liveness & durability primitives are weaker than they look

**Rule:** the OS and SQLite primitives this codebase leans on all have sharp edges that already cut:

- **Bare `kill(pid, 0)`** proves only that *some* process has the PID (reuse!). Used in
  `internal/cli/pidfile.go` and both reconcilers — corroborate (start-time/comm/nonce) where it
  gates destructive action, and never remove a pidfile without verifying it names your own PID.
- **Atomic-write-via-rename** must fsync the file AND the parent directory
  (`internal/config/atomic.go`, `pidfile.go`).
- **`CREATE VIRTUAL TABLE IF NOT EXISTS`** freezes a degraded fallback forever — a
  capability-upgrading migration must detect and replace the fallback object
  (`ensureSessionsFTS`, `internal/state/migrate.go`).
- **Append-only writers** must truncate a crash-torn trailing record on `Open()` before appending,
  or the torn tail fuses with the next record (`internal/transcript/writer.go`).
- **In-memory accumulators feeding replace-style (DELETE+INSERT) writes** must lazily reseed from
  the durable table on first use per process — an empty-seeded buffer once wiped all FTS content
  after a restart (`seedLocked`, `internal/index/indexer.go`).
- **Version/protocol gates fail, never warn-and-continue** — the ACP handshake once logged an
  incompatible protocol version and proceeded (cleanup via `shutdown()` on refusal).
- **Sentinel errors survive transport intact** — never re-wrap a sentinel into another concrete
  type (`errors.Is` broke on a synthesized `*rpcError`; watch typed-nil traps). A status-bearing
  streaming event is terminal only on an actual terminal status value (a missing status once
  defaulted to "completed" and prematurely closed tool calls).
- **Destructive CLI ops sharing the live server's DB hard-refuse on liveness** (`reindex` once only
  warned); manually-tracked version constants get a guard test asserting equality with the
  migrations slice.
- **PTY WebSocket bridge framing:** keystrokes are binary frames; text frames are reserved for the
  `{cols,rows}` resize channel — a text-frame keystroke is silently eaten
  (`ui/src/components/chat/TerminalTab.tsx`).

## 10. Ship the wiring; kill the drift

**Rule:** a feature isn't done until every surface that promises it can reach it, and every doc
that describes it matches. Gating stubs are un-gated in the same effort that ships the gate.

Paid for by:
- Clone context-menu action stayed a disabled stub with an "Available in Phase 3" tooltip long
  after Phase 3 shipped — a master-PRD promise dropped between phase specs; only a holistic
  cross-project pass caught it. When closing a phase, check the master PRD for unowned promises.
- `tmux` driver implemented + tested but unselectable from any API/UI while capabilities advertised
  it.
- Docs drift: README omitted `install.sh` defaults/prereqs; `MAP.md` described the messaging MCP as
  stdio after HTTP shipped; `architecture-flow.md` showed terminal→bus parity that didn't exist.
- `.claude/skills` vs `.agents/skills` twin drift ("Codex **or** Codex" — since fixed).
- Dead-code removal requires a tree-wide call-site check first; soft-cancel to an external peer
  needs a time-bounded escalation tier (grace → SIGINT) distinct from Stop's hard kill.

---

## Canonical helpers registry (reuse, don't re-derive)

| Helper | Where | Use for |
|---|---|---|
| `teardownAgentRegistration` | `internal/server/launch.go` | every agent exit/failure path (§4) |
| `composeLaunch`, `resolveSkipForRole`, `resolveAddDirs` | `internal/server/launch.go` | any path that builds/rebuilds a LaunchSpec (§2) |
| `takePending` | `internal/runtime/permission.go` | atomic claim for racy resolutions (§5) |
| `SubscribeWithSnapshot` | `internal/bus/bus.go` | any snapshot+subscribe consumer (§5) |
| `runtime.ReapOrphan` | `internal/runtime/` | stopping agents the registry doesn't own (§4) |
| PTY hub pattern | `internal/runtime/terminal/ptyhub.go` | any shared-fd broadcast need (§6) |
| `seedLocked` | `internal/index/indexer.go` | in-memory buffers feeding replace-style writes (§9) |
| `config.ValidSlug` | `internal/config/validate.go` | every path-param on every verb (§2) |
| `notificationPayload` | `internal/bus/` | all notification payloads (§8) |
| `fakeacp` test double | `internal/runtime/testdata/fakeacp` | env-driven protocol-level repros (`FAKEACP_LOAD_DUMP`, `FAKEACP_PROTO_VERSION`, `ignore_cancel`) |
