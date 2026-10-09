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
- **Review closed — Settings composition (FS-12.R62/A34, TS-08.R113):** every Settings tab
  adopts the Figma Make `Settings` study: 180px left section list with soft done-tinted selection,
  mono eyebrow + section title (Theme, Configured backends, Dependent work) via the new
  `SettingsHeader`, softly raised role/project/phone rows with text actions (destructive in error
  color; roles without an override read "Inherit global"), one raised card per backend with ruled
  mono-labelled groups and ruled model rows, ruled Notifications/Tasks lists, and three equal theme
  cards whose `preview-sky` band holds the action bar and signal over three surfaces (skin rules
  unchanged, no contract change). Primary actions use `--ad-action-primary` with a pressed edge.
  Excluded: study sample data, prototype notes, Appearance default tab, concept theme, invented
  backend fields; sub-1024px layout (below the desktop floor). Remote's `window.confirm` revoke is
  pre-existing and untouched. 730 UI tests, presentation/style checks, UI build, `make test`
  (`CHUCK_RUNTIME_ROOT=`), `make build` and `make embed` passed. Real-browser captures of all seven
  tabs in Core, Sky & Grove and Studio at 1024/1280 (fake-backend fixture: roles with/without
  override, active/archived long-name project, four backends) show no overflow or page errors.
  Harness and captures: shared project resources `settings-design/` (`harness/`, `runs/final`);
  Make source in `settings-design/App.tsx`. Independent review of `22dd186` found no confirmed
  defects. Source/spec review passed all 730 UI tests and 41 presentation/style checks. The
  invariant sweep found no violations: applicable classes §3, §8, §10, §13 and §17 passed;
  §1–2, §4–7, §9, §11–12 and §14–16 have no applicable changed surface. No unresolved local
  choices or open fixes remain; no fix-model recommendation is needed.
  Independent approved Chromium execution used the working UI, the built real binary, a fresh
  review-owned home and fake backend on loopback only. All seven tabs in all three appearances
  at 1024/1280 passed (42 static captures), plus hover, keyboard focus, invalid Tasks/disabled
  Save, expanded model environment, linked configuration-source effective view, long paired-phone
  row, rename/disabled Save, pairing code and pending pairing (42 state captures). All captures
  have zero page errors and horizontal overflow; screenshots were inspected against the study's
  hierarchy and spacing. Display-safe browser fixtures supplied linked-source and phone states;
  this closes presentation coverage, not live-provider or real-tailnet/device gates.
  Evidence and reusable harness: `/private/tmp/settings-independent/` (`static/report.json`,
  `states/report.json`, screenshots and `harness/`). Review-owned processes shut down; unrelated
  concurrent dirty-tree work was preserved. This closes the unit without a new review obligation.
- **Design awaiting technical decisions:** Quota indicator and automatic continuation in
  [`ideas.md`](../ideas.md). Product scope confirmed: global on by default, every chat agent,
  including task/pipeline/Think Tank work, and the exact Chuck continuation notice. Planned
  FS-01.R40–R45/FS-04.R53 and TS-04.R86–R88/TS-10.R39–R44 cover detection and same-owner
  recovery. Await confirmation of additive config/session API + existing Cancel semantics and
  one-record-per-agent retention, plus structured Codex reset forwarding through the existing
  pinned adapter patch. Codex privately receives reset data but does not forward it over ACP;
  provider text parsing is unsupported. No ready change/product code yet. Specification lint,
  launcher twin comparison and diff whitespace checks passed for the partial design; rerun after
  the pending decisions and boundary specifications before promotion.
