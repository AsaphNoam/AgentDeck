# TS-14 — Think Tank control plane

**Status:** Current
**Code:** `internal/state/`, `internal/server/`, `internal/runtime/`, `internal/messaging/`, `ui/src/features/`
**Absorbed:** —

## 1. Scope

FS-21's approved independent-session deliberation over a SQLite-owned append-only room artifact.
The selected boundary is explicit agent publication through the existing MCP gateway, ordinary
host activations, and REST/SSE for the UI. Shipped 2026-10-06; §5 lists the deviations.

## 2. Design & constraints

- **R1** — One server-owned Think Tank service progresses rooms over existing lifecycle
  and runtime services. `internal/state` is the sole SQLite writer of rooms, membership, attempts,
  published entries, queued input and retained activity. No synthetic provider agent, separate
  process, repository artifact, agent-written canonical file or expiring-mail conversation is added.
- **R2** — Each room selects one opportunity and owns at most one active attempt,
  including closing/judge work. Participants keep ordinary identities and provider histories.
  Use `server.launchAgent` with its existing `launchOptions.AgentID/Generation` reservations and
  full composition/registration/rollback for new participants and the fresh judge; use ordinary
  resume for stopped agents. Persist setup intent/reserved identities before launch effects.
  Partial setup retains created normal agents and exposes the failed slot; explicit retry does not
  silently duplicate them. Discussion starts only after setup is complete.
  Each pending setup slot claims durable `launching` state before ordinary launch; Pause/End
  prevent later claims, End abandons unstarted slots, and deletion refuses an in-flight claim.
- **R3** — Add `think_tank` to the closed activation registry and bounded executor.
  Its fixed instruction names the room tools; goal, peer messages, role instructions and allowance
  are pulled as data, not persisted in role/session prompts. Use `Registry.StartActivation` under
  the shared server lifecycle claim. Its `before(turnID)` callback atomically verifies room revision,
  project/agent eligibility and absence of conflicting task execution, then commits the attempt,
  concrete generation/turn id, context head and ordinary messaging-budget reset before the provider
  frame. Pending activation is keyed by its room attempt, not unread status. Busy/private/assigned
  work holds the same speaker; room dispatch never uses human `SendPromptOrHold`.
  Running-agent admission holds the existing agent/project archive start lease through the
  provider frame and rechecks live project and agent eligibility in `before`. Room attempts and
  task reservations mutually exclude each other in SQLite; missing/unreadable projects hold.
- **R4** — Extend common runtime emission with the actual executing turn id, separate
  from the reserved-successor counter. Pass the original id through terminal emission before
  starting held/Steer successors. Source generation/turn ownership travels internally with normalized
  events and live notices and is saved in room snapshots. Child activity inherits its original
  ownership when announced. Never infer source from current counters, registry generation or time.
  Unowned legacy/terminal activity is not guessed into a room. No ACP wire extension is required.
- **R5** — Each immutable attempt owns a random turn token, participant/role, generation,
  runtime turn id, phase and frozen context head. `submit_think_tank_turn` stages one terminal intent:
  `reply`, authorized `leave` with optional message, or `decline_closing`. Judge reply kind is
  server-derived. Reply requires nonblank text; leave accepts omitted text and decline accepts none.
  Exact replay returns the same staged receipt; conflicting, foreign, ended or stale
  submissions do not mutate state. A staged receipt does not promise publication or charge allowance.
  A completed authorized departure charges one turn even without a message; declining the sole
  closing opportunity charges none and does not advance the member's read checkpoint.
- **R6** — Matching successful provider completion finalizes that intent through the
  common event/completion seam. Match attempt, token/activation ownership, source generation and
  captured executing runtime turn id, never the mutable current registry state. One state
  transaction seals ownership, publishes the contribution
  or departure, updates completed allowance and actual read checkpoint, applies queued input and
  pause/end/rotation state, and retires its activation. Seal before a private successor can write as
  the room actor. Missing submission, cancellation, error, truncation, lost completion or capture
  failure holds for explicit intervention, without charging or automatically retrying. Assistant
  output is never silently substituted for an absent staged reply.
- **R7** — Read checkpoints are per-member rows. Turn admission freezes the published
  conversation head. Bounded ordered reads record contiguous delivered progress for that attempt,
  not the committed checkpoint; submission requires delivery of its full new conversation view.
  Successful finalization alone advances to that watermark, never latest seq or its own reply seq.
  Default delta reads suppress already authored own content while scanning continuation, avoiding
  duplicate input without skipping interleaved annotations. Older/detail reads remain explicit.
  A delivery checkpoint does not prove model memory; identify known native context rebuilds and
  permit paged reconstruction without rewriting private history.
  Continuation binds caller and attempt identity as well as room/view/head; invalid entry
  positions, out-of-range offsets and offsets inside a UTF-8 rune return the typed cursor refusal.
