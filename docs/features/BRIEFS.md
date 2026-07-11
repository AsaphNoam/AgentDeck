# AgentDeck — Human Briefs

Read only the first entry to return to the project after time away. Each session prepends one
standalone brief (maximum 250 words) and returns that exact text as its final chat response.
Older entries are immutable history; agents resume from [`HANDOFF.md`](HANDOFF.md), not this file.

---

### 2026-07-11 — implementation: Phase 7.6 + 7.7 configuration federation (backend + UI)

Phase 7's entire un-gated scope is COMPLETE on branch `claude/work-phase-hwv0z6` (8 green commits this
session). **7.6 (backend):** a `SourceManager` holds immutable per-(backend,project) generations and
resolves sources FRESH at launch — the correctness boundary: a stale/invalid/unapproved source blocks
the launch (422/409) instead of composing from cache. It watches files (fsnotify + 250 ms debounce +
30 s sweep) and mirrors a redacted cache. REST routes cover discover/preview/bind/refresh/detach with
preview-token consent (TOCTOU + expiry) and publish `config_source_update` over SSE. Launch freezes a
redacted `launch_config_json` (migration v8); resume is frozen-by-default with opt-in
`config_refresh:true`; a reserved `agentdeck-messaging` MCP-id collision returns 409. **7.7 (UI):**
zod schemas + React Query hooks + SSE invalidation; a `ConfigSourcePanel` on Claude/Codex backend
cards (discover→preview→Link, health, Refresh, Unlink, redacted effective view with provenance labels
+ inventory — never source contents/secrets); an optional onboarding Config step reusing it. Both Go
variants + 88 UI tests + build green; `make embed` done.

**Needs attention:** **New:** Detached config-source import deferred (`DELETE ?detach=true` → 501; no
verified launch-injection path for Claude/Codex assets yet — unbind works). **Carried:** Terminal
support boundary; HTTP-only agent messaging; Immediate/prompt-based UI; Runtime-switch fallbacks;
Unbounded transcript indexing; Agent env inheritance by design; Local API trusts same-machine callers;
API/model compatibility. Two ADVISORY 7.7 UI refinements (bound-source override editing; NewAgentModal
invalid-source pre-warn). Live acceptance 7.4 + 7.8 remain credential-gated.

**Next:** human — provide `opencode`/`openhands` + Claude/Codex credentials to clear gated 7.4/7.8, or
an agent picks up the two ADVISORY UI refinements.

---

### 2026-07-11 — documentation: AgentDeck learning atlas

A standalone, searchable [AgentDeck learning atlas](/Users/mcnoam/Projects/AgentDeck/docs/agentdeck-learning-atlas.html)
now teaches the repository through modern AI-tool development first (ACP, MCP, lifecycle,
permissions, provenance), then architecture, then Go. It uses AgentDeck’s own flows and linked
source ownership rather than line-by-line code, distinguishes delivered Phase 7.5 federation
foundations from planned 7.6+ work, and passed HTML/link checks plus the full Go test checkpoint.

**Needs attention:** New/changed: the local checkpoint is staged but cannot be committed because
the execution environment's Git escalation reached its usage limit. Carried: Terminal support
boundary; HTTP-only agent messaging; Immediate/prompt-based UI; Runtime-switch fallbacks;
Unbounded transcript indexing; Agent env inheritance by design; Local API trusts same-machine
callers; API/model compatibility. Live 7.4 and 7.8 acceptance remain credential-gated.

**Next:** Agent — create the staged local checkpoint when Git access resumes; Human — decide
whether the atlas should be linked from broader user-facing documentation.

**What this teaches:** ACP controls an agent session while MCP equips an agent with tools; a
reliable AI product also needs explicit state ownership, recoverable live streaming, and
launch-scoped authority.

---

### 2026-07-11 — planning: Phase 7 knowledge MCP and workflow guardrails

