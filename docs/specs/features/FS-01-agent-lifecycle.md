# FS-01 — Agent Lifecycle

**Status:** Partial
**Code:** `internal/server/{launch,resume,switch,sessions,groups}.go`, `internal/runtime/`, `internal/index/`, `internal/cli/launch.go` · **Journeys:** J3, J7, J11
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
  backend, model, interface) or the **CLI** `chuck <role>@<project> [flags]`. Both go through
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
  (Atlas, Nova, Echo, …), appending a numeric suffix once the list is exhausted. A supplied display
  name is trimmed, must contain non-whitespace text, must not contain NUL, and is limited to 256
  Unicode characters. The same display-name contract applies to rename and identity updates.
- **R5** — Optional launch parameters default: `backend` → the backend marked default (else `claude`,
  else any); `model` → that backend's `default_model`; `interface` → `chat`.
- **R30** — Launch accepts an optional **effort** alongside backend and model, from the
  New Agent modal, the `POST /api/sessions` body, and the CLI flag `--effort` (which requires a
  value like the other launch flags in R2). Its allowed values, resolution order, and rejection
  rules belong to the selected model's declared capability (FS-09.R35/R41/R42). The resolved effort
  joins backend and model as part of the agent's runtime identity: it is reported on the session
  response and agent record, shown beside backend and model on the chat and archive headers,
  restored by resume (R11), carried by clone (R9), and changeable on a running agent through switch
  runtime (R13) under the same native-resume versus primer rules a model swap follows. Switch runtime
  keeps accepting effort unchanged; FS-03.R47 adds a second, cheaper way to change it on
  a running chat agent that does not restart the process.
- **R35** — Launch accepts an optional **fast mode** alongside backend,
  model, and effort, from the New Agent modal, the `POST /api/sessions` body, and the CLI flag
  `--fast` (a boolean flag taking no operand, unlike `--effort`). Whether it is offered at all, how
  it resolves, and what happens when the live session cannot honor it belong to the selected model's
  declared capability (FS-09.R50/R54/R55). The **applied** fast mode joins backend, model, and
  effort as part of the agent's runtime identity: it is reported on the session response and agent
  record, shown on the chat and archive headers, restored by resume (R11), and carried by clone
  (R9). It is the one part of that identity **not** changed through switch runtime (R13): fast mode
  changes on a running agent through the chat-header toggle (FS-03.R45) instead, because the
  providers accept it as an ordinary session setting while switch runtime stops and restarts the
  CLI. A launch that requests fast mode for a terminal agent is rejected, because no Claude
  terminal fast-mode mechanism exists (FS-09.R52).

- **R31.** A **stopped** agent is an inactive, non-archived agent whose project is
  active: it remains visible on that project dashboard, retains its final live-status outcome, and
  can Resume under R10. Stopping never hides an agent from dashboard supervision. Individual agent
  archival removes only that agent; project archival archives every agent in the project (FS-05.R32–
  R36). R10 applies only to a non-archived inactive agent: an individually archived agent must be
  restored before Resume, and an agent whose project is archived cannot restore or resume until that
  project is reactivated.

- **R37 (shipped 2026-09-26) — Compact New Agent setup.** The modal in R1 leads with role and project;
  a project-scoped launch displays its fixed project without adding another chooser. A compact
  runtime summary shows the selected backend/model, effort when set, fast mode when enabled, and
  interface. One directly reachable **Options** disclosure contains the suggested editable name
  and the backend/model/effort/fast/interface controls. Default launches require no disclosure
  interaction. Labels use configured display names, adding ids only to disambiguate equal names;
  selection and submission still use the original ids. Existing defaults, name suggestions,
  capability gates, model/effort reset rules, source warnings, and launch payload semantics remain
  unchanged. Opening or closing Options changes no value, and a rejected launch retains entered
  values; errors associated with a hidden control reveal it. Source-health and runtime-compatibility
  warnings remain visible outside the disclosure. This refines R1's presentation, not the launch,
  native-configuration inheritance, or CLI contract.

