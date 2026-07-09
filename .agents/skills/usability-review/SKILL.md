---
name: usability-review
description: Run the top-to-bottom behavior-driven usability review of AgentDeck — build the real binary, start it on fresh/seeded/lived-in fixtures, and drive every user journey through the running app via subagents, per docs/phases/USABILITY-REVIEW.md. Finds what static diff reviews structurally miss (fresh-install crashes, unstyled surfaces, environment variance). Use when the user says "/usability-review", "usability review", "review it like a user", "test the app end to end as a user", or similar.
---

# Usability review — review the running app, not the diff

You are the **orchestrator** of a behavior-driven review. **Read-only toward product code: no code
changes, no fix commits.** Findings come from observed behavior of the running app — never from
reading the diff.

Follow the full protocol in [`docs/phases/USABILITY-REVIEW.md`](../../../docs/phases/USABILITY-REVIEW.md).
That's the single source of truth. The reminders below just help you start fast.

## Steps

1. **Read the protocol** (`docs/phases/USABILITY-REVIEW.md`) end to end, then diff its journey
   matrix (§3) against the shipped feature set (`MAP.md` feature→phase table) — flag uncovered
   surfaces before running anything (§7).
2. **Prepare once** (§2, §5): build the binary via the real build line (`-tags sqlite_fts5`; plus
   the untagged fallback binary for J8), build `fakeacp`, create the `fresh/` / `seeded/` /
   `lived-in/` fixtures under a review-owned `AGENTDECK_HOME` temp root.
3. **Phase A — static sweeps** (§4): fan S1–S5 out to cheap read-only subagents; use their hits to
   seed KNOWN RISKS in the journey charters.
4. **Phase B — journeys** (§3, §5): dispatch each charter to a subagent with its own port + fixture
   copy; enforce the report schema and budget rules. You orchestrate — you don't execute journeys
   yourself, and subagents don't debug the app.
5. **Verify** (§5): spot-replay every BLOCKER's repro yourself before reporting; downgrade what you
   can't reproduce to *unconfirmed*.
6. **Report** (§6): severity-mapped findings (BLOCKER/MAJOR → BLOCKING, MINOR → ADVISORY) into
   `## Review findings` in `HANDOFF.md` using the AGENT-WORKFLOW §8 entry shape (prefix titles with
   `J#`/`S#`), so `/fix-review` consumes them unchanged. New systemic classes → `INVARIANTS.md`.
   Give the human the checkpoint matrix + top-5 executive summary. All-PASS journeys are stated,
   not omitted.
