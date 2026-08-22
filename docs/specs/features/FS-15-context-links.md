# FS-15 — Pull-based context links

**Status:** Partial
**Code:** planned — `internal/contextref`, `internal/state`, `internal/messaging` · **Journeys:** —
**Absorbed:** —

## 1. Purpose

Agents can expose durable work to one another without copying it into mail or eagerly injecting it
into another model's conversation. AgentDeck gives an immutable source selection a stable,
target-neutral context reference, grants agents permission to retrieve that reference, and returns
its content only through an explicit bounded read. Direct sharing is an ad-hoc convenience; future
work objects may attach the same reference and expose it through their own assignment API without
turning context discovery into a global inbox scan. Nothing in this spec is shipped; every
requirement is `(planned)`.

## 2. Behavior

Requirements are user- and agent/API-observable. R-item numbering is continuous through §4.

### 2.1 References and sources

- **R1 (planned) — A reference identifies a source, not a recipient.** A context reference has one
  stable opaque id and an immutable typed source locator. Re-creating the same locator returns the
  same reference id; sharing it with another agent or attaching it to another piece of work does not
  duplicate the reference. The reference records intrinsic provenance but contains no target,
  presentation title, description, read state, work id, prompt, or copied source content.
- **R2 (planned) — The initial sources are immutable AgentDeck records.** The accepted source kinds
  are (a) an exact inclusive sequence range in one agent's append-only normalized transcript and
  (b) the accepted immutable report of one pipeline attempt. A transcript source is resolved to
  concrete `agent_id`, first-sequence, and last-sequence values before its reference is returned; a
  pipeline source is resolved to one concrete attempt id whose report has already been accepted.
- **R3 (planned) — Presentation belongs to the access path.** A direct grant or future work
  attachment may give the same reference its own bounded label and description. Those fields may
  differ between grants and attachments and are never part of reference identity. Reading the
  source by reference id is independent of which label led the caller to it.

### 2.2 Direct sharing and discovery

- **R4 (planned) — Agents can share only context they currently own.** A token-bound chat agent may
  share the transcript content accumulated in its current turn so far, its latest completed
  transcript turn, or the accepted report of its current pipeline attempt, with one resolvable chat
  agent. AgentDeck derives the caller and source identity from the live MCP session and resolves the
  friendly source selector to the exact R2 locator; the caller cannot name another agent's
  transcript or an unrelated pipeline attempt. Sharing returns both the canonical reference id and
  the direct-grant id.
- **R5 (planned) — Direct authorization is separate and reusable.** Sharing creates or refreshes a
  direct grant from the caller to the recipient for the canonical reference. The grant owns its
  label, description, grantor, recipient, and lifecycle. The same reference may have independent
  grants to several agents. A grant authorizes retrieval but does not transfer ownership of the
  source or reference.
- **R6 (planned) — The ad-hoc list is not the work-assignment protocol.** A token-bound agent may
  list bounded metadata for its direct grants, newest first, and filter out personally hidden
  entries. The list does not include future work attachments and is not where an agent is expected
  to infer which context belongs to its current assignment. A future work API returns the reference
  ids and attachment-specific presentation metadata for that work directly.
- **R7 (planned) — Seen and hidden are personal control state.** Reading a directly granted
  reference records that the recipient has seen that grant. The recipient may hide or unhide the
  grant in its ad-hoc list. Seen/hidden state changes no reference, source, authorization, or future
  work attachment; hiding is not revocation or deletion.
- **R8 (planned) — Revocation is an authorization operation.** A grantor may revoke its own direct
  grant, after which that grant no longer authorizes the recipient or appears in its normal ad-hoc
  list. Revocation does not delete the canonical reference, another direct grant, or a future work
  attachment. Re-sharing the same reference from the same grantor to the same recipient restores
  that grant and applies the newly supplied presentation metadata.

### 2.3 Retrieval and plane boundaries

- **R9 (planned) — Retrieval is explicit, authorized, and bounded.** A token-bound agent reads a
  reference by id only when an effective authorization path exists. AgentDeck returns a stable
  source description and one bounded, deterministic text page with an opaque continuation cursor;
  repeated reads can traverse the fixed source without changing it. Missing and unauthorized ids
  share one safe `context_not_found` outcome.
- **R10 (planned) — Context remains outside the conversation until pulled.** Creating, granting,
  listing, hiding, revoking, or attaching a reference starts no model turn and inserts no mailbox
  row, activation, user prompt, transcript event, pipeline result, or SSE content payload. Only the
  agent's explicit read tool result enters the active provider conversation. A sender may separately
  use ordinary mail when it wants the existing mail-specific activation behavior, referring to the
  context id rather than copying its content.
- **R11 (planned) — Context reads have their own bounds.** Context operations do not consume or
  reset FS-06's mail budget. List size, metadata fields, rendered source fields, and each returned
  content page are capped by the context feature's own fixed limits. Empty collections are arrays,
  and a continuation cursor never embeds source content or authorization data.

## 3. States & transitions

- **Reference:** absent → canonicalized. A reference is immutable and retained as a small durable
  locator; it has no read/unread, target, assignment, or activation state.
- **Direct grant:** absent → active → revoked; sharing the same reference again by the same grantor
  to the same recipient returns it to active and replaces that grant's presentation metadata.
- **Personal projection:** unseen/visible → seen/visible → seen/hidden, with unhide returning to
  visible. Revocation makes the projection irrelevant without changing the reference.
- **Read:** reference id + effective authorization → bounded page → optional next cursor. Reading is
  side-effect free except for marking an applicable direct grant seen.
