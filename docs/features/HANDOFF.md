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
- **Work units:** `fast-mode-for-agents.md` is waiting to start (designed 2026-09-07, not active).
  `migrate-internal-actions-from-mcp.md` stays paused on its recorded transport blocker.
- **Design units:** Existing entries under `Ideas being defined` may resume, and entries under
  `New ideas` are available to start. Two entries from the 2026-09-07 agent-features request are
  part-decided and resumable: streaming agent thinking (decided live-only; rendering default and
  whether `plan` ships with it still open) and steering a running turn (open on whether steering is
  Claude-native queueing or a portable hold-until-idle). The permanently unaddressable pipeline
  agent remains the newest `New ideas` entry and needs `/design-feature` before code.
- **Open findings:** Two usability findings from the 2026-09-07 v0.4.2 review: J2 incompatible
  CLI status is presented as a credential failure; J5 lower-row card menus clip lifecycle actions.
- **State:** Automated MCP contract verification is green. Pinned Claude/Codex live-provider
  checks remain unrun and must never be described as verified, but they do not block roles.
- **Branch:** `main`.

## Active change

**Change:** None. The `v0.4.2` release closed the annotation-tray and Mermaid units.

**Available by role:** `/review` has no unreviewed unit; `/fix` has no open findings; `/work` may
start `fast-mode-for-agents.md`; `/design-feature` may choose an available or resumable idea, or an
idea a person names from another `docs/ideas.md` section. Role queues are independent.

**Changelog — 2026-09-07:** Designed fast mode to ready. Added FS-09.R50–R56 (per-model `fast`
capability, chat-only claude/codex delivery, Codex autosync from `additional_speed_tiers`,
resolution, advertisement-gated application, exclusion from the switch tuple), FS-03.R45/R46
(header toggle outside the staged picker; unavailable, not-running, and the recorded cooldown
limitation), FS-01.R35, FS-16.R29, FS-14.R59, and acceptance FS-09.A23–A25, FS-03.A28/A29,
FS-01.A19, FS-16.A19, FS-14.A34. Technical side: TS-04.R45/R46 (adapter-declared fast delivery,
fail-open, and the `configOptions` decode staying in `acpmap.go`), TS-01.R28, TS-02.R30,
TS-03.R37, TS-08.R55, TS-09.R34, TS-10.R24. FS-01, FS-03, FS-14, FS-16, TS-02, TS-08, TS-09, and
TS-10 moved Current → Partial with the index updated. Provider surfaces were verified against the
pinned binaries rather than assumed; the evidence is recorded in the ready change so a later
adapter bump can re-check it. The same request's other two features stay under
`Ideas being defined` with their verified findings.

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

- **Must fix** — J2: incompatible CLI status is presented as a credential failure.
  **Where:** `internal/backend/credcheck/claude.go:25-45`, surfaced by
  `ui/src/features/onboarding/steps/BackendStep.tsx:44-46`. **Normal-use trigger:** from a fresh
  onboarding home, an installed `claude-agent-acp` that prints `error: unknown option --cli` and
  exits 2. **Why it matters:** the wizard tells the operator to repair credentials when the
  provider is actually incompatible or un-interrogable, leaving the wrong setup gate and no useful
  compatibility diagnosis. **Requirement:** `FS-04.A14`, `INV §12`. **Suggested fix/test:** classify
  unsupported CLI/status failures separately from credential failures, keep setup retryable, and
  add a J2 fixture test for an unknown option. Reproduced in
  `.review/usability-20260907/run/shots/J2-old-cli-spot-replay.png`.
- **Must fix** — J5: lower-row card context menus hide lifecycle actions below the viewport.
  **Where:** dashboard card context menu at the default 1280×720 viewport. **Normal-use trigger:**
  right-click a lower-row stopped card in a three-column grid. **Why it matters:** the fixed menu
  starts at y=640, placing Resume at y=754 and Archive at y=907 with no clipping correction or menu
  scroll; lifecycle actions become a dead-end until the operator finds a pointer-position workaround.
  **Requirement:** J5, `FS-12.A8`, `INV §8`. **Suggested fix/test:** clamp or flip the menu into the viewport
  and exercise lower-row menus across menu heights and the supported desktop floor. Reproduced in
  `.review/usability-20260907/run/shots/J5-context-menu-clipped.png`.

## Design consistency notes

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.
