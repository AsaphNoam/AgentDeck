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

- **Fix card drag-and-drop usability.** Dashboard card reorder (FS-02.R12) technically works but
  reads as broken: the drag listener is bound only to the tiny 28×28px `::` handle
  (`AgentCard.tsx`), not the card itself, so dragging the card body does nothing — and because the
  card's `onClick` navigates to the agent, a failed drag attempt looks like an accidental page
  change. Dropping a card onto another group's section also doesn't move it between groups (`order`
  is one flat array independent of `group`), so it silently snaps back. Needs whole-card drag (with
  an activation distance so a plain click still opens the agent) and either real cross-group drop
  support or a clear affordance that drag only reorders within a group.
- **No Content-Security-Policy.** Neither the Go server nor `ui/index.html` sets a CSP header or
  meta tag. CSP is the third layer every diagram/Markdown-rendering hardening guide recommends
  alongside sanitization, and it would also bound the existing Markdown, diff, and xterm surfaces.
  Kept separate from `docs/ready-changes/mermaid-diagrams-in-chat.md` on 2026-08-27 because it is a
  server-wide change that could affect the dev proxy, xterm, and inline styles. Noted while
  researching safe diagram rendering.

- **Approval notifications link to the conversation.** When a pop-up notification fires because an
  agent needs approval, make it a link that opens that agent's conversation, so the user can jump
  straight to the pending permission instead of hunting for the right agent.

- **A pipeline agent stays unmailable forever after its run ends.** `stoppedWakeGates`
  (`internal/state/messages.go:44`) excludes any agent with a `pipeline_attempts` row from the
  stopped-wakeable set, and that row survives as long as its `pipeline_runs` record. So once a
  stage agent stops, no `send_message` or `create_task` can ever wake it again — including long
  after the run reached `completed` or `stopped` — unless a person resumes it by hand or deletes
  the run. FS-06.R22's stated reason is that the stage state machine deliberately stopped the
  agent, which only holds while that state machine still owns it. The operator called this broken
  on 2026-09-05. The fix should scope the exclusion to an attempt whose run is still active
  (`state NOT IN ('completed','stopped')`, the same predicate `ListActivePipelineRuns` already
  uses) rather than to the mere existence of an attempt row, and keep the exclusion's two
  consumers — `AddressableAgents` and `StoppedWakeCandidates` — reading one shared SQL spelling
  (INV §2). Check the FS-06.R29 refusal wording and the FS-15.R17 divergence it names at the same
  time. Needs FS-06.R22 updated, not just code.

## Ideas being defined

These are worth shaping into a possible change, but are not ready to build. Defining an idea updates
the relevant feature and technical specifications; it does not change product code.

- **Show the agent's thinking, not only its tool use.** Requested 2026-09-07: the Codex app shows
  the steps and reasoning an agent takes; AgentDeck's transcript shows tool calls and final text
  only. Verified 2026-09-07: this is a deliberate drop, not a provider gap —
  `internal/runtime/acpmap.go:297` discards `agent_thought_chunk` **and** `plan` updates as
  "dropped this phase", and both pinned adapters emit them (`codex-acp` maps
  `item/reasoning/summaryTextDelta`, `item/reasoning/textDelta`, and
  `item/reasoning/summaryPartAdded` to `agent_thought_chunk`, and `item/plan/delta` /
  `turn/plan/updated` to plan updates; `claude-agent-acp` emits `agent_thought_chunk` too). So the
  work is a new normalized event plus its persistence and rendering, not a protocol problem.
  **Decided 2026-09-07:** thinking is **live-only** — streamed to the open chat and not persisted.
  It therefore never reaches the durable transcript, the archive, the FTS index, FS-13 annotation
  targets, or FS-15 context pulls, and reopening a conversation shows no reasoning. Still open:
  default collapsed or expanded rendering, and whether `plan` ships in the same slice.

