# FS-21 — Think tanks

**Status:** Partial
**Code:** `internal/state/think_tank*.go`, `internal/server/think_tank*.go`,
`internal/messaging/think_tank_tools.go`, `ui/src/features/thinktank/` · **Journeys:** —
**Absorbed:** —

## 1. Purpose

Several independent agents deliberate toward one outcome, especially architecture, system design,
research, and iterative critique. A durable shared discussion is the canonical room history;
each participant keeps its own ordinary provider session. TS-14 owns the architecture.

## 2. Behavior

- **R1** — A Think Tank has a shared goal and an append-only, durably ordered discussion.
  Every published contribution identifies its participant. The room artifact, rather than any
  individual agent conversation, is the canonical group discussion.
- **R2** — Each participant remains an independent normal Chuck/provider session with
  its own conversation and history. Other participants' contributions are explicitly read as
  attributed shared content; they are never fabricated assistant turns under the reader's identity.
  Reading shared content can enter that participant's normal provider context without making its
  transcript the canonical room history.
- **R3** — On a discussion turn, the participant can retrieve newer published entries
  in order from the point covered by its previous committed contribution, in bounded pages, and
  revisit earlier entries when needed. A read without a committed contribution must not silently
  make entries unavailable on retry. Incremental retrieval reduces repeated input; it does not
  promise to erase earlier material from provider context or replace provider context management.
- **R4** — Chuck activates the selected participant for its turn. Only one participant
  holds the discussion floor at a time, and its contribution is appended under its own identity
  before the discussion advances to another participant.
- **R5** — The room has a configured finite discussion budget and a visible completion
  state. R14 defines the budget as per-participant turn ceilings; R17 defines accounting for
  openings, exits, and optional judge synthesis; R21 defines failed-attempt accounting.
- **R6** — Independent openings are optional. When enabled, every participant forms
  its opening without access to the others' opening contributions; those contributions are
  published into the shared discussion before ordinary turn-taking begins. When disabled,
  collaboration begins immediately. This promises isolation from this room's openings, not
  erasure of a participant's pre-existing history or other knowledge.
- **R7** — Completion reports why the discussion stopped and preserves material
  disagreement. Agreement itself is not the objective, and a budget ending or participants leaving
  must not be presented as proof that the goal was achieved.
- **R8** — retired 2026-10-05: superseded by R12 and R20, which preserve configurable departure
  and add the sole remaining participant's closing-message opportunity before final completion.
- **R9** — A person inspecting the room can see its goal, participants, ordered
  attributed discussion, current phase and speaker when applicable, and completion reason.
- **R11** — Participant selection supports both creating a new normal agent and adding
  an existing normal agent. A single room can mix new and existing agents; existing participants
  retain their own session history under R2. R22 governs project eligibility; session scheduling
  eligibility is defined by R34/R37.
- **R12** — Permission to leave is configurable per participant. An allowed participant
  may choose to publish a final attributed message with its departure, or leave without a final
  message. The departure is recorded either way, and an optional final message is part of the
  canonical discussion before the remaining-participant completion rule in R20 is evaluated.
- **R13** — Final synthesis is optional. When requested, a judge agent is called only
  after the participant discussion has ended, reads the completed shared discussion, and produces
  the synthesis as the last step. Its synthesis is attributed to the judge and retained with the
  room under R1 and preserves unresolved material objections under R7. The judge does not take
  discussion turns or receive activations to monitor progress or decide whether discussion stops.
- **R14** — Discussion is bounded by a configured finite maximum turn count for each
  participant, rather than a room-wide round count or total-turn allowance. A participant cannot
  receive further discussion turns once its limit is exhausted. A limit is a ceiling, not a target
  number of contributions or evidence that convergence is required. R16 defines participant budget
  awareness, R17/R21 define contribution accounting, and R20 governs ending as participants
  leave or exhaust their limits.
- **R15** — The operator can pause discussion progression, resume it, and add user
  messages between participant turns. No new participant turn starts while the room is paused.
  User messages are durably ordered and attributed to the user in the canonical room discussion,
  available through the same incremental reading behavior as participant contributions. Handling
  an already-running turn is defined by R18; R35 defines messages submitted during a turn.
- **R16** — Participants are told their remaining turn ceiling. The instruction explicitly
  frames it as a maximum rather than a quota: use turns only for useful contributions, there is no
  obligation to use every available turn, unresolved material disagreement is acceptable, and early
  departure is available only when that participant is permitted to leave. This helps participants
  plan within the limit without requiring agreement or promising a quality improvement.
- **R17** — An independent opening consumes one participant turn. An optional final
  departure message is included within that departure turn, not an additional discussion turn.
  Optional end-only judge synthesis has its own final-step budget, separate from participant
  discussion limits. It does not reopen the participant discussion or extend their allowances.
- **R18** — Pause requested during an active participant turn lets that turn finish;
  it does not cancel the turn. Chuck acknowledges the pending pause, preserves the active speaker,
  and starts no next participant turn. Once the turn finishes, the room is paused until the
  operator resumes. R19 still applies if the active turn needs approval or fails.
