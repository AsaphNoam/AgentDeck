# AgentDeck specifications — the source of truth

AgentDeck is developed **spec-driven**: two sets of specifications govern the product, and code,
tests, plans, and documentation trace back to them. When a spec and the code disagree, that is a
defect in one of them — a review names which, and the fix lands as a code change or a spec delta.
Nothing ships "because the code already does it."

- **Feature specs** (`features/FS-nn-*.md`) — product behavior: what the user and the API client
  observe. Requirements, states, edge cases, acceptance criteria.
- **Technical specs** (`tech/TS-nn-*.md`) — architecture: system boundaries, data models,
  protocols, internal contracts, implementation constraints.
- **[`../features/INVARIANTS.md`](../features/INVARIANTS.md)** — part of the technical spec set
  (cited as `INV §n`): the paid-for bug-class catalog. Its rules are binding constraints, exactly
  like a TS requirement.

Everything else is **not** spec: [`HANDOFF.md`](../features/HANDOFF.md) and
[`BRIEFS.md`](../features/BRIEFS.md) are live state, plans are disposable sequencing,
review-run reports are records, and [`../archive/`](../archive/) is history. The protocol that
operates on all of this is [`AGENT-WORKFLOW.md`](../features/AGENT-WORKFLOW.md).

---

## Spec index

### Feature specs

| ID | Spec | Status | Covers |
|----|------|--------|--------|
| FS-00 | [features/FS-00-product-overview.md](features/FS-00-product-overview.md) | Current | Product summary, goals/non-goals, core concepts, glossary |
| FS-01 | [features/FS-01-agent-lifecycle.md](features/FS-01-agent-lifecycle.md) | Current | Launch, stop, cancel, resume, clone, rename, switch runtime, crash handling, identity |
| FS-02 | [features/FS-02-dashboard.md](features/FS-02-dashboard.md) | Current | Card grid, live status, layout/density, task groups, notifications |
| FS-03 | [features/FS-03-chat.md](features/FS-03-chat.md) | Current | Streaming chat panel, tool calls/diffs, permission prompts, transcript view |
| FS-04 | [features/FS-04-configuration-onboarding.md](features/FS-04-configuration-onboarding.md) | Current | Roles/projects/backends CRUD, settings UI, onboarding wizard |
| FS-05 | [features/FS-05-archive-tracking.md](features/FS-05-archive-tracking.md) | Current | Session archive, full-text search, resume from archive, file/command tracking |
| FS-06 | [features/FS-06-coordination.md](features/FS-06-coordination.md) | Current | Agent-to-agent messaging, nudger, budgets, unread indicators |
| FS-07 | [features/FS-07-terminal.md](features/FS-07-terminal.md) | Partial | Terminal interface, drivers (xterm/tmux/iTerm2), terminal-agent boundaries |
| FS-08 | [features/FS-08-federation.md](features/FS-08-federation.md) | Partial | Claude/Codex configuration federation: sources, binding modes, effective view |
| FS-09 | [features/FS-09-backends.md](features/FS-09-backends.md) | Partial | Backend/model catalog, credential checks, per-backend capability matrix |

### Technical specs

| ID | Spec | Status | Covers |
|----|------|--------|--------|
| TS-01 | [tech/TS-01-architecture.md](tech/TS-01-architecture.md) | Partial | Process model, package boundaries, runtime abstraction, source-of-truth rules |
| TS-02 | [tech/TS-02-data-persistence.md](tech/TS-02-data-persistence.md) | Current | SQLite schema & migrations, config files, transcripts, FTS index |
| TS-03 | [tech/TS-03-http-api.md](tech/TS-03-http-api.md) | Current | REST surface, error envelope, SSE contract, status codes |
| TS-04 | [tech/TS-04-integration-protocols.md](tech/TS-04-integration-protocols.md) | Partial | ACP, hooks, MCP messaging, PTY/WebSocket, external-CLI tolerance |
| TS-05 | [tech/TS-05-security.md](tech/TS-05-security.md) | Current | Loopback boundary, tokens, file modes, permission model, threat notes |
| TS-06 | [tech/TS-06-build-test.md](tech/TS-06-build-test.md) | Current | Build tags, embed pipeline, install, test strategy & conventions |
| TS-07 | [tech/TS-07-federation.md](tech/TS-07-federation.md) | Partial | Native configuration authority, resolvers, consent, freshness, redaction, launch freezing |
| INV | [../features/INVARIANTS.md](../features/INVARIANTS.md) | Current | Bug-class constraint catalog (path kept stable for hooks/history) |

