# FS-12 — Core interface design

**Status:** Current
**Code:** `ui/src` · **Journeys:** J2–J9, J11, J14
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

The original core-design change was limited to presentation. It did not add or alter feature
behavior, data, routes, actions, interaction flows, responsive support, keyboard behavior, zoom
behavior, accessibility policy, or loading/recovery behavior, and it explicitly left browser-native
prompt/confirmation flows unchanged. The later §2.7 requirement (R26) reverses only that
last exclusion, moving those flows into the core Dialog system without changing any request they
issue.

## 2. Behavior

Requirements are user-observable.

### 2.1 Core visual direction

- **R1** — Every first-party frontend surface uses one product-native AgentDeck visual
  language. It is distinctive through typography, composition, geometry, color, borders, depth, and
  spacing rather than a theme, story, metaphor, or renamed product concept.
- **R2** — The core direction uses a light neutral canvas, near-black structural color,
  a limited high-energy accent palette, precise rules, intentional asymmetry, and a mix of crisp
  edges with restrained corner treatment. It avoids the current generic white-card presentation as
  well as common AI-product tropes such as purple/blue glow, glass panels, soft gradient clouds, and
  an all-dark IDE shell.
- **R3** — Typography has three consistent roles: a characterful display face for
  product, route, and agent identity; a highly readable text face for content and forms; and a
  monospaced face for ids, paths, models, metrics, commands, and event metadata. Type scale, weight,
  spacing, and alignment create hierarchy without themed labels or decorative prose.
- **R4** — Repeated surfaces share one coherent construction: buttons, inputs, selects,
  tabs, badges, cards, menus, dialogs, toasts, progress, tables/lists, code, terminal framing, empty
  states, and messages. Every existing visual state rendered by a component—such as selected,
  disabled, busy, error, destructive, active, stopped, or disconnected—has an intentional treatment.
- **R5** — Existing feature vocabulary and semantic colors remain recognizable across
  the product. Agent state, connection state, permission status, context pressure, destructive
  actions, project accents, and success/error feedback use consistent visual treatment without
  changing their current meaning or behavior.

### 2.2 Application shell

- **R6** — The shell has a strong AgentDeck wordmark/mark treatment, clear current-route
  navigation for Dashboard, Pipelines, Archive, and Settings, and an integrated connection indicator. It keeps
  the existing routes and actions; the change is their composition and appearance.
- **R7** — Main content uses a deliberate page frame, consistent route-heading pattern,
  and bounded content widths appropriate to each surface. Dense operational views may use the full
  canvas; forms and long-form transcript content use narrower measures. The result does not look like
  unrelated pages placed under a generic header.

### 2.3 Dashboard and agent cards

- **R8** — The Dashboard keeps the existing toolbar, group stack, reorderable grid,
  density controls, and New Agent flow while giving them a distinctive composition and hierarchy.
  It remains the Dashboard; no metaphorical name or framing is introduced.
- **R9** — Agent cards remain cards and preserve all FS-02 information. Visual priority
  is: agent name and live state; current detail/preview; role and project; backend/model/interface;
  context usage; mail indicators; and stopped state. Project color is a bounded accent that cannot
  overwhelm the card.
- **R10** — Card construction uses recognizable AgentDeck geometry, a clear drag grip,
  a strong state edge/marker, compact technical metadata, and a designed context meter. Waiting-input
  and error states receive higher salience without changing order, grouping, or action behavior.
- **R37** — R10's "without changing order" clause is narrowed to the five live
  `state` values it was written about: `busy`, `idle`, `waiting_input`, `done`, and `error` still
  express themselves through salience alone and never reorder cards. Whether an agent is running is
  a separate axis and may order cards, as FS-02.R45 specifies. Grouping and action behavior remain
  unchanged by either axis, so raising a card's salience still never moves it and never changes what
  its menu offers.
- **R11** — Task-group headers, collapse controls, state summaries, density controls,
  and Release group share the same visual system while preserving their current placement and
  behavior. The empty Dashboard receives a complete composition with the existing New Agent action,
  not a near-empty page containing a default full-width button.

### 2.4 Chat, transcript, tracking, and terminal

- **R12** — The agent screen keeps the existing header, context meter, Transcript,
  Files, Commands, and conditional Terminal tabs, composer, and back navigation. Their layout and
  hierarchy become visually cohesive without renaming the screen or changing which tab opens.
