# FS-01 — Agent Lifecycle

**Status:** Current
**Code:** `internal/server/{launch,resume,switch,sessions,groups}.go`, `internal/runtime/`, `internal/cli/launch.go` · **Journeys:** J3, J7, J11
**Absorbed:** exact source mapping in the [phase archive manifest](../../archive/phases/README.md)

Covers the full life of an agent: launch, prompt-turn control (stop, cancel), rename, clone, resume,
switch runtime, and crash/restart behavior — all pinned to a single stable `agent_id` (FS-00.R4).
Config-composition *mechanics* (env layering, hook registration, frozen snapshots) belong to the
TS-series; this spec states only the user/API-observable effects.

## 1. Purpose

A user launches agents two ways (modal and CLI) and must get an identical result; then supervises the
running agent (stop it, cancel its current turn, rename it, clone it), brings an inactive one back
(resume), or re-parameterizes a live one without losing its history (switch runtime). Across every one
of these, the agent's identity is stable and its composed config is predictable. When the dashboard or
an agent process crashes, the system converges to an honest state rather than leaking ghost cards or
orphaned processes.

## 2. Behavior

### Launch

- **R1** — An agent launches from either the **New Agent modal** (fields: name, role, project,
  backend, model, interface) or the **CLI** `agentdeck <role>@<project> [flags]`. Both go through
  `POST /api/sessions` and produce an **identical** running agent — a dashboard card plus an openable
  chat/terminal. The modal auto-suggests a name; the CLI form auto-suggests when `--name` is omitted.
- **R2** — The CLI positional splits on the **last** `@`; both `role` and `project` are required, else
  a usage error. Flags: `--backend`, `--model`, `--interface`, `--name`, `--group` (each requires a
  value), `--new` (force a fresh launch), and `--resume <id>` (see R11). A value flag given with no
  operand (e.g. `impl@proj --resume`) is a fast error, never a silent fresh launch.
- **R3** — At launch the server composes the effective config from role + project + backend/model:
  the working directory is `project.cwd`; the system prompt is `project.context_prompt` then
  `role.system_prompt` (blank parts skipped); the model and per-backend/per-model environment come
  from `backends.json`. **Config edits after launch do not affect a live agent** — it keeps its
  composed config, which is frozen into the session snapshot. Ordinary resume and switch preserve
  it; only the explicit federation refresh in R12 re-resolves that source-owned portion.
- **R4** — When no name is supplied, the server assigns the first unused name from a curated wordlist
  (Atlas, Nova, Echo, …), appending a numeric suffix once the list is exhausted.
- **R5** — Optional launch parameters default: `backend` → the backend marked default (else `claude`,
  else any); `model` → that backend's `default_model`; `interface` → `chat`.

### Stop, cancel, rename, clone

- **R6** — **Stop** (`POST /api/sessions/{id}/stop`) terminates the agent's process group, deletes the
  running row, and sets its status to `done` (the status row is kept so the archive/UI can show a
  final state). Stop is **idempotent**: stopping an already-stopped agent whose identity still exists
  returns success, not an error.
- **R7** — **Cancel turn** (`POST /api/sessions/{id}/cancel`) interrupts the agent's in-flight turn.
  The already-streamed events stay persisted. Cancelling an idle agent is a no-op that reports
  `cancelled:false` rather than an error.
- **R8** — **Rename** (`POST /api/sessions/{id}/rename`) changes the display name
  only; the `agent_id` and all other identity fields are unchanged. An empty name is rejected. The
  UI drives rename through a browser prompt.
- **R9** — **Clone** launches a **new** agent (new `agent_id`) carrying the source agent's role,
  project, backend, model, interface, and group. Clone launches **immediately, with no confirmation
  dialog**; the source agent is untouched.

### Resume

