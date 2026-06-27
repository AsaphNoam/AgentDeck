# Phase 7 — Implementation Tech Spec: Activity Map / Ambient Visualization (F13)

**Mirrors:** `docs/phases/phase-7-polish-activity-map.md` (phase PRD)
**Feature:** F13 (activity map)
**Depends on:** Phase 2 (`/api/events` SSE bus, React app shell, global agent store) and Phase 5 (`new_message` / `notification` events that drive animations)
**Status:** optional / lowest priority. Pure frontend. No server changes.

---

## 1. Overview & scope recap

The activity map is an **ambient, spatial view** of the same agents already shown in the card grid. Each live agent is a marker on a 2D map. Markers sit in **zones** chosen by the agent's `state`, glide to a new zone when the state changes, and fire a transient animation when a message or notification involves that agent.

Hard constraints — this phase is **purely additive**:

- **No new backend.** No new REST endpoints, no new SSE event types, no new files under `~/.agentdeck/` written by the server. The map is a read-only consumer of the existing `/api/events` stream (`state_update`, `new_message`, `notification`, `ping`) and the existing global agent store from Phase 2.
- **Additive UI only.** The card grid is untouched and remains the default. The map is reachable via a toggle. Removing the map must not affect anything else.
- **Optional / skippable.** If this phase is dropped, the product is unaffected. Nothing else may take a dependency on map code.
- **One new client-read-only config file** for zone geometry, `zones.json` (schema in §3.4). It is read by the UI at runtime; it is *paintable* by hand-editing. The server does not need to write or validate it; if absent or invalid, the map falls back to a built-in default layout (§3.4, §5).

Out of scope: any gameplay/interaction beyond ambient visualization; persisting marker positions; new server data; mobile layout.

---

## 2. Technology choices

### 2.1 Rendering tech — **SVG** (decision locked, see §8)

Expected agent counts are small: the dashboard supervises on the order of **5–30 concurrent agents**, with a practical ceiling around **~50** before resource pressure (master PRD §9 flags concurrency limits as a tuning concern, not a high-N scenario). At those counts SVG is the right tool:

- **Declarative + React-native.** Markers map 1:1 to React components (`<g>` / `<circle>` / `<text>`) keyed by `agent_id`. State → position is plain React rendering; no imperative draw loop, no manual hit-testing, no manual repaint-on-resize.
- **CSS/SMIL/WAAPI animation for free.** Zone transitions and message pulses are CSS transitions / Web Animations API on DOM nodes — no per-frame `requestAnimationFrame` bookkeeping.
- **DOM affordances for free.** Hover tooltips, focus rings, click-to-open-chat, and accessibility (`role`, `aria-label`) come from the DOM. On canvas we'd reimplement all of it.
- **Performance is a non-issue at this N.** A few dozen animated SVG nodes is trivial for any modern browser. Canvas only wins in the hundreds-to-thousands-of-sprites regime, which this product never reaches.

Canvas/WebGL is explicitly **rejected** for this phase: it would buy nothing at ≤50 markers and cost us hit-testing, text layout, accessibility, and animation ergonomics.

### 2.2 Animation approach

- **Zone movement (marker repositioning):** CSS transition on the marker group's transform. Set `transform: translate(x, y)` (via a style prop or CSS custom properties) and let `transition: transform 600ms cubic-bezier(0.22, 1, 0.36, 1)` interpolate. Position changes come from React re-render on `state_update`; the browser animates the delta. No JS tween library.
- **Message / notification pulse:** the **Web Animations API** (`element.animate(...)`) triggered imperatively in an effect when a `new_message` / `notification` event arrives, because these are one-shot, fire-and-forget effects that must run even when the target position is unchanged. Two animation primitives:
  - **Pulse** — a ring/glow scaling out and fading (`transform: scale`, `opacity`) on the involved marker(s).
  - **Travel dot** — a small dot animated along the straight line from sender marker to recipient marker (interpolating `cx`/`cy` or a transform), representing message direction (§5, §8).
- **Respect `prefers-reduced-motion`:** when set, disable zone-glide transitions (snap to position) and replace pulses/travel dots with a brief static highlight (single opacity flash, no motion).

### 2.3 Libraries

- **No new animation or rendering dependency.** SVG + CSS transitions + the built-in Web Animations API cover everything. Do not add D3, react-spring, framer-motion, pixi, konva, etc. — they are unjustified for this scope and conflict with the "minimal, local-first" stack.
- Reuse the existing Phase 2 stack only: React + TypeScript + Vite, the existing SSE client, and the existing global agent store.

