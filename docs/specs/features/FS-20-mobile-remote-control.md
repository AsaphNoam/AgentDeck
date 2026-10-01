# FS-20 — Mobile remote control

**Status:** Partial
**Code:** `internal/remote/`, `internal/server/remote*.go`, `ui/src/features/settings/RemoteEditor.tsx`, `ui/src/remote/` (see TS-13) · **Journeys:** —
**Absorbed:** —

## 1. Purpose

A person can keep supervising and directing AgentDeck from a phone while the Mac keeps doing the
work. The phone is a companion control surface, not a second AgentDeck: repositories, worktrees,
terminals, credentials, backends, configuration, and agent execution stay on the Mac, and the Mac
remains authoritative for every state and decision. The phone is designed for being away, not built
from shrunken desktop screens: it opens on what needs the person, gives each decision its context,
and lets them continue conversations and start work against projects already configured on the
desktop.

The phone reaches the Mac only over the person's own Tailscale network (tailnet), and only after
being paired from the desktop. Nothing is exposed to the public internet and AgentDeck operates no
cloud service or account. This is the one planned exception to FS-00.R1/R2 (FS-00.R18).

## 2. Behavior

### 2.1 Turning remote control on

- **R1 — Remote control is off until the person turns it on.** Desktop Settings gains a
  **Remote** section. Turning remote control on makes AgentDeck join the person's tailnet as its
  own device named `agentdeck` (the product name governs it, FS-00.R16); no Tailscale application
  is required on the Mac. The first time, the section shows a Tailscale sign-in link that the person
  opens to approve the device; afterward AgentDeck rejoins on its own at every start while remote
  control stays on.
- **R2 — The section states the remote connection plainly.** It shows exactly one of:
  **Off**; **Needs Tailscale sign-in** (with the link); **Connecting**; **On** with the phone
  address `https://agentdeck.<tailnet>.ts.net`; or **Unavailable** with the reason and a repair —
  including **Turn on MagicDNS** and **Turn on HTTPS certificates** in the Tailscale admin console,
  which remote control requires.
- **R29 — The public name disclosure is stated before turning it on.** The Remote section
  says that turning on HTTPS certificates publishes the device name `agentdeck.<tailnet>.ts.net` in
  public certificate-transparency logs, while the device itself stays reachable only from the
  tailnet.
- **R3 — Only tailnet devices can reach it, over HTTPS.** The phone address is reachable
  only from devices on the same tailnet, with a valid certificate. Remote control never publishes
  AgentDeck to the public internet. The desktop browser interface on the Mac is unchanged, including
  its same-machine trust (FS-00.R2).
- **R4 — Turning it off cuts remote access at once.** Every open phone connection closes
  and the phone address stops answering. Paired phones stay paired, so turning it back on restores
  them without pairing again.
- **R5 — The phone needs Tailscale too.** The phone must run the Tailscale app signed in
  to the same tailnet. AgentDeck does not install or configure the phone's Tailscale and says so
  where pairing starts.

### 2.2 Pairing and revoking phones

- **R6 — Pairing starts only on the desktop.** **Pair a phone** shows a QR code and an
  8-character pairing code, valid once and for five minutes; a new code replaces an older one.
  Scanning opens the phone app with the code filled in; typing the code into the app works too.
  On iPhone, where a home-screen app does not share Safari's storage, the app asks the person to add
  it to the Home Screen first and to enter the code there. The phone app proposes a name for the phone
  (editable). The desktop then asks **Allow this phone?** with that name; the phone gains access only
  after the person allows it. A declined, expired, or already-used code pairs nothing and tells the
  phone to start again from the desktop.
- **R7 — Only a paired phone sees or does anything.** A tailnet device that is not
  paired, or whose pairing was revoked, receives only a page saying it must be paired from the
  desktop — never agent, task, project, or transcript data, and never an action.
- **R8 — The person manages paired phones on the desktop.** The Remote section lists each
  paired phone with its name, when it was paired, when it was last seen, and whether notifications
  are on. **Rename** and **Revoke** are available. Revoking takes effect immediately: the phone's open
  connection closes and it shows **This phone was unpaired**. A phone can also **Unpair this phone**
  itself. Pairings persist across AgentDeck restarts until revoked or unpaired.
- **R9 — A pairing lives with the installed app.** Clearing the phone browser's site data
  or deleting the installed app loses its pairing; that phone must pair again, and the desktop's
  stale entry remains until revoked.

