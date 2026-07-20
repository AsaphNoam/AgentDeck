# FS-13 — Annotate and assign

**Status:** Partial
**Code:** planned — `ui/src/components/chat/`, `ui/src/features/archive/`, `internal/server/`, `internal/runtime/`, `internal/state/` · **Journeys:** —
**Absorbed:** —

## 1. Purpose

Instead of describing a location in prose, a person points at the thing itself: select lines inside
a rendered diff or a whole transcript event, attach a short instruction, and send the result to the
current agent, another running chat agent, or a newly launched agent. AgentDeck preserves each
annotation as structured, located context — captured excerpt, anchor, instruction, target — never as
hand-pasted chat text. The chat surface belongs to FS-03, the archived view to FS-05, mail delivery
to FS-06, and launch to FS-01; this spec owns the annotation interaction, its records, and its
delivery behavior. Nothing in this spec is shipped; every requirement is `(planned)`.

## 2. Behavior

Requirements are user- and API-observable. R-item numbering is continuous through §4.

### 2.1 Selecting and capturing

- **R1 (planned).** In a live chat transcript and in the archived read-only transcript view, a user
  can select a contiguous line range inside a rendered diff block, or select one whole transcript
  event (for example a message, tool call, tool result, or error), and choose **Annotate** on the
  selection.
- **R2 (planned).** Annotating captures a structured record: the source `agent_id`, the anchored
  transcript event `seq`, the diff's file path, side, and 1-based line range when the selection is
  diff lines, the selected excerpt text verbatim (clipped at 2,000 characters with a visible
  truncation marker), and a required instruction of at most 2,000 characters. The user never types a
  file path, line number, or location description; AgentDeck derives the anchor from the selection.
- **R3 (planned).** Annotations accumulate in a pending tray scoped to the source session. The tray
  shows each entry's excerpt and instruction and supports editing an instruction, removing an entry,
  and discarding the tray. It holds at most 20 entries. The tray is per-browser draft state: it
  survives a reload of the same browser, is not visible from other browsers or the API, and creates
  no durable server data until send. A successful send clears it; a failed send preserves it.
- **R4 (planned).** The tray is sent as one batch to exactly one target — the current agent, another
  running chat agent, or a new task (R9) — with an optional overall instruction of at most 2,000
  characters.

### 2.2 Delivery

- **R5 (planned).** Every successful send appends one durable `annotation` event to the **source**
  session's transcript — whether that session is active or inactive — recording the batch and its
  target. Live chat and archive replay render it as quoted-excerpt annotation cards with their
  instructions, not as pasted user prose; rendering is sanitized under the same rules as other
  transcript content (FS-03.R20).
- **R6 (planned).** Target **current agent** requires the source agent to be a running, idle chat
  agent. The send starts one ordinary prompt turn (FS-03.R8) whose agent-visible content is a
  machine-generated annotation block: for each annotation, the file path and line range when
  present, the quoted excerpt, and its instruction, plus the overall instruction. A busy,
  waiting, stopped, or terminal source returns a typed error and the tray is preserved.
- **R7 (planned).** Target **another agent** requires a running chat-interface recipient
  (FS-06.R4, FS-06.R17). The batch is delivered as coordination mail from the reserved user sender
  (FS-06.R21) with the annotation block as its body; normal unread indicators, idle nudging, reading,
  and retention rules apply unchanged. A stopped or invalid recipient returns a typed error and the
  tray is preserved.
- **R8 (planned).** Mail delivery may clip excerpts (with a visible marker) to fit the FS-06.R5 body
  limit; anchors and instructions are never clipped. A batch whose anchors and instructions alone
  cannot fit returns `422 validation` naming the constraint, and the tray is preserved.
- **R9 (planned).** Target **new task** opens the existing New Agent modal prefilled with the source
  agent's role and project (name auto-suggest per FS-01.R1/R4). When that launch succeeds, the batch
  is delivered to the new agent as reserved-sender mail (R7), whose idle-nudge wakes the fresh agent
  so processing the annotations is its first activity, and the source session's annotation event
  (R5) records the new agent as target. Cancelling the modal preserves the tray.

### 2.3 Structure and search

- **R10 (planned).** Annotation instructions and captured excerpts are transcript content: on an
  FTS5 build they are searchable from the archive (FS-05.R5); the untagged fallback build limits
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

- **R11 (planned).** An empty tray, an empty or whitespace-only instruction, or an over-limit
  instruction, excerpt count, or overall instruction returns `422 validation` naming the violated
  constraint. An unknown source agent returns `404`.
- **R12 (planned).** Annotations are point-in-time captures. The excerpt is authoritative for what
  the user saw; delivery does not re-read the file on disk, and the anchor may reference content
  that has since changed. No revalidation or live link is implied.
- **R13 (planned).** Terminal-interface agents are neither an annotation surface nor a valid target
  (FS-03.R22, FS-06.R17). Files on disk, screenshots, and rendered web pages are not selectable
  surfaces; no such viewing surface exists in the product.
- **R14 (planned).** There are no comment threads, replies, or resolve/re-open workflow. Once
  recorded, an annotation event is immutable like every other transcript event.
- **R15 (planned).** From an archived (inactive) source session, the current-agent target is
  unavailable; another-agent and new-task targets remain. The archived view stays composer-free
  (FS-05.R14); annotating never sends a prompt to the archived session itself.

## 5. Acceptance criteria

All planned; each names the verification to be created with the implementation.

- **A1 (planned)** (R1–R4) — Selecting diff lines and a whole transcript event captures anchors and
  excerpts without typing a location; the tray edits, survives a reload, and clears on send:
  planned UI tests for the annotation tray and selection components under `ui/src/components/chat/`.
- **A2 (planned)** (R5–R6) — A self-targeted batch produces one prompt turn and one durable
  annotation event that renders as annotation cards identically live and after reload: planned
  integration test in `internal/server` plus a transcript-store UI test.
- **A3 (planned)** (R7–R8) — Assigning to a second agent inserts reserved-sender mail that raises
  the unread badge and nudges an idle recipient; an unfittable batch is rejected `422` with the tray
  preserved: planned tests in `internal/messaging` and `internal/state`.
- **A4 (planned)** (R9) — New task opens the prefilled New Agent modal and, after launch, the new
  agent's first activity is processing the delivered annotations: planned UI and integration tests.
- **A5 (planned)** (R10) — An annotation instruction is findable through archive search on the FTS5
  build: planned test in `internal/index`/`internal/archive`.
- **A6 (planned)** (R1–R15) — A user annotates a diff and a message, sends to self, to a second
  agent, and to a new task, and sees structured cards and mail arrive: a new usability journey added
  to `docs/features/USABILITY-REVIEW.md` with the implementation; manual until then.

## 6. Deviations & open decisions

- Nothing is shipped; there are no deviations. The numeric limits in R2, R3, and R4 are initial
  values and may be tuned before shipping in a spec-first update without changing the model.

## 7. Traceability

Planned anchors, to be confirmed at implementation: annotation endpoint in `internal/server`;
`annotation` event kind in `internal/runtime/event.go`; shared block renderer used by prompt and
mail delivery; tray and card components under `ui/src/components/chat/`; reserved-sender mail in
`internal/state/messages.go`; index wiring in `internal/index/indexer.go`.
