# TS-08 — Frontend presentation architecture

**Status:** Current
**Code:** `ui/src`, `ui/package.json`, `ui/vite.config.ts`
**Absorbed:** —

## 1. Scope

This specification defines how AgentDeck's confirmed core interface design is represented in the
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
  | Primary / secondary / muted text | `#17332f` / `#34524e` / `#697e7b` |
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
