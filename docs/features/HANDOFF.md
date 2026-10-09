# Chuck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then the requirements they
name. Settled state through 2026-10-09 is archived in
[`HANDOFF-through-2026-10-09`](../archive/state/HANDOFF-through-2026-10-09.md).
Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Release:** `v0.12.0` is tagged at `075cdd5` and published with `main`. The user confirmed the
  version, waived the pending clone-name review, and authorized pushing all three unpushed commits
  plus the tag. Both Go variants passed; 721 UI tests, 41 presentation/style
  checks, `go vet`, `make dist VERSION=0.12.0`, FTS5 build-tag proof, shell syntax, and old-name
  checks passed. The embedded operator package already matches the range; no package edits needed.
  Release run `37935310874` succeeded, including archive and fresh-install verification. The
  arm64 archive, `manifest.json` (size/checksum verified against the uploaded archive), `install.sh`,
  and readable release notes are published. General CI `37935311001` also succeeded.
  Release notes are in
  [`RELEASE-v0.12.0-notes.md`](../archive/state/RELEASE-v0.12.0-notes.md).
- **Repository:** GitHub is `AsaphNoam/Chuck` (renamed 2026-10-09); the old name redirects.
  Installer/updater defaults match it, so no `CHUCK_REPO`/`--repo` override is needed.
- **Active change:** None.
- **Waiting for review — mobile companion overhaul:** Figma Make `/mobile` composition across
  Home/project, Chat/Files/Manage, pipeline history, pairing, This phone and setup/confirmation
  sheets (FS-20.R43/A15, TS-08.R111). Core, Sky & Grove and Studio follow the desktop's durable
  preference through the existing semantic palette seam. Source commits `50e0d97` and `ad9d94b`
  plus this closure's generated embed/spec updates form one review unit. All 724 UI tests,
  41 presentation/style checks, TypeScript, specification checks, both Go test variants, `go vet`
  and `make dist` passed. Source-build Go tests require `CHUCK_RUNTIME_ROOT=` here because Chuck
  supplies the packaged runtime to its child processes; the initial ambient-runtime credential
  probe failure passed with that intended source-build environment.
  Actual paired-phone and independent rendered comparison passed all three palettes, 360px long
  content, 390px composition and 430px settings; desktop matrix passed at 1024/1280 in all themes.
  Genuine fake-provider pending permission, opaque viewport setup/Stop sheets, and an unchanged
  draft across phone-listener disconnect/reconnect passed without page errors. Reference sources,
  prototype renders, matrix and actual phone receipts are in shared project resources
  `mobile-design/` (`runs/final` for width/palette captures, `runs/verified-final` for corrected
  interaction receipts; earlier permission/offline/full-page sheet captures are superseded).
  Existing real-tailnet, Android/iPhone and credentialed-provider acceptance gates remain owed.
- **Waiting for review:** Think Tank room page no longer shows the per-message "tools and changes"
  disclosure between published contributions (FS-21.R39/A19, TS-14.R15, TS-08.R104, FS-03.R77);
  live and unfinished-turn activity still renders. Done without a design stage at the user's
  request. UI tests (721), UI build and `make check-specs` passed; Go untouched, `make embed` not run.
- **Paused work:** `migrate-internal-actions-from-mcp.md` remains paused on its transport blocker.
- **Branch:** `main`.
- **Known flaky check:** `internal/server` `TestOrdinaryStageAgentStopPausesPipelineRun`
  intermittently returns 409 “a resume is already in progress” (pre-existing at `74c8e84`);
  synchronization remains separate work.

## Active change

None. The Figma mobile companion overhaul is complete and available for review above.

## Acceptance gates still owed

- **FS-14.A50 / FS-21.A31:** bounded packaged Claude/Codex Think Tank stage probe (start → fresh
  participants → judge → accepted output → next stage). Fake-provider E2E and the 318-check render
  pass are not live receipts. **FS-14.A49** restart/race matrix and **FS-21.A32** live Stop-cancel
  race remain planned.
