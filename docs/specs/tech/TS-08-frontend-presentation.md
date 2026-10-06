# TS-08 — Frontend presentation architecture

**Status:** Partial
**Code:** `ui/src`, `ui/package.json`, `ui/vite.config.ts`
**Absorbed:** —

## 1. Scope

This specification defines how Chuck's confirmed core interface design is represented in the
React/Vite frontend and how that core remains distinct from future optional skins. It owns visual
tokens, cascade order, stylesheet/component boundaries, local visual assets, third-party renderer
styling, stable skin hooks, automated maintenance safeguards, and presentation verification.

It does not own feature state, API/SSE data, routes, persistence, or interaction behavior. R30–R36
own the presentation-side activation of the first bundled selectable skin; FS-04,
TS-02, and TS-03 own its preference and wire shape. External skin discovery/loading remains out of
scope. The selected architecture is layered plain CSS with a small presentation-only React
primitive seam; the rejected alternatives are recorded in §5.

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
  geometric Chuck mark plus text wordmark, using `currentColor` and no embedded raster/text
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
- **R29** — First-party UI does not call `window.prompt`/`confirm`; those input and
  confirmation flows use the core Radix Dialog convention that FS-12.R26 mandates.
  `ui/scripts/check-presentation-contract.mjs` (R19) gains a rule that fails on any browser-native
  prompt/confirm call under `ui/src`, including bare, `window`/`globalThis`, computed-property, and
  statically aliased references; it exempts only the development visual-matrix and test files, and
  the presentation-exception manifest (R20) is the sole escape hatch. The dialogs reuse existing
  feature-owned Radix behavior, form validation, and the
  backend-catalog, project-color, and client-derived group-label sources (R1); they add no API, SSE,
  persistence, or route change (R15), so no new server endpoint or migration is introduced.

### 2.2 Built-in skin selection

- **R30** — Built-in appearance selection uses no React theme provider,
  runtime CSS-in-JS engine, dynamic stylesheet loader, or browser-storage shadow preference. One
  application-level `AppearanceRoot` reads the existing React Query configuration projection and
  sets `data-skin="sky-grove"` on `document.documentElement`; Core removes the attribute. Core is
  the first paint while configuration is loading or failed, and a later config result or poll may
  change the marker without remounting routes. This is the narrow exception that supersedes
  R2's prohibition on production skin state while keeping Core structurally unskinned.
- **R31** — `styles/skins/sky-grove.css` is statically imported by
  `styles/index.css`, declares every rule inside `ad-skins`, and scopes every semantic-token or hook
  override to the documented `sky-grove` id. The CSS ships in the ordinary Vite bundle and makes no
  network request. Private `--ad-sky-grove-*` raw palette values may be declared once at `:root`
  inside that file so the inactive Settings preview can reuse them; they affect the application only
  when the documented root selector maps them to public semantic tokens.
  `presentation/contract.json` advances to version 2 and adds the finite built-in skin-id list;
  Core is represented by the absence of a skin marker, not by a parallel Core skin.
  This supersedes only R3/R9/R25's empty-production-layer statements; the cascade order, one CSS
  entry, complete Core fallback, approved-token/hook boundary, and stale-authority rules remain.
- **R32** — The config query cache is the sole browser projection of the
  durable preference. `AppearanceRoot` derives an effective id through a finite frontend allowlist
  that the contract checker verifies against the manifest and never makes its own request. The
  Settings mutation optimistically updates that same cache so
  selection is immediate, rolls the cache and root marker back on failure, surfaces the existing
  mutation error, and invalidates/refetches on success. Missing, empty, unknown, malformed, or
  failed configuration produces Core; Settings shows the server warning, unknown raw id, or query
  error instead of crashing the shell. No appearance value enters Zustand, component props, routes,
  project/session data, or launch composition.
- **R33 — retired 2026-08-01:** The initial Sky & Grove palette, diffuse depth, `6px`/`12px`/`20px`
  geometry, and card-leaf ornament are superseded by R38 after visual review found that combination
  washed out hierarchy and displaced a semantic agent-state cue.
- **R34** — The presentation checker accepts a production `data-skin` selector
  only inside `ad-skins`, rejects undocumented ids, rejects a skin rule outside the declared skin
  stylesheet/layer, and still rejects Core CSS that depends on a skin marker. It proves each manifest
  skin has production CSS, each skin public-token override is approved, Core retains exactly one
  definition of every public token, the Settings appearance variant has matching hooks/selectors,
  and the production bundle never imports the development fixture or a network URL. Private skin
  palette tokens are allowed only in the matching declared skin file and must be used by its active
  mapping or preview. The local UI agent guide is updated from its old absolute prohibition to this
  finite built-in-skin rule. This supersedes the
  production-skin bans in R19/R21/R23 only as explicitly described.
- **R35** — Syntax and diff renderers continue to consume live CSS custom
  properties and therefore follow the root marker without special state. The canvas-backed xterm
  adapter observes changes to the root `data-skin` attribute through one shared presentation helper,
  re-resolves computed colors, and assigns the existing terminal instance's theme without closing
  its WebSocket, recreating the terminal, changing dimensions, or losing content. The observer is
  disconnected during the existing terminal cleanup.
- **R36** — The development visual matrix can render paired Core and Sky &
  Grove fixtures from the same deterministic feature data, including the Appearance control and all
  FS-12.A10 surfaces. Its tests assert that selection changes only the root marker/presentation,
  behavior tests cover optimistic success/failure and polling changes, and real-browser review
  compares both appearances at the existing desktop floor. No stored pixel-baseline system is added.
- **R37** — The six preset project accents (FS-04.R39) are defined once as a single
  frontend source-of-truth constant — the canonical palette `Slate [100,116,139]` (default),
  `Blue [59,130,246]`, `Green [34,197,94]`, `Amber [245,158,11]`, `Rose [244,63,94]`,
  `Violet [139,92,246]`. No accent id becomes a CSS selector or a `contract.json`/skin entry: the
  swatches and the resulting card accent render through the existing inline project-accent data
  exception (R14), and the palette is not skin-overridable. The project card context menu is a
  cursor-positioned portal that reuses the agent menu's `context-menu` hook, portaling, and dismissal
  (FS-02.R38) and the Radix dialog convention (R29) for its Rename/Archive dialogs; the inline swatch
  picker adds no `window.prompt`/`confirm` call. Server-side color validation and the `/api/projects`
  shape are unchanged. Beyond the existing inset left-edge accent (retained), the accent is composited
  into each card's surface and border with `color-mix`, mirroring the existing
  `--ad-surface-subtle`/`--ad-surface-emphasis` derivations: a background wash of ~9% accent into
  `--ad-surface-panel` and a border of ~50% accent into `--ad-border-strong`, both derived from the
  single `--ad-project-accent` value so no new per-card inline literal is added (R14). The agent-state
  top bar is unaffected, and the mix proportions are bounded to preserve body-text contrast in Core
  and every skin (FS-02.R40).
- **R38** — Sky & Grove defines its raw palette once in its skin stylesheet and maps it to
  the existing public semantic tokens. Its reworked visual contract is:

  | Role | Value |
  |---|---|
  | Canvas / panel / raised | `#e8f4f8` / `#f4fafb` / `#ffffff` |
  | Primary / secondary / muted text | `#17332f` / `#34524e` / `#5c716e` |
  | Default / strong border | `#bed5d9` / `#3a675f` |
  | Primary action / secondary action / highlight | `#2f7058` / `#287a9b` / `#d5e9c6` |
  | Busy / idle / waiting / done / error / unknown | `#b46b1e` / `#687d7a` / `#287a9b` / `#2f7d5c` / `#b8454d` / `#829491` |
  | Technical background / surface / text / muted | `#102927` / `#183734` / `#eff8f6` / `#abc5c0` |

  The skin retains the bundled Core fonts and uses `4px`/`9px`/`14px` public
  small/medium/large radii, controlled one- and two-stage shadows, blue-white surface layering,
  evergreen structure, and low-contrast CSS-only contour/canopy linework on approved hooks. Skin
  decoration never replaces an agent state strip or another semantic-state cue. The skin introduces
  no glass, glow, decorative text, semantic-color alias, or asset that carries product meaning.
  Compact Core/Sky & Grove previews reuse the Core raw values and Sky & Grove raw values respectively
  rather than duplicating palette literals.
- **R39** — The transcript's presentation-only tool-run projection has one documented
  `tool-run` hook with `trigger` and `content` slots and collapsed/expanded states. It groups only
  normalized events already supplied by the feature owner, retains the original event nodes when
  expanded for their existing annotation behavior, and neither fetches, persists, folds, nor
  changes transcript data.

- **R40** — The transcript diagram renderer (FS-03.R37) joins the existing
  third-party integration seam of R13 rather than introducing a second styling path: one adapter
  module beside `syntaxTheme` and `xtermTheme` maps the core semantic `--ad-*` values into the
  library's theme, so the library keeps no independent default palette and diagrams follow Core and
  every skin. Because the library resolves its own colors while generating markup, the adapter reads
  computed values through the shared `resolvePresentationColors` helper rather than duplicating
  literals, and each mounted diagram reuses `observePresentationColors` to regenerate when the root
  appearance marker changes rather than introducing another theme signal. The library is a
  repository-owned bundled dependency loaded through a dynamic import, so
  it forms its own build chunk, never enters the initial bundle, and makes no content-delivery-network
  request (R10). Its version is pinned at or above the release that fixes the known diagram-source
  HTML-injection defect. A fixed host-owned 50,000-code-unit check runs before the library, and the
  pinned Mermaid external-image node grammar is rejected at that same preflight so it cannot perform
  its eager URL load; this is a deliberately unsupported Mermaid feature, not a second parser.
  Renderer initialization is host-owned and diagram directives cannot weaken those limits or enable
  interaction. Diagram markup reaches the DOM through one seam that disables the library's
  interactive features and sanitizes the generated markup with a directly declared DOMPurify
  dependency before insertion; that seam is the only place in `ui/src` permitted to insert
  renderer-produced markup, and it is recorded in the presentation exception manifest with its
  path, rule, and reason (R14, R17–R20). Markup the library generates at runtime is outside the
  static audit's reach, which is why the preflight and sanitizing seam, not the audit, are the
  controls. The same seam removes Mermaid's intrinsic root-SVG width cap after sanitization, while
  the integration stylesheet gives the figure the transcript's available width and bounds the SVG
  height; this keeps compact diagrams readable without letting wide or tall diagrams escape the
  chat surface. The react-markdown component map stays referentially stable for as long as a
  message stays mounted, including across the streamed deltas that extend its text, so neither a
  transcript scroll-state rerender nor a later delta remounts a settled diagram or repeats
  main-thread Mermaid work. The map therefore reads the current message text through a ref rather
  than closing over it. No elapsed-time timeout is claimed: main-thread Mermaid work is not interruptible, and
  adding an isolation runtime without evidence that the fixed input bound is insufficient would be
  disproportionate machinery.

