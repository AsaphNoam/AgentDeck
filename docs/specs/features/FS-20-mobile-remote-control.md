# FS-20 — Mobile remote control

**Status:** Partial
**Code:** — (planned: see TS-13) · **Journeys:** —
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

- **R1 (planned) — Remote control is off until the person turns it on.** Desktop Settings gains a
  **Remote** section. Turning remote control on makes AgentDeck join the person's tailnet as its
  own device named `agentdeck` (the product name governs it, FS-00.R16); no Tailscale application
  is required on the Mac. The first time, the section shows a Tailscale sign-in link that the person
  opens to approve the device; afterward AgentDeck rejoins on its own at every start while remote
  control stays on.
- **R2 (planned) — The section states the remote connection plainly.** It shows exactly one of:
  **Off**; **Needs Tailscale sign-in** (with the link); **Connecting**; **On** with the phone
  address `https://agentdeck.<tailnet>.ts.net`; or **Unavailable** with the reason and a repair —
  including **Turn on MagicDNS** and **Turn on HTTPS certificates** in the Tailscale admin console,
  which remote control requires.
- **R29 (planned) — The public name disclosure is stated before turning it on.** The Remote section
  says that turning on HTTPS certificates publishes the device name `agentdeck.<tailnet>.ts.net` in
  public certificate-transparency logs, while the device itself stays reachable only from the
  tailnet.
- **R3 (planned) — Only tailnet devices can reach it, over HTTPS.** The phone address is reachable
  only from devices on the same tailnet, with a valid certificate. Remote control never publishes
  AgentDeck to the public internet. The desktop browser interface on the Mac is unchanged, including
  its same-machine trust (FS-00.R2).
- **R4 (planned) — Turning it off cuts remote access at once.** Every open phone connection closes
  and the phone address stops answering. Paired phones stay paired, so turning it back on restores
  them without pairing again.
- **R5 (planned) — The phone needs Tailscale too.** The phone must run the Tailscale app signed in
  to the same tailnet. AgentDeck does not install or configure the phone's Tailscale and says so
  where pairing starts.

### 2.2 Pairing and revoking phones

- **R6 (planned) — Pairing starts only on the desktop.** **Pair a phone** shows a QR code and an
  8-character pairing code, valid once and for five minutes; a new code replaces an older one.
  Scanning opens the phone app with the code filled in; typing the code into the app works too.
  On iPhone, where a home-screen app does not share Safari's storage, the app asks the person to add
  it to the Home Screen first and to enter the code there. The phone app proposes a name for the phone
  (editable). The desktop then asks **Allow this phone?** with that name; the phone gains access only
  after the person allows it. A declined, expired, or already-used code pairs nothing and tells the
  phone to start again from the desktop.
- **R7 (planned) — Only a paired phone sees or does anything.** A tailnet device that is not
  paired, or whose pairing was revoked, receives only a page saying it must be paired from the
  desktop — never agent, task, project, or transcript data, and never an action.
- **R8 (planned) — The person manages paired phones on the desktop.** The Remote section lists each
  paired phone with its name, when it was paired, when it was last seen, and whether notifications
  are on. **Rename** and **Revoke** are available. Revoking takes effect immediately: the phone's open
  connection closes and it shows **This phone was unpaired**. A phone can also **Unpair this phone**
  itself. Pairings persist across AgentDeck restarts until revoked or unpaired.
- **R9 (planned) — A pairing lives with the installed app.** Clearing the phone browser's site data
  or deleting the installed app loses its pairing; that phone must pair again, and the desktop's
  stale entry remains until revoked.

### 2.3 The phone app

- **R10 (planned) — It is an installable web app served by the Mac.** Opening the phone address in
  the phone browser runs the app; adding it to the home screen installs it. It works on Android
  (Chrome) and iPhone (Safari, iOS 16.4 or later). It always runs the version the Mac serves; there
  is nothing to publish or update separately.
- **R11 (planned) — Home opens on what needs the person.** Home has three parts, across every
  project:
  - **Needs you** — agents with a pending permission request or in `waiting_input` or `error`
    (FS-02.R3); tasks in `interrupted` or `dependency_failed` (FS-16.R8/R16, FS-02.R44); pipeline
    runs in `paused` or otherwise needing attention (FS-14.R12/R29/R54). Oldest first, each saying
    what is needed in one line.
  - **Moving** — busy agents, running tasks, and active pipeline runs, grouped by project; a run
    shows its stage position (for example "Stage 2 of 4").
  - **Since you last looked** — agents that became `done`, tasks that finished, and runs that reached
    a terminal state since this phone last opened Home, with their outcome.
- **R12 (planned) — A decision carries its context.** A permission request opens as a card showing
  the agent (`role@project`), the tool and its command or a diff summary, and the agent's latest
  message, with **Approve** and **Deny** exactly as on the desktop (FS-03.R15/R21). A `waiting_input`
  question shows the agent's latest message and a reply box. Each card also opens the full
  conversation.
- **R13 (planned) — Conversations can be continued.** Opening an agent shows its transcript for
  reading on a phone (messages in full; tool calls collapsed to one line that expands; diffs
  viewable) and a composer offering Send, the held follow-up while the agent is busy, and Steer where
  the runtime supports it (FS-03.R6/R48–R50). Cancel, Stop, and Resume follow FS-01.R6/R7/R10.
  Terminal-interface agents show status only; the terminal itself is desktop-only.
- **R14 (planned) — Tasks and runs can be redirected.** A task offers Retry, Re-arm (reusing its
  existing prerequisites or removing them), Record result, and Cancel under their FS-16.R22–R25 rules.
  A pipeline run offers Continue (with optional input for a blocked stage), Retry stage, Replace
  orchestrator, Retry cleanup, and Stop under their FS-14.R12/R13/R20/R73/R77 rules. Each opens its
  agent's conversation where one exists.
