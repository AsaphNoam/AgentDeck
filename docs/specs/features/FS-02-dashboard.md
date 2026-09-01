# FS-02 — Dashboard (card grid home view)

**Status:** Partial
**Code:** `ui/src/components/grid/`, `ui/src/store/`, `ui/src/components/shell/NotificationCenter.tsx`, `ui/src/features/settings/NotificationsEditor.tsx`, `ui/src/api/sse.ts` · `internal/bus/`, `internal/state/`, `internal/server/handlers.go` (layout, reconcile) · **Journeys:** J5 (grid & layout), J11 (failure & recovery), J12 (restart durability)
**Absorbed:** [`agent-dashboard-prd.md`](../../archive/agent-dashboard-prd.md) F1/F2/F11 and the [phase archive manifest](../../archive/phases/README.md)

## 1. Purpose

The dashboard is the home view: one live card per agent (running or stopped), laid out in a
reorderable, density-adjustable grid with collapsible task-group sections. It is the primary
supervision surface — every agent's identity and live state at a glance — and the launch point for
per-agent lifecycle actions (FS-01) and the chat panel (FS-03). It also owns the notification
surface: in-app toasts and desktop Web Notifications on significant state transitions.

The dashboard is a pure view over server state delivered by the SSE bus; it holds no authoritative
state of its own beyond persisted layout preferences (`layout.json`).

## 2. Behavior — card grid

**R1.** The home route (`/`) renders one card per agent present in the store. Cards are seeded from
the SSE hydration burst on connect and kept live thereafter; no manual refresh is required.

**R2.** Each card displays: agent **name**; **role · project** subtitle; a **backend · model** pill;
a color-coded **state badge**; a **context-usage** meter; and a single-line **output preview**.

**R3.** The state badge maps the agent's `state` to a fixed vocabulary and palette: `busy` (amber,
animated pulsing dot), `idle` (slate), `waiting_input` (blue, "Waiting"), `done` (green), `error`
(red), and `unknown` (gray — no status row reported yet). `waiting_input` and `error` are the
actionable states and additionally draw a highlighted card treatment.

**R4 — retired 2026-07-26:** The context-usage meter rendered missing or zero values as an empty bar
with no label. This made an ordinary zero-value meter resemble an unloaded placeholder; superseded
by R26.

**R5.** The output-preview line is `status.detail` when present; otherwise the latest `assistant_text`
delta observed for that agent (client-tracked fallback); if both are empty the line is omitted. The
preview is truncated to a single line.

**R6.** A card whose agent is not running (`running === false`) is visually dimmed and shows a
`stopped` marker. A stopped agent remains a card until its identity is removed (a removal tombstone,
R21) — stopping is not removal.

**R7.** A card for a `terminal`-interface agent shows a `terminal` pill (with the driver name when
present). A card with pending inbound mail shows an unread badge (`Mail <n>`); a card that recently
sent a message shows a transient `Sent` pulse. (Unread/mail semantics are owned by FS-06.)

**R8.** Clicking a card body navigates to that agent's chat panel (`/agent/:id`, FS-03).
Right-clicking a card opens its context menu at the cursor (R15).

## 3. States & transitions

**R9.** Card state is driven entirely by `state_update` SSE events carrying the effective
`AgentState` (identity ⊕ running ⊕ status). A card reflects an underlying state change within ~1s of
the server applying it. Each `state_update` carries the full `AgentState`, so a dropped frame is
self-correcting on the next update.

**R10.** On a new or reconnected SSE stream the client replaces its agent set with the server's
hydration burst (a `state_update` per current agent, terminated by a `__hydrated__` marker). Any
agent in the store but absent from the completed burst is dropped — no stale cards survive a
reconnect. Because the burst is rebuilt from `state.db`, a still-running agent reappears after a
server restart.

**R11.** The five live states form no fixed client-side transition graph — the card renders whatever
`state` the server reports. A process that disappears (its PID is no longer alive) is reconciled to
`done` with detail "process exited", not `error` (see Deviations).

## 4. Behavior — layout, groups, context menu, empty state, notifications

### Layout & density

**R12.** Cards are drag-reorderable within the grid. A drag commits a new display `order`; the order
is persisted to `layout.json`.

**R13.** Grid density is adjustable: cards-per-row (`perRow`, 1–8) and inter-card `gap` (0–48px).
Density is persisted to `layout.json`.

**R14.** `order` + `density` + per-group collapse state load once at boot via `GET /api/layout`
(defaults returned if the file is missing — never a 404) and save on change via `PUT /api/layout`,
debounced ~400ms. `PUT` validates `perRow` (1–8) and `gap` (0–48) and writes atomically. Reload and
server restart both preserve the persisted layout.

### Context-menu actions (route to FS-01 lifecycle verbs)

**R15.** The card context menu offers: **Open chat** (navigate to `/agent/:id`), **Rename**, **Stop**
(disabled unless running), **Switch runtime**, **Clone**, **Move to group**, and — for terminal
agents — **Reveal terminal**. Click-outside or Escape closes it; it renders in a portal so card
overflow cannot clip it.

**R16.** Each mutating menu action maps to an FS-01 verb (or FS-04 identity edit):
- Rename → `POST /api/sessions/{id}/rename` (FS-01).
- Stop → `POST /api/sessions/{id}/stop` after a confirm (FS-01).
- Switch runtime → `POST /api/sessions/{id}/switch-runtime {interface?, backend?, model?}` (FS-01).
- Clone → `POST /api/sessions` with this agent's role/project/backend/model/interface/group (FS-01
  launch); it launches immediately with no confirmation.
- Move to group → identity update of the `group` field via `POST /api/sessions/{id}/identity`.

**R17.** A failed menu action surfaces an error toast carrying the server message; it does not fail
silently.

