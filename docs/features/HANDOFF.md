# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in
[`../archive/state/HANDOFF-through-2026-09-07.md`](../archive/state/HANDOFF-through-2026-09-07.md),
[`../archive/state/HANDOFF-through-2026-09-06.md`](../archive/state/HANDOFF-through-2026-09-06.md),
[`../archive/state/HANDOFF-through-2026-09-03.md`](../archive/state/HANDOFF-through-2026-09-03.md),
and [`../archive/state/HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only.

## Current position

- **Active change:** None.
- **Release:** `v0.4.2` is published and verified on tag `f56755a`. Release run `34085524105`
  succeeded in 4m5s, attached the macOS arm64 archive, `install.sh`, and `manifest.json`; main CI
  run `34085523868` also passed. The range adds the docked annotation tray, quieter self-target
  annotation transcripts, and Mermaid rendering fixes. It changes no agent-facing behavior, so the
  embedded `operating-agentdeck` package was not refreshed. The distributable binary reports
  `0.4.2` and carries `sqlite_fts5`.
- **Review units:** None open. `dock-the-annotation-tray-and-quiet-its-prompt` is reviewed,
  fixed, and closed; all earlier units through this release are closed. Review records, finding-fix
  commits, release records, and handoff/archive/queue bookkeeping are administrative closure.
- **Work units:** None waiting to start. `migrate-internal-actions-from-mcp.md` stays paused on
  its recorded transport blocker.
- **Design units:** Existing entries under `Ideas being defined` may resume, and entries under
  `New ideas` are available to start. The permanently unaddressable pipeline agent remains the
  newest `New ideas` entry and needs `/design-feature` before code.
- **Open findings:** None.
- **State:** Automated MCP contract verification is green. Pinned Claude/Codex live-provider
  checks remain unrun and must never be described as verified, but they do not block roles.
- **Branch:** `main`.

## Active change

**Change:** None. The `v0.4.2` release closed the annotation-tray and Mermaid units.

**Available by role:** `/review` has no unreviewed unit; `/fix` has no open findings; `/work` has
no ready change; `/design-feature` may choose an available or resumable idea, or an idea a person
names from another `docs/ideas.md` section. Role queues are independent.

Credentialed provider journeys and the real-browser checks below remain open acceptance gates, not
blockers. Never report them as verified without running them.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether chat should
  remain reachable with a wider continuation contract.

## Acceptance gates

**Not blocking as of 2026-09-05.** These gates have not been run, and no agent may describe them as
verified, passed, or closed. The operator chose to let roles proceed with them open.

- [ ] Pinned real-provider stage-result/file-edit approval journey (FS-03.A26/J14).
- [ ] Credentialed Claude and Codex chat, MCP, resume, task, and reported-result checks.
- [ ] Pinned Claude terminal flags/hooks and live xterm journeys.
- [ ] Pinned OpenCode/OpenHands launch and credential checks.
- [ ] Real macOS native folder-panel checks (FS-04.A22/J2/J9/J16).
- [ ] Real-browser permission-pane and drag-refusal journeys (FS-02.A35/A43).
- [ ] Phase 7 federation matrix against real Claude and Codex installations.
- [ ] Real-browser worktree creation/launch and archive-with-uncommitted-work journeys (FS-19).
- [ ] Six-tab same-origin dashboard check against a `make dist` build (FS-02.A27).

## Blocked on human

Nothing. Live-provider acceptance needs human authorization because it invokes real provider
sessions and disposable local configuration homes, but the operator chose not to run it and not to
let it block any role.

## Review findings

None open.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
