# FS-12 — Core interface design

**Status:** Current
**Code:** `ui/src` · **Journeys:** J2–J9, J11, J14
**Absorbed:** —

## 1. Purpose

Chuck needs a complete visual identity across its existing frontend. The identity must be
recognizable and distinctive without borrowing the generic appearance of an integrated development
environment (IDE), chat product, or software-as-a-service dashboard.

This first design is **Chuck's core interface**, not a skin. It represents the product directly:
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

- **R1** — Every first-party frontend surface uses one product-native Chuck visual
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

- **R6** — The shell has a strong Chuck wordmark/mark treatment, clear current-route
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
- **R10** — Card construction uses recognizable Chuck geometry, a clear drag grip,
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

- **R40** — R9's visual priority for agent cards is narrowed by
  FS-02.R58 and R59. The agent name keeps its first position but is set at a smaller size and wraps
  onto as many as three lines, so a long name reads in full instead of ending in an ellipsis; it
  therefore no longer competes with the state badge for the header's single line. Context usage
  leaves the collapsed card and appears on the expanded card as a compact figure, so it holds no
  position in the collapsed card's priority list; every other item in R9's order — live state,
  detail/preview, role and project, backend/model/interface, mail indicators, and stopped state —
  keeps its place, and R10's card construction, drag grip, state edge, and salience treatment are
  unchanged. R24's rule still holds: this is the owning feature's wrapping behavior, and it may not
  overlap a control or escape the card.

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

- **R20** — The delivered interface is the unskinned Chuck core. No skin is active
  by default, and this change adds no skin picker, stored skin preference, project-specific skin,
  downloadable asset, marketplace, import, or runtime skin-switching behavior.
- **R21** — Core product semantics are independent from visual expression. Content,
  state text, actions, validation, routes, and component structure are defined by Chuck; the
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

- **R27** — Chuck offers one optional built-in skin named **Sky & Grove**
  alongside **Chuck Core**. Core remains the initial selection for an install with no stored
  preference and remains the safe fallback; adding the skin does not reinterpret Core as a skin or
  make optional skin code necessary for the application to render.
- **R28** — Sky & Grove is an airy sky-blue and nature-green design, not a
  simple accent-color swap. It uses a clear blue canvas and layered pale-blue surfaces, deep
  evergreen structure and primary-action accents, softer organic geometry, and restrained
  botanical or topographic decoration. It keeps technical content crisp and gives warning, error,
  destructive, connection, permission, agent-state, context-pressure, and project colors enough
  separation that green or blue never changes their product meaning.
- **R29** — Settings gains an **Appearance** destination with an explicit
  choice between Chuck Core and Sky & Grove, including a compact visual sample of each. Choosing
  an option applies it across the currently open application immediately, without a reload or
  server restart, and the control always identifies the active choice by text rather than color
  alone.
- **R30** — The appearance choice is one durable, Chuck-wide preference,
  not a browser-only or project-specific value. It applies to every route, project, and agent and is
  reused by later browser sessions after configuration loads. An absent preference selects Core.
- **R31** — Sky & Grove covers every first-party surface in R1–R19 and the
  syntax, diff, and terminal integrations. Switching appearance changes presentation only: product
  copy, routes, actions, component state, feature data, focus behavior, and content structure remain
  unchanged, and the application dialogs in R26 adopt the selected appearance without changing
  their validation or consequences.
- **R32** — A missing, unknown, or unreadable stored skin id cannot prevent
  first paint or replace the application with an error boundary. Chuck renders Core, identifies
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
  plus overflow without overlapping or hiding the primary tabs, Chuck mark, connection state,
  or current-project indicator at the supported desktop floor. Link membership and routing remain
  feature-owned; presentation does not measure available width, persist state, fetch independently,
  or reinterpret project activity.
- **R41** — A pointer-anchored menu opens fully inside the viewport. One shared
  placement helper measures the rendered menu and shifts it back from the bottom and right edges
  rather than reordering or flipping its items, so a menu opened near an edge exposes the same
  actions in the same order as one opened in the middle. A menu taller than the viewport pins to the
  leading margin and scrolls, so no item is unreachable. This governs every pointer-anchored menu:
  agent cards, project cards, the project-dashboard background, and transcript annotations.

### 2.10 Third optional skin

- **R42 (shipped 2026-09-23)** — Chuck offers a third, optional built-in skin named **Studio** alongside Chuck Core
  and Sky & Grove. Core remains the default for an absent preference and the safe fallback for an
  unavailable one; both existing appearances remain selectable and visually unchanged. This
  supersedes only R27's count of optional skins and R29's two-choice limit. The new skin uses the
  existing Chuck-wide Appearance preference, applies immediately throughout the mounted app,
  and survives reload and later sessions under R30 and FS-04.R38.
- **R43 (shipped 2026-09-23)** — Studio presents a light, spacious, polished creative workspace: soft
  off-whites and pale blue-green surfaces, forest-green supporting color, and restrained saturated
  emphasis. Its open workspace canvas uses a simple, consistent, low-contrast dot grid; readable
  cards, transcript, forms, overlays, and technical content keep opaque surfaces. Generous spacing,
  friendly legible type, rounded but non-pill geometry, quiet chrome, and selective depth make the
  work dominant without generic card grids, neon, AI gradients, glass, or an IDE-like shell. It is
  visibly distinct from Sky & Grove's stronger blue canvas and botanical/topographic decoration.
