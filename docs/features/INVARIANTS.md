# AgentDeck invariants — normative technical-spec appendix

Every class below cost at least one review cycle; most recurred in two or more subsystems before
being named. The evidence base is multiple full top-to-bottom reviews plus the repo's `review fix:`
commit history. Treat these as **load-bearing rules, not suggestions**: a diff that violates one is
wrong until proven otherwise, and a review finding that matches one is almost certainly real.
This file is governed by TS-01 and indexed with the technical specs as `INV`; it is not a third
product-spec set. Add or change a binding rule here through the same spec-delta lifecycle as a TS.

The hot-spot areas: launch/resume/switch composition, `internal/runtime` concurrency,
`internal/state`/`internal/index` persistence, terminal/PTY, UI forms over seeded config.

How the loop uses this file:
- **/work** — before building in a hot-spot area, read the matching class; new interfaces
  must complete the §6 contract checklist.
- **/review** — sweep the diff against every class **by the trigger index below**; tag each finding
  with its class number.
- **/fix** — note the class in the changelog line; if a fix reveals a genuinely new class
  (or a new canonical pattern), append it here. Keep this file curated — merge near-duplicates,
  don't let it become a graveyard.

Read the trigger index first, then read in full only the classes your diff actually touches. Reading
all seventeen classes for a diff that touches one is waste, not diligence. Read one class with:

```bash
awk '/^## 7\./{f=1} f&&/^## 8\./{exit} f' docs/features/INVARIANTS.md
```

## Trigger index

| # | Class | Read it when the diff touches |
|---|---|---|
| 1 | Boundary crossing resets or republishes derived state | reconnect, relaunch, resume, switch, or any lifecycle transition that leaves derived state behind |
| 2 | Parallel paths that build the same thing share one helper | a second code path constructing an artifact or projection that already has a builder (check the canonical-helpers registry below) |
| 3 | Persisted fields never receive one-shot data | a field that is both sent to the runtime and persisted; any form that writes seeded config |
| 4 | Create/teardown symmetry | anything created at registration — hook tokens, MCP sessions/files, hook-settings files, temp artifacts |
| 5 | Check-then-act needs an atomic claim | "if pending then resolve" / "if active then fire" across goroutines; `internal/runtime` concurrency |
| 6 | A new interface/runtime joins every contract | a new interface, runtime, driver, or adapter — walk the checklist line by line |
| 7 | Read paths don't swallow errors or amplify damage | iteration, repair, migration, or recovery code over records |
| 8 | User-facing surfaces get parsed, bounded, in-vocabulary data | card previews, `status.detail`, toasts, rendered output, disabled controls, or a UI mutation's error/recovery path |
| 9 | Liveness & durability primitives are weaker than they look | SQLite, file locks, signals, process liveness, timeouts, fsync |
| 10 | Ship the wiring; kill the drift | a feature reachable from some surfaces but not all; docs or defaults left behind |
| 11 | Cross-boundary serialization and protocol contracts | Go structs crossing to the UI; nil slices/maps; ACP or streaming payload identity/status/error semantics |
| 12 | External-CLI invocations tolerate version and environment variance | any `exec.Command` or adapter call to a user-installed tool — agent CLIs, probers, git |
| 13 | Every referenced className has a defined selector | `ui/src` markup or `ui/src/styles/**` |
| 14 | Loopback is not a security boundary | HTTP handlers, CORS, file paths from requests, anything reachable from a browser |
| 15 | Commit local truth before releasing external side effects | an operation that makes work externally observable or lets a peer proceed |
| 16 | Bound work and memory where they enter the system | external output, streams, queues, sweeps, retained maps/lists, goroutine fan-out, or per-client state |
| 17 | Tests prove their contract independently | regression or acceptance tests, contract enumerations, mocks/fixtures, build tags, packaging or release gates |

