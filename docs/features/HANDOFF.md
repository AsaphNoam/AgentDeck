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
- **Review pending — desktop/phone group controls (FS-02.R72–R76/A54–A57,
  FS-20.R45–R46/A17–A18, TS-03.R56–R58, TS-08.R118, TS-13.R24):**
  shared searchable picker in ordinary launch/reassignment, collapsible phone sections,
  desktop cross-group drag, and project-scoped Stop/Archive replace global release.
  All-member lifecycle/archive reservations precede effects; per-member results survive card
  removal. Cross-group moves preserve manual order; pointer collision targets the actual header,
  keyboard drag retains rectangles, and empty Ungrouped appears during drag. Phone Manage
  follows live membership updates. Range begins `4a7899b`, includes `d5b43f5` and `c50e09e`,
  plus the approved stale FileViewer assertion correction in this closure commit.
  Both Go variants, clean UI/build/embed/tagged binary, 758 UI tests and 41 style/presentation
  checks pass. Final authenticated fake-peer/fake-provider browser journey passed 33 checks in
  Core, Sky & Grove and Studio at 1024/1440 desktop and 360/390/430 phone, including live
  regroup, collapsed-header drag beside Ungrouped, scoped Stop/Archive, individual Restore,
  legacy route absence and no exceptions. Evidence: `/tmp/chuck-group-journey-final/`;
  development matrix: `/tmp/chuck-group-matrix/`; group-only snapshot:
  `/tmp/chuck-group-final-verify/`. Original mobile detail/composer edits remain unstaged.
- **Small-change review — 2026-10-10:** mobile spinner `ada153b` (FS-20.R44/A16,
  TS-08.R117), fine right-click menus `083aa23` (FS-12.R66), agent-page width `e2621ac`
  (FS-12.R61), and token-count/stopped-card follow-up `ded893c` (FS-12.R65/R67) closed
  without findings. Room-list width `ce46d56` (FS-12.R63) remains open for ROOM-WIDTH-01
  below. The larger group-controls, room-page and base agent-card reviews remain independent.
  Source audits, 116 focused UI tests, 41 presentation checks and independent rendered review
  completed; evidence: `/tmp/chuck-small-review-20261010/`.
- **Fix closed — mobile history (MOBILE-HISTORY-01):** phone transcript windows now hold 750
  events, load older pages automatically on upward scrolling with viewport anchoring, preserve
  bounded loaded history across live refreshes, and recover intervening gaps. The formerly skipped
  reproduction and focused server/UI coverage pass; rendered phone evidence is
  `/tmp/mobile-history-phone.png`.
- **Settled closures:** Settings composition, desktop agent page and the mobile/Think Tank
  disclosure fixes are closed; details moved to the archive's 2026-10-09 settled-closures section.
- **Review closed — Think Tank room cards (`578ca50`, FS-12.R63/A35, TS-08.R114):**
  independent source/spec and rendered review passed without findings on 2026-10-10. Project
  and Archive cards passed in all three appearances at 1024/1440, including removed origins,
  separate keyboard focus and hover.
  Evidence: `/tmp/chuck-room-review-20261010/`; closure details below. The later room-list
  width follow-up `ce46d56` has ROOM-WIDTH-01 open; the original card review stays closed.