### 2.3 The phone app

- **R10 — It is an installable web app served by the Mac.** Opening the phone address in
  the phone browser runs the app; adding it to the home screen installs it. It works on Android
  (Chrome) and iPhone (Safari, iOS 16.4 or later). It always runs the version the Mac serves; there
  is nothing to publish or update separately.
- **R11 — Home opens on what needs the person.** Home has three parts, across every
  project:
  - **Needs you** — agents with a pending permission request or in `waiting_input` or `error`
    (FS-02.R3); tasks in `interrupted` or `dependency_failed` (FS-16.R8/R16, FS-02.R44); pipeline
    runs in `paused` or otherwise needing attention (FS-14.R12/R29/R54). Oldest first, each saying
    what is needed in one line.
  - **Moving** — busy agents, running tasks, and active pipeline runs, grouped by project; a run
    shows its stage position (for example "Stage 2 of 4").
  - **Since you last looked** — agents that became `done`, tasks that finished, and runs that reached
    a terminal state since this phone last opened Home, with their outcome. An agent whose task or
    run is under **Needs you** is not listed again here; that entry opens its conversation.
- **R12 — A decision carries its context.** A permission request opens as a card showing
  the agent (`role@project`), the tool and its command or a diff summary, and the agent's latest
  message, with **Approve** and **Deny** exactly as on the desktop (FS-03.R15/R21). A `waiting_input`
  question shows the agent's latest message and a reply box. Each card also opens the full
  conversation.
- **R13 — Conversations can be continued.** Opening an agent shows its transcript for
  reading on a phone (messages in full; tool calls collapsed to one line that expands; diffs
  viewable) and a composer offering Send, the held follow-up while the agent is busy, and Steer where
  the runtime supports it (FS-03.R6/R48–R50). Cancel, Stop, and Resume follow FS-01.R6/R7/R10.
  Terminal-interface agents show status only; the terminal itself is desktop-only. The phone loads
  the latest bounded window of the conversation and offers **Show earlier** for up to three older
  windows; a pending permission request or the latest reply that falls before the window still
  appears (TS-13.R18).
- **R14 — Tasks and runs can be redirected.** A task offers Retry, Re-arm (reusing its
  existing prerequisites or removing them), Record result, and Cancel under their FS-16.R22–R25 rules.
  A pipeline run offers Continue (with optional input for a blocked stage), Retry stage, Replace
  orchestrator, Retry cleanup, and Stop under their FS-14.R12/R13/R20/R73/R77 rules. Each opens its
  agent's conversation where one exists.
- **R15 — New work starts against configured projects.** **New work** asks for an
  existing active project, then offers:
  - **Ask AgentDecker** — opens that project's running `agentdecker` agent, or launches one with the
    runtime the desktop New Agent form would preselect, and sends the typed instruction;
  - **New task** — a display name, an instruction, and a role, launched with the preselected runtime
    and no prerequisites (FS-16.R1/R2);
  - **Start pipeline** — an existing template, run goal, and its required inputs, using the
    configured stage runtimes (FS-14.R3/R80).
  Creating or editing projects, roles, templates, and runtime choices beyond these defaults is
  desktop-only.
- **R16 — Desktop-only surfaces stay on the desktop.** The phone offers no terminal,
  Settings, roles/projects/backends editing, federation, annotate-and-assign, worktree creation or
  cleanup, clone, switch runtime, rename, archive search, dashboard layout, skins, or onboarding.
  R30–R32 and R35–R37 supersede this restriction only for task-prerequisite editing, pipeline
  replacement runtime choices, diff annotation-and-assignment, new-agent runtime choice, rename,
  clone, chat backend/model/effort switching, single-agent archive, and tracked-file reads (R40
  later removes the task-prerequisite editing exception); the other listed surfaces stay
  desktop-only.
- **R17 — The app is live while open.** Every view updates without refreshing while the
  phone is connected, and catches up to current state after reconnecting.
- **R30 — Re-arm can edit prerequisites on the phone.** On an `armed`, `ready`, or
  `dependency_failed` task, the Re-arm flow starts from the task's current arms and lets the person
  add, remove, or change work-result and named-signal prerequisites before submitting the complete
  replacement set. The Mac applies the same project, source, outcome, cycle, cardinality, and state
  validation as the desktop under FS-16.R5/R15/R23; the phone receives the same typed refusal and
  preserves the draft when validation fails. This supersedes R14's former reuse-or-remove-only
  mobile restriction.
