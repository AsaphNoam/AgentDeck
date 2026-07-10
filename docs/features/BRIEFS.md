# AgentDeck — Human Briefs

Read only the first entry to return to the project after time away. Each session prepends one
standalone brief (maximum 250 words) and returns that exact text as its final chat response.
Older entries are immutable history; agents resume from [`HANDOFF.md`](HANDOFF.md), not this file.

---

### 2026-07-10 — usability review: mock-driven test of the "needs-a-login" features (complete)

**TL;DR:** No product code changed. Your ask was to usability-test the features that normally need a
real Claude/Codex login — creating a chat, choosing a model, sending messages, switching agents — by
faking the agent CLI. I built that fake, **proved it works** (a real chat launches, streams a reply, and
shows the right busy→done status entirely through the running app, no login), and then drove **every
screen end-to-end** with it. A monthly spending limit interrupted the run twice; I resumed both times, so
coverage is complete (93 screenshots).

**New blocking problems found this run:**
- **Your very first "New agent" leaves the dialog stuck open** covering the screen — the agent is
  actually created, but you get no confirmation and the still-live button invites a duplicate. (Only the
  first launch; later ones close fine.)
- **Opening a brand-new agent's chat can crash the panel** (the empty transcript comes back malformed).
- **Message "unread" badges never clear** after the recipient reads the mail.
- **Onboarding can't actually finish** on a fresh install — the last step tries to launch a sample
  project whose folder doesn't exist, so it errors out and the wizard won't close.

**Still-there from last time (all re-confirmed):** the Archive page still crashes on open, the Settings
page is still completely unstyled, and several buttons still fail silently.

**Also worth knowing:** switching a running agent's model works and keeps the right model; resume,
restart-durability, and crash-recovery all behaved correctly. A gap-closure pass also drove the terminal
agents, Files/Commands tabs, the per-turn message budget, and the CLI — all passed.

**Next up:** the blocking problems above are queued for `/fix-review`. Full report:
[`usability-review-run-2026-07-10.md`](usability-review-run-2026-07-10.md).

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