### 2.3 Expanded chat panes on the card grid

- **R41** — **The dashboard chat pane composes the shipped chat surface;
  it does not become a second one.** The pane renders the existing `TranscriptView` and `Composer`
  components, which are already fully `agent_id`-parameterized and already own a per-instance scroll
  container and per-agent draft, autocomplete, and annotation state. Folding, follow-scroll,
  optimistic prompts, permission decisions, and diagram rendering are therefore the same code paths
  the agent screen uses, through the already-registered `foldTranscript` / `appendRenderedEvent`
  projection (INV §2). The pane adds no parallel transcript projection, draft store, or permission
  client, and `/agent/:id` keeps composing the same two components alongside the tabs, context
  meter, and runtime picker the pane omits (FS-03.R39).

- **R42** — **Pane geometry is grid-native and order-preserving.** An
  expanded card is a grid item spanning `min(2, perRow)` tracks of the existing
  `repeat(perRow, minmax(0, 1fr))` template (FS-02.R13/R47). `.card-grid` must set
  `align-items: start`: it currently declares only `display: grid`, so the default `stretch` would
  inflate every collapsed card sharing the pane's row to the pane's height. `grid-auto-flow` stays
  `row` and must never become `dense`, because dense packing reorders items visually and FS-02.R47
  requires that expanding never changes card order. A span that does not fit the row's remaining
  tracks wraps to the next row and leaves a gap; that gap is the accepted cost of preserving order.
  The pane has a fixed height, and its transcript is the only region that scrolls the conversation:
  streamed output moves the transcript, never the card, the grid, or the page, and the card's
  existing `overflow: hidden` continues to clip. The reused chat surface also brings two bounded,
  transient scrollers of its own — `.annotation-tray-body` and the `.composer-picker` popover
  (`ui/src/styles/features/agent.css`) — which are unaffected by this rule and keep their own
  behavior. An expanded card
  drops `.agent-card`'s `cursor: pointer` and hover-lift transform, which read as "this whole thing
  is a button" on what is now a reading and typing surface.

- **R43** — **Expansion joins the existing sortable and hook contracts
  rather than adding new ones.** An expanded agent id **stays** in the list handed to its block's
  `SortableContext`. This corrects the requirement as first written, which said the id was removed
  by the same filter that omits a collapsed section's cards: the shipped grid keeps it, because an
  expanded pane still mounts a sortable node and still occupies a wider-or-taller footprint than a
  collapsed card, so omitting it made every neighbour's measured-rect transform compute over a
  layout that is not on screen (FS-02.R47). Undraggability comes from `useSortable({ disabled })`
  and from withholding the drag grip, not from leaving the list. Each running/stopped block still
  gets its own `SortableContext` in render order, so dnd-kit's indices and measured-rect transforms
  keep matching the list the grid actually renders — the constraint FS-02.R45 already established. The expanded form is exposed through the curated
  contract as a `data-variant` on the existing `agent-card` component plus one named pane
  `data-slot`, added to `contract.json` in the same change; arbitrary pane descendants are not skin
  hooks (R14, §3.3/§3.4). Every className the pane ships has a defined selector in
  `ui/src/styles/features/dashboard.css` in the same change, because the build and Testing Library
  are both blind to CSS (INV §13). The pane's focus-cycling shortcut (FS-02.R50) is one keydown
  handler bound to the grid container rather than `window`, so it is scoped to focus inside the grid
  and cannot intercept keys for a dialog, context menu, or any other route.

  FS-02.R52's activation boundary is structural, not a set of exemptions. `AgentCard` today puts
  `onClick` and `onContextMenu` on its outer `<article>`, and the pane is composed inside that
  element, so every chat control the pane reuses would otherwise bubble a collapse and a card context
  menu out of an ordinary Send, permission decision, or autocomplete accept. The handlers therefore
  move to the card's header region for an expanded card rather than each pane control calling
  `stopPropagation` — an opt-out list is exactly the drift INV §2/§10 describe, because every control
  added to the pane later would have to remember to join it, and a missed one fails silently in a way
  no existing test would catch.

### 2.4 Active-project shell navigation

- **R44** — The active-project navigation required by FS-02.R54 and FS-12.R39 is one
  feature-owned `ActiveProjectNav` composed by `Header`. It consumes the existing `useProjects`
  React Query projection, the complete `useAgentStore` projection, and the current React Router
  match; it issues no request of its own and adds no Zustand field, layout/config value, browser
  storage, server shape, or persistence. A pure derivation sorts eligible configured projects by
  displayed title with project id as tie-breaker, adds the current configured non-archived project
  derived from `/project/:id` or the `/agent/:id` store row, and returns at most five visible entries
  plus the alphabetized remainder. If the current project is outside the first five, it replaces the
  fifth entry and both returned sets are re-sorted. Derivation runs from the current projections on
  every relevant change, so hydration pruning, route changes, stop events, and project archival
  cannot leave a retained navigation copy (INV §1/§2).

  `Header` keeps the primary links as primary navigation and appends this compact secondary group
  before the connection indicator. `shell.css` owns a four-region, non-wrapping header composition
  that fits the mark, five primary links, five compact project links, an overflow trigger, and the
  connection indicator at the 1024px desktop floor. Compact project links have a bounded width,
  ellipsized visible title, full accessible title, restrained `--ad-project-accent` tint/edge, and a
  structural selected marker independent of color. The accent is the existing project RGB custom
  property and receives one exact `ActiveProjectNav.tsx` inline-style exception under R14; no new
  token, public hook, skin selector, or presentation contract version is introduced. Core and Sky &
  Grove therefore use the same semantic construction.

  More than five entries render a locally controlled `+n` disclosure button and an absolutely
  positioned list of ordinary project route links. The disclosure closes on selection, Escape, and
  outside pointer press, preserves normal Tab/Enter link navigation, and has no new menu or motion
  dependency. A project-query initial load or failure with no cached catalog renders no secondary
  group and leaves every primary link and the connection indicator operational. The fixed cap is
  the complete overflow algorithm: no `ResizeObserver`, element measurement, recency state,
  breakpoint-specific item count, or second navigation row is added.

### 2.5 Grid stability, collapse controls, and card name legibility

- **R45** — **The pane occupies one track, and the grid template is
  what guarantees it.** FS-02.R55 is implemented by spanning a single column of the existing
  `repeat(perRow, minmax(0, 1fr))` template instead of `min(2, perRow)`, which makes an expanded
  card's grid area identical to a collapsed card's. Auto-placement then assigns every other card the
  same cell it held before the expansion, so no card can wrap to another row and the wrap-and-gap
  R42 accepted disappears with the two-track span; R42's remaining rules stand — `align-items: start`
  stays, `grid-auto-flow` stays `row` and never becomes `dense`, the pane keeps its fixed height and
  its internally scrolling transcript, and the card keeps `overflow: hidden`. The accepted cost
  moves: the pane's grid row is as tall as the pane, so collapsed cards sharing that row sit at its
  top with empty space below them. Packing them into that space requires either `dense` or a fixed
  `grid-auto-rows` with a multi-row span, and both reassign the cells of the cards after the pane,
  which is exactly the movement FS-02.R55 exists to remove; the empty space is therefore chosen
  deliberately over reintroducing it. No JavaScript measures, tracks, or compensates for layout
  here — the guarantee is the template, not a computed position (INV §1).

- **R46** — **Both collapse affordances are feature-owned composition
  over the existing expansion state.** The per-card control required by FS-02.R56 is a
  `components/ui` `Button` rendered inside the expanded card's header region, so it inherits the
  core and Sky & Grove button construction, focus ring, and hover feedback rather than defining its
  own; it calls the same `onToggle` the header region already calls and stops propagation so the
  header does not receive a second toggle. **Collapse all** (FS-02.R57) is a `Button` in the
  existing `PageHeader` actions beside `DensityControl`, rendered only when the grid's own rendered
  ids intersect the `expanded` list. It sets `expanded` to that list minus the ids the grid is
  currently showing, which is what preserves the out-of-project ids FS-02.R49 retains, and it flows
  through the one debounced `putLayout` effect that already saves order, density, groups, and
  expansion. Neither control adds a store field, a query, an endpoint, a confirmation dialog, or a
  second source of expansion state, and neither reaches the per-agent composer drafts, which stay
  owned by the chat surface (R41). Both are exposed through the curated contract: one named
  `agent-card` slot for the per-card control, added to `contract.json` in the same change; the
  toolbar control needs no new hook because `page-header` already exposes its actions region.

- **R47** — **The card name's wrap is a CSS-only change on the two
  existing name selectors.** FS-02.R58 is implemented in
  `ui/src/styles/features/dashboard.css` on `.agent-card-top strong` and `.agent-card-name-link` by
  replacing `white-space: nowrap` and `text-overflow: ellipsis` with a three-line clamp plus
  `overflow-wrap: anywhere`, and by lowering `font-size` from the shipped `1.45rem`. Sizes in this
  file are already literal `rem` values and no font-size scale token exists, so the smaller size is
  expressed the same way rather than inventing a scale; the `--ad-font-display` family, spacing,
  radii, and color continue to come from tokens, and no clamp, measurement, or size is applied from
  JavaScript or an inline style.
  `.agent-card-top`'s `auto minmax(0, 1fr) auto` template already gives the name a shrinkable track,
  so the grip and state badge keep their intrinsic widths while the name wraps inside its own track
  and cannot overlap them. Both skins inherit the change through the same selectors, and the
  deterministic visual matrix gains the long-name card FS-12.A16 checks.

- **R48** — **The expanded card's context figure reuses the shipped
  meter's derivation.** FS-02.R59 moves, and does not duplicate, the context reading: `ContextBar`
  keeps sole ownership of clamping `context_pct`, rounding it, choosing the low/medium/high ramp,
  and producing the visible `n% context used` label, and gains a compact form. Density and tone are
  orthogonal, so they take separate curated dimensions: the meter's `data-variant` stays the
  low/medium/high tone in both forms and the compact form is a `context-meter` state registered in
  `contract.json`. Folding density into the variant would leave the compact meter — the only context
  reading FS-02.R59 keeps on the dashboard — with no tone a skin could select on. `AgentCard` renders the
  existing `context` slot only while expanded, in the header region, and renders no context element
  while collapsed. A second rounding or threshold expression anywhere else is the drift INV §2
  describes, so the compact form differs from the full meter in presentation only.

### 2.6 Automatic pane opening on a waiting transition