- **R13** — Chat remains a chronological chat/transcript surface. User messages,
  assistant content, tool calls/results, diffs, permissions, errors, turn boundaries, and backend
  switches receive clearly differentiated visual components without being recast as another themed
  object or narrative concept.
- **R14** — Assistant Markdown has a deliberate reading measure and typographic rhythm.
  Code, tool arguments/results, commands, and diffs use a coordinated dark technical surface inside
  the otherwise light interface, with the current expand/collapse and inspection behavior unchanged.
- **R15** — The composer, send/cancel control, permission actions, Files and Commands
  rows, terminal frame, read-only archive label, and Resume action all use the shared component
  language. No new action, shortcut, error behavior, or interaction flow is added.

### 2.5 Archive, settings, onboarding, and overlays

- **R16** — Archive keeps its current search, results, metadata, active/inactive state,
  snippets, match tags, counts, and navigation. Its visual structure makes search primary and result
  hierarchy scannable without presenting Archive as a metaphorical catalog, library, timeline, or
  other themed concept.
- **R17** — Settings keeps its current Roles, Projects, Backends, and Notifications
  sections and all existing editor behavior. Navigation, section headers, list items, forms, backend
  and model groups, configuration-source panels, environment rows, save feedback, and destructive
  actions receive one consistent visual hierarchy suitable for dense configuration.
- **R18** — First-run onboarding keeps the existing non-dismissible modal, four steps,
  step order, copy, forms, validation, optional Config step, and completion behavior. The overlay,
  progress treatment, content hierarchy, and controls receive the core design without reframing the
  flow as a journey, mission, game, or story.
- **R19** — The New Agent modal, existing application dialogs, context menu,
  notifications, permission prompts, and error boundary use the same core design. The initial
  core-design delivery excluded browser-native `prompt()` and `confirm()` flows; R26 supersedes that
  exclusion with their application-dialog replacement.

### 2.6 Boundary for future skins

- **R20** — The delivered interface is the unskinned AgentDeck core. No skin is active
  by default, and this change adds no skin picker, stored skin preference, project-specific skin,
  downloadable asset, marketplace, import, or runtime skin-switching behavior.
- **R21** — Core product semantics are independent from visual expression. Content,
  state text, actions, validation, routes, and component structure are defined by AgentDeck; the
  core design supplies their default presentation. A future skin may override approved visual
  values and decorative assets, but may not be required for the product to render correctly.
- **R22** — Future skins may introduce strong concepts or themed interpretations. The
  core design does not pre-empt that layer by embedding its own fictional terminology, themed copy,
  narrative illustrations, or concept-specific component names into product structure.

### 2.7 Application dialogs (native-prompt replacement)

- **R26** — Every first-party input and confirmation flow uses the core application
  Dialog system instead of a browser-native `window.prompt()` or `confirm()`, and no first-party
  module invokes `window.prompt`/`confirm`. Each dialog renders as a core overlay (focus trap,
  `Escape`/overlay dismissal, exactly one confirm control and one Cancel control), surfaces the
  owning feature's existing validation as field-level messages, and treats Cancel or dismissal as
  performing no action. This supersedes R19's exclusion of browser-native prompt/confirm flows and
  the "prompt-based UI" deviations recorded by FS-01 §6, FS-02 §6, and FS-04 §6. The individual
  input and confirmation dialogs and their validation are owned by FS-01.R32 (rename, switch
  runtime), FS-02.R37 (move to group, project rename/color, stop, release group, archive project),
  and FS-04.R37 (delete role/project, delete-in-use, archive project).

### 2.8 First optional skin

- **R27** — AgentDeck offers one optional built-in skin named **Sky & Grove**
  alongside **AgentDeck Core**. Core remains the initial selection for an install with no stored
  preference and remains the safe fallback; adding the skin does not reinterpret Core as a skin or
  make optional skin code necessary for the application to render.
- **R28** — Sky & Grove is an airy sky-blue and nature-green design, not a
  simple accent-color swap. It uses a clear blue canvas and layered pale-blue surfaces, deep
  evergreen structure and primary-action accents, softer organic geometry, and restrained
  botanical or topographic decoration. It keeps technical content crisp and gives warning, error,
  destructive, connection, permission, agent-state, context-pressure, and project colors enough
  separation that green or blue never changes their product meaning.