A class with no applicable surface in the diff is a result to state, not a step to skip — the index
is enough to state it. Open the class body only when the trigger matches.

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
- `ui/src/store/annotationStore.ts` — a per-agent persisted draft tray with no cleanup on any
  lifecycle boundary: a deleted agent's drafts survived forever and could resurface against a reused
  id, and growth across agents was unbounded against the localStorage quota. Browser-local state
  needs the same boundary handling as server-derived state, plus its own retention bound when no
  server event can be relied on (drop on the delete event; expire and cap on rehydration).

**Canonical patterns:** reset connection-scoped state in `onopen`, not the constructor; republish
the affected agent after any read/delete mutation; per-agent request tokens for async UI fetches.

## 2. Parallel paths that build "the same thing" must share one helper

**Rule:** two code paths constructing the same logical artifact or projection (a LaunchSpec, a
session params struct, a pidfile guard, a validation, a live/replayed event stream) WILL drift.
Extract the shared helper the moment the second path appears, and route both through it.

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
- Live transcript append coalesced streamed assistant deltas, but full transcript replay did not,
  so Archive and resume split one assistant reply into many bubbles. Fix: both paths use the same
  `appendRenderedEvent` reducer (`ui/src/store/transcriptStore.ts`).
- Annotation capture shipped `clipExcerpt` copied verbatim into two chat components plus a third
  server implementation, and `AnnotationDraft` declared three times. Both had already drifted (the
  JS marker kept 1,998 runes against the server's 2,000; one local type made the diff anchor fields
  required and another optional) with nothing failing yet — structural typing and a CSS-blind test
  suite see none of it. Fix: `ui/src/lib/annotations.ts` for the clip, `api/types.ts` for the type.

**Canonical helpers:** `composeLaunch`, `composeResumeSpec`, `composeSwitchSpec`, `resolveSkip`,
`expandAddDirs`, `composeEnv`
(`internal/server/launch.go`); keep `sessionNewParams`/`sessionLoadParams` in lockstep;
`config.ValidSlug` on **every** verb of every path-keyed resource; `foldTranscript` and live append
share `appendRenderedEvent` for every render-affecting event transform; Go transcript
consumers derive typed text parts from `transcript.ProjectEvent` rather than adding another
`runtime.Event` switch.

Corollary: permission-relevant re-resolution **fails closed** — on a role-read error, refuse, never
fall back to the permissive global default (`resolveSkip`).

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
- `exec` discards the EXIT trap along with the shell image, so in `scripts/release/install.sh` the
  process that created the piped bootstrap's temporary file could never be the one to remove it, and
  every `curl | bash` install leaked it. Teardown belongs to the last process that still needs the
  artifact — here the lock-holding child, guarded to the installer's own `mktemp` name so a real
  `bash install.sh` cannot delete its own input.
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
      ring, non-blocking fan-out that drops slow subscribers (§16). Never `dup()` a shared fd per
      viewer (splits the stream), and never let a transient view's teardown close a long-lived fd
      it doesn't solely own (a WS unmount once SIGHUP'd the live CLI). Keystrokes use binary
      WebSocket frames; text frames are reserved for the `{cols,rows}` resize channel — a
      text-frame keystroke is silently eaten (`ui/src/components/chat/TerminalTab.tsx`).
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
builder. A disabled action names the condition that blocks it, and alternatives offered together
state their different consequences beside the controls. A mutation's warning or success-adjacent
feedback stays owned by a surface that remains mounted, or is handed off before that surface closes.

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
- Pipeline **Continue** was disabled until continuation text existed without saying what it needed,
  while **Continue** and **Retry stage** were offered together without explaining that Retry creates
  a fresh agent with bounded prior summaries (`ui/src/features/pipelines/RunBrowser.tsx`).
- A successful project create/update stored a `cwd_not_found` warning and immediately closed the
  editor that owned it. The editor now remains open on warnings and a successful create switches to
  edit mode so acknowledgement cannot accidentally create a duplicate project.

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
- **Destructive CLI ops sharing the live server's DB hard-refuse on liveness** (`reindex` once only
  warned); manually-tracked version constants get a guard test asserting equality with the
  migrations slice.

