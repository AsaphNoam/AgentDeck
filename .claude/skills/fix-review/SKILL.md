---
name: fix-review
description: Validate the open AgentDeck review findings, dismiss false positives, and fix real findings to GREEN checkpoints with regression tests. Use for "/fix-review", "fix the review findings", "address the review", or "validate and fix the findings".
---

# Fix review findings

Read [`HANDOFF.md`](../../../docs/features/HANDOFF.md), the
[`spec constitution`](../../../docs/specs/README.md), governing FS/TS/INV items, and workflow
§§2–7, §9, and §11 completely, then follow the two-gate fix action.

`$ARGUMENTS` may scope the run to a severity or finding keyword; otherwise handle every open finding,
BLOCKING first. Spec-gap or behavior-changing fixes update the governing spec in the same GREEN
checkpoint. Close with the canonical HANDOFF update, checkpoint/commit, and exact human brief.