- **R19** — If a speaker needs approval or its turn fails, the room waits for intervention
  and shows the reason. It does not automatically skip the speaker, advance discussion, or retry
  the failed turn. Waiting for approval/failure intervention does not constitute a departure or
  exhaust another participant's allowance. Ordinary agent approval controls remain in effect.
- **R20** — Ordinary discussion ends when departures or turn-limit exhaustion leave
  fewer than two eligible participants. A sole remaining participant is offered one closing-message
  opportunity using one of its remaining turns before room completion and the optional judge step.
  It may decline; no extra turn beyond its ceiling is granted. If nobody retains allowance, there
  is no further participant turn. Departure removes a participant from future discussion turns
  without stopping or deleting its normal session; exhaustion is recorded distinctly from voluntary
  departure. Approval/failure waits under R19 do not remove a participant for this test. No live
  judge is needed to decide when discussion stops.
- **R21** — Participant turn limits count completed contributions. A failed attempt
  consumes no contribution allowance; an explicitly retried turn consumes one only when its
  contribution completes. R19 prevents automatic failed-turn retries. This bounds completed room
  contributions, not total provider activations, elapsed time, or token cost across manual retries.
- **R22** — A room can include agents from different projects, provided those projects
  are not archived. Selection and subsequent room activation must respect this project boundary;
  participation does not implicitly unarchive a project. An archived project's earlier contributions
  remain in retained room history. R37 defines recovery when eligibility is lost.
- **R23** — Room history is stored in local Chuck-managed data without automatic expiry,
  retained until the room itself is explicitly deleted. Completing the room, stopping or deleting
  an agent, and archiving or deleting a project do not erase the room's committed history or
  attributed reasoning. Retained attribution remains intelligible after its agent/project is
  removed. File inspection still reflects the referenced file's actual availability; retention of
  room history does not imply retaining every file's contents.
- **R24** — The room is one chronological group-chat workspace with familiar conversation
  capabilities available inside it: Markdown, code and diagrams, selection/copy, tool/result and
  diff inspection, permission actions, annotations, file viewing, Files/Commands inspection, and
  user-message composition. Contributions identify their speaker and originating project. These
  capabilities preserve their normal behavior and source-specific availability; there is no
  synthetic shared provider identity, runtime, or working directory. R29 defines activity scope;
  R30 defines annotation targets; R35 defines participant-qualified autocomplete.
- **R25** — Each participant remains an individual normal agent with its ordinary card
  in its own project. The room presents participant identity, project, live state and access to
  that normal card/session under R27. A person can follow up privately or continue work with that
  agent through the normal conversation. Leaving or finishing
  the room does not automatically stop, archive or delete participant sessions. Existing lifecycle
  availability still governs direct follow-up; a removed agent retains room attribution without
  an unusable active-chat action. R28 governs private-send scheduling.
- **R26** — Room annotations capture the selected room contribution, diff or file with
  its actual source attribution and point-in-time excerpt, using the familiar selection and tray
  interaction. A file or diff retains its originating participant/project so equal relative paths
  in different workspaces are not conflated. File viewing resolves from the appropriate participant
  context under the existing conversation reader behavior. Room-source annotations are governed
  by FS-13.R26–R27 and R30.
- **R27** — Think Tank is a distinct room workspace, started from a non-archived project
  using a dedicated button beside **New agent**. It opens as its own full conversation page inside
  Chuck, like the existing agent-conversation page, not as
  a fabricated provider agent or a group of agent cards expanded into one provider conversation.
  Starting from one project does not move participating agents from their own projects. Clicking a
  participant in the room takes the person to that agent's normal card/conversation in its project.
  This uses the application's normal conversation-page navigation rather than requiring a separate
  browser or OS window.
- **R28** — Sending a private follow-up does not automatically pause the room. Other
  scheduled room turns continue. When the room reaches an agent busy with private work, it holds
  that speaker's opportunity, shows the wait, and starts the room turn when the agent is available.
  No simultaneous provider turns are required for one agent, and the room does not silently skip
  its selected speaker. Private turns do not consume room contribution allowance or masquerade as
  room contributions. Queueing a normal private Send remains distinct from steering a running
  turn; R33 governs normal Steer during a room-owned turn.
- **R29** — The group view and canonical room activity contain only room activity.
  Participant private exchanges, earlier session history, and unrelated tool/file/command activity
  are not automatically imported into the room. Full individual histories remain available through
  the participants' normal cards/conversations. Room tools, approvals, files and commands retain
  originating participant attribution under R24/R26. This scopes recorded activity; an independent
  agent still retains its normal private context under R2, which can inform a later room contribution.
- **R30** — Room annotations offer three explicit destinations: **Room**, a selected
  agent, or **New task**. Room delivery becomes attributed shared user input between participant
  turns, available through the shared artifact; it does not privately prompt every participant or
  interrupt the current speaker. Selected-agent delivery uses ordinary annotation delivery and
  independent follow-up behavior. **New task** means creating a new normal agent through the existing
  New Agent flow and delivering the annotations as that agent's initial work, preserving the
  ordinary launch configuration and cancellation/failure behavior. After discussion ends, the
  selected-agent and New task destinations remain available; Room delivery is unavailable and
  does not reopen completed discussion. FS-13.R27 governs these room-source destinations.
