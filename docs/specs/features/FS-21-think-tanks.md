# FS-21 — Think tanks

**Status:** Draft
**Code:** — (not implemented) · **Journeys:** —
**Absorbed:** —

## 1. Purpose

Several independent agents deliberate toward one outcome, especially architecture, system design,
research, and iterative critique. A durable shared discussion is the canonical room history;
each participant keeps its own ordinary provider session. This draft records the requested core,
not an approved feature scope or implementation-ready change.

## 2. Behavior

- **R1** (planned) — A Think Tank has a shared goal and an append-only, durably ordered discussion.
  Every published contribution identifies its participant. The room artifact, rather than any
  individual agent conversation, is the canonical group discussion.
- **R2** (planned) — Each participant remains an independent normal Chuck/provider session with
  its own conversation and history. Other participants' contributions are explicitly read as
  attributed shared content; they are never fabricated assistant turns under the reader's identity.
  Reading shared content can enter that participant's normal provider context without making its
  transcript the canonical room history.
- **R3** (planned) — On a discussion turn, the participant can retrieve newer published entries
  in order from the point covered by its previous committed contribution, in bounded pages, and
  revisit earlier entries when needed. A read without a committed contribution must not silently
  make entries unavailable on retry. Incremental retrieval reduces repeated input; it does not
  promise to erase earlier material from provider context or replace provider context management.
- **R4** (planned) — Chuck activates the selected participant for its turn. Only one participant
  holds the discussion floor at a time, and its contribution is appended under its own identity
  before the discussion advances to another participant.
- **R5** (planned) — The room has a configured finite discussion budget and a visible completion
  state. R14 defines the budget as per-participant turn ceilings; R17 defines accounting for
  openings, exits, and optional judge synthesis; R21 defines failed-attempt accounting.
- **R6** (planned) — Independent openings are optional. When enabled, every participant forms
  its opening without access to the others' opening contributions; those contributions are
  published into the shared discussion before ordinary turn-taking begins. When disabled,
  collaboration begins immediately. This promises isolation from this room's openings, not
  erasure of a participant's pre-existing history or other knowledge.
- **R7** (planned) — Completion reports why the discussion stopped and preserves material
  disagreement. Agreement itself is not the objective, and a budget ending or participants leaving
  must not be presented as proof that the goal was achieved.
- **R8** — retired 2026-10-05: superseded by R12 and R20, which preserve configurable departure
  and add the sole remaining participant's closing-message opportunity before final completion.
- **R9** (planned) — A person inspecting the room can see its goal, participants, ordered
  attributed discussion, current phase and speaker when applicable, and completion reason.
- **R11** (planned) — Participant selection supports both creating a new normal agent and adding
  an existing normal agent. A single room can mix new and existing agents; existing participants
  retain their own session history under R2. R22 governs project eligibility; session scheduling
  eligibility remains open in §6.
- **R12** (planned) — Permission to leave is configurable per participant. An allowed participant
  may choose to publish a final attributed message with its departure, or leave without a final
  message. The departure is recorded either way, and an optional final message is part of the
  canonical discussion before the remaining-participant completion rule in R20 is evaluated.
- **R13** (planned) — Final synthesis is optional. When requested, a judge agent is called only
  after the participant discussion has ended, reads the completed shared discussion, and produces
  the synthesis as the last step. Its synthesis is attributed to the judge and retained with the
  room under R1 and preserves unresolved material objections under R7. The judge does not take
  discussion turns or receive activations to monitor progress or decide whether discussion stops.
- **R14** (planned) — Discussion is bounded by a configured finite maximum turn count for each
  participant, rather than a room-wide round count or total-turn allowance. A participant cannot
  receive further discussion turns once its limit is exhausted. A limit is a ceiling, not a target
  number of contributions or evidence that convergence is required. R16 defines participant budget
  awareness, R17/R21 define contribution accounting, and R20 governs ending as participants
  leave or exhaust their limits.
- **R15** (planned) — The operator can pause discussion progression, resume it, and add user
  messages between participant turns. No new participant turn starts while the room is paused.
  User messages are durably ordered and attributed to the user in the canonical room discussion,
  available through the same incremental reading behavior as participant contributions. Handling
  an already-running turn is defined by R18; messages submitted during a turn remain open in §6.
- **R16** (planned) — Participants are told their remaining turn ceiling. The instruction explicitly
  frames it as a maximum rather than a quota: use turns only for useful contributions, there is no
  obligation to use every available turn, unresolved material disagreement is acceptable, and early
  departure is available only when that participant is permitted to leave. This helps participants
  plan within the limit without requiring agreement or promising a quality improvement.