## 10. Ship the wiring; kill the drift

**Rule:** a feature isn't done until every surface that promises it can reach it, and every doc
that describes it matches. Gating stubs are un-gated in the same effort that ships the gate.

Paid for by:
- Clone context-menu action stayed a disabled stub with an obsolete phase tooltip after the feature
  shipped; only a holistic pass caught it. When closing a change, check both the governing FS
  requirements and acceptance items for unowned promises.
- `tmux` driver implemented + tested but unselectable from any API/UI while capabilities advertised
  it.
- Docs drift: README omitted `install.sh` defaults/prereqs; `MAP.md` described the messaging MCP as
  stdio after HTTP shipped; `architecture-flow.md` showed terminal→bus parity that didn't exist.
- `.claude/skills` vs `.agents/skills` twin drift ("Codex **or** Codex" — since fixed).
- Dead-code removal requires a tree-wide call-site check first; soft-cancel to an external peer
  needs a time-bounded escalation tier (grace → SIGINT) distinct from Stop's hard kill.

## 11. Cross-boundary serialization and protocol contracts preserve meaning

**Rule:** Go nil slices/maps marshal to JSON `null`, not `[]`/`{}`. Any collection field the UI
iterates must be non-nil at the marshal boundary (initialize with `make`/literal, or
`append([]T{}, …)` — **never** `append([]T(nil), …)`, which stays nil for empty input), and the UI
API layer defends with `?? []` regardless. Protocol success must also preserve the identity, status,
and error semantics the caller relies on: sentinel errors survive transport intact; identity
ownership follows the operation's contract even when a response omits an echo; and an absent status
never acquires a terminal meaning by default.

Paid for by:
- `layoutFromConfig`/`toConfig` (`internal/server/handlers.go`) built `Order` via
  `append([]string(nil), l.Order...)` → fresh install served `order: null` →
  `CardGrid.tsx`/`agentStore.ts` called `.filter`/`.includes` on it → TypeError, dead dashboard on
  first launch (`353e940`). The MSW fixtures returned `order: []`, so no UI test could see it.
- A syntactically valid hand-edited `backends.json` with a missing `backends` map or `models:null`
  reached `Object.entries` and replaced the whole dashboard with its error boundary. Fix: validate
  required nested collections on read, fall back server-side, and retain `?? {}` at the first UI
  consumer (`internal/config/backends.go`, `ui/src/features/settings/BackendsEditor.tsx`).
- A successful ACP `session/load` response without an echoed `sessionId` was mistaken for failure,
  so Codex resume fell through to `session/new` and abandoned its conversation history. On load
  success the requested id stays authoritative; only a non-empty echoed id replaces it.
- `errors.Is` broke when a sentinel was synthesized as another concrete `*rpcError`; typed nils
  make the same mistake less obvious. A status-bearing streaming event with no status once
  defaulted to `completed` and prematurely closed tool calls.

**Canonical patterns:** `append([]T{}, src...)` at marshal boundaries; `?? []` / `?? {}` where the UI
first touches a server collection; structurally validate required nested collections decoded from
hand-editable files; validate required protocol identity/status fields before mutating runtime state.

## 12. External-CLI invocations must tolerate version and environment variance

**Rule:** any `exec.Command` of a user-installed tool (agent CLIs, probers) runs against whatever
version the user has, not the one the author tested. Optional flags need a detect-and-retry
fallback; output parsing is defensive (substring vocabulary, not exact format); a tool that can't
be interrogated reports "unknown"/"skipped", **never** "failed" — a wrongly failed gate blocks the
user harder than no gate at all. Exit zero or a successful RPC envelope is not semantic success:
validate the required output shape, identity, and effective configuration before relying on it.

