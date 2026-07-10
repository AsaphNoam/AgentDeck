---
name: work-phase
description: Autonomously implement the next AgentDeck phase/subphase in spaced, quota-limited sessions. Use when the user says "/work-phase", "work the next phase", "continue the build", "pick up the implementation", or hands the agent a phase to drive.
---

# Work a phase

Read [`docs/features/AGENT-WORKFLOW.md`](../../../docs/features/AGENT-WORKFLOW.md) §§1–7 and
[`docs/features/HANDOFF.md`](../../../docs/features/HANDOFF.md) completely, then follow the canonical
loop. The workflow wins over this launcher.

`$ARGUMENTS`, if present, names the phase/subphase to target; otherwise use the handoff's active work.
Continue until the phase is done, a canonical STOP condition occurs, or quota requires a safe exit.
Close every session with the workflow's HANDOFF update, checkpoint/commit rules, and exact human brief.