- **R31** — When optional synthesis is enabled, its judge is a fresh normal agent,
  configured through ordinary agent launch settings and launched only after participant discussion
  ends. It begins with its own fresh conversation, reads the completed room artifact, and produces
  the attributed synthesis under R13. An existing participant/provider history is not adopted as
  the judge's conversation. R36 defines final-step accounting and recovery.
- **R32** — The operator can **End discussion** before the automatic ending conditions.
  If a room turn is active, it finishes normally before discussion ends; no further participant
  turn starts, including a not-yet-started closing turn. With no active room turn, discussion ends
  without waiting for a selected participant's private work. History and objections are preserved,
  completion identifies the operator's action, and the configured optional judge runs afterward.
  This does not stop, archive or delete participant sessions or cancel their private work.
- **R33** — Normal Steer remains available under FS-03.R50 while a participant is taking
  a room turn. The individual agent view clearly identifies the active Think Tank turn, so the
  person can understand that steering affects the room contribution being formed. The steering
  instruction stays in the agent's ordinary private conversation rather than being silently copied
  into the room; the resulting room contribution retains its normal room attribution and accounting.
  Existing capability, immediate-delivery/fallback and refusal behavior remains unchanged. Steer
  does not turn a private instruction into a new shared user message or independently pause the room.
- **R34** — Setup requires at least two distinct, non-archived chat agents from
  non-archived projects. Busy agents can join without preempting their private or assigned work;
  room activation waits for ordinary execution eligibility. Turn-taking cycles through the fixed
  configured order. Each participant has an individually set positive contribution limit and
  departure permission, enabled by default. Independent openings and synthesis default off. Goal
  and membership remain fixed after start; annotation-created follow-up agents do not automatically
  join the room.
- **R35** — Room messages and Room-target annotations submitted during a participant
  turn are durably held for publication at its boundary. During independent openings they wait until
  opening publication, preserving the opening inputs. Shared user input consumes no participant
  allowance. File/skill autocomplete explicitly selects participant context so paths and available
  provider commands remain attributed to the correct workspace/session.
- **R36** — Judge setup uses ordinary launch settings, including a chosen non-archived
  project. The separate final-step budget is one completed synthesis contribution. A judge failure
  leaves participant discussion ended, shows the final-step failure separately, and supports
  explicit launch-setting repair/retry without reopening discussion or consuming participant limits.
- **R37** — Restart retains committed history and holds unfinished rooms for explicit
  resume/recovery; uncertain provider turns are not replayed automatically. Opening failure retains
  completed withheld answers and waits for explicit retry. End during openings publishes completed
  answers as a clearly partial set and records missing answers without invention. Pending user input
  is retained at End and identified as not discussed by a completed participant turn when applicable.
  An archived/removed participant or project holds the room with a reason: restore/unarchive the
  same identity and retry where normal lifecycle allows, or End discussion. Deleted identities are
  never silently substituted; retained history remains available for new independent follow-up.
- **R38** — Ordinary agent-facing room actions are membership-scoped. Recorded
  participants, including leavers, can retrieve retained room history; the fresh judge gains room
  access when launched. Contribution/departure actions require the caller's current turn authority
  and departure permission where applicable. Room creation, configuration, membership, manual
  ending and deletion are user-controlled. Room actions do not authorize private-transcript or
  unrelated project-data retrieval. The existing local API/same-machine trust boundary remains
  unchanged under TS-05.R3.
- **R39** — Rooms are listed on their originating project and in Archive, including
  after the originating project is removed. Retained room history includes committed messages and
  room activity shown with them, including tool/diff content and source attribution, rather than
  depending on a surviving participant transcript. File inspection does not snapshot entire files;
  annotation excerpts remain point-in-time captures, and file links retain origin context for
  ordinary inspection when the files remain available. FS-05.R39 governs Archive discovery.
- **R40** — Explicit room deletion is available only when paused or ended with no active
  attempt, including closing-message and judge work, through the normal confirmation flow. It deletes
  room history and abandons pending room work, including a judge not yet started, without deleting
  participant agents or their normal histories. Confirmation identifies pending synthesis when
  applicable. Live goal/membership editing, autonomous agent room creation,
  configurable full-file snapshots, export, and new phone room UI are outside this change.

- **R41 (planned)** — A pipeline-owned room under FS-14.R81–R84 uses the same attributed
  discussion, fresh end-only judge and explicit failure recovery as a standalone room. Its origin
  identifies the run/stage and all new agents use that run's project. The room page links to its
  run and exposes its phase and recovery controls. Stage context is durable shared room data,
  available to every participant and judge; it is not copied private provider history.
- **R42 (planned)** — Pipeline Stop overrides R25/R32's normal non-stopping behavior only for
  the room's owned execution. It suppresses future setup/participant/judge work, preserves already
  committed contributions, and never cancels unrelated private turns. A pipeline-pinned room cannot
  be deleted while retained run history needs it. Run deletion removes only the pin. Standalone
  room creation, manual End, private follow-up, retention and eligible deletion are unchanged.