- **R31 — Replacing an orchestrator can choose its runtime.** The phone's Replace
  orchestrator flow shows the Mac's configured backend/model choices and each model's allowed
  effort and fast-mode values, preselected to the run's standing assignment. The person may change
  those values for the replacement; the Mac validates the submitted assignment against its current
  configuration before launching it. Backend/model environment, credentials, configuration, and
  editing remain desktop-only. This is the one runtime-choice exception to R15/R16; new agents,
  tasks, and pipeline starts still use their configured defaults.
- **R32 — A phone can annotate and assign a diff.** In a chat-interface agent's phone
  conversation, selecting a contiguous diff-line range opens a phone-sized form for the required
  instruction and the FS-13 targets: the current agent, another running chat agent, or a new task.
  The phone's new task launches the source agent's role in its project with the runtime defaults
  of R15, then delivers to it; a retry after a failed delivery targets that launched agent rather
  than launching another. The FS-13 overall instruction stays desktop-only. The structured anchor, excerpt clipping, bounded browser-local drafts, delivery, durable source
  event, failure preservation, and target validation are the same as FS-13.R2–R9/R11–R17. Terminal
  agents remain ineligible. This supersedes R16's blanket annotate-and-assign exclusion only for
  live diff lines; other transcript-event and archive annotation interactions remain desktop-only.
- **R33 (shipped 2026-10-02) — The phone follows the desktop's dashboard → project → chat flow.** Home
  is the phone form of the desktop project dashboard (FS-02.R29/R30): every active configured
  project, including one with no agents, plus an unavailable-project entry for a force-deleted
  project still referenced by a non-archived agent, each showing its title and color (or durable id),
  its agent count, and its live per-state summary, ordered as the desktop orders them. Above the
  projects, a compact **Needs you** list keeps R11's attention items for agents and pipeline runs,
  oldest first, each opening its card or conversation; it is hidden when empty. R11's **Moving** and
  **Since you last looked** sections leave the phone. Archived projects and agents do not appear;
  archive browsing and search stay desktop-only. This supersedes R11's Home layout.
- **R34 (shipped 2026-10-02) — A project page lists its agents.** Selecting a project opens its phone
  project page, the phone form of `/project/:project-id` (FS-02.R31): one row per non-archived agent
  with its name, role, backend · model, state badge, and single-line preview (FS-02.R2/R3/R5),
  running agents first and stopped agents dimmed (FS-02.R6/R45); the project's pipeline runs that
  are not terminal; and two separate actions, **New agent** (R35) and **Start pipeline** (R41).
  Selecting an agent opens its phone conversation (R13);
  a stopped agent offers Resume under FS-01.R10/R31. Agents launched for tasks appear here as
  ordinary agents.
- **R35 (shipped 2026-10-02) — New agent starts work from a project page.** **New agent** on a project
  page asks for a role, shows a suggested editable name (FS-01.R1/R4), and offers the runtime choice
  of R31 — the Mac's configured backend/model and each model's allowed effort and fast-mode values,
  preselected to the defaults the desktop New Agent form would choose (FS-01.R5/R30/R35). It has no
  message field; the person writes to the agent in its conversation. The phone launches only the
  chat interface; terminal agents, worktree forks, groups, and per-launch permission overrides stay
  desktop-only. The new agent opens in its phone conversation. A rejected launch shows the desktop's
  reason and keeps every entered value (R27). The global **New work** screen — Ask AgentDecker, New
  task, and its Start pipeline — leaves the phone; R41 moves Start pipeline to the project page.
  This supersedes R15 and R31's defaults-only rule for new agents.
- **R36 (shipped 2026-10-02) — Agent management actions on the phone.** An agent's phone screen offers,
  each under its desktop rules and only where the desktop would offer it:
  - **Rename** (FS-01.R4 name contract);
  - **Effort** and **Fast mode** on a running chat agent, applied without a restart (FS-03.R45/R47);
  - **Switch runtime** — backend, model, and effort — on a running chat agent (FS-01.R13–R15),
    choosing from R31's runtime options; switching the interface stays desktop-only;
  - **Clone** under FS-01.R36, showing its unavailable reason when Clone is not available, and
    opening the new agent;
  - **Archive** under FS-05.R32. Because restoring an archived agent stays desktop-only (R33), the
    phone asks once to confirm and says that restore is on the desktop; the agent then leaves every
    phone list and its open phone screen returns to its project.
  This supersedes R16 for rename, clone, and switch runtime, and for archive of a single agent.