- **R49** — **The transition is observed from `state_update` in the
  grid, not from the notification stream.** FS-02.R61 keys on the durable `state` field every
  `state_update` carries, which is self-correcting on a dropped frame (FS-02.R9), rather than on the
  `notification` event `internal/bus/bus.go` emits for the same transition. The notification is the
  wrong source twice over: `NotificationsEditor`'s per-type mute list filters it, so muting a toast
  would silently disable an unrelated layout behavior, and the server emits it once and never
  replays it, so a reconnecting tab would see nothing. No server, SSE, or endpoint change is part of
  this requirement.

  `CardGrid` owns the detection because it already owns the `expanded` list, the four-pane cap, and
  the set of ids the grid actually renders. It keeps one ref of the last observed `state` per agent
  id, written by the same effect that reads it, so the previous value exists in exactly one place
  (INV §2); nothing else in the client stores a shadow copy of agent state, and `agentStore` keeps
  its single-writer role.

  The record is a derived cache across a connection boundary, so it is reset there (INV §1). While
  `useAgentStore`'s `hydrating` flag is set and until `hydrated` is true, the effect only reseeds
  the record and expands nothing, and ids that `hydrateComplete` prunes are dropped from it. A
  reload, a reconnect, and a re-hydration therefore reseed rather than fire, which is what makes
  FS-02.R61's "newly observed" rule true instead of aspirational. Because the record lives in
  `CardGrid`, it is created and discarded with the mounted grid, which is the mechanism behind
  FS-02.R61's stated limit that a pane opens only while a grid is on screen.

  An eligible transition expands through the same code path a person's click uses — the existing
  expansion branch that appends the id and applies R48's cap and recency — rather than a second
  expansion routine, so the cap, the least-recently-used eviction, and the persisted list cannot
  drift apart from the manual path (INV §2, §10). Eligibility reuses the grid's already-computed
  grouped/rendered set and the agent's `interface` field; it derives no second copy of which cards
  are on screen. Several eligible transitions arriving together are applied in observation order
  inside one state update, so React commits one layout change rather than one per agent.

  Observation order is a client index `agentStore` stamps on every update it applies, not the
  `updated_at` the payload carries. That field is a millisecond wall clock: a burst of transitions
  shares one value, and a stable sort over a tie falls back to whichever order the agent record
  enumerates, which is insertion order rather than transition order — so the wrong pane is evicted
  exactly in FS-02.A43's five-at-once case. The index lives beside `agents` in the store, which
  keeps it a single-writer value and drops it on every path that drops an agent (INV §1).

### 2.7 Run-page attention, pause actions, and declared outputs

- **R50** — **The run page renders the awaiting-approval state from the run's
  own data, and derives no second copy of it.** FS-14.R54's waiting state arrives on the run detail
  and `pipeline_update` shapes TS-03.R35 defines, and the run page reads it exactly as it already
  reads the attention reason — through the existing pipeline store, which stays the single state
  authority for the surface (TS-09.R23). The page does not inspect agent status, subscribe to the
  permission stream, or keep its own map of which agents are waiting; a run that is waiting says so
  because its own payload says so, so a reload and a reconnect render it identically to a live edge
  (INV §1). The waiting state uses the same attention presentation the page already gives a paused
  run rather than a second visual language for "needs you", and it is rendered from parsed,
  in-vocabulary data with an explicit fallback rather than an unlabeled blank (INV §8).

- **R51** — **Pause action copy is rendered where the action is, not in a
  branch that only sometimes shows.** FS-14.R55's explanations attach to the controls themselves —
  the reason beside the disabled **Continue**, the consequence beside each of **Continue** and
  **Retry stage** — so they render on every branch that offers the action, including the ordinary
  `blocked` pause. The current wiring places the one explanatory line inside the recovered-pause
  branch, where the choice does not exist; a person meeting the ordinary pause sees neither. The
  disabled control names its missing input through the same mechanism the start dialog already uses
  for this exact pattern (FS-14.R46) rather than a second one (INV §2), and the copy is an ordinary
  rendered element rather than a `title` attribute, so a test can see it and a pointer is not
  required to read it (INV §10, INV §13).

- **R52** — **A shipped payload field reaches a surface.** `report_outputs`
  already ships on the run detail shape and is drawn nowhere; FS-14.R56 renders it inside the
  timeline attempt that produced it, beside the summary, details, and checks that entry already
  draws, using the same bounded-text presentation those fields use rather than a new one. The
  named-values disclosure on a finished run is open by default, matching the frozen-setup disclosure
  beside it. No API, schema, or store change is needed — the data is present and parsed today, which
  is why this is INV §10's own case rather than a feature addition.

### 2.8 Docked annotation tray and annotation-block suppression

- **R53** — **The tray docks through the transcript region's own
  grid and a container query; nothing measures anything.** FS-13.R20 is implemented by making
  `.transcript-wrap` a container and a two-column grid whose second track exists only while drafts
  are pending, with `.annotation-tray` leaving `position: absolute` for that track. The threshold is
  a `@container` condition on `.transcript-wrap`, not a `@media` condition on the viewport, which is
  the whole reason one rule serves both surfaces: the dashboard chat pane (R41/R45) is narrow inside
  a wide window, so a viewport query would dock it and a container query does not. Below the
  threshold the existing absolute overlay rules apply unchanged, so the fallback is the shipped
  presentation rather than a second one. No `ResizeObserver`, element measurement, or
  JavaScript-applied width participates (INV §1), and the two forms are one component in two CSS
  states rather than two components, so the drafts, target selection, and send path cannot diverge
  between them (INV §2). While docked, `.transcript-view`'s `max(--ad-space-6, 10vw)` horizontal
  padding reduces, so the reflowed text column keeps its readable width instead of paying for the
  tray twice. FS-13.R22's roomier draft row is CSS on the existing `.annotation-draft` selectors,
  with the anchor promoted to its own heading element in `AnnotationTray.tsx`; every className
  shipped has a defined selector in `ui/src/styles/features/agent.css` in the same change, because
  the build and Testing Library are both blind to CSS (INV §13). The tray is exposed through the curated
  contract as one registered `annotation-tray` component added to `contract.json` in the same
  change, carrying its collapsed state and no variant: the docked-versus-overlay form is a container
  query nothing in the client can observe, so a skin hooks that form through the same query. The
  collapsed strip is `data-state="collapsed"`/`"expanded"`; individual descendants are not skin
  hooks (R8, R14). FS-13.R21's collapsed flag is a field on the existing per-source
  annotation draft record in `annotationStore`, so it rides that store's shipped persistence,
  30-day expiry, 20-source cap, and delete-with-agent path rather than adding a second browser
  storage key or lifecycle (FS-13.R16, INV §1).

- **R54** — **Suppressing the annotation prompt is one more
  rule in the shipped transcript projection, and it recognizes the block without respelling its
  format.** FS-13.R23 is implemented inside `appendRenderedEvent` in
  `ui/src/store/transcriptStore.ts` — the seam `foldTranscript` and the live append already share,
  the same place `permission_resolved` folding lives — so bulk replay, archive replay, and a live
  frame produce identical rendered lists by construction (INV §1/§2). Adding it to
  `foldTranscript` alone would leave the live path drawing an event the reload then removes. A
  `user_prompt` is dropped when the last already-rendered event is an `annotation` event whose
  `target.kind` is `self` and the prompt's text begins with the block's sentinel header. All three
  conditions are load-bearing: adjacency alone would swallow the message a person types right after
  assigning a batch to another agent, and the `self` check is what makes the non-self case
  structurally unreachable rather than merely unlikely. The sentinel is the only thing the client
  borrows from `runtime.FormatAnnotationBlock` (`internal/runtime/event.go`); the client does not
  reimplement the block's layout to compare against it, because a second spelling of that format
  would drift the moment either side changed (INV §2). It lives as one named exported constant whose
  comment cites the Go writer, and a Go test asserts the emitted block still starts with it, so the
  cross-language pair is pinned rather than assumed. The rule touches presentation only: `rawByAgent`,
  the transcript endpoint, the appended event, and the search index are untouched, and because the
  decision is made at render time it applies to transcripts recorded before it shipped.

- **R55 — The chat header holds two control groups with two different
  apply models, and they must not share state.** The runtime picker is staged: backend and model are
  local state compared against the agent's current runtime, that comparison reveals **Switch**, and
  only Switch sends anything. The live settings — fast mode (FS-03.R45) and effort (FS-03.R47) — are
  the opposite: they apply on change. Each live setting is held in its own state, excluded from the
  picker's changed-versus-current comparison, and excluded from the switch request body. Effort
  moving out of that comparison is the concrete change to shipped code: it is currently part of the
  picker's selection object and part of what reveals **Switch**.

  This is a correctness constraint, not a layout preference. Folding a live setting into the picker's
  selection object would either send it through switch-runtime, which stops and restarts the CLI to
  change something the provider accepts live (FS-09.R56, FS-03.R47), or reveal **Switch** for a
  change Switch does not carry. The two groups must also read as different kinds of control, so a
  person who has learned "changes here need Switch" is neither misled into pressing it for a setting
  that already applied, nor left unsure whether an applied setting is still pending (INV §8).

  Its mutation follows the ordinary optimistic-free path this file already requires of consequential
  actions: the toggle shows in-flight, takes the server's returned applied value as truth rather than
  assuming its own, and on failure returns to the agent's actual fast mode with the typed reason
  rendered as product text — never leaving the control showing a state the server did not confirm
  (INV §8, INV §1). Because the applied value also arrives on the republished agent, the toggle
  derives from that agent field rather than holding a second copy that could drift from the card and
  the archive header (INV §2).

- **R56 — A held message is a transcript-tail affordance, not a
  transcript event.** The pending follow-up (FS-03.R48) renders at the end of the transcript beside
  the R29 waiting indicator, from client state keyed to the agent and rehydrated from the runtime's
  live hold snapshot on browser mount, and is never merged into the
  event list `foldTranscript` builds. Keeping it out of that list is what makes it structurally
  impossible for a live render and a reload to disagree about it (INV §1/§2, the rule this file
  already applies to the annotation-block fold): the server sends no event for a message it has not
  delivered, so the only truthful place for it is beside the list. The snapshot includes the last
  sequence present when the hold was accepted; only a matching user event with a later sequence
  clears it, so an older identical prompt cannot do so.

  It must read as not-yet-sent rather than as a sent message awaiting reply — the failure mode is a
  person believing the agent has already seen it — and it carries its own withdraw affordance
  (INV §8). Release on stop reuses the existing per-agent draft store rather than a second text
  store, and applies R36's newer-draft rule: write the released text only into an empty composer for
  that agent, never over text typed since (INV §2).

