---
name: review
description: Explicit invocation only. Run only when the user sends `/review`; do not trigger from a natural-language request.
---

# Review the unreviewed work

Use the injected handoff header when present; otherwise read only **Current position** and **Active
change** in [`HANDOFF.md`](../../../docs/features/HANDOFF.md). Then read the FS/TS items named by the
handoff or request,
[`INVARIANTS.md`](../../../docs/features/INVARIANTS.md)'s trigger index — always, regardless of what
the handoff names — and workflow §§3–7 completely. Check whether code matches the specifications and
whether the specifications cover the code across the handoff's unreviewed range.

Then sweep the diff against every invariant class by that index and tag each finding with its class
number; open a class body only where the trigger matches. A class with no applicable surface in the
diff is a result to state, not a step to skip.

The human may name a commit/range in `$ARGUMENTS`; otherwise start at the handoff's review marker
and review one unit — the next change's implementation range, or a single commit where there is no
change boundary (workflow §1.4 and §7). Do not sweep the whole unreviewed backlog in one run; name
the range left unreviewed when you close.
Do not change product code or specs. Record every finding and local-choice outcome, make the required
state commit, and close with the concise human update.
