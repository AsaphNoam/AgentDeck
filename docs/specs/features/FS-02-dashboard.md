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

**R4.** The context-usage meter renders `context_pct` (0..1) as a proportional bar with a rounded
percentage label, color-ramped green (`< 0.6`) → amber (`0.6–0.85`) → red (`> 0.85`). A missing or
zero value renders an empty bar with no label.

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

### Task groups

**R18.** Agents carrying a non-empty `group` label are rendered under a collapsible group section;
agents with no group fall under a trailing "Ungrouped" section (which sorts last). A group header
shows the group label, member count, and a per-state count summary.

**R19.** A group section's collapsed/expanded state is toggled from its header and is persisted per
group in `layout.json` (`groups.<name>.collapsed`).

**R20.** A named group header offers **Release group**, which — after a confirm — stops every agent in
that group in one action. The Ungrouped section has no release control.

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

## 6. Deviations & open decisions

- **Immediate / prompt-based UI.** Runtime-switch and
  move-to-group collect their arguments through browser `window.prompt`/`confirm` dialogs rather than
  dedicated form modals; Clone launches immediately with no confirmation; and a disappeared process
  is surfaced as `done` rather than `error` (R11, R16, A11). Reversing any part requires an explicit
  feature-spec delta plus dedicated dialogs/confirmations or changed process-exit semantics.
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

## 7. Traceability

- **Grid & cards:** `ui/src/components/grid/CardGrid.tsx`, `AgentCard.tsx`, `StateBadge.tsx`,
  `ContextBar.tsx`, `CardContextMenu.tsx`, `DensityControl.tsx`, `EmptyState.tsx`.
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
  `NotificationCenter.test.tsx`.
