---
name: design
description: "Use automatically for AgentDeck UI work where visual design judgment is material: new screens, redesigns, meaningful layout or styling changes, motion, visual polish, or critique. Also run when the user sends `/design`. Do not trigger for routine frontend engineering where presentation is incidental, such as data wiring, type fixes, behavior tests, copy-only edits, or style-preserving bug fixes."
---

# Design AgentDeck UI

This is a design-quality companion to the active role, not authority to choose or enlarge the work.
An automatic trigger does not select a future change, replace `/design-feature`, turn a critique into
an implementation, or make routine frontend work a design exercise. `$ARGUMENTS` narrows the surface
or outcome; with no scope and no active UI request, ask what should be designed or critiqued. A
design-only or critique request changes no product code; an implementation request follows the
normal specification-first build workflow.

Read `docs/features/HANDOFF.md`, the governing feature requirements, `FS-12`, `TS-08`,
`ui/AGENTS.md`, the affected UI, and `docs/features/AGENT-WORKFLOW.md` §14. Follow the active role's
rules for specifications, edits, findings, verification, handoff, and the final human update.

Before editing, write a compact direction in the working plan or existing change document:

- the operator, frequent task, surface's single job, and real content/states;
- the first-view thesis, reading order, density, and one product-specific visual or interaction move;
- what the incumbent AgentDeck presentation must preserve and the generic category defaults to avoid;
- whether motion is absent or which feedback, spatial, or lifecycle purpose earns it.

For a standalone `/design` critique, do not create a direction or change document. Use the governing
requirements and rendered incumbent surface as the evaluation direction, report the evidence in the
conversation, and edit no repository file.

For a genuinely open new screen or redesign, present two or three materially different compositions
before implementation; do not stage a concept round for a local extension or settled direction.

Build through the existing tokens, primitives, presentation hooks, feature styles, appearances, and
visual matrix. Use agent lifecycle and coordination state as meaningful visual material, while
keeping repeated operating actions quiet and fast. Do not add a parallel design system, theme
provider, artifact hierarchy, detector suite, or motion dependency for this workflow.

Finish by inspecting the running UI in a real browser at the viewports and states the governing
requirements actually support. Compare Core and Sky & Grove when shared presentation changes, and
exercise reduced motion when motion changes. Critique the rendered result for hierarchy,
specificity, density, state clarity, interaction feedback, accessibility, overflow, and finish;
batch one repair pass, confirm it once, and stop. Source inspection or the visual matrix alone is
not visual verification.