### 2.9 File viewer beside the transcript

- **R57** — **The viewer docks through the transcript region's own grid,
  and the width cap relaxes by state rather than by measurement.** FS-03.R53 extends the mechanism
  R53 already established for the annotation tray instead of adding a second panel primitive
  (`INV §2`): `.transcript-wrap`'s grid gains a **leading** track that exists only while a file is
  open, and the `@container transcript` condition decides the docked form and the
  transcript-width form. No `ResizeObserver`, element measurement, or JavaScript-applied width
  participates (`INV §1`). The tray keeps the trailing track, so a docked tray and an open file
  coexist rather than compete. One thing a container query cannot express is FS-03.R53's relaxed
  content width, because `.chat-panel`'s `max-width` is set outside the transcript container: the
  panel therefore carries a `data-file-open` state attribute and CSS keys the relaxed cap off it.
  That is a state attribute of the same kind `annotation-tray`'s `data-state` already is, not a
  measurement, so R53's no-measuring rule stands. The viewer is one component in two CSS states
  rather than two components, so the open file, its line anchor, and its refusal state cannot
  diverge between the forms (`INV §2`).

  Content rendering reuses what ships. The file's text is drawn by `renderers/CodeBlock.tsx`, and
  its rendered Markdown form by the same `ReactMarkdown` configuration `AssistantText.tsx` uses —
  extracted into one shared Markdown component in the same change, so `rehypeSanitize`, the `code`
  override, and the diagram rules stay single-sourced and no second raw-markup insertion path is
  created (FS-03.R20/R37/R38, `INV §2`). The open file is not new client state: `?file=` and
  `?fileLine=` on the route are its single source of truth (FS-03.R54), so no store, context, or
  persisted browser key is added, and the pane's navigate-instead behavior (FS-03.R53) rides one
  more per-surface `TranscriptView` prop beside the shipped `annotationsEnabled` rather than a
  provider. The content read is an imperative call beside `getTrackedFiles`/`searchSessionFiles` in
  `ui/src/api/client.ts`, carrying the per-agent request token `FilesTab`/`CommandsTab` already use
  so a slow read cannot overwrite a newer one (`INV §1`). Every className shipped has a defined
  selector in `ui/src/styles/features/agent.css` in the same change, because the build and Testing
  Library are both blind to CSS (`INV §13`), and the viewer joins the curated contract as one
  registered `file-viewer` component in `contract.json` carrying no `data-variant` for the
  docked-versus-reflowed form, since nothing in the client observes which form is on screen (R53,
  R8, R14).
- **R80 (shipped 2026-09-30) — File annotations extend the transcript's existing selection machinery.**
  `TranscriptView` remains the composition owner for one annotation menu, one `annotationStore`
  tray, and the file viewer. A loaded `FileViewer` reports a valid selection into that shared seam;
  it does not mount its own tray, store, or delivery client. The selected excerpt goes through the
  existing `clipAnnotationExcerpt` helper, Copy goes through `copyText` and its visible-error path,
  and the shared menu adds the same draft action transcript selections use (INV §2/§8). With no
  non-whitespace selection wholly inside the loaded viewer, the handler does not prevent the native
  context menu and creates no draft.

  Source mode derives the containing 1-based range from `CodeBlock`'s shipped `data-file-line`
  nodes and emits one tagged file anchor with the exact selected text; rendered Markdown uses the
  same sanitized output and emits the tagged path-only form because rendered DOM text has no
  stable source-line mapping. Loading, error, and refusal states expose no annotation action.
  Reloading or replacing the viewer cannot mutate a captured draft, whose excerpt and displayed
  path are already point-in-time browser state. `AnnotationDraft`, the server payload, annotation
  cards, and tray labels gain the same additive discriminator in lockstep (TS-03.R49, INV §11),
  while tray caps/expiry/deletion, send behavior, and the three-column file/transcript/tray grid
  remain the shipped implementations. Focused tests drive actual selection ranges and the wire
  body rather than a helper-shaped fixture (INV §17).
- **R81** — **Context and runtime metadata extend the existing card composition.**
  `ContextBar` remains the only component that clamps and rounds `context_pct`, selects its tone,
  and composes its accessible label. It additionally accepts the optional TS-03.R50 raw pair and,
  only when both values are present, formats the exact unabridged integers with digit grouping as
  `used / total tokens` beside the existing percentage; compact and full forms consume that same
  output, and a missing pair leaves their shipped percentage-only label. No caller derives counts
  from the percentage or formats a second label (FS-02.R62, FS-03.R66, INV §2/§11).

  The expanded `AgentCard` reuses its existing backend/model/effort string and the registered
  `agent-card` `metadata` slot rather than creating a second runtime projection or control. The
  identity/name and quiet technical metadata occupy the header's content side; context, state, and
  Collapse remain a distinct action/status side that may wrap without overlap at the supported
  desktop floor. Empty effort produces no separator, long technical values wrap or clip within the
  card, and the fixed expanded height plus conversation scroll ownership from R75 remain unchanged.
  The existing `context-meter` label and `agent-card` metadata hooks are sufficient, so this adds no
  token, public hook, inline-style exception, skin branch, motion, or component framework. Focused
  component tests plus matched Core, Sky & Grove, and Studio matrix/browser views cover zero,
  unknown, long, and live-updating values (FS-02.R63/A44–A45; INV §8/§10/§13/§17).
- **R58 — Browser-local identifiers rename with a one-time copy-forward.** With
  FS-00.R19 the document title, the header wordmark component, the built-in skin name, and every
  on-screen product string say Chuck. The identifiers the browser itself keys on rename too:
  `chuck-chat-drafts`, `chuck-annotation-tray`, `chuck.pipeline-builder-agent`, and the
  `chuck-events` SharedWorker. Because that storage is per-origin and holds text a person has not
  sent yet, one module owns a startup copy-forward that reads each old key, writes the new key only
  when it is absent, and then removes the old one — never merging, never overwriting newer state,
  and tolerating unreadable or absent storage without blocking the app (INV §1, §7). The renamed
  SharedWorker name deliberately gets no copy-forward: a worker is live state, and a browser holding
  a page from before the rename simply starts a second worker until it reloads. The copy-forward is
  written so that running it twice is a no-op, since it runs on every mount rather than behind a
  persisted flag.

**R59 — Runtime-native activity stays subordinate to the conversation.** The experienced
operator's primary job remains reading the root exchange and intervening only when work needs
attention. `TranscriptView` and `appendRenderedEvent` own one `runtime-activity` projection used by
the full agent screen, dashboard chat pane and read-only archive (INV §2). Its reading order is root
conversation, nested child activity at the causal position, and a compact background-task summary
at the transcript tail while any task is active; completed child/task detail remains available by
disclosure without competing with assistant text.

The projection exposes one curated `runtime-activity` hook with `thinking`, `child`, `task-list` and
`task` slots plus active/completed/failed/stopped/disconnected states. Thinking is a subdued collapsed disclosure
and never reserves empty space after reload. Child sessions use restrained hierarchy and the
existing assistant/tool/diff/permission components rather than equal cards or a second transcript;
visual indentation is capped after two levels while labels retain ancestry. Background rows put
state before metadata, show **Stop** only for a live controllable task, keep the related tool call as
the output owner, and preserve the last state plus inline retryable error when Stop fails. A
disconnected child is read-only and states that its historical outcome is unknown rather than using
failure styling. Archived rows are read-only. The same composition works at the transcript container's narrow dashboard-pane
width and supported desktop floor in Core and Sky & Grove without a new tab, side panel, provider
label, raw color, token family or motion dependency. High-frequency updates use no entrance motion;
only existing state-color/typographic feedback changes, and reduced motion loses no information.

- **R60 (shipped 2026-09-23) — Run supervision uses the existing detail projection without a rail.**
  `RunBrowser` derives current/final stage position, the next human stage title, and each attempt's
  human title from the run detail's frozen template and stage-task fields. An absent or unmatched
  stage id falls back to the available id without inventing progress or a next stage. The existing
  result and runtime fields supply each attempt; no endpoint, schema, store, persistence, or second
  projection is added (FS-14.R79, INV §2/§10). The run page renders one timeline track and removes
  the setup/value rail and its sticky positioning. It reuses core tokens and the existing run,
  timeline, attempt, and action hooks; removing the obsolete `setup` and `values` slots updates and
  versions the curated presentation contract under R8. The bundled Sky & Grove skin has no rules
  targeting those slots. The start form and template editor retain their own disclosures and styles.
  R52's stage-local output presentation remains; its finished-run default-open value disclosure is
  superseded when this change ships. Focused interaction checks and Core/Sky & Grove browser views
  at the desktop floor and a wider viewport verify long output text, expanded attempts, attention
  actions, and unobscured stage content (INV §8/§13/§17).

### 2.8 Studio built-in skin

- **R61 (shipped 2026-09-23) — Studio extends the finite built-in appearance set.** Add the id `studio` to
  the Go write validator, frontend effective-id allowlist, Settings option list, and versioned
  `presentation/contract.json` skin list in lockstep; advance the contract version from 2 to 3.
  The existing `AppearanceRoot`, React Query config projection, optimistic save/rollback, Core
  first-paint fallback, and unknown-id warning remain the only activation path (R30–R32).
  `appearance_skin: "studio"` is the sole new stored/wire value; Core stays empty/absent, and
  `sky-grove` retains its meaning. There is no new provider, route, browser storage, migration,
  per-project preference, dynamic loader, or external skin source. Update `ui/AGENTS.md`'s
  currently singular production-skin description to match the finite three-appearance contract
  (FS-12.R42, TS-02.R36,
  TS-03.R45; INV §8/§10/§11).
- **R62 (shipped 2026-09-23) — Studio is one locally bundled skin stylesheet.**
  `styles/skins/studio.css` is statically imported by `styles/index.css`, keeps every rule inside
  `ad-skins`, defines its private raw palette only in the `--ad-studio-*` namespace, and maps only
  approved public semantic tokens under `:root[data-skin="studio"]`. The directional palette is
  canvas `#f4f8f7`, panel `#fcfefd`, raised `#fbfefd`, primary text `#263432`, muted text
  `#667571`, default border `#dce6e5`, forest support `#4e8068`, soft blue `#e2f0f6`, and
  restrained coral action `#e76d5b`; semantic state, destructive, permission, project-accent,
  and technical colors retain separate roles rather than deriving their meanings from the coral
  action. Existing bundled fonts supply Studio's friendly display/text and crisp mono roles;
  no runtime font or image request is introduced. Studio's open-canvas ornament replaces the Core
  `body::before` grid with a uniform radial dot repeat of approximately 1.2px dots on a 26px
  pitch in `#577d7a` at approximately 10% opacity. Opaque reading surfaces cover the ornament;
  the dot pattern never enters a transcript, form, dialog, technical block, or control. Studio's
  inactive Settings preview uses the same private palette and a miniature dot treatment, not a
  duplicated literal palette. All geometry, depth, and typography changes remain token or
  approved-hook overrides, not feature CSS values or new layout ownership (FS-12.R43/R45).