Published `feature/phase-7-knowledge-base` and removed every stale remote branch, leaving only it
and `main`. Phase 7 now reserves 7.9, after Claude/Codex federation acceptance, for a
binary-versioned AgentDeck knowledge base: registered live agents will retrieve release-matched,
non-secret product topics through `agentdeck_docs`, while existing role files remain user-owned. A
Sol review rejected the stale fable workflow branch wholesale but carried forward four compatible
canonical safeguards: do not weaken tests to reach GREEN, self-review the full diff before checkpoint
commits, read specs before review diffs, and require a concrete normal-use trigger for every finding.

**Needs attention:** New/changed: none. Carried: Terminal support boundary; HTTP-only agent
messaging; Immediate/prompt-based UI; Runtime-switch fallbacks; Unbounded transcript indexing; Agent
env inheritance by design; Local API trusts same-machine callers; API/model compatibility. Live
acceptance 7.4 and 7.8 remain credential-gated.

**Next:** Agent — implement 7.6 federation manager/API/launch integration, with 7.9 following
federation acceptance.

---

### 2026-07-11 — implementation: Phase 7.5 configuration federation

Phase 7.5 is GREEN in the working tree. AgentDeck now has a validated, owner-only source-binding
manifest and pure Claude Code/Codex resolvers. They preserve provider-native precedence, field
provenance and setup fingerprints while enforcing explicit canonical-root approval; setup walks are
allowlisted and bounded, malformed inputs retain sanitized partial reports, and secrets never enter
resolver outputs. Tera implemented Claude discovery/resolution, Luna implemented the manifest and
Codex regression matrix, and the primary context owned architecture, Codex integration and the full
checkpoint. Whole-module build, both Go test variants and focused race tests pass.

**Needs attention:** **New:** the required checkpoint commit/push is pending because Git escalation
was rejected when the execution environment hit its usage limit; the tree is green and must be
committed unchanged. **Carried:** Terminal support boundary; HTTP-only agent messaging;
Immediate/prompt-based UI; Runtime-switch fallbacks; Unbounded transcript indexing; Agent env
inheritance by design; Local API trusts same-machine callers; API/model compatibility. Live
acceptance 7.4/7.8 remains credential-gated.

**Next:** agent — commit the Phase 7.5 GREEN tree, then implement 7.6 source manager/API/launch integration.

**What this teaches:** Treating native CLI setup as metadata-bearing federation—not copied universal
config—preserves each provider’s semantics while keeping consent, provenance and redaction enforceable.

---

### 2026-07-11 — maintenance: merged main into the security branch (complete)

Branch `claude/agentdecker-security-review-urhvp2` had drifted behind `main`, which gained the Phase 7
configuration-federation spec (`cf3a68f`, `f0c14d3`) while the security batch sat on the branch. Merged
`origin/main` into the branch and resolved the conflicts — all were in the two state docs (HANDOFF,
BRIEFS), where both sessions had prepended same-day entries; both sides were kept, and the federation
changelog entry's stale "push not yet authorized" note was corrected (it is on `origin/main`). No code
conflicts: the incoming side was documentation-only. Verified green after the merge (Go build + both
test variants).

**Needs attention:** *Carried:* the two 2026-07-11 security HUMAN decisions (Agent env inheritance by
design; Local API trusts same-machine callers) and the six older HUMAN items still await your verdict;
7.4/7.8 acceptance remain credential-gated.

**Next:** human — merge the branch to `main` (now a clean fast-forward); agents then resume 7.5 on trunk.

---

### 2026-07-11 — specification: Phase 7 configuration federation

