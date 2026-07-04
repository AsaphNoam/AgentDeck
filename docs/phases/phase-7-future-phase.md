# Phase 7 — Future phase

**Status:** future / candidate-driven — define after the core product is solid
**Features:** candidate backlog, including F13 (activity map)
**Depends on:** Candidate-specific; most candidates build on Phases 2-6
**Enables:** —

---

## 1. Goal

Hold the post-core feature slot for the highest-value beta improvements once Phases 0-6 are stable. Phase 7 is no longer reserved only for polish; it is the place to choose the next product bet after the dashboard, archive, messaging, terminal runtime, switching, and groups exist.

Phase 7 was previously reserved for the optional activity map. Based on the current beta shape, the strongest next candidates are workflow features that help a single developer launch, supervise, and resume multiple coding agents with less overhead.

This file is intentionally product-level. Once a candidate is selected, turn it into a focused phase PRD or subphase plan with API shapes, storage decisions, UI details, and acceptance tests.

The original activity map remains a valid optional candidate, with its implementation notes in [`tech/phase-7-polish-activity-map-techspec.md`](tech/phase-7-polish-activity-map-techspec.md).

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

## 4. Candidate ranking

| Rank | Candidate | Effort | Value | Why now |
|------|-----------|--------|-------|---------|
| 1 | Dashboard triage filters + attention counters | Small | High | Cheap way to make the dashboard usable when several agents are active. |
| 2 | Command palette | Small-medium | Medium-high | Speeds common actions once the product has many screens and actions. |
| 3 | Session notes and operator annotations | Small-medium | High | Helps users recover context across many sessions without requiring summarization or new AI features. |
| 4 | Launch templates / task bundles | Medium | High | Cuts repeated setup and makes multi-agent workflows feel intentional rather than manual. |

---

## 5. Dashboard triage filters + attention counters

### Product problem

The card grid shows live state, but once several agents are running the user needs quick triage: which agents need input, which failed, which are still active, and which have unread messages. Notifications help at the moment of change; filters help when the user returns to the dashboard.

### Product shape

Add a small filter bar to the dashboard:

- `All`
- `Needs input`
- `Busy`
- `Done`
- `Error`
- `Unread`
- `Active`

Also add a browser title counter for attention-worthy work, such as `AgentDeck (2)`, where the count is agents in `waiting_input` plus errors plus unread messages.

### Likely implementation surface

- Mostly UI. Uses existing agent status, notification, and message badge state.
- Filter state should be local UI state, not server state.
- Optional persistence via `localStorage` if users expect the filter to survive reload.

### Acceptance sketch

- Filters update as live SSE state changes arrive.
- The browser title counter increments/decrements as agents enter/leave attention states.
- Filtering does not mutate layout order or group collapse state.

---

## 6. Command palette

### Product problem

AgentDeck is accumulating screens and actions: launch, open chat, resume, search archive, switch runtime, move groups, message agents, stop sessions, edit settings. A command palette gives frequent users one fast control surface without adding more visible chrome.

### Product shape

Add `Cmd+K` / `Ctrl+K` with searchable actions:

- Open agent by name, role, project, or group.
- Launch from template, if templates exist.
- Resume recent archive result.
- Switch runtime/model for a selected agent.
- Stop agent or release group.
- Message agent.
- Open settings sections.

### Likely implementation surface

- UI-only for the palette shell and search index.
- Actions call existing REST endpoints and navigation handlers.
- Start with deterministic local search over loaded agents, groups, settings routes, and recent archive entries. Do not add a new search backend for v1.

### Acceptance sketch

- `Cmd+K` opens the palette from any dashboard view.
- Searching an agent name allows opening that agent's chat.
- Command actions reuse existing permission/error handling.
- Palette can be closed with Escape and is keyboard navigable.

---

## 7. Session notes and operator annotations

### Product problem

Archive/search preserves what agents said and did, but it does not capture the human operator's memory: why the session exists, whether the result was trusted, what remains to verify, or why an agent was stopped. In multi-agent work, this becomes a real re-entry problem.

The beta does not need automatic summaries to solve this. A small human-authored annotation layer gives users a reliable place to record intent and next steps while keeping the product local-first and deterministic.

### Product shape

Each agent/session can have operator-authored metadata:

- A short note, visible on the card and archive detail.
- Optional status labels such as `needs review`, `blocked`, `verified`, `discarded`, or user-defined tags.
- Optional pinned "next action" text.
- Timestamps and author are unnecessary for v1 because AgentDeck is single-user/local.

This should feel like notes on a work item, not a separate document system. Keep it near the places where users make decisions: card details, chat header/sidebar, archive result/detail, and group view.

### Core user flows

1. **Add a note during live work**
   The user opens an agent, writes "Good approach, but verify migrations manually", and the card shows a small note indicator.

2. **Mark a session for later**
   The user tags a stopped session as `needs review`. It becomes findable/filterable in the archive and dashboard.

3. **Resume with context**
   The user opens an archived session weeks later and sees their own note before reading the transcript.

4. **Annotate a group outcome**
   If group-level notes are included, the user can mark a whole bundle/group as "auth migration attempt 2, reviewer found race in permission flow."