- **R46 (planned) — Launch without a specialized persona.** An operator can select **Default**
  in ordinary desktop and phone New Agent setup and onboarding to launch an agent with an empty
  persona prompt. The agent still receives project context, the shared Chuck operating context
  and operating-knowledge delivery under FS-18.R15, and native provider guidance under FS-18.R17;
  empty persona text does not imply an empty effective prompt. Specialized and custom roles stay
  selectable. Default selection follows FS-04.R54 once its upgrade decision is confirmed; an
  explicit role selection takes precedence. The resulting agent is an ordinary agent under the
  existing identity, permissions, history, stop, resume, clone and switch contracts. This feature
  does not change the roles of existing agents or explicit task, pipeline or Think Tank assignments.
  It supersedes R37's default-role behavior only as specified by FS-04.R54.

### Stop, cancel, rename, clone

- **R6** — **Stop** (`POST /api/sessions/{id}/stop`) terminates the agent's process group, deletes the
  running row, and sets its status to `done` (the status row is kept so the archive/UI can show a
  final state). Stop is **idempotent**: stopping an already-stopped agent whose identity still exists
  returns success, not an error.
- **R7** — **Cancel turn** (`POST /api/sessions/{id}/cancel`) interrupts the agent's in-flight turn.
  The already-streamed events stay persisted. Cancelling an idle agent is a no-op that reports
  `cancelled:false` rather than an error. If the peer ignores cooperative cancellation and the
  fallback interrupt ends its process, the resulting dead/error state identifies Cancel as the
  cause rather than presenting a generic process exit (FS-03.R9).
- **R8** — **Rename** (`POST /api/sessions/{id}/rename`) changes the display name
  only; the `agent_id` and all other identity fields are unchanged. Invalid names are rejected under
  R4. The UI drives rename through the core application dialog described by R32.
- **R9** — **Clone** launches a **new** agent (new `agent_id`) carrying the source agent's role,
  project, backend, model, interface, and group. Clone launches **immediately, with no confirmation
  dialog**; the source agent is untouched. Superseded by R36: Clone is now a conversation fork, and
  the settings-only meaning no longer ships.
- **R36** — **Clone** is a conversation fork, not a settings duplicate. It creates a
  new Chuck agent with a new `agent_id`, the source agent's role, project, backend, model,
  interface, effort, fast mode and group, and a provider-native conversation fork at the source's
  latest completed turn. The new agent receives the provider context and a durable copy of the
  source's visible transcript through that boundary plus a fork marker linking back to the source;
  later source events never enter the clone. The source is untouched and the clone launches
  immediately without a confirmation dialog. Clone is available only for a chat runtime that
  advertises native session fork and has a native session to fork; Chuck never silently falls
  back to the superseded settings-only meaning. While the source has an active turn or unresolved
  permission, Clone is unavailable with an explanation that cloning requires a completed
  conversation point. A stopped, non-archived source remains cloneable from its last native
  session. Background commands remain owned by the source and do not continue in the clone; any
  copied running task row is closed at the fork boundary as not carried. A failed fork creates no
  new agent, transcript, running process, or messaging identity.
- **R39 (shipped 2026-10-09)** — A clone is named after its source: the source's display name
  followed by ` Copy` (a clone of "Atlas" is "Atlas Copy"; cloning that gives "Atlas Copy Copy").
  The source name is shortened when needed so the result stays within R4's 256-character limit.
  A source with no display name falls back to R4's curated suggestion.
- **R38 (shipped 2026-09-26)** — A successful Clone action in the dashboard opens the newly created agent's
  conversation immediately, so the person lands on the copied transcript and fork marker instead
  of remaining on the source card. The client routes only after the fork response returns the new
  `agent_id`; a refused or failed clone leaves the person on the source surface and exposes the
  existing actionable error.

### Resume

- **R10** — **Resume from archive** (`POST /api/sessions/{id}/resume`) restores an inactive agent's
  full history and config and re-attaches a runtime, reusing the same `agent_id`. Resume rebuilds the
  launch from the **frozen session snapshot** (cwd, system prompt, last session id, and the frozen
  `skip_permissions`/`add_dirs`), so a config edit made after the original launch cannot change a
  resumed agent's permission policy or accessible directories. The live identity row supplies
  backend/model/interface (kept current by switch-runtime), so a previously switched agent resumes
  under its current runtime, not a stale one.
- **R11** — **Resume from the CLI**: `chuck <role>@<project> --resume <id>` resumes that
  `agent_id` directly. The bare form (`chuck <role>@<project>`) resumes when exactly one inactive
  session matches that `role@project`; multiple matches list the candidates and require `--resume` or
  `--new`; no match falls through to a fresh launch.