- **Review fixes closed:** mobile companion unit `50e0d97^..d3e737f` and Think Tank disclosure
  unit `7b76bc4` are closed. Resume refusals stay visible on Files (INV §8); dismissed sheets and
  confirmations restore opener focus (INV §10); bounded activity rows carry durable publication
  state so clipped contributions cannot reappear as unfinished (INV §8/§10/§11/§16/§17).
  Both Go variants, 727 UI tests, 41 presentation checks and UI/Go builds passed in
  `/private/tmp/chuck-fix-closure`, excluding pre-existing chat presentation edits. Mobile browser
  Cancel/Close/Escape focus and Files refusal checks passed across all three palettes; receipt:
  `/private/tmp/mobile-fix-browser-final.log`. The Think Tank regression fails against the old
  renderer and passes with the fix. Source-build Go checks need `CHUCK_RUNTIME_ROOT=`; the first
  native-login probe timed out, then focused and full retries passed. Existing real-device and
  credentialed-provider gates remain owed. These fixes create no new review unit.
- **Review closed — desktop agent page:** active and archived agent pages adopt the Figma Make
  `AgentConversation` composition (FS-12.R61/A33, TS-08.R112): breadcrumb with agent id, raised
  card, monogram/state/role header with labelled context meter, low runtime band with local
  Discard and a switch-style fast toggle, quiet tabs, ~700px reading column with tinted bubbles,
  ruled activity, structured permission card (Deny before Approve, shared with the dashboard pane),
  and an inset composer with `@`/`#` insert buttons and a labelled Send (supersedes FS-02.R67 on
  this page only). Workspace styles are scoped to `.chat-panel`; the dashboard pane and phone keep
  their compositions. Sky & Grove's user-tint selector now targets the bubble, not its row.
  Study data, per-message times, date dividers, Files count and decorative hints were excluded.
  730 UI tests, presentation checks, UI build, both Go variants (`CHUCK_RUNTIME_ROOT=`) and
  `make embed` passed; the server package needed one rerun. Real-browser comparison against the
  rendered study passed in Core, Sky & Grove and Studio at 1024/1280 with fake-backend agents
  (permission, activity, long name, staged runtime + Discard, `@` picker, live turn, archive,
  header Copy thread identity). Reference source, renders, harness and receipts: shared project
  resources `agent-design/` (`runs/final`).
  Static review of `ea8dfc8` found no confirmed defects; presentation/style checks and all 730 UI
  tests passed again. Independent rendered review now also passed through bounded approved
  execution in an approval-capable agent: fresh isolated fake-backend fixture, Core, Sky & Grove
  and Studio at 1024/1280, 24 static captures without horizontal overflow or page errors, plus
  activity, permission, runtime staging/Discard, `@`/`#` pickers, live turn, archive and Copy thread
  identity checks. Fresh screenshots were inspected by the reviewer and sampled by the parent.
  Evidence: shared project resources `agent-design/runs/independent-20261009/` (`report.json`,
  three `*-states.json` receipts and screenshots). The native review child's sandbox cannot
  request escalation; workflow §7/§14.4 now routes that browser work to an approval-capable
  independent agent. No product code or permission defaults changed. This closes the review unit
  and its access block; existing credentialed-provider and real-device gates remain owed.
  The review used `/private/tmp/chuck-agent-page-review` built from `c189d95`, the working UI via
  Vite, and fresh review-owned `CHUCK_HOME` fixtures. Both review-owned harness sessions shut down;
  existing processes were preserved. Fresh captures were compared with `agent-design/ref/` study
  renders.
- **Paused work:** `migrate-internal-actions-from-mcp.md` remains paused on its transport blocker.
- **Branch:** `main`.
- **Known flaky check:** `internal/server` `TestOrdinaryStageAgentStopPausesPipelineRun`
  intermittently returns 409 “a resume is already in progress” (pre-existing at `74c8e84`);
  synchronization remains separate work.

## Active change

None. All queued review findings are closed.

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

None. Existing live-provider and real-device acceptance gates above remain owed.

## Browser-verification permission investigation — 2026-10-09

- **Report (verbatim):** “Blocked why, /investigate-bug Chuck needs to be able to do this”.
  Trigger: independent desktop-agent-page review of `ea8dfc8` reported loopback `EPERM` and
  Chromium launch failure. Environment: macOS arm64, Chuck Codex ACP, native Codex review
  subagent, CLI `0.161.0`. Logs exist in the isolated Codex profile.