- **Review pending — Think Tank room page (FS-12.R64/A36, TS-08.R115):** the full room page
  adopts the Figma Make `ThinkTank` study (source: Figma MCP resource
  `file://figma/make/source/OykxmXqZnnyA67QA1lv3AU/src/ThinkTank.tsx` + `think-tank.css`):
  breadcrumb, eyebrow/title header with counted Files/Commands, context row with state chip,
  goal card, canvas discussion with round labels and speaker-tinted bubbles, current-action
  band, inset composer with `@`/`#`/source toolbar and help row (ended → read-only note),
  participant/judge cards with tinted edges and live model · effort, Files/Commands as one
  dialog. Page now scrolls; the discussion list has the study's fixed scroll height.
  732 UI tests, presentation/style checks, UI build, both Go variants (`CHUCK_RUNTIME_ROOT=`),
  `make build` and `make embed` passed. Rendered with `ui/scripts/room-render.mjs`
  (new `openings` state) in Core, Sky & Grove and Studio at 1024/1440 for live, ended and
  hidden-openings rooms; receipts in shared project resources `think-tank-design/runs/r1/`.
  Owed for review: independent rendered comparison (A36) including a paused room, a held
  failure with Retry, a pipeline room, Files dialog → file viewer, and keyboard focus.
  Reviewer note: room status chip reuses `roomStatus` tones (Running/Waiting/Needs
  attention/Paused/Ended) rather than the study's openings/judge-failed labels.
- **Review pending — project-dashboard agent cards (`4dc1b34`, FS-12.R65/A37, TS-08.R116):**
  `AgentCard` adopts the Figma `SessionCard` study; supersessions of FS-02 are listed in R65.
  Tests, build and `make test` passed; Core/Sky & Grove/Studio renders at 1024/1280 with the
  fake-backend harness are in shared resources `session-card-design/` (`ref/`, `runs/final/`).
  Owed: independent A37 comparison incl. live error, terminal agent, unread mail, pipeline
  agent, permission in an expanded pane, drag reorder, keyboard focus. Notes: the name is a
  link (body/chevron expand); state labels keep Chuck's Busy/Waiting vocabulary.
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
- **Paused work:** `migrate-internal-actions-from-mcp.md` remains paused on its transport blocker.
- **Branch:** `main`.
- **Known flaky check:** `internal/server` `TestOrdinaryStageAgentStopPausesPipelineRun`
  intermittently returns 409 “a resume is already in progress” (pre-existing at `74c8e84`);
  synchronization remains separate work.

## Active change

None.

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

### Room-list width follow-up — `ce46d56`

**Fix model:** trivial/easy — Claude Sonnet or Codex Luna.

- **Must fix — ROOM-WIDTH-01 (INV §13):**
  `ui/src/styles/features/dashboard.css:5–7` adds auto inline margins to the size-contained
  `.think-tank-list` without giving it an inline size. Open Archive with retained rooms:
  its section collapses to 0px wide inside a 960px archive page at 1024px, and room cards
  overflow as an approximately 80px sliver with word-by-word wrapping and thousands of pixels
  of vertical content. Reproduced in Core, Sky & Grove and Studio with the actual embedded
  UI and real fake-provider server. This violates FS-12.R63's Archive room-card composition
  and FS-05.R39's usable retained-room discovery. Removing only the newly added inline auto
  margins in the browser restores a 960px list. Give the centered size container an explicit
  available width while retaining its 1680px cap; verify both Project and Archive at 1024px
  and wide desktop widths in all appearances. Evidence:
  `/tmp/chuck-small-review-20261010/evidence/extra-report.json`, `*-archive-width.png`
  and `*-archive-before-margin.png`. No product fix made during review.

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

- **2026-10-10 — Review:** Reviewed all five small committed changes in one session.
  Independent bounded Luna source/spec audits found no source defects; parent rendered review
  found ROOM-WIDTH-01 on the room-list width follow-up, which stays open. Spinner, menus,
  agent-page width and token/stopped-card follow-ups closed without findings; the larger base
  card, room-page and group-control units remain pending. Built the working UI (including
  preserved uncommitted phone styling) and tagged fake-provider server in an isolated snapshot;
  bounded approved loopback desktop and fake-peer TLS phone fixtures used isolated homes.
  All three appearances passed 1024/1440/1920/2560 agent measures, project room/grid alignment,
  whole-K counts, stopped treatment, card/project/header/annotation/link menus, settled hover,
  keyboard focus, short viewport, phone busy/idle/stopped and normal/reduced-motion rotation.
  A controlled SSE error checked phone connection-loss gating; offline emulation did not break
  the existing SSE stream and was not counted as a real network-disconnect receipt.
  Main rendered run: 79 checks, no exceptions. Focused UI rerun: 116 passed; initial unrelated
  mobile-history test failure passed alone and on the full rerun. Presentation/style checks
  (41), build/embed and tagged fixture build passed. Invariant sweep: §§2/8/10/13/17 applicable;
  ROOM-WIDTH-01 is §13, with no other violations; §§1/3–7/9/11–12/14–16 have no applicable
  surface. Local choices in the passing units accepted as specified; no new product decision.
  Evidence and temporary harnesses: `/tmp/chuck-small-review-20261010/`.
  Only this state file changed; all pre-existing dirty work preserved.

