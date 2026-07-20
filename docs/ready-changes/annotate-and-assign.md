# Annotate and assign

**State:** Waiting to start
**Why:** Direct request (2026-07-20), inspired by the Codex app's diff/browser commenting: point at
the content itself instead of describing locations in prose. The idea was recorded and defined the
same day; the human confirmed the four scope decisions (surfaces, batching, new-task meaning,
cross-agent delivery) in conversation.
**Relevant requirements:** FS-13.R1–R15, FS-13.A1–A6, FS-06.R21, FS-06.A10, TS-02.R14–R15,
TS-03.R14; existing boundaries FS-03.R6, FS-03.R8, FS-03.R20, FS-05.R5, FS-05.R14, FS-06.R4,
FS-06.R5, FS-06.R17, FS-01.R1, TS-05.R4, TS-05.R10, INV §2, INV §8, INV §11, INV §14.

## Outcome

A person reviewing an agent's work selects lines inside a rendered diff or a whole transcript event
(live chat or archived view), attaches a short instruction, accumulates several such annotations in
a pending tray, and sends the batch to the current agent, another running chat agent, or a new
prefilled agent launch. The annotation persists as a structured transcript event (quoted excerpt,
anchor, instruction, target) that renders as annotation cards, is archive-searchable, and reaches
agents as a machine-generated context block — never hand-pasted prose locations.

## Included work

Included: diff-line and whole-event selection with the Annotate affordance; the per-browser pending
tray; the `POST /api/sessions/{id}/annotations` endpoint with `self`/`agent` targets; the durable
`annotation` transcript event and its card rendering (live + archive replay); reserved user-sender
mail delivery with nudge/unread/retention reuse and no turn-budget consumption; the prefilled New
Agent modal flow for "new task"; FTS index wiring; UI schema lockstep.

Excluded: terminal-output, file-on-disk, screenshot, and web-page surfaces (the latter three don't
exist in the product); comment threads, replies, or resolve workflows; any live link from an
annotation to later file state; new SSE event types; terminal agents as sources or targets.

## How we will know it works

FS-13.A1–A6 (planned UI, integration, messaging, and index tests plus a new usability journey added
to `USABILITY-REVIEW.md` at implementation) and FS-06.A10 (reserved-sender nudge/no-budget/no-spoof
tests). Shared checks per TS-06 and workflow §2.

## Waiting on

Nothing.