- **R44 (shipped 2026-09-23)** — The Figma Make exploration guides Studio's visual direction, not its
  product anatomy. Existing Chuck routes, copy, actions, card density and grouping, live
  states, permissions, chat chronology, composer, Files, Commands, conditional Terminal, and
  archive behavior remain authoritative. In particular, an expanded dashboard card retains its
  real interactive chat transcript and controls; the cramped transcript-like area in the Make
  mockup is not copied. The skin does not introduce a new dashboard pane, synthetic transcript,
  decorative replacement for content, or a behavior change.
- **R45 (shipped 2026-09-23)** — Studio treats running, waiting-input, error, stopped, success, warning,
  destructive, connection, permission, context pressure, and project accents as distinct semantic
  information. Color never becomes the only cue; long names, dense technical output, empty states,
  and narrow supported desktop layouts remain legible without clipped controls or escaped content.
  Every first-party route, dialog, menu, notification, onboarding step, syntax/diff renderer, and
  terminal surface receives the selected appearance without a network-loaded asset.

### 2.11 Studio composition upgrade

- **R46 (shipped 2026-09-25)** — Studio is a compositional alternative, not merely a recoloring of Core.
  Its shell uses a quieter, more compact single-row header with less boxed navigation; route titles
  and section headings use a friendly, readable hierarchy (route titles around 26–32px, body copy
  no smaller than 14px) instead of Core's oversized display and dense uppercase technical
  treatment. The canvas remains visually open, with substantial space
  between meaningful groups rather than a repeated equal-card frame. Route identity, project links,
  connection state, navigation order, and accessible current-route cues remain intact. Core and
  Sky & Grove retain their shipped appearance. This extends R43 beyond palette and ornament.
- **R47 (shipped 2026-09-25)** — Studio's Dashboard makes the agent work—not chrome or card borders—the first
  scan target. Group headings are quiet organizing landmarks; a collapsed card presents agent name
  and live state first, a readable current detail/preview second, and role/project/runtime/mail
  metadata as subordinate information, with the existing project and state accents bounded.
  Spacing and surface treatment distinguish card header, conversation preview, and metadata without
  adding nested generic cards or hiding content. All existing dashboard/project views, grouping,
  density values, drag affordances, ordering, collapsed/expanded states, and actions remain.
- **R48 (shipped 2026-09-25)** — Studio's expanded dashboard card and full agent screen read as actual chat
  workspaces. The existing chronological `TranscriptView` receives a clear reading region with
  message and tool hierarchy; the existing `Composer` reads as its anchored continuation, not a
  separate utility panel or a mock transcript. The card header and full-screen header/tabs are
  visually subordinate to the conversation while retaining state, context, controls, Files,
  Commands, conditional Terminal, permissions, annotations, and archived read-only behavior. The
  expanded card remains in its existing grid track, with the same scroll, focus, activation, and
  neighboring-card stability behavior; no chat action or event projection changes.
- **R49 (shipped 2026-09-25)** — Studio gives each remaining destination a deliberate content hierarchy:
  Tasks reads as an attention-first work queue, not a stack of equal cards; Pipeline Runs reads as
  an operational ledger and run timeline, while Templates/editor reads as an authoring surface;
  Archive is search-first with scan-friendly result rows; Settings uses a quiet navigation spine
  and readable configuration measures; onboarding and dialogs have clear step/form/action
  hierarchy. This applies to populated, empty, attention/error, long-content, and narrow desktop
  states without removing fields, outcomes, actions, tabs, validation, or recovery feedback.
  Existing route and task flows do not change.

- **R50 (shipped 2026-09-26) — Setup forms prioritize an experienced operator's decisions.** The compact
  setup changes in FS-01.R37, FS-04.R49, FS-08.R35–R36, FS-14.R80 and FS-16.R39–R40 use existing
  form, disclosure and dialog treatments across Core, Sky & Grove and Studio. Reading and keyboard
  order follow the task: workspace/role or work instruction, selected runtime/dependency summary,
  optional detail, action. Collapsed detail is one directly labelled interaction away; its values
  survive toggles and important errors/warnings remain visible. Duplicate-name disambiguation and
  active overrides remain legible with long real names. No tutorial, new skin, extra confirmation
  ceremony, animation, or stored beginner/expert mode is introduced. This narrowly supersedes
  R17/R18's preservation of the old editor/onboarding composition; the owning feature requirements
  define the changed behavior, including skipping unsupported Config steps.

- **R51 (shipped 2026-09-26) — Chat chrome yields space to the conversation.** The full agent workspace keeps
  one compact header whose left reading band contains Back, agent identity, and the existing runtime
  and live-session controls, while context stays subordinate on the right. The composition may wrap
  for long names, errors, or narrow supported desktop widths but does not hide, reorder, or change
  the apply model of any control. Header identity-copy and transcript selection-copy actions use
  the existing pointer-menu construction across Core, Sky & Grove, and Studio; no new motion,
  context-menu language, route, or transcript behavior is introduced (FS-03.R62–R63).

