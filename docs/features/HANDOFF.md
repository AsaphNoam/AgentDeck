# AgentDeck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then open the requirements
they name. Settled state through 2026-09-30 is archived in
[`HANDOFF-through-2026-09-30`](../archive/state/HANDOFF-through-2026-09-30.md); older epochs remain
beside it. Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Active change:** none.
- **Release:** `v0.7.0` is confirmed and being prepared from `v0.6.0..main` (73 commits). The
  release audit found no open review unit or finding and no stale release-matched operator guidance,
  README install claim, or pinned component version. `make test`, the UI suite (65 files, 555
  tests), and `make dist VERSION=0.7.0` pass; the arm64 distributable reports `0.7.0` and carries
  the `sqlite_fts5` build tag. The local tag, publication, and publication confirmation remain
  pending.
- **Work units:** `rename-product-to-deckhand.md` is Waiting to start.
  `migrate-internal-actions-from-mcp.md` stays paused on its transport blocker.
- **Review and fix units:** none available.
- **Design units:** available and resumable entries remain in `docs/ideas.md`.
- **Branch:** `main`.

## Active change

None.

## Acceptance gates still owed

- FS-20.A1/A5/A6/A8: real tailnet, Android, iPhone, and `pmset -g assertions` checks. The iPhone
  Home Screen experience also still lacks a PNG touch icon. The 390px fake-provider browser pass
  covered A3/A4/A7/A9, except the fast-mode picker and Continue on an approval pause.
- TS-06.R21: credentialed Claude and Codex login/chat checks.
- TS-06.R26: the credentialed Codex 1.12.0 receipt gating FS-03.A41/A42 and FS-01.A20.
- FS-02.A27: six-tab real-browser shared-stream check; A46's real-browser J14 pass; Sky & Grove
  with Codex capabilities.

## Blocked on human

None.

## Review findings

None.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** decide whether a pause after failed launch/resume keeps
  withholding **Open agent**, matching restart recovery, or permits chat under a wider continuation
  contract.

## Design consistency notes

- At the next presentation review, decide whether the remaining crisp asymmetric radii on technical
  surfaces are deliberate under FS-12.R52.
- When the paused direct-action change resumes, align TS-01.R25 and TS-03.R32 with TS-04.R40 and
  scope FS-17 section 6's opening sentence to the intended planned boundary.
- The injected-steer lifetime edge case needs `/investigate-bug` before `/fix`; FilesTab and
  CommandsTab still copy silently through bare `writeText`.

## Changelog

- **2026-09-30 — Release preparation: v0.7.0.** Confirmed the minor version and audited the
  `v0.6.0..main` range. The shipped `operating-agentdeck` package already matches the agent-facing
  behavior; README install claims and pinned release components remain current. Archived the prior
  live handoff epoch. Both Go test variants, all 555 UI tests, the presentation contract and the
  versioned arm64 `sqlite_fts5` distributable pass. Credentialed and real-device gates remain owed.