- **R29** — Settings gains an **Appearance** destination with an explicit
  choice between AgentDeck Core and Sky & Grove, including a compact visual sample of each. Choosing
  an option applies it across the currently open application immediately, without a reload or
  server restart, and the control always identifies the active choice by text rather than color
  alone.
- **R30** — The appearance choice is one durable, AgentDeck-wide preference,
  not a browser-only or project-specific value. It applies to every route, project, and agent and is
  reused by later browser sessions after configuration loads. An absent preference selects Core.
- **R31** — Sky & Grove covers every first-party surface in R1–R19 and the
  syntax, diff, and terminal integrations. Switching appearance changes presentation only: product
  copy, routes, actions, component state, feature data, focus behavior, and content structure remain
  unchanged, and the application dialogs in R26 adopt the selected appearance without changing
  their validation or consequences.
- **R32** — A missing, unknown, or unreadable stored skin id cannot prevent
  first paint or replace the application with an error boundary. AgentDeck renders Core, identifies
  the unavailable preference in Settings, and lets the person choose and save a valid appearance.
  If saving a new choice fails, the UI reports the failure and returns to the last durably selected
  appearance rather than presenting an unsaved selection as permanent.
- **R33** — This first skin adds no operating-system light/dark following,
  per-project choice, schedules, user-authored CSS, arbitrary skin code, downloads, imports,
  marketplace, third-party package discovery, or theme-specific product vocabulary. All skin CSS,
  fonts, and decorative assets ship locally and work without network access.
- **R34** — Tool calls and tool outcomes use compact, muted transcript rows that read as
  secondary activity rather than dark code or terminal panels. Arguments and non-empty results
  remain expandable for inspection; diffs, fenced code, commands, and the Terminal retain their
  technical treatment. An outcome with no displayable payload carries a short status label instead
  of blank geometry. This supersedes R14 only for tool calls and tool results.
- **R35** — An uninterrupted tool run is a single subdued **Ran _n_ tools** row by
  default, inviting disclosure without competing with the conversation. Opening the row reveals
  the existing compact tool-call and non-empty/failed result rows; a successful no-payload result
  adds no visual row. This supersedes R34's no-payload status label.
- **R36** — Tool-run summaries, tool calls, and tool results render as regular subdued text rather
  than coloured or enclosed surfaces. Disclosure, indentation, and semantic error text remain
  available without adding a tinted background or box. This refines R35.

- **R38** — R15's closing clause — that the chat surface adds no new
  action, shortcut, error behavior, or interaction flow — is scoped to the presentation change R15
  was written for, which restyled the shipped chat controls without altering them. It is not a
  standing ban on the chat surface ever gaining an interaction. FS-02.R50's focus cycling between
  expanded panes and FS-02.R52's activation boundary are additions to the dashboard card grid that
  reuse the composer and transcript; they are governed by FS-02 and FS-03 and do not contradict R15,
  whose components keep the shared component language R15 actually requires.

### 2.9 Active-project navigation

- **R39** — The shell places FS-02.R54's active-project links immediately to the right
  of the existing primary route tabs in one stable header row. Project links are visibly smaller
  than the primary tabs, use restrained rounded corners and a bounded tint/edge from the project's
  configured accent, and retain a non-color current-route indicator. It shows at most five project
  links, keeps the current project among them, and groups every remaining link under a compact `+n`
  overflow control that names the hidden count and opens keyboard-accessible navigation to each
  hidden project. Long project titles truncate visually while retaining their full accessible name.
  The project area is absent when no link is eligible, never wraps the header, and fits five links
  plus overflow without overlapping or hiding the primary tabs, AgentDeck mark, connection state,
  or current-project indicator at the supported desktop floor. Link membership and routing remain
  feature-owned; presentation does not measure available width, persist state, fetch independently,
  or reinterpret project activity.

## 3. States & transitions

- **Route change:** the persistent shell remains visually stable while the current-route
  treatment and page frame change to the selected existing surface.
- **Agent state:** existing busy, idle, waiting-input, done, error, unknown, running, and
  stopped values change the shared card/badge treatment without introducing a new state or transition.
- **Component state:** existing selected, expanded, collapsed, disabled, busy,
  destructive, success, and failure states use the core component language while retaining their
  owning feature's behavior.
- **Overlay:** existing modals, menus, permission prompts, and toasts appear above the
  shell with a consistent depth and surface treatment; their open/close behavior is unchanged.
- **Appearance selection:** choosing Core or Sky & Grove applies the complete selected
  presentation to the mounted application and saves the global preference; a later session restores
  it after configuration loads.

