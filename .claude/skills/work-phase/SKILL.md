---
name: work-phase
description: Autonomously implement an AgentDeck spec-driven change in short or interrupted sessions. Use for "/work-phase", "continue the build", "pick up the implementation", or a named change/requirement.
---

# Work a spec-driven change

Read [`HANDOFF.md`](../../../docs/features/HANDOFF.md), its named active work package (if any), the
[`spec constitution`](../../../docs/specs/README.md), the governing FS/TS/INV items, and
[`AGENT-WORKFLOW.md`](../../../docs/features/AGENT-WORKFLOW.md) §§1–7 and §11 completely. Then
follow the canonical loop; the workflow and specs win over this launcher.

`$ARGUMENTS`, if present, names a work package or the human’s requested change; otherwise use the
handoff's active package. Interpret the human’s normal language in context—there are no magic
keywords. If no package is active, do not choose from the product backlog; capture/clarify a newly
supplied idea or prepare a package only when the human’s intent is clearly to have it built. Continue
until the change is done, a canonical STOP condition occurs, or quota requires a safe exit.
Close every session with the workflow's HANDOFF update, checkpoint/commit rules, and exact human brief.
