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
- **Real dialogs instead of browser prompts.** Move to group — and its siblings Rename and Switch
  runtime — collect their arguments through `window.prompt`, so naming a group is an unstyled modal
  dialog with no existing-group list, no validation feedback, and no cancel/confirm affordance. FS-02
  §6 and FS-01.R8/R13 record this as a deliberate shipped limitation, so nothing half-built exists to
  plug in; reversing it needs dedicated form modals (group picker with autocomplete over current
  group labels, field-level validation) plus a feature-spec update.
- **Per-chat model picker.** The agent chat header shows `backend · model` as static text; the only
  way to change either is the dashboard context menu's Switch runtime, which asks for interface,
  backend, and model through three consecutive browser prompts. Offer a picker in the chat header
  itself, populated from the backend catalog, that drives the existing switch-runtime path (FS-01.R13
  — running agents only, history preserved by native resume or primer).
- **Claude backend model autosync.** The Codex half shipped (FS-09.R28: opt-in `autosync_models`
  reads `~/.codex/models_cache.json` on startup and add-only merges the catalog). Claude has no
  equivalent on-disk catalog to sync from — `~/.claude/settings.json` holds only the *selected*
  model, and the full available list is compiled into the CLI binary — so a Claude version needs a
  different source: parse the model strings out of the `claude` binary, ship/maintain a bundled list
  updated per release, or sync only the single selected/default model. Same guardrails as the Codex
  one: opt-in per backend, never overwrite hand-edited entries, never change the default silently.

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