## 4. Edge cases & errors

- **R23** — Empty, missing, or unknown values that already have a rendered fallback use
  a deliberate visual placeholder instead of producing broken geometry, blank badges, or
  `undefined` text. This requirement does not add new data-recovery behavior.
- **R24** — Long names, paths, models, commands, snippets, and messages continue to use
  each owning feature's existing wrapping, truncation, expansion, or scroll behavior; the new design
  must not make that behavior visibly worse by overlapping controls or escaping its component.
- **R25** — Terminal, syntax highlighting, diffs, permission prompts, error treatments,
  project colors, and all agent states remain legible against the core palette. This is a visual
  compatibility requirement, not a new contrast or accessibility policy.

## 5. Acceptance criteria

- **A1** (R1–R19, R23–R25) — A real-browser visual review covers onboarding, empty and
  populated Dashboard, Pipelines, all agent states, New Agent, chat event variants, Files, Commands, Terminal,
  active and archived sessions, every Settings section, menus, notifications, permissions, and
  representative errors. Every first-party surface clearly belongs to one core AgentDeck design and
  none uses a metaphorical skin concept. *Verify:* visual fixture/screenshot matrix plus existing
  journeys J2–J9, J11, and J14 for behavioral regression.
- **A2** (R2–R5) — The shell, controls, cards, messages, technical content, forms, and
  overlays demonstrably share the chosen typography, geometry, palette, border/depth, spacing, and
  component-state rules. *Verify:* component visual matrix and design review against the approved
  core direction.
- **A3** (R8–R11) — Dashboard fixtures cover empty and grouped/populated states, every
  density extreme, every agent state, project accents, context ranges, terminal, unread mail, sent,
  stopped, and dragging without changing FS-02 behavior. *Verify:* component tests, visual fixtures,
  and J5.
- **A4** (R12–R15) — One agent-screen fixture displays every normalized transcript
  event, pending/resolved permission, long Markdown, code, diff, tool content, Files, Commands,
  Terminal, composer states, and read-only archive controls in the core design. *Verify:* component
  tests, visual fixtures, and J3, J4, J6, J7, and J8.
- **A5** (R16–R19) — Archive, every Settings editor, all four onboarding steps, New
  Agent, existing application overlays, notifications, and error boundary retain their existing
  behavior and use the shared core design. *Verify:* existing feature tests, visual fixtures, J2,
  J8, and J9.
- **A6** (R20–R22) — The application renders the complete core design without an active
  skin or user-visible skin control. A test-only visual override can change approved presentation
  values without changing product copy, routes, actions, state meaning, or component structure.
  *Verify:* technical skin-boundary contract test defined by the matching TS.
- **A7** (R1, R4, R19) — Every literal `className` used by redesigned components
  resolves to a defined selector, and obsolete core-design selectors are removed. *Verify:* the
  stylesheet/class audit required by INV §13 plus the real-browser visual review.
- **A8** (R26) — No first-party module under `ui/src` calls `window.prompt`/`confirm`, and each
  replaced flow opens a core dialog that validates, confirms, and performs no side effect on Cancel.
  *Verify:* a static source guard test asserting the absence of `prompt`/`confirm` calls in
  first-party UI, the per-dialog component tests named by FS-01.A16, FS-02.A21, and FS-04.A17, and a
  real-browser pass of the rename, switch-runtime, move-to-group, and destructive-confirm flows.
- **A9** (R27, R29–R30) — A person can select Sky & Grove in Settings, see the
  mounted application change without reload, navigate through every route with the choice intact,
  and open a second browser session or reload to recover the same stored choice; clearing the
  preference restores Core. *Verify:* Settings/component tests, configuration round-trip tests, and
  a real-browser appearance-switch journey.
- **A10** (R28, R31, R33) — A deterministic visual matrix renders the core and
  Sky & Grove versions of the shell, Dashboard extremes, agent screen and transcript variants,
  Pipelines, Archive, Settings, onboarding, overlays, syntax, diffs, and terminal. Review confirms
  the approved blue/green direction, distinct semantic states and project colors, unchanged
  content/actions/structure, and no network-loaded asset. *Verify:* paired visual fixtures,
  presentation-contract tests, and real-browser screenshots at the existing desktop floor.
