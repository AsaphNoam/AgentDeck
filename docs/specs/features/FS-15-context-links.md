# FS-15 — Pull-based context links

**Status:** Current
**Code:** `internal/contextref`, `internal/state`, `internal/messaging` · **Journeys:** —
**Absorbed:** —

## 1. Purpose

Agents can expose durable work to one another without copying it into mail or eagerly injecting it
into another model's conversation. AgentDeck gives an immutable source selection a stable,
target-neutral context reference, grants agents permission to retrieve that reference, and returns
its content only through an explicit bounded read. Direct sharing is an ad-hoc convenience; future
work objects may attach the same reference and expose it through their own assignment API without
turning context discovery into a global inbox scan.

## 2. Behavior

Requirements are user- and agent/API-observable. R-item numbering is continuous through §4.

### 2.1 References and sources

- **R1 — A reference identifies a source, not a recipient.** A context reference has one
  stable opaque id and an immutable typed source locator. Re-creating the same locator returns the
  same reference id; sharing it with another agent or attaching it to another piece of work does not
  duplicate the reference. The reference records intrinsic provenance but contains no target,
  presentation title, description, read state, work id, prompt, or copied source content.
- **R2 — The initial sources are immutable AgentDeck records.** The accepted source kinds
  are (a) an exact inclusive sequence range in one agent's append-only normalized transcript and
  (b) the accepted immutable report of one pipeline attempt. A transcript source is resolved to
  concrete `agent_id`, first-sequence, and last-sequence values before its reference is returned; a
  pipeline source is resolved to one concrete attempt id whose report has already been accepted.
- **R3 — Presentation belongs to the access path.** A direct grant or future work
  attachment may give the same reference its own bounded label and description. Those fields may
  differ between grants and attachments and are never part of reference identity. Reading the
  source by reference id is independent of which label led the caller to it.

### 2.2 Direct sharing and discovery

- **R4 — Agents can share only context they currently own.** A token-bound chat agent may
  share the transcript content accumulated in its current turn so far, its latest completed
  transcript turn, or the accepted report of its current pipeline attempt, with one resolvable chat
  agent. AgentDeck derives the caller and source identity from the live MCP session and resolves the
  friendly source selector to the exact R2 locator; the caller cannot name another agent's
  transcript or an unrelated pipeline attempt. Sharing returns both the canonical reference id and
  the direct-grant id. `current_turn` is an immutable snapshot through the share call: the intended
  handoff sequence is to write the reasoning-relevant conclusion first, call `share_context` as the
  last context-producing action, and then optionally send ordinary mail containing only the returned
  reference id. Later assistant text is deliberately outside that reference, and so is the share
  call itself: a span never ends on a tool invocation, so the boundary is the caller's last settled
  content rather than whatever its backend happened to have written down by the time the tool ran.
  No recipient, label, description, or selector can reach the shared source through the call that
  created it, and a turn that has produced nothing else yet has nothing to share. A pipeline report is
  shareable only after `report_pipeline_stage_result` succeeds and before that same reporting turn
  reaches `turn_end`; after pipeline reconciliation advances or completes the run, this selector is
  unavailable. The narrow pipeline selector remains useful because accepted reports are already
  concise, immutable, durable results and the same canonical report reference can later be attached
  by the work-object feature without changing its identity.
- **R5 — Direct authorization is separate and reusable.** Sharing creates or refreshes a
  direct grant from the caller to the recipient for the canonical reference. The grant owns its
  label, description, grantor, recipient, and lifecycle. The same reference may have independent
  grants to several agents. A grant authorizes retrieval but does not transfer ownership of the
  source or reference.
- **R6 — The ad-hoc list is not the work-assignment protocol.** A token-bound agent may
  list bounded metadata for its direct grants, newest first, and filter out personally hidden
  entries. The list does not include future work attachments and is not where an agent is expected
  to infer which context belongs to its current assignment. A future work API returns the reference
  ids and attachment-specific presentation metadata for that work directly.
- **R7 — Hidden is personal list state.** The recipient may hide or unhide a direct grant
  in its ad-hoc list. Hidden state changes no reference, source, authorization, or future work
  attachment; hiding is not revocation or deletion. This feature has no read/unread or seen state:
  retrieval is not a control signal and no consumer needs such bookkeeping.
- **R8 — Revocation is an authorization operation.** A grantor may revoke its own direct
  grant, after which that grant no longer authorizes the recipient or appears in its normal ad-hoc
  list. Revocation does not delete the canonical reference, another direct grant, or a future work
  attachment. Re-sharing the same reference from the same grantor to the same recipient restores
  that grant and applies the newly supplied presentation metadata.

