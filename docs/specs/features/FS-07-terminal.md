# FS-07 — Terminal interface

**Status:** Partial
**Code:** `internal/runtime/terminal/`, `internal/server/terminal.go`, `internal/server/switch.go`, `internal/server/launch.go`, `ui/src/components/chat/TerminalTab.tsx` · **Journeys:** J6 (terminal runtime), J7 (stop/resume/switch)
**Absorbed:** [`agent-dashboard-prd.md`](../../archive/agent-dashboard-prd.md) F7/§4.1 and the [phase archive manifest](../../archive/phases/README.md)

## 1. Purpose

A terminal-interface agent runs the backend's **real interactive CLI** inside a server-side PTY,
bridged to an embedded xterm.js panel in the dashboard, instead of speaking ACP. The user types at
a live terminal; status is derived from hooks rather than the ACP stream. The terminal runtime is
the second `Runtime` behind the stable `agent_id`, so an agent can move between chat and terminal
without losing its history (switch-runtime, FS-01). This spec governs launching a terminal agent,
the in-dashboard terminal tab, driver selection, hook-derived status, and the boundaries that keep
terminal agents from becoming statusless or unmessageable.

## 2. Behavior

**Launch & backend eligibility**

- **R1** — Launching with `interface: "terminal"` for a `claude-acp` backend starts the interactive
  `claude` CLI under a pseudo-terminal (`creack/pty`), records the PTY's process group, `tty`,
  driver, and driver ids in the running row, and returns a running agent whose content flows over a
  WebSocket and whose status flows over hooks.
- **R2** — Only `claude-acp` supports the terminal interface. A terminal launch, resume, or switch
  targeting any other backend type (`codex-acp`, `opencode-acp`, `openhands-acp`) is rejected with
  `422 terminal_unavailable` and a reason naming the backend, rather than launching a statusless
  agent that silently drops the composed spec.
- **R3** — A terminal launch composes the same launch spec fields as chat (cwd, add-dirs, system
  prompt / project context, backend/model, env). The composed `--model`, `--add-dir`, and
  `--append-system-prompt` flags are passed to the interactive CLI so a terminal agent runs with the
  chosen model, project directories, and persona — never a bare default. `(planned)` The exact
  Claude CLI flag mapping is gated: it is not verified against a live authenticated CLI (see §6).

**In-dashboard terminal tab (xterm.js / PTY bridge)**

- **R4** — The dashboard renders an xterm.js panel per terminal agent, connected to
  `GET /api/sessions/{id}/terminal/ws`. Browser→server frames are raw keystrokes written to the PTY
  master (binary frames); server→browser frames are PTY output.
- **R5** — A `{cols, rows}` text frame resizes the PTY (`pty.Setsize`), keeping the emulator window
  and the child process's window size in step. The panel re-fits on container and window resize.
- **R6** — The server-side PTY is drained from launch by an always-on per-agent reader, so a
  terminal agent stays live and unblocked even with no browser attached; a UI that never opens the
  tab does not stall the child on a full TTY buffer.
- **R7** — Multiple concurrent viewers of the same terminal agent see identical bytes: each
  WebSocket is a subscriber of the shared per-agent PTY hub. On attach, the current scrollback
  snapshot is replayed before the live stream, so a late viewer sees prior output.
- **R8** — Closing a viewer's WebSocket only unsubscribes it; it never closes the shared PTY master
  or stops the agent. Other viewers and the child process are unaffected.

**Drivers & capabilities**

- **R9** — Three `TerminalDriver` implementations exist behind one seam: the cross-platform
  xterm/PTY driver (default, always available), tmux (offered when `tmux` is on `PATH`), and iTerm2
  (offered only when `runtime.GOOS == "darwin"` and `/Applications/iTerm.app` exists). The registry
  and the runtime never see the driver difference.
- **R10** — `GET /api/capabilities` advertises terminal availability, per-driver availability, and
  the default driver, e.g. `{terminal:{available:true, drivers:{xterm:true, tmux:<bool>,
  iterm2:{available:<bool>, reason?}}, default_driver:"xterm"}}`. An unavailable optional driver
  carries a human `reason` string for a UI tooltip.
- **R11** — The launch and switch APIs accept a `driver` field (`""`/`"xterm"` | `"tmux"` |
  `"iterm2"`). An explicitly requested driver the host lacks is rejected with `422
  terminal_unavailable` and the capability probe's reason. An empty driver selects the
  always-available xterm default. The normal UI does not yet expose optional driver selection (§6).

**Status via hooks (not ACP)**

- **R12** — A terminal agent's status (`idle`/`busy`/`waiting_input`/`done`/`error`, detail,
  last_trace, busy_since, context_pct) is produced solely by hook `POST /api/hook` events applied to
  `state.db` and republished over SSE, identical downstream to ACP-derived status. The terminal
  runtime writes no per-turn status.
