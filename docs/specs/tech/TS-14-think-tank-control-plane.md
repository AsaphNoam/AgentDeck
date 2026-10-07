# TS-14 — Think Tank control plane

**Status:** Partial
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
  resume for stopped agents. A stopped speaker whose resume fails before its attempt commits holds
  the room with a reason rather than being retried by each sweep; a lost resume race stays
  transient, and a room that changed during the resume leaves the resumed agent running idle for
  the next selection. Persist setup intent/reserved identities before launch effects.
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

- **R19 (planned)** — Pipeline room creation reuses R2's normalized setup/reservation and ordinary
  launch composition with every member/judge new and project fixed to the run. A shared transactional
  create helper accepts trusted pipeline origin plus bounded immutable stage context; standalone
  create cannot set that origin. Persist goal/objective, declared inputs/output contract and run/stage
  attribution as room-owned context before launch, so run deletion does not erase what participants
  received. The ordinary bounded room goal names the stage; the full run goal/objective appear in
  this stage context and the room's visible goal area, rather than being clipped into R17's goal
  field. Extend the existing paged room-context read with an attributed `stage_context` section
  using R7/R17's byte limits, UTF-8 continuation and read receipts; every participant and judge reads
  it before submitting. Keep stage data outside frozen provider/system prompts. Origin provides
  provenance and recovery linkage, not private-transcript or cross-project authority.
- **R20 (planned)** — A pipeline origin adds the run/stage-open check to setup claims, room
  activation admission, resume/retry and judge reservation, atomically with the relevant state claim.
  Shared state methods behind every existing room mutation (including End, messages and annotations)
  enforce the relevant origin/closure guard; stale actions return the existing typed conflict without
  changing the run. After stage completion, room messages remain closed, while ordinary retained
  inspection and selected-agent/New-task annotation follow-up remain available outside run ownership.
  No room HTTP End/Resume/Retry can reopen a stopped pipeline. Pipeline Stop is a distinct durable
  closure request from normal End: suppress future work, retain committed history and queued input,
  abandon unstarted slots/judge, cancel only the captured room-owned turn, then release its ownership.
  Unpublished cancelled contributions are not invented or charged; retain failed activity honestly.
  TS-09.R54 owns convergence before run stop. R13's standalone End/completion behavior is preserved.
- **R21 (planned)** — A successful finalized judge contribution remains canonical room data;
  TS-09.R53 consumes its immutable entry through the task result authority after commit. Stage output
  acceptance is independently idempotent and recoverable if a post-commit kick fails; it does not
  rerun the judge. Required synthesis output limits are returned in pipeline judge context and
  enforced before staging. Failure/retry preserves discussion and previous judge attempts, and
  pipeline pin checks join guarded room deletion in its transaction. Read/hydration exposes pipeline
  origin and current recovery state; all room projections retain existing privacy/activity scope.
  No new MCP action, provider feature, direct-action transport or remote room route is needed.

- **R22** — Persist a normalized room `title` (1–120 Unicode runes) separately from
  goal and include it in normalized create intent/replay matching. UI creation requires it;
  absent/blank legacy API input and existing rows use the first 120 runes of the whitespace-folded
  goal, with no goal/history rewrite. A forward migration adds title and backfills only that field.
  There is no rename endpoint. New participant and fresh-judge launch intent sets ordinary
  `launchRequest.Group` to the title before reservation/launch, using existing `launchAgent`
  composition. Ready/existing slots are never regrouped; judge repair/retry applies the same group
  only when creating a fresh identity. Existing project/group/order/lifecycle mechanisms remain.
