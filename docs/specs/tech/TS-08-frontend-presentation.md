# TS-08 — Frontend presentation architecture

**Status:** Current
**Code:** `ui/src`, `ui/package.json`, `ui/vite.config.ts`
**Absorbed:** —

## 1. Scope

This specification defines how AgentDeck's confirmed core interface design is represented in the
React/Vite frontend and how that core remains distinct from future optional skins. It owns visual
tokens, cascade order, stylesheet/component boundaries, local visual assets, third-party renderer
styling, stable skin hooks, automated maintenance safeguards, and presentation verification.

It does not own feature state, API/SSE data, routes, persistence, user-selectable skins, skin
discovery/loading, or interaction behavior. Those boundaries remain with TS-01 and the owning
feature specifications. The selected architecture is layered plain CSS with a small presentation-
only React primitive seam; the rejected alternatives are recorded in §5.

## 2. Design & constraints

- **R1** — Presentation is a leaf dependency. Feature components own data, state,
  validation, mutations, Radix behavior, drag behavior, terminal lifecycle, and routing; visual
  primitives and styles may receive those states but may not fetch, persist, or reinterpret them.
- **R2** — The core interface is not implemented as an active/default skin. The
  production document has no skin id or skin provider, the core renders without optional skin code,
  and no skin preference is read or written in this change.
- **R3** — One `styles/index.css` declares and imports these cascade layers in fixed
  low-to-high precedence: `ad-reset`, `ad-tokens`, `ad-base`, `ad-components`, `ad-features`,
  `ad-integrations`, `ad-skins`. The production `ad-skins` layer is empty; declaring it reserves
  precedence without loading a skin. Feature code imports only `index.css` (plus third-party CSS
  whose import contract requires component scope).
- **R4** — Core styles are split by responsibility: foundation/reset and bundled fonts;
  raw and semantic visual values; shared component construction; per-feature composition; and
  explicit third-party adapters. A monolithic replacement `global.css` does not remain as a second
  authority after migration.
- **R5** — Visual values use an `--ad-` namespace and flow one way: raw core palette,
  type, spacing, radius, border, and shadow values → semantic surface/text/action/state values →
  component-local values. Feature styles do not introduce hard-coded colors, font families,
  shadows, radii, or spacing where a declared role applies.
- **R6** — A small `ui/src/components/ui/` layer centralizes only repeated presentation
  markup: button/icon-button variants, field frame, badge, page header, surface, and visually hidden
  label where already needed. Primitives preserve the underlying HTML element, forwarded props/ref,
  accessible name, Radix ownership, and event behavior. This is not a component-framework rewrite;
  one-off feature structure remains in its owning component.
- **R7** — Major shared/feature surfaces expose stable presentation hooks independent of
  implementation class names: `data-ui` names the component, `data-slot` names an intentional
  subpart, and existing or explicit `data-state`/`data-variant` values describe visual state. Hooks
  use product-native names such as `agent-card` and `tool-result`, never a core-design or future-skin
  concept.
- **R8** — `ui/src/presentation/contract.json` is the machine-readable public visual
  contract. It has a schema/version, lists skin-overridable semantic tokens, lists each public
  `data-ui` hook with allowed slots/states/variants, and identifies permitted decorative asset
  slots. Adding, renaming, or removing a public item updates the manifest, its contract tests, and
  TS-08; undocumented hooks are not supported.
- **R9** — Core CSS is complete without hook overrides. A future optional skin may use
  only the manifest's approved semantic values, hooks, and decorative slots from the higher
  `ad-skins` layer; it cannot be required for layout, hide required content/actions, or become a
  source of product copy/state. Skin loading, compatibility negotiation, and trust remain future
  work.
- **R10** — Fonts, icons, marks, and decorative assets required by the core are bundled
  into the Vite build from repository-owned files with recorded licenses. The dashboard makes no
  runtime request to a font, icon, image, stylesheet, or script content-delivery network.