- **R13** — The runtime writes an initial `idle` status at launch only if a `SessionStart` hook has
  not already produced a status row within a short race-guard window, and a terminal `done` on Stop
  only if the Stop hook did not fire (e.g. hard kill).
- **R14** — An approval/permission prompt surfaces as `waiting_input` via hooks; the user answers it
  in the terminal itself. There is no ACP permission-relay channel for terminal agents, so a
  programmatic permission decision is not accepted for a terminal agent.

**Prompt delivery, cancel, stop**

- **R15** — `SendPrompt` delivers a prompt over the driver's write path (PTY master write / tmux
  `send-keys` / iTerm2 `write text`). Cancel sends `SIGINT` to the recorded process group. Stop
  sends `SIGTERM` then (after grace) `SIGKILL` to the process group, closes the tab/PTY, removes the
  running row, and leaves the persisted transcript intact.

**Switch, groups, messaging boundaries**

- **R16** — Switch chat↔terminal preserves history on the same `agent_id`: the old runtime stops,
  the identity's interface is persisted, and the target runtime resumes from the durable transcript
  (FS-01 governs the switch algorithm and rollback). Switching to terminal for an unsupported
  backend, or an unavailable requested driver, is rejected before the live agent is torn down.
- **R17** — A terminal agent is a first-class archive/resume citizen: launch and resume open the
  per-agent transcript and upsert the sessions row, so it appears in the archive and can be resumed
  (into either interface).
- **R18** — Terminal agents carry an optional `group` label like chat agents and are stopped by
  `POST /api/groups/{group}/release` alongside chat members.
- **R19** — Terminal agents do not participate in agent-to-agent messaging: the reserved per-session
  messaging MCP is not wired into the interactive CLI, so a terminal agent cannot call
  `send_message`/`check_messages` and is not a messaging peer. This is a deliberate boundary (see
  §6) rather than a bug.

## 3. States & transitions

- **Launch → idle:** driver starts the tab, PTY reader starts, running row written, race-guarded
  initial `idle` scheduled. A `SessionStart` hook may set `idle` first.
- **idle → busy → (waiting_input) → idle/done:** driven entirely by hook POSTs during a turn.
- **Stop:** process group terminated, tab closed, running row deleted, status set `done` (kept for
  the archive/UI). Idempotent; the liveness watcher self-suppresses once stopped.
- **Unexpected child exit (crash):** the runtime's watcher clears the running row, marks the agent
  `done`, drops registry ownership, and closes the PTY hub. A disappeared process becomes `done`
  (not `error`) — see FS-01 and the standing "Immediate/prompt-based UI" decision.
- **Post-restart orphan:** after a dashboard crash where the CLI survives, the runtime does not
  re-adopt the live PID. A subsequent Stop/Switch/Release detects the still-live process group,
  SIGTERM→SIGKILL's it, and clears the row rather than reporting a false success.

## 4. Edge cases & errors

- **R20** — Requesting `iterm2` off macOS (or without iTerm2 installed), or `tmux` without it on
  `PATH`, returns `422 terminal_unavailable` with the probe's reason. An unknown driver name is a
  launch failure, never a silent xterm fallback.
- **R21** — For the tmux driver (no server-side PTY master), the in-dashboard PTY bridge is
  unavailable; the user attaches to the reattachable tmux session directly. `GET
  /api/sessions/{id}/terminal/ws` returns a JSON `not found` error for an agent with no PTY bridge
  rather than a half-open socket.
- **R22** — The terminal WebSocket is subject to the same loopback Host/Origin guard as the rest of
  the API; a cross-origin handshake is rejected with `403` before the socket upgrades.
- **R23** `(planned)` — A soft cap `config.terminal.max_tabs` (default 12) rejecting a terminal
  launch/switch beyond the cap with `429 terminal_tab_limit` is specified but **not implemented**
  (see §6).

## 5. Acceptance criteria

- **A1** — A terminal claude launch records `tty` and transitions idle→busy→idle via hook POSTs.
  *Verified by* `TestTerminalLaunchRecordsTTYAndHookStatusFlow`.
- **A2** — The composed model/add-dirs/system-prompt reach the interactive CLI argv. *Verified by*
  `TestLaunchArgvHonorsComposedSpec`.
- **A3** — A terminal launch creates a sessions row that survives Stop (archive/resume citizen).
  *Verified by* `TestTerminalStartCreatesSessionRowSurvivingStop`.
- **A4** — A codex (or other non-claude) terminal request is rejected `422 terminal_unavailable`.
  *Verified by* `TestCodexTerminalRejected`, `TestNewBackendTerminalRejected`.