- **Confirmed — works as specified:** the native review child recorded `approval_policy: never`
  and `workspace-write` with `network_access: false`. Its parent recorded `on-request`. Source
  identity and policies are in
  `~/.chuck/codex/sessions/2026/10/09/rollout-2026-10-09T21-19-32-01a121e4-4d7f-7352-9e8c-d631d60a7990.jsonl`
  (`session_meta.source.subagent: review`, parent `01a121e3-c15e-7443-811f-8db575c301d0`).
  A Node listener on `127.0.0.1:0` fails with `EPERM` inside this sandbox and binds/closes
  successfully through approved `require_escalated` execution. The same Playwright/Chromium
  executable aborts with `SIGABRT` under the sandbox and renders `Browser probe` successfully
  through approved escalation. These bounded probes use no live provider or external page.
- **Boundary:** FS-03.R14/R18 and TS-05.R9 govern Chuck's ACP approval relay;
  `internal/server/launch.go:646` and `internal/runtime/permission.go:44` implement that policy.
  They do not grant the native review child OS/network access. Workflow §14.4 still requires
  rendered verification; USABILITY-REVIEW §2 explicitly retains blocked visual steps when a
  browser cannot run. No product/spec defect or missing diagnostic was established, so no
  investigation fix unit or fix-model recommendation is required.
- **Resolved:** an independent approval-capable agent completed the rendered review using bounded
  approved execution, an isolated review-owned home and fake backend. New evidence in shared
  project resources `agent-design/runs/independent-20261009/` confirms Chromium execution and the
  desktop state matrix passed without findings. Workflow §7/§14.4 now captures this routing so
  future visual reviews do not strand browser work in a native review child that cannot escalate.
  No product code or permission defaults changed.

## Blocked on human

The published `v0.11.0` release body remains empty. Its notes are preserved in
[`RELEASE-v0.11.0-notes.md`](../archive/state/RELEASE-v0.11.0-notes.md); editing that older
release is separate publication work. Do not retag or recut it.

## Release record

The full pre-release state, including settled findings and historical changelog, is preserved in
[`HANDOFF-through-2026-10-09`](../archive/state/HANDOFF-through-2026-10-09.md). Keep this live file
focused on open gates, decisions, paused work, and the current release until the release record is
updated after publication.

## Recent changelog

- **2026-10-09 — Fix / review closure (INV §10):** Browser verification routes through an
  approval-capable independent agent with bounded approved execution. The fresh desktop-agent-page
  rendered review passed in all three appearances at 1024/1280 without findings, closing `ea8dfc8`
  and its access block. No product or global permission change.

- **2026-10-09 — Investigation:** Native Codex review child cannot request escalation; loopback
  and Chromium failures reproduce in its sandbox and pass via approved parent execution.
  No Chuck launch defect established; independent desktop rendered verification remains owed.

- **2026-10-09 — Review:** Desktop agent-page `ea8dfc8` source/spec/caller review and 730 UI tests
  passed without findings. Independent rendered verification is blocked by sandbox permissions;
  the review unit remains open.

- **2026-10-09 — Feature design:** Confirmed default-on global quota continuation for every chat
  and exact attributed continuation notice; drafted feature/technical core and owner exceptions.
  Verified pinned typed-failure and reset surfaces. API/retention and Codex forwarding decisions
  remain open; the idea stays in design and implementation has not started.

- **2026-10-09 — Implementation:** Desktop agent page matches the Figma Make agent-conversation
  study across all three appearances; waiting for review.

- **2026-10-09 — Feature design:** Captured quota-limit indication and optional scheduled
  continuation; checked existing notice, prompt-error and same-task wait behavior. Proposed
  scope is awaiting the human's toggle, coverage and recovery decisions. Other dirty-tree
  presentation work is preserved.