- **R37 (shipped 2026-10-02) — The phone shows what an agent changed and ran.** An agent's phone screen
  has **Files** and **Commands** views of its FS-05.R15/R16 tracking: each changed file with its
  edit count and last-touched time, and each command with its exit status, newest first. A changed
  file with a diff opens that diff in the conversation; **Open file** shows the file's current
  text read-only with the desktop's size, binary, and encoding limits (FS-03.R64). The phone opens
  only paths in that agent's own changed-files list; it never opens an arbitrary path, searches
  files, or reads a file reached through a symbolic link at the tracked path. Annotating opened file
  text stays desktop-only. This is the one remote exception to FS-03.R64's statement that no
  remote-control route gains file-reading authority.
- **R38 (shipped 2026-10-02) — Agent mail stays off the phone.** The phone offers no mailbox reading or
  mail sending; agent mail remains agent-to-agent (FS-06).
- **R40 (shipped 2026-10-02) — Tasks leave the phone.** The phone has no task screen, task list, task
  rows, or task controls: Retry, Re-arm and its prerequisite editor (R30), Record result, Cancel, and
  New task become desktop-only. Tasks in `interrupted` or `dependency_failed` no longer appear in
  **Needs you** and no longer notify phones; the agents those tasks launched remain ordinary agents
  on their project pages, and their own attention states still appear and notify. Opening a former
  task link, such as an older notification, shows Home. Diff annotation's new-task target (R32)
  stays, because it opens the launched agent rather than a task page. This supersedes R11, R14, and
  R18 for tasks and supersedes R30.
- **R41 (shipped 2026-10-02) — Start pipeline from a project page.** **Start pipeline** on a project page
  is the former New work pipeline flow with that project fixed: a valid existing template, an
  optional display name, the run goal, and the template's required inputs, using the configured
  stage runtimes and the same shared-workspace acknowledgment as the desktop (FS-14.R3/R80). The new
  run opens on its phone run screen; a refused start shows the desktop's reason and keeps entered
  values (R27).

### 2.4 Phone notifications

- **R18 — Installed phones can receive attention notifications.** After the person
  allows notifications in the installed app, the phone is notified when an agent raises a permission
  request or enters `waiting_input` or `error`, when a task enters `interrupted` or
  `dependency_failed`, and on `pipeline_needs_attention`. Completions (`done`, `pipeline_completed`)
  do not notify the phone. The desktop per-type mutes (FS-02.R24) also apply, and each phone has its
  own notifications switch.
- **R19 — Notification text is minimal.** A notification names the agent or run, the
  project, and the kind of attention (for example "implementer@my-app needs permission"). It never
  includes commands, diffs, message text, or file contents; those appear only inside the app.
- **R20 — Notifications are grouped.** Attention from one project or run arriving within a
  short window replaces or updates one notification instead of stacking many.
- **R21 — Tapping opens the decision; nothing is decided from a notification.** Tapping
  opens the matching card. Notifications carry no Approve, Deny, reply, or other action buttons.

### 2.5 Keeping the Mac awake

- **R22 — Optional keep-awake while work is active.** The Remote section offers **Keep
  this Mac awake while work is active**, off by default and usable with or without remote control.
  While on, AgentDeck prevents idle system sleep whenever any agent is busy or waiting on a
  permission request or any pipeline run is active, and allows sleep again once none is. It states
  that closing the lid or choosing Sleep still sleeps the Mac.

## 3. States & transitions

Desktop remote connection: `Off → Needs Tailscale sign-in → Connecting → On`; `On → Connecting` on
network loss; any state `→ Unavailable` on a failure with a stated repair; any state `→ Off` when
the person turns it off.

Pairing: `Code shown → Awaiting allow → Paired`; `Code shown → Expired | Used`;
`Awaiting allow → Declined`; `Paired → Revoked | Unpaired`.

Phone connection: `Connected ↔ Reconnecting → Mac unreachable (since <time>)`; any state
`→ Unpaired` when the pairing ends.

## 4. Edge cases & errors