- **R43 (planned)** — A room has a short title distinct from its full goal. The creation UI asks
  for a title; existing rooms and API callers omitting it receive a readable goal-derived fallback.
  Titles are fixed after creation in this change. Project cards, Archive, room navigation and
  individual-chat room cues use the title. The room's compact header leads with title and phase;
  the full goal remains available in a subordinate disclosure rather than occupying the first
  conversation viewport. Title fallback neither rewrites history nor changes the goal.
- **R44 (planned)** — Originating-project room discovery follows FS-02.R71: one distinct wide
  room card per row before the agent grid. Show every participant, phase/control, current speaker
  or all active openings, per-member remaining allowance, the sum of non-departed participants'
  remaining allowances, judge state and attention/completion reason. Allowance is a maximum, not
  expected rounds or progress toward agreement. Judge budget is separate. Missing/deleted sources
  keep retained identity without active-chat actions. A card opens the full room; participant chat
  links remain independent keyboard-accessible actions. Failed reads expose unavailable state
  rather than guessed zero allowances or an empty roster.
- **R45 (planned)** — Participant and judge chats retain the ordinary compact agent header,
  transcript and private composer. A restrained Think Tank header accent and short title/active-turn
  cue identify room work; full goals and explanations move into a Think Tank tab beside Files and
  Commands under FS-03.R71. The tab exposes associated rooms, room status, participants/judge,
  allowances and links to the whole room and every surviving member's ordinary chat. Membership
  stays discoverable between turns and after completion, with separate entries for multiple rooms.
  Private Send/Steer semantics under R28/R33 remain explicit and unchanged.
- **R46 (planned)** — The room composer is anchored at the bottom of the conversation region,
  below its scrollable discussion, and looks like the standard Chuck chat input: the same textarea,
  sizing, surface, spacing, focus treatment and Send action. Enter sends, Shift+Enter inserts a
  newline, and an open suggestion picker consumes its normal selection keys first. Queued-input
  and failure feedback stay adjacent; failed drafts remain available. Ended discussion is read-only.
  `@` offers participant-name mentions alongside distinctly labelled participant-qualified file
  suggestions; `#` preserves command/skill suggestions. Duplicate names are disambiguated by
  project/identity. A selected mention records a shared addressee visible to everyone and explicitly
  identified to that participant on its next scheduled room turn. It neither privately sends nor
  interrupts/reorders turns, grants allowance or wakes the judge early. Exhausted targets expose
  their lack of allowance; departed/deleted targets cannot be newly addressed. Plain unselected
  text is not silently resolved to an agent. R35's boundary/independent-opening hold still applies.
- **R47 (planned)** — Each participant's room contributions use a consistent subtle bubble
  background tint with a matching roster cue for that room, including reload and retained history.
  Every participant and judge is distinguishable; user input is distinct. Speaker names, project
  attribution, synthesis labels and textual states remain readable independently of color. Code,
  diffs, permissions and annotations retain their technical contrast in all three appearances.
- **R48 (planned)** — Eligible independent openings run concurrently: a participant's completion
  or private-work wait does not serialize other eligible openings. R6's isolation still holds;
  opening bodies/activity are withheld until all openings settle, then published in configured
  order. Show each opening's active/waiting/completed/failed state without disclosing hidden content.
  Ordinary discussion, closing and judge work retain their single-speaker rule. Pause/End prevent
  new opening starts and let already-running openings finish. Failure retains completed hidden peers
  for explicit retry; End publishes the completed partial set and records missing answers. Restart
  fences every uncertain opening for explicit recovery, never replaying completed peers.
- **R49 (planned)** — The operator can raise an individual participant's positive finite turn
  ceiling during open openings/discussion, whether running, paused or held. Show completed, maximum
  and remaining counts with the proposed higher value before Save; acknowledge the saved ceiling
  and update all room views. Increasing an exhausted member's ceiling restores its eligibility
  only while discussion is still open. It does not undo departure, failure or private-work waits,
  resume a pause, interrupt/steer a running turn, or charge a contribution. Subsequent activations
  receive the new ceiling; an already-running turn keeps its delivered instruction. Closing,
  end-requested, ended and judging rooms reject increases without reopening. Concurrent updates
  cannot silently lose an increase, and validation/conflict errors preserve the operator's draft.
  Decreases and unlimited budgets are excluded; existing finite bounds still apply.
- **R50 (planned)** — A successfully finalized judge synthesis is readable once as an attributed
  result in its own ordinary chat as well as canonical room history, with a link back to the room.
  The body is the exact submitted synthesis, not inferred provider prose or only collapsed tool
  arguments. Live/reloaded and archived chat show the same result; failed/staged judge output is
  not a completed synthesis. No synthetic assistant messages are inserted into other participants'
  chats. Explicit room deletion retains the judge's own result with its ordinary agent history.
