# AGENTS.md — AgentDeck

Guidance for coding agents working in this repository.

## Read order

Before feature design, implementation, review, fix-review, usability-review, UX, or release work,
load only the state and rules that role needs:

1. [`docs/features/HANDOFF.md`](docs/features/HANDOFF.md) — its **Current position** and **Active
   change**; other sections only when those point at them (AGENT-WORKFLOW §1.1).
2. The change file named by the handoff, if a change is in progress.
3. The feature (`FS-*`) and technical (`TS-*` / `INV`) requirements named by the change file, handoff,
   or request. Open [`docs/specs/README.md`](docs/specs/README.md) only when the role needs its
   constitution, index, or authoring rules.
4. Only the [`AGENT-WORKFLOW.md`](docs/features/AGENT-WORKFLOW.md) sections named by the active role
   or launcher; do not read unrelated role sections.

The workflow explains the process; feature and technical specs define the product and architecture.
`MAP.md`, ADRs, archive files, plans, handoffs, briefs, and review reports provide navigation,
rationale, history, or sequencing but do not override an FS/TS requirement.

## Roles

- **Design feature:** workflow §11. Work with the human to turn one idea into planned feature and
  technical specifications plus a ready change; do not write product code.
- **Implement:** workflow §§1–6 and §10. Update the relevant specification before a behavior or
  architecture change, then keep working until done or a real blocker.
- **Review:** §7. Check that code matches the specifications and that the specifications cover the
  shipped behavior; record every real finding.
- **Fix review:** §8 and §10. Validate before editing; update the relevant specification when a fix
  changes behavior or fills missing coverage.
- **Usability review:** §9 plus `USABILITY-REVIEW.md`. Exercise FS acceptance criteria without
  changing product code or specs.
- **Investigate bug:** §12. Turn a field bug report into a diagnosed, confidence-labelled finding
  for fix; change no product code or specs beyond a skipped reproduction test.
- **Review design:** §13. Review a waiting ready change before implementation — over-engineering,
  extension over new mechanism, and unverified assumptions are findings; change nothing.
- **Design UI:** §14. `/design` is explicit or automatically accompanies work with material visual
  judgment; it improves direction and rendered iteration without selecting or enlarging the work.
- **Shape and test UX:** §15. `/ux` is explicit or automatically accompanies feature design and
  implementation only when an established task changes or material consequence, state, recovery,
  long-running, or AI uncertainty needs judgment; it optimizes the experienced operator's task.
- **Release:** §16. `/release` tags a new version, refreshes the shipped `operating-agentdeck`
  package for what the release range changed, and publishes only with explicit authorization; it
  adds no feature and fixes no finding.

`docs/ideas.md` holds new ideas and known product improvements. `docs/ready-changes/` holds changes
that are specified and ready to start. `HANDOFF.md` records only the change already in progress.
Agents never choose future work for themselves.

Every role keeps the live handoff accurate and ends with a short, plain-language human update.
`BRIEFS.md` is retained history, not a per-session write target or a resumption source.

## Repository rules

- Read the [`INVARIANTS.md`](docs/features/INVARIANTS.md) trigger index, then the matching class in
  full before a hot-spot change. Reading all fifteen classes for a diff that touches one is waste.
- Shared verification is specified by TS-06 and workflow §2. Use focused checks while iterating
  and run the applicable closure matrix once after the final relevant edit; the server remains
  loopback-only.
- Never edit `internal/server/ui/dist/**`; generate it from `ui/src` with `make embed`.
- Edit with the native patch/edit tool. Do not use Python, Perl, or shell heredoc replacement
  scripts that repeat old and new source in the transcript.
- Preserve dirty-tree work unless the user explicitly authorizes discarding it.
- One change unit per run, from implementation through its review findings and fixes. Administrative
  review, fix-closure, release-record, and handoff commits never become default review units
  (AGENT-WORKFLOW §1.4).

Response Format
Answer first. No preamble, no restating my question, no "Great question."
No flattery openers ("You're absolutely right", "Good idea").
No transition or meta-commentary filler — sentences that announce what you're about to say or editorialize on the framing rather than deliver content. Banned examples: "Different situation, and worth being precise, because...", "worth noting that...", "it's important to understand...". Cut straight to the substance.

Velocity
Velocity is critical. Adhere strictly to the following principles to maintain momentum:
Keep it Simple: Implementations must be simple, easy to understand, and easy to maintain over clever or overly abstracted code.
Propose High-Value Refactors Separately: If you spot a fix, refactor, or improvement that is highly valuable, DO NOT implement it automatically. Instead, pause and ask the user by concisely explaining:
What the fix/refactor is.
The estimated size/complexity of the change.
Exactly why it is worth doing right now.

Delegation
Before starting any non-trivial or multi-step task, stop and explicitly assess whether focused subagents would complete all or part of it better or more efficiently. Make this decision before loading substantial task context or beginning implementation.
Use subagents when they provide a concrete advantage through parallelism, clean-context reasoning, context management, or token efficiency. In long threads, prefer dispatching a well-scoped subagent with only the relevant context when carrying the full conversation history would add noise or waste tokens. Keep work local when the task is trivial, tightly coupled, or delegation overhead would exceed the benefit.

When delegating:
Decompose the work into independent, bounded subtasks and state the decomposition in the working plan.
Run independent subtasks in parallel when possible.
Give each subagent the relevant context, constraints, expected deliverable, and verification criteria without forwarding unrelated conversation history.
Choose the least expensive model that can complete the subtask without sacrificing quality. Broad, uncertain, multi-file discovery that would retain substantial raw output should use `spawn_agent` with `model: "gpt-5.6-luna"` and `fork_turns: "none"`, then return a distilled answer. Keep narrow, bounded lookups local because a child has its own startup cost. Leave `reasoning_effort` unset unless the task genuinely needs a floor. Use Sol only for work that genuinely requires its deeper reasoning, ambiguity handling, or integration judgment.
Do not default to the orchestrator's model for delegated work.
Review, integrate, and verify subagent output; delegation does not transfer responsibility for the final result.
