# FS-12 — Core interface design

**Status:** Partial
**Code:** `ui/src` · **Journeys:** J2–J9, J11
**Absorbed:** —

## 1. Purpose

AgentDeck needs a complete visual identity across its existing frontend. The identity must be
recognizable and distinctive without borrowing the generic appearance of an integrated development
environment (IDE), chat product, or software-as-a-service dashboard.

This first design is **AgentDeck's core interface**, not a skin. It represents the product directly:
the Dashboard remains a dashboard, agent cards remain agent cards, chat remains chat, Archive
remains Archive, and Settings remains Settings. It does not wrap those concepts in a fictional,
narrative, gaming, or real-world metaphor. Future skins may deliberately reinterpret the product;
this change only gives them a modular visual foundation beneath the core design.

The change is limited to presentation. It does not add or alter feature behavior, data, routes,
actions, interaction flows, responsive support, keyboard behavior, zoom behavior, accessibility
policy, loading/recovery behavior, or browser-native prompt/confirmation flows.

## 2. Behavior

Requirements are user-observable. Every item is unshipped and therefore marked `(planned)`.

### 2.1 Core visual direction

- **R1** `(planned)` — Every first-party frontend surface uses one product-native AgentDeck visual
  language. It is distinctive through typography, composition, geometry, color, borders, depth, and
  spacing rather than a theme, story, metaphor, or renamed product concept.
- **R2** `(planned)` — The core direction uses a light neutral canvas, near-black structural color,
  a limited high-energy accent palette, precise rules, intentional asymmetry, and a mix of crisp
  edges with restrained corner treatment. It avoids the current generic white-card presentation as
  well as common AI-product tropes such as purple/blue glow, glass panels, soft gradient clouds, and
  an all-dark IDE shell.
- **R3** `(planned)` — Typography has three consistent roles: a characterful display face for
  product, route, and agent identity; a highly readable text face for content and forms; and a
  monospaced face for ids, paths, models, metrics, commands, and event metadata. Type scale, weight,
  spacing, and alignment create hierarchy without themed labels or decorative prose.
- **R4** `(planned)` — Repeated surfaces share one coherent construction: buttons, inputs, selects,
  tabs, badges, cards, menus, dialogs, toasts, progress, tables/lists, code, terminal framing, empty
  states, and messages. Every existing visual state rendered by a component—such as selected,
  disabled, busy, error, destructive, active, stopped, or disconnected—has an intentional treatment.
- **R5** `(planned)` — Existing feature vocabulary and semantic colors remain recognizable across
  the product. Agent state, connection state, permission status, context pressure, destructive
  actions, project accents, and success/error feedback use consistent visual treatment without
  changing their current meaning or behavior.

### 2.2 Application shell

- **R6** `(planned)` — The shell has a strong AgentDeck wordmark/mark treatment, clear current-route
  navigation for Dashboard, Archive, and Settings, and an integrated connection indicator. It keeps
  the existing routes and actions; the change is their composition and appearance.
- **R7** `(planned)` — Main content uses a deliberate page frame, consistent route-heading pattern,
  and bounded content widths appropriate to each surface. Dense operational views may use the full
  canvas; forms and long-form transcript content use narrower measures. The result does not look like
  unrelated pages placed under a generic header.

### 2.3 Dashboard and agent cards

- **R8** `(planned)` — The Dashboard keeps the existing toolbar, group stack, reorderable grid,
  density controls, and New Agent flow while giving them a distinctive composition and hierarchy.
  It remains the Dashboard; no metaphorical name or framing is introduced.
- **R9** `(planned)` — Agent cards remain cards and preserve all FS-02 information. Visual priority
  is: agent name and live state; current detail/preview; role and project; backend/model/interface;
  context usage; mail indicators; and stopped state. Project color is a bounded accent that cannot
  overwhelm the card.
- **R10** `(planned)` — Card construction uses recognizable AgentDeck geometry, a clear drag grip,
  a strong state edge/marker, compact technical metadata, and a designed context meter. Waiting-input
  and error states receive higher salience without changing order, grouping, or action behavior.
