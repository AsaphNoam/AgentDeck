---
name: work-phase
description: Explicit invocation only. Run only when the user sends `/work-phase`; do not trigger from a natural-language request.
---

# Work a spec-driven change

Read [`HANDOFF.md`](../../../docs/features/HANDOFF.md), its named active work package (if any), the
[`spec overview`](../../../docs/specs/README.md), the relevant FS/TS/INV items, and
[`AGENT-WORKFLOW.md`](../../../docs/features/AGENT-WORKFLOW.md) §§1–6 and §10 completely. Then
follow the shared workflow; the workflow and specs take precedence over this launcher.

`$ARGUMENTS`, if present, names a work package or the human’s requested change; otherwise use the
handoff's active package. If no package is active, do not choose from the product backlog. Continue
until the change is done, a real blocker occurs, or quota requires a safe exit. Close every session
with the handoff update, commit rules, and human update.