### 2.12 Shared creative-workspace composition

- **R52** — Core, Sky & Grove and Studio share the upgraded creative-workspace
  composition throughout the shell, Dashboard/project views, agent workspaces, Tasks, Pipelines,
  Archive, Settings, onboarding and overlays. Switching appearance changes its palette, semantic
  colors and canvas ornament, not page measures, spacing, hierarchy, card geometry or control
  alignment. Each retains its recognizable existing color scheme and background treatment;
  Studio retains its restrained blue/forest-green palette and consistent dots. Route and agent
  headings use the friendly readable hierarchy of R46 rather than oversized condensed or uppercase
  technical headings; mono remains available for technical values. This supersedes the old-layout
  preservation clauses in R42/R46, R2's crisp/asymmetric geometry where inconsistent with this
  common composition, and A21's requirement that Studio alone be recognizable by layout. It does
  not remove the appearance system or prevent a future explicitly designed skin from differing.
- **R53** — Agent cards have perceptible but subtle soft depth at rest, a restrained
  keyline and bounded project/state accents, rather than a nearly flat border or hard offset slab.
  Name and state lead, preview follows, and metadata remains subordinate without empty indicator
  bands adding conspicuous gaps. The drag grip reads as a quiet grip, not a dark app-icon button.
  Collapse and composer actions have intentional compact sizing and alignment: Send/Cancel do not
  stretch to textarea height. Expanded headers size to their content, not to spare fixed-card
  height: a short-name ordinary header fits within 112px at the supported desktop floor, leaving
  the rest for real conversation and its anchored composer. Long names and additional real controls
  may wrap without clipping. Grid tracks, fixed expanded-card height, density, ordering, drag,
  expansion/collapse, context, lifecycle actions and focus behavior remain authoritative.
- **R54** — Full active and archived agent pages finish the same chat composition:
  compact identity/runtime/context header, quiet tabs, a clear conversation reading region and an
  aligned anchored composer. Runtime controls have legible grouping without oversized nested boxes;
  an unsupported/empty live-settings group does not leave an empty framed band. Ordinary prose has
  a comfortable bounded reading measure (roughly 680–840px on wide desktop), not wall-to-wall text;
  technical output, file/diff views and the annotation tray retain their needed space and existing
  scrolling. The composer belongs to that reading region without becoming an enormous full-width
  utility form. Real chronological messages, tool disclosure, permissions, annotations, errors,
  model switching/apply semantics, Files, Commands, conditional Terminal and read-only archive
  behavior remain unchanged. The Figma study's static log pane is never a substitute for real chat.
- **R55** — Tasks has consistent inset padding (normally 20–24px) around authoring
  content, deliberate label/field/group spacing and bounded form measures, rather than headings
  and inputs touching panel edges. Related short fields may share a row where they fit; Instruction
  and prerequisites have clear full-row hierarchy. Fire a signal stays a smaller supporting form;
  Fire/Create actions are content-sized and aligned with their form rather than stretched across
  it. Project/attention controls remain compact and the task ledger has clear row/action spacing,
  including attention, waiting, running, finished, empty and error states. At 1024px, forms reflow
  without clipping or changing reading/keyboard order. The actual feature-owned fields, compact
  setup disclosures, prerequisites, validation and Re-arm semantics remain authoritative, not the
  mockup's invented selectors, labels or actions.
- **R56** — Remaining routes receive the shared R46–R49 hierarchy, with content-specific
  measures rather than one global card grid: quiet compact navigation, operational run ledger and
  timeline, template authoring, search-first Archive, readable Settings and clear dialog/onboarding
  actions. Existing selected, hover, focus, disabled, busy, error, destructive, permission and
  success states remain visible in all three palettes. The only new motion is R59's explicit
  status indication; no new action or task behavior is introduced.
- **R57** — This is a presentation upgrade to existing appearances, not a fourth skin,
  replacement appearance preference, migration or legacy-layout toggle. It changes no route, data,
  persistence, API, retention, lifecycle, action, shortcut or supported viewport policy. Figma
  provides composition and finish, not a pixel-exact product specification: prototype-only menus,
  density controls, synthetic transcripts and task flows are excluded. Existing FS-02 grid and
  FS-03 chat contracts take precedence over mockup anatomy.
- **R58** — Active-project navigation joins the redesigned shell rather than retaining
  miniature boxed monospaced chips. Project titles use readable text typography, comfortable
  horizontal spacing and quiet tab-like geometry subordinate to primary navigation. The selected
  project has a clear structural marker plus restrained emphasis; a project's accent remains a
  supporting cue, not a full saturated button or the only selection signal. Hover, keyboard focus
  and overflow share that treatment. Preserve R39's single row, five-link cap, current-project
  visibility, full accessible names, truncation and `+n` access at 1024px. No new navigation row,
  project membership/order rule, route or persisted tab state is added. This refines R39's visual
  treatment without changing its navigation contract.