**R37.** Dashboard confirmations and inputs use core application dialogs rather than
browser `prompt`/`confirm` (FS-12.R26). **Stop** (R16) and **Release group** open consequence-aware
confirmation dialogs that state the effect and default focus to Cancel. **Move to group** (R16)
opens a combobox that suggests the current group labels, where a blank value clears the group. A
configured active project card's **Rename**, **Change color**, and **Archive** (R34) open,
respectively, a name form validated as the project title, the R,G,B color control (FS-04.R5)
(superseded by the inline preset picker in R39), and a
consequence-aware confirmation. Cancel performs no action; each confirmed action issues the same
request as today.

### Task groups

**R18.** Agents carrying a non-empty `group` label are rendered under a collapsible group section;
agents with no group fall under a trailing "Ungrouped" section (which sorts last). A group header
shows the group label, member count, and a per-state count summary.

**R19.** A group section's collapsed/expanded state is toggled from its header and is persisted per
group in `layout.json` (`groups.<name>.collapsed`).

**R20.** A named group header offers **Release group**, which — after a confirm — stops every agent in
that group in one action. A lifecycle transition already in progress for any member rejects the
whole release before it stops a member; the existing action-error toast says the person can retry
once that transition settles. The Ungrouped section has no release control.

### Empty state & removal

**R21.** When no agents are present, the grid renders a dedicated empty state with a "New agent"
trigger (FS-04 launch). A removal tombstone (`state_update` with `removed: true`) deletes the card;
the New-Agent modal stays mounted across the 0→1 transition so an in-flight launch is not unmounted.

### Notifications

**R22.** The server emits `notification` SSE events for significant transitions: `done` and
`waiting_input` fire only when an agent's `state` actually changes into that state;
`permission_required` fires when a permission request is raised. (`budget_exceeded` rides the same
pipeline; its semantics belong to FS-06.) Each payload carries `notification_type`, `agent_id`,
`agent_name`, `address` (`role@project`), `title`, and `body`.

**R23.** A received notification is shown in-app as a toast (newest-first stack, capped at 4, each
auto-dismissing after ~6s and dismissable on click). When the browser tab is hidden **and** desktop
notifications are enabled **and** the browser has granted `Notification` permission, a desktop Web
Notification is raised instead of the toast (deduped per agent via the notification `tag`).

**R24.** Notifications are mutable per type (`done`, `waiting_input`, `permission_required`,
`budget_exceeded`) via Settings; a muted type is dropped client-side before any toast or desktop
notification. Desktop notifications can be disabled wholesale, and desktop permission is requested
from the Notifications settings editor.

**R27.** A stopped agent's card context menu additionally offers **Resume**. It is absent from a
running agent's menu, because a resume while a running row exists is rejected (FS-01.R25). The
Dashboard Resume action reuses the session's frozen configuration and history as specified by
FS-01.R10.

**R28.** The Dashboard **Resume** action calls `POST /api/sessions/{id}/resume`. A rejected request
shows the server's message in an error toast and leaves the card available for another attempt.

**R25.** A card's project subtitle displays the current configured project title, not its durable
project id. If the corresponding project definition is unavailable (for example, it was force-deleted
after the agent launched), the card falls back to that id so the agent remains identifiable.

**R26.** The context-usage meter renders `context_pct` (0..1) as a proportional bar with a rounded,
visible percentage label, color-ramped green (`< 0.6`) → amber (`0.6–0.85`) → red (`> 0.85`). Zero
and absent values normalize to and visibly label `0% context used`; the track alone is never the only
indication of its meaning.
The meter tracks the live value during a turn, not only at turn end: each context-usage report the
runtime receives mid-turn republishes the agent, so a long turn shows the percentage moving rather
than the figure it started with (TS-04.R25).

**R29.** The home route (`/`) replaces the agent-card grid in R1 with a live project-card
grid. It contains every configured project, including projects with no agents, plus one unavailable
project card for each durable project id still referenced by at least one non-archived agent after
its configuration was force-deleted. This preserves access to stopped agent cards rather than hiding
them when their configuration disappears. The unavailable card disappears when its last
non-archived agent is archived; those archived agents remain reachable under the missing-project
group in Archive. Archived projects (FS-04.R35) are excluded from this active grid.

**R30.** A project card displays the configured project title and color, or its durable id
when configuration is unavailable; it also displays its dashboard-visible agent count and live
per-state summary. Individually archived agents and agents under an archived project (FS-05.R32) are
not counted. It updates from the same project catalog query and `state_update` stream that drive the
existing dashboard, with no manual refresh.

**R31.** Selecting a project card navigates to `/project/:project-id`. That route renders
the dashboard card grid for precisely the non-archived agents whose immutable `project` id matches
the route and whose project is active or whose project configuration is unavailable;
grouping, state, context, previews, context-menu lifecycle actions, and live updates retain their
existing FS-02 behavior. An agent card on this scoped route displays its role but not its project,
because the route heading identifies the common project.

**R32.** The scoped-dashboard header identifies the selected project using its current
title (with durable-id fallback) and includes a route back to the project-card home. An unknown
project route shows an unavailable-project state with the same return path, rather than an empty grid
that implies a valid project with no agents. A known archived-project route instead shows an
archived-project state with the same return path and a route to that project in Archive; a bookmark,
Back navigation, or stale tab never renders an empty scoped grid for an archived project.

**R33.** The Archive workspace is project-based. It lists a project for every archived
agent it contains and every archived project, then nests that project's archived agents beneath it
with access to their retained transcripts and tracking data. Each project row displays its archived-
agent count and whether the project itself remains active on the main dashboard. Thus a project with
both active and archived agents appears in both collections. Project archival neither deletes project
configuration nor any agent history.

**R34.** Right-clicking a configured active project card offers **Rename**, **Change
color**, and **Archive** (menu presentation and the color control refined by R38–R39).
Rename and Change color use the existing project configuration fields and
update the card immediately. Archive presents a warning confirmation stating that every running agent
will be stopped and every agent in the project will be archived; on confirmation it performs exactly
that and moves the project to the Archive workspace. A failed action surfaces an error toast and
leaves the project card unchanged. An unavailable-project card has no project-configuration actions;
its visible agent cards retain the Archive action in R35.