- **A5** — An explicitly requested unavailable driver is rejected `422 terminal_unavailable`.
  *Verified by* `TestTerminalDriverUnavailableRejected`.
- **A6** — Switch chat→terminal preserves the transcript on the same `agent_id` and moves status to
  hook-driven. *Verified by* `TestSwitchRuntimeChatToTerminal`.
- **A7** — Stop of a restart-orphaned terminal agent kills the still-live process before clearing
  the row. *Verified by* `TestTerminalStopKillsOrphanedLiveProcess`.
- **A8** — Live xterm panel: launch a terminal agent, type, resize, detach/reattach with output and
  keystrokes intact. *Verified by* journey **J6** (credential/browser gate — see §6).
- **A9** — iTerm2 AppleScript command/argv escaping is correct. *Verified by* `TestShellQuoteArgv`
  and the AppleScript escaping tests in `internal/runtime/terminal/applescript_test.go`.

## 6. Deviations & open decisions

- **Driver selection has no UI (tmux/iTerm2 unselectable in normal use).** The launch and switch
  **APIs** accept `driver` and `GET /api/capabilities` advertises `tmux`/`iterm2`, but the New-Agent
  modal and switch dialog expose only a chat/terminal choice and never send `driver`, so only the
  xterm default is reachable through the UI. `DriverAvailable`'s 422 path for a UI-selected driver is
  therefore unreachable in normal use. This is a tracked defect, not intended behavior; the intended
  behavior (R11) is a capability-gated driver picker. Add a driver field to the launch/switch UI or
  stop advertising the optional drivers.
- **`config.terminal.max_tabs` / `429 terminal_tab_limit` (R23) is unimplemented.** Specified in the
  Phase 6 techspec (§9) but no cap is enforced; recorded here as a deviation until implemented or
  formally dropped.
- **Terminal support boundary.** Claude terminal launches
  receive model/directories/system-prompt flags, but that live CLI mapping is not credential-tested;
  Codex (and other non-claude) terminal launches are rejected; and terminal agents cannot receive
  agent-to-agent messages (R19). This avoids statusless or endlessly nudged agents at the cost of
  advertised interface×backend combinations. Reverse by verifying each CLI's hook/flag/MCP surfaces
  and wiring adapter-specific paths before lifting the gates.
- **Live-CLI terminal acceptance is credential-gated.** Real Claude terminal launch/switch with the
  composed flags and hooks (reconciling any flag mismatch) is an acceptance gate requiring
  credentials (credential-gated acceptance). Until cleared, the Claude interactive-CLI flag/resume
  mapping in `launchArgv`/`interactiveBinary` is `(planned)`-tagged and marked GATED in code.
- **Terminal nudging is excluded.** The terminal runtime retains a `CheckMessages` method, but
  recipient resolution and the nudger exclude terminal agents under FS-06.R9/R17, so coordination
  cannot reach that method in the shipped path.

## 7. Traceability

- **Runtime:** `internal/runtime/terminal/terminal.go` (`Start`/`Resume`/`SendPrompt`/`Cancel`/
  `Stop`/`CheckMessages`, `launchArgv`, `interactiveBinary`, `reconcileOrphanStop`,
  `scheduleInitialIdle`), `capabilities.go` (`Probe`, `DriverAvailable`), `driver.go`, `xterm.go`,
  `tmux.go`, `iterm2.go`, `ptyhub.go`, `bridge.go`.
- **Server seam:** `internal/server/terminal.go` (`terminalSupported`, `validateTerminalDriver`,
  `handleCapabilities`, `handleTerminalWS`), `launch.go` (terminal gate + `driver` field),
  `switch.go` (`handleSwitchRuntime`, driver carry-through, orphan reap), `resume.go` (terminal
  gate).
- **UI:** `ui/src/components/chat/TerminalTab.tsx`, `ui/src/features/launch/NewAgentModal.tsx`
  (interface choice + capability gate), `ui/src/components/grid/AgentCard.tsx` (terminal pill +
  driver label).
- **Error codes:** `internal/runtime/errors.go` — `terminal_unavailable` (422), `no_change` (400),
  `switch_in_progress` (409), `agent_not_running` (409).
- **Key tests:** `TestTerminalLaunchRecordsTTYAndHookStatusFlow`,
  `TestTerminalStartCreatesSessionRowSurvivingStop`, `TestTerminalStopKillsOrphanedLiveProcess`,
  `TestLaunchArgvHonorsComposedSpec`, `TestCodexTerminalRejected`, `TestNewBackendTerminalRejected`,
  `TestTerminalDriverUnavailableRejected`, `TestSwitchRuntimeChatToTerminal`, `TestShellQuoteArgv`.