- **R12** — Resume optionally re-resolves a bound configuration source with the latest native setup
  (`config_refresh:true`); absent/false reproduces the frozen federation object (see FS-08).
- **R33** — **Wake on message.** Sending a prompt to a stopped chat agent that passes
  the **wake gates** transparently performs the same restore as R10 — same frozen snapshot, same
  live identity row, no `config_refresh` — and then delivers the prompt in the woken session,
  instead of returning the non-running error. The wake gates are exactly R10's shipped resume gates
  — the agent is not archived, its project is not archived (a missing project definition does not
  block, matching FS-05.R34), a persisted session snapshot exists, and the interface is chat. An
  agent's pipeline history is not a wake gate (FS-14.R74).
  Explicit Stop (R6) therefore acts as a lightweight sleep for a chat conversation: any later
  message revives it, and Stop remains the way a person reclaims an idle agent's process memory. A
  failed wake surfaces the applicable typed resume error (R25), leaves the agent stopped, and tears
  down only the wake's own registration artifacts exactly as a failed explicit resume does. Agents
  the wake gates exclude keep their existing rejection behavior unchanged. A gate that cannot be
  **evaluated** — the candidacy lookup fails, or the project definition is unreadable — is not a
  decision that the agent is unwakeable: it surfaces the same typed failure an explicit resume
  returns, rather than the ordinary not-running rejection, and it fails the messaging directory read
  rather than silently omitting the agent.

### Switch runtime

- **R13** — **Switch runtime** (`POST /api/sessions/{id}/switch-runtime`) changes any subset of
  `interface` (chat ↔ terminal), `backend`, and `model` on a **running** agent, preserving
  conversation history. At least one field must differ from the current identity. The server cancels
  any in-flight turn (bounded wait), stops the current runtime, persists the new identity under the
  **unchanged** `agent_id`, and resumes. The UI drives switch through the core application dialog
  described by R32.
- **R14** — History is preserved one of two ways, reported in the response `history_handoff`:
  - **`native_resume`** — a **same-backend** switch (e.g. interface-only, or a model swap the backend
    supports on resume) keeps the CLI's own native session; only the changed argument differs.
  - **`primer`** — a **cross-backend** switch (e.g. Claude ↔ Codex/OpenCode), or a model swap on a
    backend that cannot switch model on resume, starts a fresh native session and injects a bounded
    history primer synthesized from Chuck's transcript, appended to the launch composition for
    that resume only (not persisted to the role). A `backend_switch` marker records the transition;
    the logical session (same `agent_id`, same transcript) continues unbroken.
- **R15** — **Switch matrix.** The terminal interface is supported **only** on `claude-acp`; a switch
  (or launch, or resume) that would land the terminal interface on `codex-acp` or any additional
  backend is rejected with `422 terminal_unavailable` rather than producing a statusless agent.
  Chat is supported on every backend. Cross-backend and cross-model switches within chat are allowed.
- **R32** — The UI drives **Rename** (R8) and **Switch runtime** (R13) through core
  application dialogs rather than browser prompts (FS-12.R26). Rename opens a single-field form
  validated by R4. Switch runtime opens one form whose `interface`, `backend`, and `model` selects
  are populated from the backend catalog (terminal gated by capability per R15), requires at least
  one field to differ from the current identity before it can submit, and performs no switch when
  cancelled. Both submit the same requests as today.

### Identity

- **R16** — An existing agent's `agent_id` is stable across stop/resume, rename, and every switch
  dimension. Clone creates a distinct agent with a new `agent_id`; the source keeps its id. Only the
  ephemeral CLI `session_id` changes when an existing agent starts/resumes.

### Quota interruption and automatic continuation

- **R40 (planned)** — **Quota interruption is system state.** Every chat agent whose executing
  turn is interrupted by a provider usage quota shows **Quota reached** on its dashboard card and
  conversation, independently of the process being live or stopped. This includes ordinary chats,
  task assignees, pipeline agents and all Think Tank participant/private/closing/judge turns.
  Detection requires provider-owned terminal quota evidence; arbitrary assistant/user/tool text,
  an advisory warning, an adapter's active automatic retry, a transient rate limit, context/output
  limits and Chuck's own concurrency limits do not establish quota interruption. A known reset is
  shown as a local date/time with timezone; missing or ambiguous evidence reads **Reset time
  unknown**, never an invented countdown. Each affected chat is tracked independently, including
  several agents using the same provider quota.
