# Phase 7 — Future phase

**Status:** future / candidate-driven — define after the core product is solid
**Features:** candidate backlog, including F13 (activity map)
**Depends on:** Candidate-specific; most candidates build on Phases 2-6
**Enables:** —

---

## 1. Goal

Hold the post-core feature slot for the highest-value beta improvements once Phases 0-6 are stable. Phase 7 is no longer reserved only for polish; it is the place to choose the next product bet after the dashboard, archive, messaging, terminal runtime, switching, and groups exist.

The current candidate list lives in [`phase-7-feature-candidates.md`](phase-7-feature-candidates.md). The original activity map remains a valid optional candidate, with its implementation notes in [`tech/phase-7-polish-activity-map-techspec.md`](tech/phase-7-polish-activity-map-techspec.md).

---

## 2. Scope

### In scope
- Product-level candidate definition: what user problem each candidate solves, how it should behave, and what implementation surface it likely touches.
- Ranking candidates by early-beta value, implementation cost, and dependency on existing phase work.
- Promoting one candidate into an implementation-ready PRD/tech spec once selected.
- Keeping optional visual polish, such as the activity map, separate from higher-value operator workflow improvements.

### Out of scope
- Building multiple unrelated candidates in one phase.
- Reopening already-complete phase scope unless the selected candidate truly requires it.
- Cloud sync, remote access, multi-user collaboration, or auth.

---

## 3. Candidate selection criteria

- **Early-beta leverage:** Prefer features that reduce friction in normal daily use over presentation-only polish.
- **Reuse existing primitives:** Prefer candidates that compose existing agents, groups, archive, messages, notifications, layout, and config rather than creating new core abstractions.
- **Small reversible surface:** Prefer features that can ship incrementally and be removed or revised without destabilizing the runtime.
- **Operator clarity:** Favor features that help the human understand, launch, resume, or steer many agents.
- **Local-first fit:** Preserve the no-cloud/no-account model and keep data under `~/.agentdeck/`.

---

## 4. Acceptance criteria

- [ ] The selected Phase 7 candidate has a focused PRD or subphase plan before implementation starts.
- [ ] Candidate scope names the user workflow, storage/API/UI changes, migration needs, and acceptance tests.
- [ ] If activity map is selected, use the existing activity-map tech spec as the implementation baseline.
- [ ] If a higher-value workflow candidate is selected, activity map remains optional/future and is not a dependency.

---

## 5. Open questions
- Which candidate should become the first post-core build target: launch templates/task bundles, session notes, dashboard triage/command palette, or activity map?
- Should Phase 7 remain a single candidate, or should it be split into 7A/7B if one small QoL item and one larger workflow item are both worth doing?
- For candidate features that add config/state, which data should be hand-editable JSON versus SQLite state?