**R35.** Every dashboard-visible agent card offers **Archive** in its context menu.
Archive requires no confirmation: it stops a running agent, waits for that stop to complete, then
archives the agent; a stopped agent is archived immediately. The action removes only that agent from
the active project dashboard and nests it under its project in the Archive workspace; it does not
archive the project or other agents. A project remains visible on both the active dashboard and
Archive while it has agents in both collections.

**R36.** Project dashboards introduce no project-specific layout persistence. The
existing shared agent-card layout preferences continue to order, group, and size any scoped agent
grid; project cards use their normal responsive presentation and have no persisted reorder state.

**R38.** The project card context menu (R34) renders as a cursor-positioned portal at the
pointer, reusing the same `context-menu` presentation hook, click-outside/Escape dismissal, and
overflow-proof portaling as the agent card menu (R15), rather than expanding inline within the card
body. Its offered actions and their targets are otherwise unchanged from R34.

**R39.** Within that menu, **Change color** is presented as an inline row of the six
preset accent swatches defined in FS-04.R39; selecting one applies immediately through the existing
project update and recolors the card, and the card's current color is indicated when it equals a
preset. No color dialog opens. This supersedes the Change-color dialog clause of R37; **Rename** and
**Archive** continue to open their dialogs (R37).

**R40.** A project's accent color (FS-04.R5) visibly tints every card belonging to that
project — each agent card on a scoped grid and the project card on the home grid — as a soft full-card
background wash and a tinted card border, so an otherwise empty card still reads as that project's
hue. This is in addition to the existing inset left-edge accent, which is retained. The top state bar
keeps showing the agent's live state color (R8/state), not the project color. The wash and border are
bounded so body text keeps its contrast under Core and every built-in skin, and the treatment derives
from the single project-accent value without any per-card presentational literal (TS-08.R37).

### Project creation

**R41.** The projects home (`/`, R29) offers a **New project** action from a persistent
header button and from a cursor-positioned context menu opened by right-clicking the projects-home
background — any point in the projects view that is not a project card. Right-clicking a project card
continues to open that card's menu (R34/R38) and does not also open the background menu. The
background menu reuses the same `context-menu` portal presentation, click-outside/Escape dismissal,
and overflow-proof portaling as the card menus (R15/R38), and offers the single **New project** item.
This action exists only on the projects home, not on the scoped `/project/:id` route.

**R42.** Both entry points (R41) open one modal containing the project creation form reused
from the Settings project form (FS-04.R5/R7/R39): **Title**, **Color** (six-preset swatches),
**Working directory (cwd)**, **Additional directories**, and **Context prompt**, validated as
FS-04.R7 requires (title and cwd required). Submitting issues `POST /api/projects` with no id so the
server derives it (FS-04.R31). On success the modal closes and the new project's card appears in the
live project grid with no manual refresh (it derives from the same project catalog query as R30). On
a validation or API failure the modal stays open and surfaces the server's field/error message;
Cancel or Escape closes it with no change. A non-existent `cwd` is a non-blocking warning, not a
failure (FS-04.R7). The action creates a project only; it does not launch an agent or navigate away.

**R43.** On an active scoped project dashboard, each **New agent** action opens the
existing New Agent modal with that route's project fixed as the launch target. The modal does not
render a project picker, and its submission always sends that project id. The unscoped New Agent
modal and prefilled launches outside a scoped dashboard retain their existing project selection
behavior.

- **R44** — The dashboard shows how many tasks in view need attention, covering parked
  `dependency_failed` work and `interrupted` work whose agent went away without a result (FS-16.R8,
  R16).
  The indicator opens the Tasks view (FS-16.R14); the card grid itself gains no task object, and the
  state badge vocabulary in R3 is unchanged, because an armed task has no agent and therefore no card.
  If the task query fails, the indicator says attention is unavailable rather than asserting a zero count.

- **R45** — Within each group section of an agent card grid, running agents are
  placed before agents that are not running, and the persisted manual order (R12) is preserved
  within each of those two blocks. `running` is the sole test, matching the dimmed stopped treatment
  in R6; the five live `state` values in R3 do not affect placement, so a `busy` and an `idle`
  running agent keep their relative manual order. Group sections keep the order R18 gives them, with
  Ungrouped last, and this rule reorders cards only inside a section. The split is applied when the
  grid renders, so a card moves across the boundary as soon as its agent starts or stops, with no
  reload. Nothing is persisted for it: `layout.json` continues to hold exactly the order, density,
  and per-group collapse state it holds today (R14), a drag still commits and persists the manual
  order it produced, and the same shared preferences continue to drive every scoped project grid
  (R36). Because the manual order is one flat list shared by every project view, a drag inside one
  block is recorded in that shared list and does not become a per-project or per-status preference.
  During that drag, only cards in the same running/stopped block shift to show the possible drop;
  cards in the other block hold their positions. Dropping onto a card in the other block performs no
  reorder and does not save the layout, because manual drag order cannot override the running-first
  boundary. The existing separate cross-group drag behavior is unchanged.

- **R53** — A drag that cannot be dropped says so while it is still in flight.
  While a dragged card is over a card in the other running/stopped block, the grid marks the drag
  refused and the pointer states it, so the person learns the drop will not land before releasing
  it rather than only from an unexplained snap-back (R45). The mark clears when the pointer returns
  to the card's own block and when the drag ends or is cancelled. Nothing else about the drag
  changes: no card moves that would not otherwise move, no request is made, and no message
  interrupts the drag.

### Expanded chat panes