---

## 3. Map view design

### 3.1 Coordinate system

The map uses a fixed **logical coordinate space of `1000 × 600` units** (the SVG `viewBox="0 0 1000 600"`). All zone geometry in `zones.json` is expressed in these logical units, so the map scales responsively to any container size without changing the config. The SVG element is `width="100%"` with `preserveAspectRatio="xMidYMid meet"`.

### 3.2 Zones and state → zone mapping

A **zone** is a named rectangular region with a label and accent color. Agents are placed inside the zone their `state` maps to. The five `status.state` values (master PRD §3.1: `busy | idle | waiting_input | done | error`) map to zones via the config's `stateToZone` table. The shipped default:

| `status.state`   | Default zone id | Rationale |
|------------------|-----------------|-----------|
| `busy`           | `busy`          | Actively working. |
| `idle`           | `idle`          | Available, doing nothing. |
| `waiting_input`  | `waiting`       | Needs the human; its own zone so it stands out. |
| `done`           | `done`          | Finished; parked. |
| `error`          | `error`         | Failed; parked, visually distinct. |

The PRD names only `idle`/`busy` zones explicitly and asks for "reasonable handling" of the other three. **Resolved (§8): each of the five states gets its own zone in the default layout**, but the mapping is fully data-driven — a painter can collapse `waiting_input`/`done`/`error` into `idle`/`busy` purely by editing `stateToZone` in `zones.json`, no code change. Any state with no entry in `stateToZone`, or an entry pointing at a missing zone id, falls back to the zone marked `"fallback": true` (default `idle`).

### 3.3 Marker placement, movement, and animation

- **One marker per live agent**, keyed by `agent_id`. The agent set comes directly from the Phase 2 global store (the agents currently rendered as cards). Agents that disappear from the store (stopped) have their markers removed with a brief fade-out.
- **Marker visual:** a circle filled with the agent's `project.color` (master PRD §3.3, `[r,g,b]`), a thin ring tinted by the zone accent, and a short truncated label (agent `name`). A small state dot or icon disambiguates state within a zone. Hover shows a tooltip (name · role@project · state · `context_pct`). Click opens that agent's chat panel (reuse the existing Phase 2 open-chat action).
- **Within-zone layout:** multiple agents in one zone are arranged on a deterministic packed grid inside the zone's rectangle (row-major, fixed marker spacing, wrapping; computed from marker count). Determinism matters so a marker doesn't jump seats on unrelated re-renders — seat assignment is sorted by `agent_id` within the zone. When a zone overflows its capacity, shrink marker size and tighten spacing down to a floor, then allow a subtle internal scroll/clip (§5).
- **Movement between zones (on `state_update`):** when an agent's `state` changes, recompute its target zone and seat, update the marker's `transform`, and let the CSS transition glide it across the map (§2.2).
- **Animation on message delivery (driven by `new_message` / `notification`):**
  - `new_message` for an agent → **pulse** that agent's marker. When the event payload identifies a sender and recipient (see §5 / §8 for the direction-derivation strategy), also draw a **travel dot** from sender to recipient and pulse both endpoints.
  - `notification` (`done` | `waiting_input` | `permission_required`) → a distinct **attention pulse** (different color per type) on the subject agent's marker. (The accompanying `state_update` handles the actual zone move; the notification only adds the transient flourish.)

### 3.4 Zone layout config file (`zones.json`)

**Location.** The map reads zone geometry from a user-paintable JSON file, resolved in this order (first hit wins):

1. `${AGENTDECK_HOME:-~/.agentdeck}/zones.json` — the user's paintable layout, served read-only to the UI.
2. Built-in default bundled with the UI (`ui/src/views/map/defaultZones.json`) — used when (1) is absent or fails to parse/validate.

To keep "no new REST endpoints" (§4), the UI obtains the file's contents through the **existing static-file serving** the Go server already does for the UI bundle: the server exposes the `~/.agentdeck/` JSON store as static read-only assets (the same mechanism the dashboard already uses to read store files in the browser). The map fetches `zones.json` once on view mount via a plain `GET` of that static path — **no new API route, no new handler**. If that fetch 404s or returns invalid JSON, the UI silently falls back to the bundled default. (If the deployment does not already expose store files statically, the UI ships with only the bundled default and the "paintable file" affordance is documented as editing the bundled default — but the preferred, zero-code-change path is the `~/.agentdeck/zones.json` static read.)

