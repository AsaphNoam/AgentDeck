# FS-03 — Live chat & permission flow

**Status:** Current
**Code:** `internal/runtime/` (`chat.go`, `permission.go`, `event.go`), `internal/server/sessions.go`, `internal/transcript/`, `ui/src/components/chat/`, `ui/src/store/transcriptStore.ts`, `ui/src/api/sse.ts` · **Journeys:** J3, J4, J7
**Absorbed:** exact source mapping in the [phase archive manifest](../../archive/phases/README.md)

## 1. Purpose

AgentDeck gives each chat-interface agent a live conversation surface over the Agent Client
Protocol (ACP). A user can send a prompt, watch assistant text and tool activity stream, decide
permission requests, cancel work, and reopen the durable transcript after a reload, restart, stop,
or resume. This spec governs the user-visible chat panel, normalized transcript events, prompt and
cancel controls, and permission decisions. Agent lifecycle and runtime switching are governed by
FS-01; archive/search and inactive-session viewing by FS-05; terminal input by FS-07.

## 2. Behavior

Requirements are user- and API-observable. R-item numbering is continuous through §4.

### 2.1 Opening and reading a chat

- **R1.** Opening a chat-interface agent shows its name, its runtime identity (backend, model, and
  resolved effort — FS-01.R30), context usage, and a Transcript tab. For a running agent the runtime
  identity is an inline picker that can switch it (R23). The same surface also exposes the session's
  Files and Commands tabs (FS-05).
- **R2.** The Transcript renders normalized events in sequence: user prompts; assistant text as sanitized
  GitHub-flavored Markdown with syntax-highlighted fenced code; tool calls with expandable JSON
  arguments; tool results with error styling and expandable content after the first 600 characters;
  unified file diffs; permission prompts; turn errors; turn boundaries; and backend-switch dividers.
  An unknown event remains inspectable as formatted JSON rather than disappearing.
- **R3.** The transcript follows new events while the reader remains at the bottom. Scrolling away
  suspends auto-follow and exposes **Jump to latest**, so streaming output does not take control of
  the reader's scroll position.
- **R4.** Consecutive `assistant_text` deltas are folded into one rendered response both while they
  arrive live and when a durable transcript is replayed. A `permission_resolved` event is folded
  into its matching `permission_request`, which then renders an **Approved**, **Denied**,
  **Cancelled**, or **Timed out** chip instead of active decision buttons. Explicit approval and
  auto-approval map to Approved; the other three runtime decisions retain their distinct outcome.
- **R5.** One malformed or unrenderable event is isolated by an event-level error boundary; the
  remainder of the transcript stays usable and the failed item displays a fallback.
- **R28.** Transcript text is selectable and copyable, and an active selection is visibly
  highlighted — including inside the user's own prompt bubble, whose highlight colour must differ
  from the bubble background so the selection remains visible there. This lets a reader copy any
  message content out of the panel.

### 2.2 Prompting and turn state

- **R6.** On an idle chat agent, submitting non-whitespace text through the composer calls
  `POST /api/sessions/{id}/prompt` with `{text}`. Enter submits and Shift+Enter inserts a newline.
  The accepted response is `202` with `{accepted:true, agent_id}`.
- **R7.** The composer immediately displays the submitted user text for the current browser view
  and clears its draft. On acceptance, the runtime emits a sequenced `user_text` event that replaces
  the optimistic bubble and joins the durable transcript. If delivery fails, it shows an actionable
  error and restores the draft so the user can retry; the optimistic bubble remains visible and is
  not presented as server-acknowledged.
- **R36.** Each chat composer keeps its exact non-empty unsent text as a separate draft for
  that `agent_id` in the current browser profile. Returning to the chat after navigating elsewhere,
  refreshing, or closing and reopening AgentDeck restores that draft. The draft is removed only
  when the prompt request is accepted, the person empties the composer, or the browser's AgentDeck
  site data is cleared; deleting the agent also removes its draft through the existing browser-side
  deletion event, while archiving does not. A rejected send keeps or restores the same draft for
  retry. Drafts do not expire, enter the transcript, reach an agent or server API, or sync to another
  browser profile or device. The browser retains at most the 20 most recently edited non-empty chat
  drafts; creating or editing another discards the least recently edited retained draft. Draft
  removal and restoration are scoped to the exact text a send submitted: a newer draft typed for the
  same agent while that send is still in flight is left untouched — it is not discarded when the
  earlier send is accepted, nor overwritten when the earlier send is rejected.
- **R8.** A prompt moves the agent synchronously to `busy`. ACP output is emitted as ordered
  transcript events and a final `turn_end`; successful completion returns the agent to `idle`,
  clears `busy_since`, and updates context usage (which also updates during the turn as the runtime
  receives it — FS-02.R26). A runtime/protocol failure emits an error and
  turn end, marks fatal crashes `error`, and removes the dead running process.
- **R9.** While the agent is `busy` or `waiting_input`, the Send control becomes Cancel.
  `POST /api/sessions/{id}/cancel` returns `202 {cancelled:true}` when it claims an active turn or
  pending permission, and `202 {cancelled:false}` when the agent is already idle. Cancellation
  resolves a pending permission without executing its tool and terminates the turn with reason
  `cancelled`; if the cooperative ACP cancel does not finish, the runtime escalates to process
  interrupt. If that escalation ends the process, the runtime removes its running row, marks the
  agent `error`, and emits a fatal process error that explicitly says the agent exited after ignoring
  cancellation; it does not report the outcome as an unrelated generic process exit. Resolving a
  pending permission records a `permission_resolved` (decision `cancelled`) on the live stream and
  in the durable transcript, so the prompt renders a resolved chip on both the live view and after
  reload instead of staying actionable.
- **R29.** While a chat agent is `busy`, the transcript shows a waiting indicator at its end so it is
  visible that a response is in progress and the turn is not stuck. The indicator clears when the
  agent leaves `busy` — on `turn_end`, on error, or when a permission request pauses it to
  `waiting_input` (where the pending prompt and Cancel control already convey the wait).

- **R48.** **Queue one follow-up while the agent is working.** A person may
  submit a chat message to a `busy` or `waiting_input` chat agent instead of being refused. AgentDeck
  holds it and sends it as an ordinary prompt when the current turn ends, so it becomes the next
  turn. At most one message is held per agent: submitting another **replaces** it, and the composer
  offers withdrawing it. The held message is visible at the end of the transcript as clearly pending
  — not yet sent, not yet seen by the agent — and it enters the durable transcript only when it is
  actually sent, so replay and search never contain a message the agent never received.

  Queueing promises delivery **after** the current turn, not inside it: it does not redirect work in
  progress. Redirecting is the separate, explicit action in R50. A person who wants to stop what is
  happening entirely still uses Cancel.

  Holding is AgentDeck's, not the provider's, so it behaves identically on every backend rather than
  only where an adapter happens to queue. **Only a person's chat message queues.** An agent-initiated
  prompt — a mail wake, a task assignment, a pipeline stage instruction — still fails closed when a
  turn is in flight, because those callers use that refusal to arbitrate (FS-06, FS-14, FS-16); a
  queue there would let a run continue past the gate that paused it.