- **R11** — Core typography uses locally bundled Instrument Sans variable font for
  display/text roles and IBM Plex Mono for technical roles, with their SIL Open Font License texts
  kept beside the assets. If implementation evidence makes either font unsuitable, changing it is a
  TS-08 visual-contract change rather than an inline component choice.
- **R12** — The core mark is one repository-owned SVG React component: a simple
  geometric AgentDeck mark plus text wordmark, using `currentColor` and no embedded raster/text
  payload. Other repository-owned icons follow the same seam. Existing visible text and accessible
  names remain feature-owned; an icon never becomes the only programmatic label.
- **R13** — Syntax highlighting, `react-diff-viewer-continued`, and xterm.js do not keep
  independent default palettes. Small adapter modules map the core semantic values into each
  library. A canvas-backed integration that cannot resolve CSS custom properties directly reads the
  computed values through one shared `resolvePresentationColors` helper rather than duplicating
  literals in feature code.
- **R14** — Dynamic values that express real feature data remain inline and narrowly
  scoped: drag transforms, persisted grid columns/gap, context width, context-menu coordinates, and
  project RGB accents. They are listed in the presentation exception manifest, and the automated
  audit rejects any new inline presentational literal without a path, rule, and reason.
- **R15** — The redesign does not move Zustand/React Query ownership, change route
  composition, replace Radix behavior primitives, alter the terminal WebSocket, or modify API/SSE
  contracts. Existing feature tests remain behavioral regression gates.
- **R16** — Production continues to use the Vite output embedded by TS-06.R3. Core
  assets are content-hashed by Vite and included by `make embed`/`make dist`; source files under
  `ui/src` are the only hand-edited visual source.

### 2.1 Maintenance safeguards

- **R17** — Stylelint and a repository-owned dependency-light contract checker run as
  `npm run check:styles`. NPM `pretest` and `prebuild` both invoke it, so UI tests, CI, `make embed`,
  and `make dist` cannot bypass presentation validation. Tool versions are pinned in the UI lockfile.
- **R18** — Stylelint rejects invalid CSS, duplicate properties/selectors where unsafe,
  id selectors, unbounded specificity, `!important`, unknown custom properties, and rules outside
  the declared cascade layers. Narrow third-party exceptions live in the machine-readable exception
  manifest with a reason; blanket file or rule-family suppression is prohibited.
- **R19** — `ui/scripts/check-presentation-contract.mjs` checks both TSX and CSS and
  fails on: a literal class without a selector (INV §13); a referenced `--ad-` value without one
  definition; an unused public token; raw color/font/shadow/radius/spacing values outside their
  allowed source; an inline visual literal outside R14; a `data-ui`/slot/state not present in the
  contract; a manifest entry with no implementation; core CSS dependent on `[data-skin]`; or a skin
  rule outside `ad-skins`.
- **R20** — `ui/presentation-exceptions.json` is the only audit escape hatch. Every entry
  names an exact file and rule, states the non-visual/data-driven or third-party reason, and is
  rejected when its target no longer exists or no longer violates the rule. A new exception is a
  conscious contract change, not an inline disable comment.
- **R21** — `ui/AGENTS.md` summarizes the presentation dependency direction, token
  decision tree, stable-hook rules, exception policy, prohibited skin state/provider work, required
  checks, and the need to read FS-12/TS-08. It is created before surface migration so every later
  coding agent receives the rules while editing beneath `ui/`.
- **R22** — A development-only visual matrix renders representative core components and
  feature surfaces from deterministic fixtures without calling provider CLIs or mutating user
  state. It is unreachable and absent from production routing/bundles, and supplies repeatable real-
  browser review input for FS-12.A1–A5.
- **R23** — A contract fixture applies deliberately high-variance test values and
  hook-scoped decoration in `ad-skins` to prove the seam without shipping a skin, selector,
  provider, preference, or production skin asset. The test asserts unchanged product copy, DOM
  order, actions, routes, state values, and feature-test behavior.
- **R24** — Migration proceeds in behavior-preserving slices: contract/checker and local
  agent guide first; foundation/tokens/assets/primitives second; then shell, Dashboard, agent screen,
  Archive, Settings, onboarding/overlays, and integrations. Each slice removes superseded selectors,
  passes the style contract plus affected tests/build, and does not combine a feature/state refactor
  with visual migration.