- **A11** (R32) — Missing/unknown/unreadable appearance configuration and an
  injected save failure each leave a usable Core application; Settings explains the unavailable or
  unsaved choice and can recover by saving a valid option. *Verify:* configuration/API and Settings
  regressions plus a first-paint browser smoke test.
- **A12** (R36) — Tool-run summary, call, result, and failure fixtures remain visible as plain
  text without a coloured or boxed surface. *Verify:* the tool-activity states in
  `ui/src/presentation/VisualMatrix.tsx`.

- **A13** (R37) — A card whose agent is `waiting_input` or `error` renders its
  higher-salience treatment while holding its position, and a card's position changes only when its
  `running` value changes. *Verify by* the FS-02.A28 grid cases together with the existing card
  salience fixtures in the visual matrix.

- **A14** (R38) — The agent screen's composer, send/cancel control, and
  permission actions expose the same actions and shortcuts after the pane work as before it: the
  focus-cycling binding does nothing on `/agent/:id`, and no chat control gains or loses an
  interaction there. — `ChatPanel.test.tsx` and `Composer.test.tsx`.

- **A15** (R39) — Core and Sky & Grove fixtures render zero, one, and overflowing sets
  of active-project links in the shipped header at the supported desktop floor and a wider desktop
  viewport. The current project remains directly visible and has a text-independent selected state;
  every visible and overflowed project is reachable by keyboard and exposes its project title;
  project accents remain distinguishable without becoming the selected-state signal; and no case
  wraps, clips, overlaps, or displaces the primary navigation, mark, or connection state. *Verify:*
  shell/component tests, the paired visual matrix, and a real-browser J5 pass.

## 6. Deviations & open decisions

- The previous Field Atlas proposal was rejected because it made the default design a conceptual
  skin. This revision defines a product-native core interface and removes the proposed expedition,
  dispatch, dossier, field-log, catalog, workshop, and journey metaphors.
- Responsive targets, phone behavior, keyboard-flow improvements, focus management, zoom support,
  reduced-motion policy, new loading/recovery states, and other quality-of-life changes are
  explicitly outside the original core-design change. Dedicated replacements for browser-native
  prompt/confirm flows are no longer deferred: they are specified by R26 in §2.7.
- The confirmed core direction is the product-native light-canvas system described above. Its
  behavior-preserving token, component, integration, and future-skin boundaries are defined by
  TS-08.
- R27–R33 and A9–A11 are the confirmed first use of that future-skin boundary. The human confirmed
  the Sky & Grove name, Core default/fallback, global server-stored preference, Settings-only
  selection, unchanged product vocabulary, and explicit exclusions on 2026-07-30. TS-02.R21,
  TS-03.R21, and TS-08.R30–R36 define the matching technical boundary.
- R39 and A15 specify the confirmed same-row compact active-project navigation and `+n` overflow.
  FS-02.R54 fixes title/id alphabetical ordering and keeps the current project directly visible;
  no product decision remains open before technical design.

## 7. Traceability

- Shell, routes, and core mark: `ui/src/App.tsx`, `ui/src/routes.tsx`,
  `ui/src/components/shell/`.
- Core visual source and shared construction: `ui/src/styles/`, `ui/src/components/ui/`.
- Product surfaces: `ui/src/components/{grid,chat}/`,
  `ui/src/features/{archive,launch,onboarding,settings}/`.
- Appearance activation and bundled skin: `ui/src/features/appearance/`,
  `ui/src/features/settings/AppearanceEditor.tsx`, `ui/src/styles/skins/sky-grove.css`.
- Deterministic browser evidence: `ui/src/presentation/VisualMatrix.tsx`,
  `ui/src/presentation/contract-fixture.css`, `ui/src/presentation/VisualMatrix.test.tsx`.
- Appearance behavior regressions: `ui/src/features/appearance/AppearanceRoot.test.tsx`,
  `ui/src/features/settings/AppearanceEditor.test.tsx`,
  `ui/src/components/chat/TerminalTab.test.tsx`.
- Presentation completeness and visual-value enforcement:
  `ui/scripts/check-presentation-contract.mjs`, `ui/scripts/check-presentation-contract.test.mjs`,
  `ui/stylelint.config.mjs`, `ui/scripts/stylelint-config.test.mjs`.
- Salience-versus-order separation: `ui/src/components/grid/CardGrid.test.tsx` (a raised-salience
  card holds its position; only `running` moves one — A13).
- Cross-cutting UI bug classes: INV §8, §10, §11, and §13.