- **R23 (planned)** — FS-21.R48 narrowly supersedes R2/R8's one-attempt constraint during
  independent openings. Replace the running-room uniqueness with one running non-opening attempt
  per room plus one running room attempt per agent across rooms. Admit each opening independently
  in the existing `before(turnID)` transaction using phase/control, fixed-member eligibility,
  absent running/withheld/completed opening, task-exclusion and R20's pipeline closure guards.
  Opening starts, resume and retry share those guards and cross-room agent exclusion. Sibling completion,
  queued-input or ceiling revisions must not falsely veto another eligible opening; strict
  selected-opportunity/revision admission remains for non-opening work. No opening epoch or
  second execution service is needed: fixed membership and irreversible phase progression make
  the member/attempt identities sufficient. Freeze each attempt's own head/token/source tuple;
  shared inputs remain held until opening publication. State guards also prevent opening/non-opening
  coexistence, even though distinct partial indexes alone do not prove that exclusion.
  The existing bounded per-room dispatcher starts eligible opening activations/resumes concurrently
  through the ordinary lifecycle/start leases, with at most 32 in-flight opening starts per room
  and 128 process-wide across the existing four room workers. Keep the per-room dispatch claim
  across those bounded starts, not across provider completion. A busy member stays unattempted
  without blocking eligible peers. Holds/Pause/End suppress later admission; running siblings settle
  independently and are not cancelled by another opening's failure. Barrier/partial-End settlement
  waits until no opening attempt is running, publishes withheld entries/activity in member order
  once, and retains failed/completed peers for explicit recovery. Startup/exit/delete/capture guards
  enumerate all active openings; generation/turn matching and per-attempt byte budgets stay intact.
  An admission committed before Pause/End may settle; an admission losing that transaction race
  starts no provider frame and stays unattempted on Pause or is abandoned by End. Retry never
  bypasses a later pipeline Stop. Failed members are not readmitted by a sweep or ordinary Resume:
  user-controlled explicit turn retry authorizes their next attempt. Extend `{target: "turn"}`
  retry with `attempt_id` and a stable `command_id` for opening retries, validating that failed
  attempt's room/member/opening identity. Exact replay returns its acknowledgement; conflicting
  command reuse or ambiguous multi-failure legacy retry refuses. A paused retry authorizes recovery
  without starting until Resume; preserve other unresolved failure holds. Partial-End/closing/ended
  retry refuses, preserving history. Unattempted eligible slots may proceed after explicit recovery,
  never completed/withheld peers. Existing unambiguous legacy retry shapes remain accepted.
- **R24** — Add an atomic state mutation for an absolute higher participant cap,
  exposed as `POST /api/think-tanks/{id}/participants/{agent_id}/turn-limit` with
  `{command_id, expected_limit, limit}`. Require 1–1,000, `limit > expected_limit`, a participant,
  phase openings/discussion, no End/closing/judge/run-closure authority and the applicable R20 guard.
  Compare `expected_limit` to the current cap inside the same transaction; concurrent changes return
  typed conflict without overwriting. Store command intent/result keyed by room/command so exact
  replay returns its acknowledgement and conflicting reuse refuses. Keep this receipt separate
  from shared message inputs: a budget change is not a conversation contribution. Receipts are
  room-owned and removed on room deletion; successful increases are bounded by the finite caps.
  Update cap and
  exhausted→active eligibility atomically, preserving departed state, completed/checkpoint counters,
  control/holds and active attempts; increment room revision, commit, then emit the normal update.
  A saved change is acknowledged before dispatch; it never uses Send/Steer. Context reads and later
  activation read authoritative cap, while already-delivered instructions remain historical facts.
- **R25 (planned)** — Extend shared-message input with optional bounded structured mention ranges
  `{agent_id, start, end}` over UTF-8 bytes of the submitted body. UI picker selection retains the
  id/range, updates unaffected ranges and invalidates edited mentions; labels are presentation,
  never identity. The server validates at most 32 nonoverlapping, rune-boundary ranges and recorded
  live participant targets (not judge/departed/deleted) within the existing request/text limits,
  then snapshots addressee id/name/project in the input/entry context. Exact command replay includes
  mention intent; invalid/stale targets refuse without discarding drafts. Existing messages without
  mentions retain their meaning. Plain body text/file tokens are never parsed into target authority.
  Published room/agent-read entry projections expose the attributed addressees, and context guidance
  explicitly says when the caller is addressed. Reading requires the same full-view receipt; no
  private prompt, extra activation, membership grant, speaker reorder or new MCP action is added.
  All participants can read the same shared body/targets, subject to existing opening isolation.
