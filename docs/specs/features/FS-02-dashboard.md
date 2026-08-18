# FS-02 — Dashboard (card grid home view)

**Status:** Current
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

## 5. Acceptance criteria

**A1.** Launching an agent adds its card within ~1s with no manual refresh; a status change flips the
badge live. — J3 (launch + status transitions), J5 (grid).

**A2.** `applyStateUpdate` upserts an agent and appends its id to `order` exactly once; a single
card's `state_update` re-renders only that card. — `agentStore.test.ts` "upserts agents and appends
order once"; `sse.test.ts` state_update selector-isolation cases.

**A3.** After a reconnect, agents absent from the completed hydration burst are pruned (no stale
cards). — `agentStore.test.ts` "removes stale agents after hydration completes"; `sse.test.ts`
"resets the hydration generation on auto-reconnect so deleted agents are pruned".

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
error, and Cancel closes it with no change. — `ProjectDashboard` component test; J5.

**A25.** (R43) Opening **New agent** from a scoped project dashboard omits the project
picker and launches with the route project's id; the general modal continues to offer its picker. —
`CardGrid.test.tsx`, `NewAgentModal.test.tsx`; J5.

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
  New project create modal — A22/A24).