- **R17** (planned) — An independent opening consumes one participant turn. An optional final
  departure message is included within that departure turn, not an additional discussion turn.
  Optional end-only judge synthesis has its own final-step budget, separate from participant
  discussion limits. It does not reopen the participant discussion or extend their allowances.
- **R18** (planned) — Pause requested during an active participant turn lets that turn finish;
  it does not cancel the turn. Chuck acknowledges the pending pause, preserves the active speaker,
  and starts no next participant turn. Once the turn finishes, the room is paused until the
  operator resumes. R19 still applies if the active turn needs approval or fails.
- **R19** (planned) — If a speaker needs approval or its turn fails, the room waits for intervention
  and shows the reason. It does not automatically skip the speaker, advance discussion, or retry
  the failed turn. Waiting for approval/failure intervention does not constitute a departure or
  exhaust another participant's allowance. Ordinary agent approval controls remain in effect.
- **R20** (planned) — Ordinary discussion ends when departures or turn-limit exhaustion leave
  fewer than two eligible participants. A sole remaining participant is offered one closing-message
  opportunity using one of its remaining turns before room completion and the optional judge step.
  It may decline; no extra turn beyond its ceiling is granted. If nobody retains allowance, there
  is no further participant turn. Departure removes a participant from future discussion turns
  without stopping or deleting its normal session; exhaustion is recorded distinctly from voluntary
  departure. Approval/failure waits under R19 do not remove a participant for this test. No live
  judge is needed to decide when discussion stops.
- **R21** (planned) — Participant turn limits count completed contributions. A failed attempt
  consumes no contribution allowance; an explicitly retried turn consumes one only when its
  contribution completes. R19 prevents automatic failed-turn retries. This bounds completed room
  contributions, not total provider activations, elapsed time, or token cost across manual retries.
- **R22** (planned) — A room can include agents from different projects, provided those projects
  are not archived. Selection and subsequent room activation must respect this project boundary;
  participation does not implicitly unarchive a project. An archived project's earlier contributions
  remain in retained room history. Recovery when a participant's project becomes archived is open
  in §6.
- **R23** (planned) — Room history is stored in local Chuck-managed data without automatic expiry,
  retained until the room itself is explicitly deleted. Completing the room, stopping or deleting
  an agent, and archiving or deleting a project do not erase the room's committed history or
  attributed reasoning. Retained attribution remains intelligible after its agent/project is
  removed. File inspection still reflects the referenced file's actual availability; retention of
  room history does not imply retaining every file's contents.
- **R24** (planned) — The room is one chronological group-chat workspace with familiar conversation
  capabilities available inside it: Markdown, code and diagrams, selection/copy, tool/result and
  diff inspection, permission actions, annotations, file viewing, Files/Commands inspection, and
  user-message composition. Contributions identify their speaker and originating project. These
  capabilities preserve their normal behavior and source-specific availability; there is no
  synthetic shared provider identity, runtime, or working directory. R29 defines activity scope;
  R30 defines annotation targets; participant-qualified autocomplete remains open in §6.
- **R25** (planned) — Each participant remains an individual normal agent with its ordinary card
  in its own project. The room presents participant identity, project, live state and access to
  that normal card/session under R27. A person can follow up privately or continue work with that
  agent through the normal conversation. Leaving or finishing
  the room does not automatically stop, archive or delete participant sessions. Existing lifecycle
  availability still governs direct follow-up; a removed agent retains room attribution without
  an unusable active-chat action. R28 governs private-send scheduling.
- **R26** (planned) — Room annotations capture the selected room contribution, diff or file with
  its actual source attribution and point-in-time excerpt, using the familiar selection and tray
  interaction. A file or diff retains its originating participant/project so equal relative paths
  in different workspaces are not conflated. File viewing resolves from the appropriate participant
  context under the existing conversation reader behavior. Room-source annotations are governed
  by FS-13.R26–R27 and R30.
- **R27** (planned) — Think Tank is a distinct room workspace, started from a non-archived project
  using a dedicated button beside **New agent**. It opens as its own full conversation page inside
  Chuck, like the existing agent-conversation page, not as
  a fabricated provider agent or a group of agent cards expanded into one provider conversation.
  Starting from one project does not move participating agents from their own projects. Clicking a
  participant in the room takes the person to that agent's normal card/conversation in its project.
  This uses the application's normal conversation-page navigation rather than requiring a separate
  browser or OS window.