### Product decisions to make

- Session-only or group-level notes too. Recommendation: session notes first; group notes only if task bundles are selected in the same phase.
- Fixed labels or free-form tags. Recommendation: a few fixed labels plus free-form text; defer arbitrary tag management.
- Whether notes enter FTS search. Recommendation: yes. Notes are likely to contain the user's best retrieval phrases.
- Whether notes should be included in resume primer prompts. Recommendation: not by default. They are operator context first; adding them to agent prompts should be an explicit action later.

### Likely implementation surface

- SQLite state, because notes are machine/UI state attached to session identities and should be searchable with archive data.
- New fields or table keyed by `agent_id`: note text, label, updated timestamp.
- Archive search should include note text.
- UI: compact edit affordance in chat/session detail, note indicator on cards, note preview in archive rows.
- Optional filters: `has note`, `needs review`, `blocked`.

### Acceptance sketch

- A user can add/edit/clear a note on a live or archived session.
- Notes persist across dashboard restart and are included in archive search.
- Card/archive rows show a compact indicator without crowding the existing status UI.
- Existing session resume behavior is unchanged unless the user explicitly copies a note into a prompt.

---

## 8. Launch templates / task bundles

### Product problem

AgentDeck's core loop makes it possible to run many agents, but launching coordinated work is still too manual. A user who repeatedly does "implementer + reviewer", "researcher then implementer", or "three agents in one task group" has to choose the same role, project, backend, model, group, and starter prompt every time.

That friction matters because AgentDeck's central value is not one chat session; it is turning parallel agent work into an operator workflow. If setup takes too many clicks, users will fall back to one-off terminal tabs.

### Product shape

A launch template is a saved recipe for starting one agent. A task bundle is a saved recipe for starting several related agents together.

Examples:

- **Implement + review:** launch an implementer and reviewer in the same group. The implementer gets the main task prompt. The reviewer starts idle with a prompt like "Wait for implementation, then review the resulting diff."
- **Research then build:** launch a researcher and an implementer in one group. The researcher starts immediately; the implementer can either start with the same context or wait for a handoff message.
- **Cross-model check:** launch two agents with the same role/project/prompt on different backends or models for comparison.

The user-facing model should be simple:

- Save the current New Agent modal as a template.
- Create a bundle from multiple templates.
- Launch a bundle with one task description, optionally interpolated into each starter prompt.
- Put all launched agents into one group by default.
- Show the launched bundle as a group on the dashboard immediately.

### Core user flows

1. **Save template from launch modal**
   The user configures role/project/backend/model/interface/group/starter prompt and clicks "Save as template." The template appears in the launch modal next time.

2. **Launch from template**
   The user chooses a template, fills one task description, reviews the resolved prompt, and launches.

3. **Create bundle**
   The user selects two or more templates, gives the bundle a name, chooses whether agents start immediately or wait, and saves it.

4. **Launch bundle**
   The user chooses a bundle, enters the task, and AgentDeck launches all agents into a generated or selected group.

### Product decisions to make

- Whether templates are global or project-scoped. Recommendation: support global templates with optional project defaults, because roles like reviewer/implementer are reusable across projects.
- Whether starter prompts support variables. Recommendation: start with `{{task}}`, `{{project}}`, and `{{group}}` only.
- Whether bundles can express dependencies. Recommendation: keep v1 shallow. Use prompt wording and group/messaging; do not add a dependency graph unless that becomes the selected Phase 8 candidate.
- Whether bundle launch is all-or-nothing. Recommendation: best-effort with a clear result summary: launched N, failed M, with retry actions for failures.

### Likely implementation surface

- Config JSON under `~/.agentdeck/`, probably `templates.json` or `launch_templates.json`.
- New UI in the New Agent modal: template picker, save template, and bundle launcher.
- Server endpoint for template CRUD, or reuse the existing config handler pattern from roles/projects/backends.
- Bundle launch calls the existing `POST /api/sessions` path repeatedly; no new runtime behavior should be needed.
- Dashboard group support from Phase 6 should be reused as the primary visual container.

### Acceptance sketch

- A user can save a launch configuration as a template and use it after a dashboard reload.
- A user can launch a two-agent bundle into a shared group with one task description.
- Bundle launch failures are visible and do not hide successfully launched agents.
- Editing/deleting a template does not mutate existing agents.

---

## 9. Acceptance criteria

- [ ] The selected Phase 7 candidate has a focused PRD or subphase plan before implementation starts.
- [ ] Candidate scope names the user workflow, storage/API/UI changes, migration needs, and acceptance tests.
- [ ] If activity map is selected, use the existing activity-map tech spec as the implementation baseline.
- [ ] If a higher-value workflow candidate is selected, activity map remains optional/future and is not a dependency.


## 10. Open questions
- Which candidate should become the first post-core build target: launch templates/task bundles, session notes, dashboard triage/command palette, or activity map?
- Should Phase 7 remain a single candidate, or should it be split into 7A/7B if one small QoL item and one larger workflow item are both worth doing?
- For candidate features that add config/state, which data should be hand-editable JSON versus SQLite state?
