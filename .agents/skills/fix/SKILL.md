---
name: fix
description: Explicit invocation only. Run only when the user sends `/fix`; do not trigger from a natural-language request.
---

# Fix findings

At startup, inspect `git status` and the existing diff before any repository edit. Use the injected
handoff header when present; otherwise read only **Current position** and **Active
change** in [`HANDOFF.md`](../../../docs/features/HANDOFF.md). Open its **Review findings** section
for the review being fixed, then read the FS/TS items named by those findings,
[`INVARIANTS.md`](../../../docs/features/INVARIANTS.md)'s trigger index and the classes the
findings touch, and workflow
§§2–6, §8, and §10 completely, then follow the fix process.

`$ARGUMENTS` may scope the run to a finding priority or keyword; otherwise choose findings from any
one available review and say which one you took, then handle its **Must fix** items first as the
continuation of its originating change unit (workflow §1.4 and §8). Chronology and other role queues
do not constrain selection. Stop when that unit is closed rather than continuing into another unit.
The fix commit
closes that unit; it does not create a new default review unit. Name what is still open when you
close. Name the finding's invariant class in the changelog line. Update the
relevant specification when a fix changes behavior or fills missing coverage. Close with the handoff
update, commit, and concise human update.