- **R49.** The held message resolves on exactly three events. On **turn
  end** it is sent, and the ordinary busy progression follows. On **cancel** it is sent as the next
  turn, because cancelling is a deliberate "stop that, do this instead" and discarding the person's
  already-submitted correction is the opposite of what they asked for. On **stop, crash, or archive**
  there is no next turn: the message is released back into the composer as a draft when the composer
  is empty, and discarded when it is not — never overwriting text the person has since typed, the
  same rule R36 already applies to drafts racing an in-flight send. A held message does not survive a
  dashboard restart; it is live state, not durable work. A browser reload rehydrates the hold from
  the still-running dashboard process, including its delivery sequence boundary, so the pending
  message and Withdraw remain available and a later stop can still release it to the composer.

- **R50.** **Steer the turn that is running.** Beside Send, a chat agent
  whose runtime advertises steering offers **Steer**, which delivers the message into the in-flight
  turn immediately rather than holding it. The agent can act on it while it works — the point is
  redirecting an agent heading the wrong way, without discarding the turn as Cancel would. A steered
  message is not held: it reaches the agent at once and enters the transcript as an ordinary user
  message inside the running turn, because the agent really did receive it there.

  Send and Steer are two deliberate choices with two meanings, not one control with a fallback. Send
  is the safe default and is always present; Steer is offered only where the runtime advertises the
  capability, and never appears as a disabled or failing control elsewhere (FS-09.R26's
  explicit-capability rule). With the composer empty and a message already held, Steer delivers that
  held message immediately instead, so a person who queued and then changed their mind about waiting
  is not made to retype it.

  Steering is best-effort by nature and says so: a turn can end between the click and the delivery.
  When that happens the message is delivered as its own next turn rather than lost, and the person is
  told which of the two happened rather than left to infer it from the transcript. A message the
  runtime refuses — content the current model cannot accept — leaves the composer intact with the
  reason, and never silently downgrades to a queue.

### 2.3 Streaming and recovery

- **R10.** Live transcript events arrive on the multiplexed `/api/events` Server-Sent Events (SSE)
  stream as `new_message` envelopes keyed by `agent_id`. Each runtime event carries a per-agent
  monotonic `seq`, event `type`, timestamp, and type-specific `data`; the UI normalizes that nested
  wire shape before rendering.
- **R11.** `GET /api/sessions/{id}/transcript` returns `{agent_id, events}` from the durable
  transcript. `since_seq` returns only later events; `include_meta=true` includes session metadata.
  `events` is always an array. Unknown agents return `404`; a non-integer `since_seq` returns
  `422 validation`.
- **R12.** The chat panel fetches the transcript when opened and whenever its SSE connection
  reopens. If it detects a sequence gap for the currently-open agent, it refetches the authoritative
  transcript rather than appending a possibly incomplete delta stream. Permission decisions fold
  identically on live append and full replay.
- **R13.** Every accepted user prompt and runtime event delivered before a crash is appended to the
  per-agent transcript before it is published to the browser. A mid-turn crash therefore preserves
  both sides of the conversation plus already-delivered assistant/tool output for reload, archive,
  and later resume.

### 2.4 Permission decisions

- **R14.** When an ACP tool call requires approval and skip-permissions is false, AgentDeck emits a
  `permission_request`, sets the agent to `waiting_input`, and withholds the tool until the user
  chooses Approve or Deny, cancellation claims the turn, or the permission timeout expires.
- **R15.** Approve and Deny call `POST /api/sessions/{id}/permission` with `tool_call_id` and
  `decision`. Approve permits the selected ACP option and the tool may execute; Deny selects the
  rejection option and the tool does not execute. A successful response records
  `permission_resolved` in the transcript and returns the prompt to its resolved state.
- **R16.** Permission resolution is single-winner. Concurrent approve, deny, cancel, or timeout
  attempts cannot execute both outcomes. Re-deciding an already-resolved request returns `409`;
  an unknown/no-longer-pending tool call returns a typed failure rather than fabricating success.
- **R17.** An unanswered permission request auto-denies after the configured timeout, emits the
  error `permission timed out`, renders **Timed out**, and finishes the turn without executing the
  tool.
- **R18.** When the frozen launch policy enables skip-permissions, a permission request is recorded
  as auto-approved, the agent never enters `waiting_input`, and the tool proceeds without a user
  click. Resume and switch retain that frozen policy under FS-01.

- **R40** — AgentDeck's own actions never require a human approval. When a
  tool call names one of the actions AgentDeck itself exposes to its agents — `list_agents`,
  `send_message`, `check_messages`, `report_pipeline_stage_result`, `propose_pipeline_template`,
  `propose_pipeline_run`, `get_assigned_task`, `create_task`, `cancel_task`, `report_task_result`,
  `share_context`, `list_context_links`, `read_context_link`, `set_context_link_visibility`, or
  `revoke_context_grant` — the call proceeds immediately. The agent does not enter `waiting_input`
  for it, no approval is asked for, and no approval deadline can apply to it. This holds for every
  chat agent, whatever role or global `skip_permissions` policy was frozen at its launch, and it is
  the only category of tool that gains the treatment. Every other tool keeps R14's gate exactly as
  it is today — file reads and edits, shell commands, network fetches, and every provider- or
  user-configured MCP server — so the permission policy a person chose for their workspace is
  unchanged. The reason these actions are different is that they are AgentDeck's own control plane
  and carry no decision a person could make differently: each is reachable only across the loopback
  boundary with a per-agent credential AgentDeck minted for that agent's current generation, each is
  already authorized server-side against that agent's own identity, and none of them reads or writes
  a file, runs a command, or reaches the network. `create_task` is included even though it can cause
  a new agent to be launched: that agent appears on the dashboard, stays inside the existing
  delegation bound, and every tool it runs is gated normally.

- **R41** — An auto-allowed action is recorded, never hidden. It appears on the
  live stream and in the durable transcript with the same auto-approved shape a skip-permissions
  launch already produces (R18), so a reader can always tell which tool calls ran without a human
  decision and a run's history stays complete across reload, archive, and resume. Nothing reaches
  the agent differently: the same tool names, arguments, results, and refusal/retry classification
  apply (FS-17), and no agent-facing surface is added, removed, or renamed.

- **R42** — Identification fails closed. A tool call AgentDeck cannot
  positively identify as one of its own actions is gated under R14, exactly as today. An
  unrecognized name, a backend whose approval request does not name the tool, an identically named
  tool belonging to a different MCP server, and a malformed request all prompt rather than proceed.
  AgentDeck never infers the exemption from the fact that an agent belongs to a pipeline or a task,
  from the tool's declared category, or from the absence of arguments.

- **R44** — A permission outcome that did not execute its tool is written to
  the server log with the agent id, the tool name, and the decision. Denial, timeout where a
  deadline is configured, and cancellation each produce one entry. Today the only trace that a tool
  never ran is a generic error event inside that one agent's transcript, so a run stalled by a
  withheld tool cannot be diagnosed from the server log at all — the log records every HTTP request
  and every pipeline refusal, but not the decision that stopped the work.