- **R41 (planned)** — **Auto continue schedules the interrupted conversation.** With the global
  setting in FS-04.R53 on, a known future quota reset creates one durable scheduled continuation
  for that agent's interrupted work, visible in its conversation as **Will continue at …**.
  A repeated observation of the same interruption creates no duplicate. The schedule lives locally
  with Chuck's machine state and survives server/browser restarts. It does not create another
  conversation, duplicate a durable task, or claim that unfinished work has completed. A person can
  cancel that pending continuation in the conversation without disabling the global feature.
  Unknown reset times keep the quota indicator and explain that automatic continuation cannot yet
  be scheduled. Later trustworthy reset evidence for that same interruption can schedule it.
- **R42 (planned)** — **Continuation preserves the work and its owner.** At or after the reset,
  Chuck continues the same conversation through its ordinary prompt/resume path, with its current
  runtime identity and frozen permission/configuration rules. It asks the agent to continue the
  interrupted work from its existing history, rather than replaying the original request or tools.
  Logical agent/provider-session history is preserved; an ordinary process resume may replace
  the runtime generation, and continuation always has its own executing turn identity.
  A durable assignment keeps its task and assignee; pipeline work keeps its current run/stage
  assignment; Think Tank work keeps its room, participant and unfinished opportunity. Each owner
  authorizes its continuation under its existing exclusivity, capacity and closure rules. Quota
  interruption writes no result, consumes no completed room allowance and does not advance a stage
  or room. Already accepted task/stage results and published/withheld room submissions are not
  repeated. A staged but unpublished failed room submission remains historical; continuation must
  read current room state and make a valid submission under its new turn authority, never publish
  a failed turn's staged text as though that turn succeeded. Waiting for a busy agent, lifecycle
  action or available capacity delays the schedule
  visibly, without starting overlapping turns or turning that delay into a failure.
- **R43 (planned)** — **Manual control supersedes pending automation.** Disabling Auto continue
  cancels every pending quota continuation, including one waiting for admission; it stops no turn
  already admitted. A successful manual Send/Steer, Cancel/Stop, Resume/Retry/Continue, runtime
  change, conversation archive/deletion, or relevant task/run/room control that supersedes or closes
  the interrupted work cancels its schedule. Refused operations do not erase it. Re-enabling the
  setting applies to later quota interruptions and does not resurrect cancelled schedules.
  Each cancellation and dispatch is serialized so only one can win; no stale timer revives work
  after the person's intervention. Normal resume/project/archive eligibility still applies.
- **R44 (planned)** — **Reset recovery is bounded and honest.** Chuck must be running to dispatch
  a continuation. After restart it admits overdue schedules only after checking that the original
  work is still unfinished and eligible; it does not replay an uncertain pre-crash delivery.
  A newly observed quota interruption can schedule its newly reported future reset, with at most
  one pending continuation per agent. An unchanged/past reset never creates a retry loop. A
  non-quota failure or uncertain delivery leaves an actionable needs-attention reason and no
  automatic retry; an unavailable/archived target cannot silently create replacement work.
  Passing the reset time alone does not claim the quota has cleared. Successful continuation
  clears the interruption indicator; a fresh quota failure updates it and its next known reset.
- **R45 (planned)** — **The chat attributes automatic continuation to Chuck.** Once continuation
  actually begins, the conversation gains exactly one durable system message:
  **Chuck continued this conversation when the quota reset**. It is attributed to Chuck, never
  to the person or agent, and replays in live, archived and phone transcripts in order. Merely
  scheduling, passing the reset time, losing admission or failing to start does not emit it.
  Reconnect/restart cannot duplicate it. Existing transcript retention and search treatment of
  system notices remain unchanged.

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
- **R21** — A lifecycle action (Stop, Switch, project group Stop/Archive) on such an orphan **reaps** it: because
  the registry has no handle, the server checks the running row, SIGKILLs the live PID, and deletes
  the row — so Stop/Release report success only after the process is actually gone, and Switch cannot
  spawn a second process under the same `agent_id`.

## 4. Edge cases & errors

