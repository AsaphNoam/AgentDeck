# AgentDeck — Human Briefs

Read only the first entry to return to the project after time away. Each session prepends one
standalone brief (maximum 250 words) and returns that exact text as its final chat response.
Older entries are immutable history; agents resume from [`HANDOFF.md`](HANDOFF.md), not this file.

---

### 2026-07-10 — workflow review: low-attention agent operation

The core build/review/fix loop was sound, but its intended human-facing brief layer was absent and the
Codex/Claude role skills had drifted. The workflow is now materially better with a narrow change: every
implementation, review, fix-review, and usability-review session stores and returns the same standalone
brief, capped at 250 words. The handoff remains precise agent recovery state; this log is the bounded
human re-entry point. The handoff was condensed from 826 lines while preserving every open finding.

Decisions now route by consequence. User-visible, security/data, interoperability, spec-deviating, or
costly-to-reverse choices remain HUMAN items and repeat in briefs until acknowledged; reversible local
choices go to an independent reviewer. Reviews persist all findings and their state, so advisories and
questionable choices cannot disappear between agents. Cold resume now starts by reconciling the worktree,
and all role entrypoints follow the moved canonical documents.

**Needs attention:** **Carried:** Terminal support boundary; HTTP-only agent messaging;
Immediate/prompt-based UI; Runtime-switch fallbacks; Unbounded transcript indexing; and API/model
compatibility. **New/changed:** The workflow commit was created locally, but the requested push was
rejected because this Codex environment hit its usage limit; product work is not blocked.

**Next:** The human commits this brief update and pushes `main`. Then an agent implements the optional
iTerm2 driver (unless skipped) and begins Phase 7; live-CLI gates must pass before compatibility claims.

**What this teaches:** Agent recovery context and human re-entry context are different products. Keeping
one precise and one bounded prevents both agent context debt and owner attention debt.