Related, non-spec: [`../product-backlog.md`](../product-backlog.md) (human ideas, candidates, and
known gaps — not specifications),
[`../architecture-decisions.md`](../architecture-decisions.md) (ADR-style rationale behind TS-01;
decisions D1–D5), [`../../architecture-flow.md`](../../architecture-flow.md) (orientation
diagrams — descriptive, not authoritative; TS-01 wins on conflict).

---

## Shipped capability groups

The FS index above is the catalog of completed product capability groups—not a roadmap. A
**Current** spec contains only shipped behavior (plus explicit deviations); a **Partial** spec
contains both shipped behavior and individually tagged `(planned)` requirements. Potential work that
has not reached an FS/TS delta belongs only in the product backlog.

| Capability group | Governing feature specs | Delivery state |
|---|---|---|
| Core agent operation | FS-00 product concepts; FS-01 lifecycle; FS-02 dashboard; FS-03 chat | Shipped |
| Configuration and providers | FS-04 configuration/onboarding; FS-09 backends | Shipped core; FS-09 expansion remains Partial |
| Durable supervision | FS-05 archive/tracking; FS-06 coordination | Shipped |
| Extension boundaries | FS-07 terminal; FS-08 federation | Shipped core with explicitly tagged planned work; Partial |

---

## Requirement IDs and anchors

Inside a spec, every independently citable normative statement gets a stable ID. State/data-flow
sections may explain consequences of those IDs, but must not introduce a new obligation without one:

- **`R<n>`** — a requirement ("the system does X"). Cite as `FS-05.R3` / `TS-03.R10`.
- **`A<n>`** — an acceptance criterion (an observable check that proves a slice works). Cite as
  `FS-05.A2`. Each A-item names how it is verified: a test, a usability journey (`J<n>` from
  [`USABILITY-REVIEW.md`](../features/USABILITY-REVIEW.md)), or a manual gate.
- IDs are **append-only**. Never renumber. A dropped item stays in place as
  `R7 — retired <date>: <one line why>` so old citations still resolve.
- Do not silently change an ID's meaning. Clarifications may edit in place; a semantic replacement
  retires/supersedes the old item and appends a new one.
- Invariant classes are cited as `INV §4` and are binding on every spec.

## Spec statuses

Each spec declares one status in its header:

- **Current** — describes shipped behavior; deviations are explicitly listed in its
  "Deviations & open decisions" section.
- **Partial** — some sections describe planned-but-unshipped behavior; those items are tagged
  `(planned)` inline.
- **Draft** — not yet authoritative; being written or overhauled.

A `(planned)` tag or an entry in "Deviations & open decisions" is the only legitimate way for a
spec to differ from shipped code. An unexplained mismatch is a defect.

---

## Templates

### Feature spec

```md
# FS-nn — <Title>

**Status:** Current | Partial | Draft
**Code:** <primary packages/dirs> · **Journeys:** <J-ids or —>
**Absorbed:** <archived docs this spec replaced>

## 1. Purpose
## 2. Behavior            <numbered R-items; user/API-observable only>
## 3. States & transitions
## 4. Edge cases & errors <numbered R-items continue>
## 5. Acceptance criteria <numbered A-items; each names its verification>
## 6. Deviations & open decisions
## 7. Traceability        <key regression tests, load-bearing code anchors>
```

### Technical spec

