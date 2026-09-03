---
name: release
description: Explicit invocation only. Run only when the user sends `/release`; do not trigger from a natural-language request.
---

# Cut a release

Use the injected handoff header when present; otherwise read only **Current position** and **Active
change** in [`HANDOFF.md`](../../../docs/features/HANDOFF.md). Open its **Review findings**,
**Acceptance gates**, and **Blocked on human** sections for readiness, then read FS-10 and
TS-06.R13–R22 for the release contract,
FS-18 and TS-11 for the shipped `operating-agentdeck` package, and
[`AGENT-WORKFLOW.md`](../../../docs/features/AGENT-WORKFLOW.md) §§2–6 and §16 completely. Then
follow the shared workflow; the workflow and specs take precedence over this launcher.

`$ARGUMENTS`, if present, names the intended version. Otherwise propose one from the release range
and confirm it with the user before tagging.

The release range runs from the previous `vX.Y.Z` tag to `main`. Read it for behavior an agent
operating AgentDeck must know, and refresh the embedded package under
`internal/agentknowledge/operating-agentdeck/` — the single source, never an installed cache view
and never this repository's own skill launchers. State plainly when the range changes nothing
agent-facing. Do not add features or fix findings here; an open **Must fix** finding returns to
`/fix` before a release.

Publishing is outward-facing and irreversible. Verify with the full product checks plus `make dist`,
and push `main` and the tag only with explicit authorization; stop at the unpushed tag otherwise.
Close with the handoff update, commit, and concise human update written as readable release notes.

Archiving the handoff is part of closing a release (workflow §16.7): move what this version settled
to `docs/archive/state/`, leave live only what the next change needs, and keep the injected Current
position plus Active change slice under 8 KiB.