- **R8** — Independent openings use the same single-floor seam but withhold completed
  answer/activity in durable staging. Successful opening completion charges one turn without
  offering other opening bodies. One transaction publishes all completed openings in configured
  order at the barrier, followed by queued user input. Manual End publishes completed openings as
  a partial set, records missing ones and undiscussed input, then ends participation. Failed openings
  do not replay completed peers. Ordinary same-user/provider context isolation is not enlarged.
- **R9** — Room messages and Room annotations are durable ordered boundary inputs.
  Publish source annotation before selected-agent/new-agent delivery. Reuse transactional ordinary
  mail insertion and the existing normal launch/delivery seam for New task. Room/entry/file anchors
  are distinct from provider transcript seq. Stable command/receipt identities distinguish a retry
  from a new send; cancellation/failure preserves drafts and cannot silently duplicate delivery.
  Selected-agent annotation history, ordinary mail and its pending activation commit in one
  SQLite transaction; exact command replay creates no second delivery.
- **R10** — Copy matching normalized room-turn activity into room-owned storage as it
  arrives, including source seq, generation/turn/child scope and frozen actor/project/cwd provenance.
  Private user/Steer prompts and unrelated turns are excluded. Tool/diff/result/permission payloads
  survive source deletion; failed attempts retain identified activity without a false contribution.
  Existing reasoning notices stay live-only. Registered internal Chuck coordination/context tools
  use one shared public activity projector with compact action/outcome receipts; raw arguments and
  results containing tokens, read receipts, repeated peer bodies, private mail/context/task payloads,
  registry credentials or private launch configuration are not copied into room rows, REST or SSE.
  This preserves ordinary private provider transcripts and exposes each room action's occurrence
  and outcome without importing incidental private data. Storage failure prevents final publication. Late
  child activity keeps origin ownership and cannot create a turn or charge allowance.
- **R11** — Agent room actions use token-bound MCP identity, the authoritative tool
  registry, shared result/error classification and registry-derived approval exemptions. Caller,
  speaker and role are server-derived; a room id/turn token cannot grant nonmember access. No agent
  room action creates, edits membership, manually ends or deletes rooms. Registration and revocation
  stay on generation-scoped teardown. REST stays under `localOnly` and TS-05.R3; room routes do not
  enter the phone allowlist. Shared content is data, not provider/system instruction authority.
- **R12** — Room history has no cascading agent/project foreign keys. Explicit room
  deletion removes only room-owned data. Capture readable identity and source cwd/path context before
  dispatch; deleted sources become intelligible tombstones, never same-name replacements. A room
  file route reuses the local bounded regular UTF-8 reader and absolute/relative semantics against
  retained source context, including typed failures and file annotation capture. Viewing does not
  archive unselected file contents.
  Source references retain their attempt-to-workspace association; direct contribution and diff
  links resolve those references independently of opening Files, including after agent deletion.
- **R13** — Pause/End are durable requests, settled after active room work; End suppresses
  future participant/closing work and does not cancel private activity. Discussion-ended and judge
  state are independent. Launch the configured fresh judge only after final discussion/input
  publication, for one completed synthesis. Failure supports explicit configuration repair/retry
  without reopening discussion. Room completion/deletion stops neither participants nor judge.
- **R14** — Startup holds unfinished rooms and fences uncertain attempts. Read, idle,
  sweep, startup and duplicate callbacks never replay an attempted provider effect. Explicit retry
  creates a new attempt/token; restored eligibility plus explicit resume can admit unattempted
  work. Old generations/turn ids/tokens cannot finish a later attempt. Delete only paused/ended
  rooms with no active attempt; atomically revoke pending room activations/work, including an
  unstarted judge. Stale reads fail and delayed producers cannot launch or recreate deleted rooms.
  Generation-scoped agent exit fails and releases its running room attempts and capture, including
  requested Stop paths without a terminal event; no contribution allowance is charged.
- **R15** — UI uses a full room route, project creation/list entry and distinct Archive
  room entries without changing legacy agent Archive payloads. Reuse scoped `foldTranscript`/
  `appendRenderedEvent`, `groupTranscriptRows`/`ToolRun`, content renderers and annotation helpers;
  equal tool ids from different actors/attempts never merge. Render contributions as attributed
  room items and activity in attempt-scoped disclosures. Bind permission/child actions to their
  original source under an atomic generation/turn guard; stale activity is read-only. Participant
  links reach ordinary cards/conversations; normal agent views identify room work. Use existing
  presentation contracts and all three appearances, without a new renderer or design framework.
  Read-only settled activity keeps disclosure, copy, file viewing and annotation controls usable;
  only stale source mutations are refused.
