# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Open the other sections only when those point at them. Settled state is
archived in [`../archive/state/HANDOFF-through-2026-09-03.md`](../archive/state/HANDOFF-through-2026-09-03.md)
and [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only and is cut
back to that at every release (§16.7). Injected Current position plus Active change budget: 8 KiB.

## Current position

- **Active change:** None.
- **Release:** `v0.4.0` is published and verified on tag `5485cd7`. Credentialed Claude/Codex checks
  remain owed under TS-06.R21. A customized `agentdecker` role is deliberately not migrated under
  FS-04.R44, so it keeps the superseded product manual beside the current skill.
- **Review units:** The pre-2026-09-04 raw commit queue is retired. Its substantive changes were
  reviewed and their findings are closed; its later review records, finding-fix commits, release
  records, and handoff/queue bookkeeping are administrative closure, not new review units. The
  2026-09-04 task launch-specification effort unit is closed: its three findings are fixed, and that
  fix commit is part of the closure rather than a new review unit. The
  2026-09-04 proposal Reject/Delete unit is closed: both must-fix findings are fixed, and that fix
  commit is part of the closure rather than a new review unit. The
  stage-agent grouping unit is reviewed and closed. The workflow/queue repair unit is closed: its
  findings are fixed, and that fix commit is not a new review unit.
- **Work units:** None waiting to start. `migrate-internal-actions-from-mcp.md` stays paused on its
  recorded transport blocker.
- **Design units:** Existing entries under `Ideas being defined` may resume, and entries under `New
  ideas` are available to start. Neither is gated by work, review, or fix state. The operator called
  the permanently unaddressable pipeline agent broken on 2026-09-05; it is the newest `New ideas`
  entry and changes FS-06.R22, so it needs `/design-feature` before any code.
- **Open findings:** None.
- **State:** Automated MCP contract verification is green. Pinned Claude/Codex live-provider checks
  are still unrun, so no agent may claim those adapters accept structured results — but they no
  longer block any role (see **Acceptance gates**).
- **Usability state:** The Pipelines pages and dashboard grid were driven through a real Chromium on
  2026-08-30 against a `make dist` build of the shipped tree; that run is closed except for the
  acceptance gates below.
- **Branch:** `main`.

## Active change

**Change:** None. The proposal Reject/Delete unit is implemented, reviewed, fixed, and closed.

**Available by role:** `/review` has no unreviewed unit to take; `/fix` has no open findings; `/work`
has no
ready change waiting to start; `/design-feature` may choose any available or resumable idea, or any
idea a person names from another `docs/ideas.md` section. Selecting one role does not depend on
clearing another role's queue.

Credentialed provider journeys stay recorded as open acceptance gates, but on 2026-09-05 the
operator ruled they block no role. Never report them as verified; do not wait on them either.

## Changelog

Earlier entries are in the [archived handoff](../archive/state/HANDOFF-through-2026-09-03.md) and Git
history.

- **2026-09-05 — fix: proposal Reject/Delete findings (INV §5, INV §8, INV §17):** Delete's
  conditional claim now requires `declined_at != '' AND consumed_at = ''`, so the ordinary
  Reject → approval → stale Delete order loses with the durable `consumed` refusal instead of
  erasing the record the approval consumed (INV §5); TS-02.R29 states that predicate and why the
  unconsumed clause belongs in it, and its two neighbouring claims are corrected to the
  empty-string convention the code actually uses. A storage regression declines, consumes, refuses
  the Delete, and reads the row's own marks rather than the projection that hides a consumed record;
  the server refusal table gains the consumed-Delete case and proves a second stale Delete still
  answers `consumed` rather than gone (INV §17). Reject/Delete failure text moved from the proposal
  card to the builder panel that outlives the durable refetch, and the panel stays mounted while a
  message is unread, so a consumed refusal that empties both collections still explains itself
  (INV §8). The UI cases now model the list state each refusal really produces — consumed in
  neither collection, already-declined in the declined one — and all three fail against the previous
  card-owned message. Both Go variants, the full server package, all UI tests, and the production UI
  build pass. The proposal Reject/Delete unit is closed.
- **2026-09-04 — review: collapse, reject, and delete pending pipeline proposals (INV §§2, 5,
  7–11, 13–17):** The storage shape, one-query projection, permissive per-record UI narrowing,
  collapsed placement above the template library, route wiring, bounded summaries, and
  post-commit event publication match the requirements. Two normal multi-tab paths remain wrong.
  Delete's conditional claim accepts a row that approval already consumed after Reject, so a stale
  Delete erases that consumed record instead of losing with the durable `consumed` state. Separately,
  Reject/Delete failures live inside the proposal card while their settled hook awaits a durable
  refetch; when that refetch removes or moves the card, the required refusal message disappears with
  it. The tests omit the consumed-Delete state and model a consumed Reject refusal while incorrectly
  keeping its proposal pending, so neither regression is exposed. The proposal unit remains open
  with two must-fix findings. Focused state, pipeline, server, and UI tests pass; the broader server
  package run is blocked in this sandbox by an unrelated IPv6 `httptest` listen denial. Invariant
  classes 1, 3–4, 6, and 12 have no applicable changed surface.
- **2026-09-04 — fix: task launch-specification effort findings (INV §2, INV §9, INV §11, INV
  §17):** TS-10.R23 now describes the topology that ships: selecting the concrete backend/model
  target and resolving the effort over it are two shared seams, launch composition calls the
  selection seam, resolves the backend's bound configuration source, then calls the effort seam with
  that source's override, and the two authoring paths call the one composed helper that runs both
  back to back (INV §2). TS-10 §3 states the exact `TEXT NOT NULL DEFAULT ''` shape of the effort
  columns and says why a nullable column would fail a task's read, and a new
  `PRAGMA`-driven assertion in `internal/state` pins that shape on the migrated database rather than
  on the migration text (INV §9, INV §11, INV §17). FS-16.A18 is now proven at the provider
  boundary: a task-dispatch case captures the `session/set_config_option` call the adapter actually
  receives against a bound source carrying a lower effort override, asserts the explicit task effort
  wins there, and its second case leaves the task's effort unset so the override takes force —
  making "beats the override" non-vacuous. That case fails when the provider-facing
  `runtime.LaunchSpec.Effort` is dropped, which the pre-existing persisted-projection case did not.
  Every materially touched dispatcher, HTTP, MCP, and Tasks-view test now carries the exact
  `FS-16.A18` comment TS-06.R6 requires. The task launch-specification effort unit is closed.
- **2026-09-04 — review: task launch-specification effort (INV §§2, 3, 7–11, 14–15, 17):** Runtime
  behavior is sound across HTTP, MCP, persistence, dispatch, and the Tasks view, but TS-10.R23 still
  claims launch composition calls the composed triple resolver when federation requires it to call
  the shared selection and effort seams around source resolution; TS-10 §3 does not state the
  shipped `TEXT NOT NULL DEFAULT ''` storage shape; and FS-16.A18 is not independently proven at the
  composed provider boundary or against a bound-source override, with its materially touched tests
  also lacking the exact acceptance id TS-06.R6 requires. The task-effort unit remains open with
  three worth-fixing findings. Shared construction, persisted-value handling, read/migration safety,
  UI errors, SQLite behavior, wiring, serialization, HTTP guarding, and effect ordering otherwise
  satisfy their triggered invariants; classes 1, 4–6, 12–13, and 16 have no applicable changed
  surface. Both Go variants, the tagged build, all UI tests, and the production UI build pass. The
  proposal Reject/Delete review unit remains available independently.
- **2026-09-04 — review: pipeline stages group their own agents (INV §§1, 2, 10, 11, 15, 17):** No
  findings. The stage label is composed once as the agent name, passed through the existing launch
  seam as the ordinary persisted group, and rendered by the existing dashboard grouping behavior;
  the durable run/stage association remains the only membership authority. Retry coverage proves a
  repeated stage reuses the label, the server boundary proves the label is stored, and the grid plus
  ordinary-stop coverage proves the promised group and recovery behavior. Lifecycle, shared-helper,
  wiring, boundary-meaning, effect-ordering, and independent-test invariants are satisfied; classes
  3–9, 12–14, and 16 have no applicable changed surface. The full Go test matrix, tagged build, UI
  tests, and UI production build pass. The stage-agent grouping review unit is closed; the other two
  review units remain available.
- **2026-09-04 — build: pipeline stages group their own agents (FS-14.R58, A33; TS-09.R33):** Every
  agent a run launches now arrives carrying its stage's label — the stage title and the run's
  display name — as its ordinary task group, so a stage's work reads as one collapsible dashboard
  section with the count, per-state summary, persisted collapse, and **Release group** any group
  has, and a retried or loop-revisited stage collects its later agents in the same section. The
  whole mechanism is `LaunchStage` passing the string `stageExecution` already composes for the
  agent's name as the existing `launchRequest.Group`, so the convention has one home and the name
  and label cannot drift (INV §2). No new field, column, migration, API shape, or grid code, and no
  pipeline awareness in the dashboard. FS-14.R16 is unchanged: the run/stage association stays the
  only authority for run membership, and clearing a label removes an agent from a section, never
  from its stage. With their last `(planned)` items shipped, FS-14 and TS-09 are back to
  **Current**; the ready change file is removed.
- **2026-09-04 — fix: workflow/queue repair findings (INV §10, INV §17):** `/design-feature` again
  takes an idea a person names from any `docs/ideas.md` section, including `Known things to
  improve`, and moves that recorded entry through design instead of duplicating it; automatic
  selection with no argument stays limited to `New ideas` and `Ideas being defined` (INV §10, both
  launchers and workflow §11). The launcher contract check now asserts the complete no-op rule
  ("report that and do not make an empty commit.") and the positive any-unit review selection
  instead of fragments an opposite instruction would satisfy, matching rules across line wraps, and
  `scripts/check-launcher-contract-test.sh` proves each guarded rule fails on a mutated launcher or
  workflow copy that states its opposite (INV §17). `make check-specs` runs that mutation test;
  TS-06 records both scripts. The workflow/queue-repair unit is closed.
- **2026-09-04 — review: workflow/queue repair (INV §§10, 17):** The independent role queues and
  mirrored Claude/Codex launchers are internally consistent, but the design launcher no longer
  recognizes an explicitly named entry under `Known things to improve`, and the launcher contract
  check can accept contradictory review rules. The originating unit remains open with two findings.
- **2026-09-04 — build: collapse, reject, and delete pending pipeline proposals (FS-14.R49–R51, R57,
  A27–A28; TS-02.R29; TS-03.R36; TS-09.R32):** A pending proposal now lists collapsed — kind,
  template title, stage count or run name and goal, and how long it has been pending — and expands
  to the exact canonical payload an approval acts on, so a 32-stage draft no longer pushes the
  template library off the screen. **Reject** moves an offer to a **Declined** list with its decline
  time and a **Delete**; migration 22 adds `pipeline_proposals.declined_at`, and Reject, Delete, and
  consumption are each one conditional claim whose `WHERE` names the state it expects, so a race
  produces one effect and every loser is told what actually happened (INV §5). The durable mutation
  still wins: consumption matches `consumed_at` alone and may overwrite a decline, and the approval
  paths gained no proposal id, no pre-check, and no cross-store transaction. Re-proposing identical
  content clears a decline as well as a consumption, so a re-proposal leaves exactly one pending
  offer. TS-02 is back to **Current**; the ready change file is removed.
- **2026-09-04 — build: task launch-specification effort (FS-16.R2, R27–R28, A18; FS-09.R49, A22;
  TS-10.R23, §3):** A task that launches its own agent can now name the reasoning effort that agent
  runs at, over `create_task`, `POST /api/tasks`, and the Tasks create form. Migration 21 adds
  `tasks.effort`; the dispatcher passes it as the existing `launchRequest.Effort`, so `resolveEffort`
  and `config.ValidateModelEffort` stay the only precedence and validation code. The same change
  validates a launch specification's backend, model, and effort when the task is created — resolving
  the install defaults for an omitted field — instead of letting a bad specification spend all three
  start attempts, and rejects an effort named beside an existing-agent target. Launch composition was
  refactored onto the shared `selectLaunchTarget`/`resolveTargetEffort` seam the authoring paths use,
  so selection and effort validation each exist once (INV §2). FS-16 and TS-10 are back to
  **Current**; the ready change file is removed.
- **2026-09-04 — design: pipeline stages group their own agents (FS-14.R58, TS-09.R33):** A run now
  labels every stage agent it launches with that stage's title and the run's display name as an
  ordinary task group, so a stage's work lands in one collapsible dashboard section with the
  existing count, collapse, and **Release group** behavior instead of scattering through Ungrouped.
  The whole mechanism is one composed string on the existing `launchRequest.Group` at one call site;
  a guard for reused agent ids and one for empty labels were both dropped after the code showed them
  unreachable. FS-14.R16 is unchanged: the run/stage association stays the only authority for run
  membership. FS-14 and TS-09 were already Partial; the work is queued as
  `docs/ready-changes/pipeline-run-agent-groups.md` and is not active.
- **2026-09-04 — design: task launch-specification effort (FS-16.R27–R28, FS-09.R49, TS-10.R23):**
  Specified an optional reasoning effort on a task's launch specification, offered by `create_task`,
  `POST /api/tasks`, and the Tasks create form, stored on the task row and applied as FS-09.R41's
  explicit request when the task's agent is launched. The same design validates a task's backend,
  model, and effort at creation instead of letting a bad specification spend all three start
  attempts, and rejects an effort supplied with an existing-agent target. FS-16 and TS-10 moved to
  Partial; the later implementation is now an available review unit.
- **2026-09-04 — workflow repair: terminal review units (INV §10):** Review state now follows a
  substantive change from implementation through its review findings and fixes. Review reports,
  finding-fix commits, release records, and handoff/archive/queue bookkeeping cannot re-enter the
  review queue, and an empty queue produces no state commit. Role queues are independent and may
  each hold multiple units; an invocation may choose any available unit without chronological or
  cross-queue gating. Claude and Codex launchers carry the same rules, and the launcher check
  enforces independent selection, administrative exclusions, no-op exit, fix closure, and twin
  parity. The stale raw commit ledger and settled finding were removed from this handoff.

## Decisions needing your input

These are product decisions needed for a future change or shipped boundaries whose reversal needs
an explicit specification update. Remove an item when the human resolves it or queues that update.

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether the chat should
  remain reachable with a wider continuation contract.
Resolved on 2026-09-05: the refused card drag stays as shipped (FS-02.R53's in-flight pointer
signal); worktree restore after a consented checkout deletion stays as shipped (FS-19 §3, no
re-materialization); the stalled-`await_result` detection item is withdrawn, because the
2026-09-02 report's nineteen hours were operator absence rather than a missing signal — reopen it
only if a run is ever seen parked with a live, healthy, idle stage agent; and the permanently
unaddressable pipeline agent is not a decision but a defect, now queued as a new idea in
`docs/ideas.md`.

## Acceptance gates

Gates closed on or before 2026-08-30 are in the archived handoff.

**Not blocking as of 2026-09-05.** The operator decided that every gate below is presumed working
and that real breakage will be reported as it is found. None of them have been run, so no agent may
describe them as verified, passed, or closed, and specification status that depends on one — FS-08
and TS-07 remain Partial, FS-17.A6/A9 and FS-03.A26 remain unmet — does not change. What does change
is that no role waits on them: work, review, fix, design, and release may all proceed with these
open. Restore blocking status only on an explicit operator instruction.

- [ ] Run FS-03.A26/J14 with a pinned real chat provider: an AgentDeck stage-result action proceeds
  without a prompt, a file edit still prompts, and approval after more than three minutes continues
  the same stage. Automated exact-identity, fail-closed, no-default-deadline, and attention checks
  pass; this rendered provider boundary needs human authorization.
- [ ] Run pinned, credentialed Claude and Codex chat/MCP/resume checks before claiming those combinations.
- [ ] Run pinned Claude terminal flags/hooks and live xterm journeys before claiming full terminal support.
- [ ] Run pinned OpenCode/OpenHands launch/credential checks before claiming those backends beyond fakes.
- [ ] Run J2/J9/J16 in a real macOS browser to confirm the native folder panel opens in front,
  selects, and cancels (FS-04.A22). Narrowed on 2026-08-27: a real browser confirmed the **Browse…**
  controls are present and enabled for `cwd` and the pending `add_dirs` entry in both the Settings
  project form and the New project modal, and that the onboarding wizard renders styled. Only the
  native `osascript` panel itself is still unverified, and it needs a human at the machine.
- [ ] Drive a chat agent into a permission request with the dashboard on screen in a real browser
  and confirm its pane opens by itself, that only the rows below it move and no card changes column
  (FS-02.R55/A43, J5), that a reload with that agent still waiting opens nothing, and that a fifth
  waiting agent's eviction of the least-recently-used pane is visible rather than silent with the
  evicted draft returning on re-expansion. jsdom evaluates no layout, so unit cases cannot close this.
- [ ] Drag a running card over the stopped block in a real browser and confirm the computed cursor
  on the card under the pointer states the refusal, clears when the pointer returns to its own block,
  and clears when the drag ends (FS-02.A35, J5). jsdom evaluates no CSS, so unit cases cover only the
  marked state and stylesheet rule.
- [ ] Run a task start, an assignment turn, and a reported result against the pinned Claude and Codex
  adapters before claiming dependent work works with real providers (FS-16 §6).
- [ ] Run one successful and one refused MCP tool call through pinned Claude and Codex adapters before
  claiming they accept structured tool results without losing the text block (FS-17.A6).
- [ ] Run the Phase 7 federation discovery/precedence/refresh/launch/resume matrix against real Claude
  and Codex installations before promoting FS-08/TS-07 from Partial.
- [ ] Run J16's worktree steps in a real browser against a `make dist` build (FS-19.A1, FS-02.A42):
  the card-menu and scoped-header entry points, the pre-filled creation form, the new card appearing
  with its branch without a manual refresh, and an agent launched into the new checkout. The API half
  was driven end to end against a real repository with the built binary on 2026-09-02; only the
  rendered surface is unverified.
- [ ] Run FS-19.A4's manual gate: archive a worktree project holding uncommitted work in a real
  browser and confirm the dialog defaults to keeping, names the uncommitted state, and that accepting
  removes the checkout while the branch survives.
- [ ] Run the six-tab same-origin dashboard check against a `make dist` build (FS-02.A27). The
  transport half is covered by `ui/src/api/sse.test.ts`; the browser half has never been run against
  a build carrying the shared stream. `scripts/stress-fixture` (TS-06 §6) is the fixture.

## Blocked on human

Nothing. Live-provider acceptance still needs human authorization to run, because it invokes real
provider sessions and creates disposable local configuration homes — but as of 2026-09-05 the
operator chose not to run it and not to let it block any role, so it is an open acceptance gate
rather than a blocker. On 2026-07-15 this machine has Claude Code 2.1.202, the retired
`claude-code-acp`, Codex CLI 0.142.5, and `codex-acp` 1.1.2 installed; the new `claude-agent-acp`,
OpenCode, and OpenHands are not installed globally.

## Review findings

No open findings.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. One range is wrong; align them
  when that paused change resumes.
- FS-17 §6 opens with “The contract is shipped. Live-provider compatibility remains tracked as
  acceptance gate A6,” which reads as covering the whole section, but §6 also carries the planned
  direct-cutover boundary for R13–R19. Scope the opening sentence when that planned work resumes.
