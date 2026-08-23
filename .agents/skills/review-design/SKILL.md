---
name: review-design
description: Explicit invocation only. Run only when the user sends `/review-design`; do not trigger from a natural-language request.
---

# Review a waiting design

Read [`HANDOFF.md`](../../../docs/features/HANDOFF.md), the
[`spec overview`](../../../docs/specs/README.md), the ready change under review, every planned
FS/TS item it cites plus the shipped requirements around them,
[`INVARIANTS.md`](../../../docs/features/INVARIANTS.md) in full, and workflow §§3, §5–7, and §13
completely, then follow the design-review process: over-engineering, extension over new mechanism,
and unverified assumptions are the primary lenses.

`$ARGUMENTS` names the waiting change in `docs/ready-changes/`; with exactly one waiting change,
`/review-design` alone selects it; with several, ask. Do not change product code, specifications,
or the change file.

Every finding names who experiences what, and when; one you cannot state that way is a consistency
note, not a finding. Verify a failure against the tree before asserting it — a scenario the shipped
code already prevents is worse than silence. A round with no user-visible or unbuildable consequence
closes the design; do not ask for another pass. Record findings and consistency notes in their
separate sections, make the required state commit, and close with the exact stored human update.