- **R63 (shipped 2026-09-23) — Product anatomy stays feature-owned.** Studio may override curated
  presentation hooks for existing cards, states, shell, transcript, controls, and overlays, but
  it does not replace their TSX, DOM order, content, route, event handlers, or sizing decisions
  that protect dashboard grid order and pane behavior. In particular, the expanded card continues
  to compose the shipped `TranscriptView` and `Composer` under R41–R49; no second transcript,
  mockup markup, transcript event projection, or chat state is added. If a visual treatment truly
  needs a public hook not already present, register and version that narrow product-native hook
  before using it, with the matching checker and fixture; implementation-class selectors are not
  a substitute (FS-12.R44; INV §2/§13).
- **R64 (shipped 2026-09-23) — Verification covers the third skin without weakening the first two.** The
  contract checker proves `studio` has one declared bundled stylesheet, active and preview
  selectors, only permitted tokens/hooks and private values, and no network import. The
  deterministic matrix uses the same feature data for Core, Sky & Grove, and Studio, including
  collapsed/expanded dashboard chat, status extremes, dense configuration, overlays, syntax,
  diffs, terminal, and the supported desktop floor. Component/config/API tests prove new-id
  persistence, immediate selection, rollback, unknown-id fallback, and Core/Sky & Grove
  non-regression; a real-browser pass checks the actual transcript pane, dot placement, contrast,
  focus, overflow, and technical renderer updates after switching. No pixel-baseline framework
  or behavior-only mockup assertion substitutes for rendered evidence (FS-12.A18–A20;
  INV §10/§13/§17).

### 2.9 Studio composition completion

- **R65 (shipped 2026-09-25) — Studio may own skin-scoped spatial composition.** The initial Studio sheet
  in R62 changes mostly semantic values and ornament; that is not the full R46–R49 result. Inside
  `ad-skins`, `:root[data-skin="studio"]` may now override layout, spacing, type scale, border
  weight, and surface grouping on documented public hooks and their documented slots for the shell,
  Dashboard, agent workspace, Tasks, Pipelines, Archive, Settings, onboarding, and overlays. This
  supersedes R62's prohibition on Studio layout ownership and R63's prohibition on Studio sizing
  changes only; it does not give presentation authority over feature data or behavior. Keep Core
  styles and Sky & Grove selectors intact, and do not make either depend on Studio values. The
  Studio sheet remains the sole skin-specific source rather than a second component library or
  provider (FS-12.R46–R49, INV §10/§13).
- **R66 (shipped 2026-09-25) — Structural support is neutral and minimal.** Prefer the existing
  `data-ui`/`data-slot` surface and CSS layout over rewriting components. Where the requested
  composition cannot be expressed without selecting an implementation class or an undocumented
  descendant, add the smallest product-native wrapper or slot in the owning feature, register it
  in `contract.json` with a version advance, and keep the same semantic DOM reading order,
  handlers, accessible labels, state derivation, and children in every appearance. Components do
  not branch on `studio`, copy the Make prototype's markup, or build a second transcript or
  appearance-specific route. Shared markup changes must render with unchanged Core and Sky & Grove
  geometry and behavior (R7–R9/R26–R28, FS-12.R44/R48, INV §2/§13). The composition repair advances
  the contract to version 4 for neutral shell `header`/`brand`, `composer`, pipeline-section/workspace,
  and appearance-preview slots; the presentation checker rejects implementation-class selectors in every
  production skin.
- **R67 (shipped 2026-09-25) — Existing layout contracts bound Studio's freedom.** The dashboard keeps the
  persisted `perRow`/gap template, one-track expanded pane, fixed pane height, row order, and
  internally scrolling real transcript under R41/R45 and FS-02.R55; visual card hierarchy must
  fit those dimensions rather than silently changing density or shifting neighbors. The compact
  header still fits primary and active-project navigation plus connection state at 1024px under
  R44 and FS-12.R39. The agent workspace preserves file-viewer docking, annotation behavior,
  conditional Terminal, and composer focus; task, pipeline, archive, Settings, and onboarding
  surfaces preserve their existing action/validation order. No CSS visual reordering may make
  keyboard or screen-reader order disagree with the reading path (FS-12.R47–R49, INV §8/§10).
- **R68 (shipped 2026-09-29) — Composition is a separate acceptance gate from skin activation.** Extend the
  deterministic matrix only with representative content/states the shipped fixture lacks; do not
  duplicate feature components or construct a Make-derived dashboard. Real-browser review compares
  Core, Sky & Grove, and Studio at 1024px and a wider desktop viewport for the exact route/state
  set in FS-12.A21–A23. Record evidence of changes in spatial hierarchy and text measure on each
  named surface, including a desaturated side-by-side comparison; a palette-only diff fails even
  when R64's token/contract checks pass. Focused behavior tests protect interaction parity,
  while presentation-contract/style checks protect every new slot or skin selector. No stored
  screenshot baseline is introduced (INV §10/§13/§17).

### 2.10 Compact agent and automation setup

- **R69 (shipped 2026-09-26) — Disclosures are feature-owned composition over existing state.** Implement
  FS-01.R37/FS-12.R50 within the existing New Agent form and its runtime-selection helpers; reuse
  existing HTML/Radix and tokenized form treatments. Keep draft values owned above conditional
  content so collapsing cannot reset or reinitialize them. Visible summaries derive from the same
  ids/values that build the submitted request, not a second default resolver; duplicate display
  names never become identity. Preserve configured-default resolution and terminal/effort/fast gates.
  Validation reveals the owning disclosure before focusing a control; global errors and source or
  compatibility warnings remain outside it. Add no saved disclosure preference or UI mode, generic
  form framework, dependency, API, or persistence shape. Extend public presentation hooks only when
  the changed structure needs them, updating their contract and skin consumers together.
- **R70 (shipped 2026-09-26) — Native linking retains one connection implementation.** Give the existing
  ConfigSourcePanel a compact onboarding presentation or extract only its shared connection seam;
  do not duplicate preview/bind/import orchestration. Settings keeps its existing full inspection
  surface, while onboarding owns a Details disclosure. Both remove the unavailable detached-copy
  controls and preserve ordinary unlink. Derive the wizard's applicable steps from the selected
  backend's actual type, including resumed entry, instead of fixed indexes plus a Claude fallback.
  Preserve mutation claims, consent ownership, source/backend query invalidation, catalog drafts,
  and the mounted-wizard latch; no additional onboarding server flags or source-write authority are
  introduced. FS-04.R49 and FS-08.R35–R36 own the observable contract.
- **R71 (shipped 2026-09-26) — Dependency pickers project existing task/run queries.** Share the dependency
  selection/serialization used by Create and Re-arm within the Tasks feature (INV §2). Use
  `useTasks(project)` and the existing paginated `usePipelineRuns()` projection, filtering loaded
  runs to the selected project and exposing explicit next-page loading without fetching the entire
  history automatically. Distinguish no matching rows yet from exhausted history; errors retain
  draft selections. Named and manual input use one typed source kind/id and outcome set, serialized
  into the existing TaskArmInput. Source-specific outcome choices follow FS-16.R3/R13; source and
  project changes cannot silently turn a hidden invalid outcome into a valid different request.
  Re-arm keeps its full-set replacement request and server graph validation. Context attachments
  stay explicit ids under Advanced: there is no browser context-discovery endpoint, so this effort
  adds none and does not repurpose agent-scoped MCP discovery. No task schema or authorization change.
- **R72 (shipped 2026-09-26) — Pipeline runtime disclosure preserves resolved request semantics.** Refactor
  RunStartForm's existing assignment fields into the shared compact summary/disclosure for inline
  and dialog usage; the dialog becomes Setup → Review under FS-14.R80. Keep the existing configured
  runtime default resolution, explicit orchestrator/dedicated_assignments payloads, capability
  validation, request id and conflict-confirmation handling. Proposal hydration supplies exact
  values independently of defaults: toggling a disclosure is not an edit, while changing a value
  invalidates the existing exact-proposal confirmation. Review includes fast as well as backend,
  model and effort. Named diagnostics open the owning pane/disclosure before focusing it. Runtime
  choices stay frontend drafts until the existing start operation freezes them; no template,
  pipeline state machine, run-detail, retention, or transport change is introduced. Focused component
  tests cover request equivalence and recovery; FS-12.A24 owns the rendered gate for R69–R72.
- **R73 — The phone app is a separate, phone-first entry.** `ui/remote.html` with
  `ui/src/remote/` is a second Vite entry in the same build and embed (TS-13.R14). It has its own
  compact shell designed for phone widths (360–430px), not the desktop shell, skins, or the 1024px
  desktop floor, and it renders with Core tokens only. It reuses `ui/src/api` and the existing
  transcript, diff, and permission rendering primitives where they fit a phone, rather than forking
  them. It ships a web-app manifest (`display: standalone`) and a service worker limited to push
  display, notification click routing, and caching the app shell; it never caches API responses.
  The desktop bundle does not import `ui/src/remote/`.

### 2.11 Shared creative-workspace layout and finish

- **R74 — Shared composition has one owner, independent of appearance.** Promote the
  approved Studio composition into the appropriate `ad-features` styles and repeated construction
  into `ad-components`; shared semantic values remain in `tokens.css`. Core renders the complete
  upgraded layout with no skin marker. Remove superseded layout/type-scale/radius overrides from
  both production skin sheets so the three current appearances share geometry, measures and
  hierarchy; retain their private palettes, semantic color mappings and canvas ornament. Depth uses
  shared shadow geometry with palette-derived color, not per-skin offset/blur differences. Do not
  copy Studio selectors into a second stylesheet, introduce a theme provider/layout mode or branch
  components on appearance. This supersedes R65's instruction to keep the other layouts intact and
  R66's unchanged-Core/Sky-geometry clause, not R9's extensible future-skin seam (FS-12.R52/R56–R57;
  INV §2/§10/§13).
- **R75 — Card and composer geometry is content-driven inside existing bounds.** The
  expanded card explicitly allocates a content-sized header and `minmax(0, 1fr)` conversation
  region within its existing fixed height; spare height never stretches implicit header rows.
  Preserve R41/R45's one-track grid, persisted columns/gap, scroll ownership and neighbors.
  Apply the same compact action construction to the real shared `Composer` in dashboard and
  full-agent contexts: actions align within their own content-sized region rather than inheriting
  textarea stretch. Card identity/metadata/preview spacing, quiet drag grip and soft resting depth
  are owned by the existing card/primitive styles. Remove empty decorative indicator space without
  removing real mail/sent content or handlers. All reusable values use existing semantic tokens,
  extending the public token contract only for a genuinely missing role (FS-12.R53; INV §2/§13).