- **R15 (planned) — New work starts against configured projects.** **New work** asks for an
  existing active project, then offers:
  - **Ask AgentDecker** — opens that project's running `agentdecker` agent, or launches one with the
    runtime the desktop New Agent form would preselect, and sends the typed instruction;
  - **New task** — a display name, an instruction, and a role, launched with the preselected runtime
    and no prerequisites (FS-16.R1/R2);
  - **Start pipeline** — an existing template, run goal, and its required inputs, using the
    configured stage runtimes (FS-14.R3/R80).
  Creating or editing projects, roles, templates, and runtime choices beyond these defaults is
  desktop-only.
- **R16 (planned) — Desktop-only surfaces stay on the desktop.** The phone offers no terminal,
  Settings, roles/projects/backends editing, federation, annotate-and-assign, worktree creation or
  cleanup, clone, switch runtime, rename, archive search, dashboard layout, skins, or onboarding.
- **R17 (planned) — The app is live while open.** Every view updates without refreshing while the
  phone is connected, and catches up to current state after reconnecting.

### 2.4 Phone notifications

- **R18 (planned) — Installed phones can receive attention notifications.** After the person
  allows notifications in the installed app, the phone is notified when an agent raises a permission
  request or enters `waiting_input` or `error`, when a task enters `interrupted` or
  `dependency_failed`, and on `pipeline_needs_attention`. Completions (`done`, `pipeline_completed`)
  do not notify the phone. The desktop per-type mutes (FS-02.R24) also apply, and each phone has its
  own notifications switch.
- **R19 (planned) — Notification text is minimal.** A notification names the agent or run, the
  project, and the kind of attention (for example "implementer@my-app needs permission"). It never
  includes commands, diffs, message text, or file contents; those appear only inside the app.
- **R20 (planned) — Notifications are grouped.** Attention from one project or run arriving within a
  short window replaces or updates one notification instead of stacking many.
- **R21 (planned) — Tapping opens the decision; nothing is decided from a notification.** Tapping
  opens the matching card. Notifications carry no Approve, Deny, reply, or other action buttons.

### 2.5 Keeping the Mac awake

- **R22 (planned) — Optional keep-awake while work is active.** The Remote section offers **Keep
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

- **R23 (planned) — An unreachable Mac is stated, not hidden.** When the Mac is asleep, offline, or
  not running AgentDeck, or remote control is off, the phone shows **Mac unreachable since <time>**,
  keeps the last-known content visibly marked as stale, and disables every action. Nothing typed or
  tapped is queued for later delivery.
- **R24 (planned) — The first decision wins.** When the desktop and a phone, or two phones, act on
  the same permission, question, task, or run, the first accepted action wins and the later one sees
  what already happened (for example **Already answered on the Mac**) with the current state; it is
  never applied twice.
- **R25 (planned) — A stale notification opens the current truth.** Tapping a notification whose
  item was already resolved opens that item in its current state with what resolved it.
- **R26 (planned) — Loss of Tailscale sign-in is repairable on the desktop.** When the tailnet
  device's sign-in expires or it is removed from the tailnet, the Remote section returns to **Needs
  Tailscale sign-in** and phones see **Mac unreachable** until it is repaired.
- **R27 (planned) — Refused actions explain themselves.** An action the Mac refuses (for example
  Steer on a runtime without steering, Resume on an archived agent, Re-arm in a disallowed state)
  shows the same reason the desktop would, and the composer keeps the person's typed text.
- **R28 (planned) — Notification delivery failures are visible.** When a phone's notification
  permission is withdrawn or its subscription expires, the phone's desktop entry shows
  **Notifications off**, and the installed app offers to turn them back on.

## 5. Acceptance criteria

- **A1 (planned)** (R1–R4, R29) — Turning remote control on joins the tailnet, shows each connection
  state including the MagicDNS and HTTPS-certificate repairs and the certificate-log disclosure, and
  serves the phone address over HTTPS; turning it off closes open phone connections, and
  the loopback desktop interface behaves identically with remote control on or off. — server tests
  with a fake tailnet listener; manual gate on a real tailnet.
- **A2 (planned)** (R6–R8) — A pairing code works once within five minutes and only after the desktop
  allows the phone; an expired, reused, or declined code pairs nothing; an unpaired or revoked device
  receives no data and no action, including the live stream; revoking closes an open phone
  connection. — server tests.
- **A3 (planned)** (R11–R13, R17) — At a 390px-wide phone viewport, Home shows Needs you / Moving /
  Since you last looked from seeded agents, tasks, and runs; a permission card approves and denies
  with the desktop result; a conversation sends, holds a follow-up, steers, cancels, and stops. —
  UI tests plus a fakeACP browser pass at phone size.
- **A4 (planned)** (R14, R15, R27) — From the phone: retry an `interrupted` task, continue a paused
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
- **A7 (planned)** (R23, R24, R26) — With AgentDeck stopped, the phone shows Mac unreachable, stale
  content, and disabled actions, and delivers nothing on reconnect; concurrent desktop and phone
  permission answers apply once and the loser sees what happened. — server and UI tests.
- **A8 (planned)** (all) — Real end-to-end journey: pair an Android phone and an iPhone over a real
  tailnet off the home network, receive a permission notification, approve it with details, reply
  to a question, start a task, then revoke one phone. — manual gate.

## 6. Deviations & open decisions

- The phone's pairing credential lives in the installed app's browser storage (R9); a Face ID or
  fingerprint app lock is not part of this version.
- Tailscale is required on both devices; AgentDeck runs no relay of its own.

## 7. Traceability

— (planned)