- **R51 (planned)** — Newly deployed room participants and the fresh judge receive an ordinary
  agent group matching the room title in their own projects, reusing an existing same-named group.
  Existing participants keep their groups. Group assignment accompanies successful ordinary launch
  and retry does not reset later manual regrouping. Completing/deleting the room preserves groups
  and agents; no retrospective regrouping or new membership/group lifecycle is introduced. This
  narrowly extends FS-02.R65's previous unchanged-grouping statement for new room deployments.

## 3. States & transitions

The workspace/control upgrade in R43–R51 is planned. It extends the existing room lifecycle;
R48 changes only independent-opening concurrency and R49 permits live ceiling increases.

The core phase distinction from R4–R6, R13 and R20 is optional independent openings → shared
discussion → closing-message opportunity when one participant retains allowance → discussion ended
→ optional judge synthesis. The discussion completion reason remains distinct from the state of
the optional final step. R15/R18 add pause requested during a turn → paused after the turn finishes
→ explicit resume. R19 holds progression for approval/failure intervention. R28 waits for a selected
speaker's private work without automatically pausing the room. R32 adds end requested during a
room turn → finish that turn → discussion ended → optional fresh judge under R31. R34/R37 hold
assigned-work conflicts and restart/lifecycle loss; R36 separates synthesis failure from discussion
completion; R37 defines partial opening publication and pending input on End.

## 4. Edge cases & errors

- **R10** — A missing contribution or provider/process failure is not evidence of goal
  achievement and is not an explicit participant departure. The room exposes the unresolved
  condition honestly through the recovery behavior in R19/R36–R37.

## 5. Acceptance criteria

- **A1** (R1–R2, R4) — Run a three-participant discussion with distinguishable fake
  provider sessions. Compare the room history with each session's actual prompt/tool exchange:
  published contributions have stable room order and correct attribution, exactly one participant
  holds the floor, and no other participant's contribution is injected as the reader's assistant
  turn. *Verified by:* room/runtime integration tests and a rendered room journey after implementation.
- **A2** (R3) — A participant publishes after reading a bounded page sequence; on its
  next turn it retrieves only subsequent entries, can explicitly revisit older entries, and can
  traverse multiple pages without gaps. Interrupt a read before publication and prove those
  entries remain available to a retry. *Verified by:* room retrieval/commit integration tests.
- **A3** (R6) — With independent openings enabled, use recognizably different opening
  answers and observe all participant exchanges before publication: none can retrieve another's
  answer, and normal discussion begins only after all openings are published. Disable the option
  and prove the next speaker can read the first contribution immediately. *Verified by:* fake-provider
  integration tests; failure/recovery checks are covered by A27.
- **A4** (R5, R7, R9, R12, R20) — End a discussion at its configured budget with a material objection
  still present. Separately let permitted participants leave until only one remains, and attempt
  departure from a participant without permission. Check that the room shows the actual stop reason,
  preserves objections, refuses unauthorized departure, and leaves normal agent identities intact.
  *Verified by:* room state/action tests and a rendered completion journey after implementation.
- **A5** (R1, R10) — Restart Chuck after committed contributions and prove the same
  ordered, attributed discussion survives. A provider failure must not produce a false contribution,
  departure, or success state. *Verified by:* persistence/recovery integration tests; scheduling and
  retry assertions are additionally covered by A13/A27.
- **A6** (R2, R11–R12, R20) — Create a room with one new agent and two existing agents,
  each with distinguishable normal histories. Check that all participate under their own identities
  and existing agents retain their history. Configure departure for one participant and deny it for
  another. Exercise permitted departures with and without a final message in separate runs and
  verify the recorded departure, optional message ordering, and refusal for the participant without
  permission. *Verified by:* room selection/action integration tests and a rendered setup journey.
- **A7** (R7, R13) — Run rooms with synthesis enabled and disabled, ending by budget and
  by departures. Observe judge provider activations: none occur during participant discussion; an
  enabled final step reads the completed discussion and appends an attributed synthesis, while a
  disabled final step starts no judge turn. Preserve a known unresolved objection in the synthesis.
  *Verified by:* fake-provider integration tests and a manual synthesis-quality check; judge
  failure/retry assertions are additionally covered by A26.
- **A8** (R5, R14, R21) — Configure finite participant turn ceilings and run a fake-provider
  discussion. Observe completed contributions against their recorded counts and verify that
  none receives a discussion turn after its allowance is exhausted. Preserve an unresolved objection at budget
  completion instead of declaring convergence. *Verified by:* room/runtime integration tests;
  failed-attempt counts and unequal-exhaustion completion are additionally covered by A12–A13.
- **A9** (R1, R3, R15, R18) — Pause between participant turns, add a user message, and prove
  that no participant is activated while paused. Resume and verify the next participant can read
  the correctly ordered and attributed user message, while prior contributions remain intact.
  Separately request pause during an active fake-provider turn: the UI acknowledges the pending
  pause, that turn can commit its contribution without cancellation, and no next speaker starts
  before explicit resume. *Verified by:* fake-provider integration tests and a rendered
  pause/message/resume journey; message-submission races are additionally covered by A25.
