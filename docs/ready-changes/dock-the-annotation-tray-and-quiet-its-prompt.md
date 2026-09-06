# Dock the annotation tray and quiet its prompt in the conversation

**State:** Waiting to start
**Why:** Direct operator request on 2026-09-06: move the annotation window to the right side and
make it bigger, make each pending annotation easier to read, and stop showing so much annotation
meta in the conversation. Designed on 2026-09-07 from the `docs/ideas.md` entry that request opened.
**Relevant requirements:** FS-13.R20, FS-13.R21, FS-13.R22, FS-13.R23, FS-13.A12, FS-13.A13,
FS-13.A14, TS-08.R53, TS-08.R54, INV §1, INV §2, INV §13

## Outcome

While drafting annotations on the agent page, the tray stops covering the transcript it describes:
it becomes a full-height column on the right, the transcript reflows beside it, each draft has room
to be read, and the column can be collapsed to a strip to get the width back. After sending a batch
to the current agent, the conversation shows the annotation card alone instead of the card plus the
raw `[AgentDeck annotations]` block, in existing transcripts as well as new ones.

## Included work

Included: the docked column and its container-query fallback to today's overlay (FS-13.R20); the
collapse control and its per-source browser-local flag riding the existing annotation-draft
persistence (FS-13.R21); the roomier draft row with the anchor as its own heading (FS-13.R22);
render-time suppression of the self-target annotation prompt in `appendRenderedEvent`
(FS-13.R23); the `annotation-tray` contract registration and its CSS selectors; and the shared
sentinel constant plus the Go test that pins it to `runtime.FormatAnnotationBlock`.

Not included: any change to annotation capture, delivery, targets, mail, limits, or the New Agent
modal. The annotation card's own wording is deliberately untouched — on 2026-09-06 the operator was
offered, and declined, stripping its `Event <seq>` anchor and resolving its raw target agent id to a
name (FS-13 §6). The Tasks page's free-text Backend/Model/Effort inputs came up during design and
are not part of this change.

## How we will know it works

- **FS-13.A12** — `ui/src/components/chat/AnnotationTray.test.tsx` and
  `ui/src/store/annotationStore.test.ts`: docked at a wide transcript region, overlay at a narrow
  one, collapse and expand, and the collapsed flag surviving a reload and dying with its tray.
- **FS-13.A13** — `ui/src/components/chat/AnnotationTray.test.tsx`: a docked draft row's anchor
  heading is a separate element from its controls.
- **FS-13.A14** — `ui/src/components/chat/TranscriptView.test.tsx` and
  `internal/server/annotations_test.go`: a self-targeted send renders the card and no user message
  carrying the block, identically live and on replay, while the endpoint still returns the event.
- FS-13.A10's existing pinned header/controls check must still pass in both tray forms (FS-13.R18).
- The presentation contract checker and Stylelint (TS-08.R17–R19) cover the new component
  registration and selectors.

## Waiting on

Nothing.
