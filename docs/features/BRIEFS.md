# AgentDeck — Session briefs

Newest first. Each entry is the exact final response from a feature-design, implementation, review,
fix-review, or usability-review session. Agents resume from [`HANDOFF.md`](HANDOFF.md), not this history. Earlier
entries are preserved in [`../archive/state/BRIEFS-pre-sdd.md`](../archive/state/BRIEFS-pre-sdd.md).

### 2026-07-16 — implementation: Codex role and project prompts

Codex chat agents now receive the selected role and project guidance. AgentDeck sends the frozen composed prompt through Codex’s supported session configuration instead of the ACP field Codex ignores, while preserving any other Codex configuration you already provide. Invalid configuration overlays stop the launch clearly instead of silently losing the role. Claude behavior is unchanged.

**Needs attention:** A real authenticated Codex new-chat and resume check is still needed to confirm the adapter applies the prompt end to end.

**Next:** Run that live Codex acceptance check before claiming provider-level compatibility.

### 2026-07-16 — fix: installer, chat history, and setup resilience

The installer now keeps no-start and non-interactive choices when it takes its installation lock. Chat history now saves your messages as well as agent output, so reloads, archives, resumes, and search keep the complete conversation. Hand-edited incomplete backend settings no longer crash the dashboard; setup errors now explain the next step, and the configuration-source panel is styled correctly.

**Needs attention:** None.

**Next:** The remaining release gates are the already-recorded live provider checks.

### 2026-07-16 — usability review: first-run, chat, grid, archive, and settings in a real browser

I built the app the way users get it and drove it through a browser: first launch and the full setup
wizard, creating and chatting with an agent, the card grid, the session archive and search (on both
build types we ship), and the Settings screens — including the new per-project shared folder. Most of
it holds up well: the first screen loads cleanly and styled, the setup wizard walks all the way to a
running agent, chat works, the grid layout survives a restart, archive search works on both builds,
and the new shared-resources folder shows its path correctly as a read-only value. All of that ran
with no browser errors.

I did find four things worth fixing, two of them serious:

- If you hand-edit the backend configuration file and leave out its main section (an easy slip, since
  we tell people the config is editable), opening the Backends settings crashes the **entire**
  dashboard to a generic "Something went wrong" — with no hint that a file is the cause.
- The app never saves your own chat messages, only the agent's replies. So when you reopen, resume, or
  archive a conversation, your side of it is gone, and searching the archive can't find a session by
  what you asked — only by what the agent said. (A past review noted the reload glitch as minor; the
  archive and search impact makes it bigger than that.)
- When a credential check fails during setup, the message is a raw code like "cli_not_installed" plus
  "check your settings," which doesn't tell you what to actually do.
- One setup panel (linking your existing CLI config) renders unstyled.

The permission-prompt, terminal, resume/switch, multi-agent messaging, and failure-recovery journeys
were not exercised this session; they're recorded as not-run, not as passing.

One disclosure: while setting up a test I accidentally stopped your own running dashboard for a moment
and restarted it — it's back up on its normal port with its previous state intact.

**Needs attention:** Two new must-fix issues — the config-file edit that kills the whole dashboard,
and chat history that silently drops your messages — plus the still-open installer flag issue from
before a macOS release.

**Next:** Run `/fix` to work through the new findings, starting with the dashboard-crash and the
missing chat history.

### 2026-07-16 — review: current review boundary

There is no new product code to review: the recorded review boundary already reaches the latest
implementation. The repository's specification checks still pass, and the earlier macOS installer
flag issue remains the only open review finding.

**Needs attention:** Repair the installer flag handoff before publishing a release.

**Next:** Run `/fix` to preserve those choices through the locked installer process and add
interactive coverage.

### 2026-07-16 — review: project shared resources

The completed project shared-resources work holds up. Every project's AgentDeck-owned folder is
created before the project is saved and is handed to each agent you launch, resume, or switch in the
same consistent way; the folder stays owner-only and outside the code repository, and its contents
are never read into the dashboard, its API, or logs — only the path is shown, read-only in Settings,
and reported before a project delete. The written specifications, the automated Go tests, and the
specification checks all agree with what shipped, and I found no new problems in this range.

