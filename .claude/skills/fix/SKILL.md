---
name: fix
description: Explicit invocation only. Run only when the user sends `/fix`; do not trigger from a natural-language request.
---

# Fix findings

Read [`HANDOFF.md`](../../../docs/features/HANDOFF.md), the
[`spec overview`](../../../docs/specs/README.md), the FS/TS items named by the findings,
[`INVARIANTS.md`](../../../docs/features/INVARIANTS.md) in full, and workflow
§§2–6, §8, and §10 completely, then follow the fix process.

`$ARGUMENTS` may scope the run to a finding priority or keyword; otherwise handle every open finding,
starting with **Must fix**. Name the finding's invariant class in the changelog line. Update the
relevant specification when a fix changes behavior or fills missing coverage. Close with the handoff
update, commit, and exact human update.