- **R59** — Agent-card state badges are intentionally prominent scan targets in both
  collapsed and expanded cards: legible text, stronger state-colored fill/edge and a clear indicator,
  distinguishable from quieter runtime/mail metadata. Do not copy the mockup's ambiguous faint
  badges. Busy uses a slow smooth repeating pulse (approximately 2.4 seconds per cycle);
  error and waiting-input use a faster pulse (approximately 1.2 seconds). Waiting-input includes
  the existing blocked-on-human/approval condition; this does not invent a `blocked` agent state,
  infer a new state from pipeline status, or label every wait as permission approval. Text remains
  readable throughout: pulse the indicator/emphasis, never disappear the whole badge or alternate
  sharply between fully bright and dark. Idle, done, unknown and stopped remain static. Pulsing
  follows the current displayed live state, including after reload, and stops immediately when
  that state ends or the agent stops; an archived/read-only agent remains static. This is sustained
  state indication, not a replayed lifecycle celebration. Reduced-motion preference renders equally
  prominent static badges. Motion and color are never the sole status signals; no card movement,
  sound, notification, automatic expansion or layout shift is added.

- **R60 — Chat cleanup is shared across appearances.** FS-03.R78–R80's web-link
  actions, table readability and Commands-tab removal apply in Core, Sky & Grove and Studio on
  desktop. The phone uses Core only under FS-20.R16 and TS-08.R73. These requirements supersede
  only the earlier requirements in this specification that preserve Commands in agent
  conversations. Other actions, tab conditions, reading hierarchy and archive behavior remain
  intact. Quiet table row boundaries and comfortable column spacing remain subordinate to message
  text; wide-table overflow stays local. No new appearance, preference or motion.

- **R61 — The full agent page adopts the Figma agent-conversation composition.** The
  `AgentConversation` study in AgentDeck — Theme Exploration (`OykxmXqZnnyA67QA1lv3AU`) governs
  the desktop active and archived agent pages: a quiet breadcrumb row above the panel (Back,
  project / agent, and the stable agent id at the right); one softly raised, rounded panel; an
  identity header with a monogram tile carrying a state dot, the agent name with its existing
  live-state label (or Stopped / Archived · read-only), a role · project · speed line, and a
  labelled context meter at the right; a separate low runtime band; quiet underline tabs; a
  centred reading column of roughly 700px with right-aligned tinted user bubbles, softly tinted
  assistant bubbles, a ruled quiet activity column, dark technical blocks and a structured
  permission card whose primary Approve sits after Deny; and an inset, softly raised composer box
  whose toolbar carries `@` (file) and `#` (command) insert buttons at the left and the existing
  actions at the right, where Send shows its text beside the arrow (superseding FS-02.R67's
  icon-only Send on this page only; Cancel stays an icon and the dashboard pane is unchanged). The runtime band keeps FS-03.R47's two honest
  groups: backend/model stage a change that reveals "Unapplied changes", **Switch** and a local
  **Discard** that returns the picker to the current runtime without a request; effort and fast
  mode (a switch control) apply immediately, with "Effort and speed apply to the next turn" as the
  idle hint. This supersedes FS-03.R62's shared left reading band only; Back stays first and the
  header right-click Copy thread identity is unchanged. The insert buttons type the existing
  trigger at the caret and open the existing picker; they add no new suggestion source. All
  three appearances share the composition through their semantic palettes. The study's sample
  data, preview notes, decorative hints, per-message times, date dividers, Files count and
  turn-boundary captions are excluded; real transcript order, held follow-ups (still in the
  transcript), annotations, file viewer, Think Tank and Terminal tabs, and archive Resume/Restore
  behave as before. The dashboard chat pane and the phone keep their own compositions.

- **R62 — Settings adopts the Figma settings composition.** The `Settings` study in AgentDeck —
  Theme Exploration (`OykxmXqZnnyA67QA1lv3AU`) governs the desktop Settings page: a friendly page
  title over a two-column layout whose narrow left section list (rounded items, soft tinted
  selection, divided from content by a quiet rule) holds at the 1024px desktop floor; every
  section opens with a small mono uppercase eyebrow, a readable section title
  (Theme for Appearance, Configured backends for Backends, Dependent work for Tasks), optional
  one-line description and its primary action at the right. Roles, projects and paired phones are
  softly raised rounded rows with name and id, a quiet status chip plus excerpt or working
  directory, and text-style row actions where destructive actions use the error color; a role
  with no permission override reads "Inherit global". Each backend is one raised card whose
  models, environment, provider and configuration-source groups are separated by ruled bands with
  mono labels rather than nested boxes. Notifications and Tasks are bounded ruled lists/forms;
  appearance choices are three equal cards with a full-width palette preview over the radio,
  name and description. Primary actions use the appearance's primary color with a soft pressed
  edge. All three appearances express the composition through their semantic palettes. The
  study's sample data, prototype-only notes, Appearance default tab, concept theme and invented
  backend fields are excluded; Remote joins the same composition; every existing section, editor,
  dialog, validation, save, warning and destructive-confirmation behavior is unchanged.

