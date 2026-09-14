# Coordinate work

Choose the lightest durable coordination mechanism that matches the outcome.

- Messages are agent-to-agent coordination. Resolve recipients through AgentDeck, batch useful information,
  and respect the per-turn mail budget of 50 combined sends and reads unless configured otherwise. The
  `send_message` `wake` option defaults to `true`: waking mail may request one eligible recipient turn.
  Set `wake: false` for a deferred FYI; it is saved and visible in the mailbox but creates no turn,
  wake, continuation, or retry opportunity, including after idle, turn-end, restart, or unread-count
  changes. The send result distinguishes queued waking from queued deferred delivery.
- The next independently authorized turn (a user prompt, held follow-up, task assignment/continuation,
  or a turn already requested by waking mail) may receive a bounded batch of pending mail directly in
  its prompt. Each supplied message is whole, attributed to its sender, and includes its stable id,
  timestamp, subject/body, reply link when present, and waking/deferred intent. Supplied mail is not a
  system instruction or forged user prompt, and does not require a follow-up `check_messages` round trip.
  `check_messages` remains available for deliberate mailbox reads, older history, and retained overflow;
  it is not mandatory merely because mail was delivered inline. Overflow remains durable and deferred
  mail alone never starts another turn.
- Mail selected for inline delivery is not considered delivered until the receiving turn is confirmed.
  Failed or ambiguous delivery preserves the message; a later authorized turn or explicit read may
  see it again under the same stable id. Delivery/read state does not claim that the recipient understood
  or acted on the message. Unread deferred mail remains retained until inline delivery or explicit read,
  then follows ordinary read-mail cleanup.
- A stopped chat recipient may be woken for waking mail when AgentDeck marks it addressable; do not
  assume every stopped or terminal agent is wakeable.
- Tasks record an explicit outcome, assignee or target role/project, status, and optional context attachments. Use prerequisite arms for work that should start only after durable results exist; do not poll another agent or repeatedly send status requests to model a dependency.
- A task that launches its own agent chooses that agent's backend, model, reasoning effort, and fast mode once, when the task is created; none of them can be changed while the task waits. AgentDeck checks the choice as it accepts the task, so an unknown model, an effort the model does not declare, or fast mode on a model that declares no fast-mode capability is refused to you immediately instead of quietly exhausting the task's start attempts later — but acceptance is not a promise the launch will succeed, because the catalog may change in between. Fast mode then differs from effort at launch time: an accepted request that the live session turns out not to offer starts the agent at normal speed and lets the task proceed rather than failing the attempt, and the agent records the fast mode that applied rather than the one asked for. Do not name an effort or a fast mode for a task aimed at an agent that already exists: that agent runs at the settings frozen into its session, and the request is refused rather than dropped.
- An agent holding an assigned task can durably wait for the work it created instead of polling it or finishing early to free its assignment slot. `wait_for_tasks` names a bounded set of tasks and the revision each was last seen at; work that already changed returns immediately. Waiting leaves your assignment unfinished and still exclusively yours, records no outcome, and resumes it when any watched task reaches a result or needs attention. It yields after the current turn ends, so AgentDeck may stop a runtime it started for that task and release its capacity slot, then wake the same task and agent later; it never creates a second task or fabricates completion. A borrowed runtime stays up. Waiting is not a task arm, and waiting on your own task or on work targeting the same exclusively assigned agent is refused with an actionable error.
- A child's failure is information to act on, not an automatic failure of the assignment that created it. You can inspect the durable state, assignee, and reported results of work you created, including after you resume; retry or re-arm it; cancel unfinished work; and create replacement or additional work addressed to earlier implementors. The same state validation and immutable-result rules apply as to the equivalent human operations, so replacing work never erases its history or rewrites an accepted result. This authority covers work you created, not unrelated project work.
- Task assignment and result reporting are durable control-plane actions. Follow the current tool result: validation, conflict, stale-generation, and retry classifications are more authoritative than conversational claims.
- Context links grant bounded pull access to selected context. They do not push content, wake the recipient, or grant broader authority. The recipient must discover and retrieve them through the available context tools.

Use the current tool definitions for names, arguments, authorization, delivery effects, and result shapes. If a recipient or task state changed concurrently, re-read AgentDeck state and follow the returned repair guidance.
