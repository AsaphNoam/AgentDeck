# Coordinate work

Choose the lightest durable coordination mechanism that matches the outcome.

- Messages are immediate agent-to-agent coordination. Resolve recipients through AgentDeck, batch useful information, and respect the per-turn mail budget of 50 combined sends and reads unless configured otherwise. A stopped chat recipient may be woken for unread mail when AgentDeck marks it addressable; do not assume every stopped or terminal agent is wakeable.
- Tasks record an explicit outcome, assignee or target role/project, status, and optional context attachments. Use prerequisite arms for work that should start only after durable results exist; do not poll another agent or repeatedly send status requests to model a dependency.
- A task that launches its own agent chooses that agent's backend, model, and reasoning effort once, when the task is created; none of them can be changed while the task waits. AgentDeck checks the choice as it accepts the task, so an unknown model or an effort the model does not declare is refused to you immediately instead of quietly exhausting the task's start attempts later — but acceptance is not a promise the launch will succeed, because the catalog may change in between. Do not name an effort for a task aimed at an agent that already exists: that agent runs at the level frozen into its session, and the request is refused rather than dropped.
- Task assignment and result reporting are durable control-plane actions. Follow the current tool result: validation, conflict, stale-generation, and retry classifications are more authoritative than conversational claims.
- Context links grant bounded pull access to selected context. They do not push content, wake the recipient, or grant broader authority. The recipient must discover and retrieve them through the available context tools.

Use the current tool definitions for names, arguments, authorization, delivery effects, and result shapes. If a recipient or task state changed concurrently, re-read AgentDeck state and follow the returned repair guidance.
