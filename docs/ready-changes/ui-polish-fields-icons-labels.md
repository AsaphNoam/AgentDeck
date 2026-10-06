# UI polish: auto-grow fields, icon actions, plain labels

**State:** Waiting to start
**Why:** Human request of 2026-10-06 (five small UI items), merged with the `docs/ideas.md` entry
"Finish hiding raw ids in UI labels" (pipeline template select always rendered `title (id)`).
**Relevant requirements:** FS-02.R66–R70, FS-02.A48–A52, TS-08.R88–R91, INV §2, INV §13

## Outcome
Text fields grow to fit their content with no resize grip; Send, Cancel, Collapse and Collapse all
are icons; an expanded project-page card puts its badge and Collapse top-right above the context
meter; labels read as plain names; the Tasks page hides archived projects.

## Included work
- Shared `AutoGrowTextarea` for every textarea; composers capped near 40% of the window.
- Inline SVG icons (no dependency); Steer and Withdraw queued stay text.
- Expanded card header reorder.
- One shared duplicate-gated label helper (lifted from `NewAgentModal.displayLabel`); `(archived)`
  becomes a tag.
- Tasks page all-projects list and filter skip archived projects; an explicit focus still works.

Excluded: API, persistence, and phone surfaces.

## How we will know it works
FS-02.A48–A52, plus rendered screenshots of the composer and expanded card in each skin.

## Waiting on
Nothing.
