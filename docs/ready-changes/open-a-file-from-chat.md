# Open a file an agent mentioned

**State:** Waiting to start
**Why:** Direct request on 2026-09-10 — agents constantly emit filepath links in chat and clicking
one currently leaves the conversation. Recorded first in `docs/ideas.md` under
`Ideas being defined`, removed from there by this change.
**Relevant requirements:** FS-03.R51, FS-03.R52, FS-03.R53, FS-03.R54, FS-03.R55, FS-05.R37,
TS-03.R40, TS-05.R21, TS-08.R57, INV §1, INV §2, INV §13, INV §14

## Outcome

A filepath link an agent wrote in chat opens that file in a read-only viewer beside the transcript,
instead of doing what it does today: a real browser navigation to a path that is not an application
route, which the SPA fallback resolves to `index.html` and the router's catch-all sends to the
dashboard after a full reload, losing the reader's place. The mechanism of the current defect is
`ui/src/components/chat/renderers/AssistantText.tsx` (no `a` override, so an agent's relative link
is an ordinary anchor), `internal/server/spa.go:22`, and `ui/src/routes.tsx:56`.

## Included work

- **The link.** An `a` override in the assistant Markdown renderer opens local-path targets —
  relative, absolute, or `file://`, with an optional `:line`/`:line:col` suffix — in the viewer;
  `http`/`https`/`mailto` are untouched. **No** path detection over plain transcript text: only what
  the agent already marked as a link. (FS-03.R51)
- **The viewer.** Read-only, one file, no tabs and no history: relative path plus Reload and Close
  in the header, line-numbered syntax-highlighted text in the body reusing `CodeBlock`, the cited
  line scrolled to and marked, and a Rendered/Source toggle for Markdown that reuses the
  transcript's sanitized Markdown and diagram rendering through one shared component.
  Read at open and on Reload only — no watching or polling. (FS-03.R52, TS-08.R57)
- **Placement.** A leading track on `.transcript-wrap`'s existing grid, decided by the shipped
  `@container transcript` query: docked left with the chat panel's width cap relaxed via a
  `data-file-open` state attribute when there is room, taking transcript width when there is not.
  The annotation tray keeps the trailing track. Nothing is measured in JavaScript. In an expanded
  dashboard chat pane no viewer opens; a link there navigates to the agent screen with the file
  showing. (FS-03.R53, TS-08.R57)
- **Address.** `?file=` and `?fileLine=` on the agent and archived-agent routes beside the existing
  `?tab=`, as the single source of the open-file state — so a reload reopens, Back closes, and no
  store or persisted browser key is added. (FS-03.R54, TS-08.R57)
- **The read.** `GET /api/sessions/{id}/file?path=` returning JSON, confined to the working
  directory recorded on that agent's session, sharing `internal/server/filesearch.go`'s `withinRoot`
  resolve-and-recheck containment rather than a second copy of it. Refused on path form before any
  filesystem access when it escapes the root or names `.git`; regular files only; bounded byte read
  with a labelled partial result; non-UTF-8 refused. Not gated on a running record, so archived
  sessions work. (TS-03.R40, TS-05.R21)
- **Other entry points.** The Files tab's tracked-path rows and the transcript diff's file heading
  open the same viewer through the same affordance, keeping their existing Copy/Diff and
  line-selection behavior. (FS-05.R37)

Intentionally excluded: editing, creating, renaming, deleting, downloading, or running anything;
directory browsing or listing; multiple tabs, back/forward history, or a file tree; images, PDFs,
and other non-text content; any path outside the session working directory; watching a file for
changes; any agent-facing surface — an agent cannot open the viewer or learn it exists. Git-ignored
files inside the working directory **are** readable, decided on 2026-09-10.

## How we will know it works

- FS-03.A34 — link classification and the untouched `https:` case:
  `ui/src/components/chat/renderers/AssistantText.test.tsx`.
- FS-03.A35 — viewer content, line anchor, replace-on-open, Reload, Markdown toggle:
  `ui/src/components/chat/FileViewer.test.tsx`.
- FS-03.A36 — both layout forms, the dashboard-pane navigation, and the `?file=` round trip:
  `FileViewer.test.tsx`, `DashboardChatPane.test.tsx`, `ChatPanel.test.tsx`, with the rendered forms
  exercised in journey J3.
- FS-03.A37 — every containment refusal and the labelled partial read:
  `internal/server/fileread_test.go` (the adversarial set TS-05.R11 requires) plus the rendered
  reasons in `FileViewer.test.tsx`.
- FS-05.A20 — tracked-file row and diff heading open the viewer with Copy/Diff intact:
  `FilesTab.test.tsx`, `renderers/DiffBlock.test.tsx`.
- Journey J3 carries the rendered file-link steps, including the narrow-window form change and the
  refusal branch.

## Waiting on

Nothing. Every product and boundary decision is settled: link surface (agent-authored links only),
readable root (session working directory only), viewer content, single-file scope, dashboard-pane
navigation, archived-agent support, the `?file=` URL spelling, and Git-ignored readability.
