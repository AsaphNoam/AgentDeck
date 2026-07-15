# AgentDeck specifications — the source of truth

AgentDeck is developed **spec-driven**: two sets of specifications govern the product, and code,
tests, plans, and documentation trace back to them. When a spec and the code disagree, that is a
defect in one of them — a review names which, and the fix lands as a code change or specification update.
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
| FS-10 | [features/FS-10-macos-installation.md](features/FS-10-macos-installation.md) | Current | macOS release installation, guided provider setup, explicit updates and rollback |
| FS-11 | [features/FS-11-project-resources.md](features/FS-11-project-resources.md) | Partial | AgentDeck-owned, project-scoped shared resources outside repositories |

### Technical specs

| ID | Spec | Status | Covers |
|----|------|--------|--------|
| TS-01 | [tech/TS-01-architecture.md](tech/TS-01-architecture.md) | Partial | Process model, package boundaries, runtime abstraction, source-of-truth rules |
| TS-02 | [tech/TS-02-data-persistence.md](tech/TS-02-data-persistence.md) | Partial | SQLite schema & migrations, config files, transcripts, FTS index |
| TS-03 | [tech/TS-03-http-api.md](tech/TS-03-http-api.md) | Partial | REST surface, error envelope, SSE contract, status codes |
| TS-04 | [tech/TS-04-integration-protocols.md](tech/TS-04-integration-protocols.md) | Partial | ACP, hooks, MCP messaging, PTY/WebSocket, external-CLI tolerance |
| TS-05 | [tech/TS-05-security.md](tech/TS-05-security.md) | Partial | Loopback boundary, tokens, file modes, permission model, release-install trust notes |
| TS-06 | [tech/TS-06-build-test.md](tech/TS-06-build-test.md) | Current | Build tags, embed pipeline, release runtime, install, test strategy & conventions |
| TS-07 | [tech/TS-07-federation.md](tech/TS-07-federation.md) | Partial | Native configuration authority, resolvers, consent, freshness, redaction, launch freezing |
| INV | [../features/INVARIANTS.md](../features/INVARIANTS.md) | Current | Bug-class constraint catalog (path kept stable for hooks/history) |

Related, non-spec: [`../ideas.md`](../ideas.md) (new ideas and known things to improve — not
specifications),
[`../architecture-decisions.md`](../architecture-decisions.md) (ADR-style rationale behind TS-01;
decisions D1–D5), [`../../architecture-flow.md`](../../architecture-flow.md) (orientation
diagrams — descriptive, not authoritative; TS-01 wins on conflict).

---

## Shipped capability groups

The FS index above is the catalog of completed product capability groups—not a roadmap. A
**Current** spec contains only shipped behavior (plus explicit deviations); a **Partial** spec
contains both shipped behavior and individually tagged `(planned)` requirements. Potential work that
has not reached an FS/TS update belongs only in `docs/ideas.md`.

| Capability group | Relevant feature specs | Delivery state |
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

[`../ideas.md`](../ideas.md) is the home for a new idea before it has a relevant specification. It
separates raw ideas, ideas being defined, and known things to improve. It is deliberately outside
`docs/specs/` so a future idea cannot be mistaken for a product requirement.

[`../ready-changes/`](../ready-changes/README.md) is the home for specified work that is approved to
start but has not started. Each change file links to its relevant FS/TS/INV requirements and
acceptance evidence. `HANDOFF.md` is not a waiting list: it tracks only a change already in progress.
Agents do not choose future work on their own.

---

## Lifecycle — how a change flows through the specs

1. **Capture, define, and prepare.** Record a new idea in `docs/ideas.md`. Defining it drafts the
   needed FS/TS updates. Once the updates and acceptance criteria are adequate and implementation is
   wanted, create a change file in `docs/ready-changes/`. An agent does not decide which future work
   matters most.
2. **Update the specification first.** Any change to user-visible behavior or to an architectural rule
   starts by editing the relevant spec: add/modify R/A items (tag `(planned)` until shipped).
   If no spec covers the area, create or extend one. Pure bug fixes that restore specified behavior
   need no update. If the right update needs a product call, ask the user rather than silently deciding it.
3. **Plan.** Derive an implementation plan from the updates: the checklist in `HANDOFF.md`'s active
   work detail for small changes, or `docs/plans/<change>.md` for large ones. Plans are
   sequencing, not truth — delete the plan file (git keeps it) when the work completes.
4. **Implement and verify.** Follow the workflow's required checks. Tests that pin an acceptance criterion cite it
   (`// FS-05.A2` / `# FS-05.A2`). Commits that implement or change spec'd behavior name the IDs
   in the subject or a `Spec:` trailer.
5. **Ship the spec with the code.** In the completed change, flip `(planned)` tags,
   update the spec's Traceability section, and keep status honest. Do not call work complete with a stale specification.
6. **Review both directions.** Reviews check the diff against the relevant spec (does the code do
   what the spec says?) *and* the spec against the diff (did behavior ship that no spec covers, or
   that contradicts one?). Either mismatch is a finding.
7. **Keep decisions in one place.** Shipped behavior is described immediately in the relevant
   spec, even when a reversal is awaiting human direction. `HANDOFF.md` carries only the pending
   question and links to that requirement. When resolved, update/promote/retire the spec item and remove
   the question from live state.

## Maintenance duties by role

- **design-feature:** owns steps 1–2 for a new feature and creates its ready-change file. It works
  with the human, writes planned FS/TS requirements, and does not change product code.
- **work-phase (build):** owns steps 3–5 for a ready change. Reads the relevant specs *before* the code.
- **review-phase:** performs step 6. May record missing or incorrect specification coverage; does not edit specs itself
  (spec edits are fix/build work).
- **fix-review:** when a validated fix changes behavior (not just restores it), updates the
  relevant spec in the same completed change.
- **usability-review:** exercises journeys against feature-spec acceptance criteria; a mismatch
  between observed behavior and an A-item is a finding tagged with that ID.

## Lint

`scripts/check-specs.sh` (also `make check-specs`, wired into `make test`) verifies required H1 and
status metadata, status/planned-item consistency, required sections, unique local R/A IDs, exact
index membership and status, relative spec links, exact FS/TS citations in specs/source/tests/plans,
and common merge/tool artifacts. The post-edit hook runs its file-local checks on any spec edit.
