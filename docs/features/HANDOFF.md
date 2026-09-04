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
  records, and handoff/queue bookkeeping are administrative closure, not new review units.
  Two units are available independently: the 2026-09-04 task launch-specification effort change and
  the 2026-09-04 proposal Reject/Delete change. A review may take either; chronology and other role
  queues do not constrain selection. The workflow/queue repair has been reviewed and remains open
  in the fix queue with the findings below.
- **Work units:** `pipeline-run-agent-groups.md` is waiting to start. It may start regardless of the
  available review units.
- **Design units:** Existing entries under `Ideas being defined` may resume, and entries under `New
  ideas` are available to start. Neither is gated by work, review, or fix state.
- **Open findings:** Two **Must fix** findings from the review of the 2026-09-04 workflow/queue
  repair.
- **State:** Automated MCP contract verification is green. Pinned Claude/Codex live-provider checks
  remain owed before claiming those adapters accept structured results.
- **Usability state:** The Pipelines pages and dashboard grid were driven through a real Chromium on
  2026-08-30 against a `make dist` build of the shipped tree; that run is closed except for the
  acceptance gates below.
- **Branch:** `main`.

## Active change

**Change:** None. Proposal Reject/Delete is implemented and verified.

**Available by role:** `/review` may choose either review unit above; `/work` may start
`pipeline-run-agent-groups.md`; `/design-feature` may choose any available or resumable idea; `/fix`
may close the workflow/queue-repair findings below. Selecting one role does not depend on clearing
another role's queue.

Local implementation choices for those reviewers to confirm or raise. In the effort change: launch
composition calls `selectLaunchTarget` and `resolveTargetEffort` in place rather than the composed
`resolveLaunchSpec`, because a bound configuration source must be resolved between selecting the
model and resolving the effort; and the `tasks.effort` column ships as `TEXT NOT NULL DEFAULT ''` to
match the effort columns migrations 12 and 13 added, with TS-10 §3 reworded to that shipped shape
rather than the "nullable" it drafted. In the proposal change: `declined_at` ships as
`TEXT NOT NULL DEFAULT ''` for the same reason, matching `consumed_at` from migration 15, and
TS-02.R29 is reworded to that shipped shape; the list route returns every non-consumed record and
the manager splits pending from declined rather than the store running two queries; the UI parses
each listed record permissively and narrows it with `pipelineProposalSchema`, so a payload that does
not match its kind still lists with its kind and id and only its approval action is disabled
(FS-14.R51); and on Templates the builder panel still renders above the template library, so A28's
"library remains the surface's first content" is satisfied by the collapse rather than by
reordering the page.

Next ready change: [`pipeline-run-agent-groups.md`](../ready-changes/pipeline-run-agent-groups.md).
Keep credentialed provider journeys as acceptance gates until a human authorizes real sessions.

## Changelog

Earlier entries are in the [archived handoff](../archive/state/HANDOFF-through-2026-09-03.md) and Git
history.

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
- **Refused card drag feedback:** Confirm whether the cross-block refusal should remain an in-flight
  pointer signal (FS-02.R53) or whether snap-back alone is the intended behavior. The shipped pointer
  implementation is still owed its real-browser computed-cursor pass, which is an acceptance gate below.
- **Detecting a pipeline stage that can no longer advance:** A run parked at `await_result` with a
  silent stage agent is invisible today, and that cost the 2026-09-02 report about nineteen hours.
  Fixing it needs your call on what qualifies, because a stage agent legitimately ends many turns
  while its delegated children work: an idle stage agent with no running delegated tasks, an elapsed
  time threshold, or an explicit heartbeat from the stage. Also say whether the result is a new
  attention reason under FS-14.R29 — which puts it in the same notification category as blocked and
  crash — or a weaker run-page disclosure like FS-14.R40's delegated-agent count.
- **How long a stopped agent stays unaddressable after its pipeline stage:** FS-06.R22 excludes
  pipeline-associated agents from the addressable set while stopped, and the association is a
  `pipeline_attempts` row that lives as long as its run record. So an agent that ran one stage of a
  run that finished weeks ago can never be sent a message or a task again unless someone resumes it
  or deletes the run. The rule's stated reason — a pipeline stage agent was deliberately stopped by
  its state machine, so no message may revive it — only applies while that state machine still owns
  it. Confirm that the permanent version is intended, or scope the exclusion to a run that is still
  active, which changes FS-06.R22 and `stoppedWakeGates`.
- **Reactivating a worktree project after a consented checkout deletion:** FS-19 did not say what
  happens, and the review found its two paths answered in opposite ways. The smaller reading shipped
  on 2026-09-02 and is now stated in FS-19 §3: accepting the deletion ends AgentDeck's ownership, so
  restoring the project later gives an ordinary missing-directory error and nothing is recreated —
  recreating a checkout the person just chose to delete would undo their decision, and the branch is
  still there to fork again from. Confirm, or say that restore should re-materialize the checkout,
  which needs the ownership row to survive the deletion.

## Acceptance gates

Gates closed on or before 2026-08-30 are in the archived handoff.

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

Live-provider acceptance is waiting for human authorization because it invokes real provider sessions
and creates disposable local configuration homes. On 2026-07-15 this machine has Claude Code 2.1.202,
the retired `claude-code-acp`, Codex CLI 0.142.5, and `codex-acp` 1.1.2 installed; the new
`claude-agent-acp`, OpenCode, and OpenHands are not installed globally.

## Review findings

- **Must fix** — 2026-09-04 workflow/queue repair, `.agents/skills/design-feature/SKILL.md:19` and
  `docs/features/AGENT-WORKFLOW.md:203` (INV §10): when a person explicitly invokes
  `/design-feature` with an entry from `docs/ideas.md`'s `Known things to improve` section, the new
  rules recognize only `New ideas` and `Ideas being defined`. That strands the repository's recorded
  improvement as an unrecognized source or treats the prompt as a fresh idea, duplicating it instead
  of moving the existing entry through design. Restore explicit named selection from any real
  `docs/ideas.md` section while keeping no-argument automatic selection limited to available or
  resumable ideas, mirror the Claude launcher, and cover the distinction in the launcher contract
  check.
- **Must fix** — 2026-09-04 workflow/queue repair,
  `scripts/check-launcher-contract.sh:32` (INV §17): the changed review-launcher assertions do not
  independently prove the rules they claim. The no-op check searches only for `make an empty
  commit.`, so the opposite instruction passes; the selection check accepts any sentence containing
  `Chronology does not` without requiring the launcher to choose any available review unit. A later
  launcher regression can therefore reintroduce empty review commits or multi-unit prompting while
  this gate stays green. Assert the complete negative no-op rule and the positive any-unit selection
  rule, with contradiction fixtures or equivalent mutation evidence showing each assertion fails on
  the old behavior.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. One range is wrong; align them
  when that paused change resumes.
- FS-17 §6 opens with “The contract is shipped. Live-provider compatibility remains tracked as
  acceptance gate A6,” which reads as covering the whole section, but §6 also carries the planned
  direct-cutover boundary for R13–R19. Scope the opening sentence when that planned work resumes.