- **R10** — **Resume from archive** (`POST /api/sessions/{id}/resume`) restores an inactive agent's
  full history and config and re-attaches a runtime, reusing the same `agent_id`. Resume rebuilds the
  launch from the **frozen session snapshot** (cwd, system prompt, last session id, and the frozen
  `skip_permissions`/`add_dirs`), so a config edit made after the original launch cannot change a
  resumed agent's permission policy or accessible directories. The live identity row supplies
  backend/model/interface (kept current by switch-runtime), so a previously switched agent resumes
  under its current runtime, not a stale one.
- **R11** — **Resume from the CLI**: `agentdeck <role>@<project> --resume <id>` resumes that
  `agent_id` directly. The bare form (`agentdeck <role>@<project>`) resumes when exactly one inactive
  session matches that `role@project`; multiple matches list the candidates and require `--resume` or
  `--new`; no match falls through to a fresh launch.
- **R12** — Resume optionally re-resolves a bound configuration source with the latest native setup
  (`config_refresh:true`); absent/false reproduces the frozen federation object (see FS-08).

### Switch runtime

- **R13** — **Switch runtime** (`POST /api/sessions/{id}/switch-runtime`) changes any subset of
  `interface` (chat ↔ terminal), `backend`, and `model` on a **running** agent, preserving
  conversation history. At least one field must differ from the current identity. The server cancels
  any in-flight turn (bounded wait), stops the current runtime, persists the new identity under the
  **unchanged** `agent_id`, and resumes. The UI drives switch through browser prompts.
- **R14** — History is preserved one of two ways, reported in the response `history_handoff`:
  - **`native_resume`** — a **same-backend** switch (e.g. interface-only, or a model swap the backend
    supports on resume) keeps the CLI's own native session; only the changed argument differs.
  - **`primer`** — a **cross-backend** switch (e.g. Claude ↔ Codex/OpenCode), or a model swap on a
    backend that cannot switch model on resume, starts a fresh native session and injects a bounded
    history primer synthesized from AgentDeck's transcript, appended to the launch composition for
    that resume only (not persisted to the role). A `backend_switch` marker records the transition;
    the logical session (same `agent_id`, same transcript) continues unbroken.
- **R15** — **Switch matrix.** The terminal interface is supported **only** on `claude-acp`; a switch
  (or launch, or resume) that would land the terminal interface on `codex-acp` or any additional
  backend is rejected with `422 terminal_unavailable` rather than producing a statusless agent.
  Chat is supported on every backend. Cross-backend and cross-model switches within chat are allowed.

### Identity

- **R16** — An existing agent's `agent_id` is stable across stop/resume, rename, and every switch
  dimension. Clone creates a distinct agent with a new `agent_id`; the source keeps its id. Only the
  ephemeral CLI `session_id` changes when an existing agent starts/resumes.

## 3. States & transitions

Live status is one of `busy | idle | waiting_input | done | error` (FS-00.R4). Lifecycle-relevant
transitions:

- **R17** — A **chat** agent whose process crashes mid-session (transport closed outside a Stop)
  transitions to `error` with the process's stderr tail as detail, its running row is deleted, and
  the registry drops ownership so the same `agent_id` can be relaunched/resumed.
- **R18** — A **terminal** agent whose process disappears outside a Stop transitions to `done` (not
  `error`) — a deliberate asymmetry with the chat crash path (R17), because a terminal exit is
  indistinguishable from a normal shell exit.
- **R19** — A solicited **Stop** transitions the agent to `done` (R6). A completed turn returns the
  agent to `idle`/its last state; these are FS-03 concerns.
- **R20** — On **server restart**, stale running rows whose PID is no longer alive are reconciled:
  the running row is deleted and the status set to `done` (`"process exited"`),
  so no ghost card survives. A running row whose **PID is still alive** (the agent CLI outlived a
  dashboard crash) is **preserved** as an orphan — the new server does not adopt it into the registry.