```md
# TS-nn — <Title>

**Status:** Current | Partial | Draft
**Code:** <primary packages/dirs>
**Absorbed:** <archived docs this spec replaced>

## 1. Scope
## 2. Design & constraints   <numbered R-items; binding decisions with one-line rationale>
## 3. Interfaces & data shapes
## 4. Invariants             <which INV classes govern this area, plus local invariants as R-items>
## 5. Deviations & open decisions
## 6. Traceability
```

Keep specs lean: normative statements, shapes, and constraints — not tutorials, not history
(history lives in git and `docs/archive/`), not task lists (plans are separate and disposable).

---

## Idea intake & promotion

[`../product-backlog.md`](../product-backlog.md) is the only home for an unshipped human idea before
it has a governing spec. It separates **Inbox**, **Discovery**, candidate features, and known gaps.
It is deliberately outside `docs/specs/` so a queue item cannot be mistaken for an authoritative
contract.

[`../implementation-queue/`](../implementation-queue/README.md) is the dedicated home for
specified work that is ready to implement but has not started. Each package links to its governing
FS/TS/INV IDs and acceptance evidence. `HANDOFF.md` is not a readiness queue: it tracks only an
already-active package and its resumable checkpoint state. Agents interpret normal human intent in
context and clarify only material ambiguity; they never rely on magic words or self-select a backlog
candidate.

---

## Lifecycle — how a change flows through the specs

1. **Capture, discover, and queue.** A new human idea is recorded in the product backlog’s Inbox.
   When the human’s intent is to explore it, discovery drafts the governing FS/TS delta. Once the
   delta and acceptance criteria are adequate for a desired implementation, create a Ready package
   in the implementation queue. An agent does not infer priority from the backlog.
2. **Spec delta first.** Any change to user-visible behavior or to an architectural contract
   starts by editing the governing spec: add/modify R/A items (tag `(planned)` until shipped).
   If no spec governs the area, the delta includes creating or extending one. Pure bug fixes that
   *restore* specified behavior need no delta. If the right delta needs a product call, that is a
   STOP/HUMAN decision per the workflow — do not silently spec around it.
3. **Plan.** Derive an implementation plan from the delta: the checklist in `HANDOFF.md`'s active
   work detail for small changes, or `docs/plans/<change>.md` for large ones. Plans are
   sequencing, not truth — delete the plan file (git keeps it) when the work completes.
4. **Build to GREEN.** Implement per the workflow. Tests that pin an acceptance criterion cite it
   (`// FS-05.A2` / `# FS-05.A2`). Commits that implement or change spec'd behavior name the IDs
   in the subject or a `Spec:` trailer.
5. **Ship the spec with the code.** In the checkpoint commit, flip the delta's `(planned)` tags,
   update the spec's Traceability section, and keep status honest. A GREEN checkpoint with a stale
   governing spec is not green.
6. **Review both directions.** Reviews check the diff against the governing spec (does the code do
   what the spec says?) *and* the spec against the diff (did behavior ship that no spec covers, or
   that contradicts one?). Either mismatch is a finding.
7. **Keep decisions in one state each.** Shipped behavior is described immediately in the governing
   spec, even when a reversal is awaiting human direction. `HANDOFF.md` carries only the pending
   question and links to that contract. When resolved, update/promote/retire the spec item and remove
   the question from live state.

## Maintenance duties by role

- **work-phase (build):** owns steps 1–4 above. Reads the governing specs *before* the code.
- **review-phase:** enforces step 5. May record spec-gap findings; does not edit specs itself
  (spec edits are fix/build work).
- **fix-review:** when a validated fix changes behavior (not just restores it), updates the
  governing spec in the same checkpoint commit.
- **usability-review:** exercises journeys against feature-spec acceptance criteria; a mismatch
  between observed behavior and an A-item is a finding tagged with that ID.

## Lint

`scripts/check-specs.sh` (also `make check-specs`, wired into `make test`) verifies required H1 and
status metadata, status/planned-item consistency, required sections, unique local R/A IDs, exact
index membership and status, relative spec links, exact FS/TS citations in specs/source/tests/plans,
and common merge/tool artifacts. The post-edit hook runs its file-local checks on any spec edit.