- **Steer a running turn from the chat composer.** Requested 2026-09-07: type a follow-up while the
  agent is working, as the Claude and Codex CLIs allow. Today `SendPrompt` refuses outright —
  `ErrTurnInFlight` (`internal/runtime/chat.go:355`) becomes a `409` (`internal/server/sessions.go:554`)
  and the composer shows Cancel instead of Send while busy (FS-03.R9,
  `ui/src/components/chat/Composer.tsx:253`). Provider support verified 2026-09-07 and it is
  **asymmetric**:
  - `claude-agent-acp` 0.59.0 supports it natively: `Session.turnQueue` is a FIFO of in-flight
    prompts, a second `session/prompt` is pushed to the SDK immediately and echoed back in
    submission order, and the adapter carries explicit orphan/cancel accounting for queued turns.
  - `codex-acp` 1.1.2 does **not** queue — it **supersedes**. `prompt()` clears
    `sessionState.currentTurnId` on entry and calls `this.activePrompts.set(sessionId, activePrompt)`,
    overwriting the entry belonging to the running prompt. The displaced prompt then fails
    `promptShouldStop()` (`activePrompts.get(sessionId) !== activePrompt`), which routes its turn to
    `interruptLateStartedTurn()`. A second concurrent `session/prompt` therefore interrupts the turn
    in progress rather than following it — worse for steering than today's `409`.
  - The Codex gap is adapter-side, not a Codex limitation. The Codex app-server protocol has
    first-class `turn/steer` and `thread/queue` methods (both present in the `codex` 0.152.0 binary;
    `codex queue --thread --message` is their CLI surface), and `codex-acp` 1.1.2 wires up neither —
    its app-server method set is `turn/start`, `turn/interrupt`, `turn/completed`, `turn/started`,
    `turn/diff/updated`, `turn/plan/updated`, `turn/moderationMetadata`. Codex steering arrives with
    an adapter version bump, not a protocol redesign; re-check for `turn/steer` in `codex-acp`
    before concluding Codex cannot steer.
  So a portable design cannot assume queueing today. Open product decisions: whether steering is
  adapter-native queueing (Claude only, honest capability split like FS-09.R39) or an AgentDeck-side
  hold-until-idle that works everywhere but is deferred send rather than steering; whether a queued
  steer can be withdrawn; and whether the relaxation is person-only — `ErrTurnInFlight` is currently
  load-bearing for coordination and pipeline arbitration (`internal/server/messaging_loops.go:77`,
  `:156`), so agent-initiated activations should almost certainly keep failing closed.

- **Edit a sent chat message.** From the 2026-08-10 play session: like Codex, editing the most
  recent message edits it in place, and editing an older one forks the conversation from that point.
  Designing this on 2026-08-27 established that AgentDeck cannot give it the meaning Codex does, and
  the user chose to hold the idea rather than ship a weaker meaning under the same name. Codex owns
  its own conversation state; AgentDeck supervises a provider CLI session that has already ingested
  the message. Findings, so a later attempt does not re-derive them:
  - **The protocol has no rewind.** The ACP client is hand-rolled and pins protocol version 1. The
    complete implemented session-method set is `session/new`, `session/load`, `session/prompt`,
    `session/cancel`, `session/request_permission`, `session/set_config_option`, and
    `session/update`. Nothing edits, rewinds, truncates, forks, or resumes from a point;
    `session/load` is a whole-session resume keyed by id and takes no offset or sequence.
  - **The transcript is append-only.** `internal/transcript/writer.go` exposes only
    `Append`/`Sync`/`Close`, and `Open()` repairs a torn trailing record on the assumption that only
    the final line can ever be incomplete (TS-02.R8, INV §9).
  - **The search index is immutable per turn.** One FTS document per completed turn, never reread or
    rewritten (TS-02.R16), and one document blends several messages by seq range — so an edited
    message's original text stays searchable no matter what the UI shows. FS-05.R32 names the
    transcript and search index as things archival may not change.
  - **The only rewind-shaped seam is lossy.** The switch-runtime history primer
    (`internal/server/primer.go`) rebuilds a session from an 8000-token budget: the last six turns
    verbatim and a summary of everything older. Using it to fix a typo would silently discard the
    agent's context.
  Anything shippable is therefore one of: correct-and-resend as a new turn with the supersession
  stated honestly; a true session rebuild that accepts the primer's context loss; or a display-only
  supersede that makes the visible history stop matching what the agent received. Each needs a
  product decision about what "edit" should promise.
- **Richer agent-facing orchestration API (remainder).** The first slice — typed retry
  classification on refused tool calls and structured result delivery — is shipped in FS-17.
  Investigation of the original idea found that most of what it asked for had already shipped:
  tools return typed JSON with stable codes, `create_task` arms already register durable host-managed
  waiting instead of polling, and `get_assigned_task` already returns a task's own context-reference
  ids with per-attachment presentation. What remains unbuilt, each needing its own product decision:
  - **Agent-side re-arm and retry.** `POST /api/tasks/{id}/rearm` and `/retry` exist for people but
    have no MCP counterpart, so an agent told `retry_requires_rearm` cannot act on it.
  - **Work inspection.** Reading work you created or are assigned to. FS-16 §6 and TS-04.R29
    deliberately exclude any task-graph query as anti-polling; on 2026-08-25 the user chose to hold
    that exclusion. Reversing it needs a reason stronger than convenience.
  - **Lifecycle control.** Agent-callable stop, resume, or launch of another agent without going
    through a task. Largest new authority surface; no threat model yet.
  - **Group fan-out.** Multiple arms already give fan-in/join; creating several related tasks as one
    unit does not exist. TS-10 §5 excludes it.
  - **Declared tool output schemas.** Deferred until the pinned Claude and Codex adapters' handling
    of `outputSchema` is verified.
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
