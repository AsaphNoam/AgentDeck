# Add the Studio skin

**State:** Waiting to start
**Why:** The operator's 2026-09-23 request and approval of a third selectable skin based on the [AgentDeck Figma Make exploration](https://www.figma.com/make/OykxmXqZnnyA67QA1lv3AU/AgentDeck-%25E2%2580%2594-Theme-Exploration?t=H04Rhi3YTK9xsPTq-0).
**Relevant requirements:** FS-12.R42–R45/A18–A20, FS-02.R47/R55/R58/R59, FS-04.R38, TS-02.R21/R36, TS-03.R21/R45, TS-08.R30–R32/R34–R36/R41/R45/R61–R64, INV §2/§8/§10/§11/§13/§17

## Outcome

Settings offers Studio alongside Core and Sky & Grove. Its calm, lively workspace treatment applies across the existing product immediately and durably, while every product action, state, route, and chat behavior stays intact. Core remains the default and fallback; Sky & Grove does not change.

## Included work

Extend the existing finite built-in skin ID, config validation, Appearance option/preview, presentation contract, stylesheet, and three-appearance visual matrix. Studio uses pale blue-green off-whites, forest support, restrained coral action, opaque reading surfaces, and a uniform low-contrast dot canvas. The Make file is visual reference, not component or behavior authority: in particular, reuse the real expanded dashboard `TranscriptView` and `Composer`, never its cramped mockup transcript. Preserve the current grid/density, state meaning, error handling, terminal/syntax/diff behavior, and local-only assets. No theme provider, new preference/route, external skin loading, product-copy change, or replacement of either shipped appearance.

Design direction: the operator repeatedly scans agent state and enters a conversation when attention is needed. Keep the workspace/content dominant over quiet chrome; read name and state first, then current preview, then technical metadata. The expanded card is real chat, not a decorative sample. Use state and hierarchy for salience, not motion; no new lifecycle animation is required. Avoid IDE density, all-equal cards, pervasive pills, gradients, glass, and a dot pattern behind text.

## How we will know it works

FS-12.A18–A20: config and UI tests prove three-way selection, persistence, rollback and fallback; the style contract proves finite IDs, approved hooks/tokens and local assets. The deterministic visual matrix and actual browser review cover Core/Sky & Grove non-regression plus Studio's dashboard extremes, expanded transcript at the supported desktop floor, agent/chat variants, tasks, pipelines, Archive, Settings, onboarding, overlays, permissions and technical renderers. The expanded pane still accepts chat/permission actions and stays readable; neither its content nor neighboring cards clip or shift contrary to the shipped grid contract.

## Waiting on

None.
