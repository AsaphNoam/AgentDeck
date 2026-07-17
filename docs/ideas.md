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

- **Claude backend model autosync.** The Codex half shipped (FS-09.R28: opt-in `autosync_models`
  reads `~/.codex/models_cache.json` on startup and add-only merges the catalog). Claude has no
  equivalent on-disk catalog to sync from — `~/.claude/settings.json` holds only the *selected*
  model, and the full available list is compiled into the CLI binary — so a Claude version needs a
  different source: parse the model strings out of the `claude` binary, ship/maintain a bundled list
  updated per release, or sync only the single selected/default model. Same guardrails as the Codex
  one: opt-in per backend, never overwrite hand-edited entries, never change the default silently.
- **Rich, selectable themes.** Offer several complete skins—such as basic, SaaS, and space
  exploration—that can change more than colors or a light visual treatment.

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
- **Chunked transcript indexing.** Replace whole-session in-memory rewrites without losing old
  search content or weakening the fallback search path.
- **Operational CLI.** Complete the specification for dashboard control, install/update, pidfile
  concurrency, and actionable startup diagnostics.

## Known things to improve

These describe incomplete or deliberately limited shipped behavior. Their owning specification is
the authority; move an item to ready changes only after its exact requirements and acceptance checks
are clear.

- **Chat history fidelity.** Make replayed streaming deltas match live deltas; prevent overlapping
  transcript reloads from winning out of order; show initial-load errors.
- **Archive and tracking usability.** Refine `matched_in` when search terms span metadata and
  transcript content; add UI pagination; refresh visible files/commands without stale-request
  overwrite; and let hook-only activity update recency.
- **Coordination liveness.** Scope nudge cooldowns to a generation, limit repeated nudges, republish
  unread counts after janitor expiry, notify only on the first budget breach, and remove duplicate
  permission notices.
- **Terminal capability honesty.** Either add an optional driver picker or stop advertising
  unreachable drivers; implement or retire the planned tab cap; and bound aggregate shutdown grace
  across multiple agents.
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