- **R25** — Completion requires zero stale legacy authority: no imported
  `global.css`, no unexplained raw visual values, no literal class without a selector, no unreferenced
  public hook/token, no third-party default palette, and no production skin state, attribute,
  stylesheet, or asset.

## 3. Interfaces & data shapes

### 3.1 Cascade and file contract

```css
/* styles/index.css — declarations/imports shown logically */
@layer ad-reset, ad-tokens, ad-base, ad-components,
       ad-features, ad-integrations, ad-skins;
```

```text
ui/src/
  assets/fonts/                  bundled core fonts + licenses
  components/shell/AgentDeckMark.tsx
  components/ui/                 small behavior-transparent presentation primitives
  presentation/
    contract.json                versioned public visual tokens/hooks/slots/states
    integrations.ts              syntax/diff/xterm adapters
    resolveColors.ts             computed colors for canvas-backed integrations
  styles/
    index.css                    sole entry + cascade order
    foundation.css              reset, @font-face, element baseline
    tokens.css                  raw + semantic core values
    base.css                    shell-independent document/content rules
    components/*.css            shared presentation primitives
    features/*.css              surface composition
    integrations.css            third-party DOM adapters
  scripts/check-presentation-contract.mjs
ui/presentation-exceptions.json
ui/AGENTS.md
```

The final exact subdivision inside `components/` and `features/` follows ownership; a file must not
become another cross-product catch-all.

### 3.2 Core value contract

The implementation pins these starting values in `tokens.css`; semantic aliases, not raw names, are
the future-skin contract.

| Role | Value |
|---|---|
| Canvas / surface / raised | `#f2f0e9` / `#faf9f5` / `#ffffff` |
| Ink / text / muted | `#15171a` / `#25282d` / `#686d73` |
| Line / strong line | `#cbc7bd` / `#25282d` |
| Primary accent / secondary accent / highlight | `#ff5a36` / `#2457f5` / `#dfff4f` |
| Busy / idle / waiting / done / error / unknown | `#c97900` / `#66717a` / `#2457f5` / `#16845b` / `#c93636` / `#93989d` |
| Technical background / surface / text / muted | `#171a1f` / `#22262d` / `#f2f0e9` / `#aab0ba` |

Spacing uses a 4px base with named steps `1, 2, 3, 4, 6, 8, 12` (4–48px). Core radii are 2, 6,
10, and 16px; signature surfaces use an asymmetric small corner rather than uniform pill geometry.
Borders are 1px/2px. Shadows are crisp and bounded (`1px` keyline or `4px` offset) rather than
diffuse floating-card shadows. Gradients, `backdrop-filter`, and decorative glow are not core values.

### 3.3 Stable presentation hooks

```html
<article data-ui="agent-card" data-state="waiting_input">
  <header data-slot="header">...</header>
  <div data-slot="metadata">...</div>
  <div data-slot="context">...</div>
</article>
```

The exact hook list is curated in `contract.json` from the surfaces named by FS-12; arbitrary
descendants are not public skin hooks. Component state comes from existing props/Radix data
attributes and is never parsed back from CSS.

The dependency direction is:

```text
bundled fonts/assets
        +
raw core values → semantic values → shared component construction → feature composition
                                                              ↘ third-party renderer adapters

future skin (not shipped): approved semantic overrides + scoped hook rules + decorative assets
```

### 3.4 Contract/exception manifest shapes

```jsonc
{
  "version": 1,
  "tokens": ["--ad-surface-canvas", "--ad-text-primary"],
  "components": {
    "agent-card": {
      "slots": ["header", "metadata", "context"],
      "states": ["busy", "idle", "waiting_input", "done", "error", "unknown"]
    }
  },
  "decorative_slots": ["app-mark"]
}
```

```jsonc
[
  {
    "file": "src/components/grid/ContextBar.tsx",
    "rule": "inline-style",
    "reason": "Width is live context usage data, not presentation configuration"
  }
]
```

