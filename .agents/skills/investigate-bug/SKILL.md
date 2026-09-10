---
name: investigate-bug
description: Explicit invocation only. Run only when the user sends `/investigate-bug`; do not trigger from a natural-language request.
---

# Investigate a reported bug

At startup, inspect `git status` and the existing diff before any repository edit. Use the injected
handoff header when present; otherwise read only **Current position** and **Active
change** in [`HANDOFF.md`](../../../docs/features/HANDOFF.md). Then read the FS/TS items governing
the reported behavior,
[`INVARIANTS.md`](../../../docs/features/INVARIANTS.md)'s trigger index and the classes the
report touches, and workflow §§3, §5–7, and §12
completely, then follow the investigation process.

`$ARGUMENTS` carries the bug report: symptom text, a log excerpt, or a path to a log file; if empty,
ask for the report. Do not change product code or specifications; the only allowed tree change is a
reproduction test committed skipped. Record every fixable finding with its confidence level and
exactly one §7 **Fix model** recommendation: trivial/easy uses Claude Sonnet or Codex Luna; medium
uses Codex Terra or Claude Opus; difficult uses Codex Sol. Classify complexity independently from
severity and repeat the recommendation in the human update. Make the required state commit and close
with the concise human update.
