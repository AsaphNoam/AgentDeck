---
name: review-phase
description: Review the other agent's unreviewed AgentDeck work bidirectionally against governing specs, plus normal-use bugs and pending peer decisions.
---

# Review the unreviewed work

Read [`HANDOFF.md`](../../../docs/features/HANDOFF.md), the
[`spec constitution`](../../../docs/specs/README.md), governing FS/TS/INV items, and workflow
§§2–3, §5, §7–8 completely. Review code→spec and spec→code across the handoff's unreviewed range.

The human may name a commit/range in `$ARGUMENTS`; otherwise use the handoff's review markers.
Do not change product code or specs. Persist every finding and decision disposition, make the required
workflow-state commit, and close with the exact stored human brief.