- **R22** — Launch validation returns `400`/`422` naming the offending field for: missing role or
  project, an invalid supplied name, unknown role/project/backend/model, or an invalid interface. A
  project whose resolved `cwd` does not exist is rejected up front with a message naming the
  directory and project — not a deep fork/exec error that blames the adapter binary.
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
- **R29** — Launch and resume remain available when an FTS5-created home is opened by a build whose
  SQLite driver cannot load FTS5. The authoritative session row and lifecycle state still commit,
  derived search-document writes are skipped until an FTS5-capable build returns, and no raw SQLite
  module error reaches the launch/resume response.
- **R34** — **Stop and resume are one exclusive transition per agent.** Stop (R6), project group
  Stop/Archive (FS-02.R75–R76), explicit Resume (R10), and every wake (R33) take the same exclusive per-agent
  lifecycle claim, so an agent is never being started and stopped at once. Stop and group Stop
  run one shared stop-and-teardown seam rather than two spellings of it, so the claim cannot be
  bypassed through whichever verb omits it. A stop that arrives while a resume or wake holds the
  claim returns `409 conflict` instead of reporting success, and may be retried once the transition
  settles (the client is not obliged to retry automatically; TS-03.R25). Group actions reserve every
  member's claim before stopping any of them, so one busy member rejects the whole action with `409`
  and leaves every member running. Either way the in-flight resume completes normally with its
  registration intact. Without this, a stop could not
  tell a resume in progress from a stopped agent, so it took its idempotent already-stopped path and
  removed the running resume's hook token, MCP session, and hook-settings file while that resume went
  on to report success. Stop remains idempotent (R6) and still returns `404` for an unknown agent
  (R27) whenever it holds the claim.

## 5. Acceptance criteria

- **A1** — Modal and CLI launch produce an identical running agent, and every lifecycle entry point
  applies the bounded display-name contract. *Verify:* CLI parse `TestParseLaunch`,
  `TestParseLaunchNewAndResumeFlags`, `TestParseLaunchErrors`; server
  `TestNormalizeAgentName`, `TestComposeLaunchRejectsInvalidName`; journey **J3**.
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
- **A6** — Rename changes the name and nothing else while sharing launch's validation. *Verify:*
  `TestRenameSession`, `TestNormalizeAgentName`.
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
- **A13** — Simulated missing-FTS5 write capability preserves session metadata and turn rollups
  while skipping derived documents without error: `internal/index/indexer_test.go::TestIndexerDegradesWhenFTS5WritesAreUnavailable`.

- **A14** (R30) — A modal launch, an API launch, and a CLI `--effort` launch of the same
  role/project/backend/model produce an identical agent carrying the same resolved effort; `--effort`
  with no operand is a usage error; the resolved effort appears on the session response, the chat
  header, and the archive header; resume and clone carry it; and switch runtime changes it on a
  running agent with history preserved. *Verify by* launch/CLI parity tests, switch-runtime tests,
  `NewAgentModal.test.tsx`, and the chat/archive header UI tests.

- **A15.** Stopping an agent leaves it visible and resumable on its project dashboard;
  its final status and stable identity survive restart. — lifecycle/dashboard regressions; J5, J7,
  J12.
- **A16** (R32) — The rename and switch-runtime dialogs open from the card menu, validate input,
  reject a no-op switch, submit the same requests as before, and leave the agent unchanged on Cancel.
  *Verify:* `CardContextMenu` component tests and the FS-12.A8 source guard.
- **A17** (R33) — A launched-then-stopped fake-ACP chat agent receives a prompt: the
  agent resumes (running row restored, same `agent_id`, frozen snapshot values reapplied) and the
  prompt's turn streams normally, including for a stopped agent with a pipeline attempt
  association. An individually archived agent and an agent with no snapshot keep their existing
  rejections; a wake whose
  resume stage fails returns the typed resume error with teardown of that wake's artifacts; and
  simultaneous prompt, mail, and explicit-resume wakes on one agent produce exactly one process,
  conflict errors for the losers, and an intact winner registration (working hook token, MCP
  registration, and hook-settings file). *Verify:* server integration tests on the prompt route
  against fake ACP, including the failure and concurrency branches.
