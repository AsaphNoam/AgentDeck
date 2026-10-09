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
- **Reviewed — mobile companion overhaul, fixes open:** `50e0d97`, `ad9d94b` and `d3e737f`
  remain one review unit (FS-20.R43/A15, TS-08.R111). Review found a hidden Resume refusal on
  Files and missing focus return after sheet dismissal; see **Review findings**. Review reran
  all 53 remote tests and 41 presentation/style checks, and reproduced both findings in a
  loopback-only browser harness across Core, Sky & Grove and Studio at 360px. Conversation and
  Stop-sheet geometry stayed inside the viewport with a long project id; no page errors.
  Implementation closure already passed the full UI/Go/build matrix. Source-build Go tests need
  `CHUCK_RUNTIME_ROOT=` here. Reference, desktop matrix and actual paired-phone receipts remain
  at `/Users/mcnoam/.chuck/project-resources/agentdeck-20261007t221535z/mobile-design/`
  (`runs/final` and corrected `runs/verified-final`). Existing real-tailnet, Android/iPhone and
  credentialed-provider gates remain owed. No product code or specifications changed in review.
- **Reviewed — Think Tank room disclosure, fixes open:** `7b76bc4` removes the per-message
  "tools and changes" disclosure between published contributions (FS-21.R39/A19, TS-14.R15,
  TS-08.R104, FS-03.R77), while keeping live and unfinished-turn activity. The review found that
  a clipped entry window can misclassify activity from an older published attempt as unfinished;
  see **Review findings**. The focused UI test command is presently blocked by unrelated dirty
  presentation-contract failures; its 721-test receipt, UI build and `make check-specs` passed
  before review. Go untouched, `make embed` not run.
- **Paused work:** `migrate-internal-actions-from-mcp.md` remains paused on its transport blocker.
- **Branch:** `main`.
- **Known flaky check:** `internal/server` `TestOrdinaryStageAgentStopPausesPipelineRun`
  intermittently returns 409 “a resume is already in progress” (pre-existing at `74c8e84`);
  synchronization remains separate work.

## Active change

None. The mobile companion overhaul has been reviewed; its fixes are available above.

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

### Mobile companion overhaul — reviewed 2026-10-09

Unit: `50e0d97^..d3e737f`; findings keep this unit open. The later GitHub-rename handoff commit
is administrative and creates no review obligation. The Think Tank review unit is unchanged.

**Fix model:** trivial/easy — Claude Sonnet or Codex Luna.

- **Must fix — Resume refusal is invisible on Files.** `ui/src/remote/AgentScreen.tsx:334`
  makes Resume available above all tabs, but the stopped-agent error at line 427 is inside the
  Chat branch; Files renders only its own query errors. Open a stopped agent's Files tab and
  have the Mac refuse Resume (for example, a launch validation failure): the request returns its
  reason and `act` stores it, yet no feedback appears until switching to Chat. This violates
  FS-20.R27 and INV §8. Render lifecycle-action feedback outside tab-specific content or in
  every tab. Test a rejected Resume on Files and verify the reason remains visible. Browser
  reproduction returned 409 “Resume refused for review”: zero matching text on Files, one on
  Chat, in all three palettes. Fix difficulty: trivial/easy.
- **Worth fixing — Dismissed sheets lose the operator's focus position.**
  `ui/src/remote/PhoneSheet.tsx:5` and the new confirmation callers in
  `AgentScreen.tsx:436`, `RunScreen.tsx:69` and `PhoneSettings.tsx:178` open controlled Radix
  dialogs without a `Dialog.Trigger` or explicit close-focus restoration. Open New agent and
  close it, or open Stop and Cancel: after the dialog unmounts, `document.activeElement` is
  `body` rather than the opening button. The next Tab starts at the header navigation, making
  keyboard/assistive navigation lose its position. INV §10 (missing dialog trigger/focus wiring);
  TS-08.R111 is the relevant sheet contract. Restore focus to the opener on dismissal through
  the existing dialog seam; test cancellation/close/Escape in a browser after unmount. Confirmed
  for New agent close and Stop Cancel across all three palettes. Fix difficulty: trivial/easy.

