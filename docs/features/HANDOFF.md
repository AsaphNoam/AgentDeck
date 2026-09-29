# AgentDeck — Implementation handoff

**Live agent state.** Read the **Current position** and **Active change** below, then open the
requirements they name. Settled state is archived in `../archive/state/`: the dated
[`HANDOFF-through-2026-09-27`](../archive/state/HANDOFF-through-2026-09-27.md),
[`-25`](../archive/state/HANDOFF-through-2026-09-25.md),
[`-14`](../archive/state/HANDOFF-through-2026-09-14.md),
[`-13`](../archive/state/HANDOFF-through-2026-09-13.md),
[`-12`](../archive/state/HANDOFF-through-2026-09-12.md),
[`-11`](../archive/state/HANDOFF-through-2026-09-11.md),
[`-10`](../archive/state/HANDOFF-through-2026-09-10.md),
[`-09`](../archive/state/HANDOFF-through-2026-09-09.md),
[`-07`](../archive/state/HANDOFF-through-2026-09-07.md),
[`-06`](../archive/state/HANDOFF-through-2026-09-06.md) and
[`-03`](../archive/state/HANDOFF-through-2026-09-03.md) files, plus
[`HANDOFF-pre-sdd.md`](../archive/state/HANDOFF-pre-sdd.md). Follow
[`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md); this file holds resumable current state only.

## Current position

- **Active change:** none. `add-mobile-remote-control` finished 2026-09-28; its first and
  second-pass reviews both closed 2026-09-29.
- **Release:** `v0.6.0` is tagged and published; **Release state** carries its contents. `v0.5.0` and earlier
  are in the state archive, as are the units, findings and bug reports it closed.
- **Review units:** `add-studio-skin` (finished 2026-09-23) and `complete-studio-composition`
  (finished 2026-09-25) were reviewed and fixed together on 2026-09-29; the combined unit is
  closed. Evidence: `docs/archive/reviews/implementation-studio-acceptance-2026-09-29.md`.
  `share-creative-workspace-layout` (finished 2026-09-28, reviewed and fixed 2026-09-29) is
  closed. Evidence: `docs/archive/reviews/implementation-share-creative-workspace-2026-09-28.md`.
  `stop-telling-agents-to-poll` shipped outside this queue on the operator's explicit 2026-09-10
  instruction; it can be added later. **Available:** `drop-pipeline-recipient-refusal`
  (finished 2026-09-29; `create_task`/`send_message` refusal path in `internal/messaging`).
- **Work units:** `rename-product-to-deckhand.md` is Waiting to start.
  `migrate-internal-actions-from-mcp.md` stays paused on its transport blocker.
- **Design units:** `Ideas being defined` entries may resume (the operator deleted the
  uncommitted Cursor backend draft on 2026-09-23); `New ideas`
  entries are available. The unaddressable-pipeline-agent idea was already shipped by FS-14.R74
  (`8d8ca6e`); on 2026-09-28 its stale FS-01/03/06/14/16 and TS-04 text was reconciled and the
  leftover refusal shipped 2026-09-29 as `drop-pipeline-recipient-refusal` (FS-06.R37/A26, TS-04.R67).
  `docs/ideas.md` was pruned 2026-09-28: shipped agent re-arm/retry/inspection, fixed chat-reload,
  pagination and ACP-readiness items, and nudge-era liveness items were removed; small related
  entries were merged.
- **Open findings:** none recorded. The 2026-09-29 usability review's phone findings (both passes)
  closed the same day; a 390px browser re-check of the second pass's four fixes is owed. The injected-steer lifetime
  edge case is still named in prose but was never recorded as a finding; it needs `/investigate-bug`
  before `/fix` can take it.
  FilesTab and CommandsTab still copy silently via bare `writeText`.
- **Bug reports:** BR-6 closed 2026-09-28 (file links: rendered-file relative links, `name:line`,
  `:start-end`, case-variant roots, refusal logging). BR-7 closed 2026-09-28: live shared workers
  retain the shared reconnect path during server outages.
- **State:** Automated MCP contract verification is green.
- **Branch:** `main`.

## Active change

None. **Owed from `add-mobile-remote-control`:** FS-20.A1/A5/A6/A8 manual gates (real tailnet,
real Android phone and iPhone, `pmset -g assertions`); a PNG touch icon for iPhone Home Screen.
The fakeACP phone-size browser pass (A3/A4/A7/A9) ran at 390px on 2026-09-29; its four findings
were fixed the same day without a browser re-check. The fast-mode picker and Continue on an
approval pause stay unexercised.
Local phone passes: `AGENTDECK_DEV_FAKE_TAILNET=localhost:4529 go run -tags dev
./scripts/stress-fixture`, then `PUT /api/remote {"enabled":true}` on loopback (TS-13 §5). Allow
uses `pending_pairing.id` from `GET /api/remote`, not the code's id. For a permission card, set the
claude backend env `FAKEACP_SCENARIO=permission` via `PUT /api/backends` with `If-Match: <ETag>`.
The in-app browser pane rejects the fixture's certificate; use headless Chromium with
`--ignore-certificate-errors` over CDP.
Observed pre-existing flakes under full `go test ./...` load: `TestContextSharingStartsNoModelTurn`
and `TestStoppedRecipientKeepsContextAcrossResume` (both pass in isolation).

**Owed from archived entries** ([`HANDOFF-through-2026-09-25`](../archive/state/HANDOFF-through-2026-09-25.md)):
A46's real-browser J14 pass; the credentialed Codex 1.12.0 receipt (TS-06.R26) gating
FS-03.A41/A42 and FS-01.A20; Sky & Grove unviewed for Codex capabilities. Pre-existing, not Studio:
Sky & Grove tints the whole user event row; `--ad-shadow-project-edge` resolves at `:root`.

**Release state:** `v0.6.0` is published on tag `24ab07b` (range `v0.5.0..main`, 49 commits). The
CI and Release macOS installer runs both succeeded; the GitHub Release carries the darwin/arm64
archive, `install.sh`, and a manifest declaring `0.6.0` whose SHA-256 and size match the uploaded
archive.
`make test` (both tag variants, including `make check-specs`), the UI suite (58 files, 481 tests),
and `make dist VERSION=0.6.0` pass; the local distributable reports `0.6.0` with `sqlite_fts5`. The
range changed nothing an operating agent must know, so `operating-agentdeck` is unchanged; README,
`install.sh` and `scripts/release/assemble.sh` pins still match. Two Studio review units shipped
unreviewed on the operator's explicit decision. Owed: the credentialed Claude and Codex
login/chat gates (TS-06.R21) and every real-browser journey; none may be described as verified.

**Available by role:** `/fix` has no recorded findings; `/review` may take
`drop-pipeline-recipient-refusal`. `/work` may take `rename-product-to-deckhand`;
`/design-feature` may choose an available or resumable idea. Queues are independent.

## Decisions needing your input

- **API/model compatibility:** TS-03.R3–R4 preserve mixed legacy error envelopes; TS-04.R3 records
  provider model-ID ownership. Standardizing either is a compatibility change.
- **Failed pipeline-stage chat:** Confirm whether a pause after a failed launch or resume should
  keep withholding **Open agent**, matching restart recovery (FS-14.R48), or whether chat should
  remain reachable with a wider continuation contract.

## Blocked on human

None for shared creative-workspace implementation. Its approved system check passed after
correcting reduced-motion selector priority, and Reduce Motion was restored off.

## Review findings

None open. The 2026-09-29 usability review's second phone pass closed 2026-09-29; its 390px
browser re-check of the four fixes was not run.

## Design consistency notes

- FS-12.R52 retires crisp asymmetric radii where inconsistent with the shared composition, but
  code/Mermaid blocks, diff blocks, backend cards, annotation tray, file viewer, composer,
  terminal panel and tracked lists still use them. The 2026-09-29 review did not flag them; decide
  at the next presentation review whether they are deliberate technical-surface geometry.

- The paused direct-action change cites `TS-04.R32–R40`, while TS-01.R25 and TS-03.R32 cite
  `TS-04.R32–R39` and omit R40, the direct-action redaction clause. Align them when that change
  resumes.
- FS-17 §6's opening sentence should be scoped when its planned direct-cutover work resumes; it
  currently reads as covering a section that also contains planned R13–R19 boundaries.

## Changelog

- **2026-09-29 — Fix: phone Home listed a task's stopped agent twice (FS-20.R11; INV §8
  user-facing surfaces).** A `done` agent that owns a task or run under Needs you is no longer
  also listed as "finished" under Since you last looked; FS-20.R11 now says so.
  `TestRemoteHomeClassifiesAttention` covers it and fails on the old code. Closes the 2026-09-29
  usability review's second phone pass; `make test`, the UI suite and `make build` pass.

- **2026-09-29 — Fix: phone Re-arm feedback (FS-20.R30, TS-10; INV §8 user-facing surfaces get
  in-vocabulary data).** A successful Re-arm now shows a status line naming what the task waits
  for, kept on the task screen across the editor's remount. An incomplete prerequisite (no source,
  no outcome, blank signal) is named before anything is sent, and the Mac's typed refusals
  (`dependency_cycle`, new `unusable_source`, `invalid_state`, conflict) read as plain language.
  `PhoneAPIError` keeps `details.code`; the test double now sends the Mac's real envelope, and
  `TestRemoteRearmHasDesktopValueAuthority` pins both typed codes. The 390px re-check was not run.

- **2026-09-29 — Fix: phone transcript showed raw tool results (FS-20.R13; INV §2 parallel paths
  share one helper).** The desktop's tool-run grouping moved from `TranscriptView.tsx` into
  `ui/src/components/chat/toolRun.tsx` (registered in INV's helpers table) and the phone uses it, so
  consecutive tool calls and their results fold into one collapsed "Ran N tools" line on both.
  `AgentScreen.test.tsx` "folds tool results…" fails on the old phone code.

- **2026-09-29 — Fix: phone Start pipeline always refused (FS-20.R15/A4, FS-14.R80, TS-13.R5;
  INV §2 parallel paths share one helper / §17 tests prove their contract).** The tailnet filter
  requires the phone's runtime assignments to be empty, but nothing filled them. It now fills the
  standing owner and every dedicated coordinator from `selectLaunchTarget`'s default (the same one
  a launch with nothing chosen uses) before the shared handler runs; loopback starts are
  unchanged. The finding's suggested client-side fill would have been refused by that filter, so
  its "UI test asserts non-empty assignments" check became `TestRemotePipelineStartUsesMacDefaults`
  (phone body → 201 with default assignments; 422 on the old code). The phone now appends the
  refusal's diagnostics to its error. The 390px browser re-check was not run.

- **2026-09-29 — Usability review, second phone pass (after the hydration fix).** At 390px against
  the dev fixture: decisions, conversation controls, Show earlier, Ask AgentDecker, New task, task
  and stage Retry, Re-arm editing, Replace orchestrator runtime choice, all three annotation targets,
  failed-send draft preservation and Mac-unreachable passed. Phone Start pipeline is a new blocker;
  three minor phone issues recorded. The run file holds the rest.

- **2026-09-29 — Fix: phone app never finished connecting (FS-20.R12/R13/R17, A3; INV §11
  cross-boundary serialization / §17 tests prove their contract).** The phone now recognises the hydration marker on
  the event envelope, as `HydratedMarker` sends it, and its test emits the real envelope (it fails
  against the old code). At 390px against the dev fixture the banner cleared, agents appeared live,
  and a waiting agent's Approve reached the Mac. Closes the 2026-09-29 usability review's findings;
  FS-20.A4/A9 phone passes stay owed.
- **2026-09-29 — Usability review: first paint and phone-size remote pass.** J1 passed on a fresh
  home. The FS-20 phone pass (headless Chromium at 390px against the dev fixture) found one blocker:
  the phone never leaves Reconnecting and shows no agents, blocking A3/A4/A9. FS-20.A9's browser
  pass stays owed. The matrix has no FS-20 phone charter yet. Report:
  `docs/archive/reviews/usability-review-run-2026-09-29.md`.
- **2026-09-29 — Drop the stale pipeline-stage recipient refusal (FS-06.R37/A26, TS-04.R67).**
  Removed `pipelineRecipientRefusal` and its `send_message`/`create_task` call sites; a
  snapshot-less stopped stage agent now gets the ordinary `recipient_not_found` wording, proven by
  `TestSnapshotlessPipelineAgentGetsOrdinaryRefusal` (fails on the old code). Dropped the
  `pipeline association` subcase of `TestIneligibleMailActivationIsDiscarded` and fixed the stale
  wake-gate comments. FS-06 returns to Current. `make test` (both variants) and `make build` pass.

- **2026-09-29 — Fix shared creative-workspace layout findings (FS-12.R52/R56/A29,
  TS-08.R38/R74; INV §8/§10).** Closed the unit. Finished task rows (INV §8) now recede through
  secondary-weight names instead of whole-row opacity; Sky & Grove's muted text darkened
  `#697e7b`→`#5c716e` because it measured 3.84:1 on its own canvas even without the fade, and the
  style-contract check now pins no finished-row alpha plus muted-on-canvas ≥4.5:1 for all three
  palettes. Toast, permission prompt, context menu and user message (INV §10) take the shared
  `--ad-radius-large`/thin-keyline construction with 4px state edges, and the pipeline run title
  uses the 1.875rem route scale. Studio's Settings content measure was a no-op inside the
  1200px settings page, so nothing was promoted. Style/contract checks, the UI suite and
  `make test` pass; the matched 1024px browser screenshot pass was not re-run.

- **2026-09-29 — Review shared creative-workspace layout (FS-12.R52–R59/A26–A31,
  TS-08.R74–R79; INV §1–§17).** One Must-fix finished-row contrast regression and one
  Worth-fixing incomplete geometry promotion keep the unit open. Shared tokens, skin-sheet
  pruning, content-sized card header and composer actions, empty live-settings omission, project
  tabs and live-state-scoped badge pulse with reduced-motion fallback otherwise match the
  requirements; no local-choice notes were pending. INV §2, §13 and §17 had applicable surfaces
  and no finding (`npm run check:styles` and the focused matrix/grid/chat/tasks suites pass);
  §1, §3–§7, §9, §11–§12 and §14–§16 had no applicable surface.

- **2026-09-29 — Fix combined Studio review findings (FS-12.R43/A19–A23, TS-08.R62/R68;
  INV §10/§17).** Closed the grouped unit. Studio's open-canvas dots now render at the specified
  `0.10` opacity, with an independent CSS contract check pinning opacity, dot size and pitch. The
  independent browser pass covered matched Core, Sky & Grove and Studio routes at confirmed
  1024×900 and 1280×720 viewports, real expanded chat, keyboard focus and overflow, a genuine
  completed pipeline timeline, and desaturated common-geometry comparison under the later
  FS-12.R52/A26 and TS-08.R74/R79 supersession. Focused style/contract and matrix tests pass;
  closure matrix recorded with the fix commit.

- **2026-09-29 — Review Studio skin and composition together (FS-12.R42–R49/A18–A23,
  TS-02.R36, TS-03.R45, TS-08.R61–R68; INV §1–§17).** One Must-fix acceptance-evidence gap and
  one Worth-fixing dot-opacity mismatch keep the grouped unit open. Appearance persistence,
  optimistic rollback, finite-id lockstep, local stylesheet wiring, neutral presentation hooks,
  semantic DOM order and Core/Sky preservation otherwise match the requirements. INV §1–§3,
  §8, §10–§11, §13 and §17 had applicable surfaces and no other finding; §4–§7, §9,
  §12 and §14–§16 had none. Fix model: medium — Codex Terra or Claude Opus. The 17 focused UI
  tests, 37 style/contract checks, focused config and server appearance tests, `make check-specs`
  and diff checks pass. The broader server package reached only the already-documented
  `TestContextSharingStartsNoModelTurn` full-load flake.

- **2026-09-29 — Fix add-mobile-remote-control second-pass review (INV §1, §2, §5, §8, §10,
  §14–§16).** Closed all seven Must-fix and two Worth-fixing findings and the unit. Task controls
  now come from one shared FS-16.R22/R23 eligibility helper on desktop and phone (INV §2/§8/§10).
  The phone Re-arm editor (FS-20.R30, TS-13.R17) edits task, run, outcome and signal prerequisites,
  keeps its draft on refusal, and the tailnet route accepts only `arms` (INV §8/§10). Push rereads
  Remote enablement, type mutes and the device switch before every attempt, including retries
  (INV §1/§15). Every tailnet mutation body is capped at 1 MiB (`413 remote_body_too_large`), and
  pairing-failure tracking prunes expired windows and caps at 256 peers (INV §16). Cleanup repair
  claims the run revision under the run lock before any member mutation or cleanup effect, so a
  stale or losing request does nothing (INV §5/§15). The phone annotates diff lines through the
  shared FS-13 drafts, batch builder and allowlisted handler (FS-20.R32, TS-13.R16; INV §2/§8/§10).
  The phone Replace orchestrator form chooses from the secret-free `GET /api/remote/runtime-options`
  catalog and the shared validator rejects stale or unsupported choices (FS-20.R31, TS-13.R15, now
  Current; INV §8/§14). Phone transcript reads are windowed (150 events default, 500 max, ~1 MiB)
  with `before_seq` continuation, and the pending permission and latest reply are derived from the
  whole session (FS-20.R13, new TS-13.R18; INV §16). `make build`, the sqlite_fts5 Go variant, UI
  532 tests and UI build pass; the plain variant passed on rerun after the full-load flake
  `TestStoppedRecipientKeepsContextAcrossResume` (untouched context-sharing code, passes 5/5
  alone); focused repair/pairing/push `-race` passes. A9's phone-size browser pass and the real
  device gates remain owed.

- **2026-09-29 — Broaden the approved mobile control contract.** FS-20.R30–R32/A9 and
  TS-13.R15–R17 now specify phone prerequisite editing, pipeline replacement runtime selection from
  a secret-free catalog, and diff annotation-and-assignment. The three related review findings are
  reframed as missing/partial supported capabilities rather than forbidden-capability bypasses;
  the finding count and difficult fix model are unchanged. No product code changed.

- **2026-09-29 — Re-review add-mobile-remote-control (INV §1, §5, §8, §10, §14–§16).** Seven
  Must-fix and two Worth-fixing findings reopened the unit: unbounded transcript retrieval, a dead
  phone annotation affordance, wrong task-action eligibility, push policy not rechecked at send
  time, cleanup repair effects before revision validation, pipeline replacement runtime override,
  arbitrary mobile re-arm dependencies, unbounded remote mutation bodies, and an unbounded pairing
  failure map. Fix model: difficult — Codex Sol. `make test`, UI 516 tests, UI production build,
  `make build`, and focused remote/server race tests pass; the race run required approved loopback
  access. Real tailnet, Android, iPhone, and keep-awake gates remain owed.

- **2026-09-29 — Fix add-mobile-remote-control review (INV §1 boundary reset; §4 teardown;
  §5 atomic claim; §8 bounded user data; §9 durable files; §10 shipped wiring; §11 serialization;
  §14 security boundary; §15 durable side effects; §16 bounded work; §17 independent tests).**
  Closed all four Must-fix and four Worth-fixing findings. Phone launches reject runtime overrides;
  reconnect hydration replaces stale agents before actions re-enable; Home queries attention,
  moving, and terminal history independently; revoke and request admission share a revalidation
  boundary; VAPID initialization is serialized, durable, and owner-only; run stages cross the API as
  numbers; and the tailnet static surface follows only the phone entry's generated asset graph.
  `make test` passes both Go variants, UI 516 passes, and the UI and production binary builds pass.

- **2026-09-28 — Review add-mobile-remote-control (INV §1, §4, §5, §8–§11, §14–§17).** Four
  Must-fix and four Worth-fixing findings recorded: remote runtime overrides, stale reconnect
  hydration, attention rows lost behind terminal-history pages, a revoke/admission race, VAPID
  initialization and owner-mode gaps, missing numeric run-stage fields, and desktop-only assets on
  the tailnet static surface. Fix model: medium — Codex Terra or Claude Opus. Focused UI tests (26),
  `internal/remote`, `internal/state`, `internal/server`, and `make check-specs` pass; the first
  server run was blocked only by sandbox loopback permissions and passed when rerun with them.

- **2026-09-28 — Fix BR-6 file links (INV §11 protocol meaning; §14 filesystem boundary; §8
  surfaced errors).** Links inside a rendered file resolve against that file's directory
  (`resolveFromFile`, escapes still reach the server's refusal); `README.md:12` is a file link, not
  a scheme; `:start-end` cites its start line; an absolute link spelling the working directory in
  another case reads when that spelling is the same directory on disk (only the variant root is
  stat'ed); each file-read refusal is logged with path and code. FS-03.R51/R52/A34/A35, TS-03.R40,
  TS-05.R21 updated. The three skipped UI reproductions and two new Go tests failed before, pass
  after. `make test` (both variants), `make build`, UI 515 passed, UI build pass. BR-6 closed.

- **2026-09-28 — Finish add-mobile-remote-control (INV §2, §4, §5, §8, §10, §14, §15, §16).**
  Embedded `tsnet` node (Go 1.26.6, `tailscale.com` v1.102.5), tailnet chain with allowlist
  inventory test, node-bound cookie pairing, shared attention helper for Home and Web Push,
  keep-awake, desktop Remote tab, and the installable phone app. FS-20 R1–R29/A2–A4/A7 and TS-13
  (now Current), TS-02.R37, TS-03.R46, TS-05.R23, TS-06.R27, TS-08.R73, FS-00.R18 reconciled.
  `make test` (both variants), `make dist`, UI 510 passed/3 skipped, focused `-race` on remote
  paths, and a 390×844 fakeACP browser pass (found and fixed the desktop 1024px floor on the
  phone) pass. Real-device gates stay owed.
- **2026-09-28 — Fix BR-7 (INV §1 boundary-derived state; §16 bounded streams).** A worker port
  message now proves worker liveness for the current connection, preventing server outages from
  permanently multiplying per-tab streams. TS-03.R7 reconciled; silent-worker and failed-load
  fallbacks remain covered. The unskipped 90s-outage regression failed before the fix (two direct
  streams), then passed; initial-outage recovery and old-port teardown are also covered.
  Focused SSE tests: 26 passed; full UI: 484 passed/3 existing skips. `make dist`, both Go test
  variants and `make check-specs` pass. The first `make test` stopped on a concurrent FS-06 index
  status edit; after that session committed the correction, spec lint and both Go variants passed
  separately. BR-7 is closed; BR-6 remains open. FS-02.A27's six-tab real-browser check is still owed.