- **R43** — A permission request that does need a human decision is held until
  one is made. By default it no longer auto-denies on a deadline: the agent stays `waiting_input`
  with the request pending and the tool unexecuted, and leaves that state only when a person
  approves or denies it, cancellation claims the turn, or the agent is stopped. Waiting executes
  nothing, so a person who steps away returns to a decision to make rather than to work that failed
  while they were gone. This supersedes R17's always-on deadline. An explicit approval deadline
  remains available for a deployment that wants the previous behavior, and when one is configured
  R17's timed-out resolution, `permission timed out` error, **Timed out** rendering, and turn
  completion are unchanged. A pending request survives a browser reload — reopening the chat shows
  it still pending with Approve and Deny live — and ends on server restart or agent stop exactly as
  it does today; holding it longer does not make it durable across a process end.

### 2.5 Switching runtime from the chat header

- **R23.** For a running chat-interface agent, the header presents backend, model, and
  effort as inline selects populated from the backend catalog (FS-09), rather than static text. This
  is an additional entry point to the switch-runtime operation (FS-01.R13), complementing the
  dashboard Switch dialog (FS-01.R32); it does not change the interface (chat↔terminal), which stays
  in that dialog (FS-01.R15/R32). Choosing a backend resets model to that backend's default model and
  effort to that model's default effort; the effort select appears only for a model that declares
  efforts (FS-09.R37). A selection that differs from the agent's current runtime reveals an explicit
  **Switch** control; nothing is applied until it is activated. Activating it calls switch runtime
  with the chosen `{backend, model, effort}`, which preserves the conversation through native resume
  or a primer (FS-01.R13). A `no_change` (equal to current), rejected, or rolled-back switch
  (FS-01.R26) surfaces an actionable error and returns the picker to the agent's current runtime
  rather than presenting the unselected change as applied. **Effort's participation in this control
  is superseded by R47:** effort leaves the staged picker and applies on selection. The
  backend and model behavior above is unchanged.
- **R24.** When the agent is not running — a stopped or archived session viewed under
  FS-05 — the header shows the runtime identity as static text with no editable picker or Switch
  control, matching switch runtime's running-only rule (FS-01.R13/R26). If the agent's current
  backend or model is absent from the live catalog, the picker still shows the current identity and
  does not break; the **Switch** control stays gated on choosing a listed backend and model.
- **R27.** The chat header **Back** link targets the agent's project dashboard
  (`/project/<project>`) when that project is a current, non-archived catalog member, and otherwise
  the projects home (`/`). An agent naming a project the catalog no longer has (deleted or archived)
  never sends Back to an unavailable/archived route; it returns to the projects home instead.

- **R45.** For a running chat agent whose model declares fast-mode
  capability (FS-09.R50), the header presents a fast-mode toggle showing the agent's current fast
  mode. The toggle is **not** part of the staged runtime picker in R23: it is presented separately,
  it never contributes to the difference that reveals **Switch**, and it is never sent in a switch
  request (FS-09.R56). Activating it applies immediately through its own request rather than
  waiting for a further control, takes effect from the agent's next turn, and restarts no process
  and rebuilds no conversation. The toggle reports its own outcome: it shows that a change is being
  applied, confirms the new state on success, and on failure returns to the agent's actual fast mode
  with an actionable error rather than presenting the unapplied change as active. The applied value
  becomes the agent's stored fast mode, so resume, clone, and switch runtime carry it and the
  archive records it. Because fast mode trades increased provider usage for speed, the control names
  that trade in AgentDeck's own words rather than repeating a provider description — the pinned
  adapters disagree, one naming the usage cost and the other not. It does not interrupt with a
  confirmation: the change is one activation, immediately reversible, and affects only the turns
  taken while it is on.
- **R46.** The header states fast mode honestly in each case a person can
  reach. A model that declares no fast-mode capability shows no toggle at all. An agent that asked
  for fast mode at launch but whose live session did not offer it (FS-09.R55) shows fast mode off
  with the reason that this model does not offer it, so a request that was accepted at launch is
  never silently reported as active. A chat agent that is not running shows its fast mode as static
  text beside backend, model, and effort, matching R24. **Known limitation:** the toggle reports the
  fast mode AgentDeck applied and the person chose. The pinned Claude adapter can suspend fast mode
  on its own during a provider rate-limit cooldown and re-enable it when the cooldown clears, and
  AgentDeck does not yet consume the session update that reports this, so a suspended Claude agent
  still shows fast mode on. The pinned Codex adapter reports no fast-mode state back at all.

- **R47.** **Supersedes R23's effort clause.** For a running chat agent the
  header's effort select applies on selection rather than staging a change behind **Switch**. It
  reports its own outcome exactly as the fast-mode toggle does in R45 — in-flight while applying,
  the server's returned value as truth, and on failure a return to the agent's actual effort with an
  actionable error instead of a selection presented as applied — and the applied value becomes the
  agent's stored effort, so resume, clone, switch, and the archive carry it. The select still appears
  only for a model that declares efforts (FS-09.R37) and still resets when the backend or model
  changes, because both of those are staged changes that have not been applied yet.

  The header therefore reads as two groups with two honest meanings: backend and model stage a
  change and reveal **Switch**, because changing them requires stopping and restarting the provider
  process; effort and fast mode apply immediately, because the providers accept both as ordinary
  session settings on a live session. A person who has learned "changes here need Switch" is not
  misled into pressing it for a setting that has already taken effect, and no one pays a process
  restart for a setting that does not need one.

  Switch runtime keeps accepting effort unchanged (FS-01.R30, FS-09.R56), so no API client changes
  and effort remains part of a combined backend/model/effort switch. Using it to change *only*
  effort still restarts the process — more work than needed, retained deliberately rather than
  removed, because narrowing a shipped request field would be a compatibility break for no gain.

### 2.6 Composer file and ACP command autocomplete

- **R30.** Typing `@` in the composer at a word boundary — the start of the input or
  immediately after whitespace — opens a file picker anchored to the composer. The text typed after
  `@` up to the next whitespace is the query; the picker lists matching files from the agent's
  session working directory, ranked, and capped at a bounded result count. An empty query lists a
  bounded initial list from that directory. `@` typed inside a word (`user@example.com`) does not open
  the picker. Files are listed; directories are not offered as entries.
- **R31.** While the picker is open, ↑/↓ move the highlighted entry, Enter or Tab accepts
  it, and Escape dismisses it. Accepting replaces the typed `@query` with `@` followed by the file's
  path relative to the session working directory, then a trailing space. The result is ordinary
  composer text: it is submitted exactly as displayed, is recorded verbatim in the durable
  `user_text` event, and requires no change to the prompt request contract (R6) — AgentDeck sends no
  structured attachment and embeds no file contents. Enter accepts the highlighted entry only while
  the picker is open; with the picker closed Enter submits per R6, and Shift+Enter always inserts a
  newline.
- **R32.** Autocomplete never blocks, delays, or fails a prompt. If the working directory
  is absent or unreadable, file search fails, no ACP command list has arrived, or nothing matches,
  the applicable picker shows an empty or unavailable state and dismisses on Escape or continued
  typing, while the composer keeps accepting and submitting text exactly as it does without this
  feature. An inserted file mention is plain text, so a file later moved or deleted neither
  invalidates the draft nor blocks the send.