- **R26 (planned)** — In the successful judge-finalization transaction, insert an immutable
  agent-owned synthesis read projection with exact body, room/title/entry/attempt attribution,
  judge id, completion time and source generation/runtime turn id/completion-event seq. Key it
  uniquely by room/entry; pass the actual completion event seq from `finishThinkTankTurn`. Its
  foreign key follows ordinary agent-history deletion, never room deletion. Canonical room entry
  and this result snapshot commit atomically; failed/uncertain/staged attempts create neither result
  nor successful receipt. Duplicate finalization cannot duplicate the snapshot. This bounded
  read projection satisfies independent agent-history retention without a second provider writer,
  invented assistant event, cross-store outbox or new runtime event kind.
  Add local-only `GET /api/sessions/{id}/think-tank-results`, ordered/cursor-paged with at most
  500 records/1 MiB encoded per response and the existing 64 KiB per-result body bound. The ordinary
  desktop transcript/archive renderer merges a source-attributed synthesis row at its captured
  completion anchor; provider NDJSON/context remains unchanged. Result identity is distinct from
  provider seq and room anchors; copy/Markdown and room-source inspection retain their existing
  source availability rather than inventing a provider transcript anchor.
  An anchor outside the loaded window waits for that window; an unavailable source anchor still
  offers the retained result with truthful source-unavailable framing. After commit, room updates
  invalidate/refetch the judge result query, including live clients arriving after completion.
  Room-link metadata marks a deleted room unavailable without deleting the result. Do not blend
  the result into an incidental provider assistant delta or change provider history/search records.
- **R27 (planned)** — Extend existing version-1 room create/list/detail/entry/read shapes with
  optional-compatible title, stable member summaries/allowances, shared addressee snapshots and an
  explicit `active_attempts` array. Preserve old fields; singular `active`/`active_agent_id`/
  `current_actor` describe a sole active attempt only and are absent/empty for multiple openings.
  All REST/SSE/pipeline projections clear stale singular actors on that transition; new consumers
  prefer the explicit array when present rather than carrying forward an earlier singular speaker.
  Lists/summaries carry the full bounded roster and judge presence/status needed for cards
  without per-card detail fetches or launch-setting disclosure. Remaining allowance derives from
  cap/completed and departure state; totals exclude departed/judge members and never subtract
  an unfinished attempt as a completed contribution. Add an indexed `agent_id` list filter for
  participant/judge membership, independent of current busy state, retaining the existing 200-room
  bound and explicit clipping feedback. Existing records/callers remain readable; new arrays marshal
  as `[]`. SSE stays invalidation plus bounded owned activity: no hidden opening data in summaries.
  Register the new REST surfaces under `localOnly`, outside the phone allowlist, reuse structured
  errors and keep membership-scoped MCP access unchanged. Update producer-derived Go↔UI fixtures,
  Zod consumers, shared room projectors and embedded operating knowledge together.
- **R28 (planned)** — Closure extends R18's focused matrix with FS-21.A33–A37 and FS-02.A53/
  FS-03.A52–A53: independent overlapping provider frames, per-agent admission races, opening
  isolation/barrier/failure/Pause/End/restart, increase/replay/end races, mention range/identity and
  full-context delivery, synthesis/result atomicity/reload/archive/deletion and group launch/retry.
  Use state/server/runtime race tests in both Go variants, serialized wire fixtures and affected
  UI tests/style/build checks, then TS-06's applicable closure once. Render the real built UI at
  1024px and wider in all three appearances, including a side-by-side normal/room composer check.
  Existing credentialed Claude/Codex room-tool/approval/resume gates remain distinct from fake
  evidence; extend the finite probe to addressed input and the readable exact judge result.

## 3. Interfaces & data shapes

R22–R28 extend the initial interface inventory additively; the existing v1 fields and actions remain.

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
  attempt record. A dispatcher (`server/think_tanks.go`) progresses each room on its own goroutine
  on a 5s sweep plus kicks, at most four rooms at once, with a per-room claim keeping each room
  serial; launches and resumes in one room do not delay another. The per-attempt activity budget is
  a running UTF-8 byte total on the attempt row. The judge's read omits participant ceiling fields. Executing turn ownership is `runtime.Event.TurnID`, cleared after the terminal
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