Specification coverage and local choices: accepted the eight-character pairing/iPhone install
handoff, real stage-attempt history, retained Manage controls, and read-only shared appearance
application instead of the prototype's mock controls and data, as FS-20.R43/TS-08.R111 require.
The saved Home/conversation reference and corrected permission receipt match the required
composition with Chuck's own palette. No unresolved local-choice or specification gap found.

Invariant sweep: applicable classes §1 (appearance/lifecycle), §2 (shared construction),
§8 (feedback), §10 (wiring), §11 (run collection serialization), §13 (selectors), and §17
(test oracles) reviewed. Findings are tagged above; no other violation found. Classes §3–§7,
§9, §12, and §14–§16 have no applicable changed surface: no seeded-config writes, registration,
concurrency, adapter contract, record recovery, liveness/storage, CLI invocation, HTTP/security
handler, external-effect ordering, or new stream/retention mechanism. The timeline consumes the
existing run projection; server collections are initialized before serialization.

Review evidence: 53 focused remote tests and 41 presentation checks passed. Browser harness and
captures are in `/private/tmp/mobile-review.mjs` and `/private/tmp/mobile-review/`; those use
stubbed APIs and do not replace the outstanding real-device gates.

### Think Tank room disclosure — reviewed 2026-10-09

Unit: `7b76bc4`; finding keeps this unit open.

**Fix model:** medium — Codex Terra or Claude Opus.

- **Must fix — A clipped published contribution can reappear as an “unfinished turn.”**
  `ui/src/api/thinkTanks.ts:164` retains only the newest `THINK_TANK_ENTRY_WINDOW` entries, while
  `ui/src/features/thinktank/ThinkTankPage.tsx:182` decides whether activity is unfinished solely
  from that clipped entry list. In a long-lived room, send enough later room inputs to clip an
  earlier contribution while its activity remains inside the independently bounded activity window:
  its attempt id is no longer `placed`, so the page renders its old tool/diff disclosure as
  `Ari's unfinished turn`. The operator sees stale completed work as unresolved, violating
  FS-21.R39 and TS-14.R15. Preserve publication knowledge for every attempt represented in the
  activity window (prefer a bounded server/wire indication over an unbounded client set), and add
  a regression with a clipped published attempt plus retained activity.

Specification coverage and local choices: the change correctly removes the disclosure immediately
after a visible published contribution and keeps loose live/unfinished activity attributed and
inspectable. No additional specification gap found.

Invariant sweep: applicable classes §8 (the stale disclosure misstates turn state), §10 (the
bounded entries/activity paths no longer compose correctly), §11 (the UI lacks the publication
state needed for its activity contract), §16 (the fix must retain only a bounded publication
projection), and §17 (the test omits the clipped-window case) reviewed. Classes §1–§7, §9, and
§12–§15 have no applicable changed surface: no lifecycle reset, shared artifact construction,
seeded persistence, registration/teardown, concurrency claim, interface/runtime, record-recovery,
liveness/storage, external CLI, selector, HTTP/security, or external-effect ordering change.

Review evidence: `git diff --check 7b76bc4^ 7b76bc4` passed. The focused `ThinkTankPage` test run
could not start because its pretest presentation-contract gate fails on unrelated dirty files
(`ChatPanel`, `Composer`, and `PermissionPrompt`); preserve and resolve that separate work before
rerunning the focused suite.

## Blocked on human

The published `v0.11.0` release body remains empty. Its notes are preserved in
[`RELEASE-v0.11.0-notes.md`](../archive/state/RELEASE-v0.11.0-notes.md); editing that older
release is separate publication work. Do not retag or recut it.

## Release record

The full pre-release state, including settled findings and historical changelog, is preserved in
[`HANDOFF-through-2026-10-09`](../archive/state/HANDOFF-through-2026-10-09.md). Keep this live file
focused on open gates, decisions, paused work, and the current release until the release record is
updated after publication.