### 2.3 Retrieval and plane boundaries

- **R9 — Retrieval is explicit, authorized, and bounded.** A token-bound agent reads a
  reference by id only when an effective authorization path exists. AgentDeck returns a stable
  source description and one bounded, deterministic text page with an opaque continuation cursor;
  repeated reads can traverse the fixed source without changing it. Missing and unauthorized ids
  share one safe `context_not_found` outcome.
- **R10 — Context remains outside the conversation until pulled.** Creating, granting,
  listing, hiding, revoking, or attaching a reference starts no model turn and inserts no mailbox
  row, activation, user prompt, transcript event, pipeline result, or SSE content payload. Only the
  agent's explicit read tool result enters the active provider conversation. A sender may separately
  use ordinary mail when it wants the existing mail-specific activation behavior, referring to the
  context id rather than copying its content.
- **R11 — Context reads have their own bounds.** Context operations do not consume or
  reset FS-06's mail budget. List size, metadata fields, rendered source fields, and each returned
  content page are capped by the context feature's own fixed limits. Empty collections are arrays,
  and a continuation cursor never embeds source content or authorization data. A cursor that does
  not name a position inside the fixed source is rejected rather than answered with damaged text or
  a false completion.

## 3. States & transitions

- **Reference:** absent → canonicalized. A reference is immutable and retained as a small durable
  locator; it has no read/unread, target, assignment, or activation state.
- **Direct grant:** absent → active → revoked; sharing the same reference again by the same grantor
  to the same recipient returns it to active and replaces that grant's presentation metadata.
- **Personal projection:** visible ↔ hidden. Revocation makes the projection irrelevant without
  changing the reference.
- **Read:** reference id + effective authorization → bounded page → optional next cursor. Reading is
  side-effect free.
- **Future work integration:** a work owner stores its own attachment from work id to reference id
  and returns it from that work's assignment/detail operation. Authorization follows the work
  owner's durable participant membership: terminal completion alone does not remove access;
  reassignment or explicit participant removal does. This feature establishes the reusable
  reference boundary but does not introduce work objects, dependencies, reassignment, or an
  assignment API.

## 4. Edge cases & errors

- **R12 — Archive and process lifecycle do not invalidate context.** A stopped or archived
  source remains readable because its transcript or report is durable. A stopped recipient keeps
  its grants and sees them after an ordinary resume. Live process ids, provider session ids, runtime
  status, and task running/terminal status are never reference identity or authorization.
- **R13 — Deleted sources become tombstones, not aliases.** If an agent transcript or
  pipeline run/attempt is deleted after a reference was created, the reference remains identifiable
  but reads return a typed `context_source_unavailable` tombstone. AgentDeck neither remaps it to a
  newer source nor retains an implicit content snapshot. Archive is not deletion.
- **R14 — Identity relationships have narrow durable effects.** The schema cascades a
  recipient row deletion to its direct grants and personal preferences as defensive referential
  hygiene, while retaining a grantor id as logical provenance without a foreign-key cascade.
  AgentDeck adds no agent-deletion product operation in this feature. Deleting or revoking one
  relationship cannot cascade into the underlying transcript, pipeline run, another grant, or
  future work attachment.
- **R15 — Invalid source selection is atomic.** An empty or reversed transcript range, a
  range outside the caller's durable transcript, a current turn with no persisted content, an
  unreported or unrelated pipeline attempt, an invalid recipient, or over-limit presentation
  metadata returns a typed error and creates or changes no reference, grant, or personal state.
  A friendly selector that has no eligible source at share time returns `source_unavailable`;
  `context_source_unavailable` is reserved for a previously created reference whose source later
  became unreadable as described by R13.
- **R16 — Mutable and opaque sources remain excluded.** Pipeline named values, tracked-file
  rows, archive search snippets, workspace files, project-resource files, arbitrary filesystem
  paths, URLs, uploaded blobs, authored summaries, and generic artifacts are not context-reference
  sources in this feature. A diff is retrievable only when it is an event inside a referenced
  transcript span. Adding a mutable/file source requires its own version identity, permission,
  retention, and deletion requirements rather than weakening R1.
- **R17 — Context recipients are not mail wake candidates.** Direct sharing resolves over
  durable, non-archived chat-agent identities, whether running or stopped and regardless of pipeline
  association; it does not reuse mail's stopped-wakeable gates or imply that the recipient can be
  woken. Terminal agents and unknown, ambiguous, archived, or deleted recipients receive a typed
  recipient error and no grant. A recipient becoming stopped or pipeline-associated after the grant
  does not revoke it.

