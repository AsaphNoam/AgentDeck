# Chuck — Implementation handoff

**Live agent state.** Read **Current position** and **Active change**, then open the requirements
they name. Settled state through 2026-10-04 is archived in
[`HANDOFF-through-2026-10-04`](../archive/state/HANDOFF-through-2026-10-04.md); older epochs remain
beside it. Follow [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

## Current position

- **Active change:** Think Tanks review fixes (TT-01–TT-13).
- **Release:** `v0.9.0` is tagged at `ae93666` and published; the macOS release workflow passed. The
  GitHub Release carries the 293,150,597-byte `darwin-arm64` archive, `install.sh`, and a `0.9.0`
  manifest matching that size. Linux CI then failed `TestPublishedRootSelectsTheBundledProviders`:
  the bundle path exists only on darwin by design, so the test now skips elsewhere. The 46-commit range after `v0.8.0` ships the installed-provider
  default with explicit per-backend bundle, lean personas with shared operating context, Tasks as
  project-grouped work in motion, notifications that open the conversation, exact context tokens,
  and the phone desktop flow. The operator package and README already matched the range.
  Credentialed Claude/Codex gates remain owed.
- **Work units:** `migrate-internal-actions-from-mcp.md` stays paused on its transport blocker.
- **Review units:** `think-tanks` (2026-10-06, `46379da`..HEAD: SQLite room authority, guarded
  `think_tank` activation with executing turn ids, MCP room tools, REST/SSE, activity capture and
  projector, room-source annotations, room UI/Archive/project entry, operating-chuck reference;
  FS-21, TS-14 and adjacent FS-02.R65, FS-03.R69–R70, FS-05.R39, FS-13.R26–R27, FS-17.R21,
  FS-18.R19, TS-01.R37, TS-02.R42, TS-03.R55, TS-04.R84, TS-05.R25, TS-08.R87, TS-11.R19) is
  reviewed through `60ba960` on 2026-10-06 and remains open for the findings below. The same unit
  is available for fix; its next review includes those fixes. The single worker with 5s sweeps/kicks
  and shared composer autocomplete are sound local choices. TS-14 §5's child-capture deviation
  does not cover late child events stamped with a later turn's identity (finding TT-03).
  `use-installed-provider-clis` (2026-10-04, `4d1e9cc^`..`6c5c52f`: shared provider
  resolver, Installed default/explicit AgentDeck bundle, release wrapper, typed recovery,
  provider_runtimes + Refresh provider, Settings/New Agent UI, docs; FS-09.R68/R70–R77,
  FS-10.R21/R23–R24, TS-03.R52–R54, TS-04.R71–R77, TS-06.R30) was reviewed 2026-10-04 and its
  findings were fixed the same day; the unit is closed apart from its owed credentialed and
  rendered gates. The test-only `post-release-flaky-test-synchronization` fixes are available.
  `rename-product-to-chuck` (2026-10-04, `36afbf2`..`3855419`: module
  `github.com/AsaphNoam/Chuck`, `cmd/chuck`, `CHUCK_*`/`~/.chuck`, Chuck install tree/archive,
  `chuck-messaging`/`X-Chuck-Token`, `chuck-` tmux prefix, resident role (now Chucky),
  `operating-chuck`, UI/phone branding, legacy annotation recognition, storage copy-forward, `scripts/check-old-name.sh`,
  docs and `docs/chuck-cutover.md`; FS-00.R19, FS-04.R52, FS-10.R15/R25–R26, FS-13.R24,
  FS-18.R18, TS-02.R40–R41, TS-04.R78, TS-06.R24, TS-08.R58, TS-11.R18) was reviewed 2026-10-05;
  its findings were fixed 2026-10-05 and the resident role renamed FirstMate → Chucky (`chucky`);
  the unit is closed apart from its owed rehearsal, repository rename and release notice.
  Historical old-name mentions were kept deliberately (retired items, `AGENTDECK_CODEX_VERSION`
  history, the Figma URL, the retired BRIEFS, the uncommitted `docs/ideas.md` edit).
- **Fix units:** `phone-chat-streamed-deltas` closed 2026-10-05 (phone folds the transcript with
  `foldTranscript`; `ui/scripts/phone-render.mjs` renders the phone conversation at iPhone size).
  `notifications-open-conversation` closed 2026-10-05 apart from its owed browser/macOS click
  checks. `phone-desktop-flow-and-agent-management.md` closed 2026-10-05 apart from its owed
  fakeACP browser passes at phone size (FS-20.A10–A11). Claude 5.5 launch compatibility closed
  2026-10-05 (live Installed probe plus alias-row fix); the broader TS-06.R31 smoke stays owed.
  The former bundled-default, opt-in recovery draft FS-09.R64–R67/A33–A36 is retired;
  no product change has shipped from it.
- **Design units:** available and resumable entries remain in `docs/ideas.md`.
- **Branch:** `main`.

## Active change

Think Tanks fix plan: runtime terminal/child ownership (delegated); durable command replay and
cursor validation (delegated); retained activity/file inspection and bounds (delegated); lifecycle
exit settlement, setup launch claims and final eligibility (parent integration). Each slice adds
focused regression evidence; integrate and run the shared closure matrix before closing this unit.
Runtime ownership is verified: full runtime suite and focused race tests pass; both ownership
regressions fail against pre-fix overlays. Integrated Think Tank state/server tests pass, including
Stop/archive/deletion and final admission/setup fences. All TT-01–TT-13 fixes are implemented;
focused server/state tests, room UI tests and TypeScript pass. Both Go variants, full UI/style
checks, focused race checks and embedded build are the remaining closure pass, currently running.

Tasks wire fixture regeneration: `CHUCK_UPDATE_TASK_FIXTURE=1 go test ./internal/server
-run TestTaskWireFixture`. Think Tank room fixture: `CHUCK_UPDATE_THINK_TANK_FIXTURE=1 go test
./internal/server -run TestThinkTankWireFixture`. Room screenshots: `(cd ui && node
scripts/room-render.mjs <outDir> live|ended)`.

## Acceptance gates still owed

- FS-21 / TS-06.R33 (Think Tanks): the real-binary fake-ACP rendered journey (creation →
  discussion → annotation/private follow-up → End/judge → retained Archive at 1024px and wider in
  Core, Sky & Grove and Studio) and the bounded credentialed Claude/Codex smoke (room-tool
  read/submit, an ordinary approval/denial, private Send/Steer, native resume, end-only judge).
  Stubbed-data renders of live and ended rooms in all three appearances passed 2026-10-06.

- FS-10.R25/A14, TS-02.R41: one supervised cutover rehearsal on a disposable copy of the real
  AgentDeck home, following `docs/chuck-cutover.md`, with a receipt. Before the first Chuck release
  the GitHub repository must be renamed to `AsaphNoam/Chuck` (installer and updater fetch there),
  and the release notes must say paired phones re-pair at the new `chuck` address.

- FS-02.A46: real-browser toast click check and a manual macOS desktop-notification click are
  owed; automated component and `sse.test.ts` coverage passes.

- FS-18.A12 / TS-11.R17: the six manual role scenarios with a role-free follow-up against the
  pinned Claude and Codex providers have not been run; no qualitative receipt exists yet.
- FS-18.A13: the credentialed fresh and resumed Claude chat check of native-preset adoption is
  owed. Automated coverage proves only the sent shape against pinned
  `claude-agent-acp` 0.75.1 (`scripts/release/node_modules/.../dist/acp-agent.js` forwards an
  object `_meta.systemPrompt` as a preset append; a string replaces the preset).

- FS-20.A1/A5/A6/A8: real tailnet, Android, iPhone, and `pmset -g assertions` checks. The iPhone
  Home Screen experience also still lacks a PNG touch icon. The 390px fake-provider browser pass
  covered A3/A4/A7/A9, except the fast-mode picker and Continue on an approval pause.
- FS-20.A10–A13: focused server and UI suites pass; the remaining phone UI-coverage gaps are listed
  under Review findings. The combined 390px fake-provider browser journey
  for the new dashboard, project, agent-management, Files/Commands, and retired task flow also remains
  owed.
- TS-06.R21: credentialed Claude and Codex login/chat checks.
- TS-06.R31 / FS-09.A40/A42/A46/A47/R78, FS-10.A10–A12 (installed providers): at most four real
  combinations — Claude and Codex, each with the current bundle and one current installed CLI —
  running the finite smoke (fresh chat, native resume, model/effort, one approval/denial and
  cancel, Steer, a role/skill and an MCP action), plus two rendered fake-provider journeys
  (Installed update → Refresh → choose new model; missing Installed → Bundle save → retry →
  back to Installed with overrides). Needs authorization and credentials; `assemble.sh`'s native
  probes run first in release CI. Claude Installed fresh launch at `claude-opus-5-5` passed live
  2026-10-05 (adapter 0.75.1/SDK 0.3.257, Claude Code 2.1.282, macOS, the user's existing login); the
  Bundle still refuses it with Claude Code 2.1.257's version error, as designed.
- TS-06.R26: the credentialed Codex 1.12.0 receipt gating FS-03.A41/A42 and FS-01.A20.
- FS-02.A27: six-tab real-browser shared-stream check; A46's real-browser J14 pass; Sky & Grove
  with Codex capabilities.

## Blocked on human

None.

## Review findings

### Think Tanks — reviewed 2026-10-06, `46379da`..`60ba960` — **Fix model:** medium — Codex Terra or Claude Opus.

- **Must fix** — TT-01: Stop strands the active room attempt (INV §4/§15).
  `internal/server/server.go:390` tears down an exiting agent and interrupts tasks but never
  settles room work. Requested Stop suppresses `turn_end` (`internal/runtime/chat.go:1019`,
  `:1661`), while room settlement depends on it (`internal/server/think_tanks.go:272`). Stopping
  the current speaker leaves a running attempt/capture forever: Retry cannot replace it and End
  waits for a boundary that cannot occur. FS-21.R19/R32/R37, TS-14.R6/R14. Extend generation-scoped
  exit settlement to fail/release room attempts without charging; test Stop, archive and deletion
  during participant/closing/judge work, then explicit retry and End. Fix complexity: medium.

- **Must fix** — TT-02: completion can acquire the next turn's identity (INV §5/§11/§15).
  `internal/runtime/chat.go:995` releases the turn gate before `finishTurn` at `:921` emits the
  terminal event and clears `execTurnID`. A racing private Send or activation can claim/start B
  between A's settlement and terminal emission; A's terminal event is then stamped B at `:1746`
  and B's owner is cleared. A room contribution can remain unfinished or the wrong attempt can
  settle. TS-14.R4/R6. Bind terminal emission to the completing turn's immutable id and release
  ownership in the correct order. Regression: block A before terminal emission, admit racing Send
  B, then assert A's terminal identity and every B event retain their respective owners.
  Fix complexity: medium.

- **Must fix** — TT-03: late child activity leaks into a later room turn (INV §1/§11).
  `internal/runtime/subagent.go:136` stores a child's scope without its owning turn;
  `internal/runtime/chat.go:1746` stamps child events with the root's mutable `execTurnID`.
  A child spawned by private turn A can emit tools/diffs/permissions while room turn B runs,
  causing `internal/server/think_tank_capture.go:83` to retain them under B. Late activity from
  another room attempt is likewise misattributed. FS-21.R29, TS-14.R4/R10. Freeze original ownership
  in the child scope; test a child announced in A that emits after B starts and assert B captures
  none of it. The documented idle-gap omission does not permit reassignment. Fix complexity: medium.

- **Must fix** — TT-04: setup launches continue after room deletion (INV §4/§5/§15).
  `internal/server/think_tanks.go:209` launches pending slots from a stale room snapshot, without
  an in-flight setup claim or revalidation. During a slow setup launch, Pause/End followed by Delete
  is allowed (`internal/state/think_tanks.go:1204`); the worker can subsequently launch further
  reserved participants after deletion, then fail its state update. TS-14.R2/R14, FS-21.R40.
  Extend the existing durable launch-claim/deletion guard used by the judge to setup, and abandon
  unstarted slots at End/Delete. Test a blocked setup launch plus End/Delete, including multiple
  pending slots, and assert no later provider launch survives deleted room authority.
  Fix complexity: medium.

- **Must fix** — TT-05: eligibility checks do not guard final turn admission (INV §5/§15).
  `internal/server/think_tanks.go:86` checks archival/assigned work before acquiring the lifecycle
  claim; its `before(turnID)` callback at `:141` and `BeginThinkTankAttempt`
  (`internal/state/think_tank_turns.go:28`) recheck only room revision/opportunity. Agent/project
  archival or task assignment between those checks and admission can start room work under lost
  eligibility. TS-14.R3, FS-21.R22/R34/R37. Revalidate under the final claim and coordinate with the
  existing archive start lease and task reservation seam. Test barriers between initial selection
  and admission with archival/task reservation, observing durable rows and provider frames.
  Fix complexity: medium.

- **Must fix** — TT-06: missing/unreadable participant projects pass eligibility (INV §7).
  `internal/server/think_tanks.go:123` uses the general `projectArchiveGate`, which intentionally
  permits missing definitions, and explicitly ignores its internal-error refusal. Deleting a
  participant's project definition while retaining its agent leaves that agent room-eligible;
  corrupt/unreadable definitions also fail open. FS-21.R22/R37, TS-14.R3/R14 require a hold.
  Use a Think Tank-specific fail-closed live-project check, preserving the general lifecycle
  contract. Test removed and unreadable projects: no admission and a visible recovery/End reason.
  Fix complexity: easy.

- **Must fix** — TT-07: replayed annotation commands deliver duplicate mail (INV §15).
  `internal/server/think_tank_annotations.go:206` deduplicates the room record, then unconditionally
  inserts a fresh message at `:212`. Retrying the same command after a lost HTTP response delivers
  the instructions again, despite only one canonical annotation. TS-14.R9/§3, FS-13.R26–R27.
  Commit the room record and ordinary mail receipt atomically in the shared database, or deduplicate
  delivery with the command/input identity. Test identical retries and failure/response loss around
  publication and delivery; one room record and one delivered mail must result. Fix complexity: medium.

- **Must fix** — TT-08: exact create replay fails with any new participant (INV §11/§15).
  `internal/server/think_tank_handlers.go:135` reserves fresh agent ids on every POST before
  `CreateThinkTank` compares them (`internal/state/think_tanks.go:392`). Replaying an identical
  command with a new participant returns 409 rather than the original room. A lost create response
  therefore cannot be recovered through its stable command id. TS-14.R2/§3. Match immutable request
  intent and reuse its original reserved identities before allocating new ones; test mixed and
  all-new participant creation through REST twice with the same command. Fix complexity: medium.

- **Must fix** — TT-09: file links bypass retained room sources (INV §2/§10/§11).
  `ui/src/features/thinktank/ThinkTankPage.tsx:72` loads sources only while Files is open;
  `:107` otherwise invents `agent:<id>`, and `:400` uses the ordinary session-file endpoint.
  Clicking a contribution/diff link first can open a live file, but annotating its excerpt sends
  that invented source id and is rejected by `resolveThinkTankAnchors`
  (`internal/server/think_tank_annotations.go:114`). After source deletion even the file read fails,
  despite retained room cwd. FS-21.R26/R39, FS-13.R26, TS-14.R12/R15. Resolve immutable room source
  references independently of the Files panel and keep attempt/workspace provenance. Test direct
  Markdown/diff link → file selection → annotation without opening Files, then after agent deletion.
  Fix complexity: medium.

- **Must fix** — TT-10: completed activity cannot be inspected (INV §8/§10).
  `ui/src/features/thinktank/ThinkTankPage.tsx:391` wraps settled activity in a disabled fieldset.
  This disables ordinary `ToolRun`/`ToolCall` disclosure buttons, result Show more, diff file links
  and annotation buttons along with permission mutations. When a turn finishes, or an ended room
  is opened from Archive, retained tools/results can no longer be expanded or followed up normally.
  FS-21.R24/R26/R30/R39, TS-14.R15. Disable only stale source mutations; keep inspection, copy,
  file viewing and annotation controls enabled. Test expanding tools/results and annotating diffs
  in settled and ended rooms while stale approvals remain refused. Fix complexity: easy.

- **Must fix** — TT-11: first activity record defeats the REST byte bound (INV §16).
  `internal/server/think_tank_activity_handlers.go:38` applies the 1 MiB guard only after adding
  an item. A first tool/result/diff record may therefore return up to the 8 MiB capture ceiling in
  one activity response. TS-14.R17. Bound the first item too with an explicit marker/detail or
  continuation mechanism that preserves its anchor and allows progress. Regression with a 2 MiB
  first record returned 2,097,566 bytes against a 1,048,576-byte window. Fix complexity: medium.

- **Must fix** — TT-12: bounded activity projections silently omit history (INV §8/§16).
  `ui/src/api/thinkTanks.ts:137` silently retains only the newest 5,000 activity records, unlike
  entries which expose a clipping notice. `internal/server/think_tank_activity_handlers.go:87`
  scans only the oldest 10,000 records for Files/Commands and presents those results as complete;
  recent files/commands then disappear from valid long rooms. FS-21.R24/R39, TS-14.R16–R17.
  Keep bounded windows but expose truncation/continuation and choose the appropriate recent window
  for inspection. Test >5,000 browser rows and >10,000 server records with distinct late files and
  commands; omissions must be explicit and later activity must remain accessible.
  Fix complexity: medium.

- **Worth fixing** — TT-13: malformed read cursor panics instead of refusing (INV §8/§11).
  `internal/state/think_tank_turns.go:730` slices `e.Body[off:]` without validating the decoded
  cursor offset against the entry length (or UTF-8 boundaries). A syntactically accepted cursor
  with offset 999 for a five-byte entry causes a slice-bounds panic through the room tool path.
  TS-14.R7/§3's typed cursor refusal contract. Validate offset/position before slicing and bind
  continuation identity as specified; test out-of-range and mid-rune offsets with no mutation or
  panic. Fix complexity: easy.

Review evidence: focused Think Tank state/MCP/server tests passed after allowing temporary loopback
test listeners; focused room UI and style/presentation checks passed; spec lint and diff checks
passed. Temporary external Go overlays reproduced TT-07 (two mails), TT-08 (409), TT-11 (oversized
response), TT-13 (panic), TT-03 (child reassigned to B), TT-01 (running attempt after Stop), TT-06
(removed project accepted), and the gate-release interleaving underlying TT-02. These are review
probes, not committed regression coverage. Existing tests do not close the findings. The real-binary
rendered journey and credentialed provider gates above remain owed.

Invariant sweep: §1–§11 and §13–§17 apply across lifecycle/activation, SQLite, transport, UI,
capture and tests. No new external CLI invocation (§12) occurs in this unit. Shared launch/teardown,
closed tool/approval/result registration, room retention without agent/project cascades, array wire
fixtures, loopback/phone route boundaries, embedded knowledge inventory and defined presentation
selectors were checked; no separate findings on those surfaces. No product code or specs changed.

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

- **2026-10-06 — Think Tank fix integration checkpoint (INV §1/§2/§4/§5/§7/§8/§10/§11/§15/§16/§17).**
  Exit settles attempts without charging; setup claims fence deletion and final admission shares
  archive/task guards. Annotation mail and create replay are durable/idempotent; cursors validate
  origin and UTF-8 positions. Retained file sources, settled inspection and explicit bounded
  recent activity windows are wired. Focused tests pass; final closure checks are running.

- **2026-10-06 — Think Tank runtime ownership checkpoint (INV §1/§5/§11/§15).** Terminal
  completion holds the turn gate through its sink and uses the completing turn's immutable id;
  child scopes freeze origin ownership. Full runtime and focused race tests pass; pre-fix overlays
  fail both regressions. Lifecycle, replay and retained-activity fixes remain in active integration.

- **2026-10-06 — Review: Think Tanks.** Reviewed through `60ba960`; recorded twelve Must-fix
  findings covering Stop/completion/child ownership, setup/admission/project guards, command replay,
  retained-source file annotations, settled inspection and bounded activity, plus one malformed-cursor
  Worth-fixing item. Same unit stays open. Fix model: medium — Codex Terra or Claude Opus.

- **2026-10-06 — Work: Think Tanks implemented.** SQLite rooms with guarded single-floor turns,
  staged explicit contributions, committed read checkpoints, openings barrier, closing turn,
  pause/End, setup/judge launches, restart fencing; executing turn ids on runtime events; MCP
  `read_think_tank`/`submit_think_tank_turn`; REST/SSE; redacted activity capture; room-source
  annotations; room page, project entry, Archive list, agent room-turn notice; operating-chuck
  reference. Closure matrix (both Go variants, focused `-race`, UI tests/build, spec check) passed.
  Rendered real-binary and credentialed gates are owed.

- **2026-10-06 — Design: Think Tanks ready to implement.** Human confirmed the remaining feature
  defaults and SQLite plus explicit room tools. FS-21.R34–R40/A24–A30 and TS-14.R1–R18 close
  setup, retention, recovery, publication/read checkpoints and turn ownership; adjacent FS/TS
  requirements join shared launch, action, approval, persistence, UI and knowledge contracts.
  Added `docs/ready-changes/think-tanks.md`, removed the source idea, and left implementation
  inactive. Spec lint, twin-skill comparison and diff checks passed; a focused consistency check
  confirmed the ownership/read/privacy rules and clarified pending-judge deletion and catalog wording.

- **2026-10-05 — Design: Think Tank judge, manual ending and Steer confirmed.** FS-21.R31–R33/
  A21–A23 record the fresh end-only judge, graceful End discussion, and normal private Steer with
  visible room-turn identity. FS-03.R69/A50 add planned agent-view identification. Remaining product
  defaults are consolidated as unconfirmed proposals in FS-21 §6; technical design awaits scope
  approval. No product code was written.

- **2026-10-05 — Design: Think Tank conversation page and annotation destinations confirmed.**
  FS-21.R27/R30/A20 and FS-13.R27/A18 record the ordinary full conversation-page navigation and
  Room, selected-agent and New task destinations. New task retains its current meaning of launching
  a new normal agent with annotations as initial work. After room completion, selected-agent/new-agent
  follow-up remains available. Judge setup, End discussion and active-room steering await choices.

- **2026-10-05 — Design: Think Tank workspace entry and independent follow-up clarified.**
  FS-21.R27–R29/A17–A19 record the project-started separate workspace, normal participant cards in
  their own projects, continued room scheduling with waits for private work, and room-only activity.
  FS-02.R65/A47 add the planned button beside New agent (FS-02 now Partial). Page versus browser
  window, annotation targets and active-room steering remain open. No product code or technical
  design was added.

- **2026-10-05 — Design: Think Tank workspace and retention confirmed.** FS-21.R22–R26/A14–A16
  record cross-project participation limited to non-archived projects, local retention until explicit
  room deletion, and a full group-chat workspace with familiar features plus independent agent
  cards. FS-13.R26/A17 add planned room annotation sources (FS-13 now Partial). Card arrangement,
  private-send scheduling and room-versus-private activity visibility await choices. No product
  code or technical design was added.

- **2026-10-05 — Design: Think Tank pause, intervention and closing message confirmed.**
  FS-21.R18–R21/A11–A13 let the active turn finish on pause, hold the room visibly for approval or
  failure intervention, and end below two eligible participants with a closing-message opportunity
  within the sole remaining agent's allowance. Failed attempts do not consume the contribution
  limit and are not retried automatically. R8 is superseded by R12/R20; FS-21 remains Draft.

- **2026-10-05 — Design: Think Tank budget and operator controls confirmed.** FS-21.R14–R15/A8–A9
  record per-agent turn ceilings and pause/resume plus user messages between participant turns.
  FS-21.R16–R17/A10 also record the confirmed remaining-ceiling instruction, opening/departure
  accounting, and separate end-only judge budget. Pause/recovery and sole-participant closure were
  subsequently confirmed in R18–R21. No technical design or product code was added.

- **2026-10-05 — Design: Think Tank participation and synthesis confirmed.** FS-21.R11–R13/A6–A7
  record mixed new/existing participants, configurable departure permission and optional final
  departure message, and optional synthesis by a judge activated only after discussion ends.
  Discussion budget and operator controls are awaiting choices; the feature remains Draft.

- **2026-10-05 — Design: Think tanks resumed.** Replaced the initial consensus/group-chat framing
  with independent normal provider sessions and a canonical append-only shared discussion,
  incremental retrieval, optional independent openings, bounded discussion, and permitted departure
  ending when one participant remains. FS-21 is Draft; unresolved behavior is recorded there and
  in `docs/ideas.md`. No technical specification, ready change, or product code was written.

- **2026-10-05 — Design: provider bundle refresh ready.** Upstream check found Claude ACP 0.85.1,
  Codex ACP 2.1.1 (Codex `^0.159.1`, so 0.159.3), ACP SDK 1.7.0 and go-sdk 1.8.0. All ACP Wait-list
  gates stay closed; the Codex steering patch is still required; the MCP-migration gate is rechecked
  and closed, with MCP-over-ACP recorded as the transport to watch (TS-04 §5, FS-17 §6).
  `refresh-provider-bundle-2026-10.md` is waiting to start: FS-03.R67–R68/A48–A49,
  FS-09.R79/A48, TS-04.R79–R83, TS-06.R32, TS-08.R86 (TS-08 now Partial).

- **2026-10-05 — Claude 5.5 launch compatibility closed.** A live probe (release adapter over
  installed Claude Code 2.1.282) served `claude-opus-5-5`; the bundle still refuses it as too old.
  The probe exposed a remaining launch failure (INV §12 accepted-is-not-honored, over-applied):
  the Claude adapter answers a full model ID with its alias row (`opus`/`opus[1m]`), which Chuck
  rejected as an ignored setting. A listed reported row now honors an unlisted Claude model
  request (TS-04.R46 note); listed requests and other backends stay strict. Fake-adapter test
  failed first; Chuck's real launch path passed live and failed without the fix.

- **2026-10-05 — Phone UI coverage completed; Manage refusals shown.** Request/response tests now
  cover project-page launch and Start pipeline (with refusals keeping entered values), rename,
  fast/effort, clone, Commands, Open file, `RunScreen` controls, and the `/task/<id>` link
  (INV §17 independent oracle: MSW doubles return the server's error envelope). The refused-rename
  test failed first: the Manage tab never showed the Mac's reason (FS-20.R27); it now does. Phone
  UI tests (45) pass. Unit `phone-desktop-flow-and-agent-management` closed apart from its owed
  fakeACP phone-size browser passes.

- **2026-10-05 — Notification stale-agent path proved.** `NotificationCenter.test.tsx` now mounts
  the real `ChatPanel` route with a hydrated store lacking the agent, clicks the toast, and asserts
  **Agent not found** plus dismissal (INV §17 independent oracle; fails when the store is left
  unhydrated). UI tests pass. Unit `notifications-open-conversation` closed apart from FS-02.A46's
  owed real-browser and macOS click checks.

- **2026-10-05 — Phone chat streamed replies fixed.** The phone conversation folds its joined
  windows with the desktop's `foldTranscript`, so a streamed reply is one message (INV §2 canonical
  helpers); the forked permission folding is gone. `ui/scripts/phone-render.mjs` screenshots the
  phone conversation at iPhone size with stubbed APIs (INV §13), documented in TS-06; Playwright
  1.63.0 is a UI dev dependency. Reproduction test un-skipped (failed before, passes); UI tests
  pass. Unit closed.

- **2026-10-05 — Rename fixes; resident role is Chucky.** Cutover now rewrites and audits frozen
  session paths (`cwd`, `add_dirs`, prompt; `launch_config_json` audited), and A14 rehearses a
  resumed worktree session with the source unavailable (INV §7/§10/§15). Storage rationale names
  `~/.chuck` (INV §10). FirstMate/`firstmate` → Chucky/`chucky` across code, UI and specs. Both Go
  variants, spec/old-name checks and 107 affected UI tests pass. Rehearsal, repository rename and
  phone re-pair notice remain.

- **2026-10-05 — Rename reviewed.** Runtime, protocol, release and UI wiring passed the focused
  audit. Recorded the cutover's missing frozen-session path repair and stale current storage
  rationale; the rename unit remains open for those fixes and its already-recorded cutover gate.
- **2026-10-04 — Rename: AgentDeck is now Chuck.** Code, release, UI and docs renamed in one cut;
  supervised cutover guide written. `make test`, UI suite (595) and `make dist` pass. Rehearsal,
  GitHub repository rename and first Chuck release remain.

- **2026-10-04 — Release: `v0.9.0` published.** 46 commits after `v0.8.0`. The operator package and
  README already matched the range; pins unchanged. `make test`, full UI suite (591) and
  `make dist VERSION=0.9.0` pass. Settled state archived to `HANDOFF-through-2026-10-04`. Release
  workflow passed; Linux CI's darwin-only bundle test now skips off macOS (test-only).