Paid for by:
- `internal/backend/credcheck/claude.go` ran `claude auth status --no-color`; older Claude builds
  don't have `--no-color`, so a logged-in user failed the onboarding credential check (`353e940`).
  Fix: retry without the flag on `unknown option` (`runClaudeAuthStatus`).
- Older Git accepted an unknown `--path-format=absolute` option, echoed it on stdout, and exited
  zero, so a worktree repository anchor became `--path-format=absolute\n.git`. Detect the echo,
  retry without the optional flag, and normalize the fallback locally.
- The ACP handshake once logged an incompatible protocol version and proceeded. Version gates fail
  and run the normal shutdown path; they never warn and continue with an unsupported peer.

**Canonical pattern:** `runClaudeAuthStatus` (`internal/backend/credcheck/claude.go`) — try with
optional flags, sniff the error output, retry bare. The `/usability-review` S3 sweep audits every
external `exec.Command` for this class.

---

## 13. Every referenced className must have a defined selector (the test suite can't see CSS)

**Rule:** the UI styles itself with hand-written global CSS (`ui/src/styles/global.css`, tokens in
`tokens.css`) — no Tailwind, no CSS modules. A `className="x"` with **no** matching selector renders
as unstyled default-browser markup. Testing Library never evaluates CSS and MSW/Vitest render into
jsdom, so a whole surface can reference dozens of undefined classes and every unit test stays green;
the breakage is only visible in a real browser. Any TSX className string literal must have a
selector in a stylesheet (utility/state classes applied via template literals are the exception — they
carry their own defined selectors).

Paid for by:
- The first-run onboarding wizard referenced `.dialog-overlay`/`.wizard-*`/`.form-field` that were
  never defined → unstyled soup on first launch (`353e940`).
- **Regression (2026-07-09 usability review):** the same class re-escaped onto the entire
  Settings/config surface — `.settings-tabs*`, `.config-*`, `.backend-card`, `.model-row`,
  `.env-row`, `.string-list`, `.btn-danger/-link/-sm` referenced, defined nowhere; tabs render as
  default buttons and the Backends editor's controls overlap.

**Canonical guard:** the `/usability-review` S2 sweep three-ways the referenced-className set against
defined selectors (both directions) and every journey renders the surface in a real browser and
checks computed styles / stylesheet rule count, not just DOM presence.

---

## 14. A loopback bind is not a browser or filesystem security boundary

**Rule:** binding 127.0.0.1 keeps remote *sockets* out, but not remote *attackers*. A malicious
web page can still reach the server through the victim's own browser (DNS rebinding makes
attacker.com resolve to 127.0.0.1, so the page becomes "same-origin" with the dashboard;
cross-origin WebSocket handshakes and "simple" no-preflight POSTs are sent regardless of CORS
response headers). And any other account on the machine can read world-readable files. Therefore:

- Every HTTP route — API, raw-mounted transports (`/mcp`), WebSockets, static UI — must sit behind
  the `localOnly` guard (`internal/server/security.go`), which rejects non-local `Host` headers
  (DNS rebinding) and non-local `Origin` headers (cross-site WS/CSRF) with 403. The guard wraps
  the **entire mux** in `routes.go`; never mount a handler outside it, and never rely on CORS
  headers as access control — they only gate what a compliant browser lets a page *read*.
- Everything under `~/.agentdeck` (config with backend env/API keys, `state.db`, transcripts,
  hook/MCP token files, logs) is owner-only: `0o700` dirs, `0o600` files (hook scripts `0o700`).
  `MkdirAll` never re-modes an existing dir and SQLite creates files umask-relative, so creation
  paths must pass tight modes AND `EnsureLayout`/`state.Open` explicitly `Chmod` what may already
  exist from older builds.