- **R11** `(planned)` — Task-group headers, collapse controls, state summaries, density controls,
  and Release group share the same visual system while preserving their current placement and
  behavior. The empty Dashboard receives a complete composition with the existing New Agent action,
  not a near-empty page containing a default full-width button.

### 2.4 Chat, transcript, tracking, and terminal

- **R12** `(planned)` — The agent screen keeps the existing header, context meter, Transcript,
  Files, Commands, and conditional Terminal tabs, composer, and back navigation. Their layout and
  hierarchy become visually cohesive without renaming the screen or changing which tab opens.
- **R13** `(planned)` — Chat remains a chronological chat/transcript surface. User messages,
  assistant content, tool calls/results, diffs, permissions, errors, turn boundaries, and backend
  switches receive clearly differentiated visual components without being recast as another themed
  object or narrative concept.
- **R14** `(planned)` — Assistant Markdown has a deliberate reading measure and typographic rhythm.
  Code, tool arguments/results, commands, and diffs use a coordinated dark technical surface inside
  the otherwise light interface, with the current expand/collapse and inspection behavior unchanged.
- **R15** `(planned)` — The composer, send/cancel control, permission actions, Files and Commands
  rows, terminal frame, read-only archive label, and Resume action all use the shared component
  language. No new action, shortcut, error behavior, or interaction flow is added.

### 2.5 Archive, settings, onboarding, and overlays

- **R16** `(planned)` — Archive keeps its current search, results, metadata, active/inactive state,
  snippets, match tags, counts, and navigation. Its visual structure makes search primary and result
  hierarchy scannable without presenting Archive as a metaphorical catalog, library, timeline, or
  other themed concept.
- **R17** `(planned)` — Settings keeps its current Roles, Projects, Backends, and Notifications
  sections and all existing editor behavior. Navigation, section headers, list items, forms, backend
  and model groups, configuration-source panels, environment rows, save feedback, and destructive
  actions receive one consistent visual hierarchy suitable for dense configuration.
- **R18** `(planned)` — First-run onboarding keeps the existing non-dismissible modal, four steps,
  step order, copy, forms, validation, optional Config step, and completion behavior. The overlay,
  progress treatment, content hierarchy, and controls receive the core design without reframing the
  flow as a journey, mission, game, or story.
- **R19** `(planned)` — The New Agent modal, existing application dialogs, context menu,
  notifications, permission prompts, and error boundary use the same core design. Browser-native
  `prompt()` and `confirm()` flows remain outside this visual change and continue to behave as
  specified by their owning features.

### 2.6 Boundary for future skins

- **R20** `(planned)` — The delivered interface is the unskinned AgentDeck core. No skin is active
  by default, and this change adds no skin picker, stored skin preference, project-specific skin,
  downloadable asset, marketplace, import, or runtime skin-switching behavior.
- **R21** `(planned)` — Core product semantics are independent from visual expression. Content,
  state text, actions, validation, routes, and component structure are defined by AgentDeck; the
  core design supplies their default presentation. A future skin may override approved visual
  values and decorative assets, but may not be required for the product to render correctly.
- **R22** `(planned)` — Future skins may introduce strong concepts or themed interpretations. The
  core design does not pre-empt that layer by embedding its own fictional terminology, themed copy,
  narrative illustrations, or concept-specific component names into product structure.

## 3. States & transitions

- **Route change `(planned)`:** the persistent shell remains visually stable while the current-route
  treatment and page frame change to the selected existing surface.
- **Agent state `(planned)`:** existing busy, idle, waiting-input, done, error, unknown, running, and
  stopped values change the shared card/badge treatment without introducing a new state or transition.
- **Component state `(planned)`:** existing selected, expanded, collapsed, disabled, busy,
  destructive, success, and failure states use the core component language while retaining their
  owning feature's behavior.
- **Overlay `(planned)`:** existing modals, menus, permission prompts, and toasts appear above the
  shell with a consistent depth and surface treatment; their open/close behavior is unchanged.

## 4. Edge cases & errors

- **R23** `(planned)` — Empty, missing, or unknown values that already have a rendered fallback use
  a deliberate visual placeholder instead of producing broken geometry, blank badges, or
  `undefined` text. This requirement does not add new data-recovery behavior.
