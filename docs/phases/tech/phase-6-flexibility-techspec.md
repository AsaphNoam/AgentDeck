# Phase 6 — Flexibility: Implementation Tech Spec

**Mirrors:** `docs/phases/phase-6-flexibility.md` (phase PRD)
**Features:** F7 (switch runtime on a live agent), F2 (task groups)
**Depends on:** Phase 1 (Runtime interface, chat runtime, launch composition), Phase 2 (state manager, SSE bus, card grid, layout), Phase 4 (resume machinery, transcript persistence)
**Enables:** Phase 7
**Audience:** the engineer implementing this phase. Prescriptive; no further design decisions required.

---

## 1. Overview & scope recap

Phase 6 cashes in the stable-`agent_id` investment. Three deliverables, plus one enabling backend integration:

1. **Terminal runtime** — a second `Runtime` implementation (`interface == "terminal"`) that runs the CLI inside iTerm2 on macOS via AppleScript, deriving status from **hooks** instead of the ACP stream.
2. **Hooks** — shell scripts registered with each agent CLI that write `status/{id}.json` and `running/{id}.json` on lifecycle events, gated by interface.
3. **Switch-runtime (F7)** — `POST /api/sessions/{id}/switch-runtime` that stops the current runtime, re-launches with new interface/backend/model on the **same** `agent_id`, and resumes (reusing Phase 4 `Runtime.Resume`) so the transcript and logical session continue.
4. **Codex backend (`codex-acp`)** wired end-to-end as the real second backend, so the backend-swap path in F7 is exercised against a true second provider.
5. **Task groups (F2)** — `group` label on identity, collapsible dashboard sections with persisted collapse state, functional "Move to group", and "release group" batch-stop.

### In scope
- `Runtime` impl for `terminal` (iTerm2/AppleScript), TTY prompt writing, tab title/color/focus, `tty` in `running/`.
- Hook script set + registration with both Claude Code and Codex CLIs.
- Switch-runtime endpoint + UI flow (chat↔terminal, Claude↔Codex, model swap).
- Codex (`codex-acp`) as a real second backend.
- Task groups: identity `group` field, collapsible sections, persisted collapse, Move to group, release group.
- `rename` endpoint if not already present from Phase 2.

### Out of scope
- **Activity map** (Phase 7).
- **Non-iTerm2 terminal fallback** (tmux / Terminal.app / Linux) — explicitly an open question. This phase is **iTerm2-on-macOS only**; everywhere else the terminal interface is disabled with a clear message (see §3.5, §12). The chat runtime remains the cross-platform default.

### Assumptions inherited from prior phases
- `Runtime` interface and chat runtime exist (Phase 1): `Start`, `SendPrompt`, `Cancel`, `Stop`, `Resume`, `CheckMessages`.
- A **runtime registry** dispatches by `agent.interface` (Phase 1); `terminal` previously returned "not implemented".
- State manager watches `running/` + `status/` and emits SSE `state_update` (Phase 2).
- `layout.json` persists card order + density via `GET/PUT /api/layout` (Phase 2). We extend it with group collapse state.
- `Runtime.Resume(agent, session_id)` is implemented for chat (Phase 4); transcript persists to `sessions/{agent_id}/` as an NDJSON append log.

---

## 2. Technology choices