- **R76 — Agent and Tasks polish uses existing feature composition, not replacements.**
  `agent.css` owns the compact identity/runtime/context header, tabs, prose measure and aligned
  composer. Technical blocks, file-viewer expansion and container-query annotation docking retain
  R53/R57's layout/scroll behavior. Unsupported live-settings content leaves no decorative frame;
  capability ownership and staged/live apply semantics remain in the existing feature. `tasks.css`
  owns inset authoring padding, bounded form measures, field grouping, content-sized actions and
  ledger row rhythm; narrowly necessary wrappers remain in `TasksPage` with unchanged semantic
  reading order, draft ownership and submitted payload. Preserve R69–R72's compact setup contracts
  rather than restoring prototype fields or inventing task semantics. Base feature selectors may
  address their owned classes; skin selectors continue to use only public hooks. Register/version
  a new hook only if an actual skin consumer needs it (FS-12.R54–R55; INV §8/§13).
- **R77 — Project tabs keep one navigation projection and construction.** Refine existing
  `ActiveProjectNav`/`shell.css`, using the shared text font, restrained tab geometry and accessible
  selected/hover/focus treatments. Retain R44's catalog/store projection, alphabetical order,
  five-link/current-project rule, overflow disclosure and single-row desktop fit. Project RGB stays
  the existing exact inline-data exception; no independent query, browser measurement algorithm,
  route, state, storage or overflow implementation is added (FS-12.R58; INV §2/§8/§13).
- **R78 — Badge motion is state-driven CSS, not lifecycle bookkeeping.** Use the existing
  `StateBadge`/`Badge` construction, semantic state colors and indicator. Busy receives approximately
  2.4s smooth cycles; error and `waiting_input` approximately 1.2s. Animate indicator/emphasis
  opacity without hiding text, changing layout or moving the card. Scope this recurring effect to
  live running agent cards, excluding stopped/archived/read-only cards even if their last state was
  busy/error/waiting. Keep the existing badge status/label contract; the owning card's existing
  effective `data-state="stopped"` suppresses pulse rather than inventing a stopped badge status.
  Existing feature-owned `running`/archive values determine eligibility; do not
  add a status enum, permission subscription, synthetic pipeline-to-agent mapping, timer, persisted
  animation flag or transition observer. State changes cancel the prior CSS animation naturally.
  `prefers-reduced-motion: reduce` removes the pulse but keeps the strong static state treatment.
  This is explicitly sustained state indication under FS-12.R59, not the workflow's transition-only
  lifecycle celebration; reload in a live state may pulse. Add no motion dependency. Existing busy
  indicator animation is replaced, not stacked with a second pulse (INV §8/§13).
- **R79 — Completion proves shared geometry and finish, not just recoloring.** Extend the
  existing deterministic matrix only for missing FS-12.A26–A31 cases. Matched content and confirmed
  1024px/wider viewports must show the common layout and palette-specific appearance; compare
  desaturated geometry plus normal-color depth/status legibility. Record revision, actual viewport,
  route/state and rendered evidence; timed observation and reduced-motion checks are required for
  pulse cadence. Focused regressions protect navigation/card/composer/task behavior and missing/live
  capability cases; style-contract checks protect selectors/tokens/hooks, followed by the applicable
  TS-06 closure matrix and generated embed, never a hand edit to `dist`. No pixel-baseline framework
  or duplicate feature fixtures are added. This supersedes R68/A21's Studio-only compositional
  distinction when the shared upgrade ships; other existing acceptance debt remains independently
  open until its evidence exists (INV §10/§13/§17).

### 2.12 Tasks relationship view

- **R82** (shipped 2026-10-03) — **Project sections reuse the task query family.** Implement FS-16.R41–R45
  in the existing Tasks feature. Enumerate the existing project catalog, including archived project
  definitions with retained work, and reuse `GET /api/tasks?project=` and `TASK_QUERY_KEYS.project`;
  absence of the route's project parameter means All projects, not the first project. Share query
  options between single-project and multi-project consumers rather than duplicating the request,
  schema or cache. Admit at most four concurrent project task reads for this page, cancel queued
  reads on scope change/unmount, and retain no second global task store or event history. Existing
  React Query cache lifetime remains authoritative. Each section owns loading/error/stale feedback;
  incomplete aggregate counts are labelled as such. Mutations invalidate their actual task project;
  task events and reconnect reuse the existing task-family invalidation. A focused unknown project
  is reported rather than replaced silently. No all-project endpoint, protocol change, persistence
  migration, background polling, or history-retention change is introduced. Existing HTTP full-list
  behavior is reused, not extended into recursive per-task or per-ancestor fetches.
- **R83** (shipped 2026-10-03) — **The frontend preserves the existing task wire meaning.** Extend the shared
  task schema with `waiting`, lineage (parent/run/stage/creation-attempt ids), result details/outputs,
  timestamps, continuation/resume flags, pending yield/release and cleanup fields needed by this
  view. Match the existing Go JSON shape, including omitted/null collections and optional fields
  (TS-10.R25–R35; INV §11). A ready task with `continuation_pending` is ready-to-resume, not a new
  durable state. Pending cleanup overrides a simplistic finished/waiting visual summary without
  altering the immutable outcome. A waiting task's watch set is not present in the task-list
  projection: say waiting for work updates, without claiming its arms or every child are the watched
  set. The shared feature state/action projection consumes server `retry_eligible` and preserves
  stage ownership restrictions; mere run lineage does not mean a task is the authoritative stage.
  When detail opens for work with run lineage, reuse the existing run-detail query and match its
  `stage_tasks[].task_id` before exposing stage-restricted Record result, Re-arm or Delete controls;
  during unavailable ownership data show the run link and the reason those controls are withheld.
  Do not load every run merely to render project rows. Keep unavailable metadata explicit and server
  refusals visible rather than broadening eligibility.
  Creator/assignee labels resolve through existing agent identity data when available, with stable
  id fallback; never request every archived transcript to obtain a display name.
- **R84** (shipped 2026-10-03) — **One feature-owned projection derives related-work groups.** Index each
  project's task snapshot by stable id. Build typed prerequisite links from task work-result arms
  (source → dependent), and delegation links from parent lineage (parent → child). Use undirected
  connectivity of known same-project task nodes only for grouping; keep typed/directed relations
  for display. Creator, pipeline id, and signal name are metadata, not extra graph edges. Dangling
  references render as labelled unavailable relationships and never bridge projects or fabricate
  task rows. Build indexes/components once per changed snapshot in O(tasks + links), using visited
  sets rather than recursive path enumeration. Delegation and dependency together need not form
  a DAG, so no combined-graph topological assumption is allowed. Order rows by prerequisite topology
  with created-at/id tie breaks; delegation is a separate labelled relationship, not a false
  chronology. Guard incomplete/invalid inputs with deterministic ordering and explicit missing data.
  Each task has one row keyed by project/id, and branching/joining links reference that row.

  Project headings use readable name/id order. Within a project, attention groups precede other
  unfinished groups, with created-at/id tie breaks; settled history orders by latest available
  finish timestamp. Groups use deterministic member identity and retain expanded task/draft state
  by task id when membership changes. A selected task's group stays mounted while inspection or an
  action is in progress, even when it becomes settled. Classification into history requires all
  members finished and no pending release/yield or unresolved cleanup. No grouping result is stored
  in the database or used to decide execution, cancellation scope or authority.
- **R85** (shipped 2026-10-03) — **Connected rows belong to the shared presentation system.** Use semantic
  lists, existing buttons/badges/disclosures, feature-owned `tasks.css`, and the existing tokens and
  hooks across Core, Sky & Grove and Studio. Task title/state and wait explanation lead; shallow
  connectors plus textual relationship references preserve branches and joins without infinite
  indentation or a pan/zoom canvas. Task detail expands inline. Long settled stretches may use
  counted disclosures, but all members remain reachable and boundary relationships stay legible.
  Creation follows all project sections/history in DOM and keyboard order; existing form state,
  validation, prerequisite picker and signal request semantics are reused with concrete project
  ownership. This narrowly supersedes R76's unchanged Tasks reading-order clause, retaining its
  styling seams and R69/R71 draft/picker rules. Do not add a graph/motion dependency, new skin,
  synthetic progress percentage, ETA, or animated connector. State updates are immediate; do not
  animate row reordering. Loading, empty, partial, stale, attention and settled states are explicit.
  Projection fixtures cover branches/joins, mixed edge types, missing records, cleanup and duplicate
  names; wire fixtures include Go-produced waiting/lineage shapes. Rendered acceptance follows
  FS-16.A27–A29 before the planned tags are removed (INV §2/§7/§8/§10/§11/§13/§16/§17).
- **R86** — **Notices and steer-backgrounded tools reuse the transcript fold.** Register
  TS-04.R80's `notice` event in the shared `foldTranscript` / `appendRenderedEvent` seam so live,
  reload, archive and phone render one identical compact row: an existing badge carrying the
  severity label (`info` neutral, `warning` the existing warning tone), the title, and the optional
  description as secondary text, with no bubble, avatar or motion, styled with existing tokens in
  every skin (`NoticeRow`). A tool call TS-04.R81 links to a background task keeps its row and
  adds a textual "Continues in background" state (or "Ran in background" once the task settles),
  derived by the one `markBackgrounded` projection that the desktop transcript, Think Tank activity
  and phone conversation share; the desktop task row names the same tool. No new component
  family or dependency. Live notices never enter the card preview. Component tests cover each severity, unknown→`info`, replay order and the
  backgrounded state (FS-03.A48–A49).

- **R87** — The Think Tank page composes existing conversation/content, participant-card,
  file/command, permission and annotation seams under TS-14.R15–R16. Source unions distinguish
  room anchors from agent transcript anchors; renderer/tool grouping is actor/attempt-scoped.
  Add the route and stable room hooks to the presentation contract and all appearances together.
  The reading order is goal/phase → attributed discussion → current action, with compact participant
  identities and ordinary card links. Queued input, pending pause/end, private-work wait, failure,
  closing, judge and retained-source states have explicit text and applicable controls. Preserve
  drafts and origin labels; no new styling framework, decorative motion or phone room surface.
  Rendered validation is owed at implementation, not inferred from existing screenshots.