Manifests are declarative build/test inputs only; production does not fetch or interpret them.

## 4. Invariants

- **INV §3 — Forms merge, never replace.** Styling/extracting primitives around seeded config forms
  cannot change submit timing, default normalization, or merge-preserve behavior.
- **INV §8 — Errors surface.** Visual restructuring cannot swallow or detach mutation errors from
  the controls that currently surface them.
- **INV §9 — PTY framing.** Terminal presentation changes never modify xterm's binary-keystroke /
  text-resize WebSocket contract.
- **INV §10 — Ship every promised surface.** The visual matrix and browser review cover every
  FS-12 surface, including dense settings and third-party renderers.
- **INV §11 — Null-hostile collections.** Presentation primitives do not weaken API-boundary
  normalization or mocks.
- **INV §13 — Every literal class resolves.** R17–R20 automate and extend the existing binding rule;
  visual tests supplement rather than replace it.
- **R26** — Presentation code has no authority over product state. Removing every
  future skin override and every decorative asset must leave the core application structurally
  complete and behaviorally unchanged.
- **R27** — There is exactly one definition path for each public token and hook. A
  second token file, parallel component theme object, ad-hoc provider, or undocumented override
  mechanism is architecture drift and fails the contract checks where mechanically detectable.
- **R28** — Maintenance checks are part of the delivery contract, not optional review
  guidance. An implementation is incomplete if a rule is documented but neither automated nor
  explicitly identified as a browser-only visual check.

## 5. Deviations & open decisions

- **Selected architecture: layered plain CSS.** CSS Modules plus a React provider were rejected
  because hashed implementation classes weaken rich skin overrides and a provider makes the core
  resemble a default theme. Runtime CSS-in-JS was rejected because it adds dependency/runtime cost
  and a broad rewrite without current runtime skin behavior. Reversing this choice is a TS-08
  architecture change.
- Skin selection, persistence, discovery, packaging, compatibility/version negotiation, third-party
  skin trust, and arbitrary skin code are future features. The version-1 manifest is an internal
  compatibility seam, not a promise that external skins can be loaded today.
- Browser pixel-diff infrastructure is not added by this change. Deterministic visual fixtures,
  browser screenshots, the contract fixture, style lint, and existing behavior tests provide the
  acceptance evidence; adopting stored pixel baselines can be designed separately if manual visual
  comparison becomes unreliable.

## 6. Traceability

- Entry/build and cascade authority: `ui/src/main.tsx`, `ui/vite.config.ts`, `ui/package.json`,
  `ui/src/styles/index.css`, TS-06.R3–R5.
- Core visual source: `ui/src/styles/{foundation,tokens,base,integrations}.css`,
  `ui/src/styles/components/`, `ui/src/styles/features/`.
- Shared construction, public hooks, and local assets: `ui/src/components/ui/`,
  `ui/src/presentation/contract.json`, `ui/src/assets/`.
- Third-party renderers: `AssistantText.tsx` (`react-syntax-highlighter`), `DiffBlock.tsx`
  (`react-diff-viewer-continued`), `TerminalTab.tsx` (xterm.js),
  `ui/src/presentation/{integrations,resolveColors}.ts`.
- Data-driven inline styles retained by R14: `AgentCard.tsx`, `CardGrid.tsx`, `ContextBar.tsx`,
  `CardContextMenu.tsx`, `ProjectForm.tsx`, `ProjectsEditor.tsx`.
- Maintenance contract: `ui/AGENTS.md`, `ui/presentation-exceptions.json`,
  `ui/scripts/check-presentation-contract.mjs`, `ui/scripts/check-presentation-contract.test.mjs`,
  `ui/stylelint.config.mjs`, `ui/scripts/stylelint-config.test.mjs`.
- Deterministic visual evidence: `ui/src/presentation/VisualMatrix.tsx`,
  `ui/src/presentation/contract-fixture.css`, `ui/src/presentation/VisualMatrix.test.tsx`; the route
  is development-gated in `ui/src/routes.tsx` and absent from production bundles.
