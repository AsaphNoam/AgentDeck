# Chuck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then the requirements they
name. Settled state is archived in
[`HANDOFF-through-2026-10-07`](../archive/state/HANDOFF-through-2026-10-07.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Release:** `v0.11.0` is tagged at `ddf8dda` and pushed with `main` (51 commits after the
  prior remote main). The user requested a minor release and authorized the push.
  `make test` (both Go variants), 692 UI tests, 41 presentation/style checks,
  `make dist VERSION=0.11.0`, FTS5 build-tag proof, shell syntax and old-name checks passed.
  Release run `37655214991` first failed creating the GitHub Release with transient GitHub
  HTTP 500s (workflow unchanged from v0.10.0); a later `gh run rerun --failed` succeeded and
  published `v0.11.0` with the arm64 archive, `manifest.json` and `install.sh`.
  CI `37655215773` concluded `success`.
- **Repository:** GitHub is still `AsaphNoam/AgentDeck`. Installer/updater defaults point at
  `AsaphNoam/Chuck` until the postponed rename; use `CHUCK_REPO=AsaphNoam/AgentDeck` and
  `chuck update --repo AsaphNoam/AgentDeck` meanwhile. Release CI publishes to the current repository.
- **Active change:** `agent-chat-links-tables-and-tabs.md` is in progress (see below).
- **Work units:** `migrate-internal-actions-from-mcp.md` remains paused on its transport blocker.
  `pipeline-think-tank-stages-and-task-collapse.md` is waiting to start; its behavior remains planned.
  Other available/resumable design work is in `docs/ideas.md`.
- **Review units / findings:** None.
- **Known verification issue:** `internal/server` `TestOrdinaryStageAgentStopPausesPipelineRun`
  intermittently returns 409 "a resume is already in progress" (pre-existing at `74c8e84`);
  synchronization fix remains separate work.
- **Branch:** `main`.

## Active change

[`agent-chat-links-tables-and-tabs.md`](../ready-changes/agent-chat-links-tables-and-tabs.md) —
in progress. Slices: (1) Commands removal — done: `ChatPanel` tab/`initialTab` mapping,
phone `AgentScreen`, removed `CommandsTab`, `getTrackedCommands`, `TrackedCommand`, its CSS and
the `tracked-list` `commands` contract variant; server tracking/API untouched. (2) Web links —
done: `SanitizedMarkdown` `WebLink`, `lib/linkActions.ts`; every `AnnotationContextMenu` caller
passes `link: claimWebLink(mouse)`; tests in `renderers/webLinks.test.tsx`. (3) Table dividers,
padding and contained overflow — next. (4) Real-browser rendered checks (TS-08.R110; A59–A61,
A14, FS-12.A32 still planned) and closure matrix with `make embed`. Focused checks: `cd ui && npx vitest run <files>`, `npm run check:styles`, `npx tsc -b`.

## Acceptance gates still owed

- FS-21 / TS-06.R33: bounded credentialed Claude/Codex Think Tank smoke (room read/submit,
  addressed input, approval/denial, private Send/Steer, native resume, end-only judge and retained
  exact result). Automated/race closure and the fake-provider rendered journey passed 2026-10-07
  (60 main plus nine annotation-mail/deletion checks across appearances); fake ACP is no live receipt.
- FS-10.R25/A14 / TS-02.R41: supervised cutover rehearsal on a disposable real-home copy under
  `docs/chuck-cutover.md`, with receipt. GitHub rename to `AsaphNoam/Chuck` is still owed.
- FS-02.A46: real-browser toast click and manual macOS desktop-notification click; automated
  component/SSE coverage passes. FS-02.A27: six-tab shared-stream check, A46's real-browser J14
  pass, and Sky & Grove with Codex capabilities.
- FS-18.A12 / TS-11.R17: six manual role scenarios plus role-free follow-ups on pinned Claude
  and Codex. FS-18.A13: credentialed fresh/resumed Claude native-preset adoption; pinned adapter
  0.85.1 source proves object `_meta.systemPrompt` requests preset append, not its live adoption.
- FS-20.A1/A5/A6/A8: real tailnet, Android, iPhone and `pmset -g assertions`. iPhone Home Screen
  PNG touch icon remains absent. The 390px fake-provider pass covers A3/A4/A7/A9 except the
  fast-mode picker and Continue on approval pause. A10–A13 focused server/UI suites pass;
  combined 390px dashboard/project/agent-management/Files/Commands/retired-task journey is owed.
- TS-06.R21: credentialed Claude/Codex login/chat.
- TS-06.R31 / FS-09.A40/A42/A46/A47/R78 / FS-10.A10–A12: finite four-combination provider smoke
  (each provider with current Bundle and one current Installed CLI), plus Installed update →
  Refresh → model and missing Installed → Bundle → retry → Installed-with-overrides rendered
  journeys. Requires authorization/credentials. Claude Installed fresh launch at `claude-opus-5-5`
  passed 2026-10-05 (ACP 0.75.1, SDK 0.3.257, Claude Code 2.1.282, macOS, existing login).
- TS-06.R26/R32: credentialed Codex 2.1.1 receipt for FS-03.A41/A42 and FS-01.A20.
- TS-06.R32: two-point Claude/Codex smoke on ACP 0.85.1/2.1.1 and Codex 0.159.3, one notice,
  Steer during a running command (FS-03.A48's live half), and a refused model switch where
  account policy allows. Automated assembly/fake-runtime proofs passed 2026-10-06; v0.11.0
  release CI assembly and native-provider probes passed 2026-10-07. Live receipts remain owed.

## Blocked on human

No human decision is needed. Publication is blocked by GitHub HTTP 500 on release creation and
workflow rerun (2026-10-07); reads and git push work, and GitHub's public status reports operational.
When the API recovers, rerun release `37655214991` with `gh run rerun 37655214991 --failed`, verify
its archive/manifest/installer assets, then set the notes from
[`RELEASE-v0.11.0-notes.md`](../archive/state/RELEASE-v0.11.0-notes.md) with `gh release edit`.
The tag already points at verified `ddf8dda`; do not retag or recut a new version.

## Review findings

None.

## Decisions needing your input

- Standardizing TS-03.R3–R4's mixed legacy error envelopes or TS-04.R3's provider model-ID
  ownership would be a compatibility change.
- Decide whether a pipeline stage paused after failed launch/resume keeps withholding **Open
  agent**, matching restart recovery, or permits chat under a wider continuation contract.

## Design consistency notes

- Next presentation review: decide whether crisp asymmetric technical-surface radii are deliberate
  under FS-12.R52.
- When the direct-action change resumes, align TS-01.R25/TS-03.R32 with TS-04.R40 and scope
  FS-17 section 6's opening sentence to the planned boundary.
- Injected-steer lifetime edge case needs `/investigate-bug` before `/fix`; FilesTab
  still copies silently through bare `writeText`.

## Changelog

- **2026-10-07 — Chat cleanup design ready.** User approved web-link new-tab/copy actions,
  local-file viewer preservation, readable tables and Commands-tab removal including phones.
  Added planned FS-03.R78–R80/A59–A61, FS-05.R40/A23, FS-12.R60/A32, FS-20.R42/A14 and
  TS-08.R107–R110; promoted the idea to `agent-chat-links-tables-and-tabs.md`, waiting to start.
  Existing react-markdown/GFM renderer is retained; command tracking/API/history are preserved.
  No active implementation or product edits. Documentation and twin-skill checks passed.
- **2026-10-07 — Operating skill audit fixes finished.** Corrected the terminal messaging/task
  boundary, clarified that existing task targets use current session settings, and documented
  explicit fresh-judge retry without reopening discussion. Guidance now matches existing behavior;
  no product or specification change. `go test ./internal/agentknowledge`, `make check-specs`, and
  `git diff --check` passed. All three approved audit items are closed.
- **2026-10-07 — Release preparation: v0.11.0.** Minor release for Think Tank workspace/live
  controls, quiet completed chat turns, UI polish and the 2026-10 provider refresh. Corrected the
  optional source install's Claude ACP pin to 0.85.1 and regenerated the UI embed through `make dist`.
  The shipped operator package already matched the range. Full Go/UI/build checks passed;
  archived the settled pre-release handoff and retained manual gates above. Main/tag pushed;
  macOS assembly/install proofs passed, but GitHub release creation/rerun returns HTTP 500.
  Release notes are preserved for publication recovery; the release is not published yet.