- **R33.** Typing `#` at a word boundary opens a command picker anchored to the composer.
  The text after `#` up to the next whitespace filters the complete command/skill list most recently
  advertised by the running chat agent over ACP `available_commands_update`; entries show the
  advertised name and description and, when present, the input hint. Each later ACP update replaces
  the prior live-session list. Accepting replaces `#query` with `/` followed by the advertised name
  and a trailing space: a Codex skill advertised as `$review` therefore inserts `/$review `. The
  inserted command is ordinary composer text and is submitted and recorded exactly as displayed.
  AgentDeck neither invents commands nor attempts to classify a provider's entries as skills.

### 2.7 Diagram rendering

- **R37**. A fenced code block tagged `mermaid` inside an assistant message renders as a
  diagram in place of the code block, themed from the core presentation values so it reads correctly
  under Core and every built-in skin (TS-08.R40), including when appearance changes while the
  transcript stays open. Because a partially streamed block is not valid
  diagram source, the block renders as the ordinary syntax-highlighted code block (R2) until its
  fence closes, and only then becomes a diagram; the reader therefore never sees a diagram flicker
  or error mid-stream. Once rendered, unrelated transcript state changes such as scrolling do not
  remount or regenerate it. The diagram uses the available transcript width with a bounded height,
  so compact source does not collapse to a tiny intrinsic canvas and wide source remains contained
  by the chat surface. Each rendered diagram offers a control that shows its original source and
  returns to the diagram. Diagram rendering is presentation only: it changes no durable event, no
  sequence, no fold boundary (R4), and no archived or searched content, so the same message replays
  identically after a reload and renders identically in an archived transcript (FS-05.R14).
  Rendering applies only to assistant text. Tool calls, tool results, diffs, user prompts, and
  annotations are unchanged, and AgentDeck neither authors, edits, exports, nor downloads diagrams.

### 2.8 Chat panes on the dashboard

- **R39**. A chat-interface agent's dashboard card can hold a chat pane
  expanded in place (FS-02.R46). Inside that pane the reading, prompting, cancellation, permission,
  autocomplete, draft, and diagram behavior is exactly this spec's: R2–R5, R7–R9, R12, R14–R18,
  R28–R33, R35–R38 all apply to a pane as written for the agent screen, with the pane's own
  transcript scroll region satisfying R3. The pane deliberately carries less than the screen: it
  presents the agent's name and live state, its transcript, and its composer, and it does not
  present the Files, Commands, or Terminal tabs, the context meter, or the inline runtime picker
  (R23). The agent's name in the pane header links to `/agent/:id`, whose behavior — including its
  opening tab (`initialTab`) and its **Back** target (R27) — is unchanged. Live delivery is no
  longer scoped to one agent: R10's stream feeds every open pane and the agent screen together, and
  R12's open-time fetch, reconnect refetch, and sequence-gap refetch apply per open surface, so a
  pane that missed events recovers its own authoritative transcript without disturbing the others.
  Recovery is ordered, not merely issued: a transcript response never removes an event already
  delivered and displayed, and never returns a resolved permission chip to an undecided prompt,
  however long that response took to arrive relative to the live stream. This closes, for panes and
  for the agent screen alike, the overlapping-refetch ordering gap this spec records in §6 — a gap
  that today can only regress one surface but that four concurrent panes would make four times as
  reachable (TS-03.R31).


### 2.9 Opening a file from the conversation

- **R51** — **A file link an agent wrote opens the file.** In assistant
  Markdown, a link whose target is a local filesystem path — relative
  (`internal/state/messages.go`), absolute, or `file://`, optionally suffixed `:line` or
  `:line:col` — opens that file in AgentDeck's file viewer (R52) instead of navigating the browser.
  Today such a link is an ordinary relative anchor, so activating it leaves the agent screen, fails
  to resolve as an application route, and lands on the dashboard after a full reload, losing the
  reader's place in the conversation; that is the defect this closes. `http`, `https`, and `mailto`
  links are untouched. AgentDeck adds no path detection over plain transcript text: a path an agent
  mentions without marking it as a link stays text, so the product never invents a link that
  resolves to nothing. Link handling is presentation of an existing event — it changes no durable
  event, no sequence, no fold boundary (R4), and no annotation target (FS-13.R4), so a message
  replays identically after a reload and in an archived transcript (FS-05.R14).
- **R52** — **The viewer is read-only and holds one file.** Its header
  names the file by its path relative to the session working directory and offers **Reload** and
  **Close**. Its body shows the file's text with line numbers and the same syntax highlighting the
  transcript applies to fenced code (R2). A link carrying a line anchor scrolls that line into view
  and marks it, so a cited line is found rather than hunted for. A Markdown file additionally offers
  **Rendered** and **Source**; the rendered form uses the same sanitized Markdown and diagram
  rendering an assistant message uses (R20, R37, R38) and gains no capability beyond it. Opening
  another file replaces the open one: there is one panel, with no tabs and no history. Content is
  read when the file is opened and when **Reload** is chosen — AgentDeck does not watch or poll the
  file — so the panel states when its content was read, and a file the agent rewrites afterwards
  keeps showing the earlier read until it is reloaded. The viewer never writes, creates, renames,
  deletes, downloads, or runs anything, and offers no directory browsing.
- **R53** — **The viewer sits beside the transcript, never inside it.**
  While a file is open on the agent screen or the archived-agent screen, the viewer is a full-height
  column along the **left** edge of the transcript region and the transcript reflows into the
  remaining width instead of being overlapped; no file content is ever inserted into the
  conversation. When the surrounding region has room, the screen's bounded content width relaxes
  while the viewer is open, so the column occupies width that is empty today and the transcript
  keeps its reading measure; when it does not, the column takes width from the transcript. The left
  edge is deliberate: FS-13.R20's annotation tray docks on the right, so a docked tray and an open
  file can be present at once without contending for the same track. Width follows the transcript
  region itself rather than the browser window, exactly as FS-13.R20 does. In an expanded dashboard
  chat pane (R39, FS-02.R55) no viewer opens — the pane is one grid column wide and deliberately
  carries less than the screen — and a file link there opens the agent screen with that file already
  showing, which is the pane's existing route to the full surface.
- **R54** — **The open file is part of the screen's address.** Opening a
  file sets `?file=` (the path relative to the session working directory) and, when a line was
  cited, `?fileLine=`, on the agent or archived-agent route beside the existing `?tab=` (R27).
  Closing the viewer clears them. The consequences are intended: reloading reopens the same file,
  browser Back closes it, and the address can be handed to another browser on the same machine.
  These parameters only request an open and confer nothing — every read is authorized server-side
  against that agent's own recorded working directory (R55), so a hand-edited parameter cannot widen
  what is readable. A parameter that cannot be resolved opens the viewer in its stated-refusal form
  rather than failing silently, and never blocks the transcript.