- **R63 — Think Tank room cards adopt the Figma room-summary composition.** The `RoomSummary`
  study in AgentDeck — Theme Exploration (`OykxmXqZnnyA67QA1lv3AU`), shown on its project
  dashboard, governs the room cards before a project's agent grid and in Archive. The section
  opens with a mono uppercase "Shared rooms / Think Tank" eyebrow over "Thinking together"
  (Archive: "Retained room history") and a quiet "N rooms · separate from agent sessions" note.
  Each card is one softly raised, rounded three-column row: a room-symbol tile; the room content;
  and a ruled budget column whose large mono number is the collective remaining allowance with
  "turns remaining · Combined ceiling, not a target" (ended: "unused turns · Not a success
  signal"). Content reads title link with ↗ and a ruled phase label, a state chip at the right
  (Active ●, Paused Ⅱ, Needs attention !, Ended ■), the state-colored status line, a quiet
  "GOAL" preview, a ruled roster of every participant in three columns (tinted initials tile,
  name link with ↗ or plain name when deleted, project · turns left, and Ready/Speaking ●/
  Exhausted/Departed), then a footer with the room's scope or origin at the left and judge
  state at the right. The icon tile hides on narrow sections and the card stacks on very narrow
  ones. The room section shares the agent grid's centred maximum width, so room and agent cards
  align on wide screens. All three appearances express the composition through their semantic
  palettes. The
  study's sample rooms and copy are excluded; real status, attention, allowance, judge and
  retained-identity data drive the card, and FS-02.R71/FS-21.R44 behavior is unchanged.

- **R64 — The Think Tank room page adopts the Figma think-tank composition.** The `ThinkTank`
  study in AgentDeck — Theme Exploration (`OykxmXqZnnyA67QA1lv3AU`) governs the full room page.
  A quiet breadcrumb row leads with Back to the origin project, then "project / Think Tank" and
  the stable room id at the right. The header pairs a mono uppercase "Think Tank / project"
  eyebrow with a room-symbol tile over the large title; at the right sit Files and Commands with
  their counts, a short rule, the existing Pause / Keep going / Resume control, a quiet
  destructive End discussion and Delete. A context row follows: a state chip (Running, Waiting,
  Needs attention, Paused, Ended — colored by its tone with a dot), "Phase …", a rule,
  "Independent openings On/Off", "Final synthesis On/Off" and, for a pipeline room, its run link
  at the right. The goal is a full-width card disclosure, "Room goal" with a one-line preview and
  "View full goal"/"Collapse". Below, the discussion sits directly on the page canvas beside a
  ~272px participant column. The discussion opens with "Room discussion" and "Shared history only
  · started time"; ruled round labels introduce the independent openings and the discussion;
  each contribution has a byline (author link, project, ruled kind, time at the right), an
  optional "Addressed to" line, and a softly tinted bubble in that speaker's tint; your messages
  carry a "Y" tile and a user-tinted bubble; departures and missing openings read as quiet event
  rows; the synthesis is a ruled card. While openings are hidden, a centred "A little space for
  independent thinking" placeholder names who is still writing. The current action follows as a
  band — live dots while a turn runs, ! for attention, Ⅱ when paused, — when ended — with its
  pending note and recovery actions. The composer is an inset, softly raised box: textarea, then
  a toolbar with `@`/`#` inserts and the "Files and commands from" source at the left and a
  labelled Send at the right; a help row beneath says whether the message is held until the
  current turn or shared now, and how to send. An ended room replaces the composer with a
  read-only note. Each participant is a card whose leading edge carries its speaker tint: name
  link ↗ with project, the live agent's model · effort when known, "completed / limit turns", a
  status dot and text, and Raise turn limit; the judge sits in its own "Final synthesis · Judge"
  card with its state and a note that it does not take turns; a quiet "A room, not a group
  assistant" note closes the column. Files and Commands open as a dialog listing each source
  with its participant and project; a file opens in the existing viewer. All three appearances
  express the composition through their semantic palettes. The study's sample conversation,
  tagline, state-preview selector, per-command Reference, file "Annotate file" button and
  held-message bubbles are excluded; real entries, activity, annotations, file viewer, mentions,
  turn-limit, retry, pause/end/delete and judge-repair behavior is unchanged (FS-21).

- **R65 — Project-dashboard agent cards adopt the Figma session-card composition.** The
  `SessionCard` study in AgentDeck — Theme Exploration (`OykxmXqZnnyA67QA1lv3AU`), shown on its
  project dashboard, governs every agent card on a scoped project grid, collapsed and expanded.
  The card is one softly raised, rounded panel with a thin border that strengthens on hover and
  while expanded, and a 3px leading edge in the agent's state tone. Its head reads top to bottom:
  a state line (a small dot and the existing state label in that tone, or "Stopped" with a
  neutral tone for an agent that is not running) with the drag grip quietly at its right; the
  agent name as a link to `/agent/:id` beside a small chevron button that expands (chevron down)
  or collapses (chevron up) a chat card; a quiet runtime row of backend, mono model and effort
  separated by a thin rule; a labelled context row ("Context" at the left, "N% · used / size
  tokens" or the percentage-only label at the right) over a thin track; the preview in body text
  over at most two lines; and a footer with role (· project off the scoped grid), pipeline link,
  terminal driver and the Mail/Sent indicators. Expanding keeps the whole head and opens the
  chat pane below a rule: transcript with softly tinted assistant bubbles and right-aligned
  user bubbles, a quiet ruled activity column, and an inset, softly raised composer box whose
  Send is a quiet square arrow button. This supersedes, on these cards only: FS-02.R2/R59's
  collapsed card without context (the context row returns on every card); R3's pill badge
  and the actionable highlighted background (the state line and leading edge carry state);
  R5's single line (two lines); R6's dimming (the neutral Stopped line marks a stopped agent);
  R40's project wash, tinted border and top state bar (a scoped grid shows one project, so the
  leading edge carries state instead); R58's larger title size; and R67/R68's header placement
  (the state line sits above the name, the collapse chevron beside it, the context row below the
  runtime row). A terminal card has no chevron and still opens its agent page. All three
  appearances express the composition through their semantic palettes. The study's sample data,
  "Needs input"/"Running" wording, attention copy and its static transcript excerpt are
  excluded; the real live transcript, activity, permission cards, composer actions (Steer,
  Cancel, Withdraw queued), pane cap, recency, persistence, keyboard pane cycling, header-region
  collapse, context menu, drag reorder and running/stopped placement are unchanged.

- **R66 — Right-click menus share the session-card's fine construction.** Every pointer context
  menu (agent card, project card and projects-home background, agent header, transcript link and
  annotation menus) is one compact raised panel with a thin border, medium radius and soft
  shadow. Items are low single-line rows of regular-weight small body text with no border or
  lift, a quiet tinted hover and focus row, muted disabled rows that keep their tooltip,
  hairline separators, and a small muted section label above the project color swatches, which
  shrink to small rings. All three appearances express it through their semantic palettes. The
  items, order, placement, dismissal, keyboard behavior and actions are unchanged.

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
- **Appearance selection:** choosing Core, Sky & Grove, or Studio applies the complete
  selected presentation to the mounted application and saves the global preference; a later session
  restores it after configuration loads.

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
  representative errors. Every first-party surface clearly belongs to one core Chuck design and
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

- **A16** (R40) — Core and Sky & Grove fixtures render an agent card with a
  short name and with a name long enough to need three lines: the long name wraps and is not
  truncated to one line, does not overlap the state badge or drag grip, and does not escape the
  card; neither card shows a context meter while collapsed, and an expanded card shows the compact
  context figure in its header. Both skins keep R9's remaining order and R10's construction. —
  deterministic visual matrix plus the real-browser review in A1.

- **A17** (R41) — A context menu opened on a lower-row card at the supported desktop
  floor renders every lifecycle action, Archive included, inside the viewport, across menu heights
  and bottom/right pointer positions. — `ui/src/lib/menuPlacement.test.ts` for the geometry,
  `CardContextMenu.test.tsx` for the measured wiring, and a real-browser J5 lower-row pass.

- **A18 (shipped 2026-09-23)** (R42) — Settings presents all three named appearances with distinguishable
  previews and a text-identified active choice. Choosing the new skin changes the mounted app
  without reload; navigation, reload, and a later browser session preserve it. Choosing either
  existing appearance restores that appearance, and clearing the preference restores Core.
  Unknown configuration and a refused save retain R32's usable fallback and rollback behavior.
  *Verify:* Settings and configuration round-trip tests plus a real-browser appearance-switch
  journey.
- **A19 (shipped 2026-09-29)** (R43–R45) — A deterministic three-appearance visual matrix and real-browser
  review cover the dashboard's empty, dense, grouped, expanded-chat, long-name, and attention
  states; live and archived transcript variants; pipeline, task, archive, settings, onboarding,
  overlays, permissions, syntax, diffs, and terminal. At the supported desktop floor and a wider
  viewport, the new skin follows R43's visual direction, keeps R45's semantic distinctions, and
  has no clipping, overflow, or network-loaded asset. *Verify:* visual fixtures, presentation
  contract checks, and rendered screenshots in the actual UI.
- **A20 (shipped 2026-09-29)** (R44) — Expanding a dashboard agent card in the new skin shows the same real
  chronological chat, composer, permissions, tool disclosure, and navigation behavior as Core;
  collapsed card density and grouping are unchanged. *Verify:* existing dashboard/chat behavior
  tests plus a real-browser expanded-pane and agent-screen journey in Core and Studio.
- **A21 (shipped 2026-09-29; superseded by R52/A26)** (R46–R49) — At 1024px and a wider desktop viewport, paired rendered views of
  Core, Sky & Grove, and Studio show Studio-specific changes to spatial hierarchy, grouping,
  typography scale, and surface composition on the shell, dashboard, expanded card, agent screen,
  Tasks, Pipelines, Archive, and Settings. A desaturated comparison still identifies Studio by its
  layout and hierarchy; changing only color, radius, shadow, or the dot pattern does not pass.
  *Verify:* real-browser side-by-side design review against the Figma Make direction and the
  screen-specific requirements R46–R49, with representative long/dense and empty states.
- **A22 (shipped 2026-09-29)** (R47–R48) — A grouped dashboard at every existing density keeps the same card
  positions before and after one pane expands; a long-name card, waiting/error card, stopped card,
  and terminal card remain scannable. The expanded chat uses the real transcript and composer,
  exposes permission/tool detail, and neither clips controls nor visually reads as a static code
  sample. *Verify:* FS-02.A37–A41 regressions plus Core/Sky & Grove/Studio browser screenshots
  and the live expanded-pane journey.
- **A23 (shipped 2026-09-29)** (R48–R49) — Studio's active and archived agent views, task attention and
  waiting rows, active/paused/finished pipeline runs, template editor, archive results, all
  Settings sections, and four onboarding steps retain their existing controls and reading order
  while meeting their distinct composition in R48–R49. Focus, hover, disabled, error, success,
  permission, and long-content states remain clear. *Verify:* the deterministic visual matrix,
  affected feature tests, and real-browser route/state review at the supported desktop floor.

- **A24 (shipped 2026-09-26)** (R50) — In an isolated built application at 1024px and a wider desktop viewport,
  exercise compact/custom New Agent; bound/unbound/error native linking plus an unsupported-provider
  onboarding path; named/manual task prerequisites and Re-arm; and default/custom/proposed pipeline
  start. Compare Core, Sky & Grove and Studio with long/duplicate names, fast mode and a hidden-field
  validation error. Verify keyboard operation, visible draft/override summaries, no clipping, and
  that default flows need no advanced-editor interaction. Reuse representative deterministic fixtures
  and fake ACP where a launch is needed; record actual browser evidence at implementation closure.
  This is an affected-surface check, not a whole-application redesign or provider-compatibility claim.

- **A25 (shipped 2026-09-26)** (R51) — At 1024px and a wider desktop viewport, a running agent with the full
  runtime picker, long identity, staged Switch action, live settings, and an error keeps every
  header control visible while giving more height to the transcript than the prior stacked header.
  Core, Sky & Grove, and Studio retain their existing hierarchy and the shared pointer menu remains
  inside the viewport. *Verify:* focused component/style tests plus a real-browser comparison.

- **A26** (R52, R56–R57) — Render matched real content in all three appearances at
  confirmed 1024px and a wider desktop viewport. Shell, dashboard, agent pages, Tasks, Pipelines,
  Archive, Settings, onboarding and overlays share composition, spacing and control geometry while
  retaining each palette and ornament. A geometry/desaturated comparison confirms common layout,
  not Studio-only layout distinction. Verify appearance selection/reload/fallback still works.
  *Verify:* matched deterministic fixtures, real-browser route comparison and existing appearance
  contract tests; record actual viewport, revision, route/state and screenshot evidence.
- **A27** (R53) — Short/long names, collapsed/expanded cards, all density extremes,
  waiting/error/stopped/terminal states and empty/populated chat demonstrate a quiet grip, readable
  metadata, visible soft resting depth, content-sized header and compact Send/Cancel. A short-name
  expanded header is at most 112px; its transcript receives remaining space. Growing a textarea
  does not grow the action button. Exercise drag, Collapse, send/cancel, permissions and keyboard
  focus with neighboring cards remaining stable. *Verify:* focused existing card/composer tests,
  rendered geometry checks and real-browser FS-02 grid/chat journeys in all appearances.
- **A28** (R54) — Active and archived agent views with short/long identities, staged
  runtime edits, available/unavailable live settings, prose, code/diff/tool output, permission,
  annotation tray and file viewer demonstrate compact complete chrome, readable chat measure and
  aligned composer without empty boxes, clipped controls or scroll regressions. Empty chat alone
  is insufficient evidence. *Verify:* real-browser populated active/archive checks and affected
  chat/composer tests in all appearances; compare with the approved Figma reading hierarchy, not
  its synthetic content.
- **A29** (R55–R57) — At 1024px and wide desktop, Tasks authoring has visible inset
  padding, consistent field/group rhythm, content-sized actions and a subordinate signal form.
  Long names/instructions, prerequisites, expanded detail, validation, attention/waiting/running/
  finished rows and empty/error states remain readable. *Verify:* deterministic task fixtures,
  existing task behavior tests and real-browser keyboard/form review across appearances. Review
  the Figma comparison for spacing and hierarchy without copying its task interaction changes.

- **A30** (R58) — All three appearances render zero, one, five and overflowing active
  projects with long titles at 1024px and wider desktop. Text tabs, selection, hover/focus and
  overflow look native to the shared shell, without tiny technical chips or clipping/displacing
  primary navigation and connection state. Current-project visibility and all overflow links remain
  keyboard-reachable. *Verify:* existing project-nav tests, matched rendered shell fixtures and
  real-browser keyboard/overflow checks; compare actual title legibility, not only DOM presence.
- **A31** (R59) — Matched collapsed/expanded cards show legible, prominent badges for
  every agent state in every appearance. Observe several slow busy and faster error/waiting cycles
  at normal speed, reload in those states and transition out of them: labels never vanish, cadence
  differs clearly, and geometry stays stable. Stopped/archived and idle/done/unknown badges do not
  pulse. Repeat with reduced motion and confirm static salience. *Verify:* focused badge/card state
  tests plus real-browser motion and reduced-motion checks; a static screenshot cannot prove pace.

- **A32** (R60) — FS-03.A59–A61's rendered checks cover table spacing/dividers/overflow,
  readable link menus and remaining tab reachability in all three desktop appearances at desktop,
  expanded-pane and archive surfaces. The phone check is Core only under FS-20.R16 and TS-08.R73.
  *Verify:* presentation/style audit and focused browser journey.

- **A33** (R61) — Compare the rendered study with real fake-backend agents at 1024px and 1280px in
  Core, Sky & Grove and Studio: a completed turn with activity, a pending permission, a long agent
  name, a staged runtime change, a live turn and an archived session. Breadcrumb, header, runtime
  band, tabs, reading column, bubbles, permission card and composer match the study's hierarchy and
  spacing without horizontal overflow; Discard restores the current runtime without a request;
  `@`/`#` open the existing picker; Copy thread identity, file viewer, annotations, held
  follow-up, Steer/Cancel and Resume still work. The dashboard chat pane and phone are unchanged.
  *Verify:* focused component tests plus the working tree's real-browser comparison.

- **A34** (R62) — Compare the Settings study with every Settings section at 1024px and 1280px in
  Core, Sky & Grove and Studio using populated fake-backend configuration: roles with and without
  a permission override, active and archived projects, a backend with models, provider and
  configuration source, notifications, appearance, tasks validation and remote. Navigation,
  headings, rows, backend card, ruled lists and theme cards match the study's hierarchy and
  spacing without horizontal overflow; hover, focus, selected, disabled and destructive states
  stay visible; existing editor tests pass unchanged in behavior.
  *Verify:* focused component tests plus the working tree's real-browser comparison.

- **A35** (R63) — Compare the rendered study with real room cards at 1024px and 1440px in Core,
  Sky & Grove and Studio: concurrent openings, a speaking participant, a held room, a paused room,
  an ended room with judge state, a deleted participant and a long title. Section heading, icon
  tile, title/phase/state row, status, goal, roster, footer and budget column match the study's
  hierarchy and spacing without horizontal overflow; room and participant links stay separate
  focusable actions; Archive shows origin, including a removed project.
  *Verify:* focused component tests plus the working tree's real-browser comparison.

- **A36** (R64) — Compare the rendered study with real rooms at 1024px and 1440px in Core, Sky &
  Grove and Studio: hidden concurrent openings, a live discussion turn with a held message, a
  paused room, a failed turn needing attention, an ended room with synthesis and a departed
  participant, a long title and goal, and a pipeline room. Breadcrumb, header, context row, goal
  card, discussion, current-action band, composer and participant column match the study's
  hierarchy and spacing without horizontal overflow; Files/Commands dialogs open the file viewer;
  mentions, annotations, Raise turn limit, retries and Delete still work.
  *Verify:* focused component tests plus the working tree's real-browser comparison.

- **A37** (R65) — Compare the rendered study with real fake-backend agent cards at 1024px and
  1280px in Core, Sky & Grove and Studio: busy, idle, waiting, error and stopped chat agents, a
  terminal agent, a long unbroken name, unread mail, a pipeline stage agent, and an expanded pane
  with a completed turn, activity and a pending permission. State line, title row, runtime row,
  context row, preview, footer, leading edge and expanded composer match the study's hierarchy
  and spacing without horizontal overflow; the chevron and the header region collapse the pane,
  the name opens the agent page, drag reorder, context menu, Send/Cancel and pane cycling still
  work, and keyboard focus is visible on the name, chevron and grip.
  *Verify:* focused component tests plus the working tree's real-browser comparison.

## 6. Deviations & open decisions

- R52–R59/A26–A31 are the 2026-09-28 approved cross-appearance upgrade, including the operator's
  follow-up for project tabs and stronger pulsing badges. The operator approved this scope on
  2026-09-28; TS-08.R74–R79 specify its architecture. Implementation and acceptance closed
  2026-09-28, including actual system reduced-motion verification after correcting the CSS
  fallback specificity. Evidence is recorded in the shared-layout implementation report.
  A26 supersedes A21's Studio-only layout oracle. The remaining A19–A23/TS-08.R68 evidence debt
  closed on 2026-09-29 with a matched confirmed-viewport route/state pass, real expanded chat,
  a genuine completed pipeline timeline, and independent desaturated/common-geometry review.

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
- Studio's first shipped slice (R42–R45) established selection and visual values but retained
  Core's composition. The operator rejected that as an incomplete UI redesign on 2026-09-25.
  R46–R49 (shipped 2026-09-25) added Studio-only spatial and typographic composition while
  preserving the existing interactions; TS-08.R65–R67 (shipped) superseded the restrictive Studio
  clauses in R62–R63. A19–A23 closed on 2026-09-29 after the independent rendered acceptance pass;
  A21 is read through the later R52/A26 common-geometry supersession. An earlier desaturated Core/Sky & Grove/Studio dashboard comparison showed
  Studio's bounded card accent beyond palette differences; a later correction removed the
  full-chat panel gaps. An isolated built-app pass then exercised all four onboarding steps,
  populated attention/waiting Tasks, active and paused pipeline states, the template editor,
  Archive and archived chat, active permission chat, and all six Settings tabs; populated Tasks
  were compared across all three appearances. The closing pass covered the remaining long/dense
  and interaction states at confirmed 1024×900 and 1280×720 viewports, a non-dashboard
  desaturated/common-geometry comparison, and a genuine finished-run timeline.

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
- R40 and A16 narrow R9's card priority: the name wraps at a smaller size and context
  usage moves onto the expanded card (FS-02.R58/R59).
- Cross-cutting UI bug classes: INV §8, §10, §11, and §13.
