---
name: work
description: Explicit invocation only. Run only when the user sends `/work`; do not trigger from a natural-language request.
---

# Work a change

Read [`HANDOFF.md`](../../../docs/features/HANDOFF.md), its named change in progress (if any), the
[`spec overview`](../../../docs/specs/README.md), the relevant FS/TS/INV items, and
[`AGENT-WORKFLOW.md`](../../../docs/features/AGENT-WORKFLOW.md) §§1–6 and §10 completely. Then
follow the shared workflow; the workflow and specs take precedence over this launcher.

`$ARGUMENTS`, if present, names a change or the human’s requested change. Otherwise use the
handoff's active change. If there is no active change, inspect `docs/ready-changes/`:

- with exactly one `Waiting to start` change, the user's explicit `/work` authorizes starting it;
  move it into the handoff and proceed;
- with two or more, list their names and ask the user to choose; do not prioritize one yourself;
- with none, say that no implementable change is available.

An explicitly named ready change is likewise authorized by `/work <name>`; verify that it exists,
move it into the handoff, then proceed. Continue until the change is done, a real blocker occurs, or
quota requires a safe exit. Close every session with the handoff update, commit rules, and human
update.