- **A18** (R33–R34) — A Stop issued while a wake is inside its resume returns `409`, the wake
  completes with its hook token, MCP registration, and hook-settings file intact, and the retried
  Stop then succeeds — one coherent final state, never a running process whose registration was
  torn down. A Release group covering that agent returns `409` without stopping any group member;
  the wake keeps its registration and the retried release stops the whole group. A stopped agent whose
  project definition is unreadable answers a prompt with the typed internal failure rather than the
  not-running `404`. *Verify:*
  `internal/server/wake_test.go::TestStopDuringWakeConflictsAndKeepsRegistration`,
  `TestReleaseGroupDuringWakeKeepsRegistration`, and `TestWakeGateFailureSurfacesTypedError`.

- **A19** (R35) — A modal launch, an API launch, and a CLI `--fast` launch
  of the same role/project/backend/model produce an identical agent carrying the same applied fast
  mode; `--fast` takes no operand; the applied fast mode appears on the session response, the chat
  header, and the archive header; resume and clone carry it; a terminal launch requesting it is
  rejected; and switch runtime leaves it unchanged. *Verify by* launch/CLI parity tests,
  resume/clone tests, switch-runtime tests, `NewAgentModal.test.tsx`, and the chat/archive header UI
  tests.

- **A20** `(planned)` (R36) — Clone against a fork-capable fake ACP session at idle creates one
  running agent with a distinct `agent_id`, identical configured identity, a distinct native
  session, copied visible history through the last completed turn, and a source-link marker; a
  later source turn appears only on the source. The same action succeeds from a stopped source by
  reopening its native session for the fork. A busy or waiting source, a terminal or non-advertising
  runtime, and a source without a native session expose no weaker clone path, while a rejected fork
  leaves no partial Chuck state. *Verify by* lifecycle/runtime/API tests and card-menu tests for
  capability, state, success and rollback, plus journey J7.

- **A21 (shipped 2026-09-26)** (R37) — Launch from both global and fixed-project entry points without opening
  Options, then customize name and runtime and launch again. Assert the request values match the
  existing default/custom contracts; toggling Options changes none. Duplicate display names remain
  distinguishable; unsupported effort/fast/terminal controls stay gated; source warnings remain
  visible and a refused launch preserves values and exposes the relevant control. *Verify by*
  `NewAgentModal.test.tsx` and the rendered setup pass in FS-12.A24.

- **A22 (shipped 2026-09-26)** (R38) — Activating Clone from an eligible dashboard card calls only the fork
  route and, after success, opens `/agent/<new-agent-id>`; a failed fork stays on the source surface
  and keeps the existing error feedback. *Verify:* `CardContextMenu.test.tsx`.

- **A23 (planned)** (R40–R41) — Drive terminal provider quota evidence in ordinary chats and
  concurrently in several agents sharing a backend. Each card/chat shows its own quota reason,
  trustworthy reset time and exactly one schedule. Try arbitrary prose, advisory warnings,
  retrying-provider errors, transient 429s and unknown/ambiguous reset times; none falsely schedules.
  *Verify:* provider-boundary fixtures, state/API tests and a focused desktop/phone browser journey.
- **A24 (planned)** (R41–R42, R45) — Reach a known reset in a partially completed ordinary chat,
  once with the runtime live and once stopped. Continue the same identity/history without replaying
  the original request; append the exact Chuck message once only after work begins. Reload live,
  Archive and phone transcripts; ordering, attribution and retained history agree. *Verify:*
  controllable-clock fake-provider integration tests, transcript/component tests and A23's journey.
- **A25 (planned)** (R42) — Quota-interrupt an ordinary task, standing/coordinator pipeline work,
  delegated work and each Think Tank phase (opening, discussion, private, closing and judge),
  including a room-backed pipeline stage. Reset resumes only the unfinished owned opportunity in
  the same conversation; task/assignee/room/run provenance is retained, no outcome/allowance/stage
  advances on quota, and accepted/published/withheld work is never duplicated. *Verify:*
  owner-state and fake-provider integration tests with post-submission quota failure cases.
- **A26 (planned)** (R41, R43) — Race reset admission against cancelling one schedule, global
  disable, successful/refused manual input/control, runtime switch, archive/deletion and owner
  closure. Only the winning eligible action takes effect, failed manual actions retain recovery,
  and re-enable does not revive a cancelled schedule. *Verify:* state/lifecycle/API race tests and
  A23's cancellation/settings journey.
