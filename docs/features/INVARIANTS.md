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

**Canonical helpers:** `composeLaunch`, `composeResumeSpec`, `composeSwitchSpec`, `resolveSkip`,
`expandAddDirs`, `composeEnv`
(`internal/server/launch.go`); keep `sessionNewParams`/`sessionLoadParams` in lockstep;
`config.ValidSlug` on **every** verb of every path-keyed resource.

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
- Every className shipped in a component must resolve to a defined selector — `npm run build` and
  Testing Library are both blind to CSS, so a missing stylesheet is green everywhere and unusable
  on screen. The onboarding wizard/dialogs shipped referencing `.dialog-overlay`/`.wizard-*`/
  `.form-field` with no definitions anywhere (`353e940`). The `/usability-review` S2 sweep audits
  this both directions (referenced-but-undefined, defined-but-unreferenced).

## 11. Cross-boundary serialization contracts: nil marshals to `null`, and mocks must tell the truth

**Rule:** Go nil slices/maps marshal to JSON `null`, not `[]`/`{}`. Any collection field the UI
iterates must be non-nil at the marshal boundary (initialize with `make`/literal, or
`append([]T{}, …)` — **never** `append([]T(nil), …)`, which stays nil for empty input), and the UI
API layer defends with `?? []` regardless. Symmetrically: **test doubles must mirror what the real
marshaler emits, not the idealized contract** — an MSW mock returning `[]` where the server sends
`null` makes every UI test pass against a server that doesn't exist.

Paid for by:
- `layoutFromConfig`/`toConfig` (`internal/server/handlers.go`) built `Order` via
  `append([]string(nil), l.Order...)` → fresh install served `order: null` →
  `CardGrid.tsx`/`agentStore.ts` called `.filter`/`.includes` on it → TypeError, dead dashboard on
  first launch (`353e940`). The MSW fixtures returned `order: []`, so no UI test could see it.

**Canonical patterns:** `append([]T{}, src...)` at marshal boundaries; `?? []` where the UI first
touches a server collection; when adding a response field, add the null-shape case to the mock.

## 12. External-CLI invocations must tolerate version and environment variance

**Rule:** any `exec.Command` of a user-installed tool (agent CLIs, probers) runs against whatever
version the user has, not the one the author tested. Optional flags need a detect-and-retry
fallback; output parsing is defensive (substring vocabulary, not exact format); a tool that can't
be interrogated reports "unknown"/"skipped", **never** "failed" — a wrongly failed gate blocks the
user harder than no gate at all.

Paid for by:
- `internal/backend/credcheck/claude.go` ran `claude auth status --no-color`; older Claude builds
  don't have `--no-color`, so a logged-in user failed the onboarding credential check (`353e940`).
  Fix: retry without the flag on `unknown option` (`runClaudeAuthStatus`).

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
| `localOnly` | `internal/server/security.go` | wraps the whole mux; every new route inherits it (§14) |
| `notificationPayload` | `internal/bus/` | all notification payloads (§8) |
| `fakeacp` test double | `internal/runtime/testdata/fakeacp` | env-driven protocol-level repros (`FAKEACP_LOAD_DUMP`, `FAKEACP_PROTO_VERSION`, `ignore_cancel`) |
