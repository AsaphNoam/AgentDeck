---
name: review
description: Explicit invocation only. Run only when the user sends `/review`; do not trigger from a natural-language request.
---

# Review the next substantive change

At startup, inspect `git status` and the existing diff before any repository edit. Use the injected
handoff header when present; otherwise read only **Current position** and **Active
change** in [`HANDOFF.md`](../../../docs/features/HANDOFF.md). Then read the FS/TS items named by the
handoff or request,
[`INVARIANTS.md`](../../../docs/features/INVARIANTS.md)'s trigger index — always, regardless of what
the handoff names — and workflow §§3–7 completely. Check whether code matches the specifications and
whether the specifications cover the code across the handoff's eligible change unit.

Then sweep the diff against every invariant class by that index and tag each finding with its class
number; open a class body only where the trigger matches. A class with no applicable surface in the
diff is a result to state, not a step to skip.

The human may name a commit/range in `$ARGUMENTS`; otherwise review only the earliest eligible unit
named by the handoff (workflow §1.4 and §7). Administrative commits never become default review
units: skip review reports, finding-fix commits, release records, and handoff, archive, or queue-only
commits when advancing review state. Do not review a later unit out of order. A no-finding review
closes its unit without creating another review obligation; if no eligible unit is pending, report
that and do not make an empty commit.
Do not change product code or specs. Record every finding and local-choice outcome, make the required
state commit, and close with the concise human update. Findings keep that same unit open; they do not
create a second review unit.
