# AgentDeck product backlog

This is the durable intake and prioritization queue for **unshipped work**. It is not an FS/TS
spec, it creates no commitment, and an agent must not implement an entry merely because it appears
here. Shipped behavior lives only in [`specs/README.md`](specs/README.md) and its FS/TS/INV index.

## How an idea moves

| Lane | Meaning | Who may move it |
|---|---|---|
| **Inbox** | A human idea captured faithfully; no implied priority, design, or commitment. | Any agent receiving the idea. |
| **Discovery** | An agent is drafting a spec/design proposal; no product code yet. | The user’s clear intent or a clarification promotes it. |
| **Implementation queue** | Relevant FS/TS requirements and acceptance criteria exist; the feature is ready but has not started. | The agent creates a work package in [`implementation-queue/`](implementation-queue/README.md). |
| **Known gaps** | Shipped behavior is incomplete or a documented deviation. The owning FS/TS remains authoritative. | The user’s clear intent selects it for discovery or delivery. |

Use `I<n>` for newly captured inbox ideas, `B<n>` for triaged feature candidates, and `G<n>` for
known gaps. Each non-inbox entry names its likely relevant spec(s). Do not create a duplicate
feature spec merely to hold an idea.

An idea that is merely being considered stays in **Inbox**. If the user clearly wants a
proposal, it enters **Discovery**. If the user clearly wants it built, the agent drafts or confirms
the needed FS/TS updates, then creates a ready work package. Ask a concise question only when the intended
level of commitment would materially change the result.

An implementation agent works from an active package in the implementation queue. With no active
package, it may capture/clarify a newly supplied idea but must not self-prioritize a backlog item.

## Inbox

No untriaged ideas.

New-item shape:

```md
- **I<n> — <short idea>.** Source: <human/task/date>. Desired outcome: <one sentence>.
  Constraints or examples: <optional>. No relevant spec yet.
```

## Discovery

No discovery work is active. A discovery outcome is either a clarified Inbox/candidate item, a
retired idea, or a ready work package in [`implementation-queue/`](implementation-queue/README.md).

## Candidate features

- **B1 — Phase 7 real-provider acceptance.** Clear the credentialed OpenCode/OpenHands and
  Claude/Codex federation gates in FS-08, FS-09, TS-04, and TS-07; reconcile any observed provider
  incompatibility before making release claims.
- **B2 — AgentDeck product knowledge MCP.** Specify a binary-versioned, non-secret
  `agentdeck_docs` topic service for fresh AgentDecker roles. It must not overwrite existing user role
  files and must define ownership, versioning, registration, and acceptance before implementation.
- **B3 — Detached configuration import.** Define verified copyable fields/assets and provider
  injection paths before implementing TS-07.R11.
- **B4 — Activity map.** Explore a repository/session activity view using server APIs only. Never
  statically serve `$AGENTDECK_HOME`; define privacy, scale, and normal-user value first.
- **B5 — API authentication / multi-user boundary.** Revisit TS-05.R3 only with an explicit threat
  model and UI/CLI handshake design.
- **B6 — Chunked transcript indexing.** Replace whole-session in-memory rewrites without dropping
  old search content or weakening the untagged fallback.
- **B7 — User-prompt persistence.** Specify durable/searchable user messages and hydration behavior
  in FS-03/TS-02 before changing the current assistant/tool-only normalized transcript.
- **B8 — Operational CLI.** Complete a feature spec for `dashboard start/stop/open`, install/update,
  pidfile concurrency, and actionable startup diagnostics.

## Known gaps and implementation deviations

These have an owning spec but are not yet satisfied or fully specified. Moving one to delivery means
adding the exact R/A items (when absent), then creating a Ready work package in the implementation
queue. The handoff is updated only when that package becomes active.

- **G1 — Chat history fidelity (FS-03).** Persist user prompts; fold replayed streaming deltas like
  live deltas; generation-guard overlapping transcript refetches; surface initial load errors.
- **G2 — Archive/tracking usability (FS-05).** Define mixed metadata+transcript `matched_in`, add UI
  pagination, make visible Files/Commands refresh safely, and let hook-only activity advance recency.
- **G3 — Coordination liveness (FS-06).** Generation-scope nudge cooldown/in-flight state, cap/back
  off repeated nudges, republish expiry/delete unread changes, emit budget notification only on first
  breach, and remove duplicate permission notifications.
- **G4 — Terminal capability honesty (FS-07).** Add the optional driver picker or stop advertising
  unreachable drivers; implement or retire the tab-cap requirement; make shutdown grace parallel and
  context-aware; prevent nudges from injecting into partially typed input.
- **G5 — Federation UI/watch completion (FS-08/TS-07).** Expose custom root/profile, invalidate the
  effective view on source events, register prompt watches immediately after bind, and clear preview
  consent when project selection changes.
- **G6 — Backend launch diagnostics (FS-09.R27/TS-04.R10).** Use executable overrides consistently,
  bound ACP readiness, probe/fallback optional flags, and return provider-specific missing/old CLI
  guidance. Make credential probes platform/version-aware.
- **G7 — HTTP compatibility (TS-03.R3–R4).** Decide and specify a migration path from mixed legacy
  error envelopes before standardizing clients or handlers.
- **G8 — Frontend state ownership.** Add a technical-spec update for Zustand vs React Query ownership,
  SSE hydration/refetch generations, and the mutation-error behavior before broad frontend refactors.
- **G9 — Lifecycle/process hardening (FS-01/TS-04).** Corroborate PID identity, generation-scope
  crash teardown, serialize concurrent event dispatch, and specify/test detached-start pidfile races.
- **G10 — Local filesystem hardening (TS-02/TS-05).** Decide whether startup should recursively
  repair existing descendant modes and whether valid-name role/project symlink files must be
  rejected; specify same-user threat assumptions and add adversarial tests before implementation.
- **G11 — HTTP request-size limits (TS-03/TS-05).** Define per-route or shared request-body limits,
  the structured over-limit error, and streaming exceptions, then install the bound before decode.
- **G12 — Spec semantic coverage.** Audit Current specs against executable behavior, especially
  API payloads, security claims, and persistence shapes; promote Partial specs only after their
  credentialed/manual gates and planned items are closed.

## Provenance

This file was created by the SDD migration in commit `735a4bf` (2026-07-13); it did **not** replace
a pre-existing backlog file. `B1`–`B8` were synthesized from archived phase/future-work material
and explicit unshipped feature ideas. `G1`–`G12` were synthesized from relevant-spec deviations,
manual acceptance gates, and the migration's semantic audits. Treat every item as a lead to be
revalidated, not as an inherited product commitment.

## Maintenance rule

Candidates may be clarified here, but normative behavior, data shapes, security choices, or
implementation constraints belong only in feature and technical specs. Retire stale entries with a
dated one-line reason; do not turn this file into a shadow roadmap or phase plan.
