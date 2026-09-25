# Complete Studio's UI composition

**State:** Waiting to start
**Why:** The operator said on 2026-09-25 that shipped Studio is only another color scheme; the approved Figma Make direction was intended as an actual interface-design upgrade. The operator confirmed composition changes only, with existing interactions preserved.
**Relevant requirements:** FS-12.R46–R49/A21–A23, FS-12.R42–R45/A19–A20, FS-02.R55/R58/R59, TS-08.R65–R68, TS-08.R41/R44/R45, INV §2/§8/§10/§13/§17

## Outcome

Selecting Studio changes the spatial hierarchy, typography, and surface composition of AgentDeck's core workflows—not just their colors—while Core and Sky & Grove retain their shipped look and all three appearances use the same product behavior.

## Included work

Studio-scoped composition for the shell, Dashboard and agent cards, real expanded chat and full agent workspace, Tasks, Pipeline Runs/Templates, Archive, Settings, onboarding, and overlays. Use the [AgentDeck Figma Make exploration](https://www.figma.com/make/OykxmXqZnnyA67QA1lv3AU/AgentDeck-%25E2%2580%2594-Theme-Exploration?t=H04Rhi3YTK9xsPTq-0) for visual direction and FS-12.R46–R49 for product-shaped screen hierarchy. The first view should make agent state and live work easiest to scan; the transcript and composer must remain a real conversation, not a decorative preview. Existing density, grid stability, route/action/state meaning, validation, error recovery, keyboard flow, and terminal/file/annotation behavior remain. No new workflow, product data, theme provider, external asset, or replacement of Core/Sky & Grove.

Motion decision: none is required. Use quiet state, hover, and focus feedback already present; avoid decorative entrance motion. Avoid all-equal card grids, oversized technical chrome, pervasive pills, gradients, glass, and dots behind readable content.

## How we will know it works

FS-12.A21–A23 are the composition gate: side-by-side rendered Core/Sky & Grove/Studio views at 1024px and a wider desktop viewport must demonstrate non-color differences on every named route, including a desaturated comparison. Review long names, dense/empty/attention states, expanded dashboard chat, live and archived agent views, settings and onboarding; test interaction parity, grid stability, focus, and technical surfaces. The remaining FS-12.A19/A20 visual and live-chat evidence is closed only when observed, not inferred from token tests. A palette/radius/dot-only result fails this change.

## Waiting on

None.
