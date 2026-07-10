---
name: review-phase
description: Review the other agent's latest AgentDeck GREEN-checkpoint work for spec adherence, dead code, bad practices, flagrant normal-use bugs, and pending peer decisions. Use when the user says "/review-phase", "review the last commit", "review the other agent's work", or similar.
---

# Review the last agent's work

Read [`docs/features/AGENT-WORKFLOW.md`](../../../docs/features/AGENT-WORKFLOW.md) §§2–3, §5, §7,
and §8 plus [`docs/features/HANDOFF.md`](../../../docs/features/HANDOFF.md) completely, then follow
the canonical review action. The workflow wins over this launcher.

The human may name a commit/range in `$ARGUMENTS`; otherwise use the handoff's review markers.
Do not change product code. Persist every finding and decision disposition, make the required
workflow-state commit, and close with the exact stored human brief.