- **R16** — After durable commit, emit versioned `think_tank_update` summaries.
  `think_tank_activity` carries bounded owned live activity/notices. Hydration/reconnect uses atomic
  snapshot/subscription plus bounded REST refill; revision/entry gaps refetch rather than guess.
  Browser windows/drafts are bounded and reset on room change/deletion. SSE is notification, not
  authoritative history, and blind openings are not offered through either room stream before publication.
  Entry and activity windows retain the newest 5,000 records with explicit clipping notices.
  Files/Commands inspect the newest 10,000 visible activity records and report clipping; retained
  earlier records remain accessible through sequence-based REST windows.
- **R17** — Initial bounds: requests 256 KiB; submitted reply/input UTF-8 text 64 KiB;
  nonblank goal 8,000 runes; participants 2–32 and individual limits 1–1,000; agent conversation
  pages 32 KiB entry/activity text with continuation and separately bounded goal/metadata;
  REST activity windows 500 records/1 MiB; executor batches 32.
  Capture caps are 8 MiB per normalized record (the existing transcript-record ceiling) and 64 MiB
  per attempt. Preserve identity/anchors
  and explicit truncation markers for bounded display payloads. REST byte limits include encoded
  record metadata; an oversized first record becomes an explicit bounded display marker with the
  same sequence/attempt anchor so pagination progresses. Never silently truncate a submitted
  contribution. Enforce before allocation/expansion, return typed refusals and preserve drafts.
  These bound work/memory and completed contributions, not spend across manual retries.
- **R18** — §3 is the closed initial interface inventory. Update shared result/approval
  contracts, producer-derived action lists, independent Go↔UI fixtures and progressive embedded
  `operating-chuck` knowledge together. Turn-specific room data stays outside frozen launch config.
  Implementation closure follows TS-06.R5 and §4's focused matrix; fake peers cannot satisfy
  credentialed provider acceptance.

## 3. Interfaces & data shapes

R18's initial REST family is `/api/think-tanks`: `POST` create/setup, bounded `GET` list, `GET /{id}`
detail, and `GET /{id}/{entries,activity,files,commands}`. `GET /{id}/sources/{source_id}/file` resolves a
retained room source. `POST /{id}/{messages,annotations,pause,resume,retry,end}` performs the named
human action; `DELETE /{id}` is guarded room deletion. Mutations carry stable command identity and
expected revision where state-dependent; exact replay returns the original acknowledgement.
Responses use versioned room types and standard structured errors; all collections are arrays.
Creation stores immutable normalized request intent separately from repairable judge settings;
replay reuses original reserved participant identities before any new identity is allocated.
Source ids are opaque immutable room-owned references, distinct from room/provider sequences.
Retry identifies the failed setup slot or participant/judge attempt; only judge launch-setting repair
is accepted after discussion starts. No retry edits goal/membership or replays a completed effect.

```text
MCP read_think_tank(room_id?, cursor?, view?)
  omitted id resolves caller's current attempt; explicit id requires recorded membership
  view: context (default) | history | activity; opaque cursor bound to caller/room/view/head
  returns goal/phase/role/ceiling, attributed conversation/activity page, continuation,
  read receipt, and turn token only for caller's current attempt
MCP submit_think_tank_turn(turn_token, disposition, message?, read_receipt)
  disposition: reply | leave | decline_closing
  returns staged receipt; room/actor/role derive from server state; publication follows R6
SSE think_tank_update: version, room_id, revision, phase/control, current_actor?, reason?
SSE think_tank_activity: room_id, attempt_id, actor/source tuple, bounded event/live notice
```

R1/R5–R9/R12 imply these state-owned records:

| Record | Durable fields |
|---|---|
| Room | id, goal, origin snapshot, phase/control/revision/rotation, pause/end reason, judge config/status |
| Member | room/id/order, actor/project snapshot, cap/completed, departure setting/state, committed read/context checkpoint |
| Attempt | id/token/role/actor, generation/runtime turn id, frozen read head/delivered progress, state, intent/receipt, final entry |
| Entry | room seq/id/kind, attributed source, immutable published content, attempt link, timestamp |
| Activity | room/attempt/source id and seq, generation/turn/child scope, normalized bounded payload, path/cwd, truncation markers |
| Input | command/id/order, user/annotation content and context, pending/publication identity, discussion coverage |

Published sequence belongs to the room; source sequence belongs to the provider session. Snapshot
identity is logical provenance. Attempt staging/capture is operational data until publication.
The generic activation points to its room attempt; it does not contain goal or contribution payload.
Context pages start at the committed checkpoint and end at the attempt's frozen head; history can
explicitly revisit older published entries. Oversized entries continue at UTF-8-safe offsets with
stable entry anchors. Activity-only/detail reads do not advance conversation delivery. Goal/full
metadata accompany the initial page; continuation avoids repeating already delivered bodies.