- **R46** — An agent card whose agent has the `chat` interface can be
  expanded in place into a chat pane. The host is the agent card grid on a project dashboard
  (`/project/:project`), which after the R29 project-first split is the only surface that renders
  agent cards; the projects home renders project cards and gains nothing here. Clicking a collapsed
  card toggles that expansion. This supersedes
  R8's clause that a card-body click navigates to `/agent/:id`; R8's right-click behavior, the
  context menu's **Open chat** (R15), and the route itself are unchanged. A card whose agent has the
  `terminal` interface does not expand and keeps R8's navigation, because the pane deliberately
  carries no terminal (FS-03.R39) and a terminal agent takes no composer input (FS-07).

- **R47** — An expanded pane occupies `min(2, perRow)` columns of the card
  grid (R13) at a fixed height, and its transcript scrolls inside the pane rather than growing the
  page, so streamed output in one pane never moves the reader's position in another. Collapsed cards
  continue to flow through the remaining grid tracks. A pane stays in its own group section (R18) and
  in its running/stopped block (R45), so expanding or collapsing a card never changes card order or
  grouping and never changes what its context menu offers. An expanded card is not draggable: its
  drag grip is withheld while expanded and returns on collapse, so reordering it means collapsing it
  first. It remains a member of its block's sortable set for the purpose of *other* cards' drags,
  though — a collapsed card dragged past a pane must see the pane's two-column footprint, or every
  neighbour's in-drag preview is computed over a layout that is not on screen. Withholding the grip,
  not removing the card from the set, is what makes it undraggable. Collapsing returns the card to
  its ordinary size in the position R45 gives it.

- **R48** — At most four panes are expanded at once. Expanding a fifth
  collapses the least-recently-used expanded pane, without a prompt and without losing work: unsent
  composer text is already retained per agent by the browser (FS-03.R36) and is restored when that
  pane is expanded again. Three events, and only these, mark a pane as used: expanding it, focus
  entering anywhere inside it, and a pointer press inside it. The pointer press is required because
  the transcript's scroll region is not focusable, so an operator who spends minutes reading a pane
  without touching its composer would otherwise still be holding the least-recently-used pane and
  would lose the one they were reading. Panes are ordered least-recently-used first, and the first
  entry is the one a fifth expansion collapses.

- **R49** — The set of expanded panes is an ordered list of agent ids
  persisted in `layout.json` beside order, density, and per-group collapse, loaded by the same
  `GET /api/layout` and saved by the same debounced `PUT /api/layout` (R14). A reload or server
  restart restores the same panes. A layout file that records no expanded panes — including every
  file written before this feature — loads with every card collapsed. An id naming an agent that no
  longer exists or is archived expands nothing and is dropped from the next save. An id naming an
  agent outside the grid's current project is **retained**: it renders no pane and is written back
  unchanged. Pruning it instead would destroy one project's arrangement every time the operator
  opened another project, because R29 gives every agent grid a project scope and only one is ever
  mounted. Like order and density (R36), the list is one shared preference rather than a per-project
  one, so R48's limit of four is a limit on the whole list; a list that somehow exceeds it keeps its
  first four entries.

- **R50** — While focus is inside a card grid, `Ctrl+Alt+ArrowDown` moves
  focus to the next expanded pane's composer and `Ctrl+Alt+ArrowUp` to the previous one, wrapping at
  each end and scrolling the target pane into view. Order follows the panes as displayed. With fewer
  than two panes expanded, both do nothing. Neither binding reaches the composer's own keys (FS-03.R6)
  or its `@`/`#` picker keys (FS-03.R31), so typing and completion are unaffected.

- **R51** — A pane is opened only by a person. No notification, state
  change, permission request, or `waiting_input` transition expands a card by itself, so the grid
  never reflows while it is being read; R3's badge and salience treatment remain the attention
  signal. A pane whose agent is removed (R21) closes with its card. A pane whose agent stops stays
  open and keeps showing the durable transcript, matching R6's rule that stopping is not removal.


- **R52** — Expanding a card moves its activation target. While a card is
  collapsed, its whole body toggles expansion (R46) and its whole body opens the card context menu
  (R8). While it is expanded, both targets narrow to the card's header region — the row carrying the
  name, state, and collapse control. The pane's content region activates neither: a click on Send,
  Cancel, Approve, Deny, a transcript disclosure, an autocomplete entry, the pane's name link, or a
  text selection does not collapse the pane, and a right-click inside the transcript reaches the
  annotation flow (FS-13) rather than the card menu. The boundary is the region, not a list of
  exempted controls, so a control added to the pane later inherits it and cannot silently reintroduce
  the collapse-on-Send defect.

- **R54** — The persistent application shell offers a compact project-navigation link
  for each configured, non-archived project that has at least one non-archived agent whose
  `running` value is true. Membership updates from the existing project catalog and hydrated
  `state_update` projection without a manual refresh. Selecting a link opens the existing
  `/project/:project-id` dashboard. A project remains represented and selected while the person is
  viewing that project dashboard or an `/agent/:id` route belonging to it, even after its last
  running agent stops; it leaves the shell after the person navigates elsewhere. A deleted or
  otherwise unavailable project configuration receives no shell link, even when a running agent
  still references its id; it remains reachable through the unavailable-project card required by
  R29–R32. Eligible projects are ordered alphabetically by displayed title, with durable project id
  as the deterministic tie-breaker. At most five project links are directly visible. When the
  current project would fall outside the first five, it replaces the fifth visible project; both
  the resulting visible set and the remaining overflow set retain alphabetical order. FS-12.R39
  presents every remaining project under `+n`.

### Pane stability, collapse, and card legibility

- **R55 (planned)** — An expanded pane occupies exactly one column track of the
  card grid, superseding R47's `min(2, perRow)` span; every other clause of R47 stands. Expanding or
  collapsing a card therefore changes no card's column or row index and moves no card sideways: the
  card grows taller in the cell it already occupies, its collapsed neighbours in the same grid row
  keep their positions and their own heights, and only the rows below that row move. The card the
  person clicked stays under the pointer, so the same click collapses it again. This removes the
  wrap-and-gap the two-track span produced, in which a pane that did not fit a row's remaining
  tracks jumped to the next row and left a hole behind it. The pane's fixed height, its internally
  scrolling transcript, and the rule that streamed output never moves the grid or the page are
  unchanged.