- **2026-10-10 — Group work closed:** user approved updating the stale FileViewer layout
  assertion to FS-12.R61's page-spanning panel; all 758 UI tests pass. Group implementation,
  complete Go/build checks and rendered receipts are ready for independent review. Existing
  mobile detail/composer work was preserved.

- **2026-10-10 — Feature design:** Completed group selection/creation, mobile sections and
  bidirectional changes, desktop cross-group drag, project-only Stop/Archive and narrow phone
  mutation access. The user waived compatibility; global release will be removed and callers
  migrated to the scoped API. Ready change:
  [`agent-groups-on-desktop-and-mobile.md`](../ready-changes/agent-groups-on-desktop-and-mobile.md).
  FS-02.R72–R76/A54–A57, FS-20.R45–R46/A17–A18, TS-03.R56–R58, TS-08.R118 and TS-13.R24
  remain planned. No product code, schema migration or retention change; Active change remains None.

- **2026-10-10 — Feature design:** Confirmed project-only group bulk actions and desktop
  drag/mobile picker; completed feature scope and drafted the API, lifecycle, presentation and
  narrow phone access contracts. Await technical boundary approval before ready promotion.

- **2026-10-10 — Feature design:** Confirmed group picker/creation and mobile sections;
  expanded draft for bidirectional moves, cross-group drag, Stop group and Archive group.
  Audited global release and existing lifecycle/archive seams; action scope and phone drag
  remain pending. Unrelated mobile presentation changes preserved.

- **2026-10-10 — Feature design:** Recorded group selection/creation and mobile visibility;
  drafted FS-02/FS-20 behavior and acceptance. Read-only audit confirmed global suggestions,
  existing launch group support and phone group data. Await product scope confirmation before
  technical design; unrelated mobile presentation edits preserved.

- **2026-10-10 — Review:** Closed room-card unit `578ca50` without findings. Independent Luna
  source/spec/caller audit and parent browser comparison against shared `room-design/ref/` passed.
  Bounded approved loopback-only Vite/Playwright harness stubbed every API/event source; no live
  provider or operator state was used. Project and Archive in Core, Sky & Grove and Studio at
  1024/1440 covered concurrent openings, speaking, held, paused, ended/judge, deleted participant,
  long title, removed origin, hover and separate outlined room/participant Tab stops, with no
  horizontal overflow or page errors. Development matrix passed at 1024 in all appearances.
  Focused RoomList tests (3), style checks and presentation-contract checks (41) passed.
  Local choices accepted: attention uses `--ad-state-waiting`; instant hover and token keyline
  replace prototype motion/shadow; `RoomTurnNotice` retains its separate pill roster. Existing
  failed-judge attention tone preserves the ended phase and separate judge footer; an identical
  title/goal suppresses a duplicate preview while retaining the goal in the title. Both edge cases
  were rendered. Invariant sweep: §§8/10/13/17 applicable and passed; §§1–7/9/11–12/14–16 have
  no applicable surface. **Fix model:** none — no open findings. Screenshots and temporary harness
  retained in `/tmp/chuck-room-review-20261010/`; no product code or specification changed.

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