- **FS-21 / TS-06.R33:** credentialed Claude/Codex Think Tank smoke covering room read/submit,
  addressed input, approval/denial, private Send/Steer, native resume, end-only judge, and retained
  exact result. Fake-provider journey and automated/race closure passed; live receipt is owed.
- **FS-10.R25/A14 / TS-02.R41:** supervised disposable-real-home cutover rehearsal with receipt;
  GitHub rename to `AsaphNoam/Chuck` remains owed.
- **FS-02.A46 / A27:** real-browser toast and macOS notification clicks; six-tab shared-stream,
  J14 real-browser, and Sky & Grove with Codex capabilities.
- **FS-14.A54:** packaged Claude/Codex probe that a standing owner/dedicated coordinator adopts
  shared orchestrator instructions; fake delivery capture is not a live receipt.
- **FS-18.A12/A13:** six manual role scenarios and role-free follow-ups on pinned Claude/Codex;
  credentialed fresh/resumed Claude native-preset adoption (pinned adapter 0.85.1 only proves
  the `_meta.systemPrompt` request, not live adoption).
- **FS-20.A1/A5/A6/A8:** real tailnet, Android, iPhone, and `pmset -g assertions`; iPhone Home
  Screen PNG touch icon is absent. The combined 390px dashboard/project/agent-management/Files/
  Commands-retired journey remains owed; fast-mode picker and Continue on approval pause remain
  outside the fake-provider coverage.
- **TS-06.R21:** credentialed Claude/Codex login/chat.
- **TS-06.R31 / FS-09.A40/A42/A46/A47/R78 / FS-10.A10–A12:** finite four-combination provider
  smoke (each provider with current Bundle and one current Installed CLI), plus Installed update →
  Refresh → model and missing Installed → Bundle → retry → Installed-with-overrides journeys.
  Claude Installed fresh launch passed 2026-10-05; the broader credentialed matrix is owed.
- **TS-06.R26/R32:** credentialed Codex 2.1.1 receipt for FS-03.A41/A42 and FS-01.A20.
- **TS-06.R32:** two-point Claude/Codex smoke on ACP 0.85.1/2.1.1 and Codex 0.159.3, one notice,
  Steer during a running command, and refused model switch where policy allows. Automated assembly,
  fake-runtime, release-CI assembly, and native-provider proofs passed; live receipts are owed.

## Decisions needing input

- Whether to standardize TS-03.R3–R4 mixed legacy error envelopes or TS-04.R3 provider model-ID
  ownership (compatibility change).
- Whether a pipeline stage paused after failed launch/resume continues withholding **Open agent**,
  matching restart recovery, or permits chat under a wider continuation contract.

## Design consistency notes

- Next presentation review: decide whether crisp asymmetric technical-surface radii are deliberate
  under FS-12.R52.
- When direct-action work resumes, align TS-01.R25/TS-03.R32 with TS-04.R40 and scope FS-17
  section 6's opening sentence to the planned boundary.
- Injected-steer lifetime edge case needs `/investigate-bug` before `/fix`; FilesTab still copies
  silently through bare `writeText`.

## Clone first-message disposition

The user closed the unconfirmed 2026-10-08 clone first-message failure on 2026-10-09 because no
further evidence is available. It is not a release blocker and must reopen if it recurs; then
capture the failing request URL/body and bounded `runtime: provider prompt failed` diagnostic from
`~/.chuck/dashboard.log`. Four live Claude variants did not reproduce it. The diagnostic-gap fix
and the `FS-01.R39` source-name-plus-`Copy` behavior are archived with their evidence.

The user waived the pending clone-name review for this release. This is not a completed review
and does not close any manual provider gates.

## Review findings

None open.

## Blocked on human

The published `v0.11.0` release body remains empty. Its notes are preserved in
[`RELEASE-v0.11.0-notes.md`](../archive/state/RELEASE-v0.11.0-notes.md); editing that older
release is separate publication work. Do not retag or recut it.

## Release record

The full pre-release state, including settled findings and historical changelog, is preserved in
[`HANDOFF-through-2026-10-09`](../archive/state/HANDOFF-through-2026-10-09.md). Keep this live file
focused on open gates, decisions, paused work, and the current release until the release record is
updated after publication.
