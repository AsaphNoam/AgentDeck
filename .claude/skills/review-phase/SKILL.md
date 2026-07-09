---
name: review-phase
description: Review the last commit or merged PR on this repo against the AgentDeck phase specs — checking spec adherence, dead code, bad practices, and flagrant bugs. Does NOT chase micro-optimizations or rare edge cases; only flags real issues from normal usage. Use when the user says "/review-phase", "review the last commit", "review the other agent's work", "check what was just built", or similar.
---

# Review the last agent's work

You are reviewing another agent's commit or PR — not your own. **Read-only: no code changes, no commits.**

Follow the full review protocol in [`docs/phases/AGENT-WORKFLOW.md`](../../../docs/phases/AGENT-WORKFLOW.md) **§8**. That's the single source of truth. The reminders below just help you start fast.

## Steps

1. **Find the target.** Check for a recently merged PR (`gh pr list --state merged --limit 3`); if none, use the last commit (`git log -1`). The human may also name a specific commit or range — use that.
2. **Get the diff.** `git show <sha>` or `git diff <base>..<head>`.
3. **Read HANDOFF.md** to know which phase was just completed, then open its PRD (`docs/phases/phase-N-*.md`) and tech spec (`docs/phases/tech/phase-N-*-techspec.md`).
4. **Review against the four criteria** (§8): spec adherence · dead code · bad practices · flagrant bugs.
5. **Sweep against the paid-for bug classes.** Check the diff against every class in [`docs/phases/INVARIANTS.md`](../../../docs/phases/INVARIANTS.md) and tag each finding with its class number — a match is strong evidence the finding is real.
6. **Skip** style nits, micro-optimizations, and edge cases that won't arise in normal use. The bar: *would this cause a real problem during regular usage?*
7. **Peer-review the pending autonomous decisions** (workflow §8/§10). The human is zoomed out — you
   are the reviewer of record: audit each un-endorsed `## Autonomous decisions` entry in `HANDOFF.md`
   against the specs; endorse sound ones (append `— peer-reviewed <date>`), convert unsound ones into findings.
8. **Categorize** each finding as **BLOCKING** or **ADVISORY**.
9. **Record + report.** Write **every** finding — BLOCKING *and* ADVISORY — to `## Review findings` in
   `HANDOFF.md` in the §8 entry shape (that's what the fix step consumes). If a finding reveals a
   genuinely new systemic class, append it to `INVARIANTS.md` (doc writes are allowed; code writes are
   not). Then report to the human via the **brief** (workflow §10): prepend a ≤250-word plain-language
   entry to `docs/phases/BRIEFS.md` — severity counts, each BLOCKING finding as one plain sentence,
   advisories as a count + theme — and paste it as your end-of-turn message. If nothing found, the
   brief says so plainly; write nothing to the handoff.