## 5. Acceptance criteria

Each names the verification that demonstrates it.

- **A1** (R1–R3) — Repeated canonicalization of one transcript range returns one
  reference id, while two direct grants retain independent labels without changing that id:
  state/service tests under `internal/state` and `internal/contextref`.
- **A2** (R2, R4–R5, R15) — Two token-bound fake-ACP agents share the caller's current-turn
  and latest-completed-turn transcript spans; the current-turn case includes conclusion text emitted
  before the share call and excludes later text; recipient grants are durable across server restart,
  and attempts to name another source agent or unrelated pipeline attempt mutate nothing:
  MCP integration tests under `internal/messaging` and `internal/server`. Both adapter event orders —
  the invoking `tool_call` already durable, and not yet written — resolve one identical span that
  carries none of the share's own arguments:
  `internal/contextref/service_test.go::TestCurrentTurnSpanExcludesTheInFlightShareCall`.
- **A3** (R2, R4–R5) — A pipeline agent with an accepted report shares a canonical attempt
  report inside the reporting turn, while a current mutable named value and an unreported attempt
  are rejected; after `turn_end` and pipeline quiescence the same friendly selector returns
  `source_unavailable` without mutation, while the already-created reference remains readable:
  `internal/contextref`/`internal/pipeline` integration tests.
- **A4** (R6–R8) — Direct-grant listing is caller-scoped and bounded; hide only removes
  a grant from the normal list, unhide restores it, reads create no personal-state
  mutation, grantor revocation removes authorization, and re-share restores that grant without
  affecting another grant for the same reference: MCP and state tests.
- **A5** (R9–R11) — Authorized transcript and pipeline reads return deterministic bounded
  pages whose cursors traverse the fixed source; every current normalized event kind is rendered or
  deliberately classified metadata-only, unknown kinds and skipped oversized records produce
  bounded markers including at a selected span's leading and trailing edges, an altered page cursor
  is rejected, missing and unauthorized ids are indistinguishable, no source content appears in
  logs/SSE, and mail budget state is unchanged: service, protocol, and server-bus tests.
- **A6** (R10) — Sharing and listing context produce no provider prompt, activation,
  mailbox row, or normalized transcript event; only an explicit read returns content to the MCP
  caller: fake-ACP provider-prompt and persistence assertions.
- **A7** (R12–R14, R17) — Stop/resume and source archive retain reads; reachable source
  deletion yields a tombstone without cascading into other grants or sources; and sharing to a
  stopped pipeline-associated chat agent succeeds without waking it, while an archived or terminal
  target is rejected: lifecycle and deletion integration tests plus state-level foreign-key
  coverage for the defensive recipient cascade.

## 6. Deviations & open decisions

- **Planned transport supersession.** If and only if FS-17.R20 passes and the direct-action
  migration ships, FS-17.R13–R19 replace only this specification's internal-MCP transport wording.
  Reference identity, grants, authorization, bounded reads, source handling, outcomes, and retention
  remain authoritative. Until then the shipped MCP requirements and acceptance criteria are current.

- No deviations.
- **Direct grants deliberately have no owner UI or automatic expiry.** A direct grant remains active
  until its grantor revokes it or its recipient identity is removed. This accepts the narrow risk of
  long-lived agent access to accidentally sensitive transcript content in the current local,
  single-user product rather than adding a management surface or a timer that can silently break
  delayed work. Source content is still never copied into the grant or exposed without an explicit
  authorized read. Revisit this policy only with evidence that the edge case matters in practice.
- Work objects, dependency evaluation, task reassignment, host-managed waiting, and semantic
  assignment APIs are deliberately excluded. Their approved integration rule is recorded in §3 so
  those features attach reusable references and expose them through the owning work object rather
  than turning `list_context_links` into an orchestration inbox.
- MCP resources are not the initial agent-facing surface. The pinned Claude and Codex ACP adapters
  lower ACP resource blocks into prompt-visible material, while reliable provider exposure of MCP
  `resources/list` and `resources/read` is not yet proven. A later capability-gated MCP resource
  template may delegate to the same authorization and bounded-read service.

## 7. Traceability

Anchors: canonical reference/grant service in `internal/contextref`; forward-only state in
`internal/state`; token-scoped tools in the existing `internal/messaging` MCP server; transcript
source resolution through `internal/transcript`; immutable pipeline reports through
`internal/state`/`internal/pipeline`; existing generation-scoped MCP registration in
`internal/server/messaging_registration.go`.
