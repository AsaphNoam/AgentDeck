# FS-13 — Annotate and assign

**Status:** Current
**Code:** `ui/src/components/chat/`, `ui/src/features/archive/`, `internal/server/`, `internal/runtime/`, `internal/state/` · **Journeys:** J13
**Absorbed:** —

## 1. Purpose

Instead of describing a location in prose, a person points at the thing itself: select lines inside
a rendered diff or a whole transcript event, attach a short instruction, and send the result to the
current agent, another running chat agent, or a newly launched agent. AgentDeck preserves each
annotation as structured, located context — captured excerpt, anchor, instruction, target — never as
hand-pasted chat text. The chat surface belongs to FS-03, the archived view to FS-05, mail delivery
to FS-06, and launch to FS-01; this spec owns the annotation interaction, its records, and its
delivery behavior. The behavior below is shipped.

## 2. Behavior

Requirements are user- and API-observable. R-item numbering is continuous through §4.

### 2.1 Selecting and capturing

- **R1.** In a live chat transcript and in the archived read-only transcript view, a user
  can select a contiguous line range inside a rendered diff block, or select one whole transcript
  event (for example a message, tool call, tool result, or error), and choose **Annotate** on the
  selection.
- **R2.** Annotating captures a structured record: the source `agent_id`, the anchored
  transcript event `seq`, the diff's file path, side, and 1-based line range when the selection is
  diff lines, the selected excerpt text verbatim (clipped at 2,000 characters with a visible
  truncation marker), and a required instruction of at most 2,000 characters. The user never types a
  file path, line number, or location description; AgentDeck derives the anchor from the selection.
- **R3.** Annotations accumulate in a pending tray scoped to the source session. The tray
  shows each entry's excerpt and instruction and supports editing an instruction, removing an entry,
  and discarding the tray. It holds at most 20 entries. The tray is per-browser draft state: it
  survives a reload of the same browser, is not visible from other browsers or the API, and creates
  no durable server data until send. A successful send clears it; a failed send preserves it.
- **R4.** The tray is sent as one batch to exactly one target — the current agent, another
  running chat agent, or a new task (R9) — with an optional overall instruction of at most 2,000
  characters.

### 2.2 Delivery

- **R5.** Every successful send appends one durable `annotation` event to the **source**
  session's transcript — whether that session is active or inactive — recording the batch and its
  target. The event is appended **before** the batch is delivered, so no agent ever receives
  annotations the source transcript does not record. When delivery itself fails after that append,
  the batch is undelivered and the tray is preserved (R3); re-sending it delivers exactly once and
  records a second annotation event for the retried batch. Live chat and
  archive replay render it as quoted-excerpt annotation cards with their
  instructions, not as pasted user prose; rendering is sanitized under the same rules as other
  transcript content (FS-03.R20).
- **R6.** Target **current agent** requires the source agent to be a running, idle chat
  agent. The send starts one ordinary prompt turn (FS-03.R8) whose agent-visible content is a
  machine-generated annotation block: for each annotation, the file path and line range when
  present, the quoted excerpt, and its instruction, plus the overall instruction. A busy,
  waiting, stopped, or terminal source returns a typed error and the tray is preserved.
- **R7.** Target **another agent** requires a running chat-interface recipient
  (FS-06.R4, FS-06.R17). The batch is delivered as coordination mail from the reserved user sender
  (FS-06.R21) with the annotation block as its body; normal unread indicators, idle nudging, reading,
  and retention rules apply unchanged. A stopped or invalid recipient returns a typed error and the
  tray is preserved.
- **R8.** Mail delivery may clip excerpts (with a visible marker) to fit the FS-06.R5 body
  limit; anchors and instructions are never clipped. A batch whose anchors and instructions alone
  cannot fit returns `422 validation` naming the constraint, and the tray is preserved.
- **R9.** Target **new task** opens the existing New Agent modal prefilled with the source
  agent's role and project (name auto-suggest per FS-01.R1/R4). When that launch succeeds, the batch
  is delivered to the new agent as reserved-sender mail (R7), whose idle-nudge wakes the fresh agent
  so processing the annotations is its first activity, and the source session's annotation event
  (R5) records the new agent as target. Cancelling the modal preserves the tray.

### 2.3 Structure and search

- **R10.** Annotation instructions and captured excerpts are transcript content: on an
  FTS5 build they are searchable from the archive (FS-05.R25-R27); the untagged fallback build limits
  search per FS-05.R6.

## 3. States & transitions

- **Tray:** empty → drafting (add/edit/remove entries) → sending → cleared on acknowledged send. A
  failed or rejected send returns to drafting with all content intact.
- **Delivery:** current agent → one prompt turn under the normal FS-03 busy lifecycle; another agent
  → unread mail following FS-06 pending → nudged → read; new task → prefilled modal → launch →
  reserved-sender mail nudges the new agent's first turn.
- **Durability:** the annotation event follows the source transcript's lifetime; a mail copy follows
  FS-06.R8 retention independently.

## 4. Edge cases & errors

