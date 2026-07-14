---
name: work-phase
description: Explicit invocation only. Run only when the user sends `/work-phase`; do not trigger from a natural-language request.
---

# Work a spec-driven change

Read [`HANDOFF.md`](../../../docs/features/HANDOFF.md), its named change in progress (if any), the
[`spec overview`](../../../docs/specs/README.md), the relevant FS/TS/INV items, and
[`AGENT-WORKFLOW.md`](../../../docs/features/AGENT-WORKFLOW.md) §§1–6 and §10 completely. Then
follow the shared workflow; the workflow and specs take precedence over this launcher.

`$ARGUMENTS`, if present, names a change or the human’s requested change; otherwise use the
handoff's active change. If no change is in progress, do not choose future work yourself. Continue
until the change is done, a real blocker occurs, or quota requires a safe exit. Close every session
with the handoff update, commit rules, and human update.