- **R56 (planned)** — An expanded card carries a visible, labelled collapse
  control in its header region. It states that it collapses the pane, is reachable and operable from
  the keyboard, and gives hover and focus feedback, so collapsing no longer depends on finding
  unoccupied pixels between the name link and the state badge. R52's rule that the whole header
  region collapses the pane is unchanged and remains the larger secondary target; the header's name
  link keeps navigating to `/agent/:id` rather than collapsing. Collapsing through the control and
  collapsing through the header region have exactly the same effect, including the retention of
  unsent composer text (R48).

- **R57 (planned)** — The card grid's toolbar offers **Collapse all**, which
  collapses every pane expanded on that grid in one action. It is present only while at least one
  pane on the grid is expanded, so the toolbar never shows a control that would do nothing. It
  spans every group section of the grid rather than one section, matching R48's whole-grid cap and
  R50's whole-grid cycling. It collapses panes and nothing else: group-section collapse (R18), card
  order, density, grouping, and each agent's retained unsent composer text (R48) are untouched, and
  a re-expanded pane restores its draft as it does after any other collapse. Because the expanded
  list is one shared preference (R49), **Collapse all** removes only the ids the current grid is
  showing; an id retained for an agent outside this grid's project stays in the list and is written
  back unchanged, for the same reason R49 retains it. The result persists through the same debounced
  layout save as any other collapse, and a save that fails reports the failure exactly as R49's save
  path already does, leaving the panes collapsed on screen.

- **R58 (planned)** — An agent's name on a card is legible rather than clipped
  to one line. On both a collapsed and an expanded card the name renders at a smaller type size than
  the shipped card title and wraps onto as many as three lines instead of ending in an ellipsis, so
  the long generated names agents actually carry are readable without opening anything. A name too
  long for three lines is still clipped there, and a single unbroken token wider than the card is
  broken rather than allowed to overlap the state badge, the drag grip, or the card's edge. A
  wrapped name makes its own card taller and changes nothing else: card minimum height, column
  count, gap, and the density control (R13) keep their shipped values and behavior, and no other
  card's geometry changes because one name is long. R2's list of what a card displays is otherwise
  unchanged.

- **R59 (planned)** — The context-usage meter leaves the collapsed card and
  appears on the expanded card instead, narrowing R2's card-content list and R26 to that placement.
  A collapsed card shows no context usage. An expanded card states the same live value as a compact
  figure in its header region, keeping R26's rounded percentage, its live mid-turn tracking, and its
  rule that zero and absent values are visibly labelled rather than left as an unexplained empty
  track; the proportional bar itself is not required at that size. The agent screen's own context
  meter (FS-12.R12) is unchanged, and no context data is fetched or retained differently.

## 5. Acceptance criteria

**A1.** Launching an agent adds its card within ~1s with no manual refresh; a status change flips the
badge live. — J3 (launch + status transitions), J5 (grid).

**A2.** `applyStateUpdate` upserts an agent and appends its id to `order` exactly once; a single
card's `state_update` re-renders only that card. — `agentStore.test.ts` "upserts agents and appends
order once"; `sse.test.ts` state_update selector-isolation cases.

**A3.** After a reconnect, agents absent from the completed hydration burst are pruned (no stale
cards). — `agentStore.test.ts` "removes stale agents after hydration completes"; `sse.test.ts`
"resets the hydration generation on auto-reconnect so deleted agents are pruned".

**A27.** Six same-origin dashboard tabs share one underlying SSE connection and can load and refetch
REST data concurrently without waiting for another tab to close. — `ui/src/api/sse.test.ts` proves
the transport half: one shared stream across ports, a joining port replayed on its own without
restarting the stream or re-hydrating the other tabs, a departing port removed and the stream
dropped with the last one, and all three fallback routes. The six-tab claim itself is a
real-browser check against a `make dist` build and is **owed**, not met: it has never been run
against a build carrying the shared stream. `scripts/stress-fixture` (TS-06 §6) is the fixture for
it.

**A4.** Reorder, density, and group-collapse persist across page reload and server restart. — J5;
`TestPutLayoutValidatesAndPersists`, `TestLayoutDefault`.

**A5.** A `PUT /api/layout` with out-of-range `perRow`/`gap` is rejected. — `TestPutLayoutValidatesAndPersists`.

**A6.** A context-menu action failure surfaces an error toast with the server message. —
`CardContextMenu.test.tsx` "shows an error toast … when switch-runtime/rename/stop/clone/move fails".

**A7.** Clone launches a new session with the source agent's config. — `CardContextMenu.test.tsx`
"clones an agent by launching a new session with the same config".

**A8.** Releasing a group stops all of its member agents. — `TestReleaseGroupStopsMembers`; J5.

**A9.** A `done`/`waiting_input` notification fires only on a state transition into that state. —
`TestStateUpdateEmitsNotificationOnTransition`; `TestPermissionRuntimeEventEmitsNotification` for
permission_required.

**A10.** A muted notification type produces no toast and no desktop notification; a hidden tab with
granted permission uses a Web Notification. — `sse.test.ts` "drops muted notification types",
"uses Web Notification for hidden tabs when permission is granted"; `NotificationsEditor.test.tsx`
"persists a per-type mute toggle".

**A11.** A disappeared agent process reconciles its card to `done` (not `error`). —
`TestPruneStaleRunning`.

**A12.** Toast auto-dismiss timers are per-toast; a new toast does not restart older timers. —
`NotificationCenter.test.tsx` "dismisses each toast independently".

