# Phase 6 — Flexibility: Implementation Tech Spec

**Mirrors:** `docs/features/phase-6-flexibility.md` (phase PRD)
**Features:** F7 (switch runtime on a live agent), F2 (task groups)
**Depends on:** Phase 1 (Runtime interface, chat runtime, launch composition), Phase 2 (state manager, SSE bus, card grid, layout), Phase 4 (resume machinery, transcript persistence)
**Enables:** Future-phase candidates
**Audience:** the engineer implementing this phase. Prescriptive; no further design decisions required.

---

## 1. Overview & scope recap

Phase 6 cashes in the stable-`agent_id` investment. Three deliverables, plus one enabling backend integration:

1. **Terminal runtime** — a second `Runtime` implementation (`interface == "terminal"`) that drives a terminal emulator instead of the ACP stream, deriving status from **hooks**. The cross-platform default is an embedded **xterm.js** terminal in the UI (or tmux); iTerm2/AppleScript is an optional macOS-only extra behind a capability probe.
2. **Hooks** — thin shell scripts registered with each agent CLI that **`POST /api/hook`** with the per-launch token on lifecycle events. The server applies the update to `state.db` and emits SSE. Gated by interface.
3. **Switch-runtime (F7)** — `POST /api/sessions/{id}/switch-runtime` that stops the current runtime, re-launches with new interface/backend/model on the **same** `agent_id`, and resumes (reusing Phase 4 `Runtime.Resume`) so the transcript and logical session continue.
4. **Codex backend (`codex-acp`)** wired end-to-end as the real second backend, so the backend-swap path in F7 is exercised against a true second provider.
5. **Task groups (F2)** — `group` column on the identity row, collapsible dashboard sections with persisted collapse state, functional "Move to group", and "release group" batch-stop.

### In scope
- `Runtime` impl for `terminal` — embedded xterm.js (or tmux) as the default; write prompts to the TTY; tab title/color/focus; `tty` in the running row.
- Optional iTerm2/AppleScript path behind a macOS capability probe.
- Hook script set (shell `curl` → `POST /api/hook`) + registration with both Claude Code and Codex CLIs.
- Switch-runtime endpoint + UI flow (chat↔terminal, Claude↔Codex, model swap).
- Codex (`codex-acp`) as a real second backend.
- Task groups: identity `group` field, collapsible sections, persisted collapse, Move to group, release group.
- `rename` endpoint if not already present from Phase 2.

### Out of scope
- **Activity map and other future-phase candidates.**

### Assumptions inherited from prior phases
- `Runtime` interface and chat runtime exist (Phase 1): `Start`, `SendPrompt`, `Cancel`, `Stop`, `Resume`, `CheckMessages`.
- A **runtime registry** dispatches by `agent.interface` (Phase 1); `terminal` previously returned "not implemented".
- The state manager owns `state.db` (identity, running registry, live status) as sole writer, ingests status via `POST /api/hook` and the chat runtime's ACP stream, and emits SSE `state_update` (Phase 2).
- `layout.json` persists card order + density via `GET/PUT /api/layout` (Phase 2). We extend it with group collapse state.
- `Runtime.Resume(agent, session_id)` is implemented for chat (Phase 4); transcript persists to `sessions/{agent_id}/` as an NDJSON append log.

The hooks-over-HTTP channel and the cross-platform terminal default follow directly from the recorded architecture decisions (see [../../architecture-decisions.md](../../architecture-decisions.md), D2 and D4).

---

## 2. Technology choices

### 2.1 Terminal emulator control from Go

The default terminal path is **embedded xterm.js in the UI** (with tmux as the headless/server-side alternative), which is cross-platform and matches the rest of the browser-served stack:

- **Embedded xterm.js (default):** the server runs the CLI under a PTY (`github.com/creack/pty`) and bridges that PTY to the browser over a WebSocket. The React UI renders an [xterm.js](https://xtermjs.org/) panel per terminal agent; keystrokes go UI → WebSocket → PTY master, and PTY output streams back the same way. The server records the PTY's `tty` in the running row. This works identically on macOS and Linux and needs no platform-specific scripting.
- **tmux (alternative):** for users who want a real, reattachable session outside the browser, the CLI can be launched inside a named tmux session (`tmux new-session -d -s agentdeck-<id> '<cmd>'`). Prompts are delivered with `tmux send-keys`; the `tty` is read with `tmux display-message -p '#{pane_tty}'`. The same PTY-bridge UI can attach to it, or the user attaches manually.
- The terminal runtime is structured behind a small `TerminalDriver` seam (`StartTab`, `WriteText`, `ReadTTY`, `CloseTab`, `RevealTab`) so the xterm/PTY driver, the tmux driver, and the optional iTerm2 driver are interchangeable. The registry never sees the difference.

### 2.2 Optional iTerm2/AppleScript driver (macOS only)

iTerm2 control is an **optional extra** selected only when the host advertises it via the capability probe (§3.5). It implements the same `TerminalDriver` seam:

- **Mechanism:** AppleScript via `osascript`, shelled out from Go with `os/exec`. No CGo ScriptingBridge dependency; `osascript` keeps the binary pure-Go. The operations needed (create tab, set title/color/profile, write text, read the session's TTY, activate) are first-class in iTerm2's AppleScript dictionary.
- **TTY discovery:** iTerm2 AppleScript exposes `tty` on a session object; after creating the tab we read it back and store it in the running row.
- **Writing prompts:** iTerm2 `write text "<prompt>"` types into the session as if at the keyboard (appends a newline / submits) so the CLI's line editor receives a proper submitted line.
- **Templating:** AppleScript is rendered from Go `text/template` strings with the agent's params, then passed to `osascript -` over stdin (avoids shell-quoting hell; see §3.6 for escaping rules).

This driver is never the core path; on any host where the probe reports iTerm2 unavailable, the terminal interface falls back to the xterm.js/tmux default.

### 2.3 Hook script language and registration
- **Language:** POSIX `sh` (not bash-specific) shell scripts. Each does one thing — assemble a small JSON object and **`POST` it to `/api/hook`** with `curl`, carrying the per-launch token. No file writes, no JSON files on disk. The control flow stays in `sh`; `curl --json` (or `-H 'Content-Type: application/json' --data` ) sends the body. JSON values that may contain untrusted tool arguments are encoded by `jq` (`jq -nc --arg ...`), which is a documented prereq — there is no python3 and no runtime Node.
- **Location:** installed to `~/.agentdeck/hooks/` by `install.sh` and on server startup (the server writes/refreshes them so they always match the running binary's expectations). One script per event, plus a shared `_post.sh` helper.
- **Per-launch token + endpoint:** at `Start`/`Resume`, the runtime mints a one-time token and passes the server URL and token to the CLI process via environment variables (`AGENTDECK_HOOK_URL`, `AGENTDECK_HOOK_TOKEN`), alongside `AGENTDECK_AGENT_ID` and `AGENTDECK_INTERFACE`. A single generic script serves all agents; the token is rotated on every launch/resume so a stale token can't post.
- **Registration with Claude Code:** Claude Code reads hooks from its settings JSON. At launch the runtime composes a `--settings` (or project `.claude/settings.json` injection) that maps each lifecycle event to the matching `~/.agentdeck/hooks/<event>.sh`. Identity, interface, and token reach the hooks via the env vars above.
- **Registration with Codex (`codex-acp`):** Codex's hook surface differs (see §6). Where Codex lacks a 1:1 hook for an event, the terminal runtime falls back to the events Codex *does* expose and fills gaps from the ACP/notification channel. Hook scripts are identical; only the registration mapping differs per backend (kept in a per-backend `hookMap`).

### 2.4 Codex (`codex-acp`) integration
- `backends.json` already defines a `codex` backend of `type: "codex-acp"` with `default_model: "gpt-5.5"` and a backend-level `env` (`CODEX_HOME`).
- `codex-acp` speaks ACP over stdio like `claude-acp`, so it reuses the **chat runtime** transport; differences are confined to: launch argv, env (`CODEX_HOME`, `OPENAI_API_KEY`/`OPENAI_BASE_URL` per-model overrides), the resume/session-id mechanism, and the hook event names. These are captured in a per-backend **adapter** (§6) rather than branching the runtime.

---

## 3. Terminal runtime design

Package: `internal/runtime/terminal` implementing the same `Runtime` interface as chat. Registered in the registry under `interface == "terminal"`. It dispatches to a `TerminalDriver` (§2.1): the xterm.js/PTY driver and tmux driver are cross-platform; the iTerm2 driver is an optional macOS extra (§3.5).

### 3.1 Lifecycle

```
Start(agent):
  1. Compose config exactly as chat runtime (cwd, context_prompt, system_prompt, backend/model, env).
  2. Mint a per-launch hook token; set AGENTDECK_HOOK_URL / AGENTDECK_HOOK_TOKEN /
     AGENTDECK_AGENT_ID / AGENTDECK_INTERFACE=terminal on the CLI process env.
  3. Build the CLI argv (resume form if a prior session_id exists for this agent_id — see §5).
  4. driver.StartTab(spec): launch
        `cd <cwd> && <env...> <cli argv>`
     under a PTY (xterm/tmux default) or an iTerm2 tab (optional). Hooks are registered via
     the CLI's settings (env-pointed), same as chat.
  5. Read back the new tab's tty (and, for iTerm2, the window/tab/session identifiers).
  6. Write the running ROW to state.db:
        {agent_id, pid, session_id:"", interface:"terminal", tty, started_at, driver, driver_ids}
     - pid: the process group of the launched CLI (the PTY's child pgid; for iTerm2, `job pid of
       session`, falling back to the tty's foreground pgid).
  7. Set the live status row to {state:"idle"} ONLY if no hook has posted one within 500ms
     (avoid clobbering a SessionStart hook POST).
```

`SendPrompt(agent, text)`: `driver.WriteText(tab, text)` — for the PTY/xterm and tmux drivers this writes the prompt (plus newline) to the PTY master / `tmux send-keys`; for iTerm2 it renders the "write text" AppleScript. No ACP round-trip; the response renders in the terminal, and **status flows through hooks**.

`Cancel(agent)`: send `SIGINT` to the foreground process group of the recorded `tty` (`syscall.Kill(-pgid, SIGINT)`), matching how the chat runtime cancels. (Writing a raw `^C` into the emulator is unreliable across drivers, so we signal the pgid directly.)

`Stop(agent)`: `SIGTERM` then (after grace) `SIGKILL` to the process group; then `driver.CloseTab(tab)` (best-effort — close the PTY/WebSocket, kill the tmux session, or close the iTerm2 tab); then delete the running ROW from state.db. Leave `sessions/` intact. The live status row is set to `done`/`idle` by the Stop hook, or by the runtime if the hook didn't fire (e.g. hard kill).

`Resume(agent, session_id)`: identical to `Start` but the argv uses the backend's resume form (§5/§6) with `session_id`. Used by switch-runtime and by Phase 4 archive resume when the target interface is terminal.

`CheckMessages(pid)`: nudger support (Phase 5) — deliver a `check_messages` prompt via the same `driver.WriteText` path. (Same contract as chat; just a different delivery mechanism.)

### 3.2 Tab title / color / focus
- **Title:** `"{name} · {role}@{project}"`. Re-set on rename and on switch-runtime. For xterm.js this is the panel header in the UI; for tmux it's the window name; for iTerm2 the session `name`.
- **Color:** the project accent from `project.color`. xterm.js tints the panel chrome in the UI; iTerm2 sets `background color` (mapped from `[r,g,b]` 0–255 to iTerm2's 0–65535 scale). tmux leaves emulator chrome to the client.
- **Focus:** `Start` brings the new tab forward in its surface (focus the xterm panel / `activate` the iTerm2 window); subsequent `SendPrompt` does **not** steal focus unless the call is user-initiated from the UI "reveal terminal" action. A `RevealTab(agent)` helper exists for the UI to bring a tab forward on demand.

### 3.3 Status derivation (hooks only)
The terminal runtime writes **no** status updates during a turn. All state (`busy`/`idle`/`waiting_input`/`done`/`error`, `detail`, `last_trace`, `busy_since`, `context_pct`) is produced by hook POSTs to `/api/hook` (§4), which the server applies to `state.db`. The runtime only sets the **initial** idle status at `Start` if the SessionStart hook hasn't beaten it (race guard, §3.1 step 7), and a terminal `done`/`idle` on `Stop` if the Stop hook didn't fire.

### 3.4 PTY / WebSocket bridge (xterm.js default)
- The server allocates a PTY per terminal agent and registers a per-agent WebSocket endpoint (`/api/sessions/{id}/terminal/ws`, token-authed like the rest of the local API).
- Browser → server frames are raw keystrokes written to the PTY master; server → browser frames are PTY output. Resize messages (`{cols, rows}`) call `pty.Setsize`.
- The bridge is independent of status: the terminal *content* flows over the WebSocket, while terminal *status* flows over hooks. A user with no browser open still has a live terminal agent (the PTY runs server-side); the UI simply attaches when opened.

### 3.5 Capability probe & graceful degradation
A `terminal.Capabilities()` helper reports which drivers the host can use:
- The **xterm.js/PTY driver is always available** (pure-Go PTY on macOS and Linux), so the terminal interface itself is never globally disabled.
- **tmux** is offered if `tmux` is on `PATH`.
- **iTerm2** is offered only if `runtime.GOOS == "darwin"` **and** iTerm2 is installed — probed via `osascript -e 'id of application "iTerm2"'` (or `/Applications/iTerm.app`). Non-zero/empty → iTerm2 driver omitted, with a reason string.

The server exposes this at **`GET /api/capabilities`**:
```json
{ "terminal": { "available": true,
                "drivers": { "xterm": true, "tmux": true,
                             "iterm2": { "available": false, "reason": "iTerm2 is not installed" } },
                "default_driver": "xterm" } }
```
Behavior:
- A terminal launch/switch always succeeds via the default driver; the UI uses the default unless the user explicitly picks an available optional driver.
- If a request explicitly asks for an **unavailable** optional driver (e.g. `driver:"iterm2"` on Linux), the server returns **422** `{error:"terminal_unavailable", reason}`. The New-Agent modal and Switch-runtime dialog disable the iTerm2 option and show `reason` as a tooltip so the UI doesn't guess.

### 3.6 iTerm2 AppleScript templates & escaping (optional driver)
Used only by the iTerm2 driver. Three templates, rendered with Go `text/template`, piped to `osascript -`:

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
(Create a **window** per agent by default; grouping windows into one window's tabs is a future-phase nicety. We still call them "tabs" loosely.)

**set-appearance.applescript** — sets `name` (title) and `background color` from `project.color` (mapped 0–255 → 0–65535).

**write-text.applescript** — `tell session id "<sid>" of ... to write text "<escaped prompt>"`.

**Escaping rules (mandatory):**
- All dynamic strings are escaped for AppleScript string literals: `\` → `\\`, `"` → `\"`, and newlines converted to explicit `" & return & "` segments (do **not** embed raw newlines in `write text`).
- The `LaunchCommand` is a shell command; build it as an argv joined with proper shell quoting (a small Go shell-quote helper, never `fmt.Sprintf` interpolation of user strings), then AppleScript-escape the whole string.
- Never interpolate user content into the AppleScript template without going through both quoting layers. (The xterm/tmux drivers avoid AppleScript escaping entirely; they write bytes to a PTY or pass argv to `tmux` directly.)

---

## 4. Hooks design

### 4.1 Files (`~/.agentdeck/hooks/`)
- `_post.sh` — shared helper. Reads `AGENTDECK_AGENT_ID`, `AGENTDECK_INTERFACE`, `AGENTDECK_HOOK_URL`, `AGENTDECK_HOOK_TOKEN`, plus event-specific args, builds a JSON body, and **POSTs it to `/api/hook`** with `curl`.
- `session-start.sh`, `user-prompt-submit.sh`, `pre-tool-use.sh`, `post-tool-use.sh`, `stop.sh` — thin wrappers that map the CLI's event payload to `_post.sh` arguments.

`_post.sh` core (illustrative; `jq` encodes arbitrary strings safely):
```sh
#!/bin/sh
# usage: _post.sh EVENT STATE [k=v ...]   -> POST /api/hook
[ -n "$AGENTDECK_AGENT_ID" ] || exit 0
[ -n "$AGENTDECK_HOOK_URL" ] || exit 0
event="$1"; state="$2"; shift 2

# interface gate: chat status is owned by the runtime's ACP stream (see §4.3)
if [ "$AGENTDECK_INTERFACE" = "chat" ]; then
  case "$event" in
    SessionStart|UserPromptSubmit|PreToolUse|PostToolUse|Stop) exit 0 ;;
  esac
fi

# build the JSON body; jq encodes every value safely
body="$(jq -nc \
  --arg agent_id "$AGENTDECK_AGENT_ID" \
  --arg event "$event" \
  --arg state "$state" \
  --args '$ARGS.positional
          | map(split("=") | {(.[0]): (.[1:] | join("="))})
          | add // {}
          | . + {agent_id:$agent_id, event:$event, state:$state}' \
  "$@")"

curl -fsS --max-time 4 \
  -H "Content-Type: application/json" \
  -H "X-AgentDeck-Token: $AGENTDECK_HOOK_TOKEN" \
  --data "$body" \
  "$AGENTDECK_HOOK_URL" >/dev/null 2>&1 || true
```

### 4.2 Event → POST mapping

| Event | state | fields in the POST body | notes |
|-------|-------|--------------------------|-------|
| SessionStart | `idle` | `detail:"session started"`, `last_trace:"SessionStart"`, `busy_since:""`, plus `session_id`/`tty` if the CLI exposes them | server refreshes the running row from these |
| UserPromptSubmit | `busy` | `detail:"thinking"`, `busy_since:<now>`, `last_trace:"UserPromptSubmit"`, `context_pct` if available | — |
| PreToolUse | `busy` | `detail:"<tool>: <short args>"`, `last_trace:"PreToolUse: <tool>"` | — |
| PostToolUse | `busy` | `detail:"<tool> done"`, `last_trace:"PostToolUse: <tool>"`; `waiting_input` if the tool was a permission/ask | — |
| Stop | `idle` or `done` | `detail:"turn complete"`, `last_trace:"Stop"`, `busy_since:""` | on CLI exit the server clears the running row |

`waiting_input`: when the CLI emits a permission/approval prompt event (Claude Code surfaces this; mapping noted per-backend), the relevant hook POSTs `state:"waiting_input"`. Terminal agents thus show the same `waiting_input` badge as chat agents.

`context_pct`: posted from whatever the CLI exposes in the hook payload (Claude Code provides context usage; if absent, omit the field — never post a stale value).

### 4.3 Interface gating
- Every hook carries `AGENTDECK_INTERFACE`. For `interface == "chat"`, status is authoritatively produced by the chat runtime from the ACP stream, so the hook scripts **skip the POST** for the events the chat runtime already covers (SessionStart/UserPromptSubmit/Pre/PostToolUse/Stop). Concretely: `_post.sh` exits early (`exit 0`) when `AGENTDECK_INTERFACE=chat` and the event is one the chat runtime owns. This keeps a single status producer per agent.
- For `interface == "terminal"`, hooks are the sole status producer; they always POST.
- The gate is a single guard at the top of the shared helper, so the same scripts ship for all agents.

### 4.4 Server-side ingest (`POST /api/hook`)
- The endpoint validates `X-AgentDeck-Token` against the agent's current per-launch token (rotated each launch/resume); a missing/stale token → `401`, so other local processes can't spoof status.
- On a valid POST the server **applies the update to `state.db`** — upserts the live status row (and refreshes/clears the running row on SessionStart/Stop) — then emits SSE `state_update`. This is the same code path the chat runtime uses to apply ACP-derived status; hook-produced status is indistinguishable downstream.
- Because the server is the sole writer to `state.db`, there is no file-watcher, no atomic-rename dance, and no two-writer race on disk: ordering is whatever order the POSTs (and ACP applications) arrive in, serialized by the state manager. (The reconciliation sweep over `sessions/` from Phase 2 remains only as a fallback for CLI-written transcript files, not a status channel.)

---

## 5. Switch-runtime design (F7)

`POST /api/sessions/{id}/switch-runtime {interface?, backend?, model?}` — any subset of the three may change; at least one must differ from current, else **400**.

### 5.1 Core algorithm (interface and/or model swap, same backend)
```
switchRuntime(id, {interface?, backend?, model?}):
  agent  = readIdentity(id)                  // stable agent_id (state.db row)
  prev   = readRunning(id)                    // current pid, session_id, interface, tty (state.db row)
  target = merge(agent, requested fields)     // new interface/backend/model

  validate(target)                            // §3.5 driver availability; backend/model exist
  acquire per-agent switch lock (§9)

  1. Flush + checkpoint: ensure transcript persistence (Phase 4) has flushed all events
     for the current ephemeral session to sessions/{agent_id}/. Record current session_id.
  2. Cancel any in-flight turn (Runtime.Cancel) and wait for turn_end or timeout (§9 mid-turn).
  3. oldRuntime = registry.For(prev.interface)
     oldRuntime.Stop(agent)                   // terminates process group, removes running row, keeps sessions/
  4. Persist new identity fields: update the agents identity ROW in state.db
     (interface/backend/model). agent_id UNCHANGED.
  5. newRuntime = registry.For(target.interface)
  6. resumeSessionId = resolveResumeId(agent, prev, target)   // §5.3 backend hand-off
     newRuntime.Resume(agent, resumeSessionId)                // reuses Phase 4 resume:
        - re-composes config from updated identity
        - restores transcript view from sessions/{agent_id}/
        - launches CLI in resume mode, writes a fresh running ROW (new ephemeral session_id)
  7. release lock; emit state_update (identity changed → card re-renders backend/model/interface badges)
  return 200 with updated identity + running summary.
```
The **same logical session** continues because `agent_id` never changes and `sessions/{agent_id}/` is the durable transcript the resume restores from. The ephemeral `session_id` changes (expected).

### 5.2 chat ↔ terminal
- **chat → terminal:** Stop chat runtime; Resume under terminal runtime. The terminal CLI is launched in resume mode so the user sees prior context in the terminal where the backend supports `--resume <id>` (Claude Code does; Codex via `CODEX_HOME` session, §6). Status switches from ACP-derived to hook-derived automatically (the hooks were always registered; the interface gate now lets them POST). The chat panel for this agent becomes read-only (shows persisted transcript) and the card gains a "terminal" badge / "reveal terminal" affordance.
- **terminal → chat:** Stop terminal runtime (close tab); Resume under chat runtime; chat panel goes live again. Hook POSTs for this agent now self-suppress (gate sees `interface=chat`).

### 5.3 backend swap (Claude ↔ Codex) — history hand-off (concrete)
The two backends have **incompatible native session formats**. We do **not** translate one CLI's session store into the other's. The hand-off is defined as:

1. **AgentDeck transcript is the source of truth**, not the CLI's session file. `sessions/{agent_id}/` holds normalized events (Phase 4 NDJSON).
2. On a backend swap, `resolveResumeId` returns **empty** (there is no compatible native session id to resume in the new backend). The new backend starts a **fresh native session** (`session_id` new).
3. To preserve continuity, switch-runtime injects a **history primer** into the new backend's launch composition: a system/context message synthesized from the AgentDeck transcript — a bounded, summarized rendering of prior turns (assistant text + tool outcomes + final state), appended to `role.system_prompt` for this launch only (not persisted to the role). Budget the primer to a token cap (default 8k tokens; configurable `config.json: switch.primer_token_budget`); when the transcript exceeds the cap, prime with (a) a generated running summary of older turns plus (b) the last N verbatim turns (default N=6). Summary generation uses the **target** backend's default model via a one-shot non-interactive call before the interactive launch, or a cheap local heuristic truncation if that call fails (degrade, don't block).
4. The AgentDeck transcript continues to append to the **same** `sessions/{agent_id}/` log under the new backend — so from the user's and the archive's perspective it is one continuous session. A transcript marker event `{type:"backend_switch", from, to, at}` is appended so the chat panel can render a divider.

Same-backend model swap (Claude sonnet → opus, or Codex gpt-5.5 → gpt-4o) uses the **native resume** path (`resolveResumeId` returns the existing native `session_id`); no primer needed, because the CLI keeps its own session and only the model arg changes. (Claude Code supports model switch on resume; if a given backend cannot change model on resume, fall back to the primer path — recorded per-backend in the adapter.)

### 5.4 Concurrency & atomicity
- Per-agent **switch lock** (in-memory mutex keyed by `agent_id`) serializes switch/stop/prompt for that agent.
- If `Resume` (step 6) fails after `Stop` (step 3), **roll back**: re-launch the previous interface/backend/model via `Resume` with the previous resume id; restore the identity row; return **500** with `{error:"switch_failed_rolled_back", detail}`. If rollback also fails, leave identity at target, set the status row to `error`, and return **500** `{error:"switch_failed", detail}` (the agent is recoverable via archive resume).

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
- `group` is a column on the identity ROW in `state.db` (`"group": "auth-migration"`, optional/empty). No schema migration beyond the column existing from Phase 2's identity table.
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
- Selecting a target issues an **identity edit**: `POST /api/sessions/{id}/identity {group}` (see §8) which updates the identity ROW in `state.db`. Because the server is the sole writer and already holds the merged state, the endpoint **emits an SSE `state_update` for that agent directly** after the write — no watcher and no extra ingest path.
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
- `422 {error:"terminal_unavailable", reason}` — target interface terminal with an explicitly requested optional driver the host lacks (§3.5).
- `500 {error:"switch_failed_rolled_back", detail}` / `500 {error:"switch_failed", detail}` — see §5.4.

### 8.2 `POST /api/sessions/{id}/rename` (add if absent)
Request `{ "name": "Atlas" }` → updates the identity ROW `name`, retitles the terminal tab if terminal, emits `state_update`.
- `200 {agent_id, name}` · `400 {error:"empty_name"}` · `404 {error:"agent_not_found"}`.

### 8.3 `POST /api/sessions/{id}/identity` (group + other identity edits)
Request (partial; supports `group` and `name`):
```json
{ "group": "auth-migration" }
```
`group:""` or omitted-with-explicit-null removes from group.
- `200 {agent_id, group, name, role, project, backend, model, interface}` (full identity echo) · `404 {error:"agent_not_found"}` · `400 {error:"invalid_group_name"}` (e.g. reserved `_ungrouped`).
- Side effect: emit `state_update` for the agent (§7.3).

### 8.4 `POST /api/groups/{group}/release`
Request: empty body.
Response `200`:
```json
{ "group": "auth-migration", "stopped": [
    {"agent_id":"a_1","ok":true},
    {"agent_id":"a_2","ok":false,"error":"stop_timeout"} ] }
```
- `404 {error:"group_not_found"}` — no agents currently carry this group. Releasing an empty group → `404` (distinguishes typos).
- Partial failures are reported per-agent with overall `200` (the operation ran); the client surfaces which agents failed.

### 8.5 `GET /api/capabilities`
Response `200`:
```json
{ "terminal": { "available": true,
                "drivers": { "xterm": true, "tmux": true,
                             "iterm2": { "available": false, "reason": "iTerm2 is not installed" } },
                "default_driver": "xterm" } }
```
Consumed by the New-Agent modal and Switch-runtime dialog: the terminal interface is always offered (the xterm default is always available); only optional drivers the host lacks are disabled, with `reason` as a tooltip.

### 8.6 `POST /api/hook`
Request (per-launch token in `X-AgentDeck-Token`):
```json
{ "agent_id": "a_8f3c12", "event": "PreToolUse", "state": "busy",
  "detail": "Edit: src/auth.ts", "last_trace": "PreToolUse: Edit", "context_pct": 0.42 }
```
- `204` — applied to `state.db`, `state_update` emitted.
- `401 {error:"bad_token"}` — missing/stale per-launch token.
- `404 {error:"agent_not_found"}` — unknown `agent_id`.

---

## 9. Concurrency, edge cases & error handling

- **Switch mid-turn:** switch-runtime first calls `Cancel` and waits up to `config.switch.cancel_timeout_ms` (default 5000) for `turn_end`. On timeout, proceed to `Stop` anyway (the turn's streamed events are already persisted; the in-flight tool result may be lost — acceptable, recorded as a transcript `{type:"turn_interrupted"}` marker). Never switch while a turn is mid-stream without canceling first.
- **Optional driver missing / disappears:** the xterm default is always available, so the terminal interface never hard-fails. An explicit request for an absent optional driver returns `422` (§3.5). If iTerm2 is force-quit while an iTerm2-driven agent runs, the next hook/liveness check finds the process gone; the state manager marks the agent stopped (running row cleared on CLI exit, or reaped by a periodic liveness sweep that checks pgid existence and prunes stale running rows).
- **Backend swap with incompatible history:** always handled by the primer path (§5.3) — never attempt cross-backend native resume. If primer summary generation fails, fall back to truncated verbatim tail; if even the launch fails, roll back (§5.4).
- **Many terminal tabs:** enforce a soft cap `config.terminal.max_tabs` (default 12). Launch/switch-to-terminal beyond the cap returns `429 {error:"terminal_tab_limit", limit}`; the UI surfaces it. Release-group and stop free slots. (Chat agents are uncapped here; a global agent concurrency limit remains a master-PRD open question, not resolved in this phase.)
- **Release-group concurrency:** stop agents with a bounded worker pool (default 4 concurrent stops) to avoid contention (osascript/iTerm2 or tmux) when closing many tabs.
- **Single status producer per agent:** guaranteed by the interface gate (§4.3) — chat status comes from the ACP stream, terminal status from hook POSTs, and the server serializes all `state.db` writes as sole writer. During the switch window the per-agent lock (§5.4) ensures the old producer is fully stopped before the new one starts.
- **Hook token security:** `POST /api/hook` requires the current per-launch token; stale tokens (after resume/switch) are rejected `401`, so a lingering hook from a stopped launch can't post.
- **osascript failures (iTerm2 driver only):** wrap every `osascript` call with a timeout (default 4s) and capture stderr; map failures to actionable errors (`terminal_unavailable`, `iterm_script_failed`). Never hang the request on a stuck AppleScript.

---

## Subphase plan (incremental / quota-limited implementation)

The invariant: every subphase ends at a GREEN checkpoint — `go build ./...` passes (and `npm run build` for UI subphases) and all existing tests pass — so work is never half-done and a fresh agent can resume cold at the next subphase without inheriting partial state.

### Subphase 6.1 — Hook ingest + backend adapter + Codex (chat)
- **Goal:** Harden the `/api/hook` ingest path and land `codex-acp` as a real second backend behind a per-backend adapter, all on the existing chat runtime.
- **Deliverables:** Hardened `POST /api/hook` ingest with per-launch-token validation and running-row refresh/clear on SessionStart/Stop (§4.4, §8.6, task 1); `internal/backend/adapter.go` with `BackendAdapter` interface + `claude-acp` impl extracted from existing code, carrying capability flags, `hookMap`, `resolveResumeId` (§6.3, task 2); `codex-acp` adapter + per-model env in the launch composer (§6.1–6.2, task 3).
- **Depends on:** Phase 1 (Runtime, chat runtime, launch composition), Phase 2 (state manager, SSE, existing hook endpoint), Phase 4 (resume, transcript persistence). First subphase — no prior subphase.
- **Done when (checkpoint):** `go build ./...` and existing tests pass; hook-ingest unit tests (valid token applies status + emits `state_update`; stale token → `401`) pass; a Codex chat agent runs launch → prompt → stream → stop → native resume green (§6 acceptance gate).
- **Resume note:** At start, the chat runtime and Phase 2 hook endpoint exist with Claude-only logic inline. Begin by extracting `BackendAdapter`, then add Codex, then harden `/api/hook` token validation.
- **Size:** M

### Subphase 6.2 — Hook scripts + registration + interface gate
- **Goal:** Ship the shell hook script set and wire CLI-settings registration so terminal-bound status can flow over `POST /api/hook`, with chat agents self-gating.
- **Deliverables:** `~/.agentdeck/hooks/` script set — `_post.sh` helper + `session-start.sh`, `user-prompt-submit.sh`, `pre-tool-use.sh`, `post-tool-use.sh`, `stop.sh` (shell `curl` → `/api/hook`, `jq`-encoded); install/refresh on server startup; CLI-settings registration injecting `AGENTDECK_HOOK_URL`/`AGENTDECK_HOOK_TOKEN`/`AGENTDECK_AGENT_ID`/`AGENTDECK_INTERFACE` per launch via the per-backend `hookMap`; interface gate in `_post.sh` (§4.1–4.3, task 4).
- **Depends on:** Subphase 6.1 (hardened ingest + adapter `hookMap`).
- **Done when (checkpoint):** `go build ./...` and existing tests pass; interface-gate test green (`AGENTDECK_INTERFACE=chat` → no POST for covered events; `terminal` → POSTs); scripts install on startup and a chat agent shows no redundant hook POSTs.
- **Resume note:** At start, `/api/hook` ingest and `hookMap` exist but no scripts are installed and no registration happens. Begin by writing `_post.sh` + event scripts, then the startup installer, then launch-time settings injection.
- **Size:** M

### Subphase 6.3 — Terminal runtime (xterm/PTY default + tmux)
- **Goal:** Implement the `terminal` Runtime behind the `TerminalDriver` seam with the cross-platform xterm.js/PTY default (and tmux), deriving status from hooks.
- **Deliverables:** `internal/runtime/terminal` implementing `Start/SendPrompt/Cancel/Stop/Resume/CheckMessages`; `TerminalDriver` seam (`StartTab`, `WriteText`, `ReadTTY`, `CloseTab`, `RevealTab`); xterm.js/PTY driver (`github.com/creack/pty`) + PTY↔WebSocket bridge at `/api/sessions/{id}/terminal/ws`; tmux driver; `terminal.Capabilities()` + `GET /api/capabilities`; `tty`/`driver`/`driver_ids` in the running row (§3.1–3.5, §8.5, task 5).
- **Depends on:** Subphase 6.2 (hooks are the terminal status producer).
- **Done when (checkpoint):** `go build ./...` and existing tests pass; PTY-bridge unit tests (keystroke→master write, output→frame, resize→`Setsize`) pass; `GET /api/capabilities` returns `xterm: true` with `default_driver: "xterm"`; a terminal agent launches, records `tty`, and transitions idle→busy→idle via hook POSTs.
- **Resume note:** At start, hooks POST status but `interface == "terminal"` still returns "not implemented" in the registry. Begin with the `TerminalDriver` interface, then the PTY driver + WS bridge, then capabilities.
- **Size:** M

### Subphase 6.4 — Switch-runtime: same-backend (interface/model swap)
- **Goal:** Land the switch-runtime endpoint for the non-risky path — interface and/or model swap on the same backend via native `Runtime.Resume`.
- **Deliverables:** `POST /api/sessions/{id}/switch-runtime`; per-agent switch lock; cancel→stop→identity-update→resume algorithm; `resolveResumeId` returning the existing native `session_id` for same-backend model swap; chat↔terminal interface swap; rollback on Resume failure (§5.1–5.2, §5.4, §8.1, task 7 partial). Primer/backend-swap path explicitly deferred to 6.5 (`resolveResumeId` may return empty + a TODO guard for cross-backend, not yet wired to a primer).
- **Depends on:** Subphase 6.3 (terminal runtime exists so chat↔terminal swap is real).
- **Done when (checkpoint):** `go build ./...` and existing tests pass; same-backend model-swap integration test green (same `agent_id`, prior transcript intact, new `session_id`, turn continues); chat↔terminal swap test green; rollback test green (`Resume` failure after `Stop` → previous runtime restored, identity reverted, `500 switch_failed_rolled_back`).
- **Resume note:** At start, terminal + chat runtimes and adapters exist; no switch endpoint. Begin with the lock + core algorithm + native `resolveResumeId`; leave cross-backend returning empty without a primer.
- **Size:** M

### Subphase 6.5 — Switch-runtime: backend-swap history primer (riskiest)
- **Goal:** Complete F7 by wiring the cross-backend (Claude↔Codex) history-primer hand-off — the riskiest path, isolated so a quota-limited agent can stop cleanly after 6.4.
- **Deliverables:** `resolveResumeId` returns empty for cross-backend; bounded history-primer synthesis appended to launch composition (running summary of older turns + last N=6 verbatim turns, capped at `switch.primer_token_budget` = 8k; one-shot target-model summary call with truncation fallback); `{type:"backend_switch", from, to, at}` transcript marker; `history_handoff` field in the `200` response; `CanSwitchModelOnResume` gating native-vs-primer for model swaps (§5.3, §8.1, task 7 remainder).
- **Depends on:** Subphase 6.4 (switch endpoint + lock + same-backend resume).
- **Done when (checkpoint):** `go build ./...` and existing tests pass; primer-synthesis unit tests (respects token budget; truncation fallback when summary call fails; emits `backend_switch` marker) pass; Claude→Codex backend-swap integration test green (handoff=primer, marker in transcript, new Codex session runs, archive shows one continuous session).
- **Resume note:** At start, switch-runtime works for same-backend; cross-backend returns empty with no primer. Begin with primer synthesis + budget, wire it into Resume's launch composition, then the marker + response field.
- **Size:** M

### Subphase 6.6 — Task groups + remaining endpoints + UI
- **Goal:** Land task groups end-to-end plus the remaining identity/rename/release endpoints, liveness sweep, and the switch/terminal UI affordances.
- **Deliverables:** `POST /api/sessions/{id}/rename` (if absent) + `POST /api/sessions/{id}/identity` (group) with direct `state_update` (§8.2–8.3, task 8); `POST /api/groups/{group}/release` bounded worker pool (§8.4, task 9); liveness sweep pruning stale running rows (§9, task 10); UI — collapsible group sections with `layout.json` collapse persistence, Move-to-group picker, Release-group, group state summary (§7, task 11); UI — switch-runtime dialog with capability-gated drivers + handoff note (task 12); UI — xterm.js panel attaching to the PTY WebSocket + "terminal" badge + "Reveal terminal" (task 13).
- **Depends on:** Subphase 6.5 (full switch path) and Subphase 6.3 (terminal/PTY WS for the UI panel).
- **Done when (checkpoint):** `go build ./...` and `npm run build` pass; existing tests pass; group-collapse-persists e2e green (collapse, reload, still collapsed); Move-to-group and Release-group e2e green; terminal panel attaches to the PTY WS and shows hook-driven badges.
- **Resume note:** At start, all backend switch/terminal/hook machinery is green; groups still stubbed and no switch/terminal UI. Begin with identity/group endpoints, then UI sections, then the switch dialog and terminal panel.
- **Size:** M

### Subphase 6.7 — OPTIONAL iTerm2/AppleScript driver
- **Goal:** Add the optional macOS-only iTerm2 driver behind the capability probe — isolated and fully skippable without affecting any prior checkpoint.
- **Deliverables:** iTerm2 `TerminalDriver` impl via `osascript`; AppleScript templates (create-tab, set-appearance, write-text) rendered with `text/template`; the mandatory escaping + shell-quote helper; capability-probe wiring so an explicit unavailable-driver request returns `422 terminal_unavailable` (§2.2, §3.6, task 6).
- **Depends on:** Subphase 6.3 (`TerminalDriver` seam + capability probe). Independent of 6.4–6.6.
- **Done when (checkpoint):** `go build ./...` and existing tests pass; AppleScript-escaping helper unit tests (quotes, backslashes, newlines, argv shell-quoting) pass; on a non-macOS host an explicit `driver:"iterm2"` request returns `422` with a reason.
- **Resume note:** At start, the xterm/tmux drivers and `Capabilities()` exist. Begin with the escaping helper + templates, then the driver impl, then probe registration. Fully skippable if quota is tight.
- **Size:** S

## 10. Implementation task breakdown (ordered)

1. **`POST /api/hook` ingest:** confirm Phase 2's hook endpoint applies status to `state.db` and emits `state_update`; add per-launch-token validation and running-row refresh/clear on SessionStart/Stop. (Prereq for hook flow.)
2. **Backend adapter layer:** extract `BackendAdapter` (`claude-acp` impl from existing code); add capability flags + `hookMap` + `resolveResumeId`. No behavior change for Claude.
3. **Codex backend (chat only):** implement `codex-acp` adapter; per-model env in launch composer (verify/add); launch → prompt → stream → stop → native resume green. (§6)
4. **Hook scripts + registration:** write `_post.sh` + 5 event scripts (shell `curl` → `/api/hook`); install on startup; wire CLI settings registration with env vars (URL, token, agent_id, interface); implement interface gate. Test chat-gating (no redundant POSTs) and terminal status production. (§4)
5. **Terminal runtime — xterm/PTY default:** `TerminalDriver` seam; PTY+WebSocket bridge; `terminal.Capabilities()` + `GET /api/capabilities`; `Start/SendPrompt/Cancel/Stop/Resume/CheckMessages`; `tty`/driver ids in the running row. tmux driver. (§3)
6. **Optional iTerm2 driver:** AppleScript templates + escaping helper behind the macOS probe. (§2.2, §3.6)
7. **Switch-runtime endpoint:** per-agent lock; cancel→stop→identity-update→resume; `resolveResumeId`; primer synthesis for backend swap; rollback. Wire all three swap dimensions. (§5, §8.1)
8. **rename + identity endpoints:** `POST /sessions/{id}/rename` (if absent), `POST /sessions/{id}/identity` (group), with direct `state_update` emission. (§8.2–8.3)
9. **Release-group endpoint:** `POST /api/groups/{group}/release` with bounded worker pool. (§8.4)
10. **Liveness sweep:** periodic prune of stale running rows (dead pgid). (§9)
11. **UI — task groups:** collapsible sections, persisted collapse in `layout.json`, Move-to-group picker, Release-group action, group state summary. (§7)
12. **UI — switch-runtime dialog:** interface/backend/model pickers, optional drivers disabled via capabilities, history-handoff note ("model switch keeps native session" / "backend switch primes history"). (§5, §8.5)
13. **UI — terminal affordances:** xterm.js panel attaching to the PTY WebSocket, card "terminal" badge + "Reveal terminal" action. (§3.3, §3.4)

---

## 11. Testing strategy

**Unit / Go**
- AppleScript escaping helper (iTerm2 driver): quotes, backslashes, newlines, shell-quoting of launch argv (table-driven).
- PTY bridge: keystroke in → PTY master write; PTY output → frame out; resize applies `Setsize`.
- `resolveResumeId`: same-backend model swap → native id; cross-backend → empty + handoff=primer; capability flag off → primer.
- Primer synthesis: respects token budget; falls back to truncation when summary call fails; emits `backend_switch` marker.
- Switch rollback: inject `Resume` failure after `Stop` → previous runtime restored, identity reverted, `500 switch_failed_rolled_back`.
- Hook ingest: valid token applies status to `state.db` and emits `state_update`; stale token → `401`; interface gate in `_post.sh` (`AGENTDECK_INTERFACE=chat` → no POST for covered events; `terminal` → POSTs).

**Integration (server + fake CLI / real CLIs where available)**
- **Model swap preserves history on same agent_id:** launch chat Claude agent, run 2 turns, switch model, assert `agent_id` unchanged, prior transcript present in `sessions/{id}/`, new `session_id` in the running row, turn 3 continues. (Acceptance F7.)
- **Interface swap chat↔terminal:** switch to terminal, assert terminal runtime active, `tty` recorded in the running row, status now hook-driven; switch back, chat live again, transcript intact.
- **Backend swap Claude→Codex:** assert handoff=primer, `backend_switch` marker in transcript, new Codex session runs, archive shows one continuous session. (Acceptance F7.)
- **Terminal status via hooks:** drive a terminal agent through a turn; assert the live status row transitions idle→busy→(waiting_input on permission)→idle via hook POSTs, and SSE `state_update` fires for each (badge updates). (Acceptance F2/terminal.)
- **Codex chat end-to-end:** launch/prompt/stream/stop/resume green. (§6 gate.)

**UI / e2e**
- **Group collapse persists:** two agents same group → one collapsible header; collapse, reload → still collapsed. (Acceptance F2.)
- **Move to group:** move an agent via picker → identity updated, card relocates on `state_update`.
- **Release group stops all:** release → all member running rows removed, cards leave; partial-failure path surfaces per-agent error. (Acceptance F2.)
- **Terminal default works cross-platform:** on a host without iTerm2, the terminal option is still offered (xterm default), the agent launches, the xterm panel attaches to the PTY, and status flows via hooks.
- **Optional-driver degradation:** explicitly requesting `iterm2` on a host without it → `422`; the dialog disables that driver with the reason tooltip.

**Manual / macOS-only (optional iTerm2 driver)**
- Real iTerm2: tab created with correct title/color, prompt typed and submitted, Cancel sends SIGINT, Stop closes tab; 12-tab cap returns 429 on the 13th.

---

## 12. Resolved decisions (answers to phase open questions)

1. **Cross-platform terminal default (xterm.js vs tmux).** — **Embedded xterm.js in the UI is the default**, backed by a server-side PTY bridged to the browser over a WebSocket; it works identically on macOS and Linux. **tmux** is offered as an alternative when present (reattachable sessions). **iTerm2/AppleScript is an optional macOS-only extra** selected only when `GET /api/capabilities` reports it available; it is never the core path. All three sit behind the `TerminalDriver` seam, so adding another terminal backend later requires no API changes.

2. **Backend-swap history hand-off semantics.** — **AgentDeck's normalized transcript (`sessions/{agent_id}/`) is the source of truth; native CLI sessions are never translated across backends.** Same-backend model swaps use native resume (CLI keeps its session; only the model arg changes), gated by the adapter's `CanSwitchModelOnResume`. Cross-backend swaps start a fresh native session and inject a **bounded history primer** (running summary of older turns + last N=6 verbatim turns, capped at `switch.primer_token_budget` = 8k tokens) appended to the launch composition only. A `backend_switch` transcript marker records the transition; the logical session (same `agent_id`, same `sessions/` log) continues unbroken. Primer summary failures degrade to truncated verbatim tail rather than blocking the switch.

3. **Concurrency limits with many terminal tabs.** — **Soft cap `terminal.max_tabs` = 12**; launching/switching-to terminal beyond it returns `429 terminal_tab_limit`. Per-agent **switch lock** serializes switch/stop/prompt per agent. Release-group and any batch stop use a **bounded worker pool (4 concurrent)** to limit contention. Every `osascript` call (optional iTerm2 driver) is wrapped in a 4s timeout. A periodic **liveness sweep** prunes stale running rows for dead process groups. (A global cross-interface agent concurrency / launch-queue limit remains a master-PRD-level open question and is intentionally not resolved here.)
