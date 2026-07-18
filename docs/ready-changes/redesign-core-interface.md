# Redesign the core interface

**State:** Waiting to start
**Why:** The current frontend is functional but visually bare-bones; the human requested a complete,
distinctive product-native design and selected the layered plain-CSS architecture for a future-skin
seam with strong unattended-maintenance safeguards.
**Relevant requirements:** FS-12.R1–R25, FS-12.A1–A7, TS-08.R1–R28, INV §3, INV §8, INV §9,
INV §10, INV §11, INV §13

## Outcome

Every existing frontend surface will share one distinctive unskinned AgentDeck core design, and
future visual skins will have a controlled token/component-hook seam without adding any current skin
behavior. Automated contract checks and local agent guidance will prevent routine later changes from
silently bypassing that seam or leaving surfaces unstyled.

## Included work

Implement the layered stylesheet/token contract, bundled fonts/mark/icons, small presentation-only
primitives, stable manifest-backed hooks, third-party renderer adapters, complete visual migration
of shell/Dashboard/agent screen/Archive/Settings/onboarding/overlays, deterministic visual fixtures,
Stylelint and the cross-TSX/CSS contract checker, exception manifest, and `ui/AGENTS.md`. Preserve all
existing feature behavior. Do not add responsive/zoom/keyboard/accessibility work, new recovery
states, browser-dialog replacements, or any skin selection/loading/persistence.

## How we will know it works

FS-12.A1–A7 pass through the deterministic visual matrix and real-browser journeys, the high-
variance contract fixture changes presentation without changing behavior/structure, style and hook
audits are mandatory through NPM pretest/prebuild, all affected UI tests/build pass, and the shared
specification/build/distribution checks remain green.

## Waiting on

None.