**Needs attention:** The earlier macOS installer flag issue is still open and still blocks a release.

**Next:** Run `/fix` to repair the installer flag handoff before publishing a release.

### 2026-07-15 — implementation: project shared resources

Every project now gets its own AgentDeck-owned folder that lives outside the project's code
repository, so agents have a reliable place to leave and reuse working material (specs, guides,
research, test results) without any risk of it becoming an accidental commit. The folder is created
when you make a project and is handed to every agent you launch — as an accessible directory, an
environment variable, and a short instruction — while the agent still works in your actual project
directory. Its path shows up in Settings as a read-only value you can copy, and deleting a project
leaves the folder in place (Settings tells you the retained path first) so no saved work is lost by
accident. The folder is owner-only and its contents are never read into the dashboard, its API, or
logs; only the path is shown.

**Needs attention:** None.

**Next:** Nothing required. The one open item elsewhere is still the macOS installer flag fix
(`/fix`) before a release.

### 2026-07-15 — review: macOS installer locking

The updated macOS installer can lose explicit no-start and non-interactive choices when it restarts
under its new concurrency lock. In an interactive terminal, that can unexpectedly prompt, edit the
shell profile, run sign-in, or start the dashboard. The rest of the reviewed release, workflow, and
planned project-resources work is consistent, and automated checks pass.

**Needs attention:** Repair the installer flag handoff before publishing a release.

**Next:** Run `/fix` to preserve those choices through the locked installer process and add
interactive coverage.

### 2026-07-15 — workflow: review command

Use `/review` to inspect the unreviewed completed work. The Codex and Claude workflow skills now
use that same command name; its review behavior is unchanged.

**Needs attention:** None.

**Next:** Use `/review` when you want the next completed change checked.

### 2026-07-15 — workflow: work and fix commands

Use `/work` to build a change and `/fix` to repair recorded findings. When no change is active,
`/work` now starts the only waiting ready change; if several are waiting, it asks you which one to
start instead of silently choosing or claiming there is nothing to do.

**Needs attention:** None.

**Next:** Run `/work` to start the waiting project shared-resources change.

### 2026-07-15 — implementation: no active change

There is no active change to implement, so no product work was started. The repository is clean and the next implementation must be selected explicitly from the ready changes.

**Needs attention:** Choose which ready change to start.

**Next:** Name a ready change, then run `/work-phase` again.

### 2026-07-15 — feature design: project shared resources ready

Project shared resources are now fully designed and waiting to build. Every project will get one
private AgentDeck folder outside its repository, where people and its agents can share notes,
specifications, research, test harnesses, and validation results without risking a commit. AgentDeck
will make the folder available to every new, resumed, or switched agent, show its path in Settings,
and retain it if the project configuration is removed. It will not inspect, search, sync, or expose
the folder’s contents.

**Needs attention:** None.

**Next:** Start the waiting project shared-resources change when you want implementation.

### 2026-07-15 — feature design: project shared resources clarification

Proposed behavior:

- Every project gets a stable folder at `~/.agentdeck/project-resources/<project-id>/` (or the
  equivalent AgentDeck data home), outside the repository.
- AgentDeck creates it with the project, or lazily when an existing project first launches an
  agent. It stays empty until a person or agent writes something there.
- Each new project agent receives the path and a clear instruction to use it for shared notes,
  specifications, research, harnesses, and validation results. Its ordinary working directory stays
  the repository.
- Settings shows the path for copying, but does not let a person relocate it. AgentDeck does not
  scan, search, sync, display, or otherwise interpret its contents.
- Removing the project configuration retains the folder and its contents. That is the proposed safe
  default, because deleting configuration should not quietly erase useful work.

Repository-resident folders, configurable locations, cloud sync, content browsing, and automatic
cleanup are not part of this feature.

The feature-design instructions now require this concrete explanation in conversation before asking
for confirmation.

**Needs attention:** Confirm whether retained project folders after project deletion are the desired default.

**Next:** Confirm the proposed behavior, or change the retention rule, then complete the technical design.

