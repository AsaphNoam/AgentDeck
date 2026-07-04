# Phase 6 — Flexibility: terminal runtime, switch-runtime, task groups

**Status:** ready to build after Phases 1, 2, 4
**Features:** F7 (switch runtime on a live agent), F2 (task groups)
**Depends on:** Phases 1, 2, 4 (resume machinery)
**Enables:** Future-phase candidates

---

## 1. Goal

Pay off the stable-identity investment: let a user change a live agent's interface (chat ↔ terminal), backend (Claude ↔ Codex), or model without losing history, and organize many agents into collapsible task groups. This phase adds the second runtime (terminal) and the non-destructive switch flow that re-launches on the same `agent_id` and resumes.

---

## 2. Scope

### In scope
- Terminal runtime: cross-platform default — an embedded **xterm.js** terminal in the UI (or tmux); write prompts to the TTY, manage tab title/color/focus. iTerm2/AppleScript is an optional macOS-only extra behind a capability probe, not the core.
- Hooks: lifecycle scripts (`SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Stop`) that `POST /api/hook` with the per-launch token; gated by interface so chat agents skip redundant POSTs.
- Switch-runtime (F7): change interface/backend/model on a running agent, preserving conversation history (stop current runtime → re-launch with new params on stable `agent_id` → resume).
- Second backend wired end to end (Codex / `codex-acp`) for the backend-swap path.
- Task groups (F2): assign agents a `group` label; dashboard renders collapsible group sections; collapse/expand persists; "release group" stops all agents in it.

### Out of scope
- Activity map and other future-phase candidates.

---

## 3. Detailed requirements

### 3.1 Terminal runtime (master PRD §4.1)
- Implement the `Runtime` interface for `interface: "terminal"`; the registry dispatches to it.
- Default path: an embedded **xterm.js** terminal in the UI (or tmux) — cross-platform. Launch the same CLI with the same stable identity; manage tab title/color/focus.
- Write prompts to the TTY; `tty` recorded in the running row in `state.db`.
- Status derived from **hooks** (`POST /api/hook`) rather than the ACP stream.
- Optional iTerm2/AppleScript path behind a capability probe (macOS only); its absence degrades gracefully (that option disabled with a clear message). It is never the core.

### 3.2 Hooks (master PRD §4.4)
- Thin shell scripts registered with the agent CLI, firing on the lifecycle events above, that `POST /api/hook` with the per-launch token.
- Gate by interface: terminal agents rely on hooks; chat agents derive status from ACP and may skip redundant POSTs.
- Hook POSTs are applied to `state.db` by the server and flow to the UI via SSE, identically to runtime-produced status.

### 3.3 Switch-runtime (F7)
- Right-click card → Switch runtime; choose new interface and/or backend and/or model.
- Server stops the current runtime, re-launches with new params using the stable `agent_id`, and resumes the session (reusing Phase 4 resume).
- Conversation history is preserved across the switch; the same logical session continues.
- `POST /api/sessions/{id}/switch-runtime {interface?, backend?, model?}`.

### 3.4 Task groups (F2)
- Agents carry an optional `group` label (already in identity schema).
- Dashboard renders groups as collapsible sections; collapse/expand state persists (layout).
- "Release group" stops all agents in the group in one action.
- Card context menu "Move to group" (stubbed in Phase 2) now functional.

---

## 4. REST surface added

```
POST /api/sessions/{id}/switch-runtime    {interface?, backend?, model?}
POST /api/sessions/{id}/rename            {name}   (if not already added in Phase 2)
```

Group assignment is an identity edit (`group` field) via the existing session/identity update path; release-group is a batch stop.

---

## 5. Acceptance criteria

- [ ] Switching model mid-session keeps the full prior transcript and continues the same logical session.
- [ ] Switching interface chat → terminal (and back) preserves history on the same `agent_id`.
- [ ] Switching backend Claude → Codex preserves history and continues.
- [ ] A terminal-interface agent reports status via hooks and shows live badges in the dashboard.
- [ ] Creating two agents in the same group renders them under one collapsible header; collapse state persists.
- [ ] Releasing a group stops all of its agents.

---

## 6. Open questions (master PRD §9)
- Terminal default: embedded xterm.js vs tmux — which first?
- Backend swap semantics: does resuming a Claude session under Codex translate history cleanly? Define the history hand-off.
- Concurrency limits when many terminal sessions spawn.
