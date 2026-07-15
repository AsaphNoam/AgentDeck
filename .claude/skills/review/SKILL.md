---
name: review
description: Explicit invocation only. Run only when the user sends `/review`; do not trigger from a natural-language request.
---

# Review the unreviewed work

Read [`HANDOFF.md`](../../../docs/features/HANDOFF.md), the
[`spec overview`](../../../docs/specs/README.md), relevant FS/TS/INV items, and workflow
§§3–7 completely. Check whether code matches the specifications and whether the specifications cover
the code across the handoff's unreviewed range.

The human may name a commit/range in `$ARGUMENTS`; otherwise use the handoff's review markers.
Do not change product code or specs. Record every finding and local-choice outcome, make the required
state commit, and close with the exact stored human update.