- **R55** — **Reads are confined, and every refusal is stated.** A file is
  readable only inside the working directory recorded for that agent's session — the same
  containment R34 already applies to composer file search — so no link, however written, reads a
  path outside it, and a symlink leading out of that directory is refused. A path that resolves
  outside the directory is refused on its form before any file is opened, so the viewer cannot be
  used to test whether a file exists elsewhere on the machine. `.git` is excluded. Files Git ignores
  **are** readable: an agent that wrote a generated file or a log and linked to it produces a link
  that works, with the same reach the agent itself already had and with the person doing the
  clicking. The viewer names the reason for each refusal in its own surface, leaving the transcript
  readable: a path outside the working directory, a file that no longer exists, a recorded working
  directory that is gone (as a long-archived session's may be), a directory rather than a file,
  content that is not text, and a file whose beginning is shown because it exceeds the read limit —
  labelled as a partial read rather than presented as the whole file. A stopped or archived agent
  reads from its recorded working directory exactly as a running one does, because reading back
  through what an agent did is when these links matter most. Nothing here is durable: no database
  row, transcript event, archive content, search document, or browser-stored value is added, and no
  agent-facing surface changes — an agent cannot open the viewer, be shown it, or learn it exists.

- **R56 — A steer that loses the active turn remains host-owned.** If the
  adapter handles a Steer request after no provider turn is still active, it returns a
  no-consumption `promptRequired` outcome instead of launching a detached turn. AgentDeck then sends
  the unchanged message through the ordinary prompt path exactly once, so the normal busy,
  waiting, cancellation, and terminal lifecycle applies and the public outcome remains `new_turn`.
  AgentDeck never retries `startedNewTurn`: that outcome means the adapter may already have consumed
  the message. Steer remains available wherever the runtime advertises it; an advertised adapter
  must provide this host-owned fallback before the planned behavior ships.


## 3. States & transitions

- **Open/reload:** panel fetches durable events → normalizes/folds them → subscribes to live SSE
  deltas. SSE reconnect or a detected seq gap repeats the authoritative fetch.
- **idle → busy:** a prompt or coordination nudge starts a turn and resets that turn's coordination
  budget (FS-06).
- **busy → waiting_input → busy:** a permission request pauses the ACP tool call; approve/deny
  atomically resolves it and lets the turn continue. Timeout or Cancel ends the turn instead.
- **busy → idle:** normal `turn_end`; context percentage and status detail are updated.
- **busy/waiting_input → error:** fatal transport/process failure emits the delivered error/turn-end
  events, removes the running row, and preserves the transcript.
- **stopped/resumed:** the durable transcript stays under the same `agent_id`; FS-01 and FS-05
  govern restoring the runtime and upstream ACP session.

## 4. Edge cases & errors

- **R19.** Empty or whitespace-only prompts are not sent. Invalid JSON or empty prompt bodies at
  the API boundary return `422 validation`; an agent FS-01.R33 cannot wake — an unknown id, a
  terminal agent, an archived agent, an agent without a resumable snapshot, or a
  pipeline-associated agent — returns the applicable `404`/typed runtime error.
- **R35** — The composer of a stopped chat agent remains an enabled **wake surface**:
  submitting sends the prompt, which wakes the agent per FS-01.R33, and the transcript shows the
  user message plus the ordinary busy progression while the runtime re-attaches (wake adds the
  resume latency before the turn starts). A failed wake surfaces the server's typed error through
  the existing chat error surface and preserves the composer draft.
- **R20.** The UI sanitizes assistant Markdown before inserting it into the DOM; transcript content
  cannot inject arbitrary HTML or script through Markdown rendering.
- **R38**. Diagram rendering (R37) upholds R20 rather than excepting it. Diagram source
  is treated as untrusted, because assistant text can carry content the agent read from a repository
  or the network. The rendered diagram is produced with the renderer's own interactivity disabled —
  no click handlers, no scripts, no HTML in labels — and the resulting markup is sanitized again
  before it reaches the DOM, so neither a renderer bug nor a crafted diagram can inject active
  content. Markup produced this way enters the DOM at exactly one reviewed place in the UI, and no
  other transcript renderer gains a raw-markup path. Rendering makes no network request. Source that
  uses Mermaid's external-image node syntax is rejected before the renderer is called, and URL or
  active-content attributes are removed from returned markup before insertion. Source that fails to
  parse or exceeds 50,000 UTF-16 code units leaves the ordinary code block visible with a short note
  that it cannot be rendered; it neither empties the message, nor throws to the event-level error
  boundary (R5), nor blocks the rest of the transcript. The size bound limits ordinary accidental
  cost; the UI does not claim that main-thread Mermaid execution has an interruptible elapsed-time
  deadline.
- **R21.** A permission decision must be exactly `approve` or `deny`; invalid JSON or any other
  decision returns `422 validation`. A failed UI decision leaves the buttons available and shows an
  error instead of optimistically resolving the prompt.
- **R22.** A terminal-interface agent does not use this composer or ACP permission relay. Its panel
  opens on Terminal and directs input/permission handling to the live terminal (FS-07).
- **R25.** A terminal tool result with no displayable `content`, `error`, or `result` remains a
  visible completed tool outcome, but renders as a compact labelled status rather than a blank
  transcript panel. Non-empty and failed results retain R2's inspection and error behavior in both
  live and archived transcripts.
- **R26.** The transcript locally groups each uninterrupted run of normalized
  `tool_call` and `tool_result` events into a collapsed **Ran _n_ tools** row, where _n_ is the
  number of calls. Activating that row reveals the original calls and every non-empty or failed
  result for inspection; any non-tool event ends the run. A successful result with no displayable
  payload is omitted whether it is in a run or arrives alone. This supersedes R25. Grouping changes
  neither durable events, sequence order, live/replay behavior, nor the individual event's
  right-click annotation target.

- **R34.** File search is confined to the agent's session working directory. No query,
  however written, returns or reveals a path outside it; `.git` is always skipped; and, when the
  directory is in a Git worktree, files excluded by Git's effective ignore rules are not listed.
  Traversal and result count are bounded so a very large directory cannot hang the picker, and an
  unknown or non-chat agent returns the applicable `404`/typed error rather than a directory listing.

## 5. Acceptance criteria

- **A1** (R6, R8, R10) — Prompting streams ordered assistant deltas, finishes with `turn_end`, and
  transitions busy→idle with context usage: `internal/runtime/chat_test.go::TestChatStreamText`.
- **A2** (R2, R8) — Tool call, correlated result, and file diff survive ACP normalization:
  `internal/runtime/chat_test.go::TestChatToolFlow`.
- **A3** (R14–R16) — End-to-end HTTP prompt → permission request → approval → tool execution →
  durable transcript: `internal/server/integration_test.go::TestLaunchPromptPermissionFlow`.
- **A4** (R14–R18, R21) — Approve, deny, timeout, auto-approve, unknown call, and single-winner
  resolution: `internal/runtime/permission_test.go::TestPermissionApprove`,
  `TestPermissionDeny`, `TestPermissionTimeout`, `TestPermissionSkip`,
  `TestPermissionUnknownToolCall`, `TestTakePendingSingleWinner`, and
  `TestTakePendingReportsAlreadyResolved`; HTTP/SSE denial completion in
  `internal/server/integration_test.go::TestPermissionDenyReturnsIdleAfterTurnEnd`; server mapping in
  `internal/server/server_test.go::TestPermissionErrorAlreadyResolved`.
- **A5** (R9) — Cancel claims a pending permission, prevents tool execution, records a
  `permission_resolved` (decision `cancelled`) and a cancelled turn, becomes a no-op once idle, and
  identifies a peer killed by escalation as a cancellation-caused process exit:
  `internal/runtime/permission_test.go::TestCancelDuringPendingPermission` and
  `TestCancelEscalatesToSIGINT`.
- **A6** (R4, R10–R12) — Nested wire events normalize, assistant deltas fold on live append and
  replay, and all four permission labels fold on live/replay paths and render distinctly:
  `ui/src/store/transcriptStore.test.ts` and
  `ui/src/components/chat/renderers/PermissionPrompt.test.tsx`.
- **A7** (R7, R13) — Accepted user prompts and delivered partial output remain in both the transcript
  endpoint and NDJSON after a mid-turn process crash:
  `internal/server/integration_test.go::TestCrashMidTurnPersistsDeliveredTranscript`.
- **A8** (R1–R18) — A user launches a fake-ACP chat agent, sends a prompt, observes streaming and
  status transitions, and completes approve/deny/timeout without a stuck prompt: journeys **J3**
  and **J4** in `docs/features/USABILITY-REVIEW.md`.
- **A9.** (R23, R24) — For a running chat agent the header renders backend/model/effort
  selects; choosing a backend resets the model and effort to that backend/model's defaults; the
  effort select appears only for an effort-capable model; a differing selection reveals **Switch**
  and calls `POST /api/sessions/{id}/switch-runtime` with the chosen runtime; and a stopped agent
  renders the static identity with no picker. *Verify by* a new chat-header component test under
  `ui/src/components/chat/ChatPanel.test.tsx`; the browser acceptance pass changes a live fake
  agent's model and verifies that stopping it restores static identity.

- **A10** (R25) — Superseded by R26/A11.

- **A11** (R26) — Two uninterrupted calls and their results initially render one
  **Ran 2 tools** control. Opening it reveals the calls and non-empty/failed results but no
  successful no-payload outcome; intervening assistant text starts a new run. *Verify:*
  `ui/src/components/chat/TranscriptView.test.tsx`.

- **A12** (R27) — The chat header **Back** link points at `/project/<project>` for an agent
  whose project is a current, non-archived catalog member, and at `/` for an agent whose project is
  absent from or archived in the catalog. *Verify:* `ui/src/components/chat/ChatPanel.test.tsx`.

- **A13** (R29) — The transcript renders the waiting indicator when the open agent is `busy` and
  omits it otherwise: `ui/src/components/chat/TranscriptView.test.tsx`.

- **A14** (R28) — Transcript text is selectable and the user bubble's `::selection` colour differs
  from its background. *Verify by* the presentation styles (`.transcript-view` selectable text and
  the distinct `.user-message::selection` rule in `ui/src/styles/features/agent.css`) and a browser
  check that highlighting a sent message shows a visible selection.

- **A15** (R30, R31) — `@` at a word boundary opens the picker and `@` inside a word does
  not; typing filters it; ↑/↓ move and Escape dismisses; accepting inserts `@<relative path>` plus a
  trailing space; Enter accepts while the picker is open and submits the unchanged prompt text once
  it is closed. *Verify by* a new `ui/src/components/chat/Composer.test.tsx`.

- **A16** (R32, R34) — The search endpoint lists a working directory's files, omits
  `.git` and Git-ignored files when the directory is in a worktree, refuses to return any path
  outside the working directory, bounds its result count, and returns `404` for an unknown agent;
  an unreadable working directory leaves the composer able to send. *Verify by* a new server test beside
  `internal/server/files_commands_test.go` and an unavailable-source case in
  `ui/src/components/chat/Composer.test.tsx`.

- **A17** (R32, R33) — A fake ACP session publishes a command list, `#` opens one picker
  containing every advertised entry, later complete updates replace the list, and accepting an entry
  inserts `/<advertised-name>` plus a trailing space (including `/$<skill>`). No update, an empty
  replacement, and a stopped/non-chat session leave prompt entry and submission usable. *Verify by*
  runtime mapping/snapshot tests, a server endpoint test, and `ui/src/components/chat/Composer.test.tsx`.
- **A18** (R35) — With a stopped chat agent open, the composer accepts and submits a
  prompt; the UI issues the ordinary prompt request and renders the user message and busy
  progression from the resulting events, and a rejected wake shows the server error while keeping
  the draft. *Verify:* `Composer`/chat component tests against a mocked prompt route for both
  branches; the end-to-end wake itself is FS-01.A17's server test.
- **A19** (R7, R36) — Distinct text entered in two chat composers survives navigation,
  remount, and page reload and restores only in its matching chat. An accepted prompt and manually
  empty composer remove their drafts; a rejected prompt restores and retains its exact text. A newer
  same-agent draft typed while an earlier send is in flight survives both that send's acceptance and
  its rejection. Agent
  deletion removes its draft, and retaining a twenty-first draft evicts the least recently edited
  entry. No draft is included in a transcript or prompt request before Send. *Verify:*
  browser-storage-backed `Composer` and deletion-event component/unit cases plus a manual
  reload/navigation check.

- **A20** (R37) — A closed ```mermaid fence in an assistant message renders a diagram;
  the same block renders as a code block while its fence is still open, and becomes the diagram once
  the closing fence arrives; neither a parent-only rerender after it settles nor a later streamed
  delta that appends prose after the closed fence replaces it with source or invokes Mermaid again;
  the source toggle returns the original text; a ```mermaid fence in a
  tool result, a diff, or a user prompt renders exactly as it does today. Replaying the identical
  durable events produces the identical rendered output. *Verify by* a new
  `ui/src/components/chat/renderers/AssistantText.test.tsx` covering the streaming, closed-fence,
  toggle, and non-assistant cases, plus a `transcriptStore` replay case.
- **A21** (R38, R20) — Diagram source carrying a script element, an HTML label, an
  event-handler attribute, and a click/interaction directive produces rendered output containing
  none of them and executes nothing; Mermaid external-image node syntax is rejected without invoking
  `parse` or `render` and makes no request; a returned style element or style attribute whose
  URL-bearing token is spelled with CSS identifier escapes is dropped and makes no request;
  a 50,001-code-unit or unparseable source leaves a visible code block with its note and the
  following events still render; and the reviewed injection seam is
  the only place in `ui/src` that inserts renderer-produced markup. *Verify by* injection,
  renderer-spy, request-spy, size-bound, and failure cases in the same renderer test, plus a
  repository check that asserts the single seam.
- **A22** (R37) — A real browser renders a diagram in a live streamed reply and in the
  archived transcript of the same session, correct under Core and Sky & Grove; switching between
  the two while the transcript remains mounted regenerates the diagram with the new palette. A
  compact graph uses the available transcript canvas at a readable scale, a wide graph stays within
  it, and scrolling away from and back to the bottom never exposes source or regenerates either:
  journey **J3** in `docs/features/USABILITY-REVIEW.md`.

- **A23** (R39) — With two chat panes expanded on the dashboard and the
  agent screen closed, an assistant delta for each agent appends to that agent's own pane and to no
  other; a permission request raised in one pane is decidable there and folds to a resolved chip
  (R4); a sequence gap for one pane refetches only that pane's transcript and leaves the other
  pane's rendered events unchanged; and a pane exposes no Files, Commands, Terminal, or runtime
  picker control while the agent screen still exposes all of them. A transcript request that is held
  open while newer deltas arrive, and a stale response that resolves after a newer one, both leave
  the displayed transcript carrying every delivered event and every resolved permission chip. — new
  `sse.test.ts` multi-open and delayed-response cases, a `transcriptStore.test.ts` case applying an
  out-of-order transcript, and a pane render test beside `ChatPanel.test.tsx`.

- **A24** (R40–R42) — Each of the fifteen AgentDeck actions raised as an
  approval request executes with no `waiting_input` transition and no pending request, under a
  launch policy with `skip_permissions` false. A same-named tool advertised by a different MCP
  server, an unnamed approval request, and an ordinary file-edit, shell, and fetch request each
  still enter `waiting_input` and wait for a decision. The auto-allowed call is present in the live
  stream and the durable transcript with the auto-approved shape and an `auto_approve` resolution,
  and it is present again after the transcript is re-read. — `internal/runtime/permission_test.go`
  and `internal/server/integration_test.go`.

- **A25** (R43) — With no approval deadline configured, an undecided
  permission request is still pending, still `waiting_input`, and its tool still unexecuted well
  past the previous 180-second deadline; a decision made after that point resolves it normally and
  the turn continues. Cancelling the turn and stopping the agent each resolve it without executing
  the tool. With a deadline explicitly configured, the previous timed-out resolution, error, and
  rendering are unchanged. — `internal/runtime/permission_test.go` and
  `internal/server/integration_test.go`.

- **A27** (R44) — A denied permission, a cancelled one, and a timed-out one
  under a configured deadline each write one server-log entry carrying the agent id, tool name, and
  decision; an approved one does not claim the tool was withheld. —
  `internal/runtime/permission_test.go`.

- **A26** (R40, R43) — In a browser, a pipeline run whose stage agent reports
  its result advances with no approval prompt shown to the person, while a file edit the same stage
  agent attempts still raises a prompt that waits for them; leaving that prompt undecided for longer
  than three minutes and then approving it lets the stage continue rather than failing it.
  *Verify:* journey **J14** in `docs/features/USABILITY-REVIEW.md`.

- **A28** (R45) — For a running chat agent on a fast-declaring model the
  header renders a fast-mode toggle; activating it applies without any further control, leaves the
  agent running with the same conversation and the same process, and confirms the new state; the
  agent's stored fast mode is the applied value afterwards; a rejected apply returns the toggle to
  the agent's actual fast mode with a visible error rather than showing the change as active; and
  the toggle never reveals **Switch** nor appears in a switch-runtime request body. *Verify by*
  `ChatPanel.test.tsx` and a server test asserting the applied value is persisted and that no
  process restart occurs.
- **A31** (R48) — Sending to a `busy` chat agent is accepted rather
  than refused, holds exactly one message, and delivers it as the next turn once the current turn
  ends, in the agent's transcript exactly once; a second send replaces the held one rather than
  queueing two; the held message is absent from the durable transcript and the search index until it
  is sent; and a task assignment, pipeline stage instruction, and mail wake each still fail closed
  against a busy agent. *Verify by* chat-runtime tests over a `fakeacp` turn that assert the held
  message reaches neither the provider nor the runtime transcript before release and runs exactly
  once after it, the prompt-route tests in `internal/server/queue_steer_test.go`, and the existing
  dispatcher/pipeline tests asserting the refusal is unchanged.
- **A33** (R50) — Against a runtime advertising steering, **Steer**
  delivers into the running turn without ending it, and the message appears once in that turn's
  transcript; against a runtime that does not advertise it, no Steer control is rendered at all while
  Send still queues; Steer with an empty composer and a message held delivers the held one and clears
  the hold; a turn that ends between click and delivery results in the message running as its own
  next turn with that outcome reported rather than silently dropped; and a refused message leaves the
  composer's text in place with the reason. *Verify by* chat-runtime tests over `fakeacp` scenarios
  that advertise and withhold steering plus one that reports the adapter's new-turn outcome,
  `ChatPanel.test.tsx` for control presence, and `Composer.test.tsx` for the two outcome messages,
  the empty-composer promotion, and the refusal.
- **A32** (R49) — Cancelling a turn with a message held sends it as the
  next turn; stopping the agent with an empty composer returns the text to that agent's composer
  draft; stopping it with text already in the composer discards the held message and leaves the typed
  text untouched; and a dashboard restart clears the held message. *Verify by* `Composer.test.tsx`
  for the two stop branches against the existing draft store, and a runtime test for
  cancel-then-send. A browser remount rehydrates the still-live hold and its sequence boundary from
  the dashboard; a dashboard restart clears it because the runtime field is intentionally in-memory.
- **A30** (R47) — Choosing an effort in the chat header applies it
  without revealing or requiring **Switch**, leaves the agent running with the same process and
  conversation, and persists as the agent's effort; a rejected apply returns the select to the
  agent's actual effort with a visible error; changing backend or model still stages and still
  reveals **Switch**, and that switch request still carries effort. *Verify by* `ChatPanel.test.tsx`
  covering both groups, and a server test asserting the applied effort is persisted with no process
  restart.
- **A29** (R46) — A model declaring no fast-mode capability renders no
  toggle; an agent launched asking for fast mode whose session did not offer it renders fast mode
  off with the model-does-not-offer-it reason rather than on; and a stopped or archived chat agent
  renders its fast mode as static text with no toggle. *Verify by* `ChatPanel.test.tsx` and
  `ArchiveAgentPage` header tests.

- **A34** (R51) — In an assistant message, a link whose target is a
  relative path, an absolute path, and a `file://` path each open the viewer with no browser
  navigation and no route change, a `:line` suffix is carried through to the viewer, an `https:`
  link keeps its ordinary behavior, and a bare path written in prose renders as text with no link.
  *Verify by* `ui/src/components/chat/renderers/AssistantText.test.tsx`.
- **A35** (R52) — Opening a file renders its text with line numbers,
  scrolls to and marks a cited line, replaces the open file when a second is opened, re-reads on
  **Reload**, and offers **Rendered**/**Source** only for a Markdown file with the rendered form
  going through the sanitized renderer. *Verify by* `ui/src/components/chat/FileViewer.test.tsx`.
- **A36** (R53, R54) — With a file open, a wide transcript region renders
  the viewer as a docked left column beside a reflowed transcript and a narrow one takes the width
  from the transcript, with the conversation's own content unchanged in both; a file link inside an
  expanded dashboard chat pane navigates to the agent screen with that file open instead of opening
  a panel in the pane; and `?file=`/`?fileLine=` survive a reload and are cleared by **Close**.
  *Verify by* `ui/src/components/chat/FileViewer.test.tsx`,
  `ui/src/components/grid/DashboardChatPane.test.tsx`, and
  `ui/src/components/chat/ChatPanel.test.tsx`, with the rendered docked/reflowed forms exercised in
  journey J3.
- **A37** (R55) — A path outside the session working directory, a symlink
  escaping it, a `.git` path, a missing file, a directory, non-text content, and an absent working
  directory each return their typed refusal without opening anything outside the directory; an
  oversized file returns its labelled partial read; a Git-ignored file inside the directory reads
  successfully; and each outcome renders as its stated reason in the viewer while the transcript
  stays usable. *Verify by* `internal/server/fileread_test.go` and
  `ui/src/components/chat/FileViewer.test.tsx`.
- **A38** (R56) — A fake adapter whose active turn settles before it
  handles a Steer request returns `promptRequired` without emitting a detached turn; AgentDeck
  submits the unchanged text once through the ordinary prompt path, reports `new_turn`, keeps the
  agent busy until that prompt's terminal event, and lets Send and Cancel arbitrate against that
  host-owned turn. The transcript contains one user message and one terminal outcome. A provider
  that returns `startedNewTurn` is covered as an incompatible legacy contract, not as a passing
  fallback. *Verify by* focused runtime and route tests over a fake ACP scenario plus the Composer
  outcome test.


## 6. Deviations & open decisions

- **Transcript-load failure is silent in the panel.** The initial `getTranscript` rejection is
  swallowed, leaving an empty transcript until a later SSE event/refetch. Prompt, cancel, and
  permission mutation failures are surfaced as required above; initial history-load diagnostics are
  an open UX gap.

- **Confirmed AgentDeck-action approval boundary.** R40–R44 exempt AgentDeck's own fifteen actions
  from the approval gate and stop the default auto-deny. They add no per-tool, per-role, per-stage,
  or per-template autonomy setting, do not change what `skip_permissions` means for any other tool,
  do not pre-authorize anything at the provider CLI, and add no agent-facing tool, argument, result,
  or knowledge change. An approval request AgentDeck cannot identify keeps prompting. R43's
  indefinite hold is safe only because FS-14.R54 makes a run waiting on an undecided request say so;
  the two ship together.

## 7. Traceability

- **Runtime/events:** `internal/runtime/chat.go` (`Start`, `Resume`, `SendPrompt`, `Cancel`, `emit`,
  ACP update normalization), `internal/runtime/event.go`, `internal/runtime/permission.go`.
- **HTTP/persistence:** `internal/server/sessions.go` (`handlePrompt`, `handleTranscript`,
  `handleCancel`, `handlePermission`), `internal/transcript/writer.go`, `reader.go`.
- **Composer autocomplete (R30–R34):** `ui/src/components/chat/Composer.tsx` owns one
  reusable picker for `@` files and `#` ACP commands. Bounded working-directory search and the live
  command-snapshot read are session-scoped server handlers beside `internal/server/files_commands.go`;
  `internal/runtime/acpmap.go` decodes command updates into replace-only live runtime state. Inserted
  files and commands are plain composer text, so `SendPrompt` and the prompt route are unchanged.
- **UI:** `ui/src/components/chat/ChatPanel.tsx`, `Composer.tsx`, `TranscriptView.tsx`,
  `renderers/`, `ui/src/store/transcriptStore.ts`, `ui/src/api/sse.ts`. The header runtime picker
  (R23/R24) reuses `switchRuntime` (`ui/src/api/client.ts`) and the same backend/model/effort reset
  logic as the dashboard Switch dialog (`ui/src/components/grid/CardContextMenu.tsx`).
- **Diagram rendering (R37–R38):** one branch in
  `ui/src/components/chat/renderers/AssistantText.tsx`'s existing `code` component override, which
  already receives the fence's language tag and its source span. `renderers/MermaidDiagram.tsx` owns
  the only sanitized-markup insertion seam and `renderers/mermaid.ts` owns the size bound, the
  refused external-image grammar, strict initialization, and the DOMPurify pass; the shared
  `renderers/CodeBlock.tsx` draws both the ordinary fence and the source/fallback view. The renderer
  library is loaded on demand so it stays out of the initial bundle, and its theme is supplied by the
  presentation adapter seam in `ui/src/presentation/integrations.ts` beside `syntaxTheme` and
  `xtermTheme` (TS-08.R13/R40). Durable events, `transcriptStore` folding, the transcript endpoint,
  and the archive are unchanged. Regressions:
  `ui/src/components/chat/renderers/AssistantText.test.tsx`, the diagram fold case in
  `ui/src/store/transcriptStore.test.ts`, and the `diagram_stream`/`diagram_injection` scenarios in
  `internal/runtime/testdata/fakeacp` that drive J3.
- **Browser-local drafts (R36):** `ui/src/components/chat/drafts.ts` owns the bounded browser
  record; `Composer.tsx` restores and writes it, and `ui/src/api/sse.ts` removes it only with the
  existing deleted-agent event. `drafts.test.ts`, `Composer.test.tsx`, and `sse.test.ts` cover
  restore, pruning, send outcomes, malformed/unavailable storage, and deletion cleanup.
- **File links and the file viewer (R51–R55):** the `a` component override
  `ui/src/components/chat/renderers/AssistantText.tsx` does not have today, beside its existing
  `code` override, decides link handling; `renderers/filePath.ts` owns the one place a link target
  is classified as a local path and its `:line` suffix parsed. `ui/src/components/chat/FileViewer.tsx`
  is the panel and renders through the shipped `renderers/CodeBlock.tsx` and the shared Markdown
  component extracted from `AssistantText.tsx`, so sanitization and diagram rules stay
  single-sourced. `TranscriptView.tsx` places the panel in `.transcript-wrap`'s grid opposite
  `AnnotationTray.tsx` and carries the pane's navigate-instead behavior as one more per-surface prop
  beside `annotationsEnabled`. The read is `internal/server/fileread.go` sharing
  `internal/server/filesearch.go`'s `withinRoot` containment, with `getFileContent` beside
  `getTrackedFiles` in `ui/src/api/client.ts`. No durable event, store, or persisted browser key is
  added; `?file=`/`?fileLine=` on the route is the open-file state.
- **Key regression tests:** `TestChatStreamText`, `TestChatToolFlow`,
  `TestLaunchPromptPermissionFlow`, `TestPermissionApprove`, `TestPermissionTimeout`,
  `TestCancelDuringPendingPermission`, `TestCrashMidTurnPersistsDeliveredTranscript`,
  `ui/src/store/transcriptStore.test.ts`, and
  `ui/src/components/chat/ChatPanel.test.tsx` (R23/R24/R27),
  `ui/src/components/chat/renderers/ToolResult.test.tsx` (R25), and
  `ui/src/components/chat/TranscriptView.test.tsx` (R29 waiting indicator).
- **Steering lifecycle (R50/R56, A33/A38):** `internal/runtime/chat.go` owns the
  active-turn gate and steer handoff; `internal/runtime/queue_steer_test.go` and the fake ACP
  scenarios cover active injection, the no-consumption idle fallback, Send arbitration, Cancel, and
  exactly-once terminal delivery. `internal/server/queue_steer_test.go` and
  `ui/src/components/chat/Composer.test.tsx` cover the public outcome and message presentation.
- **Composer autocomplete regression tests:** `ui/src/components/chat/Composer.test.tsx`
  (R30–R33 picker/insert/unavailable-source), `internal/runtime/acpmap_test.go`
  (`TestDecodeAvailableCommands`, `TestAvailableCommandsSnapshotReplaceOnly`, R24 snapshot),
  `internal/server/files_commands_test.go` (`TestFileSearchGitWorktree`, `TestFileSearchCap`,
  `TestFileSearchUnreadableDir`, `TestFileSearchNonChatAgent`, R34 containment/ignores), and
  `internal/server/integration_test.go` (`TestAvailableCommandsSnapshotEndpoint`, A17).