- **R21** — A lifecycle action (Stop, Switch, Release group) on such an orphan **reaps** it: because
  the registry has no handle, the server checks the running row, SIGKILLs the live PID, and deletes
  the row — so Stop/Release report success only after the process is actually gone, and Switch cannot
  spawn a second process under the same `agent_id`.

## 4. Edge cases & errors

- **R22** — Launch validation returns `400`/`422` naming the offending field for: missing role or
  project, unknown role/project/backend/model, or an invalid interface. A project whose resolved
  `cwd` does not exist is rejected up front with a message naming the directory and project — not a
  deep fork/exec error that blames the adapter binary.
- **R23** — A terminal launch/resume/switch onto an unsupported backend returns `422
  terminal_unavailable` with a reason (R15). This gate lives in one helper shared by the launch,
  resume, and switch composers so the three paths cannot drift.
- **R24** — An explicitly requested terminal **driver** (`xterm`/`tmux`/`iterm2`) that is unavailable
  on the host returns `422 terminal_unavailable` with a reason; the always-available xterm default
  passes. Chat launches ignore the driver field.
- **R25** — Resume errors: resuming an already-running agent returns `409` (running row present);
  resuming an agent with no persisted session snapshot returns `422`; unknown `agent_id` returns
  `404`. A failed resume tears down all registration artifacts (hook token, MCP session, hook
  settings file) so nothing is left behind.
- **R26** — Switch errors: a request equal to current state returns `409 no_change`; a switch on a
  non-running agent returns `409 agent_not_running`; a concurrent switch on the same agent returns
  `409 switch_in_progress`. If the target Resume fails, the server **rolls back** to the previous
  identity (re-registers and re-resumes it) and returns `switch_failed_rolled_back`; if rollback
  itself fails, status is set to `error` and the agent remains recoverable via archive resume.
- **R27** — Stop on an unknown `agent_id` (no identity row) returns `404`. Cancel on an unknown agent
  returns `404`.
- **R28** — Launch/resume/switch never leak a spoofable messaging identity: if identity write or
  runtime Start fails, the server rolls back the identity row and every registration artifact.

## 5. Acceptance criteria

- **A1** — Modal and CLI launch produce an identical running agent. *Verify:* CLI parse
  `TestParseLaunch`, `TestParseLaunchNewAndResumeFlags`, `TestParseLaunchErrors`; journey **J3**.
- **A2** — Config composition is observable and correct (system prompt order, env layering, skip
  resolution) and frozen against later edits. *Verify:* `TestJoinSystemPrompt`,
  `TestComposeEnvLayering`, `TestResolveSkip`, `TestResumeAndSwitchUseFrozenSkipAndAddDirs`; journey
  **J7**.
- **A3** — A launch against a missing project directory fails up front naming the directory. *Verify:*
  `TestComposeLaunchRejectsMissingCwd`.
- **A4** — Stop is idempotent and reaps a live orphan after a restart. *Verify:*
  `TestStopReapsOrphanRuntimeAfterRestart`, `TestChatStopKillsOrphanedLiveProcess`.
- **A5** — Cancel interrupts an in-flight turn and is a no-op when idle. *Verify:* `TestRealCLICancel`,
  `TestCancelDuringPendingPermission`, `TestCancelEscalatesToSIGINT`.
- **A6** — Rename changes the name and nothing else. *Verify:* `TestRenameSession`.
- **A7** — Resume restores identity, model, system prompt, and add_dirs, observed from UI and process.
  *Verify:* `TestResumeAndSwitchCarryRoleAndProjectFields`, `TestResumeTerminalAgent`,
  `TestResumeFailureRemovesHookSettings`; journey **J7**.
- **A8** — Same-backend switch uses native resume; cross-backend uses a primer and keeps one
  continuous session; codex/other-backend terminal is rejected. *Verify:*
  `TestSwitchRuntimeModelSwapSameBackend`, `TestSwitchRuntimeChatToTerminal`,
  `TestSwitchRuntimeBackendSwapUsesPrimer`, `TestSwitchClaudeToOpenCodePrimer`,
  `TestCodexTerminalRejected`, `TestNewBackendTerminalRejected`; journey **J7**.