Paid for by: the 2026-07-11 security review — unauthenticated dashboard API exposed to DNS
rebinding, terminal WS accepting any origin (`InsecureSkipVerify` with no outer check), `/mcp` and
the WS route mounted outside the API middleware, and a world-readable home tree
(`TestDNSRebindingHostRejected`, `TestCrossOriginRequestRejected`, `TestHomeTreeIsOwnerOnly`,
`TestStateDBIsOwnerOnly`, `TestTranscriptIsOwnerOnly`).

**Canonical guard:** `localOnly` + `isLocalHost`/`isLocalOrigin` (`internal/server/security.go`).
Test requests must carry a loopback Host — use `newLocalRequest` (`server_test.go`), not bare
`httptest.NewRequest` (whose default Host, example.com, is rejected by design).

---

## 15. Commit local truth before releasing external side effects

**Rule:** before an operation makes work externally observable or lets an external peer proceed,
persist and publish the local state needed to explain that effect. A retryable mutation spanning
multiple stores or processes must be atomic, roll back every partial effect, or carry a stable
idempotency key that makes replay harmless. Never return a retryable failure after an irreversible
effect if the natural retry can duplicate that effect.

Paid for by:
- Permission approve/deny and timeout answered the ACP peer before writing the resolved-but-active
  status. A fast peer completed the turn and wrote `idle`, then the late local write overwrote it
  back to `busy`. Fix: claim once, publish status and `permission_resolved`, then release the peer
  (`internal/runtime/permission.go`).
- Cancel released a pending permission without emitting `permission_resolved`, so the live UI and
  durable transcript kept a dead Approve/Deny action after the peer had continued. Fix: persist the
  terminal decision before responding.
- annotate-and-assign delivered reserved mail and fired its nudge before appending the source
  annotation event, so a failed append returned 500 after delivery and the preserved-tray retry
  inserted a second copy for the recipient to act on twice (`internal/server/sessions.go`). Fix:
  validate the target and compose its payload, append the durable event, and only then deliver —
  the delivery is deferred into a closure precisely so the ordering cannot drift back.

**Canonical patterns:** durable event/outbox before notification; one transaction when stores share
one database; otherwise compensating rollback or a caller-supplied idempotency key with a uniqueness
constraint. Tests inject the failure after the first would-be side effect and retry the operation.

---

## 16. Bound work and memory where they enter the system

**Rule:** any stream, queue, retained collection, background sweep, or fan-out receiving input of
unknown size must be bounded at admission or capture. Trimming a value only after it accumulated
does not bound memory. State the byte/item limit, in-flight concurrency limit, and lifetime that
apply; admit only that much work, retain only what consumers need, and release it when its last
owner or lifecycle ends (§4).

Paid for by:
- The mail-activation sweep selected every pending row and started one goroutine per row every two
  seconds. It now selects `messaging.ActivationBatch` oldest rows and shares one in-flight cap across
  sweeps, because a stopped-recipient wake can outlive the ticker (`internal/server/messaging_loops.go`).
- Transcript reconciliation kept a permanent second copy of every event and copied the growing
  array on every live delta. `rawByAgent` is now a short-lived append tail that exists only while an
  authoritative fetch is in flight and is cleared on settle, last-surface removal, or agent removal
  (`ui/src/store/transcriptStore.ts`).
- A joining tab restarted the shared SSE stream for every tab while dead ports accumulated for the
  worker's lifetime. The worker now replays one bounded retained snapshot to the joining port,
  releases each port on goodbye/pagehide, and closes the stream with the last port
  (`ui/src/api/sse-shared-worker.ts`).
- Worktree setup used `CombinedOutput`, buffering arbitrary command output before clipping its
  stored tail to 64 KiB. `setupOutputTail` enforces the byte bound while stdout/stderr are produced
  and preserves a valid UTF-8 boundary (`internal/server/worktree.go`).

**Canonical patterns:** bounded tail writers for external output; oldest-first limited queries plus
a process-wide in-flight cap for sweeps; ref-counted registration with per-owner teardown; temporary
reconciliation tails instead of permanent duplicate projections.

---