**A13.** A dashboard card with a configured project renders its title rather than its id, falls back
to the id if the definition is absent, and visibly labels a zero-value context meter. —
`CardGrid.test.tsx` "uses the configured project title on agent cards"; `AgentCard.test.tsx`
"falls back to the project id when its title is unavailable"; `ContextBar.test.tsx` "labels zero
context usage"; J5.

**A14.** A stopped card's context menu shows **Resume** and resumes through the existing endpoint;
the same menu on a running card omits it, and a rejected resume surfaces a toast. —
`CardContextMenu.test.tsx` "resumes a stopped agent from its card" and "shows an error toast when
resume fails"; J5.

**A15.** The home grid shows configured empty projects and state/count summaries; an
agent whose project definition was removed remains reachable through an unavailable-project card and
updates live, and that card disappears when its last non-archived agent is archived. — project-
dashboard component/store regressions; `ProjectDashboard.test.tsx` "shows project color and live
state summary, with all active-project actions"; J5.

**A16.** Selecting a project filters the dashboard to exactly its agents, omits project
text from their cards, preserves their existing lifecycle actions, lets an unavailable-project card
open its remaining non-archived agents, and supports browser Back to the project grid. — route/
component regressions; J5.

**A17.** An archived project is absent from the active home grid, remains reachable in
the Archive with its agents, and reappears on the active grid after Restore without losing its
configuration. Agents remain archived until individually restored. Direct `/project/:project-id`
navigation while the project is archived shows an archived-project state linking to Archive rather
than an empty dashboard. — project-dashboard/config regressions; J5.

**A18.** A stopped agent remains on its active project's dashboard with Resume available.
Right-clicking that project's card can rename, recolor, or archive it; confirmed project archival
stops running agents and archives every agent, while a rejected action keeps it visible. — project
context-menu/dashboard regressions; `ProjectDashboard.test.tsx` "shows project color and live state
summary, with all active-project actions"; J5, J7.

**A19.** Archiving one agent stops it when necessary without a confirmation, removes only
that agent from the active dashboard, and groups it beneath its project in Archive; that group shows
its count and marks the project active when it still appears on the main dashboard. — agent/archive
dashboard regressions; J5, J7, J8.

**A20.** Switching between project dashboards does not create or migrate layout state;
each scoped agent grid applies the existing shared layout preferences. — `CardGrid.test.tsx` "merges
a scoped reorder back into the shared layout"; J5.

**A21.** (R37) Each dashboard dialog opens from its menu, validates or suggests as specified, issues
the same request on confirm, and performs no action on Cancel; the move-to-group combobox lists
existing group labels. — `CardContextMenu`, `CardGrid`, and `ProjectDashboard` component tests plus
the FS-12.A8 guard.

**A22.** (R38, R39) Right-clicking an active project card opens a cursor-positioned portal
menu — not an inline card expansion — offering Rename, an inline six-swatch color picker, and
Archive; picking a swatch immediately recolors the card and marks the active preset, while Rename and
Archive still open their dialogs. The unavailable-project card offers no such menu. — `ProjectDashboard`
component tests; J5.

**A23.** (R40) Agent cards and the home project card render a project-accent background
wash and a tinted border alongside the retained left-edge accent, while the top bar still reflects
agent state; the visual-matrix fixtures show the tint across the six presets in Core and Sky & Grove
at readable contrast. — `AgentCard`/`ProjectDashboard` component tests and the FS-12 visual matrix; J5.

**A24.** On the projects home, a persistent **New project** button and a background
right-click menu each open the create modal, while right-clicking a project card opens the card menu
rather than the create menu. Submitting a valid project creates it through `POST /api/projects` and
its card appears on the grid without a reload; an invalid submission keeps the modal open showing the
error, and Cancel closes it with no change. The background surface covers the whole canvas including
the shell's padding frame (R41), which the component suite cannot observe (INV §13): the padding's
ownership is asserted against the stylesheets, and J5 checks the live surface. —
`ProjectDashboard` component test (behavior and stylesheet ownership); J5.

**A25.** (R43) Opening **New agent** from a scoped project dashboard omits the project
picker and launches with the route project's id; the general modal continues to offer its picker. —
`CardGrid.test.tsx`, `NewAgentModal.test.tsx`; J5.

- **A26** (R44) — The needs-attention count reflects exactly the `dependency_failed` and
  `interrupted` tasks in view, updates after their authoritative commit, does not count an ordinary
  `armed`, `ready`, `starting`, or `running` task, and reads zero when no task needs attention:
  dashboard UI tests. A failed task query renders the unavailable state instead of zero.

- **A28** (R45) — In a group containing running and stopped agents in interleaved
  manual order, the grid renders every running agent before every stopped one while preserving the
  manual order inside each block; flipping one agent's `running` via a `state_update` moves only that
  card across the boundary and leaves both blocks otherwise unchanged; during a drag inside one
  block, only that block's cards shift, and the completed drag produces the same `PUT /api/layout`
  order payload it produces today and survives reload; a drop onto the other block leaves both the
  displayed order and persisted layout unchanged; and the Ungrouped section stays last. *Verify by*
  new render, drag-geometry, payload, and no-request cases in
  `ui/src/components/grid/CardGrid.test.tsx` beside the existing scoped-reorder case, and by **J5** in
  `docs/features/USABILITY-REVIEW.md` for the live start/stop and drag behavior in a real browser.

- **A35** (R53) — Dragging a running card over a stopped card marks the grid refused;
  moving back over a card in its own block clears the mark, and ending or cancelling the drag clears
  it. *Verify by* a drag-over case in `ui/src/components/grid/CardGrid.test.tsx`, and by **J5** in
  `docs/features/USABILITY-REVIEW.md` for the pointer treatment itself, which jsdom cannot evaluate.

