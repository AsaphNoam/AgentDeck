---
name: fix-review
description: Validate the open AgentDeck review findings, dismiss false positives, and fix real findings to GREEN checkpoints with regression tests. Use for "/fix-review", "fix the review findings", "address the review", or "validate and fix the findings".
---

# Fix review findings

Read [`docs/features/AGENT-WORKFLOW.md`](../../../docs/features/AGENT-WORKFLOW.md) §§2–7 and §9 plus
[`docs/features/HANDOFF.md`](../../../docs/features/HANDOFF.md) completely, then follow the canonical
two-gate fix action. The workflow wins over this launcher.

`$ARGUMENTS` may scope the run to a severity or finding keyword; otherwise handle every open finding,
BLOCKING first. Close with the canonical HANDOFF update, checkpoint/commit, and exact human brief.