### 2026-07-15 — fix review: macOS release delivery

The release installer and updater now prevent a second run from changing the selected runtime while
another run is in progress. The stable command is replaced safely during updates, and the macOS
release workflow now exercises the release checks, including the fresh bootstrap path.

**Needs attention:** Real provider sign-in and chat checks still require credentialed manual testing.

**Next:** Run the credentialed provider acceptance checks before making release compatibility claims.

### 2026-07-15 — feature design: project shared resources

The feature draft gives every project one stable, AgentDeck-owned folder outside its repository for
shared agent material such as specifications, research, validation harnesses, and working notes.
Agents will be told its path at launch while continuing to work in the repository; removing a
project will retain the folder so useful material is not silently lost.

**Needs attention:** Confirm this behavior before the technical design chooses the filesystem and launch-composition details.

**Next:** Confirm the feature scope, or say what should change, then complete the technical design.

### 2026-07-15 — review: macOS release installer

The macOS release installer is not ready to publish: two installer or update runs can both activate
a runtime, the stable command can be briefly truncated during an update, and the release workflow
skips required integrity, update, rollback, and non-interactive checks. The automated specification,
test, build, and distribution checks otherwise pass.

**Needs attention:** Fix these release-path defects before publishing a release.

**Next:** Run `/fix-review` to repair and verify the findings.

### 2026-07-15 — implementation: macOS release installer

AgentDeck can now be installed from a macOS Apple-silicon GitHub Release without a source checkout,
Go, Node, npm, or global ACP adapters. The installer verifies the release archive, keeps the app
runtime separate from your AgentDeck data, offers guided provider sign-in, and supports explicit
update, check, and rollback commands. Release publishing now builds and verifies the private runtime
and documents the intentional no-signing/no-notarization and Gatekeeper limitations.

**Needs attention:** A real Claude or Codex sign-in is still a credentialed manual acceptance check.

**Next:** Publish a version tag when a release is ready for friends to install.

### 2026-07-15 — feature design: macOS release installer

The installer is now fully specified and ready to build for Apple-silicon Macs. It will download a GitHub Release containing AgentDeck, its own Node runtime, and the official Claude and Codex adapters, so friends will not need a repository, Go, npm, or globally installed adapters. It keeps the app runtime separate from your existing AgentDeck data, offers provider sign-in without handling credentials itself, starts the dashboard after an interactive install, and supports manual check, update, and rollback commands.

The MVP intentionally uses GitHub Release checksums but no code signing or notarization. That keeps publishing lightweight, but macOS may ask each friend to approve an unidentified developer, and checksums cannot independently prove publisher identity if the release account were compromised. There are no automatic updates, Homebrew package, Intel build, or other platforms in this first version.

**Needs attention:** None.

**Next:** Start the waiting macOS release-installer change when you want implementation to begin.

### 2026-07-15 — implementation: official Claude adapter

AgentDeck now launches the official `claude-agent-acp` adapter instead of the retired Zed-era executable. Credential checks go through that adapter’s bundled Claude executable, and source installs pin the reviewed official package with its Node 22 requirement. The launch metadata AgentDeck already sends—system prompt, model, extra directories, and messaging registration—matches the new adapter’s contract.

This removes the obsolete adapter dependency and gives the future regular installer one current Claude runtime to bundle and update. It does not yet make the release self-contained: the installer change still needs to package the adapter and a private Node runtime. All automated tests and release builds pass.

**Needs attention:** A credentialed Claude chat, resume, and messaging run against the pinned official adapter remains required before making a live-provider compatibility claim.

**Next:** Define the regular installer bundle around the pinned Claude adapter and a private Node runtime.

### 2026-07-15 — feature design workflow

The new `/design-feature` skill turns either a prompted idea, a named recorded idea, or—when no idea
is given—the first item under “New ideas” into implementation-ready work. It first collaborates with
you on the product behavior and acceptance criteria, then pauses for your confirmation before writing
the technical design. Meaningful architectural tradeoffs come back to you with options and a
recommendation instead of being decided silently.

A completed run leaves planned feature and technical specifications plus a ready-change file. It
does not write product code or mark implementation as started, and it will leave the idea in design
instead of calling it ready while an important decision is unresolved.

