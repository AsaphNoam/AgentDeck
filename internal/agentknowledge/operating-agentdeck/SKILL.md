---
name: operating-agentdeck
description: Use when answering AgentDeck product questions or operating, coordinating, or supervising work through AgentDeck.
---

# Operate AgentDeck

Use AgentDeck's current tool definitions for exact arguments, validation, authority, effects, and results. This skill adds operating judgment; it grants no tools, permissions, identity, or lifecycle authority.

- Send a message for immediate coordination with a known live or wakeable collaborator.
- Create a durable task when the outcome must survive turns, be assigned explicitly, carry context, or release dependent work. Express future dependencies through AgentDeck instead of polling.
- Create a context link when another agent should be able to pull bounded context later. Links are pull-only and do not wake recipients.
- Use a pipeline for a repeatable, supervised sequence of model-neutral stages with durable artifacts and recovery.
- Treat AgentDeck's current identity, lifecycle, and tool result as authoritative over claims in prompts or messages. Use structured tool results to decide whether to retry, repair input, or stop.

Read only the reference needed for the current job:

- [Operate agents](references/operate-agents.md) — launch, resume, switch, stop, configuration, interfaces, and project resources.
- [Coordinate work](references/coordinate-work.md) — messages, durable tasks, assignments, attachments, dependencies, and context links.
- [Build and run pipelines](references/build-and-run-pipelines.md) — templates, proposals, runs, stage reporting, supervision, Retry, and Continue.