## 4. Invariants

Governing classes: INV §1–§11 and §13–§17. No new runtime/driver or external CLI invocation
is introduced. Any shared runtime guard/interface extension still joins every existing implementation.

R18's focused matrix independently observes provider frames, wire payloads and durable rows:
duplicate/stale publication, busy/private/task races, held/Steer-successor attribution, blind opening
barriers, UTF-8 pagination/read receipts, unseen input/checkpoints, store failure before publication,
failure after staging, late generation/child events, restart fencing, judge failure, source/room
deletion (including refusal during closing/judge attempts), public receipt redaction in rows/REST/SSE
and empty arrays. Add focused runtime/state/control-plane race tests to both Go variants
and affected UI tests/build/styles at final closure. Render creation → discussion → annotation/
private follow-up → End/judge → retained Archive at 1024px and a wider desktop in Core, Sky & Grove
and Studio. Credentialed Claude/Codex checks cover new room tools, approvals and native resume;
they remain explicit implementation gates, not design-time or fake-ACP claims.

## 5. Deviations & open decisions

- The human selected SQLite authority and explicit room tools after reviewing canonical-file and
  automatic-assistant-publication alternatives. No material technical choice remains open here.
- Current normalized assistant output is a delta without publication intent. Explicit submission
  makes intended contribution/leave/closing decisions unambiguous, at the cost of a tool-call
  obligation; instructions and missing-submission recovery are specified rather than inferred.
- No provider bump, ACP multi-party identity, MCP resource/template adoption, direct-action
  migration, background judge, new sandbox or broad coordination framework is required. The
  paused FS-17.R20 direct-transport gate stays closed. Normal private provider context remains normal.
- Shipped shape (2026-10-06). The room's durable opportunity is its own state: the `think_tank`
  activation kind writes no `activations` row; the room attempt committed in `before(turnID)` is the
  attempt record. One server worker (`server/think_tanks.go`) serializes progression on a 5s sweep
  plus kicks. Executing turn ownership is `runtime.Event.TurnID`, cleared after the terminal
  emission. Admission retains a hidden `session_meta` workspace-source marker so contribution
  file links work even without tool activity. Capture copies tool, diff, permission, error and child-activity records; assistant
  prose, prompts and reasoning are not copied. Child scopes freeze their original generation/turn;
  late child records keep that identity and are refused once its capture has settled, including
  while a later room turn is active. Room-source annotations publish an
  attributed text batch whose anchors are server-resolved (`/annotations` with target room or
  agent); selected-agent delivery commits annotation mail and the batch atomically. Pause/resume/end are
  idempotent and carry no expected revision; create, messages and annotations carry command ids.
  Retry targets are `setup`, `turn` and `judge`; a judge retry always launches a fresh judge.
  Settled attempt activity retains inspection and annotation controls; unresolved retained approvals
  render cancelled and cannot send a stale decision. Live permission actions use the ordinary
  per-agent decision endpoint, which already refuses a settled tool call.
- R18's credentialed Claude/Codex checks and the real-binary rendered journey remain owed.

## 6. Traceability

- Feature authority: FS-21.R1–R40/A1–A30; FS-02.R65/A47; FS-03.R69–R70/A50–A51;
  FS-05.R39/A22; FS-13.R26–R27/A17–A18; FS-17.R21/A12; FS-18.R19/A15.
  Shared extensions: TS-01.R37, TS-02.R42, TS-03.R55, TS-04.R84, TS-05.R25, TS-06.R33,
  TS-08.R87 and TS-11.R19. Existing task/pipeline semantics stay in TS-09/TS-10.
- Verified lifecycle/activation: `server/launch.go` (`launchAgent`, `launchOptions`),
  `runtime/{runtime,activation_kinds,chat}.go` (`StartActivation`, `settleTurnContext`,
  `finishTurn`, `emitIn`), `server/messaging_loops.go`, `server/task_dispatcher.go` (`dispatchTurnEnd`).
- Verified data/action: `state/activations.go`, `messaging/messaging.go` (`addTool`, `ToolNames`),
  registry-derived approvals in `server/launch.go`, `server/fileread.go`, `runtime/activity.go`,
  `contextref/limits.go`, `server/transcript_window.go`, `server/archive.go`.
- UI: `ui/src/store/transcriptStore.ts`, `components/chat/{TranscriptView,toolRun}.tsx`,
  `features/launch/NewAgentModal.tsx`, `features/dashboard/ProjectDashboard.tsx`, `routes.tsx`.
