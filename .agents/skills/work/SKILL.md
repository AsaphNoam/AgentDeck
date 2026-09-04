---
name: work
description: Explicit invocation only. Run only when the user sends `/work`; do not trigger from a natural-language request.
---

# Work a change

At startup, inspect `git status` and the existing diff before any repository edit. Use the injected
handoff header when present; otherwise read only **Current position** and **Active
change** in [`HANDOFF.md`](../../../docs/features/HANDOFF.md). Then read the named change in progress
(if any), the FS/TS items named by the change or request,
[`INVARIANTS.md`](../../../docs/features/INVARIANTS.md)'s trigger index — always, then the classes
the change actually touches, in full — and
[`AGENT-WORKFLOW.md`](../../../docs/features/AGENT-WORKFLOW.md) §§1–6 and §10 completely. Then
follow the shared workflow; the workflow and specs take precedence over this launcher.

A new interface, runtime, or driver completes the invariant §6 contract checklist line by line;
silence on any line is a bug, not a default.

`$ARGUMENTS`, if present, names a change or the human’s requested change. Otherwise choose any
available resumable unit in the handoff or `Waiting to start` change in `docs/ready-changes/`, and
say which one you took. Several available changes are not a reason to ask, and pending design,
review, or fix units do not block work. If none is available, say so.

An explicitly named ready change is likewise authorized by `/work <name>`; verify that it exists,
move it into the handoff, then proceed. Continue until the change is done, a real blocker occurs, or
quota requires a safe exit. When substantive work finishes, add that completed change to the
available review units without replacing or blocking units already there; administrative commits
are not additional units. Close every session with the handoff update, commit rules, and human
update.