- **R24** `(planned)` — Long names, paths, models, commands, snippets, and messages continue to use
  each owning feature's existing wrapping, truncation, expansion, or scroll behavior; the new design
  must not make that behavior visibly worse by overlapping controls or escaping its component.
- **R25** `(planned)` — Terminal, syntax highlighting, diffs, permission prompts, error treatments,
  project colors, and all agent states remain legible against the core palette. This is a visual
  compatibility requirement, not a new contrast or accessibility policy.

## 5. Acceptance criteria

- **A1** (R1–R19, R23–R25) `(planned)` — A real-browser visual review covers onboarding, empty and
  populated Dashboard, all agent states, New Agent, chat event variants, Files, Commands, Terminal,
  active and archived sessions, every Settings section, menus, notifications, permissions, and
  representative errors. Every first-party surface clearly belongs to one core AgentDeck design and
  none uses a metaphorical skin concept. *Verify:* visual fixture/screenshot matrix plus existing
  journeys J2–J9 and J11 for behavioral regression.
- **A2** (R2–R5) `(planned)` — The shell, controls, cards, messages, technical content, forms, and
  overlays demonstrably share the chosen typography, geometry, palette, border/depth, spacing, and
  component-state rules. *Verify:* component visual matrix and design review against the approved
  core direction.
- **A3** (R8–R11) `(planned)` — Dashboard fixtures cover empty and grouped/populated states, every
  density extreme, every agent state, project accents, context ranges, terminal, unread mail, sent,
  stopped, and dragging without changing FS-02 behavior. *Verify:* component tests, visual fixtures,
  and J5.
- **A4** (R12–R15) `(planned)` — One agent-screen fixture displays every normalized transcript
  event, pending/resolved permission, long Markdown, code, diff, tool content, Files, Commands,
  Terminal, composer states, and read-only archive controls in the core design. *Verify:* component
  tests, visual fixtures, and J3, J4, J6, J7, and J8.
- **A5** (R16–R19) `(planned)` — Archive, every Settings editor, all four onboarding steps, New
  Agent, existing application overlays, notifications, and error boundary retain their existing
  behavior and use the shared core design. Browser-native prompt/confirm behavior is unchanged and
  explicitly excluded. *Verify:* existing feature tests, visual fixtures, J2, J8, and J9.
- **A6** (R20–R22) `(planned)` — The application renders the complete core design without an active
  skin or user-visible skin control. A test-only visual override can change approved presentation
  values without changing product copy, routes, actions, state meaning, or component structure.
  *Verify:* technical skin-boundary contract test defined by the matching TS.
- **A7** (R1, R4, R19) `(planned)` — Every literal `className` used by redesigned components
  resolves to a defined selector, and obsolete core-design selectors are removed. *Verify:* the
  stylesheet/class audit required by INV §13 plus the real-browser visual review.

## 6. Deviations & open decisions

- The previous Field Atlas proposal was rejected because it made the default design a conceptual
  skin. This revision defines a product-native core interface and removes the proposed expedition,
  dispatch, dossier, field-log, catalog, workshop, and journey metaphors.
- Responsive targets, phone behavior, keyboard-flow improvements, focus management, zoom support,
  reduced-motion policy, new loading/recovery states, dedicated replacements for browser-native
  prompt/confirm flows, and other quality-of-life changes are explicitly outside this visual change.
- The exact core visual direction and its behavior-preserving scope await human confirmation.
  Technical component/token/skin boundaries are intentionally deferred until then.

## 7. Traceability

- Current shell and routes: `ui/src/App.tsx`, `ui/src/routes.tsx`,
  `ui/src/components/shell/`.
- Current visual source: `ui/src/styles/tokens.css`, `ui/src/styles/global.css`.
- Current product surfaces: `ui/src/components/grid/`, `ui/src/components/chat/`,
  `ui/src/features/{archive,launch,onboarding,settings}/`.
- Cross-cutting UI bug classes: INV §8, §10, §11, and §13.
- Planned implementation/test anchors will be added when the confirmed technical design ships.
