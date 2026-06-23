# Phase 6 — Flexibility: terminal runtime, switch-runtime, task groups

**Status:** ready to build after Phases 1, 2, 4
**Features:** F7 (switch runtime on a live agent), F2 (task groups)
**Depends on:** Phases 1, 2, 4 (resume machinery)
**Enables:** Phase 7

---

## 1. Goal

Pay off the stable-identity investment: let a user change a live agent's interface (chat ↔ terminal), backend (Claude ↔ Codex), or model without losing history, and organize many agents into collapsible task groups. This phase adds the second runtime (terminal) and the non-destructive switch flow that re-launches on the same `agent_id` and resumes.

---

## 2. Scope

### In scope
- Terminal runtime: launch the CLI inside a real terminal emulator (iTerm2 on macOS via AppleScript), write prompts to the TTY, manage tab title/color/focus; status from hooks.
- Hooks: lifecycle scripts (`SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Stop`) writing `status/{id}.json` and `running/{id}.json`; gated by interface so chat agents skip redundant writes.
- Switch-runtime (F7): change interface/backend/model on a running agent, preserving conversation history (stop current runtime → re-launch with new params on stable `agent_id` → resume).
- Second backend wired end to end (Codex / `codex-acp`) for the backend-swap path.
- Task groups (F2): assign agents a `group` label; dashboard renders collapsible group sections; collapse/expand persists; "release group" stops all agents in it.

### Out of scope
- Activity map (Phase 7).
- Non-iTerm2 terminal fallback (open question; chat runtime remains the cross-platform default).

---

## 3. Detailed requirements

### 3.1 Terminal runtime (master PRD §4.1)
- Implement the `Runtime` interface for `interface: "terminal"`; the registry dispatches to it.
- Launch the same CLI with the same stable identity inside iTerm2 (AppleScript); manage tab title/color/focus.
- Write prompts to the TTY; `tty` recorded in `running/{id}.json`.
- Status derived from **hooks** rather than the ACP stream.
- macOS-only and optional; absence of iTerm2 degrades gracefully (terminal option disabled with a clear message).

### 3.2 Hooks (master PRD §4.4)
- Small shell scripts registered with the agent CLI, firing on the lifecycle events above, writing `status/` and `running/`.
- Gate by interface: terminal agents rely on hooks; chat agents derive status from ACP and may skip redundant hook writes.
- Hook writes flow through the Phase 2 state manager → SSE, identically to runtime-produced status.

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
- Cross-platform terminal fallback (tmux / OS terminal) — in scope later, or iTerm2-only?
- Backend swap semantics: does resuming a Claude session under Codex translate history cleanly? Define the history hand-off.
- Concurrency limits when many terminal tabs spawn.