- **R88** (planned) — One shared `AutoGrowTextarea` in `ui/src/components/ui` owns FS-02.R66: it
  sets height from `scrollHeight` on value change and resize, accepts an optional max height
  (composers pass `40vh`, overflow scroll beyond it), and every `<textarea>` in `ui/src` uses it
  (INV §2). The global `textarea` rule changes `resize: vertical` to `resize: none`. CSS
  `field-sizing: content` is not relied on because Safari support is not guaranteed for the shipped
  WebKit targets.

- **R89** (planned) — FS-02.R67 icons are inline SVG components in `ui/src/components/ui/icons.tsx`
  using `currentColor`; no icon dependency is added. Icon buttons carry `aria-label` and `title`,
  keep existing classes/data-slots, and every new className has a selector (INV §13).

- **R90** (planned) — FS-02.R68 moves the state badge and Collapse out of
  `.agent-card-header-actions` into the header row's trailing slot; the context meter renders as a
  sibling row below `.agent-card-top`. Collapsed cards are unchanged.

- **R91** (planned) — FS-02.R69 labels come from one shared helper, lifted from
  `NewAgentModal`'s `displayLabel`, that returns the readable name and appends `(id)` only for
  duplicate names within the list (INV §2). All `name (id)` renders identified in selectors and
  status labels switch to it; option `value`s and keys stay ids. FS-02.R70 filters `archived`
  projects from the Tasks page's all-projects list and filter only, leaving an explicit focus as is.

- **R92 (planned)** — FS-16.R46–R47 extend the existing `taskWork.ts` projection/Tasks rows with
  parent-lineage child indexing and a pure visible-row projection; never derive descendants from
  `WorkRow.depth` or prerequisite topology. Preserve R84's dependency ordering/group membership.
  Build descendant counts/state summaries with bounded iterative traversal and visited guards,
  without enumerating graph paths or fetching hidden tasks. Missing/cyclic lineage renders visible
  with an unavailable relationship rather than losing rows. Retain one row per project/task;
  boundary prerequisite links reveal the recorded ancestor path. Inspection/mutation pins and their
  ancestor paths override hiding until released. Descendant toggles are separate sibling Buttons
  with accessible expanded state, counts, keyboard focus and controls identity, never nested inside
  the existing detail button. After permitted hiding, focus moves to the controlling visible parent.
- **R93 (planned)** — Collapse choices belong to a feature-owned sessionStorage map keyed by
  project/task identity, shared across Tasks route mounts in that tab. Store only collapsed ids;
  absent ids expand. Bound to 5,000 choices, evict least-recently changed on overflow, prune only
  from authoritative complete project reads/deletion, and tolerate storage errors by retaining
  in-memory session behavior. Refetch/group changes do not reset nested choices or detail drafts;
  filter changes do not interpret unloaded tasks as deleted. Session end removes choices naturally.
  Reuse existing task CSS, semantic tokens, badges and disclosures in all appearances, with instant
  state changes and capped visual indentation. No motion/library, server setting or graph canvas.
- **R94 (planned)** — FS-14.R85 repairs `.pipeline-warning` through the existing paired semantic
  technical background/text tokens, with explicit text treatment for children and normal shared
  button states. `RunStartForm` currently inherits text on a technical background (`pipelines.css`);
  verify this source hypothesis in the actual modal/inline surfaces in all appearances. Do not change
  acknowledgement handlers, warning content or form state. Browser contrast/focus/pending-state
  acceptance is FS-14.A51; DOM tests alone cannot close it.
- **R95 (planned)** — Think Tank template controls extend the stage editor and Setup → Review
  runtime summary/customization; preserve draft state, exact-proposal values and invalid-field focus.
  Run/Tasks rows name Think Tank execution and link to the existing full room route; room headers
  link back to the run. Current room phase, judge output source and valid room recovery lead rather
  than a synthetic standing-agent link. Use the current room UI/renderers and shared hooks; extend
  the presentation contract for any new public slots in all three skins. Rendered closure exercises
  FS-14.A50–A51 and FS-16.A30, with no phone room workspace added.

- **R96 (planned)** — FS-02.R71/FS-21.R43–R44 extend the existing `RoomList` feature projection
  into distinct room navigation cards before the project agent grid, one row per room independent
  of `CardGrid` density/order. Archive uses titled room discovery without acquiring agent-grid
  layout. Use existing Surface/Badge/Button/link seams, semantic depth/radius/spacing and full
  wrapping member identities. Read title/state → quiet goal preview → roster/allowances → current
  room activity/attention; judge state is separate. Preserve independent link focus, loading/error
  and retained tombstone states. This refines R87's goal-first hierarchy to compact title/state;
  the full goal remains available without competing with conversation. No provider-agent card,
  duplicate layout setting, dashboard grouping system or new motion is introduced.
- **R97 (planned)** — FS-21.R46's room composer uses the ordinary `composer` construction,
  shared `composer-input`/actions/picker styling and public `data-ui="composer"` hooks; it receives
  the same R88 `AutoGrowTextarea` and R89 Send icon as ordinary chat. Extract only repeated visual
  markup if needed; feature-owned room send/queue state remains separate from private Send/Steer.
  Do not use the agent-bound `Composer` request handler to submit room input. The conversation
  region owns scrolling and anchors the same bounded reading-measure input beneath it; the roster
  and Files/Commands never turn the input into a side form. Use normal Enter/Shift+Enter and picker
  precedence, with a visually hidden accessible input label and truthful adjacent pending/error
  feedback. Extend the existing autocomplete hook/picker with discriminated participant versus file
  items, stable id/range selection and grouping labels; preserve file context selection and `#`
  behavior. Do not create another suggestion engine or a room-specific textarea style.
- **R98 (planned)** — FS-03.R71–R72 extend `ChatPanel`/`TranscriptView` with a compact room cue,
  Think Tank tab and source-attributed host synthesis row. Use room-title membership queries under
  TS-14.R27, including idle/ended memberships; do not scan only busy-agent turns. Fixed membership
  and source identities own links, not labels; deleted room/agent links become truthful unavailable
  text. Header accent is restrained and independent of lifecycle status colors. Full room goal and
  explanatory Send/Steer text live in the tab, keeping ordinary header height and transcript first
  view. Merge the TS-14.R26 result query by immutable result identity and captured completion anchor
  in a pure renderer projection shared by live and archived desktop chat; do not emit fake provider
  events or duplicate the result row on replay/refetch. Tool arguments do not replace that row. Use the
  existing Markdown/content seam and a clear synthesis/source label; retained result body remains
  visible when the room is deleted. Query completion/failure and room switches preserve current
  drafts and reject stale replies. Existing annotation source availability remains explicit.
- **R99 (planned)** — FS-21.R47 speaker tints are a finite shared semantic palette keyed by fixed
  room member order, with one distinct slot for each of the at most 32 participants and a separate
  judge slot. Retained identity/order keep the same mapping through reload/deletion; judge retries
  use the judge-role slot. Define colors only through Core tokens and supported skin overrides,
  deriving feature treatments from those roles. Update the presentation contract for the bounded
  speaker-slot hook/token family and any new room-card/tab/result hooks; no raw feature colors or
  unlisted inline visual values. Tints appear behind contribution prose and on a quiet roster cue,
  not as permission/state meaning or a recoloring of syntax/diffs. Names and textual states remain
  primary identity/status signals. No decorative animation, new appearance or style dependency.
  Closure renders FS-21.A33–A34/A37 at 1024px/wide in all three appearances, with matched ordinary
  and room composer inputs, long goals/names, dense/concurrent openings, live ceiling edits,
  failure/pending/ended states, keyboard focus and retained judge results. DOM tests do not close
  visual parity or contrast.

- **R100 — Turn activity extends the shared transcript presentation.** FS-03.R73–R77
  extend the existing normalized `foldTranscript`/`appendRenderedEvent`, `nestActivities`,
  `groupTranscriptRows`/`ToolRun` and event-renderer seams; add one shared pure turn projection
  over supplied normalized events rather than a second transcript reducer. Partition root turns
  at root `turn_end`, never child `turn_end`, permission waits, `busy=false` alone or an in-turn
  `user_text` (Steer). Preserve sequence/activity identity and original event nodes. Session/backend
  boundaries fence incomplete history without claiming success; an unclosed tail remains partial
  with its original content and actionable approvals. Render one **Show activity** trigger for a
  proven completed root turn with hideable content, at its first activity position. The trigger
  controls all hideable segments of that turn in place; leave visible user/Steer/response/status
  nodes at their causal positions rather than moving or duplicating them into a summary.
  This supersedes R39's uninterrupted-only scope at the completed-turn level and R59's default
  collapsed thinking clause; both existing shared ownership and presentation-only constraints stand.
- **R101 — Visible response selection is structural and conservative.** For each
  completed root turn, select its last normalized root `assistant_text` passage as the visible
  response and hide earlier root assistant passages in activity. Consume the shared fold's merged
  passage unchanged: ephemeral thought insertion, task updates and renderer rerenders cannot
  split or concatenate durable assistant text differently. No regex, prose classifier, provider
  name/version inference, new channel, synthetic final event or API payload rewrite is introduced.
  Child assistant text stays in its attributed child scope. Calls/results/related diffs and
  completed child detail remain causally inspectable through the existing renderers inside activity.
  Root errors, terminal reasons and unresolved root/child approval controls remain outside hidden
  segments. Active background work continues through `collectTasks`/`BackgroundTaskList` with
  existing runtime authority and tool-output ownership; neither root collapse nor child disconnect
  fabricates its terminal state. Room attempt rendering scopes keys and boundaries by attempt;
  FS-21's canonical contributions/synthesis are not assistant-text activity to hide.
- **R102 — Collapse state is local, bounded and transition-owned.** Each mounted
  transcript owns choices keyed by source agent, the durable sequence of the boundary that opened
  the turn (the preceding root `turn_end` or session/backend/clone fence; `start` for the first),
  and optional child activity scope. That key exists before the turn's first durable event, so
  optimistic input never becomes an identity and its durable replacement cannot reset a live
  manual choice; no key adoption step exists. Component rerenders/refetches preserve stable
  choices. Root/child reasoning
  starts open for a new active turn; manual close applies to later spans in that scope/turn and can
  be explicitly reopened. On first observing its root terminal boundary, that turn's activity and
  nested thought/tool disclosure choices reset closed once. Subsequent replay, task updates and
  later turn endings cannot reset a manually reopened completed turn. A child terminal outcome
  means normalized terminal `activity_state` (completed/failed/stopped/disconnected), not a child
  `turn_end`; it closes thought/tool detail while retaining the attributed outcome in parent activity;
  a root terminal boundary closes all remaining detail. Permission pause is not terminal.
  Store only user overrides, with at most 256 records per mounted source and no copies of
  transcript text; evict the oldest first. Completed turns start closed, so the reset needs no
  recorded transition: hidden rows unmount and remount closed. Opened-turn choices live in the
  mounted transcript and drop on source change or unmount; live thought choices live beside the
  bounded reasoning spans and drop with them on reconnect or runtime-generation change.
  No localStorage/sessionStorage, server setting, database migration or account preference is added.
