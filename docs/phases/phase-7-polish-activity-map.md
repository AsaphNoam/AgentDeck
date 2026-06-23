# Phase 7 — Polish: activity map & ambient visualization

**Status:** optional / lowest priority — ship after the core product is solid
**Features:** F13 (activity map)
**Depends on:** Phases 2 (state + SSE) and 5 (message events for animation)
**Enables:** —

---

## 1. Goal

Add an ambient-awareness layer: a 2D spatial map where each agent is a marker that moves between "idle" and "busy" zones and animates when it sends a message. This is purely a visualization over existing state — no new data, no new backend concepts. Explicitly safe to ship in a later milestone or skip.

---

## 2. Scope

### In scope
- A map view: each live agent rendered as a marker positioned by state.
- Markers move between idle/busy zones reflecting live `state_update`.
- Markers animate on message delivery (driven by `new_message`/`notification` events from Phase 5).
- Paintable zones defined in a config file.

### Out of scope
- Any new persisted data or REST endpoints — this is a pure consumer of the existing SSE event bus.
- Gameplay/interaction beyond ambient visualization.

---

## 3. Detailed requirements

- Consume the existing `/api/events` stream (`state_update`, `new_message`, `notification`).
- Map agent `state` → zone placement (`idle`/`busy`, with reasonable handling of `waiting_input`/`done`/`error`).
- Animate a marker when it sends/receives a message (transient effect on the relevant agent(s)).
- Zone layout read from a config file (paintable/editable); no hard-coded geometry.
- View is additive — does not replace the card grid; user can toggle between them.

---

## 4. Acceptance criteria

- [ ] Markers reflect live state and move between zones as agents transition busy/idle.
- [ ] A marker animates on message delivery.
- [ ] Editing the zone config file changes the map layout without code changes.
- [ ] The view is purely additive and introduces no new server data.

---

## 5. Open questions
- Rendering tech (SVG vs. canvas) for marker count expected.
- How to represent message direction (sender → recipient) in the animation.
