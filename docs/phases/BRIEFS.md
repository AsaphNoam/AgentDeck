# AgentDeck — Human Briefs

**This is the only file the human reads.** Every work / review / fix / usability session ends by
prepending one brief here (newest on top) and pasting the same brief as the end-of-turn message.
Full contract — entry shape, length caps, writing rules: [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md) §10.
Keep the **last 4 briefs**; delete older ones when adding a new one (git keeps history).
Agent-to-agent state lives in [`HANDOFF.md`](HANDOFF.md) — never point the human there.

---

## 2026-07-10 — usability review — Mock-driven test of the "needs-a-login" features (complete)

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
restart-durability, and crash-recovery all behaved correctly.

**Honest gap:** three areas can't be faked yet — terminal agents, fully-automatic agent-to-agent chatter,
and usage budgets — so "every feature tested" isn't 100% until small extra fakes exist.

**Next up:** these are queued for `/fix-review`. Full report:
[`usability-review-run-2026-07-10.md`](usability-review-run-2026-07-10.md).

---

## 2026-07-09 — docs — Reporting rezoomed: you read short briefs now, not the handoff

**TL;DR:** No product code changed. The workflow now ends every session with a short plain-language
brief in this file — you read only this, never `HANDOFF.md`. Agents decide more on their own: the
review agent audits their judgment calls (a job that used to be yours), your silence on a brief
counts as consent, and per your instruction agents now work on `main` and push to origin
automatically when a task completes (force-pushes still ask).

**Where this fits:** AgentDeck is the multi-agent dashboard (a Go server with an embedded React UI)
being built phase by phase by autonomous agents. `HANDOFF.md` is the agents' shared memory between
sessions; until now it was also your review queue, which assumed reading time you don't have. That
role moves here, at digest granularity.

**Where the project stands:** Phases 0–6 are done — chat runtime, dashboard, settings, archive/search,
agent-to-agent messaging, terminal agents. Phase 7 (two new backends: OpenCode and OpenHands) is
code-complete and green against test fakes. A recent in-browser usability review left open findings,
four blocking: a fresh-install crash on the Archive page, an unstyled Settings page, a misleading
error on first launch, and UI actions that fail silently. They're queued for `/fix-review`.

**Needs your input:** Live verification of the new backends is blocked on you installing the
`opencode`/`openhands` CLIs plus provider keys (older siblings: the Codex CLI and MCP-registration
checks). Default if you never get to it: Phase 7 ships tested against fakes, gaps documented.

**Next up:** Run `/fix-review` to burn down the blocking findings before Phase 7 wraps.