- **A10** (R6, R12–R14, R16–R17) — Observe the actual activation/context delivered to fake
  participants and verify that it states their current remaining ceiling and the maximum-not-quota
  instruction. Exercise independent openings, a departure with a final message, and end-only judge
  synthesis: the opening uses one participant turn, the departure message grants no extra turn,
  and the judge operates under its separate final-step budget without extending participant limits.
  *Verified by:* room/runtime accounting and prompt-delivery integration tests. Real-run checks of
  quota-filling or premature convergence are qualitative; no quality outcome is promised by R16.
- **A11** (R10, R18–R19) — Hold the current speaker at an ordinary provider approval,
  then separately fail a turn. Check that the room names the reason, starts no next speaker or
  automatic retry, and records no voluntary departure. Resolve approval through normal agent
  controls; request pause while approval is outstanding and prove that the next speaker remains
  held after the active turn finishes. *Verified by:* fake-provider/room integration tests and a
  rendered intervention journey. Restart recovery is additionally covered by A27.
- **A12** (R7, R12–R14, R20) — Use departures and, separately, exhausted turn allowances
  to leave one eligible participant. Check that ordinary discussion stops, that participant is
  offered a closing-message opportunity within its remaining allowance, its published message
  precedes any judge synthesis, and the room preserves the actual stop reason and objections.
  Exercise declining the closing message and ending when no participant retains allowance; neither
  grants an extra turn. A temporarily approval-blocked speaker does not trigger this ending.
  *Verified by:* room/runtime integration tests and a rendered closing-message journey.
- **A13** (R14, R19, R21) — Fail a participant turn before a completed contribution and
  verify unchanged remaining allowance and no automatic retry. Explicitly retry, complete one
  contribution, and verify exactly one consumed turn and one canonical contribution. Reach the
  ceiling and refuse further participant turns, including a closing turn beyond that ceiling.
  *Verified by:* fake-provider/room accounting integration tests.
- **A14** (R11, R22–R23) — Create a room with participants from two non-archived projects,
  refuse an archived project, and retain committed contributions after an agent and a project are
  removed. Reload and restart to verify the same ordered history and readable attribution, without
  fabricated live-agent links. Explicitly delete the room under R40.
  *Verified by:* room/project lifecycle integration tests and a rendered retained-history journey;
  active-room archival and deletion behavior is additionally covered by A27/A30.
- **A15** (R24, R26) — In one room, two participants refer to the same relative file path
  in different workspaces. Open each file inside the room, inspect source/rendered content, select
  a file excerpt and diff lines, and annotate a room message. Verify correct participant/project
  attribution, path/line identity, captured excerpt, normal bounded tray behavior and preservation
  after a failed send. *Verified by:* room/file/annotation integration tests and a rendered room
  annotation/file-view journey; destination routing is additionally covered by A20.
- **A16** (R2, R24–R25) — Read a room containing Markdown, code, a diagram, tools, a diff,
  an approval, and user messages. Exercise the corresponding familiar controls inside the room,
  then open a participant card and continue in that agent's normal conversation, preserving its
  identity/history. Finish the room and prove the remaining normal sessions are still available
  for follow-up. *Verified by:* room component/fake-provider integration tests and a real-browser
  journey in Core, Sky & Grove and Studio at the supported desktop floor and a wider viewport;
  private-send scheduling and activity visibility are additionally covered by A18–A19.
- **A17** (R11, R22, R25, R27) — From a non-archived project, activate the Think Tank
  button beside New agent and open the separate room workspace. Mix new participants and existing
  participants from other non-archived projects. Verify that actual participant cards remain in
  their own projects, the room is not a provider-agent card, and clicking each room participant
  reaches its own normal card/conversation with preserved identity and history. *Verified by:*
  project/room navigation component tests and a rendered creation/follow-up journey, using the
  normal application's full conversation-page navigation.
- **A18** (R4, R14, R21, R28–R29) — Send a private follow-up to an off-floor participant
  while another agent holds the room turn. Let the room advance to that still-busy participant:
  verify visible waiting with the same scheduled speaker, no automatic pause or skipped speaker,
  and no overlapping provider turn for that agent. Complete its private work and then its room
  contribution. Only the room contribution consumes allowance or appears as a room reply.
  *Verified by:* fake-provider/room scheduling integration tests and a rendered room/private-chat
  journey; steering an active room turn is additionally covered by A23.
- **A19** (R24, R26, R29) — Seed participants with earlier and private messages, files,
  commands and tool events, then give each distinguishable room-turn activity. Inspect the room's
  conversation, Files/Commands and participant affordances: room activity is attributed and
  inspectable, private/earlier activity is not imported, and the normal individual conversation
  still exposes its own history. Equal relative paths from two room participants open in the
  correct workspace. *Verified by:* room projection/file integration tests and a rendered group
  view versus individual-history journey.
- **A20** (R15, R26, R30) — From a room annotation tray, exercise Room, selected-agent
  and New task delivery. Verify shared Room input appears between participant contributions and
  is read on subsequent turns; selected-agent delivery reaches that normal session; New task opens
  the normal New Agent flow and starts a new agent whose initial work is the delivered annotations.
  Cancel creation or refuse delivery and preserve drafts. After room completion, Room delivery is
  unavailable while selected-agent/New task follow-up still works and records its source annotation.
  *Verified by:* room/annotation/launch integration tests and a rendered three-destination journey.