- **Future work integration:** a work owner stores its own attachment from work id to reference id
  and returns it from that work's assignment/detail operation. Authorization follows the work
  owner's durable participant membership: terminal completion alone does not remove access;
  reassignment or explicit participant removal does. This feature establishes the reusable
  reference boundary but does not introduce work objects, dependencies, reassignment, or an
  assignment API.

## 4. Edge cases & errors

- **R12 (planned) — Archive and process lifecycle do not invalidate context.** A stopped or archived
  source remains readable because its transcript or report is durable. A stopped recipient keeps
  its grants and sees them after an ordinary resume. Live process ids, provider session ids, runtime
  status, and task running/terminal status are never reference identity or authorization.
- **R13 (planned) — Deleted sources become tombstones, not aliases.** If an agent transcript or
  pipeline run/attempt is deleted after a reference was created, the reference remains identifiable
  but reads return a typed `context_source_unavailable` tombstone. AgentDeck neither remaps it to a
  newer source nor retains an implicit content snapshot. Archive is not deletion.
- **R14 (planned) — Identity deletion has narrow effects.** Deleting a recipient removes its direct
  grants and personal projections. Deleting a grantor does not delete already granted references or
  erase their recorded provenance. Deleting or revoking one relationship cannot cascade into the
  underlying transcript, pipeline run, agent, another grant, or future work attachment.
- **R15 (planned) — Invalid source selection is atomic.** An empty or reversed transcript range, a
  range outside the caller's durable transcript, a current turn with no persisted content, an
  unreported or unrelated pipeline attempt, an invalid recipient, or over-limit presentation
  metadata returns a typed error and creates or changes no reference, grant, or personal state.
- **R16 (planned) — Mutable and opaque sources remain excluded.** Pipeline named values, tracked-file
  rows, archive search snippets, workspace files, project-resource files, arbitrary filesystem
  paths, URLs, uploaded blobs, authored summaries, and generic artifacts are not context-reference
  sources in this feature. A diff is retrievable only when it is an event inside a referenced
  transcript span. Adding a mutable/file source requires its own version identity, permission,
  retention, and deletion requirements rather than weakening R1.
- **R17 (planned) — Context recipients are not mail wake candidates.** Direct sharing resolves over
  durable, non-archived chat-agent identities, whether running or stopped and regardless of pipeline
  association; it does not reuse mail's stopped-wakeable gates or imply that the recipient can be
  woken. Terminal agents and unknown, ambiguous, archived, or deleted recipients receive a typed
  recipient error and no grant. A recipient becoming stopped or pipeline-associated after the grant
  does not revoke it.

## 5. Acceptance criteria

All planned; each names the verification to be created with the implementation.

- **A1 (planned)** (R1–R3) — Repeated canonicalization of one transcript range returns one
  reference id, while two direct grants retain independent labels without changing that id: planned
  state/service tests under `internal/state` and `internal/contextref`.
- **A2 (planned)** (R2, R4–R5, R15) — Two token-bound fake-ACP agents share the caller's current-turn
  and latest-completed-turn transcript spans; recipient grants are durable across server restart,
  and attempts to name another source agent or unrelated pipeline attempt mutate nothing: planned
  MCP integration tests under `internal/messaging` and `internal/server`.
- **A3 (planned)** (R2, R4–R5) — A pipeline agent with an accepted report shares a canonical attempt
  report, while a current mutable named value and an unreported attempt are rejected: planned
  `internal/contextref`/`internal/pipeline` integration tests.
- **A4 (planned)** (R6–R8) — Direct-grant listing is caller-scoped and bounded; read marks the grant
  seen, hide only removes it from the normal list, unhide restores it, grantor revocation removes
  authorization, and re-share restores that grant without affecting another grant for the same
  reference: planned MCP and state tests.
- **A5 (planned)** (R9–R11) — Authorized transcript and pipeline reads return deterministic bounded
  pages whose cursors traverse the fixed source; missing and unauthorized ids are indistinguishable,
  no source content appears in logs/SSE, and mail budget state is unchanged: planned service,
  protocol, and server-bus tests.
- **A6 (planned)** (R10) — Sharing and listing context produce no provider prompt, activation,
  mailbox row, or normalized transcript event; only an explicit read returns content to the MCP
  caller: planned fake-ACP provider-prompt and persistence assertions.
- **A7 (planned)** (R12–R14, R17) — Stop/resume and source archive retain reads; source deletion
  yields a tombstone; recipient deletion removes only its direct grants; and sharing to a stopped
  pipeline-associated chat agent succeeds without waking it, while an archived or terminal target
  is rejected: planned lifecycle and deletion integration tests.

## 6. Deviations & open decisions

- Nothing is shipped; there are no deviations.
- Work objects, dependency evaluation, task reassignment, host-managed waiting, and semantic
  assignment APIs are deliberately excluded. Their approved integration rule is recorded in §3 so
  those features attach reusable references and expose them through the owning work object rather
  than turning `list_context_links` into an orchestration inbox.
- MCP resources are not the initial agent-facing surface. The pinned Claude and Codex ACP adapters
  lower ACP resource blocks into prompt-visible material, while reliable provider exposure of MCP
  `resources/list` and `resources/read` is not yet proven. A later capability-gated MCP resource
  template may delegate to the same authorization and bounded-read service.

## 7. Traceability

Planned anchors: canonical reference/grant service in `internal/contextref`; forward-only state in
`internal/state`; token-scoped tools in the existing `internal/messaging` MCP server; transcript
source resolution through `internal/transcript`; immutable pipeline reports through
`internal/state`/`internal/pipeline`; existing generation-scoped MCP registration in
`internal/server/messaging_registration.go`.