- **A29** (R46, R47) — Clicking a collapsed chat-agent card expands it in place
  instead of navigating; clicking its header again collapses it; the expanded card carries the
  column span R47 requires for `perRow` 4 and for `perRow` 1; and a terminal-agent card still
  navigates to `/agent/:id`. *Verify by* new render and interaction cases in
  `ui/src/components/grid/CardGrid.test.tsx` and `AgentCard.test.tsx` — these assert the rendered
  span and component tree, which is all jsdom can see.

  The geometry itself is verified only in a real browser, by **J5** in
  `docs/features/USABILITY-REVIEW.md`: computed pane height is fixed, streaming a long assistant
  response scrolls that pane's transcript while the page and grid scroll positions do not move, and
  the collapsed cards sharing the pane's row keep their own height and position. jsdom evaluates no
  grid track sizing, stretch, computed overflow, or scroll position, so a jsdom-only claim here would
  pass against a pane that stretches its neighbours or grows the page (INV §13).

- **A30** (R47, R51) — Expanding and collapsing a card changes no card's order,
  group, or context-menu contents, including across the running/stopped boundary; an expanded card
  exposes no drag grip and a collapsed one still does; a `state_update` moving an agent to
  `waiting_input` expands nothing; and a removal tombstone for an expanded agent removes both the
  pane and the card. An agent that goes from running to stopped while its pane is open keeps that
  pane, its durable transcript, and its composer as a wake surface (FS-03.R35/R39) — pane membership
  is not keyed to `running`, which every other case here would fail to catch. *Verify by*
  `CardGrid.test.tsx` cases asserting order, grip presence, and pane membership before and after each
  event, including the running→stopped `state_update`.

- **A31** (R48) — With four panes expanded, expanding a fifth leaves exactly four
  expanded, collapses the least-recently-used one and no other, shows no confirmation, and
  re-expanding the collapsed agent restores the composer text that was unsent when it closed. Each
  of R48's three recency events is exercised separately as the thing that saves a pane from
  eviction: focusing its composer, pressing a pointer inside its transcript without focusing
  anything, and expanding it. — `CardGrid.test.tsx` and `Composer.test.tsx`/`drafts.test.ts` for the
  retained draft.

- **A32** (R49) — A layout response with no expanded field renders every card
  collapsed; expanding and collapsing panes issues the same debounced `PUT /api/layout` that order
  and density issue, carrying the expanded ids; the persisted set is restored on reload; a persisted
  id for an unknown or archived agent expands nothing and is absent from the next `PUT` body; and a
  persisted id belonging to another project expands nothing yet still appears, unchanged, in that
  `PUT` body, so opening a second project does not discard the first project's panes. —
  `CardGrid.test.tsx` payload and load cases plus a Go handler test that round-trips the field and
  accepts a body without it.

- **A33** (R50) — With three panes expanded, `Ctrl+Alt+ArrowDown` from the first
  pane's composer focuses the second, repeats to the third, and wraps to the first;
  `Ctrl+Alt+ArrowUp` reverses it; with one pane expanded neither changes focus; and neither
  intercepts a keystroke while the composer's `@` or `#` picker is open. — `CardGrid.test.tsx`
  keyboard cases and **J5** in `docs/features/USABILITY-REVIEW.md` for the real-browser focus and
  scroll-into-view behavior.

- **A34** (R52) — With a pane expanded, none of the following collapses it or opens
  the card context menu: clicking Send, clicking Cancel mid-turn, deciding a permission request,
  expanding a tool-result disclosure, accepting an `@` or `#` autocomplete entry, dragging a text
  selection across the transcript, or right-clicking inside the transcript — which instead reaches
  the annotation flow. The pane's name link navigates to `/agent/:id` without first toggling the
  pane. Clicking the card header still collapses it, and every one of these gestures on a *collapsed*
  card behaves as it does today. — `CardGrid.test.tsx` and `AgentCard.test.tsx` interaction cases,
  each asserting both that the pane stayed open and that no card context menu opened.

- **A36** (R54) — With running agents in two configured projects, both project links
  appear from every primary route and each opens the matching scoped dashboard. Stopping the last
  running agent removes its project's link without a reload when another route is open, but keeps
  that link selected while the person remains on the project's dashboard or one of its agent
  routes; leaving that context then removes it. A configured project containing only stopped or
  archived agents and an unavailable project id referenced by a running agent produce no link. A
  completed hydration that omits a formerly running agent removes the corresponding stale link.
  With six alphabetically distinct eligible projects, the first five are directly visible and the
  sixth is under `+1`; selecting that sixth project makes it directly visible, displaces the prior
  fifth project into `+1`, and leaves each set alphabetized. — shell/component and hydration
  regressions; J5.

- **A37 (planned)** (R55) — In a three-column grid, expanding the first card of a
  six-card section leaves every other card in the column and row index it held before the
  expansion, and collapsing it restores the same layout; no card wraps to a different row and no
  empty track appears. The same holds for expanding a middle card and for a `perRow` of 1. An
  expanded card occupies one track, not two. — `ui/src/components/grid/CardGrid.test.tsx`; a
  real-browser check at the supported desktop floor and at a wide viewport, in Core and Sky & Grove,
  confirming that nothing but the rows below the pane moves.

- **A38 (planned)** (R56) — An expanded card exposes a collapse control with an
  accessible name that says it collapses; activating it by pointer and by keyboard collapses the
  pane, and the control shows hover and focus feedback. Clicking the header region outside the name
  link still collapses, clicking the name link navigates to `/agent/:id` without collapsing, and
  A34's list of pane interactions still collapses nothing. Composer text typed into the pane is
  present again after re-expanding, whichever of the two collapse routes was used. —
  `ui/src/components/grid/AgentCard.test.tsx`; J5.