- **A21** (R2, R13, R31) — Configure optional judge synthesis and complete participant
  discussion. Observe no judge launch during discussion; afterward a distinct fresh normal agent
  starts with the room context and its own fresh conversation, then appends the attributed synthesis.
  A room with synthesis disabled launches no judge. *Verified by:* fake-provider/room launch and
  context-delivery integration tests, plus A7's synthesis-quality check.
- **A22** (R7, R25, R31–R32) — End discussion during an active room turn and verify that
  turn can finish, no later participant/closing turn starts, the actual stop reason and objections
  survive, and the fresh judge runs only if configured. Separately end while waiting for another
  agent's private work; completion does not cancel that work or delete participant sessions.
  *Verified by:* room/fake-provider state integration tests and a rendered early-ending journey.
- **A23** (R14, R29, R33) — While a participant holds a room turn, open its normal
  conversation and observe the Think Tank turn identification. Use supported normal Steer and
  verify that the instruction enters that agent's private transcript, can affect its room
  contribution, and is not copied as a shared user entry. The contribution consumes its normal
  allowance; no additional turn or pause is fabricated. Preserve ordinary capability/refusal and
  turn-finished fallback checks. *Verified by:* room/agent composer fake-provider integration tests
  and a rendered room-turn steering journey.
- **A24** (R11, R14, R22, R34) — Create a mixed new/existing room with unequal positive
  limits; reject duplicate, archived, terminal and insufficient participants. Observe the fixed
  rotation skipping only departed/exhausted members and waiting for busy/assigned work. Check the
  documented toggle defaults and that an annotation-created agent remains outside membership.
  *Verified by:* setup validation and fake-provider scheduling tests.
- **A25** (R3, R6, R15, R35) — Send user input during an active turn and during independent
  openings; restart before publication. Verify durable preservation, publication only at the proper
  boundary, correct order/attribution, no changed later opening input, and unchanged participant
  allowance. Exercise autocomplete against two participants with distinct cwd/command catalogs.
  *Verified by:* room input/context integration tests and a rendered queued-input journey.
- **A26** (R13, R31, R36) — Fail a configured fresh judge, inspect the separately failed
  final step alongside ended participant discussion, repair its launch settings and explicitly retry.
  Verify one completed synthesis, no participant restart, and unchanged participant budgets.
  *Verified by:* fake-provider judge lifecycle/accounting tests and a rendered final-step recovery.
- **A27** (R6, R19, R23, R32, R37) — Restart during a provider attempt, end incomplete
  independent openings, and archive/remove a participant or project in separate cases. Verify no
  automatic uncertain replay, preservation of completed openings, partial/missing markers and retained
  undiscussed user input. Restore eligibility where possible or End; no substitute agent is invented.
  *Verified by:* restart/lifecycle integration tests and a rendered interruption-recovery journey.
- **A28** (R2, R12, R33, R38) — Use token-bound participant, leaver, fresh judge and
  nonmember identities. Check retained reads for members, rejection for nonmembers through room
  actions, and refusal of stale/foreign turn contributions and unauthorized departures. Room tools
  cannot create/end/delete rooms or fetch another agent's private transcript. *Verified by:*
  MCP/room authorization and stale-attempt tests; local API trust remains TS-05.R3.
- **A29** (R23, R26, R39) — Find active and ended rooms on the originating project and
  in Archive; remove the project/participant rows, reload, and inspect retained messages, tools,
  diffs and annotations. Existing files still open from retained origin context; missing files return
  normal unavailable feedback without losing excerpts. *Verified by:* room/Archive/file integration
  tests and a rendered retained-room discovery journey.
- **A30** (R23, R25, R40) — Refuse deletion while a participant, closing-message or judge
  attempt is active, including a judge running after discussion ended. Pause/end, confirm
  deletion with no active attempt, and verify room data disappears while participant identities,
  provider histories and normal agent conversations remain intact. Separately delete before a
  pending judge starts and verify no delayed judge activation. Cancel confirmation and change
  nothing. *Verified by:* room delete/lifecycle integration tests and a rendered deletion journey.

- **A31 (planned)** (R41) — A pipeline-created room shows attributed stage inputs, same-project
  fresh participants and required fresh judge, with links in both directions. A failed judge is
  retried without participant discussion reopening; published synthesis supplies FS-14.A48's
  output. *Verify:* server/state integration tests and FS-14.A50's rendered journey.
- **A32 (planned)** (R42) — Pipeline Stop during a room turn cancels that turn and fences later
  launches, while a participant's unrelated private turn is untouched. Retained run history pins
  the room; deleting the terminal run preserves history and releases the pin. Standalone End still
  finishes the active turn normally and does not stop agents. *Verify:* lifecycle/race tests under
  FS-14.A49 and the existing standalone End regression coverage.

