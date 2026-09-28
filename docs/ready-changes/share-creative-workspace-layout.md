# Finish and share the creative-workspace layout

**State:** In progress
**Why:** Operator requested Figma/Studio parity and the improved layout across all appearances on
2026-09-28, added active-project tabs and prominent paced status badges, then approved the scope.
**Relevant requirements:** FS-12.R52–R59/A26–A31; TS-08.R74–R79; INV §2/§8/§10/§13/§17.
Preserve FS-12.R39/R44/R50–R51, FS-02.R55/R61 and TS-08.R41/R44–R45/R53/R57/R69–R72.

## Outcome

Core, Sky & Grove and Studio use one polished creative-workspace layout while retaining their
palettes and canvas ornaments. This is an actual composition/control/depth upgrade, not another
color scheme or a fourth skin.

## Included work

Finish dashboard card controls, content-sized expanded headers and soft resting depth; full-agent
header/chat/composer measures; Tasks inset padding and rhythm; readable active-project tabs; and
prominent badges with slow busy/faster error or waiting-input pulses and reduced-motion fallback.
Promote the improved shell and remaining route composition into shared styles. Keep real chat,
grid density/stability, task/runtime semantics, preference persistence, routes and API unchanged.
No synthetic transcript, invented mockup action, new blocked-state enum, legacy-layout toggle,
provider, external asset or motion dependency.

## Design grounding

Frequent operators should find the active agent and anything needing intervention at a glance,
then read or reply with minimal chrome. Quiet groups, friendly readable typography, bounded prose
and opaque surfaces keep content dominant; status badges are deliberately the stronger scan cue.
Motion indicates current state only: no card movement or celebratory animation.

Reference: [AgentDeck Figma Make exploration](https://www.figma.com/make/OykxmXqZnnyA67QA1lv3AU/AgentDeck-%25E2%2580%2594-Theme-Exploration).
Compare composition, not prototype anatomy. The pre-design live comparison showed 640px expanded
cards with ~253px headers, ~78px Send buttons, zero Tasks authoring inset padding and boxed runtime
groups with an empty live-settings band. These are baseline observations, not acceptance evidence.
Source seams are the existing feature styles, `Composer`, `AgentCard`, `StateBadge`,
`ActiveProjectNav`, `TasksPage` and public presentation contract; no missing-provider workaround
is needed. Recheck the implementation revision because compact setup/header work already shipped.

## How we will know it works

FS-12.A26–A31 are the closure gates. Use matched three-appearance fixtures and actual browser
screenshots at confirmed 1024px and wider desktop; populated active/archive chat, Tasks state rows,
five-plus project tabs, long identities and runtime controls must be represented. Measure expanded
headers/action geometry, inspect resting shadows in color, and observe pulse timing/transitions and
reduced motion; DOM-only checks and palette-only differences do not pass. Use focused existing
behavior tests and the TS-06 applicable closure checks with generated embed.

Prior Studio review units and acceptance debt remain separate. This change supersedes only the
old-layout preservation and Studio-only layout-distinction clauses identified in the new FS/TS;
it must not silently mark older evidence as passed. Record closure evidence so review has the
revision, viewport and route/state context.

## Waiting on

Implementation and automated closure are committed. The operator approved the remaining checks;
browser Send/Cancel now passes in all appearances. Reduced-motion verification needs the Mac
unlocked for System Settings because macOS refused the direct preference write. See the live handoff and the
[checkpoint evidence](../archive/reviews/implementation-share-creative-workspace-2026-09-28.md).