**Needs attention:** None.

**Next:** Invoke `/design-feature` with an idea, the title of an existing idea, or no argument to
start with the first new idea.

### 2026-07-15 — verification status clarified

Detached federation import has not shipped: asking AgentDeck to detach a linked Claude or Codex setup
returns a clear “not implemented” response, and no native configuration is copied. It stays a known
capability gap until there is a verified way to inject an AgentDeck-owned copy into each provider.

The remaining real-world checks are now explicit: test Claude and Codex chat, messaging, resume,
terminal behavior, and federation with real authenticated CLIs; OpenCode/OpenHands need installation
before their equivalent checks. This machine already has Claude Code, Codex, and the Codex ACP adapter.

**Needs attention:** Authorize a disposable, credentialed live-provider test run before AgentDeck makes
compatibility claims for those CLI features.

**Next:** Run the Claude/Codex acceptance checks against disposable configuration homes.

### 2026-07-14 — Codex model autosync

The New Agent model picker was stale (sonnet-4-6/gpt-5.5) while the native CLIs had moved on. It turns
out the available models *are* stored on disk, but differently per provider: **Codex** publishes a
machine-readable catalog at `${CODEX_HOME:-~/.codex}/models_cache.json`, while **Claude** compiles its
list into the CLI binary (settings.json holds only the selected model). So the Codex half was cheap to
automate and shipped; the Claude half stays an idea.

New behavior (FS-09.R28/A8): a `codex-acp` backend can set `autosync_models: true` (a checkbox in
Settings → Backends). On dashboard startup, after seeding, AgentDeck reads the Codex cache and
**add-only** merges every user-visible model (`visibility:"list"`) into the backend's catalog, keyed by
the Codex slug. It never edits or removes an existing entry, never changes `default_model`, writes
nothing when there's nothing new, and treats a missing/unparseable cache as a silent skip that can't
block startup. Implementation is `internal/config/codexmodels.go` (`ReadCodexModelCatalog`,
`syncCodexModels`, `Store.AutoSyncBackends`) invoked from `resolveConfig` in the dashboard CLI, plus the
`AutoSyncModels` config field and the UI toggle. Verified with new Go tests and a live restart that
pulled gpt-5.6-sol/terra/luna and gpt-5.4/-mini into the catalog while leaving gpt-4o and a hand-added
entry intact; full GREEN checkpoint (both Go variants, build, 95 UI tests, UI build) passed.

**What this teaches:** on macOS, `cp`-ing a binary over itself *in place* while a copy is running
corrupts its ad-hoc code signature, after which the kernel stalls or kills execs of that file (it
looked like a hung binary despite an identical shasum). Reinstall with remove-then-copy to a fresh
inode. This bit the `agentdeck` PATH binary mid-session until a clean `rm && cp` fixed it.

**Needs attention:** None.

**Next:** Restart your dashboard so the picker shows the synced Codex models. Optionally set your
preferred new model (e.g. gpt-5.6-terra) as the Codex default in Settings → Backends.

### 2026-07-14 — auto-generated project ids

Creating a project no longer asks you for a "Project ID (slug)". That field was the source of a
confusing failure: the id had to match `^[a-z0-9][a-z0-9-]{0,62}$`, so typing a normal name like
`AgentDeckDemo` (capitals) or leaving the separate slug field blank produced a cryptic
"must match ^[a-z0-9]..." error even though the title was fine. The id is now derived on the server
from the title as `slug(title)-<timestamp>` — e.g. title "AgentDeck Demo" becomes
`agentdeck-demo-20260714t204631z` — and both the Settings → Projects form and the onboarding wizard
simply drop the slug input. The title (shown prominently) can be anything; the id is a stable,
filesystem-safe handle you never have to think about, still immutable once created.

This is a spec-driven change: FS-04 gained **R31** (derivation rule) and **A11** (acceptance), with
R6/R18 amended. The server keeps honoring an explicitly supplied id, so API/CLI callers are
unaffected; only an empty/absent id triggers derivation. Verified with new Go tests
(`TestGenerateProjectID`, `TestProjectsAutoGeneratedID`), updated UI tests, the full GREEN checkpoint
(both Go variants, `make build`, all 95 UI tests, UI build), and an end-to-end run in the live
dashboard creating a title-only project.