- **A39 (planned)** (R57) — With no pane expanded the toolbar shows no **Collapse
  all**; with panes expanded in two different group sections it appears and one press collapses
  both, leaves both group sections' collapse state, the card order, and the density unchanged, and
  survives a reload. An expanded id belonging to an agent outside the current grid's project is
  still in the saved layout afterwards, and expanding a pane again restores its retained draft. A
  refused layout save reports the failure and leaves the panes collapsed on screen. —
  `ui/src/components/grid/CardGrid.test.tsx`.

- **A40 (planned)** (R58) — A card whose agent name needs three lines renders all
  three, at a smaller size than the shipped card title and with no ellipsis, without overlapping the
  state badge or the drag grip and without escaping the card; a name longer still is clipped at the
  third line, and a single unbroken token wider than the card is broken rather than overflowing. The
  same name in an expanded card's header behaves the same way. Cards whose names are short keep the
  shipped card height, and the column count and gap the density control produces are unchanged. —
  `ui/src/components/grid/AgentCard.test.tsx`; a real-browser check with a long generated agent name
  in Core and Sky & Grove.

- **A41 (planned)** (R59) — A collapsed card renders no context meter. An expanded
  card states the live context percentage in its header, updates it mid-turn as further usage
  reports arrive, and labels a zero or absent value rather than showing an unexplained blank. The
  agent screen's context meter is unchanged. — `ui/src/components/grid/AgentCard.test.tsx` and the
  existing context-meter regressions; J5.

## 6. Deviations & open decisions

- **Immediate clone UI.** Clone launches immediately with no confirmation, and a disappeared process
  is surfaced as `done` rather than `error` (R11, R16, A11); reversing either requires an explicit
  feature-spec update plus a confirmation or changed process-exit semantics.
- **Context-menu items are all wired.** The Phase-2 tech spec specced Switch runtime / Clone / Move
  to group as visible-but-disabled stubs (tooltips "Available in Phase 3/6"). Phase 6 shipped, so
  current truth is that every menu item is functional; the stubbing described in the tech spec is
  superseded history, not current behavior.
- **`budget_exceeded` notification type.** It shares the notification pipeline and the per-type mute
  list surfaced here, but its emission and meaning are owned by FS-06 (coordination); this spec only
  governs its display and mute in the notification surface.
- ⚠ unverified: the "~1s" freshness bound (R9, A1) is an interactive/manual observation (J3/J5) —
  no automated timing assertion pins it; it is gated behind the credential-limited live E2E journeys
  (credential-gated acceptance).
- **Confirmed project-first boundary.** R29–R36 retain configured projects at the home level,
  surface non-archived agents under an unavailable-project card, and keep the current shared card-
  layout preferences. Independent per-project order, density, and collapsed-group preferences are
  excluded from this change. Archive is project-based; project archival is warning-confirmed, while
  individual agent archive stops and archives immediately without confirmation.
- **Confirmed active-project shell boundary.** R54 uses `running` as the sole activity test,
  excludes unavailable project configurations, retains the current project only across its last
  agent stopping, and uses title/id alphabetical order with a `+n` overflow. It adds no pinning,
  recency tracking, activity ranking, count badge, or project-specific persistence.

- **Confirmed grid-stability and legibility boundary.** R55–R59 keep expansion in place and change
  only what the reported experience named: the pane's track span, an explicit collapse control, one
  **Collapse all**, a wrapped smaller card name, and the context meter's move onto the expanded
  card. Card minimum height, column count, gap, and the density control keep their shipped values —
  making cards larger was considered and declined. Panes are not moved out of the grid into a
  separate region, expansion gains no keyboard collapse binding beyond the control's own focus,
  **Collapse all** gains no expand-all counterpart and no confirmation, and nothing here changes
  grouping, running-first placement, drag behavior, the four-pane cap, pane persistence, or any
  server surface.

## 7. Traceability

- **Grid & cards:** `ui/src/components/grid/CardGrid.tsx`, `AgentCard.tsx`, `StateBadge.tsx`,
  `ContextBar.tsx`, `CardContextMenu.tsx`, `DensityControl.tsx`, `EmptyState.tsx`.
- **Project cards and scoped routes:** `ui/src/features/dashboard/ProjectDashboard.tsx` (project
  cards, context menus, and the New project button/background-menu create modal reusing
  `ui/src/features/settings/ProjectForm.tsx` and `useCreateProject`); archive catalog updates in
  `ui/src/features/archive/ArchivePage.tsx`.
- **Stores:** `ui/src/store/agentStore.ts` (upsert/hydrate/remove/order), `uiStore.ts` (density,
  groupLayout, toasts, context menu), `transcriptStore.ts` (last-line fallback source).
- **SSE + notifications dispatch:** `ui/src/api/sse.ts` (`onStateUpdate`, `onNotification`),
  `ui/src/components/shell/NotificationCenter.tsx`, `ui/src/features/settings/NotificationsEditor.tsx`.
- **Server:** `internal/bus/bus.go` (`state_update`, notification emission on transition),
  `internal/state/manager.go` (effective-state recompute, tombstones),
  `internal/server/handlers.go` (`GET/PUT /api/layout`, `pruneStaleRunning`, release group).
- **Key regression tests:** `TestStateUpdateEmitsNotificationOnTransition`,
  `TestPublishDropsOldestForSlowSubscriber`, `TestManagerRecomputeRunningFalseAndRemovalTombstone`,
  `TestPruneStaleRunning`, `TestReleaseGroupStopsMembers`, `TestPutLayoutValidatesAndPersists`;
  UI: `agentStore.test.ts`, `sse.test.ts`, `CardContextMenu.test.tsx`, `NotificationsEditor.test.tsx`,
  `NotificationCenter.test.tsx`, `ProjectDashboard.test.tsx` (project cards, context menus, and the
  New project create modal — A22/A24), `CardGrid.test.tsx` (running-first placement, the sortable
  registry order, and same-block/cross-block drop behavior — A28).
- **Planned grid stability and legibility:** R55–R59 and A37–A41 are unshipped; their evidence is
  named there and in TS-08.R45–R48 and FS-12.R40.