## 17. Tests must prove their contract independently

**Rule:** a regression, acceptance, agreement, or release test must have an oracle independent of
the implementation it is checking and must be capable of failing when the claimed contract breaks.
Derive the expected surface from a separate authority such as registered producers, the wire
boundary, artifact metadata, or an explicit requirement enumeration — never from a copy of the
table or helper under test. A regression test is confirmed against the pre-fix behavior when that
check is practical. Test doubles mirror the real producer's failure modes and serialized shapes,
not the idealized contract.

Paid for by:
- The retry-classification guard compared one hand-maintained classifier table with another list
  that omitted the same pipeline refusals. It passed while emitted errors had no declared class;
  the replacement derives emitted literals from producer files and enumerates forwarded dynamic
  pipeline codes (`internal/messaging/tool_result_contract_test.go`).
- A mail-activation test asserted a condition that could not fail, leaving its stated guarantee
  unproven until the assertion was made against the actual durable rows and provider prompts.
- The AgentDecker migration's unreadable-file case exercised decode corruption instead, then
  skipped the whole case as root. Corrupt content, a real read-I/O failure, and write failure now
  have separate fixtures (`internal/config/config_test.go`).
- Release CI passed `-tags sqlite_fts5` only to packages with no tagged implementation and never
  inspected the shipped binary. The gate now tests the FTS packages and checks the packaged
  executable's build metadata (`.github/workflows/release.yml`).
- The server once marshaled an empty collection as `null` while its MSW double returned `[]`, so
  every UI test passed against a payload the server did not produce (§11).

**Canonical patterns:** enumerate producers independently from their policy table; drive the real
wire/serialization boundary; make fixtures trigger the named OS or storage failure; inspect built
artifacts rather than trusting build commands; for a bug regression, demonstrate the test fails on
the old behavior before relying on it.

---

## Canonical helpers registry (reuse, don't re-derive)

| Helper | Where | Use for |
|---|---|---|
| `teardownAgentRegistration` | `internal/server/launch.go` | every agent exit/failure path (§4) |
| `composeLaunch`, `composeResumeSpec`, `composeSwitchSpec`, `resolveSkip`, `expandAddDirs`, `composeEnv` | `internal/server/{launch,resume,switch}.go` | any path that builds/rebuilds a LaunchSpec (§2) |
| `takePending` | `internal/runtime/permission.go` | atomic claim for racy resolutions (§5) |
| `SubscribeWithSnapshot` | `internal/bus/bus.go` | any snapshot+subscribe consumer (§5) |
| `runtime.ReapOrphan` | `internal/runtime/` | stopping agents the registry doesn't own (§4) |
| PTY hub pattern | `internal/runtime/terminal/ptyhub.go` | any shared-fd broadcast need (§6) |
| `seedLocked` | `internal/index/indexer.go` | in-memory buffers feeding replace-style writes (§9) |
| `config.ValidSlug` | `internal/config/validate.go` | every path-param on every verb (§2) |
| `foldTranscript` / `appendRenderedEvent` | `ui/src/store/transcriptStore.ts` | identical live and replay event projection (§2) |
| `transcript.ProjectEvent` / `runtime.AllEventTypes` | `internal/transcript`, `internal/runtime` | every Go transcript text consumer and exhaustive normalized-event coverage (§2) |
| `clipAnnotationExcerpt` | `ui/src/lib/annotations.ts` (server copy authoritative: `internal/server/sessions.go`) | every UI surface that captures an annotation excerpt (§2) |
| `localOnly` | `internal/server/security.go` | wraps the whole mux; every new route inherits it (§14) |
| `notificationPayload` | `internal/bus/` | all notification payloads (§8) |
| `fakeacp` test double | `internal/runtime/testdata/fakeacp` | env-driven protocol-level repros (`FAKEACP_LOAD_DUMP`, `FAKEACP_PROTO_VERSION`, `ignore_cancel`) |