- **R28** (planned) — Sending a private follow-up does not automatically pause the room. Other
  scheduled room turns continue. When the room reaches an agent busy with private work, it holds
  that speaker's opportunity, shows the wait, and starts the room turn when the agent is available.
  No simultaneous provider turns are required for one agent, and the room does not silently skip
  its selected speaker. Private turns do not consume room contribution allowance or masquerade as
  room contributions. Queueing a normal private Send remains distinct from steering a running
  turn; R33 governs normal Steer during a room-owned turn.
- **R29** (planned) — The group view and canonical room activity contain only room activity.
  Participant private exchanges, earlier session history, and unrelated tool/file/command activity
  are not automatically imported into the room. Full individual histories remain available through
  the participants' normal cards/conversations. Room tools, approvals, files and commands retain
  originating participant attribution under R24/R26. This scopes recorded activity; an independent
  agent still retains its normal private context under R2, which can inform a later room contribution.
- **R30** (planned) — Room annotations offer three explicit destinations: **Room**, a selected
  agent, or **New task**. Room delivery becomes attributed shared user input between participant
  turns, available through the shared artifact; it does not privately prompt every participant or
  interrupt the current speaker. Selected-agent delivery uses ordinary annotation delivery and
  independent follow-up behavior. **New task** means creating a new normal agent through the existing
  New Agent flow and delivering the annotations as that agent's initial work, preserving the
  ordinary launch configuration and cancellation/failure behavior. After discussion ends, the
  selected-agent and New task destinations remain available; Room delivery is unavailable and
  does not reopen completed discussion. FS-13.R27 governs these room-source destinations.
- **R31** (planned) — When optional synthesis is enabled, its judge is a fresh normal agent,
  configured through ordinary agent launch settings and launched only after participant discussion
  ends. It begins with its own fresh conversation, reads the completed room artifact, and produces
  the attributed synthesis under R13. An existing participant/provider history is not adopted as
  the judge's conversation. Final-step accounting and recovery remain in §6.
- **R32** (planned) — The operator can **End discussion** before the automatic ending conditions.
  If a room turn is active, it finishes normally before discussion ends; no further participant
  turn starts, including a not-yet-started closing turn. With no active room turn, discussion ends
  without waiting for a selected participant's private work. History and objections are preserved,
  completion identifies the operator's action, and the configured optional judge runs afterward.
  This does not stop, archive or delete participant sessions or cancel their private work.
- **R33** (planned) — Normal Steer remains available under FS-03.R50 while a participant is taking
  a room turn. The individual agent view clearly identifies the active Think Tank turn, so the
  person can understand that steering affects the room contribution being formed. The steering
  instruction stays in the agent's ordinary private conversation rather than being silently copied
  into the room; the resulting room contribution retains its normal room attribution and accounting.
  Existing capability, immediate-delivery/fallback and refusal behavior remains unchanged. Steer
  does not turn a private instruction into a new shared user message or independently pause the room.

## 3. States & transitions

The core phase distinction from R4–R6, R13 and R20 is optional independent openings → shared
discussion → closing-message opportunity when one participant retains allowance → discussion ended
→ optional judge synthesis. The discussion completion reason remains distinct from the state of
the optional final step. R15/R18 add pause requested during a turn → paused after the turn finishes
→ explicit resume. R19 holds progression for approval/failure intervention. R28 waits for a selected
speaker's private work without automatically pausing the room. R32 adds end requested during a
room turn → finish that turn → discussion ended → optional fresh judge under R31. Assigned-work conflicts,
restart/interruption, synthesis failure, and failed-opening publication await the decisions in §6.

## 4. Edge cases & errors

- **R10** (planned) — A missing contribution or provider/process failure is not evidence of goal
  achievement and is not an explicit participant departure. The room must expose the unresolved
  condition honestly; the operator's recovery actions remain open in §6.

## 5. Acceptance criteria

- **A1** (planned; R1–R2, R4) — Run a three-participant discussion with distinguishable fake
  provider sessions. Compare the room history with each session's actual prompt/tool exchange:
  published contributions have stable room order and correct attribution, exactly one participant
  holds the floor, and no other participant's contribution is injected as the reader's assistant
  turn. *Verified by:* room/runtime integration tests and a rendered room journey after implementation.