- **R103 — Ephemeral thoughts have a stable presentation turn association.** Keep
  TS-01.R35/TS-04.R63's runtime notification and reasoning retention unchanged. At live reasoning
  admission, associate each span with its root turn's presentation key and optional child scope
  only when current active-turn lifecycle and event ordering establish ownership, using the
  generation-fenced root boundary cursor; retain its anchor and existing 50-span/
  64,000-character limits. A later root turn cannot reuse a span across its boundary even if the
  provider repeats a span id. Do not attach a late delta to a completed turn or create a historical
  thought placeholder when ownership is unknown. An ambiguously owned notification may remain
  in the bounded live-only display but never be backfilled into a completed turn's activity.
  SSE reconnect/generation change still clears thoughts and live choices.
  Thought association and live manual choices are per mounted source;
  route mounts hydrate only currently available reasoning for a demonstrably active turn.
  Reasoning stays outside durable events, API reads, search, annotations and clone/context history.
- **R104 — All chat entries consume one turn contract.** Full agent chat, dashboard
  pane and Archive use the shared projection and existing `TranscriptView` renderer. The phone
  `remote/AgentScreen.tsx` retains its phone composition/permission actions but consumes the same
  projection and disclosure state policy; wire its live reasoning through the existing authenticated
  remote `/api/events` feed and shared bounded reasoning store: `remote/connection.ts` currently
  handles `new_message`, so add a `runtime_activity` handler and clear reasoning/live choices on
  its reconnect/generation boundaries, with the same limits as desktop. Do not introduce a phone-only
  final/turn classifier or new transport. Retained Think Tank attempt activity reuses the turn
  projection within its existing source-agent/attempt boundary without changing room message
  publication, retention or live reasoning availability. Empty/non-chat activity and unknown
  legacy events remain visible under their existing renderer behavior, not silently discarded.
- **R105 — The disclosure uses Chuck's existing presentation construction.** Add
  a documented `turn-activity` hook with `trigger`/`content` slots and `collapsed`/`expanded` states
  to the versioned contract during implementation; use ordinary subdued text, chevron, semantic
  tokens and existing disclosure/button construction. Each shared turn control exposes its expanded
  state and controls the in-place content regions by stable accessible ids. The open control names
  the close action (**Hide activity**); no activity yields no empty trigger. Completed tool/child/
  thought inspection stays nested and compact with existing capped ancestry. No skin branching,
  new token family, motion dependency or layout animation is required. Preserve bottom-follow;
  when reading older content, anchor on the nearest surviving visible event/control through
  automatic collapse. Focus within hidden content returns to its controlling disclosure. Reuse
  this construction at narrow pane/phone widths and in Core, Sky & Grove and Studio; revealed
  messages retain their original annotation/copy/file targets and supported read-only restrictions.
- **R106 — Closure proves lifecycle and rendered long-chat behavior.** FS-03.A54–A58
  require independently authored interleaved root/child transcripts and real runtime terminal
  scenarios, including manual choices during later deltas, terminal-only/tool-only/no-terminal
  history, optimistic input reconciliation, in-turn Steer, newer-turn completion after old activity
  is reopened, bounded-state eviction, desktop/remote reconnect/generation reset and active
  background controls. Phone tests prove live reasoning admission, bounded rendering and reset
  through its real remote connection path rather than assuming desktop SSE wiring covers it.
  Add focused projection/component tests and rendered fake-ACP desktop/dashboard/archive/phone
  journeys through the working tree's real binary. Run applicable TS-06 closure checks after the
  final implementation edit, including the presentation contract/style audit and embed generation;
  never hand-edit dist. No credentialed provider smoke is added solely for this display change.

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
  components/shell/ChuckMark.tsx
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
    skins/*.css                 declared built-in skin palette/hook overrides
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

The public `toast` state list includes the FS-14 `pipeline_needs_attention` and
`pipeline_completed` notification categories; they reuse the existing toast hook and do not create
a pipeline-specific presentation authority.

The dependency direction is:

```text
bundled fonts/assets
        +
raw core values → semantic values → shared component construction → feature composition
                                                              ↘ third-party renderer adapters

built-in skin: approved semantic overrides + scoped hook rules + decorative assets
```

### 3.4 Contract/exception manifest shapes

```jsonc
{
  "version": 2,
  "skins": ["sky-grove"],
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

### 3.5 Appearance state and CSS shape

```text
GET /api/config → React Query config cache → effective built-in id → <html data-skin="sky-grove">
                                            ↘ Core: remove data-skin

Settings mutation → optimistic cache/root marker → PUT /api/config
                  ↘ failure: restore prior cache/root marker + visible error
```

```css
/* styles/skins/sky-grove.css */
@layer ad-skins {
  :root {
    /* private --ad-sky-grove-* palette values; inert until mapped or previewed */
  }

  :root[data-skin="sky-grove"] {
    /* one raw Sky & Grove palette mapped onto approved --ad-* semantic tokens */
  }

  :root[data-skin="sky-grove"] [data-ui="agent-card"] {
    /* optional hook-scoped construction/decorative override */
  }

  [data-ui="config-editor"][data-variant="appearance"] [data-preview-skin="sky-grove"] {
    /* compact inactive preview reads the same private palette */
  }
}
```

The Go config validator, frontend effective-id allowlist, manifest, and checker fixtures carry the
same finite ids and are guarded in lockstep. This duplication is the explicit cross-language API
boundary; the manifest remains the visual contract and arbitrary ids never become CSS selectors.

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

- FS-12.R52–R59/A26–A31 and R74–R79 define the approved 2026-09-28 shared-layout upgrade.
  Implementation has not started; all remain planned. The earlier Studio-specific architecture
  stays incumbent until rollout, with the superseded preservation/oracle clauses explicitly scoped
  above. No persistence, protocol, security, retention or external-skin decision is changed.

- **Selected architecture: layered plain CSS.** CSS Modules plus a React provider were rejected
  because hashed implementation classes weaken rich skin overrides and a provider makes the core
  resemble a default theme. Runtime CSS-in-JS was rejected because it adds dependency/runtime cost
  and a broad rewrite without current runtime skin behavior. Reversing this choice is a TS-08
  architecture change.
- R30–R36 define selection and persistence only for the bundled Sky & Grove skin. Discovery, external
  packaging, compatibility negotiation beyond the in-repository manifest, third-party skin trust,
  and arbitrary skin code remain future features. The version-2 manifest stays an internal
  compatibility seam, not a promise that external skins can be loaded.
- Browser pixel-diff infrastructure is not added by this change. Deterministic visual fixtures,
  browser screenshots, the contract fixture, style lint, and existing behavior tests provide the
  acceptance evidence; adopting stored pixel baselines can be designed separately if manual visual
  comparison becomes unreliable.

- **Corrected, not extended: the sortable clause in R43.** As first written it said an expanded id
  leaves its `SortableContext`; the shipped grid keeps it, for the reason FS-02.R47 states. The
  requirement now records the shipped truth so a later reader does not "restore" the removal and
  reintroduce neighbour previews computed over a layout that is not on screen.
- **Empty space beside a pane is chosen over card movement.** R45 accepts that collapsed cards
  sharing an expanded pane's grid row sit at the top of a tall row. `dense` packing and a fixed
  `grid-auto-rows` with a multi-row span would both fill that space, and both were rejected because
  they reassign the cells of the cards after the pane. A masonry-style layout is not available.

- **The annotation tray's docked form is a registered component and a CSS state, not a
  `data-variant`.** R53 exposes the docked form and its collapsed strip through the curated contract.
  The collapsed strip is `data-state="collapsed"`/`"expanded"` on the registered `annotation-tray`
  component, but the docked-versus-overlay form deliberately carries no `data-variant`: nothing in
  the client knows which form is on screen, because the choice is a container query and R53 forbids
  measuring. Setting a variant would require the `ResizeObserver` the same requirement rules out, so
  a skin hooks the docked form through the container query it already lives in.

- **Automatic expansion is deliberately not driven by notifications.** R49 rejects reusing the
  `notification` stream that already computes the same transition on the server, because that stream
  is mute-filtered and is never replayed to a reconnecting tab. The cost is one client-side
  comparison against a per-grid record of last observed states; the benefit is that a person
  silencing a toast does not silently change what the dashboard opens.

## 6. Traceability

- Entry/build and cascade authority: `ui/src/main.tsx`, `ui/vite.config.ts`, `ui/package.json`,
  `ui/src/styles/index.css`, TS-06.R3–R5.
- Core visual source: `ui/src/styles/{foundation,tokens,base,integrations}.css`,
  `ui/src/styles/components/`, `ui/src/styles/features/`.
- Appearance bridge and finite built-in allowlist: `ui/src/features/appearance/`; bundled skin:
  `ui/src/styles/skins/sky-grove.css`.
- Shared construction, public hooks, and local assets: `ui/src/components/ui/`,
  `ui/src/presentation/contract.json`, `ui/src/assets/`.
- Third-party renderers: `AssistantText.tsx` (`react-syntax-highlighter`), `DiffBlock.tsx`
  (`react-diff-viewer-continued`), `TerminalTab.tsx` (xterm.js),
  `chat/renderers/{MermaidDiagram.tsx,mermaid.ts}` (Mermaid plus DOMPurify, R40),
  `ui/src/presentation/{integrations,resolveColors}.ts`.
- Data-driven inline styles retained by R14: `AgentCard.tsx`, `CardGrid.tsx`, `ContextBar.tsx`,
  `CardContextMenu.tsx`, `ProjectForm.tsx`, `ProjectsEditor.tsx`.
- Maintenance contract: `ui/AGENTS.md`, `ui/presentation-exceptions.json`,
  `ui/scripts/check-presentation-contract.mjs`, `ui/scripts/check-presentation-contract.test.mjs`,
  `ui/stylelint.config.mjs`, `ui/scripts/stylelint-config.test.mjs`.
- Deterministic visual evidence: `ui/src/presentation/VisualMatrix.tsx`,
  `ui/src/presentation/contract-fixture.css`, `ui/src/presentation/VisualMatrix.test.tsx`; the route
  is development-gated in `ui/src/routes.tsx` and absent from production bundles.
- Selection and live-renderer regressions: `ui/src/features/settings/AppearanceEditor.test.tsx`,
  `ui/src/features/appearance/AppearanceRoot.test.tsx`, `ui/src/components/chat/TerminalTab.test.tsx`.
