---
name: investigate-bug
description: Explicit invocation only. Run only when the user sends `/investigate-bug`; do not trigger from a natural-language request.
---

# Investigate a reported bug

Use the injected handoff header when present; otherwise read only **Current position** and **Active
change** in [`HANDOFF.md`](../../../docs/features/HANDOFF.md). Then read the FS/TS items governing
the reported behavior,
[`INVARIANTS.md`](../../../docs/features/INVARIANTS.md)'s trigger index and the classes the
report touches, and workflow §§3, §5–7, and §12
completely, then follow the investigation process.

`$ARGUMENTS` carries the bug report: symptom text, a log excerpt, or a path to a log file; if empty,
ask for the report. Do not change product code or specifications; the only allowed tree change is a
reproduction test committed skipped. Record every finding with its confidence level, make the
required state commit, and close with the concise human update.