- **A2** (planned; R3) — A participant publishes after reading a bounded page sequence; on its
  next turn it retrieves only subsequent entries, can explicitly revisit older entries, and can
  traverse multiple pages without gaps. Interrupt a read before publication and prove those
  entries remain available to a retry. *Verified by:* room retrieval/commit integration tests.
- **A3** (planned; R6) — With independent openings enabled, use recognizably different opening
  answers and observe all participant exchanges before publication: none can retrieve another's
  answer, and normal discussion begins only after all openings are published. Disable the option
  and prove the next speaker can read the first contribution immediately. *Verified by:* fake-provider
  integration tests; failure/recovery checks await §6.
- **A4** (planned; R5, R7, R9, R12, R20) — End a discussion at its configured budget with a material objection
  still present. Separately let permitted participants leave until only one remains, and attempt
  departure from a participant without permission. Check that the room shows the actual stop reason,
  preserves objections, refuses unauthorized departure, and leaves normal agent identities intact.
  *Verified by:* room state/action tests and a rendered completion journey after implementation.
- **A5** (planned; R1, R10) — Restart Chuck after committed contributions and prove the same
  ordered, attributed discussion survives. A provider failure must not produce a false contribution,
  departure, or success state. *Verified by:* persistence/recovery integration tests; scheduling and
  retry assertions await the recovery contract in §6.
- **A6** (planned; R2, R11–R12, R20) — Create a room with one new agent and two existing agents,
  each with distinguishable normal histories. Check that all participate under their own identities
  and existing agents retain their history. Configure departure for one participant and deny it for
  another. Exercise permitted departures with and without a final message in separate runs and
  verify the recorded departure, optional message ordering, and refusal for the participant without
  permission. *Verified by:* room selection/action integration tests and a rendered setup journey.
- **A7** (planned; R7, R13) — Run rooms with synthesis enabled and disabled, ending by budget and
  by departures. Observe judge provider activations: none occur during participant discussion; an
  enabled final step reads the completed discussion and appends an attributed synthesis, while a
  disabled final step starts no judge turn. Preserve a known unresolved objection in the synthesis.
  *Verified by:* fake-provider integration tests and a manual synthesis-quality check; judge
  failure/retry assertions await §6.
- **A8** (planned; R5, R14, R21) — Configure finite participant turn ceilings and run a fake-provider
  discussion. Observe completed contributions against their recorded counts and verify that
  none receives a discussion turn after its allowance is exhausted. Preserve an unresolved objection at budget
  completion instead of declaring convergence. *Verified by:* room/runtime integration tests;
  failed-attempt counts and unequal-exhaustion completion are additionally covered by A12–A13.
- **A9** (planned; R1, R3, R15, R18) — Pause between participant turns, add a user message, and prove
  that no participant is activated while paused. Resume and verify the next participant can read
  the correctly ordered and attributed user message, while prior contributions remain intact.
  Separately request pause during an active fake-provider turn: the UI acknowledges the pending
  pause, that turn can commit its contribution without cancellation, and no next speaker starts
  before explicit resume. *Verified by:* fake-provider integration tests and a rendered
  pause/message/resume journey; message-submission races await §6.
- **A10** (planned; R6, R12–R14, R16–R17) — Observe the actual activation/context delivered to fake
  participants and verify that it states their current remaining ceiling and the maximum-not-quota
  instruction. Exercise independent openings, a departure with a final message, and end-only judge
  synthesis: the opening uses one participant turn, the departure message grants no extra turn,
  and the judge operates under its separate final-step budget without extending participant limits.
  *Verified by:* room/runtime accounting and prompt-delivery integration tests. Real-run checks of
  quota-filling or premature convergence are qualitative; no quality outcome is promised by R16.
- **A11** (planned; R10, R18–R19) — Hold the current speaker at an ordinary provider approval,
  then separately fail a turn. Check that the room names the reason, starts no next speaker or
  automatic retry, and records no voluntary departure. Resolve approval through normal agent
  controls; request pause while approval is outstanding and prove that the next speaker remains
  held after the active turn finishes. *Verified by:* fake-provider/room integration tests and a
  rendered intervention journey. Restart recovery awaits §6.
- **A12** (planned; R7, R12–R14, R20) — Use departures and, separately, exhausted turn allowances
  to leave one eligible participant. Check that ordinary discussion stops, that participant is
  offered a closing-message opportunity within its remaining allowance, its published message
  precedes any judge synthesis, and the room preserves the actual stop reason and objections.
  Exercise declining the closing message and ending when no participant retains allowance; neither
  grants an extra turn. A temporarily approval-blocked speaker does not trigger this ending.
  *Verified by:* room/runtime integration tests and a rendered closing-message journey.