One judgment call worth flagging: the timestamp uses **local wall-clock** time with a literal `z`
suffix (matching the example you gave), even though `z`/`Z` conventionally denotes UTC. Say the word
if you'd prefer true UTC.

**Needs attention:** None.

**Next:** Create your real project from Settings → Projects (title + working directory only), then
launch an agent against it.

### 2026-07-14 — simpler future-work language

Future work now uses plain names: [ideas and improvements](../ideas.md) for thoughts to keep or
problems to improve, [ready changes](../ready-changes/README.md) for fully described work waiting to
start, and the handoff for the one change currently in progress. The letter-number labels and
“work package” terminology are gone. Specification requirement IDs remain because they link directly
to the exact rule being changed or checked.

**Needs attention:** None.

**Next:** Add a new thought under “New ideas” in `docs/ideas.md`; define it further only when you
want to explore it.

### 2026-07-14 — workflow skills: explicit invocation only

The work, review, fix-review, and usability-review skills now run only when you use their matching slash command. Natural-language requests no longer trigger them automatically, in either the Claude or Codex skill copies.

**Needs attention:** None.

**Next:** Use `/work-phase`, `/review-phase`, `/fix-review`, or `/usability-review` when you want one of these workflows.

### 2026-07-14 — historical-document cleanup

Archived handoffs and session updates now clearly say that their old labels and instructions describe a former process, not the one agents should follow today. The archive overview points readers back to the current workflow, specifications, and work state; older entries in this live brief file now carry the same reminder.

**Needs attention:** None.

**Next:** Agents should use the current workflow and handoff, and treat older records as context only.

### 2026-07-14 — workflow: remove redundant intent guidance

The workflow no longer teaches agents how to interpret ordinary requests. It retains only the one rule that needs to be explicit: agents must not independently select work from the backlog. The backlog, specification overview, handoff, and both work-phase launchers now use the same concise rule.

**Needs attention:** None.

**Next:** Future agents should follow the specifications and active work state without adding intent-classification rules.

### 2026-07-14 — workflow simplification

Agent instructions now use ordinary language instead of a private process dialect. They say “required checks,” “specification update,” “relevant requirement,” and “Must fix” where they previously used labels such as GREEN, checkpoint, spec delta, governing contract, and BLOCKING. Stable requirement IDs remain, because they are useful links to the exact behavior being discussed.

Human updates are now explicitly written for you rather than for another agent: plain language, no internal labels, no command inventories, and no requirement-ID strings unless you need one to decide or act. The canonical workflow, skill launchers, handoff/queue templates, review protocol, map, and related specifications now agree on that approach.

**Needs attention:** None.

**Next:** Future agents should follow the simplified workflow.

> **Earlier briefs are historical messages, not current instructions.** They preserve the exact
> language sent at the time and may use retired process labels. For current work, use
> [`HANDOFF.md`](HANDOFF.md) and [`AGENT-WORKFLOW.md`](AGENT-WORKFLOW.md).

### 2026-07-14 — implementation: dedicated ready-work queue

Ready-but-unstarted features now live in the dedicated
[`../implementation-queue/`](../implementation-queue/README.md), one `W-<number>-<slug>.md` work
package per feature. Each package links to its governing FS/TS/INV requirements and acceptance
evidence, and has a simple Ready → Active → Shipped/Paused/Retired lifecycle. `HANDOFF.md` now holds
only the checkpoint state of a package that has actually started; it is no longer a waiting list.

The workflow no longer requires special wording such as “consider,” “design,” or “build.” Agents
interpret the user’s normal language and conversation context, asking only when the desired level of
commitment is materially unclear. An exploratory idea stays in the product backlog; a requested
proposal enters discovery; a requested change becomes a Ready package once its FS/TS delta and
acceptance criteria are adequate. Work-phase reads the active package named by the handoff and never
self-prioritizes backlog items.

