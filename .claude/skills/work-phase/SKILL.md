---
name: work-phase
description: Autonomously implement an AgentDeck spec-driven change in short or interrupted sessions. Use for "/work-phase", "continue the build", "pick up the implementation", or a named change/requirement.
---

# Work a spec-driven change

Read [`HANDOFF.md`](../../../docs/features/HANDOFF.md), its named active work package (if any), the
[`spec overview`](../../../docs/specs/README.md), the relevant FS/TS/INV items, and
[`AGENT-WORKFLOW.md`](../../../docs/features/AGENT-WORKFLOW.md) §§1–6 and §10 completely. Then
follow the shared workflow; the workflow and specs take precedence over this launcher.

`$ARGUMENTS`, if present, names a work package or the human’s requested change; otherwise use the
handoff's active package. If no package is active, do not choose from the product backlog. Continue
until the change is done, a real blocker occurs, or quota requires a safe exit. Close every session
with the handoff update, commit rules, and human update.