Phase 7 now treats Claude Code/Codex setup as federated configuration rather than a one-time copy.
Linked mode is preferred: native files remain authoritative, while AgentDeck stores bindings,
overrides, provenance and fingerprints. Mirrored mode is a rebuildable compatibility cache; detached
snapshot preserves the old independent-import option. The specs cover models/provider/effort plus
native instructions, skills, agents, rules/hooks/plugins and MCP servers, following the documented
[Claude configuration hierarchy](https://code.claude.com/docs/en/settings) and
[Codex precedence](https://developers.openai.com/codex/config-basic).

Auto-sync is defined as watch + reconciliation + mandatory launch-time freshness, with stale config
blocking dependent launches. External files are never written; auth stores and secret values are
never imported. Existing sessions retain frozen high-level settings, while new launches resolve the
latest valid source. Phase 7 adds implementation-ready API/SSE/UI contracts and subphases 7.5–7.8;
7.5 is now next. Go build, both Go test variants, all 83 UI tests, and UI build pass.

**Needs attention:** New/changed: checkpoint `cf3a68f` is committed locally, but direct push to
`origin/main` was rejected because this request did not explicitly authorize publishing to the shared
default branch; authorize it if you want it pushed. Carried: the six existing HUMAN decisions remain
unchanged. Live OpenHands/OpenCode and federation compatibility checks remain acceptance gates.

**Next:** Agent implements 7.5: the source-binding schema and pure, redacted Claude/Codex resolvers.

**What this teaches:** A pointer-based source of truth still needs explicit snapshot and freshness
semantics; otherwise resume behavior and watcher misses quietly recreate configuration drift.

---

### 2026-07-11 — fix-review: security review, all 7 findings resolved (complete)

All seven security findings are dispositioned at a green checkpoint (both Go test variants; UI
untouched). The work lives on branch `claude/agentdecker-security-review-urhvp2` — this session was
restricted to that branch, so it needs a merge to `main` to restore the trunk rule.

Fixed (5 of 7, one root cause each):
- **DNS rebinding, WebSocket origin, CORS-as-auth, middleware bypass** (findings 1–3, 6): the
  loopback server never checked *which site* the victim's browser was acting for. A new `localOnly`
  guard now wraps every route (API, `/mcp`, terminal WebSocket, static UI): requests whose Host or
  Origin isn't localhost get 403, closing rebinding, cross-site terminal keystrokes, and no-preflight
  CSRF. Regression tests cover each path; new invariant §14 documents the boundary.
- **World-readable `~/.agentdeck`** (finding 7): confirmed real — API keys, state.db, and transcripts
  were group/other-readable. The whole tree is now owner-only, including re-tightening homes created
  by older builds.

**Needs attention:** *New this session:* **Agent env inheritance by design** (agents see the full
server environment per spec; allowlist would break provider keys) and **Local API trusts
same-machine callers** (no API auth; browser paths closed, other local OS users not) — both need
your verdict. *Carried:* Terminal support boundary, HTTP-only agent messaging, Immediate/prompt-based
UI, Runtime-switch fallbacks, Unbounded transcript indexing, API/model compatibility; 7.4 live
acceptance still blocked on credentials.

**Next:** human — merge the branch to `main` and rule on the two new HUMAN items.

---

### 2026-07-10 — fix-review: all eight usability BLOCKERs cleared (complete)

All eight open blocking usability findings (2026-07-09 & 2026-07-10 runs) are fixed, each with a
regression test, at a green checkpoint (both Go test variants, both builds, UI 83 tests + build;
pushed as `062cb5d`).

What changed:
- **Two crash-on-null bugs** (empty Archive, a new agent's chat): the server sent JSON `null` where the
  UI expected a list — fixed at the source and guarded in the UI.
- **Settings was unstyled**: the components referenced ~40 CSS classes no stylesheet defined; I wrote
  them, including the fix for ids overlapping model labels.
- **Misleading first-launch error**: launching a project whose folder is missing now says so by name,
  not blaming the agent binary.
- **First "New agent" dialog stuck open**: now a single stable element that survives the empty→populated
  switch, so it closes on success.
- **Silent failures**: release-group, cancel, notifications, and the config editors now show real errors
  instead of vanishing or printing "HTTP 400".
- **Unread badge never cleared / onboarding never finished**: reading mail now refreshes the badge;
  onboarding launches the project you just created, so setup completes.

**Needs attention:** No new blockers. Carried: the six open HUMAN decisions in HANDOFF (terminal-support
boundary, HTTP-only messaging, immediate/prompt-based UI, runtime-switch fallbacks, unbounded transcript
indexing, API/model compatibility) are unchanged, still awaiting your acknowledgement.

**Next:** Agent can take the remaining ADVISORY items, or you can proceed to Phase 7.4 (gated on your
`opencode`/`openhands` CLIs + provider keys).

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

---

### 2026-07-09 — docs — Reporting rezoomed: you read short briefs now, not the handoff

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