- **R11.** An empty tray, an empty or whitespace-only instruction, or an over-limit
  instruction, excerpt count, or overall instruction returns `422 validation` naming the violated
  constraint. An unknown source agent returns `404`.
- **R12.** Annotations are point-in-time captures. The excerpt is authoritative for what
  the user saw; delivery does not re-read the file on disk, and the anchor may reference content
  that has since changed. No revalidation or live link is implied.
- **R13.** Terminal-interface agents are neither an annotation surface nor a valid target
  (FS-03.R22, FS-06.R17). Files on disk, screenshots, and rendered web pages are not selectable
  surfaces; no such viewing surface exists in the product.
- **R14.** There are no comment threads, replies, or resolve/re-open workflow. Once
  recorded, an annotation event is immutable like every other transcript event.
- **R15.** From an archived (inactive) source session, the current-agent target is
  unavailable; another-agent and new-task targets remain. The archived view stays composer-free
  (FS-05.R14); annotating never sends a prompt to the archived session itself.
- **R16.** Pending trays are bounded browser-local drafts, not durable data. Deleting an
  agent discards its tray, because no target can accept a batch from a source that no longer exists
  (R11). Independently, stored trays expire 30 days after their last edit and only the 20
  most-recently-edited sources are retained; both are applied when the browser reloads the stored
  trays. This keeps a session's own tray across reloads (R3) — including an archived session's,
  which is not in the live agent list — while bounding what accumulates in browser storage. If a
  retained tray's source can no longer be found, its missing-agent route identifies the pending
  draft count and offers a direct discard action rather than trapping the retained slot. Because
  that discard is destructive, the route treats a source as missing only after the browser's first
  agent hydration has completed; until then a deep link or reload shows a loading state and offers
  no discard.
- **R17.** A selectable diff block labels its line numbers as the selection control and presents
  them with a pointer cursor. Clicking one line starts a selection; clicking another line on the
  same side extends the contiguous range before **Annotate** captures it (R1–R2).
- **R18.** The pending tray keeps its header and delivery controls visible within its bounded
  overlay while the draft list and overall instruction scroll independently. Multiple detailed
  drafts therefore cannot push target selection, errors, or **Send annotations** below the tray's
  visible edge.

## 5. Acceptance criteria

Each acceptance item names its delivered verification.

- **A1** (R1–R4) — Selecting diff lines and a whole transcript event captures anchors and
  excerpts without typing a location; the tray edits, survives a reload, and clears on send:
  `ui/src/store/annotationStore.test.ts` plus chat component coverage.
- **A2** (R5–R6) — A self-targeted batch produces one prompt turn and one durable
  annotation event that renders as annotation cards identically live and after reload: focused
  fake-ACP integration check plus transcript-store replay coverage.
- **A3** (R7–R8) — Assigning to a second agent inserts reserved-sender mail that raises
  the unread badge and nudges an idle recipient; an unfittable batch is rejected `422` with the tray
  preserved: `internal/server/annotations_test.go` and existing messaging/state nudge coverage.
- **A4** (R9) — New task opens the prefilled New Agent modal and, after launch, the new
  agent's first activity is processing the delivered annotations: `NewAgentModal.test.tsx` launch coverage and the annotation delivery integration test.
- **A5** (R10) — An annotation instruction is findable through archive search on the FTS5
  build: `internal/index/indexer_test.go::TestAnnotationFlushesSearchableContent`.
- **A6** (R1–R15) — A user annotates a diff and a message, sends to self, to a second
  agent, and to a new task, and sees structured cards and mail arrive: journey **J13** in
  `docs/features/USABILITY-REVIEW.md`.
- **A7** (R5) — A failed source-transcript append delivers no mail, and the preserved tray's
  retry delivers exactly one copy:
  `internal/server/annotations_test.go::TestAnnotationAppendFailureDeliversNoMailAndRetrySendsOnce`.
- **A8** (R16) — Deleting an agent drops its pending tray, stored trays are capped and expired on
  rehydration, and a missing-agent route can discard its retained tray only after hydration
  completes (pre-hydration, present-after-hydration, and absent-after-hydration are covered):
  `ui/src/store/annotationStore.test.ts`, `ui/src/api/sse.test.ts`, and
  `ui/src/components/chat/ChatPanel.test.tsx`.
- **A9** (R1–R2, R17) — A diff block visibly explains line-number selection and applies the pointer
  affordance while the library-shaped id regression captures the selected range:
  `ui/src/components/chat/renderers/DiffBlock.test.tsx`.
- **A10** (R3–R4, R18) — A tray with three drafts renders them in the scrollable body and keeps its
  target and Send action in a separate fixed footer: `ui/src/components/chat/AnnotationTray.test.tsx`.

## 6. Deviations & open decisions

- The numeric limits in R2, R3, and R4 are initial values and may be tuned only through a
  spec-first update.

## 7. Traceability

Delivered anchors: annotation endpoint in `internal/server`;
`annotation` event kind in `internal/runtime/event.go`; `runtime.FormatAnnotationBlock` used by prompt and mail delivery; tray and card components under `ui/src/components/chat/`; reserved-sender mail in
`internal/state/messages.go`; index wiring in `internal/index/indexer.go`.
