# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Open the other sections only when those point at them. Settled state is
archived in [`../archive/state/HANDOFF-through-2026-09-06.md`](../archive/state/HANDOFF-through-2026-09-06.md),
[`../archive/state/HANDOFF-through-2026-09-03.md`](../archive/state/HANDOFF-through-2026-09-03.md),
and [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only and is cut
back to that at every release (§16.7). Injected Current position plus Active change budget: 8 KiB.

## Current position

- **Active change:** None.
- **Release:** `v0.4.1` is published and verified on tag `0454209`. Release run `34015979034`
  succeeded in 3m2s and attached the archive, `install.sh`, and a `manifest.json` carrying the
  archive's SHA-256; the `main` CI run passed alongside it. The operator chose the patch number
  knowing the range also carries new user-visible capability. Credentialed Claude/Codex checks
  remain owed under TS-06.R21 and were not run. A customized
  `agentdecker` role is deliberately not migrated under FS-04.R44, so it keeps the superseded
  product manual beside the current skill.
- **Review units:** None. Every unit through the `v0.4.1` range is reviewed and closed. Review
  records, finding-fix commits, release records, and handoff/archive/queue bookkeeping are
  administrative closure and never re-enter the queue.
- **Work units:** `dock-the-annotation-tray-and-quiet-its-prompt.md` is waiting to start (FS-13.R20–R23,
  TS-08.R53–R54). `migrate-internal-actions-from-mcp.md` stays paused on its recorded transport blocker.
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

**Change:** None. `v0.4.1` closed the epoch: every change in its range is implemented, reviewed,
fixed, and closed.

**Available by role:** `/review` has no unreviewed unit to take; `/fix` has no open findings; `/work`
may take `dock-the-annotation-tray-and-quiet-its-prompt.md`, the one
ready change waiting to start; `/design-feature` may choose any available or resumable idea, or any
idea a person names from another `docs/ideas.md` section. Selecting one role does not depend on
clearing another role's queue.

Credentialed provider journeys stay recorded as open acceptance gates, but on 2026-09-05 the
operator ruled they block no role. Never report them as verified; do not wait on them either.

## Changelog

Entries through the `v0.4.1` epoch are in the
[archived handoff](../archive/state/HANDOFF-through-2026-09-06.md); earlier ones are in the
[`v0.4.0` archive](../archive/state/HANDOFF-through-2026-09-03.md) and Git history.

- **2026-09-07 — design: dock the annotation tray and quiet its prompt.** The operator's request to
  move the annotation window right, enlarge it, make each draft readable, and cut annotation meta
  from the conversation is specified as FS-13.R20–R23 / A12–A14 and TS-08.R53–R54, and waits in
  `docs/ready-changes/dock-the-annotation-tray-and-quiet-its-prompt.md`. Decisions taken with the
  operator: the tray docks as a right-hand column and falls back to today's overlay through a
  container query on the transcript region — not a viewport media query — so the narrow dashboard
  chat pane keeps the overlay inside a wide window; the column is collapsible and the flag rides the
  existing per-source annotation-draft persistence; and the self-target annotation block is
  suppressed at render time in `appendRenderedEvent`, which quiets transcripts recorded before it
  ships and leaves the event, the endpoint, and the index untouched. Declined on 2026-09-06:
  stripping the annotation card's `Event <seq>` anchor and resolving its raw target agent id
  (FS-13 §6). The third complaint — that the annotation **New task** target lacks model/effort — was
  withdrawn once `NewAgentModal` was shown to render Backend, Model, and Effort already. FS-13 and
  TS-08 are now `Partial`. No product code changed.
- **2026-09-06 — release: `v0.4.1`:** Cut from the range `v0.4.0..main`: a task's launch
  specification can name its reasoning effort and is validated when the task is created, pending
  pipeline proposals list collapsed and can be rejected and deleted, each pipeline stage's agents
  land in their own dashboard group, and a crash-teardown generation race and the release CI's FTS5
  verification were fixed. The shipped `operating-agentdeck` package gained one correction: its
  coordination reference now states that a task's launch target chooses backend, model, and effort
  once, that AgentDeck refuses a bad specification at creation rather than spending the task's start
  attempts, and that an effort named for an existing-agent target is refused (TS-11.R8, FS-16.R27–R28).
  The proposal Reject/Delete and stage-grouping changes carry no agent-facing payload change, so
  they needed none. README, `install.sh`, and `scripts/release/assemble.sh` claims were not
  falsified by the range. Both Go variants, `make check-specs`, all UI tests, and
  `make dist VERSION=0.4.1` pass; the built binary reports `0.4.1` and carries `sqlite_fts5`.
  Credentialed Claude and Codex checks remain owed, not run. Published from tag `0454209`: release
  run `34015979034` verified archive contents, FTS5 tagging, pinned components, checksum rejection,
  and a fresh installation (TS-06.R21), and the GitHub Release carries the archive, `install.sh`,
  and the manifest with its SHA-256.

## Decisions needing your input

These are product decisions needed for a future change or shipped boundaries whose reversal needs
an explicit specification update. Remove an item when the human resolves it or queues that update.

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether the chat should
  remain reachable with a wider continuation contract.

The items the operator resolved on 2026-09-05 are in the
[archived handoff](../archive/state/HANDOFF-through-2026-09-06.md).

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
