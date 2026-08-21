# AgentDeck ideas and improvements

This is a place to keep future thoughts without accidentally treating them as promises or approved
work. The specifications describe the product today; this file does not.

## New ideas

Put a half-formed thought here. It needs only a short title and, if useful, a sentence about what
prompted it.

Example:

```md
- **Pinned agents.** Let people keep frequently used agents at the top of the dashboard.
```

- **Projects page problems (new design).** Remaining redesigned-projects-page issues (the context
  menu and six-preset colors were promoted to
  `docs/ready-changes/project-context-menu-and-preset-colors.md`; "create a new project from the
  projects page" moved to Ideas being defined):
  1. The new design looks bad and needs rework.
- **Fix card drag-and-drop usability.** Dashboard card reorder (FS-02.R12) technically works but
  reads as broken: the drag listener is bound only to the tiny 28×28px `::` handle
  (`AgentCard.tsx`), not the card itself, so dragging the card body does nothing — and because the
  card's `onClick` navigates to the agent, a failed drag attempt looks like an accidental page
  change. Dropping a card onto another group's section also doesn't move it between groups (`order`
  is one flat array independent of `group`), so it silently snaps back. Needs whole-card drag (with
  an activation distance so a plain click still opens the agent) and either real cross-group drop
  support or a clear affordance that drag only reorders within a group.
- **Approval notifications link to the conversation.** When a pop-up notification fires because an
  agent needs approval, make it a link that opens that agent's conversation, so the user can jump
  straight to the pending permission instead of hunting for the right agent.
- **Pull-based context links.** Agents should be able to reference and consume useful context
  produced by other agents without requiring that context to be copied into mailbox messages or
  proactively injected into their conversations.

  Potential context sources could include another agent's transcript or selected turns, task/output
  artifacts, diffs, files, summaries, or other durable AgentDeck-managed information.

  The important idea is signal/reference + pull, rather than automatically pushing large amounts of
  context between agents.

  For example, Agent A might complete an investigation. Agent B could receive or discover a
  reference to A's work and retrieve the relevant output, transcript section, diff, or artifact when
  it actually needs it.

  This should lean on the context/artifact plane rather than treating inter-agent context transfer
  as ordinary conversation.

  Think about what a first-class “context link” should mean in AgentDeck, how an agent discovers or
  receives one, how granular retrieval should be, what should be durable, and how it should interact
  with existing transcripts, mailboxes, task groups, roles, and MCP capabilities.
- **Dependency-aware / armed agents.** AgentDeck should understand dependencies between pieces of
  work natively.

  An agent or task should be able to exist in a state roughly like:

  “ready to run once these prerequisites are satisfied”

  rather than relying on another LLM to repeatedly check whether prerequisite agents have finished,
  or on agents sending conversational “I'm done” messages purely to advance orchestration.

  Examples include:

  - start B when A finishes;
  - start D after A, B, and C have all completed;
  - unblock a reviewer after an implementation reaches a terminal state;
  - allow a pipeline or task graph to advance based on explicit AgentDeck state transitions.

  This is fundamentally a control-plane capability. The host should observe prerequisite state and
  advance work without needing an LLM to poll, wait, relay status, or interpret routine completion
  signals.

  Dependencies should attach to durable work, result, and attempt state—not merely to an agent's
  lifecycle status. An agent becoming idle, done, or error is not equivalent to a task succeeding,
  failing, or being cancelled. The future scheduler should advance from explicit durable outcomes
  and may reuse the existing pipeline run/attempt/result machinery where it fits, rather than
  turning agent lifecycle signals into a new social completion protocol.

  Think about how this should relate to AgentDeck's existing concepts of agents, groups, pipelines,
  statuses, wake/resume behavior, task completion, and lifecycle.
- **Richer agent-facing orchestration API.** I also want to consider exposing a more semantic
  orchestration API to agents.

  Today, some orchestration behavior is expressed through relatively low-level mechanisms such as
  messaging and lifecycle operations. That often forces agents to implement orchestration protocols
  themselves through prose: ask another agent to do something, poll it, wait, inspect messages,
  infer completion, pass context manually, and so on.

  Instead, agents should increasingly be able to express orchestration intent directly to AgentDeck,
  with AgentDeck executing the deterministic parts through its control and context planes.

  Conceptually this might eventually include operations in areas such as:

  - launching or assigning agents;
  - defining dependencies;
  - linking or exposing context;
  - waiting on structured work state;
  - requesting verification or review;
  - inspecting agent/task state;
  - stopping or resuming agents;
  - coordinating groups of work.

  The exact API surface is not decided. I do not want to jump directly to a large workflow engine
  or an over-designed DSL.

  Typed orchestration outcomes and host-managed waiting should be first-class design requirements.
  Operations should return structured states such as `started`, `armed`, `blocked`, `target_busy`,
  and `dependency_failed`, plus a structured `retry_when` condition where applicable, instead of
  prose that another model must interpret. “Wait for X” should register durable control-plane
  waiting/subscription state and let AgentDeck wake the agent when the condition changes; it should
  not ask the LLM to poll repeatedly.

  Consider where the boundary should sit between a small set of composable primitives and useful
  higher-level operations. The API should expose AgentDeck's orchestration semantics cleanly rather
  than making agents reconstruct those semantics themselves through messages and polling.

  AgentDeck already has useful foundations for this direction: persistent agent identity, resumable
  provider sessions, durable transcripts, mailboxes, lifecycle management, groups/pipelines,
  MCP-based agent communication, and a distinction between running and stopped agents. These
  improvements should build on the good abstractions already present rather than replace them
  wholesale.

### From play session 2026-08-10

- **Crash protection for bad projects.** AgentDeck created invalid projects that broke the process.
  Add input validation on project creation and guard the process against malformed project state.
- **Adjustable chat window size.** Let the chat window be resized.
- **Edit message (fork on older).** Like Codex: editing the most recent message edits it in place;
  editing any older message forks/copies the conversation from that point (this is the "split
  conversation" mechanism).
- **'Approve for me' mode.** Like the agents already have — just expose the toggle, with the default
  persisting to whatever was last selected.

## Ideas being defined

These are worth shaping into a possible change, but are not ready to build. Defining an idea updates
the relevant feature and technical specifications; it does not change product code.

- **Real-provider acceptance.** Run the credentialed OpenCode/OpenHands and Claude/Codex federation
  checks, then reconcile any observed provider incompatibility before making release claims.
- **AgentDeck product knowledge MCP.** Define a versioned, non-secret `agentdeck_docs` topic service
  for AgentDeck roles, including ownership, registration, and acceptance checks.
- **Detached configuration import.** Define verified copyable fields/assets and provider injection
  paths before implementing detached import.
- **Activity map.** Explore a repository/session activity view using server APIs only, with clear
  privacy, scale, and normal-user value boundaries.
- **API authentication / multi-user boundary.** Revisit local API authentication only with an
  explicit threat model and UI/CLI handshake design.
- **Operational CLI.** Complete the specification for dashboard control, install/update, pidfile
  concurrency, and actionable startup diagnostics.

## Known things to improve

These describe incomplete or deliberately limited shipped behavior. Their owning specification is
the authority; move an item to ready changes only after its exact requirements and acceptance checks
are clear.

- **Local API authentication.** The loopback API currently relies on same-machine trust. Revisit a
  token or browser/UI handshake only if the security benefit outweighs its setup and compatibility
  cost.
- **Child-process environment.** Agent processes currently inherit the full environment except for
  backend strip keys. Revisit an allowlist only if it can preserve required provider compatibility.
- **Chat history fidelity.** Make replayed streaming deltas match live deltas; prevent overlapping
  transcript reloads from winning out of order; show initial-load errors.
- **Archive and tracking usability.** Add UI pagination; refresh visible files/commands without
  stale-request overwrite; and let hook-only activity update recency.
- **Cross-turn transcript search.** Turn-document indexing intentionally chooses a small design over
  the more complete segmented model: all query terms and quoted phrases must occur within one turn,
  annotation flush, metadata document, or migrated legacy document. It does not combine a term from
  one turn with a term from another, does not match a phrase across turn boundaries, and one
  pathologically large turn can still use substantial temporary memory. Revisit with size-bounded
  chunks, boundary overlap, and per-term session aggregation only if real sessions show oversized
  individual turns or users repeatedly fail to find conversations because their query spans turns;
  those additions otherwise impose more schema, ranking, pagination, and phrase-boundary machinery
  than the observed long-session rewrite problem warrants.
- **Coordination liveness.** Scope nudge cooldowns to a generation, limit repeated nudges, republish
  unread counts after janitor expiry, notify only on the first budget breach, and remove duplicate
  permission notices.
- **Terminal capability honesty.** Codex works as a chat backend, but its terminal interface is
  intentionally rejected until a Codex-specific interactive-CLI hook/flag path is verified; terminal
  agents are not messageable for the same reason. Also either add an optional driver picker or stop
  advertising unreachable drivers; implement or retire the planned tab cap; and bound aggregate
  shutdown grace across multiple agents.
- **Federation UI and watches.** Expose custom roots/profiles, refresh the effective view after
  source events, register prompt watches after binding, and clear preview consent on project change.
- **Backend launch diagnostics.** Use executable overrides consistently, bound ACP readiness,
  and provide provider-specific missing/old CLI guidance.
- **HTTP compatibility.** Decide and specify how mixed legacy error envelopes should converge.
- **Frontend state ownership.** Define Zustand/React Query ownership and mutation-error behavior
  before broad frontend refactors.
- **Lifecycle and process hardening.** Corroborate process identity, scope crash cleanup by
  generation, serialize concurrent events, and define/test detached-start pidfile races.
- **Local filesystem hardening.** Decide whether startup repairs existing descendant modes and
  whether valid-name role/project files may be symlinks; add adversarial tests for the chosen rules.
- **HTTP request-size limits.** Define shared JSON request limits and the structured over-limit error
  before enforcing them.