- **A13** (planned; R14, R19, R21) — Fail a participant turn before a completed contribution and
  verify unchanged remaining allowance and no automatic retry. Explicitly retry, complete one
  contribution, and verify exactly one consumed turn and one canonical contribution. Reach the
  ceiling and refuse further participant turns, including a closing turn beyond that ceiling.
  *Verified by:* fake-provider/room accounting integration tests.
- **A14** (planned; R11, R22–R23) — Create a room with participants from two non-archived projects,
  refuse an archived project, and retain committed contributions after an agent and a project are
  removed. Reload and restart to verify the same ordered history and readable attribution, without
  fabricated live-agent links. Explicitly delete the room under the eventual deletion contract.
  *Verified by:* room/project lifecycle integration tests and a rendered retained-history journey;
  active-room archival and deletion behavior await §6.
- **A15** (planned; R24, R26) — In one room, two participants refer to the same relative file path
  in different workspaces. Open each file inside the room, inspect source/rendered content, select
  a file excerpt and diff lines, and annotate a room message. Verify correct participant/project
  attribution, path/line identity, captured excerpt, normal bounded tray behavior and preservation
  after a failed send. *Verified by:* room/file/annotation integration tests and a rendered room
  annotation/file-view journey; destination routing is additionally covered by A20.
- **A16** (planned; R2, R24–R25) — Read a room containing Markdown, code, a diagram, tools, a diff,
  an approval, and user messages. Exercise the corresponding familiar controls inside the room,
  then open a participant card and continue in that agent's normal conversation, preserving its
  identity/history. Finish the room and prove the remaining normal sessions are still available
  for follow-up. *Verified by:* room component/fake-provider integration tests and a real-browser
  journey in Core, Sky & Grove and Studio at the supported desktop floor and a wider viewport;
  private-send scheduling and activity visibility are additionally covered by A18–A19.
- **A17** (planned; R11, R22, R25, R27) — From a non-archived project, activate the Think Tank
  button beside New agent and open the separate room workspace. Mix new participants and existing
  participants from other non-archived projects. Verify that actual participant cards remain in
  their own projects, the room is not a provider-agent card, and clicking each room participant
  reaches its own normal card/conversation with preserved identity and history. *Verified by:*
  project/room navigation component tests and a rendered creation/follow-up journey, using the
  normal application's full conversation-page navigation.
- **A18** (planned; R4, R14, R21, R28–R29) — Send a private follow-up to an off-floor participant
  while another agent holds the room turn. Let the room advance to that still-busy participant:
  verify visible waiting with the same scheduled speaker, no automatic pause or skipped speaker,
  and no overlapping provider turn for that agent. Complete its private work and then its room
  contribution. Only the room contribution consumes allowance or appears as a room reply.
  *Verified by:* fake-provider/room scheduling integration tests and a rendered room/private-chat
  journey; steering an active room turn is additionally covered by A23.
- **A19** (planned; R24, R26, R29) — Seed participants with earlier and private messages, files,
  commands and tool events, then give each distinguishable room-turn activity. Inspect the room's
  conversation, Files/Commands and participant affordances: room activity is attributed and
  inspectable, private/earlier activity is not imported, and the normal individual conversation
  still exposes its own history. Equal relative paths from two room participants open in the
  correct workspace. *Verified by:* room projection/file integration tests and a rendered group
  view versus individual-history journey.
- **A20** (planned; R15, R26, R30) — From a room annotation tray, exercise Room, selected-agent
  and New task delivery. Verify shared Room input appears between participant contributions and
  is read on subsequent turns; selected-agent delivery reaches that normal session; New task opens
  the normal New Agent flow and starts a new agent whose initial work is the delivered annotations.
  Cancel creation or refuse delivery and preserve drafts. After room completion, Room delivery is
  unavailable while selected-agent/New task follow-up still works and records its source annotation.
  *Verified by:* room/annotation/launch integration tests and a rendered three-destination journey.
- **A21** (planned; R2, R13, R31) — Configure optional judge synthesis and complete participant
  discussion. Observe no judge launch during discussion; afterward a distinct fresh normal agent
  starts with the room context and its own fresh conversation, then appends the attributed synthesis.
  A room with synthesis disabled launches no judge. *Verified by:* fake-provider/room launch and
  context-delivery integration tests, plus A7's synthesis-quality check.
