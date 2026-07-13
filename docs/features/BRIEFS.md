# AgentDeck — Session briefs

Newest first. Each entry is the exact final response from an implementation, review, fix-review, or
usability-review session. Agents resume from [`HANDOFF.md`](HANDOFF.md), not this history. Earlier
entries are preserved in [`../archive/state/BRIEFS-pre-sdd.md`](../archive/state/BRIEFS-pre-sdd.md).

### 2026-07-14 — implementation: dedicated ready-work queue

Ready-but-unstarted features now live in the dedicated
[`../implementation-queue/`](../implementation-queue/README.md), one `W-<number>-<slug>.md` work
package per feature. Each package links to its governing FS/TS/INV requirements and acceptance
evidence, and has a simple Ready → Active → Shipped/Paused/Retired lifecycle. `HANDOFF.md` now holds
only the checkpoint state of a package that has actually started; it is no longer a waiting list.

The workflow no longer requires special wording such as “consider,” “design,” or “build.” Agents
interpret the user’s normal language and conversation context, asking only when the desired level of
commitment is materially unclear. An exploratory idea stays in the product backlog; a requested
proposal enters discovery; a requested change becomes a Ready package once its FS/TS delta and
acceptance criteria are adequate. Work-phase reads the active package named by the handoff and never
self-prioritizes backlog items.

Verified: `make check-specs`, shell syntax, all twinned skills, and `git diff --check`.

::git-commit{cwd="/Users/mcnoam/Projects/AgentDeck"}

### 2026-07-13 — implementation: explicit idea intake and work selection

The former `docs/specs/backlog.md` was a **new** SDD-migration file, not a migrated historical
backlog. It has moved to [`../product-backlog.md`](../product-backlog.md), outside the authoritative
spec tree. Its provenance now says exactly what happened: B1–B8 were synthesized from archived
phase/future-work material and unshipped ideas; G1–G12 came from current-spec deviations, manual
gates, and migration audits. They are leads to revalidate, not inherited commitments.

The product backlog now separates **Inbox** (faithfully captured ideas), **Discovery**
(human-authorized spec/design work), **Ready to build** (specified and human-authorized work),
candidate features, and known gaps. FS/TS remain the grouped catalog of shipped capabilities:
Current specs describe shipped behavior, while Partial specs mark only selected, unshipped
requirements as `(planned)`.

The workflow, handoff, AGENTS guidance, repository map, README, and twinned work-phase skills now
enforce the selection boundary. “Consider” captures an Inbox item; “design” activates Discovery;
“build” activates Implementation after an adequate FS/TS delta. A work-phase agent executes only an
active `Implementation` item in `HANDOFF.md`; it cannot self-prioritize a candidate, gap, or
planned requirement. The handoff template requires source ID, stage, governing IDs, and a testable
Done-when line.

Verified: `make check-specs`, shell syntax, twin work-phase skill parity, and `git diff --check`.

::git-commit{cwd="/Users/mcnoam/Projects/AgentDeck"}

### 2026-07-13 — implementation: spec-driven development foundation

AgentDeck now has two authoritative specification sets: feature specs FS-00–FS-09 for observable
behavior and acceptance criteria, and technical specs TS-01–TS-07 plus INV for architecture,
protocols, persistence, security, delivery, and implementation constraints. Each spec has stable
R/A identifiers, an honest Current/Partial/Draft status, deviations, acceptance evidence, and code/
test traceability. The lifecycle is spec delta → disposable plan → implementation → GREEN → spec and
handoff update → bidirectional review; shipped items lose `(planned)`, retired IDs are never reused.

The repository instructions, Claude guidance, MAP/README, canonical agent workflow, usability
protocol, architecture orientation, and twinned work/review/fix/usability skills now route agents to
the governing FS/TS/INV items first. Traceability is enforced by exact citations in tests, plans,
specs, and commits; `make check-specs`, the Claude post-edit hook, `make test`, and clean-clone CI
check spec structure/index/status/links/citations plus both Go variants, vet, and UI tests/build.

The master PRD, phase PRDs/tech specs, old handoff/brief history, stale HTML guides, and completed
usability evidence moved under `docs/archive/`. An archive manifest maps every phase slice to its
current authority. Useful rationale remains in non-authoritative ADR/orientation docs; obsolete live
phase instructions were removed rather than maintained in parallel.

Current gaps are explicit: FS-07–FS-09 and TS-01/TS-04/TS-07 remain Partial; real-provider and
federation compatibility still need credentialed gates; prompt-history fidelity, frontend state
ownership, operational CLI behavior, local filesystem hardening, and uniform HTTP request bounds
need further spec work. The next step is a semantic audit of the highest-risk Partial area, starting
with real Claude/Codex federation/MCP/terminal acceptance, then promoting only the requirements the
evidence proves. The maintained queue is `docs/specs/backlog.md`.

**Needs attention:** HUMAN — Local API authentication; Child-process environment; Terminal and
messaging support boundary; Detached federation import; API/model compatibility. These are recorded
shipped boundaries or explicit planned work, not blockers.

**What this teaches:** SDD remains practical across short agent sessions when authority is small and
stable (R/A IDs), while sequencing and memory stay disposable (plans/handoff). Mechanical lint can
prove references and lifecycle hygiene; only bidirectional review can prove that authoritative prose
still matches executable behavior.

::git-commit{cwd="/Users/mcnoam/Projects/AgentDeck"}