**Schema** (`zones.json`):

```jsonc
{
  "version": 1,
  "viewBox": { "width": 1000, "height": 600 },   // logical coordinate space; markers scale to fit
  "background": "#0e1116",                         // optional map backdrop color
  "zones": [
    {
      "id": "idle",                  // unique zone id, referenced by stateToZone
      "label": "Idle",               // shown as the zone caption
      "rect": { "x": 40, "y": 40, "w": 280, "h": 240 },  // logical units within viewBox
      "color": "#4b9fff",            // zone accent (caption + marker ring tint)
      "fallback": true               // optional; exactly one zone should set this. Default target for unmapped states.
    },
    {
      "id": "busy",
      "label": "Busy",
      "rect": { "x": 360, "y": 40, "w": 600, "h": 360 },
      "color": "#ffb020"
    },
    {
      "id": "waiting",
      "label": "Waiting for input",
      "rect": { "x": 40, "y": 320, "w": 280, "h": 240 },
      "color": "#c060ff"
    },
    {
      "id": "done",
      "label": "Done",
      "rect": { "x": 360, "y": 420, "w": 290, "h": 140 },
      "color": "#3ecf8e"
    },
    {
      "id": "error",
      "label": "Error",
      "rect": { "x": 670, "y": 420, "w": 290, "h": 140 },
      "color": "#ff5c5c"
    }
  ],
  "stateToZone": {                   // maps status.state → zone id. Edit freely to re-route states.
    "idle": "idle",
    "busy": "busy",
    "waiting_input": "waiting",
    "done": "done",
    "error": "error"
  }
}
```

**Validation rules (client-side, on load):**

