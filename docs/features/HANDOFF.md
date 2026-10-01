# AgentDeck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then open the requirements
they name. Settled state through 2026-10-01 is archived in
[`HANDOFF-through-2026-10-01`](../archive/state/HANDOFF-through-2026-10-01.md); older epochs remain
beside it. Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Active change:** none.
- **Release:** `v0.8.0` is tagged at `a9f33c5` and published. The 13-commit range after `v0.7.0`
  ships unrestricted on-demand local text-file viewing and file-selection annotations in chat,
  plus reliable Claude model application after resume. The operator skill explains the file-viewer
  boundary; README claims and pinned release components remain current. The macOS release workflow
  passed. CI's first run hit one non-reproducing context-sharing timing assertion; the full rerun
  passed. A post-release test-only fix now waits for the held prompt at the fake-provider wire before
  measuring that context operations add no turns. The GitHub Release carries the 293,072,856-byte
  `darwin-arm64` archive, `install.sh`, and a `0.8.0` manifest whose size and SHA-256 match the
  archive asset.
- **Work units:** `show-exact-context-and-runtime-metadata.md` and
  `rename-product-to-deckhand.md` are waiting to start.
  `migrate-internal-actions-from-mcp.md` stays paused on its transport blocker.
- **Review units:** the test-only `context-sharing-test-synchronization` fix is available.
- **Fix units:** none available.
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
