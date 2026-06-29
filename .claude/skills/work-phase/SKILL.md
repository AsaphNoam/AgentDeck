---
name: work-phase
description: Autonomously implement the next AgentDeck phase/subphase in spaced, quota-limited sessions. Reads the handoff, builds subphase by subphase to GREEN checkpoints, keeps the handoff current, and only stops when done, blocked on the human, or out of quota. Use when the user says "/work-phase", "work the next phase", "continue the build", "pick up the implementation", or hands you a phase to drive.
---

# Work a phase (spaced-session implementation loop)

You are driving AgentDeck's **fire-and-forget** implementation loop. Build the codebase one
subphase at a time, keep [`docs/phases/HANDOFF.md`](../../../docs/phases/HANDOFF.md) accurate
enough for the next agent (Claude **or** Codex) to resume cold, and keep going until the phase
is done, you're blocked on the human, or your quota runs out.

## The protocol is canonical — read it

**Read [`docs/phases/AGENT-WORKFLOW.md`](../../../docs/phases/AGENT-WORKFLOW.md) and
[`docs/phases/HANDOFF.md`](../../../docs/phases/HANDOFF.md) now, then follow the workflow's loop
(§1) exactly.** That doc is the single source of truth; everything below is just the operating reminders.

`$ARGUMENTS`, if present, names a specific phase/subphase to target (e.g. `5` or `5.3`). Otherwise
work whatever the handoff marks active.

## Non-negotiables (don't skip these even if you skim)

1. **Verify before you trust.** Confirm green on arrival: `go build ./... && go test ./...`. A subphase
   is only done when its GREEN checkpoint passes (`+ cd ui && npm run build` if it touched `ui/`).
2. **Do the work yourself.** Do **not** spawn coding subagents — they have Bash denied here and can't
   build/test, so they can't reach a checkpoint. Read-only Explore research is fine.
3. **Update + condense the handoff after every change**, and especially at each GREEN checkpoint:
   tick steps, collapse finished subphases, collapse finished phases to one line (workflow §5).
4. **Commit at every GREEN checkpoint** — code + `HANDOFF.md` together, **directly on `main`**
   (trunk-based: no per-phase branches, no PRs), message `phase N.M: <title> — green checkpoint`,
   with the `Co-Authored-By: Claude Opus 4.8` trailer. Do **not** push unless the user asked.
5. **Keep going** past each finished subphase to the next one. A green subphase boundary is the best
   place to be cut off, not a reason to quit.
6. **Only stop for STOP conditions** (workflow §3): unresolvable ambiguity, a checkpoint you can't get
   green, missing credentials/external input, or a destructive/irreversible action. When you stop, write
   it under `## Blocked on human` in the handoff with enough context to answer cold, and give the user a
   one-line summary of what's blocking and what's next.
7. **Flag every judgment call you made.** If something ambiguous or under-specified forced *you* to pick
   a design/implementation decision (without it being a hard blocker), record it under `## Autonomous
   decisions (please review)` in the handoff **and** call it out explicitly in your closing summary —
   never let the user discover a self-made decision by reading the diff (workflow §3).

## When you finish a session (any exit, including running low)

Leave the tree at a GREEN checkpoint (or the handoff clearly says what's half-done), `HANDOFF.md`
updated + condensed, work committed. Tell the user what moved and what's next — and **explicitly list
any autonomous decisions you made**, or say plainly there were none.