### 2.1 iTerm2 control from Go
- **Mechanism:** AppleScript via `osascript`, shelled out from Go with `os/exec`. We do **not** add a CGo ScriptingBridge dependency; `osascript` keeps the binary pure-Go and matches the PRD ("iTerm2 on macOS via AppleScript").
- **Why AppleScript over the iTerm2 Python API:** no extra runtime dependency, no daemon, and the operations we need (create tab, set title/color/profile, write text, read the session's TTY, activate) are all first-class in iTerm2's AppleScript dictionary.
- **TTY discovery:** iTerm2 AppleScript exposes `tty` on a session object. After creating the tab we read it back and store it in `running/{id}.json`.
- **Writing prompts:** iTerm2 `write text "<prompt>"` types into the session as if at the keyboard (it appends a newline / submits). We use `write text` rather than raw TTY `write()` so the CLI's line editor receives a proper submitted line.
- **Scripts are templated** Go `text/template` strings rendered with the agent's params, then passed to `osascript -` over stdin (avoids shell-quoting hell; see §3.2 for escaping rules).

### 2.2 Hook script language and registration
- **Language:** POSIX `sh` (not bash-specific) shell scripts. They do one thing — assemble a small JSON object and write it atomically. `python3` is a stated prereq and is used **inside** the hook only for safe JSON encoding of arbitrary strings (`detail`, file paths); the control flow stays in `sh`. (Rationale: hooks receive untrusted tool arguments; hand-rolling JSON in shell is unsafe.)
- **Location:** installed to `~/.agentdeck/hooks/` by `install.sh` and on server startup (server writes/refreshes them so they always match the running binary's expectations). One script per event, plus a shared `_emit.sh` helper.
- **Registration with Claude Code:** Claude Code reads hooks from its settings JSON. At launch the chat/terminal runtime composes a `--settings` (or project `.claude/settings.json` injection) that maps each lifecycle event to the matching `~/.agentdeck/hooks/<event>.sh`. The agent's `agent_id` and `interface` are passed to hooks via **environment variables** set on the CLI process (`AGENTDECK_AGENT_ID`, `AGENTDECK_INTERFACE`, `AGENTDECK_HOME`), so a single generic script serves all agents.
- **Registration with Codex (`codex-acp`):** Codex's hook surface differs (see §6). Where Codex lacks a 1:1 hook for an event, the terminal runtime falls back to the events Codex *does* expose, and fills gaps from the ACP/notification channel. Hook scripts are identical; only the registration mapping differs per backend (kept in a per-backend `hookMap`).

### 2.3 Codex (`codex-acp`) integration
- `backends.json` already defines a `codex` backend of `type: "codex-acp"` with `default_model: "gpt-5.5"` and a backend-level `env` (`CODEX_HOME`).
- `codex-acp` speaks ACP over stdio like `claude-acp`, so it reuses the **chat runtime** transport; differences are confined to: launch argv, env (`CODEX_HOME`, `OPENAI_API_KEY`/`OPENAI_BASE_URL` per-model overrides), the resume/session-id mechanism, and the hook event names. These are captured in a per-backend **adapter** (§6) rather than branching the runtime.

---

## 3. Terminal runtime design

Package: `internal/runtime/terminal` implementing the same `Runtime` interface as chat. Registered in the registry under `interface == "terminal"`. Guarded by `runtime.GOOS == "darwin"` + iTerm2 presence (§3.5).

### 3.1 Lifecycle

```
Start(agent):
  1. Compose config exactly as chat runtime (cwd, context_prompt, system_prompt, backend/model, env).
  2. Build the CLI argv (resume form if a prior session_id exists for this agent_id — see §5).
  3. Render the iTerm2 "create tab + run command" AppleScript (§3.2) and run via osascript.
     - The command launched in the tab is: `cd <cwd> && AGENTDECK_AGENT_ID=<id> AGENTDECK_INTERFACE=terminal AGENTDECK_HOME=<home> <env...> <cli argv>`
     - Hooks are registered via the CLI's settings (env-pointed), same as chat.
  4. Read back the new session's `tty` and the iTerm2 window/tab/session identifiers; store them.
  5. Write running/{id}.json: {agent_id, pid, session_id:"", interface:"terminal", tty, started_at, iterm:{window_id,tab_id,session_id}}.
     - pid: the process group of the launched CLI. Obtain it from iTerm2 (`job pid of session`) once the CLI is foregrounded; if unavailable immediately, poll the tty's foreground pgid.
  6. Write initial status/{id}.json {state:"idle"} ONLY if no hook has written one within 500ms (avoid clobbering SessionStart hook).
```

`SendPrompt(agent, text)`: render the "write text" AppleScript targeting the stored iTerm2 session id and run it. Text is escaped per §3.2. No ACP round-trip; the response renders in the terminal, and **status flows through hooks**.

`Cancel(agent)`: send an interrupt to the tab. Implementation: iTerm2 `write text` of the control sequence is unreliable, so send `SIGINT` to the foreground process group of the recorded `tty` (`syscall.Kill(-pgid, SIGINT)`), matching how the chat runtime cancels.

`Stop(agent)`: `SIGTERM` then (after grace) `SIGKILL` to the process group; then close the iTerm2 tab via AppleScript (best-effort); delete `running/{id}.json`. Leave `sessions/` and `status/` intact (status will be set to `done`/`idle` by the Stop hook or by the runtime if the hook didn't fire).

`Resume(agent, session_id)`: identical to `Start` but the argv uses the backend's resume form (§5/§6) with `session_id`. Used by switch-runtime and by Phase 4 archive resume when the target interface is terminal.

`CheckMessages(pid)`: nudger support (Phase 5) — write a `check_messages` prompt to the TTY via the same `write text` path. (Same contract as chat; just a different delivery mechanism.)

### 3.2 AppleScript templates & escaping

Three templates, rendered with Go `text/template`, piped to `osascript -`:

**create-tab.applescript** (returns `tty\twindow_id\ttab_id\tsession_id` on stdout):
```applescript
tell application "iTerm2"
  activate
  set newWindow to (create window with default profile)
  tell current session of newWindow
    set name to "{{.TabTitle}}"
    write text "{{.LaunchCommand}}"
    set ttyVal to tty
  end tell
  set wid to id of newWindow
  return ttyVal & tab & wid
end tell
```
(Tab vs window: create a **window** per agent by default; grouping windows into one window's tabs is a Phase 7 nicety. We still call them "tabs" loosely.)

**set-appearance.applescript** — sets `name` (title) and `background color` from `project.color` (mapped from `[r,g,b]` 0–255 to iTerm2's 0–65535 scale).

**write-text.applescript** — `tell session id "<sid>" of ... to write text "<escaped prompt>"`.

**Escaping rules (mandatory):**
- All dynamic strings are escaped for AppleScript string literals: `\` → `\\`, `"` → `\"`, and newlines converted to explicit `" & return & "` segments (do **not** embed raw newlines in `write text` — split into multiple `write text` calls or join with `return`).
- The `LaunchCommand` is a shell command; build it as an argv joined with proper shell quoting (use a small Go shell-quote helper, never `fmt.Sprintf` interpolation of user strings), then AppleScript-escape the whole string.
- Never interpolate user content into the AppleScript template without going through both quoting layers.

### 3.3 Tab title / color / focus
- **Title:** `"{name} · {role}@{project}"`. Re-set on rename and on switch-runtime.
- **Color:** `background color` from `project.color`. Optionally tint the **tab/badge** by state later; v1 sets the project accent only.
- **Focus:** `Start` activates the new window; subsequent `SendPrompt` does **not** steal focus (no `activate`) unless the call is user-initiated from the UI "reveal in terminal" action (future). A `RevealTab(agent)` helper exists for the UI to bring a tab forward on demand.

### 3.4 Status derivation (hooks only)
The terminal runtime writes **no** `status/` updates during a turn. All state (`busy`/`idle`/`waiting_input`/`done`/`error`, `detail`, `last_trace`, `busy_since`, `context_pct`) comes from the hook scripts (§4). The runtime only writes the **initial** idle status at `Start` if the SessionStart hook hasn't beaten it (race guard, §3.1 step 6), and a terminal `done`/`idle` on `Stop` if the Stop hook didn't fire (e.g. hard kill).

### 3.5 macOS-only graceful degradation
Detection helper `terminal.Available() (bool, reason string)`:
- `runtime.GOOS != "darwin"` → `(false, "Terminal runtime requires macOS")`.
- iTerm2 not installed — probe via `osascript -e 'id of application "iTerm2"'` (or check `mdfind`/`/Applications/iTerm.app`); non-zero/empty → `(false, "iTerm2 is not installed")`.
Behavior when unavailable:
- `POST /api/sessions` with `interface:"terminal"` and any switch-runtime to `terminal` → **422** `{error:"terminal_unavailable", reason}`.
- The New-Agent modal and the Switch-runtime dialog **disable** the terminal option and show `reason` as a tooltip. The server exposes availability at `GET /api/capabilities` → `{terminal:{available:bool, reason}}` so the UI doesn't guess.

---

## 4. Hooks design

### 4.1 Files (`~/.agentdeck/hooks/`)
- `_emit.sh` — shared helper. Reads `AGENTDECK_AGENT_ID`, `AGENTDECK_HOME`, `AGENTDECK_INTERFACE`, plus event-specific args, and **atomically** writes JSON via temp-file-rename.
- `session-start.sh`, `user-prompt-submit.sh`, `pre-tool-use.sh`, `post-tool-use.sh`, `stop.sh` — thin wrappers that map the CLI's event payload to `_emit.sh` arguments.

`_emit.sh` core (illustrative; real version uses `python3 -c` for JSON encoding of `detail`):
```sh
#!/bin/sh
# usage: _emit.sh STATUS_FIELD=value ...   (writes status/$AGENTDECK_AGENT_ID.json)
home="${AGENTDECK_HOME:-$HOME/.agentdeck}"
id="$AGENTDECK_AGENT_ID"
[ -n "$id" ] || exit 0
out="$home/status/$id.json"
tmp="$(mktemp "$home/status/.$id.XXXXXX")"
python3 - "$@" >"$tmp" <<'PY'
import json,sys,os,datetime
kv=dict(a.split("=",1) for a in sys.argv[1:] if "=" in a)
kv["agent_id"]=os.environ["AGENTDECK_AGENT_ID"]
print(json.dumps(kv))
PY
mv -f "$tmp" "$out"
```

### 4.2 Event → write mapping

| Event | status.state | status fields written | running/ touch |
|-------|--------------|------------------------|----------------|
| SessionStart | `idle` | `detail:"session started"`, `last_trace:"SessionStart"`, set `busy_since` empty | write/refresh `running/{id}.json` (pid, session_id if CLI exposes it, tty, started_at) |
| UserPromptSubmit | `busy` | `detail:"thinking"`, `busy_since:<now>`, `last_trace:"UserPromptSubmit"`, `context_pct` if available | — |
| PreToolUse | `busy` | `detail:"<tool>: <short args>"`, `last_trace:"PreToolUse: <tool>"` | — |
| PostToolUse | `busy` | `detail:"<tool> done"`, `last_trace:"PostToolUse: <tool>"`; if the tool was a permission/ask → `waiting_input` | — |
| Stop | `idle` or `done` | `detail:"turn complete"`, `last_trace:"Stop"`, clear `busy_since` | on session end (CLI exit): delete `running/{id}.json` |

`waiting_input`: when the CLI emits a permission/approval prompt event (Claude Code surfaces this; mapping noted per-backend), the relevant hook writes `state:"waiting_input"`. Terminal agents thus show the same `waiting_input` badge as chat agents.

`context_pct`: written from whatever the CLI exposes in the hook payload (Claude Code provides context usage; if absent, omit the field — never write a stale value).

### 4.3 Interface gating
- Hooks always read `AGENTDECK_INTERFACE`. For `interface == "chat"`, status is authoritatively produced by the chat runtime from the ACP stream, so the hook scripts **skip the `status/` write** for the redundant events (SessionStart/UserPromptSubmit/Pre/PostToolUse/Stop) and write **only** `running/` housekeeping that the runtime doesn't already own. Concretely: `_emit.sh` exits early (`exit 0`) when `AGENTDECK_INTERFACE=chat` **and** the event is one the chat runtime already covers. This prevents two writers racing on the same `status/` file.
- For `interface == "terminal"`, hooks are the sole status writer; they always write.
- The gate is a single guard at the top of each event script, so the same scripts ship for all agents.

### 4.4 Flow through Phase 2 state manager (identical path)
Hook writes land in `status/{id}.json` / `running/{id}.json` via atomic rename. The Phase 2 fsnotify watcher fires on the rename (watch for `CREATE`/`RENAME` in addition to `WRITE`, since atomic writes appear as rename), recomputes the merged agent state, and emits SSE `state_update`. **No new server code path** — hook-produced status is indistinguishable from runtime-produced status downstream. The only watcher change required: ensure rename/create events are handled (Phase 2 may have only watched WRITE); confirm and fix if needed.

---

## 5. Switch-runtime design (F7)

`POST /api/sessions/{id}/switch-runtime {interface?, backend?, model?}` — any subset of the three may change; at least one must differ from current, else **400**.

### 5.1 Core algorithm (interface and/or model swap, same backend)
```
switchRuntime(id, {interface?, backend?, model?}):
  agent  = readIdentity(id)                  // stable agent_id
  prev   = readRunning(id)                    // current pid, session_id, interface, tty
  target = merge(agent, requested fields)     // new interface/backend/model

  validate(target)                            // §3.5 terminal availability; backend/model exist
  acquire per-agent switch lock (§9)

  1. Flush + checkpoint: ensure transcript persistence (Phase 4) has flushed all events
     for the current ephemeral session to sessions/{agent_id}/. Record current session_id.
  2. Cancel any in-flight turn (Runtime.Cancel) and wait for turn_end or timeout (§9 mid-turn).
  3. oldRuntime = registry.For(prev.interface)
     oldRuntime.Stop(agent)                   // terminates process group, removes running/, keeps sessions/
  4. Persist new identity fields: write agents/{id}.json with updated interface/backend/model.
     (agent_id UNCHANGED.)
  5. newRuntime = registry.For(target.interface)
  6. resumeSessionId = resolveResumeId(agent, prev, target)   // §5.3 backend hand-off
     newRuntime.Resume(agent, resumeSessionId)                // reuses Phase 4 resume:
        - re-composes config from updated identity
        - restores transcript view from sessions/{agent_id}/
        - launches CLI in resume mode, writes fresh running/{id}.json (new ephemeral session_id)
  7. release lock; emit state_update (identity changed → card re-renders backend/model/interface badges)
  return 200 with updated identity + running summary.
```
The **same logical session** continues because `agent_id` never changes and `sessions/{agent_id}/` is the durable transcript the resume restores from. The ephemeral `session_id` changes (expected).

### 5.2 chat ↔ terminal
- **chat → terminal:** Stop chat runtime; Resume under terminal runtime. The terminal CLI is launched in resume mode so the user sees prior context in the terminal where the backend supports `--resume <id>` (Claude Code does; Codex via `CODEX_HOME` session, §6). Status switches from ACP-derived to hook-derived automatically (the hooks were always registered; the gate now lets them write). The chat panel for this agent becomes read-only (shows persisted transcript) and the card gains a "terminal" badge / "reveal tab" affordance.
- **terminal → chat:** Stop terminal runtime (close tab); Resume under chat runtime; chat panel goes live again. Hook status writes for this agent now self-suppress (gate sees `interface=chat`).

### 5.3 backend swap (Claude ↔ Codex) — history hand-off (concrete)
The two backends have **incompatible native session formats**. We do **not** translate one CLI's session store into the other's. The hand-off is defined as:

1. **AgentDeck transcript is the source of truth**, not the CLI's session file. `sessions/{agent_id}/` holds normalized events (Phase 4 NDJSON).
2. On a backend swap, `resolveResumeId` returns **empty** (there is no compatible native session id to resume in the new backend). The new backend starts a **fresh native session** (`session_id` new).
3. To preserve continuity, switch-runtime injects a **history primer** into the new backend's launch composition: a system/context message synthesized from the AgentDeck transcript — a bounded, summarized rendering of prior turns (assistant text + tool outcomes + final state), appended to `role.system_prompt` for this launch only (not persisted to the role). Budget the primer to a token cap (default 8k tokens; configurable `config.json: switch.primer_token_budget`); when the transcript exceeds the cap, prime with (a) a generated running summary of older turns plus (b) the last N verbatim turns (default N=6). Summary generation uses the **target** backend's default model via a one-shot non-interactive call before the interactive launch, or a cheap local heuristic truncation if that call fails (degrade, don't block).
4. The AgentDeck transcript continues to append to the **same** `sessions/{agent_id}/` log under the new backend — so from the user's and the archive's perspective it is one continuous session. A transcript marker event `{type:"backend_switch", from, to, at}` is appended so the chat panel can render a divider.

Same-backend model swap (Claude sonnet → opus, or Codex gpt-5.5 → gpt-4o) uses the **native resume** path (`resolveResumeId` returns the existing native `session_id`); no primer needed, because the CLI keeps its own session and only the model arg changes. (Claude Code supports model switch on resume; if a given backend cannot change model on resume, fall back to the primer path — recorded per-backend in the adapter.)

### 5.4 Concurrency & atomicity
- Per-agent **switch lock** (in-memory mutex keyed by `agent_id`) serializes switch/stop/prompt for that agent.
- If `Resume` (step 6) fails after `Stop` (step 3), **roll back**: re-launch the previous interface/backend/model via `Resume` with the previous resume id; restore identity file; return **500** with `{error:"switch_failed_rolled_back", detail}`. If rollback also fails, leave identity at target, write `status:"error"`, and return **500** `{error:"switch_failed", detail}` (the agent is recoverable via archive resume).

---

## 6. Codex backend wiring

Goal: make `codex-acp` a real second backend so the F7 backend-swap path is genuinely exercised.

### 6.1 Launch & env
- argv: `codex-acp` launcher with model and cwd flags (per the `codex-acp` CLI contract). Compose cwd from `project.cwd`, system prompt append from `role.system_prompt` + `project.context_prompt`.
- env precedence: process env ← backend-level `backends.codex.env` (e.g. `CODEX_HOME`) ← per-model `env` (`OPENAI_API_KEY`, `OPENAI_BASE_URL`) — per-model overrides backend-level (matches PRD §3.4).
- `CODEX_HOME` points Codex's session store at an AgentDeck-controlled directory so sessions are discoverable for native resume.

### 6.2 Model config
- Default `gpt-5.5`; `gpt-4o` carries a per-model `env` with an alternate `OPENAI_API_KEY`/`OPENAI_BASE_URL`. The launch composer must apply per-model env (already required by PRD §3.4 / Phase 1 composition) — verify Phase 1 implemented per-model env, not just backend-level; if missing, add it here (it's a prerequisite for Codex's `gpt-4o`).

### 6.3 ACP differences vs Claude (captured in a per-backend adapter)
Introduce `internal/backend/adapter.go` with a `BackendAdapter` interface; one impl per `type` (`claude-acp`, `codex-acp`). The **chat runtime stays generic**; the adapter encapsulates:
- **Launch argv builder** (flags differ).
- **Resume mechanism:** Claude `--resume <session_id>`; Codex resumes via `CODEX_HOME` + its own session id form. `resolveResumeId` consults the adapter.
- **Event normalization quirks:** any differences in ACP tool-call / diff / permission-request shapes between the two CLIs are normalized to AgentDeck's internal transcript event types here. Pin tested CLI versions (carry over Phase 1's pinning note; add the Codex version).
- **Hook event-name map:** Codex may name lifecycle events differently or expose fewer; the adapter provides `hookMap` (AgentDeck event → CLI hook key) and lists events Codex cannot emit, so the terminal runtime knows which states to backfill from ACP/notifications.
- **Model-on-resume capability flag:** `CanSwitchModelOnResume bool` — drives §5.3's native-resume vs primer decision for model swaps.
- **Permission/`waiting_input` signal:** how each CLI surfaces an approval prompt, mapped to `waiting_input`.

Acceptance for this section: launch a Codex agent end-to-end (chat interface), send a prompt, stream a response, stop, resume — all green — before wiring the swap path.

---

## 7. Task groups design (F2)

### 7.1 Data
- `group` is already on `agents/{id}.json` (`"group": "auth-migration"`, optional/empty). No schema change.
- **Collapse state** is UI/layout, not identity → stored in `layout.json`:
```jsonc
// layout.json (extended)
{
  "order": [...], "density": {...},
  "groups": { "auth-migration": { "collapsed": true }, "_ungrouped": { "collapsed": false } }
}
```
Persisted via the existing `GET/PUT /api/layout`. Unknown groups default to expanded.

### 7.2 Dashboard rendering
- The card grid groups cards by `agent.group`; agents with empty `group` render under a synthetic `_ungrouped` section (rendered last, header optionally hidden if it's the only section).
- Each group is a **collapsible section**: header shows group name, agent count, an aggregate state summary (e.g. "2 busy · 1 idle"), a collapse chevron, and a "Release group" action.
- Collapse toggles update `layout.groups[name].collapsed` and PUT `/api/layout` (debounced). State survives reload (acceptance criterion).
- Drag-reorder within a group reorders; dragging a card onto another group's header is an alternate **Move to group** gesture (see §7.3).

### 7.3 Move to group (Phase 2 stub → functional)
- Card context-menu "Move to group" opens a small picker (existing groups + "New group…" + "Remove from group").
- Selecting a target issues an **identity edit**: `POST /api/sessions/{id}/identity {group}` (see §8) which writes `agents/{id}.json`. The state manager sees the `agents/` change… **but Phase 2 only watches `running/` + `status/`**. Two options; choose **(a)**:
  - **(a, chosen):** the identity-edit endpoint, after writing the file, emits an SSE `state_update` for that agent directly (the endpoint already has the merged state in hand). No new watcher. Simpler and avoids watching `agents/` (which churns on every edit).
  - (b) extend the watcher to `agents/` — rejected as unnecessary surface.
- Card animates into the target group section on the next `state_update`.

### 7.4 Release group (batch stop)
- `POST /api/groups/{group}/release` → server enumerates agents with `agent.group == group`, calls `registry.For(interface).Stop(agent)` for each (concurrently, bounded — §9), returns a per-agent result list. Stopping leaves transcripts intact (archive still has them). The group section disappears when it has no running agents (its `layout.groups` entry is harmless to keep; prune lazily).

---

## 8. API contracts

All under `http://127.0.0.1:{port}/api`. JSON request/response. Errors: `{ "error": "<code>", "detail"?: "<human msg>", ... }`.

### 8.1 `POST /api/sessions/{id}/switch-runtime`
Request (any subset; ≥1 field must differ from current):
```json
{ "interface": "terminal", "backend": "codex", "model": "gpt-5.5" }
```
Responses:
- `200` →
```json
{
  "agent_id": "a_8f3c12",
  "interface": "terminal", "backend": "codex", "model": "gpt-5.5",
  "running": { "pid": 49001, "session_id": "codex-sess-abc", "tty": "/dev/ttys007", "started_at": "..." },
  "history_handoff": "primer"   // "native_resume" | "primer"
}
```
- `400 {error:"no_change"}` — request equals current state.
- `400 {error:"invalid_field", detail}` — unknown interface/backend/model.
- `404 {error:"agent_not_found"}`.
- `409 {error:"switch_in_progress"}` — switch lock held.
- `422 {error:"terminal_unavailable", reason}` — target interface terminal on a host without iTerm2.
- `500 {error:"switch_failed_rolled_back", detail}` / `500 {error:"switch_failed", detail}` — see §5.4.

### 8.2 `POST /api/sessions/{id}/rename` (add if absent)
Request `{ "name": "Atlas" }` → writes `agents/{id}.json name`, retitles terminal tab if terminal, emits `state_update`.
- `200 {agent_id, name}` · `400 {error:"empty_name"}` · `404 {error:"agent_not_found"}`.

### 8.3 `POST /api/sessions/{id}/identity` (group + other identity edits)
Request (partial; v1 supports `group` and `name`):
```json
{ "group": "auth-migration" }
```
`group:""` or omitted-with-explicit-null removes from group.
- `200 {agent_id, group, name, role, project, backend, model, interface}` (full identity echo) · `404 {error:"agent_not_found"}` · `400 {error:"invalid_group_name"}` (e.g. reserved `_ungrouped`).
- Side effect: emit `state_update` for the agent (§7.3a).

### 8.4 `POST /api/groups/{group}/release`
Request: empty body.
Response `200`:
```json
{ "group": "auth-migration", "stopped": [
    {"agent_id":"a_1","ok":true},
    {"agent_id":"a_2","ok":false,"error":"stop_timeout"} ] }
```
- `404 {error:"group_not_found"}` — no agents currently carry this group (still return 200 with empty `stopped`? **No** — 404 to distinguish typos). Releasing an empty group → `404`.
- Partial failures are reported per-agent with overall `200` (the operation ran); the client surfaces which agents failed.

### 8.5 `GET /api/capabilities`
Response `200`:
```json
{ "terminal": { "available": false, "reason": "iTerm2 is not installed" } }
```
Consumed by the New-Agent modal and Switch-runtime dialog to disable the terminal option.

---

## 9. Concurrency, edge cases & error handling

- **Switch mid-turn:** switch-runtime first calls `Cancel` and waits up to `config.switch.cancel_timeout_ms` (default 5000) for `turn_end`. On timeout, proceed to `Stop` anyway (the turn's streamed events are already persisted; the in-flight tool result may be lost — acceptable, recorded as a transcript `{type:"turn_interrupted"}` marker). Never switch while a turn is mid-stream without canceling first.
- **iTerm2 missing / disappears:** `Start`/`Resume` for terminal probe `terminal.Available()` first → `422`. If iTerm2 is force-quit while a terminal agent runs, the next hook/`running` read will find the process gone; the state manager marks the agent stopped (running file removed by Stop hook on CLI exit, or reaped by a periodic liveness sweep that checks pgid existence and prunes stale `running/` entries).
- **Backend swap with incompatible history:** always handled by the primer path (§5.3) — never attempt cross-backend native resume. If primer summary generation fails, fall back to truncated verbatim tail; if even the launch fails, roll back (§5.4).
- **Many terminal tabs:** enforce a soft cap `config.terminal.max_tabs` (default 12). Launch/switch-to-terminal beyond the cap returns `429 {error:"terminal_tab_limit", limit}`; the UI surfaces it. Release-group and stop free slots. (Chat agents are uncapped here; a global agent concurrency limit remains a master-PRD open question, not resolved in this phase.)
- **Release-group concurrency:** stop agents with a bounded worker pool (default 4 concurrent stops) to avoid AppleScript/osascript contention when closing many iTerm2 tabs.
- **Double-writer race on `status/`:** prevented by the interface gate (§4.3) — exactly one writer (runtime for chat, hooks for terminal) per agent at a time. During the switch window the lock (§5.4) ensures the old writer is fully stopped before the new one starts.
- **osascript failures:** wrap every `osascript` call with a timeout (default 4s) and capture stderr; map failures to actionable errors (`terminal_unavailable`, `iterm_script_failed`). Never hang the request on a stuck AppleScript.
- **Atomic file writes everywhere:** hooks and runtime write via temp+rename so the watcher never reads a partial JSON.

---

## 10. Implementation task breakdown (ordered)

1. **Watcher rename-event fix:** confirm Phase 2 fsnotify handles `CREATE`/`RENAME` (atomic writes); fix if it only watched `WRITE`. (Prereq for hook flow.)
2. **Backend adapter layer:** extract `BackendAdapter` (`claude-acp` impl from existing code); add capability flags + `hookMap` + `resolveResumeId`. No behavior change for Claude.
3. **Codex backend (chat only):** implement `codex-acp` adapter; per-model env in launch composer (verify/add); launch → prompt → stream → stop → native resume green. (§6)
4. **Hook scripts + registration:** write `_emit.sh` + 5 event scripts; install on startup; wire CLI settings registration with env vars; implement interface gate. Test chat-gating (no double writes) and terminal status production. (§4)
5. **Terminal runtime:** AppleScript templates + escaping helper; `terminal.Available()` + `GET /api/capabilities`; `Start/SendPrompt/Cancel/Stop/Resume/CheckMessages`; `tty`/iterm ids in `running/`. (§3)
6. **Switch-runtime endpoint:** per-agent lock; cancel→stop→identity-update→resume; `resolveResumeId`; primer synthesis for backend swap; rollback. Wire all three swap dimensions. (§5, §8.1)
7. **rename + identity endpoints:** `POST /sessions/{id}/rename` (if absent), `POST /sessions/{id}/identity` (group), with direct `state_update` emission. (§8.2–8.3)
8. **Release-group endpoint:** `POST /api/groups/{group}/release` with bounded worker pool. (§8.4)
9. **Liveness sweep:** periodic prune of stale `running/` entries (dead pgid). (§9)
10. **UI — task groups:** collapsible sections, persisted collapse in `layout.json`, Move-to-group picker, Release-group action, group state summary. (§7)
11. **UI — switch-runtime dialog:** interface/backend/model pickers, terminal disabled via capabilities, history-handoff note ("model switch keeps native session" / "backend switch primes history"). (§5, §8.5)
12. **UI — terminal affordances:** card "terminal" badge + "Reveal tab" action calling a reveal endpoint/runtime helper. (§3.3)

---

## 11. Testing strategy

**Unit / Go**
- AppleScript escaping helper: quotes, backslashes, newlines, shell-quoting of launch argv (table-driven).
- `resolveResumeId`: same-backend model swap → native id; cross-backend → empty + handoff=primer; capability flag off → primer.
- Primer synthesis: respects token budget; falls back to truncation when summary call fails; emits `backend_switch` marker.
- Switch rollback: inject `Resume` failure after `Stop` → previous runtime restored, identity reverted, `500 switch_failed_rolled_back`.
- Hook `_emit.sh`: golden JSON for each event; interface gate (`AGENTDECK_INTERFACE=chat` → no status write for covered events; `terminal` → writes).

**Integration (server + fake CLI / real CLIs where available)**
- **Model swap preserves history on same agent_id:** launch chat Claude agent, run 2 turns, switch model, assert `agent_id` unchanged, prior transcript present in `sessions/{id}/`, new `session_id` in `running/`, turn 3 continues. (Acceptance F7.)
- **Interface swap chat↔terminal:** switch to terminal, assert terminal runtime active, `tty` recorded, status now hook-driven; switch back, chat live again, transcript intact.
- **Backend swap Claude→Codex:** assert handoff=primer, `backend_switch` marker in transcript, new Codex session runs, archive shows one continuous session. (Acceptance F7.)
- **Terminal status via hooks:** drive a terminal agent through a turn; assert `status/{id}.json` transitions idle→busy→(waiting_input on permission)→idle via hook writes, and SSE `state_update` fires for each (badge updates). (Acceptance F2/terminal.)
- **Codex chat end-to-end:** launch/prompt/stream/stop/resume green. (§6 gate.)

**UI / e2e**
- **Group collapse persists:** two agents same group → one collapsible header; collapse, reload → still collapsed. (Acceptance F2.)
- **Move to group:** move an agent via picker → identity updated, card relocates on `state_update`.
- **Release group stops all:** release → all member `running/` removed, cards leave; partial-failure path surfaces per-agent error. (Acceptance F2.)
- **Terminal degradation:** on a host without iTerm2 (or simulated `capabilities.terminal.available=false`), terminal option disabled with the reason tooltip; switch-to-terminal returns 422.

**Manual / macOS-only**
- Real iTerm2: tab created with correct title/color, prompt typed and submitted, Cancel sends SIGINT, Stop closes tab; 12-tab cap returns 429 on the 13th.

---

## 12. Resolved decisions (answers to §6 open questions)

1. **Cross-platform terminal fallback (tmux / OS terminal)?** — **Out of scope for Phase 6. iTerm2-on-macOS only.** Everywhere else the terminal interface is disabled at the API (`422 terminal_unavailable`) and in the UI (option greyed with `reason` from `GET /api/capabilities`). The chat runtime is the cross-platform default. A tmux/Terminal.app/Linux fallback is a future enhancement; the runtime registry + `terminal.Available()` seam is built so adding another terminal backend later requires no API changes.

2. **Backend-swap history hand-off semantics.** — **AgentDeck's normalized transcript (`sessions/{agent_id}/`) is the source of truth; native CLI sessions are never translated across backends.** Same-backend model swaps use native resume (CLI keeps its session; only the model arg changes), gated by the adapter's `CanSwitchModelOnResume`. Cross-backend swaps start a fresh native session and inject a **bounded history primer** (running summary of older turns + last N=6 verbatim turns, capped at `switch.primer_token_budget` = 8k tokens) appended to the launch composition only. A `backend_switch` transcript marker records the transition; the logical session (same `agent_id`, same `sessions/` log) continues unbroken. Primer summary failures degrade to truncated verbatim tail rather than blocking the switch.

3. **Concurrency limits with many terminal tabs.** — **Soft cap `terminal.max_tabs` = 12**; launching/switching-to terminal beyond it returns `429 terminal_tab_limit`. Per-agent **switch lock** serializes switch/stop/prompt per agent. Release-group and any batch stop use a **bounded worker pool (4 concurrent)** to limit osascript contention. Every `osascript` call is wrapped in a 4s timeout. A periodic **liveness sweep** prunes stale `running/` entries for dead process groups. (A global cross-interface agent concurrency / launch-queue limit remains a master-PRD-level open question and is intentionally not resolved here.)
```