- **A33 (planned)** (R43–R45, R47, R51) — Create a titled room with a long goal, dense roster and
  mixed existing/new cross-project agents. At 1024px and a wider desktop in Core, Sky & Grove and
  Studio, inspect its card before the agent grid, all member identities/budgets, concurrent-opening
  and attention/ended states, compact room/participant headers and stable speech tints. Navigate
  project → room → participant → Think Tank tab → another chat/room; multiple memberships and
  deleted sources stay intelligible. New agents/judge use the title group, existing agents retain
  theirs, reused labels/manual regrouping survive retries and completion. Existing/title-less
  creation and pre-migration rows get a fallback without goal/history changes; duplicate create
  commands reuse reserved identities and the original title. *Verify:* server/UI contracts plus an
  isolated real-binary fake-provider rendered journey, including Archive.
- **A34 (planned)** (R46) — The room input matches a normal Chuck chat input side by side in all
  appearances at desktop floor/wide widths. Scroll a long discussion; the input remains anchored.
  Exercise Enter/Shift+Enter, keyboard picker selection/Escape, distinguish agent/file/command
  suggestions and duplicate names, send to multiple shared addressees, preserve failed drafts and
  queued input through turn/opening boundaries. Targets see explicit addressees on their next room
  turn; private Send/Steer, speaker order and allowance remain independent. Exhausted/departed/
  deleted targets, stale membership and ended-room sends report truthful refusal/feedback.
  *Verify:* state/MCP/wire/UI tests and the rendered A33 journey; include FS-02.A48–A49's planned
  shared textarea/icon behavior when integrated.
- **A35 (planned)** (R48) — Hold two or more fake-provider opening turns simultaneously and
  observe overlapping provider frames before any finishes. Read room history/activity through
  REST/SSE and another participant: no peer opening leaks. Finish out of order; publication uses
  configured order once after the barrier. Busy peers, failed opening/retry, Pause/End during
  concurrent admission/completion, Stop/restart and stale callbacks preserve committed openings
  and exact accounting without duplicate effects. Ordinary discussion still has one floor.
  *Verify:* state/server/runtime race integration tests plus A33's concurrent-opening UI journey.
- **A36 (planned)** (R49) — Increase a ceiling during an active opening/turn and while paused or
  held; verify persistence/reload, unchanged completed counts, updated cards, future activation
  context and exhausted-member re-eligibility. Test duplicate commands, simultaneous increases,
  completion/End/closing races, bounds, departed/judge targets and planned pipeline closure guards.
  A refused update preserves input and changes no counts/phase; a saved increase does not resume,
  steer or clear unrelated holds. *Verify:* state/HTTP/MCP tests and A33's live-budget journey.
- **A37 (planned)** (R50) — Complete synthesis with a body different from the judge's incidental
  provider reply; inspect its ordinary chat live, on reload and in Archive. Exactly one attributed
  result equals the committed synthesis and links to the room; failed attempts and duplicated
  completion callbacks produce no false/duplicate result. Room deletion preserves that result in
  the judge history; participant chats receive no fabricated assistant messages. *Verify:* durable
  transcript/projection and UI tests plus the rendered judge/Archive journey in A33.

## 6. Deviations & open decisions

R43–R51/A33–A37 are the 2026-10-06 confirmed Think Tank workspace/live-control upgrade, including
the explicit requirement to match the standard Chuck chat input. They remain unshipped and planned.
FS-02.R66–R67 and TS-08.R88–R89 already own pending shared auto-grow fields and icon actions;
this upgrade uses their shared seams rather than a separate room input design. Pipeline-owned
rooms under R41–R42 use these general room controls subject to their run/stage closure guards.

Shipped 2026-10-06. R35's autocomplete uses the agent composer's shared `@`/`#` picker against one
participant chosen in the room composer, and appends that participant's name to each inserted
token. R8 is superseded by R12/R20. A new participant's turn limit defaults to 3 in setup.
Rendered acceptance so far uses stubbed room data in every appearance at 1024px and 1440px; the
real-binary fake-ACP journey and the credentialed Claude/Codex smoke (TS-06.R33) remain owed.

## 7. Traceability

- Origin: resumed **Think tanks** entry in `docs/ideas.md`; human scope revision 2026-10-05.
- Adjacent capabilities: FS-01 independent agent lifecycle; FS-03 ordinary provider chat and
  permission controls; FS-06 point-to-point mail; FS-11 opaque project resources; FS-13 annotations;
  FS-15 bounded context retrieval; FS-16 durable work coordination. Their existing contracts remain
  distinct.
- Adjacent coverage: FS-02.R65/A47, FS-03.R69–R70/A50–A51, FS-13.R26–R27/A17–A18,
  FS-05.R39/A22, FS-17.R21/A12 and FS-18.R19/A15. TS-14 owns the selected architecture.
- Tests: `internal/state/think_tanks_test.go`, `internal/server/think_tank{s,_handlers,_capture}_test.go`,
  `internal/messaging/think_tank_tools_test.go`, `ui/src/features/thinktank/ThinkTankPage.test.tsx`;
  rendered review `ui/scripts/room-render.mjs`.