- **R23 — An unreachable Mac is stated, not hidden.** When the Mac is asleep, offline, or
  not running AgentDeck, or remote control is off, the phone shows **Mac unreachable since <time>**,
  keeps the last-known content visibly marked as stale, and disables every action. Nothing typed or
  tapped is queued for later delivery.
- **R24 — The first decision wins.** When the desktop and a phone, or two phones, act on
  the same permission, question, task, or run, the first accepted action wins and the later one sees
  what already happened (for example **Already answered on the Mac**) with the current state; it is
  never applied twice.
- **R25 — A stale notification opens the current truth.** Tapping a notification whose
  item was already resolved opens that item in its current state with what resolved it.
- **R26 — Loss of Tailscale sign-in is repairable on the desktop.** When the tailnet
  device's sign-in expires or it is removed from the tailnet, the Remote section returns to **Needs
  Tailscale sign-in** and phones see **Mac unreachable** until it is repaired.
- **R27 — Refused actions explain themselves.** An action the Mac refuses (for example
  Steer on a runtime without steering, Resume on an archived agent, Re-arm in a disallowed state)
  shows the same reason the desktop would, and the composer keeps the person's typed text.
- **R39 (shipped 2026-10-02) — Phone screens follow archival and removal.** When an agent is archived or
  its project archived from either device, it leaves every phone list and an open phone screen for it
  shows **Archived on the Mac** with a way back to its project or Home; when a project is archived,
  its open project screen says so and returns to Home.
- **R28 — Notification delivery failures are visible.** When a phone's notification
  permission is withdrawn or its subscription expires, the phone's desktop entry shows
  **Notifications off**, and the installed app offers to turn them back on.

## 5. Acceptance criteria

- **A1 (planned)** (R1–R4, R29) — Turning remote control on joins the tailnet, shows each connection
  state including the MagicDNS and HTTPS-certificate repairs and the certificate-log disclosure, and
  serves the phone address over HTTPS; turning it off closes open phone connections, and
  the loopback desktop interface behaves identically with remote control on or off. — server tests
  with a fake tailnet listener; manual gate on a real tailnet.
- **A2** (R6–R8) — A pairing code works once within five minutes and only after the desktop
  allows the phone; an expired, reused, or declined code pairs nothing; an unpaired or revoked device
  receives no data and no action, including the live stream; revoking closes an open phone
  connection. — server tests.
- **A3** (R11–R13, R17) — At a 390px-wide phone viewport, Home shows Needs you / Moving /
  Since you last looked from seeded agents, tasks, and runs; a permission card approves and denies
  with the desktop result; a conversation sends, holds a follow-up, steers, cancels, and stops. —
  UI tests plus a fakeACP browser pass at phone size.
- **A4** (R14, R15, R27) — From the phone: retry an `interrupted` task, continue a paused
  run, start a pipeline run from a template, create a task, and Ask AgentDecker both with and without
  a running `agentdecker` agent; a refused action shows the desktop reason and keeps typed text. —
  server and UI tests plus the fakeACP browser pass.
- **A5 (planned)** (R18–R21, R25, R28) — Each attention event notifies a subscribed phone once,
  honoring desktop mutes and the per-phone switch; completions never notify; payload text contains
  no command, diff, message, or file content; notifications carry no actions; a resolved item's
  notification opens its current state. — server tests on the notification payload; manual gate on
  a real Android phone and a real iPhone.
- **A6 (planned)** (R22) — With keep-awake on, idle sleep is prevented exactly while work is active
  and released when idle; with it off, nothing is prevented. — server tests on the sleep-assertion
  lifecycle; manual `pmset -g assertions` check.
- **A7** (R23, R24, R26) — With AgentDeck stopped, the phone shows Mac unreachable, stale
  content, and disabled actions, and delivers nothing on reconnect; concurrent desktop and phone
  permission answers apply once and the loser sees what happened. — server and UI tests.
- **A8 (planned)** (all) — Real end-to-end journey: pair an Android phone and an iPhone over a real
  tailnet off the home network, receive a permission notification, approve it with details, reply
  to a question, launch a new agent from a project page, then revoke one phone. — manual gate.
- **A9 (planned)** (R30–R32) — At a 390px phone viewport, edit an eligible task from one valid
  prerequisite set to another, replace a pipeline orchestrator with a different configured
  model/effort/fast selection, and annotate diff lines to each supported target; invalid arms and
  runtime choices are rejected without mutation, and a failed annotation send preserves its draft.
  — server and UI tests plus a fakeACP browser pass at phone size.