- **A22** (planned; R7, R25, R31–R32) — End discussion during an active room turn and verify that
  turn can finish, no later participant/closing turn starts, the actual stop reason and objections
  survive, and the fresh judge runs only if configured. Separately end while waiting for another
  agent's private work; completion does not cancel that work or delete participant sessions.
  *Verified by:* room/fake-provider state integration tests and a rendered early-ending journey.
- **A23** (planned; R14, R29, R33) — While a participant holds a room turn, open its normal
  conversation and observe the Think Tank turn identification. Use supported normal Steer and
  verify that the instruction enters that agent's private transcript, can affect its room
  contribution, and is not copied as a shared user entry. The contribution consumes its normal
  allowance; no additional turn or pause is fabricated. Preserve ordinary capability/refusal and
  turn-finished fallback checks. *Verified by:* room/agent composer fake-provider integration tests
  and a rendered room-turn steering journey.

## 6. Deviations & open decisions

Nothing is shipped. R1–R33 record the confirmed core, with retired R8 superseded by R12/R20.
The following proposed closure defaults are **unconfirmed**, not binding requirements. Overall
feature-scope confirmation is required before technical design.

- **Setup and scheduling:** at least two distinct non-archived chat agents from non-archived
  projects; existing busy agents can join and wait at their turn without preempting other assigned
  work. Turn-taking cycles through the configured participant order, with an individually set
  positive contribution limit and departure permission per participant. Departure permission defaults
  on; independent openings and judge synthesis default off. Membership and the goal remain fixed
  after start; newly created annotation follow-up agents do not automatically join the room.
- **Input:** submitted room user messages and Room annotations wait durably for a participant-turn
  boundary. During independent openings they wait until all openings have been published, so they
  do not change later participants' opening input. Shared input does not consume participant
  allowance. File/skill autocomplete requires explicit participant context rather than inventing a
  shared cwd or provider command list.
- **Judge:** configure normal judge launch settings during room setup; launch the fresh judge only
  at the end, in its chosen non-archived project, for one completed synthesis contribution. Judge
  failure is shown separately while participant discussion remains ended; ordinary launch-setting
  repair and explicit retry do not reopen discussion.
- **Recovery:** after server restart, retain committed discussion and hold unfinished rooms for
  explicit resume/recovery rather than replaying uncertain in-flight turns. Preserve withheld
  openings during a failure and wait for explicit retry. End during openings publishes completed
  openings as an explicitly partial set and records missing openings without inventing answers.
  Pending room input is retained at End and identified as not discussed when no participant received
  it. An archived/removed participant or project holds the room with a reason: restore/unarchive
  the same identity and retry where the normal lifecycle permits, or End discussion. Deleted
  identities are not silently replaced; a new discussion/follow-up can use retained history.
- **Access:** ordinary agent-facing room actions are membership-scoped. Recorded participants,
  including those who left, can retrieve room history while it exists; the fresh judge gets room
  access when launched. Agents retrieve/contribute/leave within their membership and turn authority;
  room creation,
  membership, configuration, manual ending and deletion remain user-controlled. Membership does not
  authorize private-transcript or unrelated project-data retrieval through room actions. The existing
  local API/same-machine trust boundary remains unchanged (TS-05.R3); this is not new process isolation.
- **Discovery and retention:** list rooms on their originating project and provide room entries
  in Archive, including rooms whose project was removed. The room retains committed messages and
  the room activity shown with them, including tools/diffs and source attribution; file viewing
  itself does not snapshot entire files. Annotation excerpts are retained point-in-time, and file
  links retain enough origin context for ordinary inspection when their files remain available.
- **Deletion and exclusions:** explicitly delete only a paused or ended room with no active room
  turn, using the normal confirmation flow. Delete room history without deleting normal agent
  histories or participants. Live goal/membership editing, autonomous agent room creation,
  configurable full-file snapshots, export, and new phone room UI are outside this change.

Technical design must still verify the existing seams before choosing protocols, persistence,
turn/cursor commit rules or provider-specific compatibility. No new externally visible protocol is
selected by this feature draft.

## 7. Traceability

- Origin: resumed **Think tanks** entry in `docs/ideas.md`; human scope revision 2026-10-05.
- Adjacent capabilities: FS-01 independent agent lifecycle; FS-03 ordinary provider chat and
  permission controls; FS-06 point-to-point mail; FS-11 opaque project resources; FS-13 annotations;
  FS-15 bounded context retrieval; FS-16 durable work coordination. Their existing contracts remain
  distinct.
- No product code, technical design, or ready change accompanies this draft.
