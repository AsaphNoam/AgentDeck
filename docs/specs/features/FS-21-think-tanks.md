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
  state. Reaching the budget ends further discussion turns; the exact budget unit and treatment of
  openings, exits, and the optional judge synthesis remain open in §6.
- **R6** (planned) — Independent openings are optional. When enabled, every participant forms
  its opening without access to the others' opening contributions; those contributions are
  published into the shared discussion before ordinary turn-taking begins. When disabled,
  collaboration begins immediately. This promises isolation from this room's openings, not
  erasure of a participant's pre-existing history or other knowledge.
- **R7** (planned) — Completion reports why the discussion stopped and preserves material
  disagreement. Agreement itself is not the objective, and a budget ending or participants leaving
  must not be presented as proof that the goal was achieved.
- **R8** (planned) — A participant can be configured as allowed to leave the discussion. Leaving
  records that decision in the shared history and removes the participant from future discussion
  turns; it is distinct from stopping or deleting its normal agent session. After all but one
  participant have left, the discussion ends. No dedicated judge agent is required to decide when
  discussion stops.
- **R9** (planned) — A person inspecting the room can see its goal, participants, ordered
  attributed discussion, current phase and speaker when applicable, and completion reason.
- **R11** (planned) — Participant selection supports both creating a new normal agent and adding
  an existing normal agent. A single room can mix new and existing agents; existing participants
  retain their own session history under R2. Project and scheduling eligibility remain open in §6.
- **R12** (planned) — Permission to leave is configurable per participant. An allowed participant
  may choose to publish a final attributed message with its departure, or leave without a final
  message. The departure is recorded either way, and an optional final message is part of the
  canonical discussion before the remaining-participant completion rule is evaluated.
- **R13** (planned) — Final synthesis is optional. When requested, a judge agent is called only
  after the participant discussion has ended, reads the completed shared discussion, and produces
  the synthesis as the last step. Its synthesis is attributed to the judge and retained with the
  room under R1 and preserves unresolved material objections under R7. The judge does not take
  discussion turns or receive activations to monitor progress or decide whether discussion stops.

## 3. States & transitions

The core phase distinction from R4–R6 and R13 is optional independent openings → shared discussion
→ discussion ended → optional judge synthesis. R8 can end discussion when only one participant
remains. The discussion completion reason remains distinct from the state of the optional final
step. Busy, approval, interruption, pause/resume, synthesis failure, and failed-opening transitions
await the decisions in §6; this draft does not specify automatic advancement or recovery for those
cases.

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
- **A4** (planned; R5, R7–R9) — End a discussion at its configured budget with a material objection
  still present. Separately let permitted participants leave until only one remains, and attempt
  departure from a participant without permission. Check that the room shows the actual stop reason,
  preserves objections, refuses unauthorized departure, and leaves normal agent identities intact.
  *Verified by:* room state/action tests and a rendered completion journey after implementation.
- **A5** (planned; R1, R10) — Restart Chuck after committed contributions and prove the same
  ordered, attributed discussion survives. A provider failure must not produce a false contribution,
  departure, or success state. *Verified by:* persistence/recovery integration tests; scheduling and
  retry assertions await the recovery contract in §6.
- **A6** (planned; R2, R8, R11–R12) — Create a room with one new agent and two existing agents,
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

## 6. Deviations & open decisions

Nothing is shipped. Product confirmation is required before technical design.

- Project scope and minimum participant count; eligibility of busy agents, agents assigned other
  work, and terminal agents. New/existing participant selection is settled by R11.
- Departure permission defaults. Departure messages are optional under R12; whether explicit
  goal-achievement judgments require a separate ending mechanism beyond permitted departures.
- Judge selection/configuration, final-step budget accounting, and failure/retry behavior. Optional
  synthesis by an end-only judge is settled by R13; there is no live monitoring judge.
- Budget unit, turn order, and whether independent openings and departures consume that budget.
- Operator controls and input: start, pause, stop, resume, and whether people can add room messages
  or change membership/goal during discussion. These are not implied by ordinary agent chat controls.
- Waiting and recovery: busy speaker, ordinary chat/mail competing for its session, pending approval,
  failure before contribution, restart during a turn, and incomplete independent openings.
- Storage/access boundary, retention after completion and agent/project deletion, and any explicit
  room deletion/export. Existing mail expiry and opaque project-resource rules do not define room
  retention or authorize room disclosure.
- Expected surface: project entry point, room detail/history, and agent/API creation/inspection
  authority. No new externally visible protocol is selected by this draft.

## 7. Traceability

- Origin: resumed **Think tanks** entry in `docs/ideas.md`; human scope revision 2026-10-05.
- Adjacent capabilities: FS-01 independent agent lifecycle; FS-03 ordinary provider chat and
  permission controls; FS-06 point-to-point mail; FS-11 opaque project resources; FS-15 bounded
  context retrieval; FS-16 durable work coordination. Their existing contracts remain distinct.
- No product code, technical design, or ready change accompanies this draft.