- `version` must equal `1` (forward-compat guard; unknown version → fall back to default + console warn).
- Every `stateToZone` value must reference an existing `zone.id`; dangling references route to the `fallback` zone.
- Zone `rect`s should fit inside `viewBox` and ideally not overlap; overlap is *allowed* (painter's choice) but logged as a soft warning. Out-of-bounds rects are clamped to the viewBox.
- At most one zone may be `fallback: true`; if none is, the first zone in the array is treated as fallback.
- On any structural parse error, discard the file entirely and use the bundled default (do not partially apply).

"Painting" a new layout = editing `~/.agentdeck/zones.json` (move/resize rects, rename labels, recolor, re-route states) and reloading the map view. No rebuild, no code change.

---

## 4. Integration

### 4.1 Consuming the existing SSE stream

The map **does not open its own SSE connection.** It subscribes to the **existing Phase 2 global agent store / event bus** that the card grid already uses. Concretely:

- **Agent set + state:** read from the same store the card grid renders from. `state_update` already flows into that store; the map re-derives marker zones from store state on every update. Reusing the store (not a second `EventSource`) guarantees the map and the grid never diverge and avoids a second connection against the per-client SSE buffer.
- **Transient events (`new_message`, `notification`):** these need to trigger one-shot animations, which a normalized store may swallow (it stores latest state, not event occurrences). Subscribe to the existing event bus's transient-event channel: extend the Phase 2 SSE client to expose a lightweight **event emitter / subscribe hook** (e.g. `onEvent(type, handler)`) that fans out raw `new_message` and `notification` events to any listener. The map registers handlers on mount and unregisters on unmount. This is a **client-side** addition to the existing SSE client — not a new network stream and not a new server event. If Phase 2's client already surfaces raw events, just consume that.

### 4.2 Toggle between card grid and map

- Add a **view toggle** in the existing dashboard header: `Grid | Map`. The selected view persists to UI-local state (`localStorage`, key `agentdeck.dashboardView`) so it survives reload. **Do not** persist this to `layout.json` or any server file (that would touch backend data; out of scope).
- The map view mounts inside the existing dashboard shell, sharing the header, the agent store, and the SSE connection. Switching views does **not** tear down the SSE connection or refetch agents.
- The card grid remains the **default** view on first run.

### 4.3 No new REST endpoints

No routes are added. The only network reads the map performs are: (a) the shared SSE stream (already open), and (b) a one-time static `GET` of `zones.json` via the existing static-asset path (§3.4). Confirm via a network-tab check that switching to the map issues **zero new XHR/fetch calls to `/api/...`** beyond the static `zones.json` read (acceptance, §7).

---

## 5. Edge cases & error handling

- **Many markers / zone overflow.** When a zone holds more agents than its packed grid comfortably fits: progressively shrink marker radius and spacing to a floor; past the floor, clip to the zone rect and show a small `+N` overflow badge in the corner of the zone (clicking it could later expand — out of scope, just show the count). Never let markers spill outside their zone rect.
- **Rapid state transitions (flicker).** An agent toggling `busy ↔ idle` quickly would cause the marker to ping-pong across the map. **Debounce zone moves**: apply a per-marker move debounce of ~250ms — only commit a zone change after the state has held for the debounce window. This rides on top of the state manager's own file-write debounce (Phase 2 §3.1). The marker still updates its small state dot immediately (cheap), but the cross-map glide only fires after the state settles.
- **Animation pile-up under message storms.** Cap concurrent travel dots/pulses (e.g. max ~20 in flight); coalesce repeated `new_message` pulses on the same marker within a short window into one (restart, don't stack). Drop excess animations rather than queueing — these are ambient, lossy by design (mirrors the SSE bus's drop-oldest philosophy).
- **Message direction representation (sender → recipient).** Locked decision in §8. Summary: when sender and recipient can both be resolved to on-map markers, draw a directional travel dot from sender to recipient (a faint connecting line that the dot traverses), arriving with a pulse on the recipient. When direction cannot be resolved (see below), degrade to a plain pulse on whichever agent(s) the event names — never block on missing direction.
- **Unresolvable / off-map endpoints.** If a `new_message`/`notification` references an `agent_id` not currently on the map (stopped agent, archived, or sender/recipient unknown): skip the travel dot and pulse only the resolvable endpoint; if neither is on-map, drop the animation silently.
- **Missing / invalid `zones.json`.** Fall back to the bundled default layout; log one console warning. The map must always render.
- **Project color missing/odd.** If `project.color` is absent or malformed, fall back to the zone accent color for the marker fill.
- **Empty state.** No live agents → render the zones with a centered "No active agents" caption (consistent with the grid's empty state).
- **Reduced motion.** Honor `prefers-reduced-motion` (§2.2): snap moves, flash instead of travel/pulse.
- **Resize.** Because geometry is in logical viewBox units with `preserveAspectRatio`, resizing the window needs no recompute — SVG scales. Within-zone seat layout is in logical units too, so it's resize-stable.

---

## 6. Implementation task breakdown (ordered)

1. **View scaffolding + toggle.** Add `MapView` component and a `Grid | Map` toggle in the dashboard header; persist selection to `localStorage`; default to Grid. Map renders an empty SVG (`viewBox 0 0 1000 600`) for now. *(No store wiring yet.)*
2. **Zone config load + validate.** Implement `zones.json` fetch from the static path with fallback to bundled `defaultZones.json`; implement the §3.4 validation rules; expose a typed `ZoneConfig`. Render the zones (rects + labels + accent) from config.
3. **Static markers from the store.** Subscribe `MapView` to the existing agent store; render one marker per live agent placed in its `stateToZone` zone using the deterministic packed-grid seat layout (sorted by `agent_id`). No animation yet — markers just appear in the right zone and re-place on store change.
4. **Marker visuals + interaction.** Add project-color fill, zone-tinted ring, truncated name label, state dot, hover tooltip, click-to-open-chat (reuse existing action), and accessibility attributes.
5. **Zone-move animation + debounce.** Add the CSS transform transition for cross-zone glide; add the ~250ms per-marker move debounce; handle marker add (fade-in) / remove (fade-out).
6. **Transient event subscription.** Extend the SSE client with an `onEvent` subscribe hook (if not already present); register `new_message` / `notification` handlers in `MapView`.
7. **Pulse + attention animations.** Implement Web Animations API pulse on the involved marker for `new_message`; implement the per-type attention pulse for `notification`.
8. **Direction travel dot.** Resolve sender/recipient → markers; animate a travel dot along the connecting line; handle unresolvable endpoints by degrading to a pulse.
9. **Edge-case hardening.** Zone overflow shrink + `+N` badge; animation pile-up caps + coalescing; `prefers-reduced-motion` branch; empty state; malformed color/config fallbacks.
10. **Tests + acceptance pass.** Per §7; verify zero new `/api/...` calls and that editing `zones.json` re-lays-out the map without a rebuild.

---

## 7. Testing strategy

Test against the four PRD acceptance criteria plus the locked decisions.

**Unit (Vitest + React Testing Library):**

- `stateToZone` mapping: each of the five states resolves to the configured zone; unmapped state → fallback zone; dangling reference → fallback.
- `zones.json` validation: valid config applied; `version != 1`, malformed JSON, missing fallback, out-of-bounds rect each handled per §3.4 (default or clamp).
- Seat layout: deterministic for a given agent set (same input → same seats); markers stay within zone rect; overflow path shrinks then clips + shows `+N`.
- Direction resolution: sender+recipient on map → travel-dot params produced; one endpoint off-map → pulse-only; both off-map → no animation.

**Component / integration (RTL + mocked store + mocked event bus):**

- *Markers reflect live state:* push a `state_update` (busy→idle) into the mocked store; assert the marker's target zone/transform updates (after debounce). (PRD AC: "markers reflect live state and move between zones.")
- *Animate on message:* emit a mocked `new_message`; assert the involved marker's animation is invoked (spy on `element.animate`) and a travel dot element appears for a resolvable sender→recipient pair. (PRD AC: "a marker animates on message delivery.")
- *Zone config drives layout without code changes:* render with config A, then re-render with a config B that moves/renames zones and re-routes a state; assert zones and marker placements change accordingly — no component code changed, only the input config. (PRD AC: "editing the zone config file changes the map layout without code changes.")
- *Toggle:* switching Grid↔Map keeps the shared store/SSE mounted; selection persists across remount via `localStorage`.

**No-new-server-data verification (the load-bearing constraint):**

- Spy on `fetch`/`EventSource` during a Grid→Map switch; assert **no new `/api/...` requests** and **no second `EventSource`** are created — only the one-time static `zones.json` read. (PRD AC: "the view is purely additive and introduces no new server data.")
- Static check / review gate: the Phase 7 diff touches **only `ui/`**; it adds no Go handler, no new SSE event type, no new file written under `~/.agentdeck/`.

**Manual / E2E smoke:**

- Run 3+ real agents; open Map; watch markers glide between zones as they work; trigger an agent-to-agent message (Phase 5) and confirm the travel dot + pulse; hand-edit `~/.agentdeck/zones.json` (move a zone, re-route `done`→`busy`), reload, confirm new layout with no rebuild; toggle `prefers-reduced-motion` and confirm motion degrades gracefully.

---

## 8. Resolved decisions

These answer the phase PRD's open questions (§5 of the PRD) with concrete, implement-now choices.

1. **Rendering tech → SVG.** For the expected ≤~50 markers, SVG wins on React integration, free DOM affordances (hover/click/a11y), and zero-cost CSS/WAAPI animation. Canvas/WebGL is rejected; it only pays off in the hundreds-plus-sprite regime this product never reaches. (Full rationale §2.1.)

2. **State → zone handling → all five states get their own default zone, but fully data-driven.** Default layout has distinct `idle`, `busy`, `waiting`, `done`, `error` zones. The mapping lives in `zones.json` `stateToZone`, so a painter can collapse states (e.g. `waiting_input`/`done`/`error` → `idle`) without touching code. Unmapped/dangling states route to the `fallback` zone. (§3.2.)

3. **Message-direction representation → directional travel dot + endpoint pulses, with graceful degradation.** On a `new_message` with a resolvable sender and recipient, animate a small dot traveling along a faint line from the **sender marker to the recipient marker**, arriving with a pulse on the recipient (and a fainter pulse on the sender at launch). Direction is conveyed by motion (the dot's travel) and by the arrival pulse landing on the recipient — no arrowheads needed at this marker size. When direction can't be resolved (only one party named, or an endpoint is off-map/stopped), degrade to a single pulse on the known marker; if neither party is on-map, drop the animation. Sender/recipient are derived from the existing event payloads: `new_message` is keyed by an `agent_id` (the affected agent), and the messaging model (Phase 5: messages addressed `to` a recipient, stored as message rows in `state.db`) supplies the counterpart; where the raw event lacks an explicit pair, treat the keyed agent as the subject and pulse-only. **No server change is required** — this consumes only the existing `new_message`/`notification` payloads. (§3.3, §5.)

4. **View toggle persistence → `localStorage` only.** The Grid/Map choice is UI-local state; it is deliberately **not** written to `layout.json` or any `~/.agentdeck/` file, keeping the phase free of new server data. (§4.2.)

5. **Zone config location → `~/.agentdeck/zones.json`, read via existing static-asset serving, with a bundled default fallback.** No new REST route; the file is hand-paintable; absent/invalid → bundled default. (§3.4.)