- **A27 (planned)** (R42, R44–R45) — Restart before reset, after reset and around admission/provider
  delivery/notice append. Eligible overdue work continues; uncertain delivery holds for attention;
  busy/capacity deferral does not spend failures; no duplicate turn or system message occurs.
  New quota evidence with a future reset replaces the wait; stale times and non-quota errors stop
  automatic retry with a readable reason. *Verify:* fake-clock restart/failure-injection tests.
- **A28 (planned)** (R40–R45) — Exercise packaged and supported installed Claude/Codex quota
  signals through ordinary and owner-activation prompt paths. Verify terminal classification,
  reset provenance/timezone and actual continuation using captured provider wire shapes, plus a
  bounded credentialed provider check when available. Record unknown reset support honestly;
  fake output is not proof of provider delivery. *Verify:* pinned-adapter contract fixtures and a
  recorded live-provider gate before claiming provider coverage.

- **A29 (planned)** (R46, FS-04.R54) — Launch Default from desktop global/project setup, phone
  project setup and onboarding, then launch an explicitly selected specialized/custom role. Check
  the selected role in the request and resulting agent, an empty Default persona contribution,
  retained project/shared/native guidance, and the existing permission policy. Stop/resume, clone
  and switch a Default agent under their normal availability rules; confirm its saved persona
  choice is retained. *Verify:* focused launch/onboarding/phone component tests, server composition
  and lifecycle tests, provider instruction-delivery contracts, and one rendered launch journey.

## 6. Deviations & open decisions

- **Persona-free launch scope awaiting confirmation:** R46/A29 are a feature-side draft only.
  Proposed Default is an empty ordinary persona; FS-04.R54 owns default selection and the pending
  upgrade decision. CLI/API can select it through their existing explicit-role launch contracts;
  omitting the API role or introducing new CLI syntax is outside this draft. No technical design
  or implementation-ready change exists yet.

- **Group API replacement (shipped 2026-10-10):** FS-02.R75–R76 and TS-03.R57–R58 replace the
  dashboard's global Release group with project-scoped Stop group and Archive group. R21/R34's
  orphan cleanup, shared stop seam and all-member conflict guarantees continue under those routes;
  the old HTTP release route is removed. No new lifecycle path bypasses these claims.

- **Pipeline replacement:** FS-14.R74 (shipped 2026-09-13) replaced R33's historical
  pipeline-association wake veto with current ownership; R33 and A17 were reconciled on
  2026-09-28. TS-10.R28–R30 route waits and resumes
  through existing lifecycle seams; TS-10.R32 adds a generation/turn-guarded cancellation call for
  borrowed task turns. Normal identity, snapshot, archive, project and orphan checks remain.

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
- **Immediate/dialog-based UI.** Clone launches immediately with no confirmation under current R9;
  planned R36 preserves that interaction once its stable fork boundary is available. Rename and
  switch runtime use the application dialogs specified by
  R32/FS-12.R26; a disappeared
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
- Resume: `internal/server/resume.go` (`handleResume`, `resumeSession`, `claimResume`,
  `composeResumeSpec`).
- Wake on message: `internal/server/resume.go` (`wakeAgent`, `wakeCandidate`),
  `internal/server/sessions.go` (`handlePrompt`); `internal/server/wake_test.go`.
- Archive transition coordination: `internal/server/archive_gate.go`; `TestAgentArchiveClaimBlocksConcurrentResume`.
- Switch + rollback + matrix: `internal/server/switch.go` (`handleSwitchRuntime`, `rollbackSwitch`,
  `validateSwitchTarget`), `internal/server/terminal.go` (`terminalSupported`).
- Stop/cancel/rename/clone + orphan reap: `internal/server/sessions.go`, `internal/server/groups.go`,
  `ui/src/components/grid/CardContextMenu.tsx`.
- CLI launch/resume forms: `internal/cli/launch.go` (`parseLaunch`, `runLaunch`).
- Crash/reconcile/done-vs-error: `internal/runtime/chat.go` (`onTransportClosed`),
  `internal/runtime/terminal/terminal.go` (`startWatcher`, `setDone`), `internal/runtime/reconcile.go`
  (`ReconcileStale`).
- Missing-FTS5 write degradation: `internal/index/indexer.go` (`ftsWritesAvailable`, metadata and
  turn-document write boundaries).
- Bug classes guarded: INV §2 (identity across resume/switch), §4 (orphan reaping / liveness), §6
  (LaunchSpec contract + capability honesty).