- **A10 (planned)** (R33–R35, R41) — At a 390px phone viewport with seeded projects (one empty, one
  unavailable, one archived) and running, stopped, and archived agents: Home shows the Needs you
  list and exactly the active and unavailable projects with desktop counts and per-state summaries,
  updating live; a project page lists its non-archived agents in desktop order and its active runs,
  opens an agent's conversation, and resumes a stopped agent; New agent launches a chat agent with a
  non-default model/effort/fast selection; Start pipeline from that page starts a run in that
  project; an invalid runtime choice or a permission-bypass or interface field is rejected without
  launching, and a rejected launch keeps entered values; the global New work screen is gone. — server tests (launch body filter, shared
  validator) and UI tests plus a fakeACP browser pass at phone size.
- **A11 (planned)** (R36, R39) — From the phone: rename an agent, change effort and fast mode on a
  running chat agent, switch its model, clone it at idle (and see the unavailable reason while a
  turn is active), and archive it after the confirmation; the desktop shows each result; an
  interface switch, a restore, and every refused action return the desktop's reason; archiving an
  agent or its project on the desktop while its phone screen is open shows the archived state. —
  server and UI tests plus the fakeACP browser pass.
- **A12 (planned)** (R37, R38) — The phone lists an agent's changed files and commands, opens a
  changed file's diff and current text; a request for an untracked path, another agent's tracked
  path, a relative escape, or a tracked path replaced by a symbolic link is refused without reading
  the file; file search and mailbox routes remain unreachable from the tailnet. — server tests on
  the tailnet file filter and route inventory, UI tests.
- **A13 (planned)** (R40) — The phone shows no task screen, row, control, or New task; an
  `interrupted` task neither appears in Needs you nor notifies a phone, while its launched agent
  appears on its project page and its own attention still notifies; a `/task/<id>` link opens Home;
  every task route is unreachable from the tailnet. — server tests on attention, push, and the route
  inventory; UI tests.

## 6. Deviations & open decisions

- R33–R35 and R40 restructured the phone behavior on 2026-10-02: R11's Moving and Since you
  last looked sections, R14's task controls, R15's New work screen, R30, and the task halves of
  R18 and A3/A4/A9 were retired or narrowed in the same change.

- The phone's pairing credential lives in the installed app's browser storage (R9); a Face ID or
  fingerprint app lock is not part of this version.
- Tailscale is required on both devices; AgentDeck runs no relay of its own.
- The installed app's icon is SVG only. Android uses it; iPhone Home Screen falls back to a page
  snapshot until a PNG touch icon is added.
- A1, A5, A6, and A8 keep their manual gates owed: no real tailnet, Android phone, iPhone, or
  `pmset -g assertions` check has run. Their automated halves pass.
- A9's server and UI halves pass; its fakeACP browser pass at phone size ran 2026-09-29, leaving
  only the replacement fast-mode picker unexercised.

## 7. Traceability

- A2: `internal/server/remote_routes_test.go`, `remote_pairing_test.go` (guard, node-bound cookie,
  allowlist inventory, single-use claim under `-race`, revoke closing the stream).
- A3/A4: `ui/src/remote/*.test.tsx`, `internal/server/remote_home_test.go`; 2026-09-28 fakeACP
  browser pass at 390×844 through the dev-only fake tailnet (pair, Home, approve, send, stop,
  create task, revoke) with no horizontal overflow.
- A5 (automated half): `internal/server/remote_push_test.go`, `internal/remote/push_test.go`.
- A6 (automated half): `internal/server/keepawake_test.go`, `internal/remote/keepawake_test.go`.
- A7: `ui/src/remote/PhoneApp.test.tsx`, `AgentScreen.test.tsx` (unreachable, stale, disabled,
  first decision wins).
- Settings: `ui/src/features/settings/RemoteEditor.test.tsx`.
- A9 (automated half): `ui/src/remote/WorkScreens.test.tsx` (task-control matrix, Re-arm editor,
  orchestrator replacement), `AgentScreen.test.tsx` (diff annotation);
  `internal/server/remote_routes_test.go` (`TestRemoteRearmHasDesktopValueAuthority`,
  `TestRemoteReplaceValidatesChosenRuntime`, `TestRemoteRuntimeOptionsAreSecretFree`),
  `annotations_test.go` (`TestRemoteAnnotationUsesSharedDelivery`).
