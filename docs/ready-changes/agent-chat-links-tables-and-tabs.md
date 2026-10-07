# Agent chat links, readable tables and fewer tabs

**State:** Waiting to start
**Why:** Direct `/design-feature` request on 2026-10-07; proposed scope approved by the user.
**Relevant requirements:** FS-03.R78–R80/A59–A61, FS-05.R40/A23, FS-12.R60/A32,
FS-20.R42/A14, TS-08.R107–R110, INV §2/§8/§10/§13

## Outcome

Web links open in new tabs and offer Open in new tab/Copy link on right-click. Markdown tables
are easier to read. Agent conversations no longer offer Commands, including on phones.

## Included work

Keep local-file links in Chuck's viewer and selection/annotation actions available. Add faint
row dividers, increased column spacing and contained wide-table scrolling across appearances,
chat surfaces and the shared rendered file viewer. Remove the Commands view while preserving
tracking, APIs, archive counts and transcript activity. No stored-data, protocol or phone
file-authority change, replacement command screen, new Markdown dialect or renderer migration.

Renderer audit completed before document/Markdown changes: `ui/package.json` already pins
react-markdown 9.0.3, remark-gfm 4.0.1 and rehype-sanitize 6.0.0; `SanitizedMarkdown.tsx` wires
them together. [remark-gfm](https://github.com/remarkjs/remark-gfm) and
[react-markdown](https://github.com/remarkjs/react-markdown) document semantic GFM tables.
The current styles lack table-cell spacing/dividers; parser replacement would not address that.
`TranscriptView`'s annotation handler suppresses link right-clicks, and the shared renderer's
web anchors lack a new-tab target. Extend those existing seams.

Design direction: experienced operators reading agent output need to follow references without
losing their conversation, scan technical tables and reach remaining chat views quickly. Keep
the transcript as the focal reading surface, quiet token-based table boundaries, existing menu
language and no new motion. Local-file viewing and command-tool activity are the Chuck-specific
continuity to preserve. This design has source evidence; rendered acceptance remains for implementation.

## How we will know it works

FS-03.A59–A61 cover browser link actions, copy failure/annotation regressions, table alignment,
spacing and local overflow, and every affected tab surface. FS-05.A23 verifies tracking retention;
FS-20.A14 covers phone views; FS-12.A32 covers all appearances. TS-08.R110 names closure checks.

## Waiting on

Nothing. Specifications are approved in scope and remain planned until implementation.