Verified: `make check-specs`, shell syntax, all twinned skills, and `git diff --check`.

::git-commit{cwd="/Users/mcnoam/Projects/AgentDeck"}

### 2026-07-13 — implementation: explicit idea intake and work selection

The former `docs/specs/backlog.md` was a **new** SDD-migration file, not a migrated historical
backlog. It has moved to [`../product-backlog.md`](../product-backlog.md), outside the authoritative
spec tree. Its provenance now says exactly what happened: B1–B8 were synthesized from archived
phase/future-work material and unshipped ideas; G1–G12 came from current-spec deviations, manual
gates, and migration audits. They are leads to revalidate, not inherited commitments.

The product backlog now separates **Inbox** (faithfully captured ideas), **Discovery**
(human-authorized spec/design work), **Ready to build** (specified and human-authorized work),
candidate features, and known gaps. FS/TS remain the grouped catalog of shipped capabilities:
Current specs describe shipped behavior, while Partial specs mark only selected, unshipped
requirements as `(planned)`.

The workflow, handoff, AGENTS guidance, repository map, README, and twinned work-phase skills now
enforce the selection boundary. “Consider” captures an Inbox item; “design” activates Discovery;
“build” activates Implementation after an adequate FS/TS delta. A work-phase agent executes only an
active `Implementation` item in `HANDOFF.md`; it cannot self-prioritize a candidate, gap, or
planned requirement. The handoff template requires source ID, stage, governing IDs, and a testable
Done-when line.

Verified: `make check-specs`, shell syntax, twin work-phase skill parity, and `git diff --check`.

::git-commit{cwd="/Users/mcnoam/Projects/AgentDeck"}

### 2026-07-13 — implementation: spec-driven development foundation

AgentDeck now has two authoritative specification sets: feature specs FS-00–FS-09 for observable
behavior and acceptance criteria, and technical specs TS-01–TS-07 plus INV for architecture,
protocols, persistence, security, delivery, and implementation constraints. Each spec has stable
R/A identifiers, an honest Current/Partial/Draft status, deviations, acceptance evidence, and code/
test traceability. The lifecycle is spec delta → disposable plan → implementation → GREEN → spec and
handoff update → bidirectional review; shipped items lose `(planned)`, retired IDs are never reused.

The repository instructions, Claude guidance, MAP/README, canonical agent workflow, usability
protocol, architecture orientation, and twinned work/review/fix/usability skills now route agents to
the governing FS/TS/INV items first. Traceability is enforced by exact citations in tests, plans,
specs, and commits; `make check-specs`, the Claude post-edit hook, `make test`, and clean-clone CI
check spec structure/index/status/links/citations plus both Go variants, vet, and UI tests/build.

The master PRD, phase PRDs/tech specs, old handoff/brief history, stale HTML guides, and completed
usability evidence moved under `docs/archive/`. An archive manifest maps every phase slice to its
current authority. Useful rationale remains in non-authoritative ADR/orientation docs; obsolete live
phase instructions were removed rather than maintained in parallel.

Current gaps are explicit: FS-07–FS-09 and TS-01/TS-04/TS-07 remain Partial; real-provider and
federation compatibility still need credentialed gates; prompt-history fidelity, frontend state
ownership, operational CLI behavior, local filesystem hardening, and uniform HTTP request bounds
need further spec work. The next step is a semantic audit of the highest-risk Partial area, starting
with real Claude/Codex federation/MCP/terminal acceptance, then promoting only the requirements the
evidence proves. The maintained queue is `docs/specs/backlog.md`.

**Needs attention:** HUMAN — Local API authentication; Child-process environment; Terminal and
messaging support boundary; Detached federation import; API/model compatibility. These are recorded
shipped boundaries or explicit planned work, not blockers.

**What this teaches:** SDD remains practical across short agent sessions when authority is small and
stable (R/A IDs), while sequencing and memory stay disposable (plans/handoff). Mechanical lint can
prove references and lifecycle hygiene; only bidirectional review can prove that authoritative prose
still matches executable behavior.

::git-commit{cwd="/Users/mcnoam/Projects/AgentDeck"}
