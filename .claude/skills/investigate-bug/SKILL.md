---
name: investigate-bug
description: Explicit invocation only. Run only when the user sends `/investigate-bug`; do not trigger from a natural-language request.
---

# Investigate a reported bug

Read [`HANDOFF.md`](../../../docs/features/HANDOFF.md), the
[`spec overview`](../../../docs/specs/README.md), the FS/TS items governing the reported behavior,
[`INVARIANTS.md`](../../../docs/features/INVARIANTS.md) in full, and workflow §§3, §5–7, and §12
completely, then follow the investigation process.

`$ARGUMENTS` carries the bug report: symptom text, a log excerpt, or a path to a log file; if empty,
ask for the report. Do not change product code or specifications; the only allowed tree change is a
reproduction test committed skipped. Record every finding with its confidence level, make the
required state commit, and close with the exact stored human update.
