---
name: usability-review
description: Explicit invocation only. Run only when the user sends `/usability-review`; do not trigger from a natural-language request.
---

# Review usability

Use the injected handoff header when present; otherwise read only **Current position** and **Active
change** in [`HANDOFF.md`](../../../docs/features/HANDOFF.md). Open **Acceptance gates** when the
scope points there, then read relevant FS acceptance items,
[`USABILITY-REVIEW.md`](../../../docs/features/USABILITY-REVIEW.md), and workflow §§3, §5–6, and
§9 completely, then follow the usability-review process.

`$ARGUMENTS` may name the journey or scope. Do not change product code. Record every finding or
unable-to-run journey, make the required state commit, and close with the concise human update.