- **A9** — Switch rejects a no-op and rolls back a failed resume without leaking registration.
  *Verify:* `TestSwitchRuntimeNoChange`, `TestSwitchRuntimeRollbackOnResumeFailure`,
  `TestSwitchRuntimeKeepsTargetRegistration`.
- **A10** — A crashing agent tears down registration and is reflected in the card. *Verify:*
  `TestCrashMidTurn`, `TestCrashTearsDownAgentRegistration`, `TestRegistryForgetsAgentAfterCrash`;
  journey **J11**.
- **A11** — On restart, stale (dead) running rows become `done` and ghost-free. *Verify:*
  `TestReconcileStale`.
- **A12** — A terminal agent's process disappearing marks it `done`, not `error`. *Verify:* manual
  gate (terminal `startWatcher` path) / journey **J6**.

## 6. Deviations & open decisions

- **Terminal support boundary.** Claude terminal launches
  receive model/directory/system-prompt flags, but that live CLI mapping is not credential-tested;
  Codex (and additional-backend) terminal launches are rejected (R15); and terminal agents cannot
  receive agent-to-agent messages. This avoids statusless or endlessly-nudged agents at the cost of
  advertised combinations. Reverse by verifying each CLI's hook/flag/MCP surfaces.
  Live Claude terminal flag mapping is a credential-gated acceptance (FS-07.A8).
- **Runtime-switch fallbacks.** Cross-backend context uses
  local transcript truncation rather than a live target-model summary; cancellation polls status for a
  hardcoded ~5 seconds before stopping; the live identity updates before the archived snapshot; and a
  switch on a stopped identity returns `409 agent_not_running`. These are user/API-visible
  interoperability choices.
- **Immediate/prompt-based UI.** Clone launches immediately
  with no confirmation (R9); runtime/group changes use browser prompts (R8, R13); a disappeared
  terminal process becomes `done` not `error` (R18); and an invalid seeded project is explained after
  launch fails (R22, mitigated by the up-front cwd check).
- **Agent env inheritance by design.** Child agents inherit
  the full server environment minus each backend's stripped keys, so unrelated host credentials are
  visible to agents. Reverse by defining a per-backend env allowlist. (See TS-05.)
- **Real Codex chat resume is credential-gated (credential-gated acceptance).** Codex chat launch,
  turn, stop, and resume, and reconciling model/resume/hook behavior with real credentials, are
  acceptance gates (FS-09.A7) before a release claims that live-CLI compatibility.

## 7. Traceability

- Launch compose + rollback: `internal/server/launch.go` (`composeLaunch`, `handleLaunch`,
  `teardownAgentRegistration`, `reapOrphanRuntime`).
- Resume: `internal/server/resume.go` (`handleResume`, `composeResumeSpec`).
- Switch + rollback + matrix: `internal/server/switch.go` (`handleSwitchRuntime`, `rollbackSwitch`,
  `validateSwitchTarget`), `internal/server/terminal.go` (`terminalSupported`).
- Stop/cancel/rename/clone + orphan reap: `internal/server/sessions.go`, `internal/server/groups.go`,
  `ui/src/components/grid/CardContextMenu.tsx`.
- CLI launch/resume forms: `internal/cli/launch.go` (`parseLaunch`, `runLaunch`).
- Crash/reconcile/done-vs-error: `internal/runtime/chat.go` (`onTransportClosed`),
  `internal/runtime/terminal/terminal.go` (`startWatcher`, `setDone`), `internal/runtime/reconcile.go`
  (`ReconcileStale`).
- Bug classes guarded: INV §2 (identity across resume/switch), §4 (orphan reaping / liveness), §6
  (LaunchSpec contract + capability honesty).
