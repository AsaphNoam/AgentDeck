# AgentDeck — Session briefs

Newest first. Each entry is the exact final response from a feature-design, implementation, review,
fix-review, or usability-review session. Agents resume from [`HANDOFF.md`](HANDOFF.md), not this history.
Briefs through 2026-08-21 are archived in
[`../archive/state/BRIEFS-through-2026-08-21.md`](../archive/state/BRIEFS-through-2026-08-21.md).

### 2026-09-02 — Review: the agent cards, the worktree design, and a stale embedded UI

The dashboard card work is finished and correct. Panes now stay in one grid column, so opening or
closing a conversation moves only the rows underneath it; the labelled **Collapse** button and
**Collapse all** both work through the same saved layout; long agent names wrap onto three lines
instead of being cut off; and the context meter has moved off the collapsed card onto the expanded
one. The two blocking problems from the last review are gone — the specification check passes and
the whole change is committed.

One new blocking problem replaced them, and it is invisible unless you look for it. The change
updated the app's source but not the built copy of the UI that ships inside the server. On a machine
that already has a build, pulling this and running the normal `make build` starts a dashboard
serving the *previous* interface, with no error and nothing on screen to explain it. Every earlier
UI change in this repository updated both together; the rebuilt file is sitting in the working tree
uncommitted. It is a one-command fix.

The Mermaid diagram problem is still only half fixed. Scrolling no longer disturbs a settled
diagram, but a diagram still flips back to raw source and re-renders each time the agent keeps
typing after it — which is the ordinary shape of a reply that contains a diagram, and is what was
originally reported.

The new worktree-projects design is strong where it matters most — how it calls Git, and how
carefully it verifies a checkout before deleting one — but three things should be corrected before
anyone builds it. It runs the checkout-creating, setup-running step inside the pipeline's *pre-flight
check*, which runs once per stage and is supposed to only look, never act; a 32-stage run would
trigger it 32 times and report failures as if a model were unavailable. Its crash-safety argument
does not hold for one of its own steps, which can leak a branch, a directory, and a database row
that nothing can ever clean up. And it never says what happens when you archive a worktree project,
agree to delete its checkout, and then change your mind and bring the project back — the two paths
in the design answer that opposite ways.

Two smaller worktree notes and two carried-over specification inconsistencies are recorded for
whoever picks this up.

**Needs attention:** The pipelines proposal work from another session is still sitting uncommitted
with no owner and no technical design behind it. It is either finished or reverted; leaving it is
the one thing that loses work.

**Next:** Commit the rebuilt UI file and fix the Mermaid streaming case, then correct the three
worktree-design points before that change starts.

### 2026-09-01 — Feature design: open a conversation waiting for approval

A chat agent that newly enters `waiting_input` now expands its own pane on the dashboard. That
reverses one clause of FS-02.R51, which had forbidden any automatic expansion so the grid would
never reflow while it was being read; every other event — a notification, `done`, `error`, mail, a
pipeline change — still opens nothing.

Only a newly observed transition opens a pane. A reload, a reconnect, a re-hydration, an agent first
seen already waiting, and returning to the dashboard with an agent already waiting all open nothing,
because otherwise every dropped connection would reopen panes you had deliberately collapsed. The
cost of that rule is stated plainly in the spec: a pane opens only while a card grid is on screen.
Collapsing an auto-opened pane keeps it collapsed until the agent asks again, and answering the
request leaves the pane open.

The trigger is the durable state on `state_update`, not the `permission_required`/`waiting_input`
notification the server already emits, because that stream is filtered by the notification mute list
and is never replayed to a reconnecting tab — muting a toast must not silently change what the
dashboard opens.

You chose least-recently-used eviction, so with four panes already open an automatic open closes the
least-recently-used one. Unsent composer text survives and returns when that pane is reopened.

**Needs attention:** An automatic open can close a pane you had open. Another session's work was
live in this tree, so these requirements are R61/A43 rather than R60/A42. The card-grid
implementation beside them is still uncommitted, so this design is uncommitted too.

**Next:** Run `/work` on `open-waiting-approval-panes.md` after the concurrent card-grid and
worktree work is committed.

### 2026-09-01 — Feature design: worktree projects

Designed first-class Git worktree support around the decision that fits AgentDeck's existing model:
the project is the workspace. A worktree project is an ordinary project whose working directory is
a Git worktree AgentDeck created from a chosen base branch, bootstrapped with an optional
per-project setup command, and tracks as its own. Spinning up an isolated stream of parallel work
becomes forking a repo-backed project — one action that creates the branch, the checkout, and the
project together; agents, tasks, and pipeline stages need no changes because everything in a
project already shares its directory. Checkouts are disposable: a missing one is recreated from its
branch at the next start, and deletion happens only by explicit consent when archiving or deleting
the project, with uncommitted work disclosed first. Checkouts AgentDeck did not create are never
deleted. The new FS-19 and TS-12, small planned additions to five existing specifications, and the
ready change are committed together; no product code changed. Only this design's files were
committed — the card-grid work stays uncommitted in the working tree.

**Needs attention:** Nothing new; the uncommitted card-grid tree and its failing check remain from
the prior review.

### 2026-09-01 — Review: agent cards, Mermaid diagrams, and the paused action redesign

Reviewed the stable-card work, the Mermaid display fix, the active-project fix, and the paused
internal-action redesign. The card work itself is sound: opening a pane leaves every other card in
place, Collapse and Collapse all behave as designed, long names wrap, and a save that fails still
tells you. Six problems came out of the review, three of which should be dealt with before this is
called done.

The Mermaid fix is incomplete. Scrolling no longer flips a diagram back to its source, but the same
flip still happens whenever an agent keeps writing after a diagram — which is the ordinary shape of
a diagram reply. I reproduced it against the shipped component. Separately, the card work was never
committed: the code, the specification updates, and the session notes are all sitting loose in the
working tree, where an ordinary Git operation would destroy them, and one required check fails on a
stale link that also leaves finished work advertised as still waiting to be picked up. The three
smaller items are two documentation statements that no longer match the shipped grid and a small
loss of theming information on the new compact context meter.

An unrelated, unfinished piece of design about declining pipeline proposals is loose in the same
uncommitted tree, with nothing recording it as approved work.

**Needs attention:** The card work and an unrelated pipeline-proposal design are uncommitted and one
required check fails; a diagram still flips to source while an agent is writing.

**Next:** A fix session should close the three must-fix items, then commit the card work.

### 2026-09-02 — Implementation: stable, readable agent cards

Opening a dashboard chat pane now leaves every other agent card in place. Expanded cards have a
clear Collapse button, the toolbar gains Collapse all whenever panes are open, and long agent names
wrap to three lines instead of disappearing behind an ellipsis. Context usage is removed from the
collapsed card and shown compactly in the expanded header.

The behavior and layout passed the full automated suites and were checked in both AgentDeck Core
and Sky & Grove at the desktop floor and a wide viewport.

**Needs attention:** None.

**Next:** An independent reviewer should review this implementation.

### 2026-09-01 — Fix: readable, stable Mermaid diagrams

Mermaid diagrams now stay rendered while you scroll instead of flashing back to source and
redrawing. They also use the available chat width rather than collapsing to their tiny intrinsic
size, with a height bound that keeps tall diagrams manageable and wide diagrams contained.

In the real product, the compact test diagram grew from 124 pixels to 735 pixels in the dashboard
pane and to 768 pixels in the full chat. A wide graph stayed within both surfaces without horizontal
overflow. Scrolling preserved the same rendered SVGs, and the result was checked under both
AgentDeck Core and Sky & Grove.

**Needs attention:** None.

**Next:** An independent code review can verify the fix with the rest of the current unreviewed
range.

### 2026-09-01 — Feature design: stop the card grid moving, and make cards readable

The rearranging is a real defect with one cause: an expanded pane spans two columns of an
auto-placed grid, so every card after it lands in a different cell, and TS-08.R42 accepted the
resulting wrap and gap in writing. A pane now spans one column, which makes its grid area identical
to a collapsed card's, so no card can change column or row and only the rows below the pane move.

An open card also had no collapse control at all: FS-02.R52 makes the header the target, but the
header holds only a name link that navigates instead and the state badge, so the only collapsing
pixels were the gaps between them. It gains a visible labelled control, and the toolbar gains
**Collapse all**, shown only while a pane is open and scoped to that grid's own cards so another
project's remembered panes survive. Card names drop to a smaller size and wrap onto up to three
lines instead of ending in an ellipsis, and the context meter leaves the collapsed card for the
expanded one. You declined making cards larger, so minimum height, column count, gap, and the
density control are unchanged.

One tradeoff is deliberate: collapsed cards sharing an open pane's row sit at the top of a tall row
with space below them. Filling that space needs either dense packing or fixed row heights with a
multi-row span, and both reassign cells for every card after the pane — the exact movement being
removed.

Reading the area also found TS-08.R43 claiming an expanded id leaves its `SortableContext` while
FS-02.R47 and the shipped grid keep it; R43 is corrected to the shipped truth. No product code
changed, and the unrelated pipeline-proposal design already in the tree was left untouched.

**Needs attention:** Empty space beside an open pane replaces the movement; nothing fills it. FS-02,
FS-12, and TS-08 are now Partial.

**Next:** Run `/work` on `stabilize-and-declutter-agent-cards.md`. A37 and A40 need a real-browser
check at the 1024px desktop floor in both skins.

### 2026-09-01 — Bug investigation: unstable, tiny Mermaid diagrams

Both reported Mermaid problems are confirmed. Scrolling across the transcript's bottom boundary
remounts every diagram, briefly replacing it with raw source while Mermaid renders the same SVG
again. Separately, the diagram container shrink-wraps Mermaid's percentage-width SVG: in the real
product the test graph was only 124 pixels wide inside a 761-pixel transcript.

I reproduced both on the current production UI with an isolated deterministic chat, traced them to
the original Mermaid rendering change, and committed a skipped regression that fails on the scroll
flash. No product code or specification changed, and the unrelated work already in the tree was
left untouched.

**Needs attention:** Both are normal-use display defects and should be fixed before Mermaid is
treated as a reliable transcript surface.

**Next:** Run `/fix` to stabilize the Markdown renderer identity, activate the regression, and
replace the shrink-to-fit sizing with browser-verified readable sizing for compact and wide graphs.

### 2026-09-01 — Feature design: pause the internal MCP migration

Paused the internal MCP migration and kept AgentDeck's existing internal MCP path as the released
behavior. The migration can resume only after packaged Codex/ACP supports a narrowly scoped direct
transport that works under the default sandbox and AgentDeck proves it with all four chat providers.

The docs explicitly reject broad shell networking, filesystem mailbox IPC, and a Codex-specific MCP
fallback. They also preserve the resolved review decisions for the eventual migration: generation,
hook credentials, and action credentials stay separate; `agentdeck action describe` reads the
compiled registry locally; and FS-06, FS-15, and FS-16 state exactly which MCP transport wording the
future cutover may supersede. Separately launched Codex clients remain outside this managed runtime
credential boundary. No product code changed.

**Needs attention:** The migration now depends on a future Codex/ACP transport capability and is not
ready to implement.

**Next:** When that capability exists, prove it with the packaged runtime under the default sandbox,
validate all four providers, and return the transport contract to design review before implementation.

### 2026-08-31 — Fix: the five open problems in the shipped dashboard code

I closed all five problems the last code review left open. I left the MCP-migration design findings
alone, as you asked.

Two were things you would have hit while using AgentDeck. The overflow list of active projects in
the header could not be closed with Escape once you had tabbed into it — Escape only worked while
the "+2" button itself still had focus, which is not where your focus is after you start looking
through the list. And when you drag a running agent card over the stopped ones, AgentDeck is
supposed to change the pointer to say the drop will be refused before you let go; it never did,
because the card under the pointer set its own pointer shape and won. Both now behave as intended,
and the pointer change now applies to everything inside a card, so a button added to a card later
cannot quietly reintroduce the same gap.

The other three were gaps in what the tests actually prove. The list of work waiting to start still
linked to a change file that had already been finished and deleted, so a reader would follow a dead
link into finished work presented as available. The test that claims AgentDeck leaves your
AgentDecker prompt untouched when it cannot read the file was not actually testing an unreadable
file, and it skipped itself entirely on machines running as root — it now tests a real unreadable
file and always runs. And the test that proves every way of starting an agent hands it the shared
operating knowledge only ever resumed and switched chat agents, so a terminal-only break could have
gone unnoticed; terminal agents are now covered too.

**Needs attention:** A task-cancel test failed once under load and passed everywhere else; it
expects the runtime released by the time the cancel answers, but the design leaves a failed stop to
recovery. Recorded, not fixed. The refused-drag pointer needs one real-browser check. Someone's
uncommitted pipeline-proposal design work is in the working copy, untouched.

**Next:** You still need to settle the two blocking questions on the MCP-migration design before any
of that work starts. Everything here is committed and green.

### 2026-08-31 — Design review: migrating internal actions off MCP

I reviewed the waiting design that moves AgentDeck's own agent actions off MCP onto a packaged
`agentdeck action` command, before any code is written. The direction is right and the design is
mostly tight, but two problems have to be settled first, so the change stays waiting.

The first is the one that would sink the migration. The design has each agent reach AgentDeck by
running a small command that calls the dashboard over a local network connection. Codex runs the
commands its agents issue inside a sandbox, and I confirmed on this machine that a command in that
sandbox cannot open a local network connection at all — it works outside the sandbox and fails
inside it, under the setting Codex ships as its default and that AgentDeck copies over verbatim when
it launches a Codex agent. Today's MCP setup is unaffected, because the Codex program itself makes
that call rather than a sandboxed command it spawned. Since the design deliberately allows no
per-provider fallback, this would be discovered late — after the registry, the routes, the command,
and the launch plumbing were all built — and the whole migration would have to be abandoned. It
needs a real check against all four agent programs now, and a stated fallback plan if the check
fails.

The second is a security problem the design would widen. AgentDeck currently reuses one launch
secret as an agent's "generation" marker, and that marker is stored with pipeline attempts and sent
to the dashboard over the local interface. Right now that only lets someone forge status events. The
design makes that same secret the key to every agent action, so anything that can read a pipeline
run could then send mail, create or cancel work, read shared context, and report results while
pretending to be another agent. The generation marker needs to stop being the secret, and the
acceptance check needs to look at the pipeline records and the interface responses, not just the
places it currently looks.

Two smaller items: the feature documents that own messaging, tasks, and context links still require
the MCP mechanism this change removes, with nothing recording that they are superseded, so whoever
implements the cutover will hit a contradiction with no authority to resolve it. And the command's
built-in help asks the dashboard over the network for information that is already compiled into the
same program the agent just ran, which adds a route and a failure mode for nothing.

**Needs attention:** The Codex sandbox result is a genuine feasibility risk to the whole migration,
and the shared-secret problem is a real privilege leak between agents. Both need your decision
before implementation starts.

**Next:** Take the two must-fix items back through feature design with me, then re-run this review
on the revised design.

### 2026-08-31 — Feature design: migrate internal actions from MCP

Validated: migrating AgentDeck’s internal actions away from MCP is the correct move. The precise
reason is not that MCP is obsolete—current Codex guidance supports both MCP servers and CLIs as
agent integration surfaces ([OpenAI developer docs](https://developers.openai.com/),
[Codex use cases](https://developers.openai.com/codex/use-cases))—but that AgentDeck controls both
sides of this particular boundary. Its current in-process MCP adds fifteen model-visible schemas
and provider/protocol coupling without buying interoperability. The replacement is one packaged
`agentdeck action` CLI backed by a private local Action API and the existing domain handlers.

The credentials and command contract are settled. Chat launches reuse the existing per-launch,
generation-scoped credential; it stays in the environment and never appears in command arguments.
`agentdeck action <name> --input -` reads one JSON object from stdin, emits one JSON result on
stdout, and uses stderr only for diagnostics. `agentdeck action describe <name>` provides the exact
schema on demand. Terminal agents remain excluded, while provider- and user-configured MCP support
is explicitly unchanged.

The migration is planned as a guarded cutover: freeze all fifteen current contracts, extract one
transport-neutral registry, add the private adapter and CLI, compare domain effects and results,
exercise lifecycle and adversarial cases, validate Claude, Codex, OpenCode, and OpenHands, then
delete the internal MCP route/configuration/dependency before release. There is no shipped fallback
or dual-transport mode. The UX work ensures autonomous mail, task, and pipeline activations name the
exact next action even if the optional operating skill is unavailable.

The ready change is
[`migrate-internal-actions-from-mcp.md`](/Users/mcnoam/Projects/AgentDeck/docs/ready-changes/migrate-internal-actions-from-mcp.md),
with sequencing in
[`migrate-internal-actions-from-mcp.md`](/Users/mcnoam/Projects/AgentDeck/docs/plans/migrate-internal-actions-from-mcp.md).
Specification checks, twin-skill comparison, and whitespace validation pass.

**Needs attention:** The four live-provider checks are release gates during implementation; they
are not claimed complete by this design.

**Next:** Run `/review-design` on the waiting change, then `/work` if the review finds no blocker.

### 2026-08-31 — Review: active project tabs and the preceding fixes

The reviewed range has five open problems. Two affect behavior: pressing Escape does not close the
project overflow after keyboard focus moves into its links, and a refused running-to-stopped card
drag still shows the normal pointer because the card overrides the refusal cursor. Three smaller
gaps remain: the waiting-work index links to the deleted finished change, the AgentDecker migration
test does not exercise a real file-read failure and skips too broadly under a privileged test user,
and terminal resume/switch are missing from the shared operating-knowledge lifecycle matrix.

Everything else in the range matches the specifications, including active-project membership and
ordering, current-project retention, header presentation, failed pipeline actions, layout-error
recovery, diagram sanitization, and the shared knowledge overlay. The full automated and
distributable checks pass.

**Needs attention:** Please confirm two earlier behavior choices: failed pipeline launch/resume
pauses hide **Open agent**, and a refused cross-block card drag should signal through the pointer
rather than rely on snap-back alone.

**Next:** Run `/fix`, starting with the Escape and drag-cursor defects, then close the three coverage
and state gaps.

### 2026-08-31 — Implementation: active project tabs

The header now shows compact links for projects with running agents, so switching active work no
longer requires returning to the projects home. The current project stays visible while you remain
in its dashboard or agent, project colors make links easy to recognize, and additional projects sit
under a keyboard-accessible `+n` menu. The layout was verified at the supported desktop floor and
wide desktop widths in both appearances, and the full automated and distributable checks pass.

**Needs attention:** None.

**Next:** An independent reviewer should review the active-project navigation implementation.

### 2026-08-30 — Feature design: active-project navigation tabs

Active-project navigation tabs are fully specified and waiting to start.

The shell will show smaller, slightly rounded project-colored tabs immediately after the primary
navigation. A configured project appears while it has a running agent; the current project stays
visible and selected while its project or agent route remains open, even if its last agent stops.
Projects are alphabetical, five tabs are shown directly, and the selected project displaces the
fifth when necessary while the remaining projects stay under `+n`.

The design uses the existing project query, hydrated agent state, routes, and project-color seam.
It adds no server or API change, persistence, recency tracking, layout measurement, dependency,
second header row, or product code. Core and Sky & Grove, keyboard navigation, long titles, live
hydration changes, and the 1024px desktop floor are covered by the acceptance criteria.

`make check-specs`, the twin-skill comparison, and `git diff --check` pass.

**Needs attention:** None.

**Next:** Run `/review-design active-project-navigation-tabs`, then `/work` once the design review
is clear.

### 2026-08-30 — Fix: all seven open problems closed, two of them needing your confirmation

Everything the last code review and browser run found is fixed.

The three that could bite a person during ordinary use: a diagram in a reply could still reach out to
the network by spelling its web address in a way the safety check did not recognise — diagram styles
are now decoded before they are judged and thrown away whole if anything still points outward. If the
dashboard failed to load your saved layout even once, nothing was said and nothing you rearranged was
saved for the rest of that session — the failure now tells you, and saving keeps working. And a
pipeline run paused because its stage failed to start or failed to resume no longer offers to open a
chat that cannot report back; it points you at Retry, which is the only thing that moves the run.

Two smaller ones: a failed or interrupted pipeline stage now stands out in its own colour instead of
the same grey as an ordinary state, and dragging a card across the running/stopped line now shows you
the drop will not land while you are still dragging, rather than letting the card snap back with no
explanation. Two test gaps the review flagged are also filled: every way an agent can be started or
restarted is now checked to still deliver the shared operating knowledge, and the one-time AgentDecker
prompt migration is now checked to leave the role untouched when the file cannot be read or written.

**Needs attention:** Two of these needed a product call and I took the smaller option in each, so
please confirm. A run paused by a failed stage launch or resume now hides **Open agent** — the same
rule already used for restart-paused runs — rather than keeping the chat and widening what it can do.
And a refused card drag now signals the refusal through the cursor rather than showing a written
message. The cursor treatment has only been tested in code, not in a real browser yet.

**Next:** You confirm those two choices; the browser check of the drag cursor rides along with the
next usability run.

### 2026-08-30 — Usability review: the new Pipelines pages and the changed dashboard hold up; two real problems

I drove the new pages in a real browser against a build of exactly what v0.3.0 ships: the split
Runs and Templates pages, a run's own page with its execution timeline, the focused template
editor, and the dashboard grid with its expanding chat panes and running-first card order.

Twenty-nine checks passed. That includes the three fixes made after the last review that had never
actually been looked at in a browser, and everything that was still owed on the dashboard: running
agents really do sort ahead of stopped ones, a card really does move across that line the moment you
start or stop it, dragging a card inside one block leaves the other block alone, and dragging one
across the line changes nothing. Expanding a fifth chat pane quietly closes the one you have
touched least, and the text you had half-typed in it comes back when you reopen it.

Two problems worth fixing, neither of them cosmetic. First: if the dashboard fails to load your
saved layout even once — a brief server hiccup on page load is enough — your card arrangement and
open panes silently disappear from the screen, nothing tells you anything went wrong, and from then
on nothing you do to the layout is saved for the rest of that session. Second: when a pipeline stage
fails to start or fails to resume, the run page still offers to open that stage's chat. That chat
cannot report back to the run, so anything you do there is lost work — the same dead end we removed
last week for runs paused by a restart, just reached by a different route.

Two smaller ones: failed pipeline stages render in the same neutral grey as ordinary states, so a
failure does not stand out; and when you drag a card across the running/stopped line, the card
snaps back mid-drag without saying why the move was refused.

**Needs attention:** The second problem needs a product call, and it is the same one you answered
last week: should any pause whose stage agent is gone hide the chat link, or only a restart pause?
The rule we wrote says the principle broadly and then applies it narrowly.

**Next:** Run `/fix` on the two must-fix items; I did not touch product code.

### 2026-08-30 — Release: v0.3.0 is published

AgentDeck v0.3.0 is live. The build job passed in three and a half minutes, and the download page now
carries the Mac archive, the installer script, and a manifest whose checksum matches the archive
GitHub actually stored. Installing with no version selected now gets 0.3.0.

This version splits Pipelines into separate Runs and Templates pages, lets dashboard chat panes
expand in place so several agents can be followed without leaving the grid, sorts running agents
ahead of stopped ones, renders diagrams written in chat, and gives every agent AgentDeck launches a
bundled operating guide instead of leaving it to guess how messages, tasks, pipelines, and agent
lifecycle work here. A pipeline stage agent is now told where its part of a run ends, which closes a
case where it could keep working on something the run could never accept.

Two things you should know. Pushing the release also pushed a usability-review commit another
session had left on the branch, so it is now public too — it changes no product code. And anyone who
edited their AgentDecker role keeps their old copy of the built-in product manual: AgentDeck will not
overwrite an edited role, so that stale text now sits beside the new operating guide and can
contradict it. They get the new guide either way; removing the old text from their role prompt is a
manual edit. Nothing in the product tells them this.

**Needs attention:** The Claude and Codex sign-in checks are still manual and were not part of this
release's verification, so 0.3.0 is not evidence that those provider paths work.

**Next:** Decide whether the AgentDecker upgrade note belongs in the README now rather than waiting
for someone to hit it.

### 2026-08-30 — Release: v0.3.0 is tagged and waiting for your go-ahead

AgentDeck v0.3.0 is built, verified, and tagged on this machine. It is not published: pushing is
what publishes, and I have not done that.

What people get in this version. Pipelines are now two separate surfaces, Runs and Templates,
instead of one crowded page. Chat panes on the dashboard expand in place, so you can follow several
agents at once without leaving the grid, and running agents sort ahead of stopped ones in every
section. Diagrams written in chat render as diagrams. Agents launched by AgentDeck now receive a
bundled operating guide, so they stop guessing at how messages, tasks, pipelines, and agent
lifecycle actually work here, and a stage agent is now told plainly where its part of a pipeline
run ends — previously it could keep working on something the run could never accept.

The release check found one thing out of date: the README's description of the `~/.agentdeck/`
folder never mentioned the new folder this version creates for that bundled guide. Fixed. The
bundled guide itself needed no changes, because it shipped at the end of this same range and
already matches the behavior it describes.

**Needs attention:** Publishing needs your explicit go-ahead. Pushing the branch sends 16 local
commits, not just the release, and the tag needs them because it has to point at a commit GitHub
has. Two things are worth knowing before you say yes: 20 of those commits have never been read by a
second agent — you accepted that — and the Claude and Codex sign-in checks are still manual, so
this release is not evidence they work.

**Next:** Tell me to push, and I will push `main` and `v0.3.0` and then confirm the release job
produced its archive, checksum, and manifest.

### 2026-08-30 — Workflow: a repeatable way to cut a release

AgentDeck now has a release procedure, which is what the bundled operating skill has been waiting
for. Running `/release` starts from the previous released version, lists everything that has landed
since, and refuses to go further while a must-fix problem is open or the tree is dirty. It then
reads that range specifically for changes to what an agent needs to know to operate AgentDeck, and
updates the operating guidance that ships inside the binary so a release never hands people advice
that its own version has already made wrong. The same check covers the README and the pinned
installer component versions.

The version number is proposed with a reason and confirmed by you before anything is tagged, the
full build and test set runs first, and publishing still requires your explicit go-ahead. The
credentialed Claude and Codex sign-in checks stay manual and are reported as owed rather than
quietly counted as passed. The final word of a release session is readable release notes, not a
requirement list.

**Needs attention:** None.

**Next:** Someone should run `/release` for the 55 commits since v0.2.3 — that range includes the
new operating skill itself, so it is the first real test of the procedure.

### 2026-08-29 — Feature design follow-up: shared AgentDeck skill

The shared-skill design is ready to implement. Skill installation must now succeed before AgentDeck
migrates the old AgentDecker prompt, so a failed installation leaves the existing product knowledge
intact for a later retry. The new thin prompt also refers to current operating guidance without
claiming that the bundled skill is available when it is not.

The other two review items are retained as alignment cleanups: skill directories and prompt text
must stay runtime-only rather than entering saved session configuration, and fresh PM and teammate
prompts will stop repeating coordination mechanics and the numeric message budget. Existing
user-owned PM and teammate role files will not be migrated. The irrelevant invariant citation and
impossible comparison-error test were removed.

**Needs attention:** None.

**Next:** Run `/work agentdeck-shared-skill` to implement the waiting change.

### 2026-08-29 — Design review: shared AgentDeck skill

The design is not ready to implement. Its delivery overlay is described as process-local, but the
existing fields it plans to use are written into the durable session snapshot; after a later failed
installation, a resumed agent could still receive the stale skill directory and prompt pointer. The
thin AgentDecker prompt also tells the role to use the bundled skill even when installation failed,
and the proposed migration can remove its old product manual during that same failed startup.

The PM and teammate roles are a third maintained source of coordination behavior: they still embed
tool names, wake rules, recipient addressing, and the numeric message budget that the new
coordination reference is supposed to own. That would leave ordinary roles following stale guidance
after a future release. The native-discovery assumption itself was checked against the pinned Claude
and Codex adapters and is supported.

**Needs attention:** Revise the transient delivery seam, install-failure/migration ordering, and
ownership or migration of the PM and teammate coordination prompts.

**Next:** Run `/design-feature agentdeck-shared-skill` to resolve the three findings, then repeat the
design review.

### 2026-08-29 — Feature design revision: operator-skill boundaries and safe degradation

FS-18 and TS-11 now keep the shared skill strictly about operating AgentDeck. The core contains
only cross-capability choices and authority/result mental models; messaging budgets moved to the
coordination reference, while blocked attempts, Continue, and proposal behavior moved to the
pipeline reference. The shipped package now has exactly three references, with release-maintenance
knowledge excluded for a future repository release workflow.

Skill installation remains atomic, owner-only, and verified, but it is no longer an AgentDeck
availability dependency. A failed installation logs a clear warning, starts AgentDeck normally, and
suppresses the skill directory, environment variable, and prompt pointer for that dashboard process.

**Needs attention:** None.

**Next:** Run `/review-design` on `agentdeck-shared-skill`, then `/work` after the design review is
clear.

### 2026-08-29 — fix: Everything on the open list is now closed

Every problem on the list is fixed — the twelve from the last code review, the three from the
pipeline bug you reported, and the seven left over from the review before that.

The pipeline one needed a product decision, and I took the smaller of the two options rather than
leave everything else waiting on it: say plainly where a stage agent's part ends, instead of
building a way to continue a paused stage from its own chat. So the instructions it gets, the
acknowledgement when it reports, and the refusal if it reports again all now say that answering in
its chat during a pause is out of band and that your answer arrives as a fresh assignment. In the
same spirit, the run page stops offering **Open agent** after a restart, where following it led
nowhere — it now says the agent can no longer report and that Retry is the way forward. And every
refusal is written to the server log with enough detail to diagnose the next report like yours,
which was impossible before: the only record lived inside the agent's own chat.

Two things were quietly making the dashboard slower rather than faster. Opening a new tab restarted
the shared live connection for every tab already open, so each one reloaded everything and briefly
lost updates — on exactly the many-tab workload the shared connection exists to speed up. New tabs
are now caught up on their own without disturbing anyone else. Separately, closed tabs were never
released, so memory grew all day on a dashboard left open.

Two places were building the same thing twice and disagreeing. A card showed the end of an agent's
last message while it streamed and the beginning of it after a reload; both now show the end, and an
emoji at the cut no longer turns into a box. And the Tasks list was deciding for itself whether
Retry would work — it had already got that wrong once and hidden the button on work that Retry was
the only repair for — so the server now answers that question and the list just reads the answer.

The rest are smaller: keyboard jumping between chat panes works again once cards are in groups;
dragging a card near an expanded pane or near the running/stopped divide no longer makes the wrong
neighbours jump; a diagram that fails to draw stops leaving debris in the page; and several pipeline
screens that had no tests at all now have them.

**Needs attention:** Confirm the pipeline decision above. Saying where the boundary is costs
nothing and can stay either way, but if you would rather a paused stage could be continued from its
own chat, that is a design change and I have not started it. Two checks also need a real browser and
I had none: the six-tab dashboard test, and watching the drag behaviour around the running/stopped
divide — both are written down as owed. One loose end that is not mine: an edit to the agent
workflow document has been sitting uncommitted in the working tree since an earlier session, and I
have left it alone.

**Next:** Run `/review` over this session's commits, then the two browser checks when you have a
machine in front of you.

### 2026-08-29 — Feature design: thin AgentDecker and shared AgentDeck skill

AgentDecker is now specified as a thin resident operator role, with reusable AgentDeck expertise
moving into a product-owned `operating-agentdeck` skill available to every launched role. FS-18
defines the role/skill/tool/reference boundary, progressive disclosure, exact-only legacy prompt
migration, and requested-only orchestration. TS-11 defines one embedded package, owner-only managed
provider views, one launch/resume/switch delivery seam, direct-path fallback, and a release-
maintenance rubric distinguishing tool, core-skill, reference, and no-documentation changes. The
change is specified and waiting to start; no product code changed.

**Needs attention:** None.

**Next:** Run `/review-design` on `agentdeck-shared-skill`, then `/work` after the design review is
clear.

### 2026-08-29 — review: Everything built since the last review

I read every code change since the last review in one pass, including three that had only ever had design and usability reviews before — the diagrams in chat, the Pipelines split into Runs and Templates, and the new expandable chat panes that landed while I was working. Fifteen problems are written up; three matter now.

The first is a privacy one. Diagrams are drawn from text the model writes, and we strip out anything that could make the page fetch from the internet — except we missed one place. A diagram can still smuggle a web address into a styling instruction, and the browser will fetch it, which quietly tells whoever owns that address that you opened the diagram. The second: if the request for your pipeline templates fails for a moment, the template page tells you the template was deleted and offers no way to retry. The run page next to it already handles this correctly, so the two drifted apart. The third is about the new chat panes — they keep a full second copy of every message that arrives and never let it go, and the copying gets slower as a conversation grows. That lands squarely on what the panes are for: four agents streaming at once on a dashboard left open all day, which is the same surface that froze on us before.

The rest are smaller: keyboard jumping between panes stops working once your cards are split into groups, dragging a card next to an expanded pane makes its neighbours jump to the wrong places, and some of the pipeline screens have no test behind them at all. All the tests and checks pass, so none of this is currently visible as a failure.

**Needs attention:** The diagram fetch problem is the one I would fix first — it is a small change and it is the only finding with a privacy consequence.

**Next:** Run `/fix` on the three main findings, starting with the diagram one.

### 2026-08-29 — implementation: Expandable dashboard chat panes

Chat cards on a project dashboard now expand in place, letting you read, reply, resolve permissions, and supervise up to four agents without leaving the grid. Open panes persist across reloads, cycle by keyboard, keep drafts safe, and recover concurrent live transcripts without stale responses erasing newer events. The final interaction and layout were verified in a real browser under both AgentDeck Core and Sky & Grove.

**Needs attention:** None.

**Next:** Run an independent review of the expandable-pane implementation.

### 2026-08-29 — Design review correction: project dashboard scope

The projects-home finding was based on a terminology misunderstanding and has been removed.
“Projects page” means the agent-card dashboard reached after selecting a project, not the root page
that lists project cards. That existing dashboard is the intended and buildable host for expandable
chat panes.

The wording that suggested two separate grid surfaces is now recorded as a consistency note for the
next design pass. Two blocking findings remain: pane controls need an interaction boundary so card
click handling does not collapse the pane, and transcript recovery needs ordering protection so an
older refetch cannot replace newer streamed events. The three lower-severity coverage gaps remain.

**Needs attention:** No additional product decision is needed for project scope.

**Next:** Run `/design-feature expandable-chat-panes-on-the-dashboard` to resolve the remaining
findings and wording notes, then repeat the design review before implementation.

### 2026-08-29 — Design review: expandable dashboard chat panes

The expandable-chat-pane design is not ready to build yet. The review found three blocking defects.
The projects home contains project cards, not agent cards, so one of the two promised expansion
surfaces does not exist. The current agent card also owns click and right-click handling at its outer
edge; without an explicit interaction boundary, using Send, permission controls, autocomplete,
links, transcript disclosures, or annotations inside a pane can collapse it or open the card menu.
Finally, a transcript refetch can currently replace newer streamed events that arrive while the
request is in flight, so reconnect recovery can make live text or a resolved permission disappear.

Three lower-severity gaps also need tightening: real-browser evidence must cover pane height and
scroll isolation, the exact events that make a pane “recently focused” need definition, and stopping
an agent must have an acceptance case proving its open pane and transcript remain. The incumbent
Core surfaces were inspected in a real browser; the review changed no product code, specifications,
or change file.

**Needs attention:** Choose whether expansion is scoped to project dashboards or whether the
projects home should gain a new agent-card surface; the latter is a materially larger product change.

**Next:** Run `/design-feature expandable-chat-panes-on-the-dashboard` to resolve the findings,
then repeat the design review before implementation.

### 2026-08-29 — Design review resolved; the pane change is ready to build

I did not push back on any finding. All five, and all three consistency notes, held up when I checked
them against the actual code, and each one named something an operator would really hit. Two of them
changed the design rather than its wording.

The first is the one that would have hurt most. An agent card owns the click and right-click handlers
on its outer element, and the chat pane sits inside that element — so clicking Send, deciding a
permission request, or picking an autocomplete entry would have collapsed the pane out from under
you, or opened the card menu. Fixed: once a card is expanded, only its header row responds to clicks
and right-clicks. The pane's body is not an activation target at all.

Here I did diverge from the review on *how* to fix it. The obvious repair is to have each control in
the pane stop the click from bubbling. I rejected that shape: it is a list of exemptions, so every
control anyone adds to the pane later has to remember to join the list, and forgetting fails silently
in a way no test would catch. One structural boundary is a rule; an opt-out list is a future bug.

The second is that the transcript refetch can lose messages. When a pane reloads its history, the
response replaces everything — so if new text streams in while that request is in flight, the reload
can erase it or turn an answered permission prompt back into an unanswered one. This already exists
today and the chat specification already records it as a known gap, but four panes make it four times
as likely. I scoped the fix to exactly the two repairs that recorded gap already named, rather than
building a general merge: ignore a response that has been overtaken, and keep any messages newer than
the ones the response carried. The agent page gets the same fix for free, so a long-standing known
gap closes with this work rather than being multiplied by it.

The other three were about evidence and definitions, and were cheap: the pane's height and scrolling
now have to be checked in a real browser, because the test environment cannot see layout at all and
would pass a pane that stretched its neighbours; "least recently used" now names the three things
that count as using a pane, including simply pressing the mouse in it, because reading a transcript
does not move focus and you could have lost the pane you were reading; and an agent that stops while
you are reading it now has to keep its pane, which nothing previously tested.

I also fixed a worse defect the review did not catch, and it came out of its own first note. I had
written that a pane belonging to a different project gets dropped when the arrangement is saved. But
the agent grid only ever exists inside a project, so *every* saved pane belongs to "a different
project" the moment you open another one — opening a second project would have quietly wiped the
first one's panes. Panes for other projects are now kept and simply not drawn.

That same correction settles the review's first note: I told you earlier that the card grid appears
on both the projects home and inside a project. That was wrong. The projects home shows project
cards; the agent grid exists only inside a project, which is what you meant by "the project page"
anyway. The specifications and the change file now say so.

No product code changed. The change is ready for `/work`.

**Needs attention:** Nothing from the review is open. The two reversible choices I flagged before
still stand — the `Ctrl+Alt+↑/↓` binding, and panes always spanning exactly two columns even at the
densest grid settings.

**Next:** Run `/work` on `docs/ready-changes/expandable-chat-panes-on-the-dashboard.md` when you want
it built.

### 2026-08-29 — Expandable chat panes on the dashboard are specified and ready to build

Clicking an agent card will expand it in place into a chat pane instead of navigating away. The pane
spans two grid columns, its transcript scrolls inside itself, and up to four can be open at once, so
you can read one agent, answer a blocked one, and still see the rest of the grid's state. Opening a
fifth silently collapses the pane you touched least recently — nothing is lost, because unsent
composer text is already kept per agent by the browser. `Ctrl+Alt+↓` and `Ctrl+Alt+↑` move focus
between open composers. The set of open panes is saved in `layout.json` beside card order and
density, so a reload or a server restart brings your arrangement back.

A pane carries the agent's name, live state, transcript, and composer — everything needed to read a
turn, decide a permission request, and reply. It deliberately does not carry the Files, Commands, or
Terminal tabs, the context meter, or the runtime picker. Those stay on the full agent page, which is
completely unchanged and is still reachable from the pane's name. Terminal agents keep opening that
page directly, since a pane with no terminal would be useless to them.

Two things in the shipped code changed the design. Cards live in an equal-width grid, so a card that
simply expanded in place would have been *one column* wide — narrower than the chat page it was
meant to improve on. That is why the pane spans two columns and why the transcript, not the page,
does the scrolling: otherwise one agent's streaming output would shove the page under you while you
read another. Separately, the browser's event client tracks a single "open agent" and appends live
messages only for that one, so several live chats at once is exactly what it cannot do today. The
technical spec turns that into a small registered set that survives reconnects and refetches at most
four transcripts, which keeps it inside the connection limits that caused an earlier load failure.

Four calls were yours: click expands rather than a separate chevron; four panes; saved in
`layout.json`; and that keyboard shortcut. Two exclusions you accepted: nothing ever auto-expands
itself, so the grid never rearranges while you are reading it, and expansion works on the projects
home as well as inside a project.

No product code changed. The specifications, the ready-change file, and the handoff are updated, and
the documentation checks are green. One unrelated edit appeared in the tree: a Go toolchain run
during this session moved `coder/websocket` and `creack/pty` out of the indirect block in `go.mod`.
Both are imported directly by the terminal and server packages, so the markers were stale and the
correction is right; I left it rather than reverting something the next build would redo.

**Needs attention:** Two choices I made that are easy to reverse before anyone builds this — the
`Ctrl+Alt+↑/↓` binding, and the decision that a pane always spans exactly two columns even at the
densest grid settings, where two columns is a narrow chat.

**Next:** The change is waiting in `docs/ready-changes/`; run `/work` on it when you want it built.

### 2026-08-28 — Workflow correction: `/ux` now optimizes for expert operators

`/ux` no longer runs a full process for every meaningful user-facing design. It first makes a cheap
trigger decision and continues with an ordinary feature workflow unless the work changes an
established task or introduces a consequential decision, ambiguous state, recovery path,
long-running operation, or AI uncertainty. A skipped pass creates no rationale or UX artifact.

The default user is now AgentDeck's actual experienced internal operator. The workflow optimizes
repeated work for speed, density, keyboard flow, predictability, learned shortcuts, and control. It
frames one primary task by default and adds another only for a materially different high-risk
branch. Onboarding and discoverability are considered when they are genuinely in scope or when a
rare consequential action must be rediscovered—not to simplify the whole product for hypothetical
new users.

Real-browser UX validation is also conditional now. It runs when rendered interaction, timing or
state transitions, recovery behavior, or an unresolved design risk could change the answer, and for
an explicit standalone `/ux` critique. Behavior already established by acceptance tests does not
pay for the browser harness merely because `/ux` was considered.

**Needs attention:** None.

**Next:** Use the lightweight trigger on the next feature design; most ordinary or familiar
additions should proceed without a full `/ux` pass.

### 2026-08-28 — Workflow: first-party UX judgment before behavior hardens

AgentDeck now has a `/ux` workflow for the experience of accomplishing a task, not just whether the
feature works. It joins feature design automatically whenever meaningful user-facing behavior is
being defined, before the interaction contract hardens. It frames the few tasks that matter, walks
them from what a person actually knows at each step, and turns hidden prerequisites, unclear state,
unsafe consequences, and recovery dead ends into concrete feature behavior and acceptance evidence.

After implementation it can drive the focused task in the real product, using AgentDeck's isolated
browser fixtures when needed. A finding must name the task, observed friction, consequence, and a
proportionate improvement; source inspection alone is only a risk lead. Standalone `/ux` critique
is read-only. `/design` still owns composition, visual hierarchy, motion, and polish, while the two
share one task frame and browser pass when visual treatment affects understanding. The broader
`/usability-review` remains the product-wide acceptance regression.

The guidance distills
[cognitive walkthroughs](https://www.nngroup.com/articles/cognitive-walkthroughs/),
[NN/g's usability heuristics](https://www.nngroup.com/articles/ten-usability-heuristics/),
[Microsoft HAX](https://www.microsoft.com/en-us/haxtoolkit/ai-guidelines/),
[CLI Guidelines](https://clig.dev/), and
[Good Services](https://good.services/15-principles-of-good-service-design) without reproducing
their checklists. PAIR contributed only the useful caution against implying human understanding;
HAX already covers the operational AI controls. Forward tests on Pipelines, Mermaid diagrams, and
running-first cards caught the existing blocked-run continuation failure, avoided a false Mermaid
finding, and identified drag-refusal feedback as a browser-dependent risk rather than invented
evidence. An internal connection fallback correctly stayed outside `/ux`.

**Needs attention:** The real-browser check already owed for running-first card placement should
observe whether a refused cross-boundary drop feels intentionally constrained or simply broken;
that static risk is not a finding yet.

**Next:** Let `/ux` accompany the next user-facing `/design-feature`, and include that drag-feedback
observation when someone completes the owed dashboard browser check.

### 2026-08-28 — Implementation: running agents now sit at the top of a project's grid

Opening a project dashboard starts with live work. Inside each group on the grid, the agents that
are actually running are drawn first and the stopped ones follow, so you no longer hunt for a
running agent among cards sitting wherever you last dragged them.

Your manual order is untouched. It still decides the sequence within the running block and within
the stopped block, and it is still the one order saved for every project view — nothing new is
stored and no new setting appeared. Starting or stopping an agent moves just that card across the
line straight away, with no reload. A card that is waiting for input or has errored still stands
out more without moving, because only running or stopped changes a card's place.

Dragging works as before inside a block, and the saved order after a drag is exactly what it would
have been before this change. Dragging a card onto the other side of the running/stopped line does
nothing and saves nothing, since a manual drag cannot put a stopped agent above a running one.

**Needs attention:** This is verified by automated tests, not by a person in a real browser. The
live start/stop movement, how the other block behaves mid-drag, and the refused cross-boundary drop
should be watched once on screen. No browser was available in this session.

**Next:** Someone should open a project with a mix of running and stopped agents, stop one, drag
another, and confirm it feels right; then the outstanding fix and review work can continue.

### 2026-08-28 — Workflow correction: rendered iteration stops on quality, not pass count

The `/design` workflow no longer imposes exactly one repair pass and one confirmation. That rule
could force an agent to stop after confirming obvious regressions introduced by its own repair.

Agents still batch rendered findings into a coherent repair pass and render again. If material
in-scope problems remain, they repair and re-render them; otherwise they stop. The guard against
runaway pixel polishing is now outcome-based: iteration ends once the chosen direction and material
findings are satisfied, and does not continue for subjective polish alone.

The small skill and canonical workflow now use the same rule.

**Needs attention:** The commits remain local. Pushing to `github.com/AsaphNoam/AgentDeck` requires
your explicit approval of that destination.

**Next:** Explicitly approve that GitHub destination if you want both commits pushed.

### 2026-08-28 — Implementation: a first-party UI design workflow

AgentDeck now has a `/design` workflow that activates automatically only when UI work genuinely
depends on design judgment: new screens, redesigns, meaningful layout or styling, motion, visual
polish, and critique. Routine frontend wiring and style-preserving fixes stay out of it.

The workflow distills the strongest ideas from
[Impeccable](https://github.com/pbakaus/impeccable),
[Emil Kowalski's skills](https://github.com/emilkowalski/skills),
[Anthropic's frontend-design skill](https://github.com/anthropics/skills/tree/main/skills/frontend-design),
and [designer-skills' design review](https://github.com/julianoczkowski/designer-skills/tree/main/design-review).
Agents now establish a compact, product-specific direction before material visual work, compose
hierarchy before decorating, avoid category-default “AI UI,” make motion justify itself through
feedback or lifecycle clarity, and finish by inspecting the actual running interface. Major visual
work gets one batched repair pass and one confirmation instead of an open-ended polish loop.

This builds on AgentDeck's existing tokens, primitives, presentation contract, Core and Sky & Grove
appearances, deterministic visual matrix, and browser checks. No skill pack was installed or
vendored, and no design framework, detector suite, screenshot baseline, or runtime dependency was
added. Standalone critique remains read-only, and automatic invocation never authorizes or enlarges
the underlying task.

**Needs attention:** None.

**Next:** Use `/design` directly for a visual critique or design task; it should also join the next
material UI change automatically.

### 2026-08-28 — Fix: the Tasks page offers Retry again where Retry is the repair

A task can end up parked for two quite different reasons, and the fix that shipped last week treated
them as one. If a task is waiting on something that can never happen, retrying it is pointless and
re-arming it is the repair — that part was right, and the Retry button was correctly removed.
But a task can also park simply because it failed to start three times, say because a provider was
briefly unreachable, or because the agent it was meant to run in was deleted. For those, Retry *is*
the repair: it hands the task a fresh set of attempts and puts it back in the queue. Hiding the
button removed the only sensible route back.

The page now decides the same way the server does: Retry appears for interrupted work, and for
parked work unless something it depends on has become impossible. In the second case you still get
Re-arm and no Retry, so you are never offered a button that would only come back with an error.

I also recorded this in the acceptance criteria. The rule that parked work is repairable was checked
only on the server side; nothing verified that the page offers the right repair for each reason,
which is exactly how the two got collapsed. There is now a test with both kinds of parked task in
one list.

**Needs attention:** None.

**Next:** Five smaller items remain from the review, plus one I found while verifying this batch.
Say the word for the next one.

### 2026-08-28 — Fix: the dashboard now recovers when the shared connection cannot run

I fixed the one item I flagged as must-fix. The change that made browser tabs share a single live
connection had no way to fail safely: it checked whether your browser *has* the shared-worker
feature, but never whether it actually works here. If it existed but refused to start, the error
escaped before the connection watchdog was even switched on, and the dashboard sat on "connecting"
with no live updates, no message, and nothing that would ever try again — a reload failed the same
way. If the worker's script failed to load instead, nothing was listening for that either, so the
watchdog just kept reconnecting into the same dead worker every twenty-five seconds.

There was already a perfectly good fallback to the old one-connection-per-tab behaviour sitting in
the same file; nothing could reach it. Now all three failure routes reach it — the feature refusing
to start, its script failing to load, and a connection that never opens within the liveness window.
That tab quietly drops back to its own direct connection for the rest of the session and keeps
working. It gives up the connection-pool benefit, which is the right trade against showing nothing.

I also wrote the fallback into the connection specification, which had promised sharing
unconditionally, and added tests for the shared connection itself. That matters more than it sounds:
the test environment does not provide the shared-worker feature, so every existing connection test
had been silently exercising the old path. There are now four covering the real one.

Two pieces of bookkeeping came out of verifying it. The six-tab browser check that would have caught
this is still owed — it was written into the specification as evidence but never run — and I noticed
that two generated files are committed inside a directory the repository otherwise ignores, one of
which points at a file that is not in the repository at all.

**Needs attention:** None.

**Next:** Seven smaller items remain from this review; the Tasks Retry button is the one you are
most likely to notice. Say the word and I will take them one at a time as before.

### 2026-08-28 — Review: the dashboard and SSE fixes shipped on 27 August

I read the shipped code for the batch of thirteen dashboard, connection, and small usability fixes.
The big ones are genuinely done: browser tabs now share a single live connection instead of each
holding one open, so the freeze where a sixth tab could never load is fixed at the root; the
background sweep that used to re-read every stored conversation on every write now only re-reads the
one that changed; and busy agents no longer make the whole card grid recompute. Nine problems came
out of the read, three of them worth fixing before anything else.

The one worth fixing before anything else: the new shared-connection mechanism has no failure path. It has a perfectly good fallback to
the old direct connection sitting right beside it, but nothing can ever reach it — if the shared
worker fails to start or fails to load in a given browser, the dashboard sits on "connecting"
forever with no live updates and no message saying why. It also has no test at all, and because the
test environment does not provide the browser feature it uses, every existing connection test is
still exercising the old path. The six-tab browser check that would have caught this was written
into the specification as evidence but never actually run after the fix.

Next in weight: on the Tasks page, the **Retry** button was removed from every parked task. That was
right for one kind of parked task — one waiting on something that can never happen — but it also
removed it from the other kind, a task that simply failed to start three times because, say, a
provider was briefly unreachable. Restarting that is exactly what Retry is for. The work is not
lost: submitting the **Re-arm** form with both boxes empty does put the task back in the queue. But
nothing tells you that, the form is labelled and worded for repairing prerequisites, doing it wipes
the prerequisites you recorded, and the task keeps its spent attempts, so the next failed start
parks it again immediately.

The remaining five are smaller: the shared connection restarts for everyone each time a tab opens
(so opening six tabs costs far more work than it should), the worker never forgets closed tabs, card
previews now show the end of a message live but the beginning after a reload, one new test does not
check what its name says it checks, and an agent-assigned task row starts with a stray "·".

You judged the settings-refresh finding a non-issue and I have removed it — the change that stopped
the constant refreshing also means an open Settings → Sources panel can keep showing a superseded
view until you reload, and that is the accepted trade.

**Needs attention:** None.

**Next:** Run `/fix` on the shared-connection failure path, then the seven smaller items and the
four open pipeline findings from yesterday's investigation. The Mermaid diagram work and the
Pipelines split still have no code review.

### 2026-08-28 — Bug investigation: the blocked pipeline stage that could not report back

I found it, and I could reproduce it. When a stage agent says it is blocked and needs an answer from
you, AgentDeck pauses the run but deliberately leaves that agent alive and idle so it can carry on
later — and the run screen offers **Open agent** right next to the **Continue** box. If you answer
the question in the agent's own chat, which is the natural thing to do because the question is
sitting right there, the agent does the work and then tries to file its result, and AgentDeck refuses
it with "caller is not the current stage attempt". That sentence is simply untrue: it is the current
stage agent, and AgentDeck's own records say so. The real reason is that the run already recorded
this agent's "blocked" answer and is waiting for you to press **Continue**. Worse, the refusal is
tagged as one the agent must never retry, so it gives up and a full turn of its work is thrown away.
Pressing **Continue** genuinely works; it is only the chat route that dead-ends.

Three smaller things came out of it. Nothing ever tells the agent that answering in chat is out of
band, so it cannot avoid the trap or explain it to you. Nothing about this refusal is written to the
server log at all, which is why your report could not be checked against any log on this machine —
the only record was inside the agent's own transcript. And there is a narrow timing hole where a run
can quietly stop advancing with no error and no notification, which I could not reproduce and have
recorded as unconfirmed.

I changed no product behavior. The only thing I added to the code is a test that reproduces the
failure, committed switched off so the repository stays green until it is fixed.

**Needs attention:** One product decision is yours. Right now the boundary is that a paused agent's
chat is not part of the pipeline, and the fix on that basis is to say so honestly — in the agent's
instructions and in the refusal — so the work is never wasted. The alternative is to make answering
in chat actually continue the run. The first is a small fix; the second is a real feature. Tell me
which and I will fix accordingly.

**Next:** I fix the wrong refusal, the missing log line and the timing hole, once you have told me
which side of that boundary you want.

### 2026-08-28 — Housekeeping: correcting what has actually been reviewed

The handoff's "last reviewed code" marker was stale. It pointed at a commit from before the last
review pass, because that pass verified the fixes and then forgot to move the marker. Anything
reading the file cold — including me, earlier today — would conclude that twenty-plus commits had
never been looked at, when most of them are review sessions themselves.

The marker now points at the last commit that was genuinely read, and the handoff names the four
commits that really have had no code review of what shipped: the thirteen dashboard fixes, the
Mermaid diagram rendering, and the Pipelines split with its follow-up fixes. Each of those had a
design review before it was built and a usability pass after, which is useful but is not the same as
someone reading the code against the specification.

**Needs attention:** None.

**Next:** Those four commits are the review backlog. Note that a separate session has since filed
four bug findings on the pipeline `stale_assignment` report, and one of them needs a product
decision from you before it can close.

### 2026-08-28 — Housekeeping: the idea list and the Pipelines paperwork

I removed the two ideas you said were already done — the projects-page rework and the three items
from the August 10 play session — and tidied up after the Pipelines redesign. Its change file was
still sitting in the queue marked finished while also being listed as work waiting to start, which
made shipped work look unstarted; the file is gone and the specification is now the only place that
describes what the surface does.

I checked the other two against the code before deleting them, and they are not done. Dragging a
card still only works from the small handle in its corner, not the card itself, and there is still
no Content-Security-Policy anywhere in the server or the page. You chose to keep both, so both are
still on the list.

**Needs attention:** None.

**Next:** Run the independent code review of the Pipelines redesign, or pick the next idea to design.

### 2026-08-28 — Fix: nothing left to fix

There are no open problems on the list. Everything found by the last code review and the last
usability pass is already closed, and the working copy is clean, so I changed no code, no tests, and
no specifications this session.

**Needs attention:** None.

**Next:** Run the independent code review of the Pipelines redesign and its fixes, which is the work
that is actually waiting.

### 2026-08-28 — Fix: the six Pipelines review findings

All six problems the usability review found on the redesigned Pipelines surface are fixed, and the
redesign itself, the review notes, and these fixes are now committed rather than sitting loose in
the working tree.

The numbered badge on each attempt now sits on the timeline line instead of half off the left edge
of the window, and in a narrow desktop window the attempts come first again instead of being pushed
a full screen down by the run's setup details. The Start button no longer goes grey in silence: when
a required field is still empty the dialog names it and marks the field, and on a setup with no
usable template yet, Start run points at Templates instead of opening a dialog that can never
proceed. A new attempt arriving on a live run now gets the brief fade the design called for, played
only for the entry that actually just arrived and never for the ones already on screen. A deleted
run's page explains itself in ordinary language; a run that fails to load for any other reason now
says that instead, and still shows the underlying error rather than hiding it.

On the one finding the review left open as a judgement call, I added the fade rather than trimming
the promise out of the specification: it is a few lines of code and it is what the design says the
surface does. The specification now also records the two new "say what is missing" behaviors, which
no requirement covered before.

**Needs attention:** The three visual fixes — the badge, the narrow-window order, and the new-attempt
fade — are backed by tests and by the stylesheets, not by a browser run. The next time someone drives
this surface in a real browser, those are the three to look at.

**Next:** Run the independent code review of the Pipelines redesign, now that it and its fixes are
committed.

### 2026-08-28 — Usability review: the redesigned Pipelines surface

I drove the new Pipelines surface through a real browser against a release build of the current
working tree, on three separate throwaway setups: one with nothing in it, one where I started a run
from scratch through the dialog, and one loaded up with a run that looped through a repair cycle
seven times, runs paused for approval, for a blocked stage and for a crashed agent, 121 stored runs,
26 pieces of delegated work under a single attempt, a helper whose agent record no longer exists,
and a 32-stage template.

It holds up well. Nothing is broken, no journey dead-ends, and there was not a single error in the
browser console anywhere on the surface. Landing on Runs, opening a run from its own link, reading a
repair loop as separate attempts in the order they happened, opening the agents that did the work
(live ones to their conversation, finished ones to their transcript), paging through the whole
history to a definite end, editing one stage of a 32-stage template without losing unsaved changes,
old links still resolving, deleted things explaining themselves, and the reduced-motion version of
the whole flow all behaved exactly as intended, in both appearances.

Six things are worth fixing, none of them blocking. Two are layout: the small numbered badge on each
attempt is positioned wrong and ends up half off the left edge of the window on every run page, and
on a narrow desktop window the run's setup details get pushed above the timeline, so the attempts you
opened the page to read start a full screen down. Two are the same missing explanation: when a
template needs a named input you haven't filled, and when you have no templates at all, the Start
button in the dialog simply goes grey without saying what is missing. One is a stray technical error
string on a deleted run's page. The last is a small promise in the specification—a gentle fade when a
new attempt appears—that the code does not implement; I could not trigger a live attempt with the
stand-in agent, so that one may be better fixed by trimming the promise than by adding the animation.
Full detail is in the run report at `docs/archive/reviews/usability-review-run-2026-08-28-pipelines.md`.

**Needs attention:** The whole redesign—code, specifications and notes—is still sitting uncommitted
in the working tree. I did not commit my review notes on their own, because that would have put a
"shipped the redesign" entry into the history without the code it describes. Committing the redesign
is your call, and my notes should go in with it or right after.

**Next:** Commit the staged redesign, then hand the six findings to a fix pass.

### 2026-08-28 — Implementation: Pipelines redesign

Pipelines is now two focused experiences instead of one dense page. Runs opens as a compact
operational ledger with complete retained history, human stage names, and a start dialog that keeps
setup, runtime choices, and review separate. Every run has a reloadable page with its live state and
controls first, a true execution timeline—including repeated repair attempts—plus frozen setup,
named values, stage agents, and one-hop delegated work. Agent cards honestly distinguish live,
archived, and unavailable work, and disclose delegated agents still running after a stage finishes.

Templates now has its own library and full-width editor. The editor keeps one selected stage in
focus, preserves unsaved work while moving between stages, points validation back to the affected
field, and scales to the maximum stage count. AgentDecker proposals live where they can be acted on,
legacy run links still resolve, missing records explain themselves, background refreshes keep
content in place, and reduced-motion users get the same complete flow without decorative movement.

The redesign was exercised in a real browser at the supported desktop floor across empty and
populated Runs, the full three-step start flow, looping run history, delegated-agent disclosure, the
template library, and focused editing. The layout defect found during that pass—runtime rows hiding
the dialog footer—was fixed and rechecked.

**Needs attention:** None.

**Next:** Run an independent review of the redesigned Pipelines surface.

### 2026-08-27 — Fix: active-first Ordering design

The Ordering design now uses the same running-first card sequence for what the person sees and what
the drag library measures. A drag within the running or stopped block can reorder that block without
making cards from the other block jump into the wrong places, while the saved layout remains the
same flat manual order used today. Dropping across the running/stopped boundary does nothing and
writes nothing, so the forced active-first split cannot create a hidden reorder.

The broader whole-card and cross-group drag problems remain separate work. The design evidence also
now names the `unknown` status fallback instead of miscounting the status vocabulary. No product code
changed.

**Needs attention:** None.

**Next:** Run `/review-design Ordering` once more; if it closes cleanly, implement it with
`/work Ordering`.

### 2026-08-27 — Implementation: diagrams in the chat window

Agents can now answer with a diagram instead of diagram source. When a reply contains a fenced
`mermaid` block, the chat shows the drawn diagram once the block is complete; while the reply is
still streaming it stays a plain code block, so nothing flickers or shows a half-drawn error. Each
diagram has a **Show source** control that flips back to the original text, and a diagram that is
too large, unparseable, or asks to load an external image quietly stays a code block with a short
note rather than breaking the message. Diagrams pick up the app's colours, including the Sky & Grove
appearance, and re-draw immediately if you change appearance while reading.

Because agent replies can repeat text an agent read from a repository or the web, diagram source is
treated as untrusted: the drawing library runs with its interactive features off, the drawn result is
cleaned before it reaches the page, and it can make no network request. A diagram carrying a script
and an HTML label was driven through a real browser and reached the page as harmless text. The
drawing library only downloads when a diagram is actually shown, so the app starts as fast as before.

**Needs attention:** None.

**Next:** Two designed changes are still waiting: ordering running agents ahead of stopped ones,
which has an open drag-behaviour question from its design review, and splitting the Pipelines
surface. Pick whichever you want built next.

### 2026-08-27 — Design review: active-first Ordering

The running-first split is the right small extension: it can reuse the existing project-grid,
grouping, live-update, and saved-layout paths without adding an API, preference, or parallel ordering
system. One drag interaction needs design work before implementation. The drag library currently
indexes cards in saved manual order, but the new grid would render them in running-first order. In a
mixed group, dragging between running cards can therefore animate stopped cards into the wrong places
or make the drop target unstable even when the eventual saved order is correct.

The ready-change evidence also calls agent state a five-value enum even though the shipped fallback
adds an `unknown` value; that is a documentation-only consistency note because ordering uses the
separate running flag.

**Needs attention:** Align the drag library's logical order with the rendered running/stopped blocks,
define cross-block drops, and verify that an in-block drag moves no stopped card and survives reload.

**Next:** Revise the Ordering design and acceptance evidence for that drag mapping, then run
`/review-design Ordering` again.

### 2026-08-27 — Fix: Mermaid diagram design

The Mermaid design is now the small integration it should be. Mermaid remains responsible for
parsing and drawing; AgentDeck adds only its existing Markdown hook, lazy loading, sanitization,
theme adaptation, and fallback. External-image nodes are rejected before Mermaid can load their
URLs, oversized input has one exact source limit instead of an unenforceable browser-thread timeout,
and visible diagrams update through the existing appearance observer when the skin changes. No
worker, iframe renderer, custom parser, persistence change, or server API is being added.

**Needs attention:** None.

**Next:** Run `/review-design mermaid-diagrams-in-chat` once more; if it closes cleanly, implement it
with `/work mermaid-diagrams-in-chat`.

### 2026-08-27 — Design review: Mermaid diagrams in chat

The design uses the right existing chat and presentation seams, but three gaps need to be resolved
before implementation. A diagram can currently ask Mermaid to load an attacker-controlled image
before the proposed sanitizer gets a chance to inspect the generated markup. A timeout around
main-thread rendering cannot interrupt a diagram that has already blocked the page. And a diagram's
generated colors would remain stale if someone switches appearance while the transcript is open.

**Needs attention:** Add a pre-render no-network boundary, make the render-work bound enforceable,
and make mounted diagrams react to appearance changes.

**Next:** Revise the Mermaid requirements and acceptance evidence for these three findings, then run
the design review again.

### 2026-08-27 — Design: Mermaid in chat, active-first ordering

Two of the three requested features are ready to implement, and the third is deliberately held.

Mermaid diagrams will render inline in the chat transcript. A fenced `mermaid` block in an assistant
reply becomes a themed diagram once its fence closes, stays an ordinary code block while it is still
streaming, can be toggled back to its source, and falls back to a code block with a short note when
the source does not parse. Nothing durable changes, so replay, archive, and search are untouched.
The renderer is bundled and loaded on demand, so the initial download is unaffected, and it is
pinned above the release that fixes a known diagram HTML-injection defect. Safety follows what
established tools converged on rather than a new boundary: interactivity off and a second sanitizing
pass over the generated markup at a single reviewed seam, because neither strict mode nor sandbox
mode alone has held up in practice. That matters here specifically, since this renders agent output.

Project grids will put running agents ahead of stopped ones inside each group, keeping the manual
drag order within each block and moving a card the moment its agent starts or stops. Nothing new is
stored. This required narrowing an interface requirement that forbade status from affecting order.

Editing a sent chat message was not specified. AgentDeck cannot give it the meaning Codex does: the
protocol version in use has no rewind, the transcript and search index are append-only, and the only
rewind-shaped mechanism would silently discard the agent's context. Rather than ship a weaker
meaning under the same name, the idea is held with those findings recorded so a later attempt starts
from evidence.

**Needs attention:** AgentDeck sets no Content-Security-Policy anywhere. Recorded as its own idea,
deliberately kept out of the diagram change because it is server-wide.

**Next:** Implement either waiting change; both are independent and need no decision to start.

### 2026-08-27 — Fix: Pipelines experience design

The Pipelines design is ready to implement as a deliberate product experience rather than a page
split alone. Runs is now a compact operational ledger with human stage names, an exact retained
count, and load-more history. A run gives live state and valid actions priority, keeps the execution
timeline as the main reading surface, and moves frozen setup, values, completed results, and agent
details into purposeful disclosure. Template authoring becomes a focused one-stage-at-a-time
workspace, and starting a run becomes a stable Setup, Runtimes, Review flow that preserves input and
returns errors to the right step.

The design now specifies brief route, dialog, disclosure, and live-entry transitions without blanking
content during background refresh, plus an equally complete reduced-motion mode. Reloaded run pages
receive human-readable, bounded stage and delegated-agent summaries immediately; unavailable agents
remain honest non-linking cards. Older runs no longer disappear after 100, and delegated work is
read through one bounded, indexed projection that attributes reused agents' tasks to the correct
attempt without loading every task and prerequisite in the project.

**Needs attention:** None.

**Next:** Implement the waiting Pipelines change from the revised specification and acceptance
fixtures.

### 2026-08-27 — Design review: Pipelines experience

The Runs/Templates split is the right foundation, but it is not ready to implement as the sleek,
high-end experience requested. The design currently proves that content moved to separate pages,
not that those pages have deliberate hierarchy, progressive disclosure, smooth state changes, good
use of space, or a reduced-motion equivalent; the existing dense forms could simply be rearranged
and still pass every check. The data contract also lacks the human-readable current-stage title and
agent preview the new views promise, older retained runs silently disappear after the first 100,
and the delegated-agent block applies its display cap only after reading every task in the project.

**Needs attention:** The experience, response shapes, run-history pagination, and bounded delegated
read need one follow-up design pass before implementation.

**Next:** Revise the waiting Pipelines design against the recorded findings, then review the revised
design before starting product code.

### 2026-08-27 — Design: split the Pipelines surface

The Pipelines page is specified to split in two. Today it stacks the AgentDecker builder, the
template editor, the start-run form and the run browser onto one scrolling screen, and a live run
shows its position only as a stage identifier. The designed replacement gives Pipelines two
sub-destinations: Runs, where it opens, and Templates. Each run and each template gets its own
addressable page, so authoring and supervision never share a screen.

A run's page shows its goal, project, state, frozen setup and named values, and then an execution
timeline: every attempt in the order it actually ran, so a repair loop reads as repeated entries
instead of collapsing into one. Under each attempt sit compact agent cards — the stage agent, plus
the agents it delegated work to, followed one hop. A card shows the agent's name, state and a short
preview and opens its live conversation or its archived transcript; it performs no lifecycle action.
When a run moves past a stage whose delegated agents are still running, the page says how many
remain. Starting a run moves into a dialog on Runs and opens the run it created, and old links
carrying a selected run still resolve.

Nothing about how pipelines behave changes: the template model, routing, approval gates, loop
bounds, recovery and the run state machine are untouched, no agent-facing tool or payload changes,
and no database column, index or migration is added. The one server change is an additive, capped
block on the run detail response listing each attempt's delegated agents with the work they were
given, composed from data that is already stored and already served.

**Needs attention:** None. Every product and technical decision is resolved.

**Next:** The change is queued at `docs/ready-changes/split-pipelines-surface.md` and is waiting to
start. A design review before implementation would be reasonable given its size.

### 2026-08-27 — Fix: dashboard reliability and review findings

Dashboard tabs now share one live-event connection, preventing the sixth-tab freeze while keeping
live updates and ordinary data requests responsive. I also removed the amplification paths around
unchanged configuration refreshes and busy agent transcripts, kept missing-directory warnings
visible, improved task supervision and defaults, removed controls that could never succeed, and
aligned the remaining technical acceptance text with what is actually verified. All automated
tests, builds, specification checks, and the distributable build pass.

**Needs attention:** None.

**Next:** An independent review should verify the completed fixes and the six-tab browser behavior.

### 2026-08-27 — Bug investigation: multi-tab dashboard freeze

The root cause is confirmed: AgentDeck opens one permanent server-sent-event connection per tab over
plain HTTP/1.x. In Chromium, five populated tabs loaded normally; the sixth opened its live-event
connection, exhausted the same-origin connection pool, and stayed on “Loading project…” because its
ordinary data requests never reached AgentDeck. Closing one older tab released a connection and the
stalled page completed in 127 milliseconds. This explains the complete symptom cluster: older tabs
keep receiving notifications, the terminal shows only successful requests, agent work continues,
and memory pressure is irrelevant.

I added a deterministic local stress fixture with one orchestrator and six workers displayed as
Claude Haiku, backed by the repository's fake agent so it uses no credentials or provider spend. Run
`go run ./scripts/stress-fixture` and add tabs to the printed URL one at a time. The configured run
produced 7,000 streamed deltas and the tab failure also reproduced with the agents idle, so the
pipeline load is an amplifier, not the root cause. The earlier configuration-source and transcript
scaling findings remain real independent defects, but evidence from this computer cannot be
attributed to the separately affected Mac.

**Needs attention:** The product fix is a medium-sized transport/lifecycle change: either share one
live-event stream across tabs or move live events off per-tab long-lived HTTP connections. It should
be handled separately with a six-tab browser regression.

**Next:** If you want the fix now, have me implement the smallest robust transport option and verify
it with this fixture.

### 2026-08-27 — Bug investigation: pipeline-era dashboard freeze

The strongest cause is a configuration-source refresh storm: unchanged 30-second sweeps and project
file writes emitted live updates that forced every open AgentDeck tab to refetch, reaching 100
duplicate reads per minute while the server still returned fast 200s. Pipeline transcript writes
also hit two confirmed scaling hot paths — full transcript-tree rescans on every file event and
whole-dashboard transcript recomputation on every streamed agent delta — but the logs cannot prove
how much they contributed. The dashboard has since shut down; four surviving tabs show Reconnecting,
and the durable default home has zero running sessions, so those AgentDeck pipeline agents are not
still progressing now.

**Needs attention:** The shutdown was graceful but its source is undetermined because AgentDeck logs
neither the terminating signal/caller nor active-session counts. The config-source storm is captured
in a skipped regression test for the fix session.

**Next:** Run `/fix` to suppress unchanged configuration-source publications/refetches first, then
benchmark and bound the two transcript amplification paths and add sampled liveness diagnostics.

### 2026-08-27 — Usability review: everything user-facing since the last version

I drove the changes released since the previous version through a real browser against the built
app, on throwaway copies of the config so nothing touched your own setup. That meant the new Tasks
page and the dependent-work flow end to end, the "needs attention" count on the dashboard, the new
limit on how many tasks run at once, unsent chat drafts, the browse-for-a-folder button, and the
right-click menu on the projects canvas.

The headline is that dependent work holds up. I created work that waits on other work, satisfied it,
failed it, cancelled it, repaired it, deleted it, and restarted the server underneath it — and the
page told the truth at every step, updating live without a refresh. A task interrupted by a restart
says so in plain words and offers a working Retry. The concurrency limit refuses bad numbers, saves
correctly, and actually holds. Chat drafts survive navigating away and reloading, don't leak between
conversations, and don't come back after you send them. No blockers, and no console errors anywhere.

One problem needs fixing. If you point a project at a folder that doesn't exist — a typo, or a repo
you haven't cloned yet — it saves silently. The server does notice and sends back a warning saying
the directory isn't there, and the form even has the text prepared to show it, but the dialog closes
in the same instant the warning arrives, so the warning is never displayed. You find out later, when
launching an agent in that project fails. It affects creating a project as well as editing one, and
it isn't new in this release.

Five smaller items are recorded for the fix pass: the Tasks page never tells you which agent is
actually doing a task, so there's no way to jump to that conversation; a waiting task names what it
is waiting for by internal id rather than by the name shown two rows above it; the new-task form
ignores your configured default role and quietly preselects the internal AgentDeck helper role
instead; a task parked by a failed prerequisite offers a Retry button that can never work, when the
repair it needs is sitting beside it; and the attention count says "1 task need attention".

I also closed a check that had been sitting open: right-clicking anywhere on the projects canvas,
including the empty margin around the cards, really does offer New project, verified in a real
browser at eight different spots. The related folder-picker check is now narrowed — the buttons are
all present and wired, but the macOS folder panel itself opens a real system dialog that automation
can't drive, so confirming it needs you at the keyboard for about a minute.

Full detail and screenshots: `docs/archive/reviews/usability-review-run-2026-08-27-release-delta.md`.

**Needs attention:** Another session was editing the shared handoff file at the same time as this
run — it added a code review with three findings while I was working. I appended to it rather than
overwriting, and nothing was lost, but two agents writing that file at once will eventually drop
something. Worth checking whether that second session was intentional.

**Next:** Run the fix pass on the six recorded findings, starting with the silently-saved bad project
directory. Separately, when you have a minute at the machine, click one of the Browse… buttons so
the folder-panel check can be closed.

### 2026-08-26 — Review: checking the fixes actually landed

I went back over each of the nine items from this morning's review against the current code and ran
the full checks. Seven are genuinely fixed, with tests that would catch a regression rather than just
code that looks right. The pipeline dependency failure now reaches the Tasks view and travels all the
way down a chain of waiting work, and chasing it turned up a further problem worth knowing about: the
same repair after a restart had never worked at all, because the recovery pass was reading a list of
prerequisites that was always empty. That is fixed too.

One of my two headline items was partly my error. I said five pipeline refusals were being described
to agents as worth retrying; three of them were, and those are fixed. The other two can only come
from actions a person takes in the browser, never from a tool an agent can call, so they were
correctly left alone.

Three things remain open, none urgent. The tool-contract document still promises a few checks that
were never written, so it claims more testing than exists. A task whose agent cannot be shut down
after a failed start still sits silently part-way through starting until the server restarts — a
deliberate choice to avoid abandoning a live process, but nothing tells the person waiting. And one
existing test fails occasionally under load because it stops an agent a fraction of a second before
the system is ready to accept it; the product behaves correctly, the test is just impatient.

**Needs attention:** None. Everything left is small and recorded.

**Next:** A fix session can take the three remaining items whenever convenient; nothing blocks new
work.

### 2026-08-26 — Review: dependent-work fixes and the new agent tool result contract

I reviewed the two rounds of dependent-work fixes and the newly shipped rule for how AgentDeck answers
an agent's tool call. The fixes hold up on the paths they were written for, but two problems are
serious enough to fix before relying on either feature.

First, dependency failure still goes unnoticed when the thing that failed is a pipeline run rather
than another task. The waiting work is parked correctly in the database, but the Tasks view and the
dashboard's attention count keep showing it as still waiting until someone refreshes, and any work
queued behind that parked task waits forever with nothing left to release it. The same gap opens
after a server restart. The previous round fixed exactly this, but only for task-to-task
dependencies.

Second, the new retry advice is wrong for the pipeline tools. Five of their refusals are missing from
the classification table, so an agent that reports a stage result too late — or proposes a malformed
pipeline — is told the call might work if it tries again, when it never will. That is the one mistake
this feature exists to prevent. The test meant to guard against a missing entry cannot catch it,
because it only compares the table to a copy of itself.

Seven smaller items are recorded too: the rule for who may attach shared context is now written out
twice and the two copies already drifted apart once; two paths can leave a task stuck part-way
through starting with nothing signalling it to a person; the dashboard says "0 tasks need attention"
when it could not load the tasks at all; a mistyped context reference is reported as a prerequisite
error in raw internal wording; and several of the tool-contract acceptance checks describe tests that
were never written.

**Needs attention:** The two serious items above. Both are the same class of problem the last round
closed, surviving on a path that round did not cover.

**Next:** A fix session should take them in the order listed, starting with the pipeline dependency
failure.

### 2026-08-25 — Fix: dependent-work review findings

All thirteen dependent-work review findings are now closed. Tasks assigned to existing agents keep the real runtime identity, independent task starts run in parallel again, and context attachments respect every valid authorization path. The task workflow is protected by Go and interface regressions rather than only manual review.

**Needs attention:** Live Claude and Codex task runs are still required before relying on dependent work with real providers.

**Next:** Run one task end to end with each pinned provider when authorized.

### 2026-08-25 — Feature design: telling agents whether a refused tool call is worth retrying

Most of what your "richer agent-facing orchestration API" idea asked for is already built. I checked
the code rather than the notes. Tools already answer agents in structured data with stable codes, not
prose. Agents already say "start this work when that other work finishes" and AgentDeck already wakes
them when it does, with no polling and no model in the loop. An agent given a task already receives
that task's own attached context directly, instead of scanning a shared list and guessing which
entries are meant for it. Those were the idea's three main asks.

Two real gaps were left. The first is that when a tool says no, an agent cannot tell what kind of no
it is. A malformed request, a name that matches two agents, a per-turn message allowance that has run
out, and a database hiccup all come back as an English sentence the model has to interpret. It gets
this wrong in the case that costs something: told "this message was not sent", an agent re-sends
inside the same turn, which cannot work. The second is that results are handed over as text that
happens to contain structured data, so every client re-parses a string the protocol could carry
properly.

I also went looking for the "wait until X, then retry" condition your idea described, and it has no
home. Asking an agent to do work while it is busy is not refused at all — the work is recorded and
starts when there is room. The one refusal that names a target is permanent, not temporary: it means
that agent can never be given work, not that it is occupied right now. So there is nothing for a
retry condition to point at, and the honest version is a plain label saying which of four kinds of no
this was: never, try again with different arguments, not in this turn, or a temporary glitch. I wrote
the label as a small object rather than a bare word so a real condition can be added later without
renaming anything.

You made four calls: keep the deliberate rule that agents cannot browse the work graph, since letting
them would bring back exactly the polling this design removes; have waiting keep working the way it
does now, where an agent finishes its turn and AgentDeck starts a fresh one when the wait ends, rather
than holding a conversation open; add structured results alongside the existing text rather than
replacing it, so nothing breaks; and build the reduced version rather than parking it.

This is specification and planning only — no product code changed. The new specification is FS-17
with two supporting technical requirements, and the change is queued in `docs/ready-changes/` and not
started. The pieces you set aside — letting an agent repair its own stuck work, letting it inspect
work state, letting it stop or resume other agents, creating groups of tasks at once, and declaring
formal result schemas — are recorded in `docs/ideas.md` with what each one still needs.

**Needs attention:** Corrected on 2026-08-26. When I wrote this, the dispatcher fixes were sitting
uncommitted in the working tree and I described the dependency work as still carrying its review
findings. That is no longer true: all ten **Must fix** and three **Worth fixing** findings from the
review through `76b1493` were closed in `e1e827b` and `b121fd0`. What remains is that those two fix
commits have not themselves been independently reviewed. This design has no dependency on either.

**Next:** Nothing of mine is in progress. Either review the two dependent-work fix commits, or build
the queued change; neither blocks the other.

### 2026-08-24 — Fixes then build: review findings closed, dependent work started

All four findings from the last review are fixed, each with a test that fails without the fix. Mail
activation no longer races a pipeline for the same agent's turn — it now competes for the same lock
the pipeline takes, instead of glancing at whether one was held a moment earlier, so a pipeline stage
can no longer be paused by a message arriving at the wrong instant. Sharing the current turn can no
longer include the share call itself: that was leaking the recipient and the private label and
description you wrote for them into the content the recipient reads, and only on Claude, so the same
share behaved differently depending on the backend. The rule about when AgentDeck starts a model turn
now matches what it actually does. And the loop that wakes agents with waiting mail now has a real
limit — before, fifty agents with unread mail meant fifty agent processes starting at once, every two
seconds.

With those closed I started dependent work — the feature where a piece of work waits on other work
and then starts by itself. Three pieces are in and each stands on its own. Work items, their
prerequisites, and a shared record of what finished with what outcome now exist and survive restart.
The logic that decides when waiting work becomes runnable is in and tested: a task waiting on three
things becomes runnable exactly once, when the last one lands, and a prerequisite that fails or is
deleted parks its dependents visibly instead of leaving them waiting forever. And the instruction
AgentDeck sends when it wakes an agent is now looked up per reason rather than hard-coded to mail, so
"you have a task" can be said differently from "you have mail".

None of this is reachable from the app yet, and nothing in the specification is marked as shipped —
the tables and logic exist, the feature does not. The next piece is the part that actually starts the
work: taking a slot and claiming the agent in one step, then launching or waking it.

**Needs attention:** One test failed once, early on, in a run of several packages together, and then
passed in every run afterwards — six full suite runs and several targeted ones. I could not identify
or reproduce it, so there is nothing to fix yet, but it is worth knowing that something in that area
may be timing-sensitive.

**Next:** Continue building dependent work — the scheduler that admits ready work against the
concurrency budget and starts its agent. No decision needed from you.

### 2026-08-24 — Implementation: dependent-work admission and concurrency setting

Dependent work can now reserve an agent and capacity atomically before any runtime starts, so concurrent tasks cannot both claim the same agent and a full budget defers work without consuming a retry. The install-wide task-runtime limit defaults to ten and can be changed in Settings; invalid values are refused without overwriting the saved setting.

**Needs attention:** None.

**Next:** Continue implementation by connecting admitted tasks to the existing launch and resume paths and confirm them as running only after the assignment reaches the live agent.

### 2026-08-24 — Implementation: work that waits for other work and starts itself

You can now describe a piece of work, say what it is waiting for, and AgentDeck starts it when those
things are done — no agent polls another, waits, or sends a message just to say it finished. Work can
wait on another piece of work finishing a particular way, on a pipeline run, or on a signal you fire
by name from outside (a CI result, an approval). AgentDeck either launches a fresh agent with the
instruction or hands the work to an agent you already have, and it only calls the work "running" once
the agent really has it. A new Tasks screen shows everything a project is waiting on, and the
dashboard tells you how many pieces need you.

Finishing is explicit. An agent reports success, failure, or blocked and gets its answer back before
anything stops it; you can record the result yourself when an agent went away without one. An agent
that crashes never turns into a "success" — the work is marked interrupted for you to retry or
resolve. AgentDeck stops only the agents it started for the work and never touches a conversation you
were already in, restarts pick up exactly where they left off, and a limit you control (ten by
default) keeps it from bringing up more agents at once than your machine wants.

Two older bugs surfaced and are fixed: starting two agents in the same project at the same moment
could fail one of them, and one internal delete check could hang.

**Needs attention:** None of this has been run against the real Claude or Codex CLIs yet — it is
proven against the test adapter, the database, and the interface tests. That live pass is the one
thing owed before trusting it with real work.

**Next:** Run one real task end to end with a live provider, then decide whether anything about the
Tasks screen needs to change once you have used it.

### 2026-08-24 — Review: dependent work

The review found ten must-fix and three worth-fixing issues. The most urgent are that tasks assigned
to existing agents cannot report results reliably, cancellation can still launch work, failed
prerequisites can strand downstream tasks, cleanup can lose ownership of a live runtime, and task
state changes can remain stale in the interface. Context attachment creation and the Tasks interface
also need validation and completeness fixes. The existing automated suites remain green but do not
cover these paths.

**Needs attention:** Do not treat dependent work as complete until the must-fix findings are resolved;
live-provider validation also remains outstanding.

**Next:** Run `/fix` to address the recorded findings, starting with runtime ownership and reporting.
### 2026-08-23 — Design fixes round two: all fourteen re-audit findings closed

All fourteen findings from the second review are fixed in the requirements. Still no product code.

Three of them rested on claims about how AgentDeck already works, so I checked those instead of
taking them at face value, and two turned out to be partly wrong in a way that made the fix smaller.
The review worried that deleting a half-finished pipeline could leave work waiting forever on a
result that will never come. It can't: AgentDeck already refuses to delete a pipeline that hasn't
finished, and a pipeline that has finished records its result in the same moment, so by the time
deletion is even possible everything waiting on it has already been released or parked. I wrote down
why rather than building the machinery the review asked for. The review also said pipelines decide
what to do next in the same step where they accept an agent's report; they don't, that happens later.
So tasks and pipelines now share only what is genuinely the same — the words for a result, the size
limits, and the check that the reporter still owns the work — instead of pretending one shared step
could do both jobs.

The third claim held, and was worse than described: the instruction an agent receives when AgentDeck
wakes it is hard-coded to say "you have new messages", in one place, with the word "mail" written
literally into every database statement behind it. An agent woken for a task would have gone looking
through its inbox and found nothing. That instruction is now per-kind and code-owned.

The rest were real design mistakes. Deleting a task while its agent is still running would have
thrown away the only record of that agent — deletion is now refused until cleanup finishes. Restart
would have killed an agent a person was using, because the task had merely borrowed it — restart now
checks what the task actually owns before stopping anything. Retry had no limit and no rule for what
counts as a failure, so a busy machine could have burned through a task's chances; only genuine
failures count now, and waiting your turn never does. A person recording a result, or cancelling,
could have left a live agent behind if the machine died at the wrong moment; every ending now writes
down what still needs cleaning up before it does it. And retry now says which agent picks the work
back up, instead of leaving it to chance.

**Needs attention:** None.

**Next:** `docs/ready-changes/dependency-aware-armed-agents.md` is updated and still waiting to start.
Two review rounds have now landed twenty-three findings; a third pass is worth it before building.

### 2026-08-23 — Design fixes: all nine review findings closed, budget raised to ten

The concurrency budget now defaults to ten, and all nine findings the design review raised are fixed
in the requirements. Still no product code.

I checked the two findings that rested on claims about existing code rather than taking them on
trust, and both held — one of them was worse than the review said. It claimed restart could not
confirm that an agent which survived a crash is still doing its job. That is right: AgentDeck starts
with an empty picture of which processes it owns, and it deliberately never re-adopts an agent that
outlived the previous run. So the recovery I had written was asking a question that cannot be
answered. Recovery now does what the rest of the product already does — it does not pretend anything
survived. A task caught mid-start has its leftover process cleaned up and is started again; a task
that was running becomes interrupted and asks for attention; a result that was recorded but whose
cleanup never finished is finished during recovery.

The second was about stopping an agent too early. Pipelines already wait for the reporting turn to
finish before stopping the agent that reported, and for good reason: stopping mid-call would cut off
the very response the agent is waiting for. My design stopped immediately. It now waits the same way,
and the single place the server notices a turn ending becomes a shared one rather than gaining a
second copy.

The other seven were real too. A task whose agent crashes is now *interrupted* rather than absurdly
still "running". A person can record a result, which the design promised but gave no way to do. Re-arm
and retry are now two different things: retry re-attempts work, re-arm replaces what a task is waiting
on, and only re-arm can rescue a task stuck behind something that will never happen — retrying that
now says so instead of quietly re-parking. Deleting a piece of work no longer has contradictory
effects: a result outlives the task that produced it, so anything already released by it is untouched,
and only work still waiting gets parked. Tasks record who created them, so "an agent may cancel what
it created" is enforceable, and it survives that agent being stopped and resumed. One-active-task-per-
agent is now a database guarantee taken at the same moment as capacity, not a hopeful consequence of
ordering. And a task's context access is separated from the short-lived per-launch registration, so
stopping and resuming an agent no longer both revokes and preserves the same permission.

**Needs attention:** None.

**Next:** `docs/ready-changes/dependency-aware-armed-agents.md` is updated and still waiting to start.
Nine findings on the first pass is worth a second design review before building it.

### 2026-08-23 — Design review: dependency-aware work

The design is not ready to implement. Nine confirmed gaps make normal recovery and concurrency
ambiguous or impossible, including what state a task enters when its agent disappears, how a person
records the missing result, how retry repairs a failed dependency, how one agent is protected from
two active tasks, when a reporting agent may safely be stopped, and how restart handles an in-flight
start. Creator ownership and attached-context lifetime also lack coherent durable contracts.

**Needs attention:** Revise the planned design before implementation; several fixes require explicit
choices about interrupted-task recovery, retry, and deletion behavior.

**Next:** Use `/design-feature Dependency-aware / armed agents` to resolve every finding, then run
`/review-design Dependency-aware / armed agents` again.

### 2026-08-23 — Design review: dependency-aware work still has lifecycle gaps

The change is not ready to implement. The second review found twelve must-fix gaps and two smaller
ambiguities even after the first review’s fixes. The most serious cases can lose ownership of a live
agent: deleting or cancelling active work can erase the cleanup record, restart can stop a personal
agent that the task only borrowed, and a person-recorded result can become final before its required
stop is durably recoverable.

The start path also lacks a complete contract. Assignment is recorded at two different moments,
retry does not say whether an interrupted launch-created task reuses its agent or creates another,
and the promised bounded automatic retries have no limit or failure policy. The existing activation
path only knows how to tell an agent to check mail, so dependent work needs its own fixed instruction
to retrieve the assignment.

Pipeline integration needs narrowing. A pipeline stage report and a terminal pipeline-run result are
different events with different vocabularies and transactions, so they cannot share the single
acceptance transaction currently required. Deleting an unfinished pipeline run also has no path that
parks tasks still waiting on it. Two smaller stale lines count healthy running work as needing
attention and ask restart to consult an in-memory registry that is empty by design.

No product code, specifications, or ready-change file changed.

**Needs attention:** Revise the planned lifecycle, retry, activation, deletion, and result-layer
contracts before implementation.

**Next:** Use `/design-feature Dependency-aware / armed agents` to resolve every finding, then run
`/review-design Dependency-aware / armed agents` again.

### 2026-08-23 — Design review: orchestration-plane separation

The separation between control state, durable context, and model conversations is structurally sound,
but the retrospective review found three correctness gaps. Mail activation can still race a pipeline
or other lifecycle transition and take the agent's turn first. The top-level product rule accidentally
excludes the rich assignment and continuation prompts that pipelines already use. And with Claude's
real tool-event ordering, sharing the current turn can include the `share_context` call itself — along
with recipient and label metadata — even though the context source is meant to stay independent of
its access path. The activation recovery loop also needs a fixed batch/concurrency limit before its
“bounded” claim is true.

The overall architecture does not need to be replaced. The payload-free activation record,
mail-specific retry policy, single context service, separate non-waking recipient lookup, and scoped
pull reads all reuse the intended existing seams without creating a second scheduler, context store,
or messaging authority.

**Needs attention:** Resolve the three correctness findings before extending activation to dependent
work; the concurrency bound can follow as a smaller hardening change.

**Next:** Run `/fix` to validate and close the activation race first, then revise the two crossing and
context-source contracts before implementation continues on dependent work.

### 2026-08-23 — Design revision: budgeted starts, confirmed runtimes, released claims

All three points were right, and the design now reflects them. Nothing is implemented yet.

Readiness and running processes are no longer the same question. A graph that fans out to fifteen
tasks still has fifteen ready tasks — the dependency model is never narrowed to fit a machine — but a
budget, four by default and adjustable in settings, limits how many agent processes AgentDeck brings
up for that work at once. As each one finishes and its process is stopped, the next ready task starts,
in the order they became ready. A task that lands on an agent already running for its own reasons
costs no slot, because it starts no process. The budget can be changed at any time and never changes
what a graph means.

The start path now has the step it was missing. A task goes ready, then starting, then running, and
`running` means the assignment genuinely crossed into a live runtime. The starting record names the
agent, the generation, and the attempt, so after a crash AgentDeck checks whether that exact runtime
is alive: if it is, the task is running; if it is not, the attempt is abandoned and the task goes back
to ready to be started once more. There are no longer rows that claim to be running when nothing is.

Finishing a task now releases the task's hold on the runtime, and stopping is a consequence rather
than the rule. If the task launched the agent, or woke a dormant one just to do the work, that agent
is stopped and left visible and resumable, as before. If the task merely borrowed a runtime someone
was already using, it is left alone — finishing a piece of work no longer ends a conversation that
was never about it.

**Needs attention:** None.

**Next:** The ready change `docs/ready-changes/dependency-aware-armed-agents.md` is updated and still
waiting to start. FS-04 joins the specs that flip back from `Partial` when it ships.

### 2026-08-23 — Design: dependency-aware work that starts itself

Dependency-aware work is specified and waiting to build. No product code changed.

The reason this was impossible before is that AgentDeck had nothing durable to hang a dependency on.
A task group is only a dashboard label, "assign to a new task" launches an agent on the spot, and an
agent's status describes its process, not its work — `done` is what you get from an ordinary Stop and
from a terminal window closing, so it never meant "the work succeeded". Only pipelines recorded a
real result, and only inside a single run.

So the design adds the missing piece: a task. It is a durable record with a name, an instruction, who
runs it, and one explicit result — succeeded, failed, blocked, or cancelled. The result is always
reported, by the agent or by you, and is never guessed from an agent going idle, stopping, or
crashing. A task can wait on other work: on another task's result, on a pipeline run's result, or on
a named signal you or an outside system fires when something AgentDeck cannot see is ready. All the
conditions have to be met, and there is no expression language to learn.

When the last condition is met, AgentDeck starts the work itself. If several pieces become ready at
once, all of them start, because fan-out is the point. Nothing polls, waits, or sends an "I'm done"
message to release the next step. When a task finishes, its agent is stopped but not archived, so the
process stops eating memory while the card stays where it is and can be resumed like any other
stopped conversation. If something a task was waiting on ends in a way that can never satisfy it, the
task parks and asks for attention instead of being quietly cancelled or waiting forever. If an agent
disappears without reporting, its task says so and everything downstream keeps waiting.

Pipelines and tasks converge where they genuinely duplicated each other and nowhere else. They now
share one result vocabulary and one path for accepting a reported result, and a finished run
registers its outcome so other work can depend on it. Their run machinery stays separate on purpose:
a pipeline run walks a fixed template and can loop back on itself, while a task graph never has
cycles, so folding runs into tasks would have forced the task model to grow a loop engine it does not
need.

A task can also carry context references with their own labels, so an agent starting dependent work
is handed what it needs instead of scanning a shared list and guessing. Reassignment and shared
ownership of a task are deliberately left out, along with any way for an agent to browse the task
graph — that would put back the polling this exists to remove.

**Needs attention:** None.

**Next:** The change is `docs/ready-changes/dependency-aware-armed-agents.md`, waiting to start. It
is the largest of the three orchestration follow-ons, so a design review before building it would be
time well spent. The sibling idea, a richer orchestration API for agents, is still in `ideas.md` and
now has a much smaller gap to close.

### 2026-08-23 — Fix: context sharing now points at the right work

All four problems the review found in context sharing are fixed. An agent can no longer hand over
its last finished turn while it is sitting idle — that option now only works from inside a turn it
started for its own reasons, which is what it was always meant for. If an agent is interrupted or
crashes mid-turn and is then resumed, the work it shares afterwards starts cleanly at the resume
instead of dragging in abandoned text from before. When a transcript record is too large for
AgentDeck to read and it happens to be the first or last record of the shared turn, the reader now
sees the same "something was omitted here" note it already got for records in the middle, so a page
never looks complete when it is not. And a page cursor that has been altered to point into the
middle of a character, or past the end of the shared text, is now refused instead of returning
damaged text or falsely claiming there is nothing more.

Each fix has a test that fails against the old code, and the two full-dashboard tests now share from
inside a genuinely open turn rather than the idle shortcut they were accidentally endorsing. The
written product rules were updated to state these boundaries explicitly. The whole test suite is
green in both database modes.

**Needs attention:** None.

**Next:** Context links are clear of findings, so the next move is yours: pick the follow-on feature
(work objects and assignment retrieval are the recorded candidates) or ask for a fresh review.

### 2026-08-23 — Review: pull-based context links

The storage, authorization, mail separation, and tool wiring are sound, and the full test suite is
green. The review found three cases where a shared transcript can describe the wrong slice of work:
an agent can share its last finished turn while it is idle even though that option is meant for a
later independent turn; a new turn after an interrupted stop or crash can accidentally include text
from before the resume; and an oversized record at the start or end of a selected turn can disappear
without the promised omission marker. A lower-priority cursor issue can also produce damaged text if
a caller alters a page cursor to point inside a multi-byte character.

**Needs attention:** Fix the three turn-selection and omission-marker findings before building more
features on context references; the cursor validation should follow in the same pass if practical.

**Next:** Run `/fix` and handle the findings one at a time, starting with the idle last-turn share.

### 2026-08-22 — Design review: pull-based context links

I reviewed the pull-based context-links design before any code is written, and it is not ready to
build yet. Two problems have to be settled first, and six more are worth settling with them.

The first blocker: the design gives the new context service its own way of turning an agent's
recorded conversation into readable text, and AgentDeck already has two such converters — one in the
search indexer, one in the chat view. A third copy means the next time we add a new kind of
conversation event, someone updates one or two of the three and a shared transcript quietly comes
back missing a whole class of content, with every test still green. This is the exact drift the
project's invariant list already paid for twice.

The second blocker: the design offers a pipeline stage's finished report as one of the two things an
agent can share, but the way pipelines already work, a stage agent is stopped as soon as its turn
ends, and the report stops being reachable at the same moment. The only window in which that share
can happen is the same turn in which the agent files the report, and no later agent can ever point at
an earlier stage's report. That may still be worth building, but the design should say so plainly
instead of presenting it as a first-class source.

The six smaller ones, in short: the "seen / hidden" bookkeeping is recorded but never read by
anything; deletion rules are written for deleting an agent, which the product currently cannot do;
the shared material can include credentials, yet only the sharing agent — never you — can withdraw
access, and nothing ever expires; the promise that no content is silently dropped is contradicted by
the transcript reader, which does silently drop very large records; there are two error names for
what looks like one situation and only one is defined; and neither sharing option can cover the turn
the agent is in the middle of finishing, which is exactly when it would want to hand work over.

The design's factual claims about existing code mostly checked out, and I have recorded those so the
next pass does not re-verify them.

**Needs attention:** One is a decision only you can make — durable, invisible, never-expiring access
grants to conversation content that may contain secrets, with no owner-facing way to see or revoke
them. Either we add a small owner list-and-revoke surface, or we accept it deliberately and write
down why.

**Next:** Run `/design-feature Pull-based context links` to work the findings through with me;
implementation should not start until the two blockers are closed.

### 2026-08-22 — Design revision: pull-based context links

Seven findings are resolved; one product/privacy decision remains. The change is paused, not ready
for implementation.

- **Duplicate transcript projection — adopted.** Search and context rendering will share one Go
  semantic event projection in `internal/transcript`, backed by an exhaustive normalized-event
  registry/test. Context adds readable framing; search builds its search text from the same decoded
  parts. The TypeScript chat reducer stays separate because it produces UI objects in another
  language, while its live and replay paths already share one reducer.
- **Pipeline-report reachability — adopted, source retained.** The contract now says the only direct
  selector window is `report_pipeline_stage_result` → `share_context` → `turn_end`. After pipeline
  reconciliation, selection returns `source_unavailable`; an already-created reference stays
  readable. Keeping the source is worthwhile because accepted reports are compact immutable results
  and the imminent work-object feature can attach the same canonical reference.
- **Seen/hidden projection — partially adopted.** Unused `seen_at` and read-side mutations are gone.
  Hide/unhide remains because `list_context_links` directly consumes it and it implements the
  approved rule that hiding an ad-hoc entry does not revoke access or detach work context.
- **Agent deletion journey — adopted.** No agent-deletion product behavior is invented. The design
  retains only defensive recipient foreign-key cleanup and grantor provenance; acceptance tests the
  cascade at state level. Reachable pipeline-source deletion still produces a tombstone.
- **Owner control/retention — valid, unresolved.** The specs and change file now expose this as a
  blocking product choice rather than silently accepting indefinite invisible access.
- **Oversized transcript records — adopted.** Existing tolerant readers may keep skipping records
  above 8 MiB, but they expose an opt-in diagnostic; context reads must render a bounded omission
  marker instead of silently returning an apparently complete transcript.
- **Two unavailable errors — adopted.** `source_unavailable` is a share-time selector failure that
  creates nothing. `context_source_unavailable` is an authorized existing reference whose durable
  source was later deleted or became unreadable.
- **Current-turn conclusion — adopted.** The documented handoff is: emit the reasoning-relevant
  conclusion, call `share_context` as the final context-producing action, then optionally send mail
  containing only the reference id. Tests include pre-share text and exclude later text.

The review's non-findings remain unchanged: context correctly uses a separate durable recipient
directory rather than mail wakeability; per-page transcript rescanning is an optimization to revisit
with evidence; MCP resources remain behind a real-provider capability gate; and SQLite foreign-key
cascades are already enabled.

My recommendation for the remaining choice is **A: a minimal owner-only list and revoke surface**.
It preserves durable context for delayed review and reassignment while giving the human an escape
hatch for accidental secret sharing. A retention limit is operationally simpler but can silently
break legitimate long-lived work; permanent agent-only grants are the smallest implementation but
leave the privacy issue intact.

`make check-specs` and `git diff --check` pass. No product code changed.

**Needs attention:** Choose A (owner list+revoke), B (retention limit), or C (explicitly accepted
indefinite agent-only grants).

**Next:** Once chosen, I’ll close the final finding and return the change to Waiting to start.

### 2026-08-22 — Feature design: pull-based context links

Pull-based context links are fully specified and ready to build. An immutable transcript span or
accepted pipeline-attempt report gets one reusable, target-neutral reference id. Direct grants,
future work attachments, and personal seen/hidden state remain separate, so labels can vary by use,
reassignment does not require copying context, and hiding an ad-hoc share cannot detach task context.

The first change adds one in-process reference service, separate SQLite reference/grant/view rows,
and token-scoped MCP tools to share, list direct ad-hoc grants, read bounded pages, hide/unhide, and
revoke. It adds no artifact store, workflow graph, REST/UI context inbox, MCP resource registration,
mail, activation, prompt injection, transcript event, or SSE content payload. Work-attached discovery,
durable participation authorization, dependencies, and the richer orchestration API remain recorded
for their own designs.

Verified with `make check-specs` and `git diff --check`. No product code was changed.

**Needs attention:** None.

**Next:** Start the waiting change with `/work pull-based-context-links`, or run `/review-design`
first.

### 2026-08-22 — Fix: all eight mail-activation review findings

Every open finding from the two architectural reviews is fixed. Mail no longer starts an empty model turn when someone reads the mailbox just before the work is picked up, and mail can no longer steal the turn from a pipeline stage that is mid-launch. A failed status save now stops the model turn instead of running it invisibly while the dashboard shows the agent as idle. Setup that fails right after waking an agent now cleans up fully instead of leaving a live agent running with nothing to do, shutting down no longer leaves a stray agent process behind, and a failed startup repair now stops the dashboard loudly instead of quietly losing that mail for the rest of the session.

The specifications were also brought back in line: they no longer describe the old unread-polling wake mechanism that was replaced, and the promised guarantees about mail producing exactly one turn are now actually proven by tests that count how many turns reach the model — across batched mail, an emptied mailbox, unread mail left behind, restarts, mail arriving mid-turn, and failed wakes. Four of the fixes have regression tests confirmed to fail against the old code.

**Needs attention:** One lower-priority cleanup was found and deliberately left for a separate pass: the old wake-tracking code the replaced mechanism used is now unreachable but still present, and removing it touches the mailbox read path.

**Next:** Someone should review this fix work; the reviewed-code marker still points at the commit before it.

### 2026-08-22 — Fix: mail delivery to agents that have run a pipeline

An agent that is running normally now receives its mail again, even if it took part in a pipeline earlier in its life. The last change checked a "was this agent part of a pipeline?" rule against every recipient, but that rule exists only to keep a stopped pipeline agent asleep — the pipeline decided to stop it, so a message should not restart it. Applied to a running agent it meant the sender saw a success, the unread badge went up, and the agent was never prompted to look at its mailbox. Since the pipeline record is never cleared, that agent would have stayed silently unreachable by mail forever.

The three rules that only make sense for a stopped agent — archived agent, archived project, and pipeline participation — now run only on the stopped path, where they already had a single shared implementation; the duplicate copy that caused this is gone. The one rule that still applies to both is the terminal interface, since a terminal agent cannot read a mailbox at all. A new test sends mail to a running agent that has run a pipeline stage and checks it gets exactly one prompt, and the tests protecting stopped pipeline agents from being woken still pass. Full test, build, race, and specification checks are green.

**Needs attention:** None.

**Next:** A review can cover this fix, and then the mail activation work is finished.

### 2026-08-22 — Fix: remaining mail activation findings

Mail activation now waits until resume validation passes before its one non-replayable attempt, so a removed backend or model leaves sent mail ready for retry after Settings are repaired. Work aimed at a recipient that has since become terminal, archived, project-archived, or pipeline-owned is retired rather than retried every two seconds; mail itself stays unread.

The release path now coalesces safely when newer mail arrives, errors are logged, and the superseded wake-tracking code and test shim are gone. Tests cover each case; the full test, build, distribution, and specification checks pass.

**Needs attention:** None.

**Next:** A new review can cover this fix before another change begins.

### 2026-08-22 — Implementation: agents can hand each other context without copying it

An agent can now give another agent access to a piece of its own work — the turn it is in the middle
of, its last finished turn, or the report it just filed for a pipeline stage — and the other agent
retrieves it only when it actually wants it. The sharer gets back a short id. Nothing is copied,
nothing is pushed into the other agent's conversation, and the share on its own never starts a model
turn, sends mail, or wakes a stopped agent. If the sender also wants attention, it sends ordinary
mail carrying just that id.

The receiving agent can list what has been shared with it, read it a page at a time, hide entries it
does not care about, and the sender can withdraw a share at any point. Hiding and withdrawing are
deliberately different things: hiding only tidies one agent's own list. The same piece of work always
gets the same id no matter how many agents it is shared with, so a later feature can attach it to a
task without inventing a second identity for it.

Two safety properties are worth calling out. An agent can only share its own work — the request names
which of *its* things to share, never someone else's — and an id someone is not entitled to read
looks exactly like an id that does not exist, so ids cannot be used to probe what other agents are
doing. Archiving an agent keeps its shared context readable; deleting it turns the share into an
honest "no longer available" rather than quietly pointing at something newer.

One piece of cleanup came with this: search indexing and this new reader now share a single
description of what a transcript event contains, instead of two copies that would drift. A test
enumerates every event type so a future one cannot silently vanish from search or from shared
context, and an oversized record that the reader has always skipped is now visibly marked rather
than leaving a page that looks complete.

Full test, build, race, and specification checks are green, including a check on the running
dashboard that sharing and reading send nothing to the model provider.

**Needs attention:** None.

**Next:** A review can cover this implementation; it is the only unreviewed code on `main`. There is
no waiting change after it — work objects and dependency-aware agents remain ideas.

### 2026-08-22 — Pull-based context-link lifecycle decision

Resolved with the simplest policy: no owner-management UI and no automatic expiry wiring. A direct
grant remains active until its grantor revokes it or the recipient identity is removed. This adds no
new user workflow and cannot make valid delayed work unexpectedly lose context.

FS-15, TS-02, and TS-05 now record this as an intentional tradeoff for AgentDeck’s current local,
single-user trust model, not an unresolved omission. They also say to revisit it only if AgentDeck
gains a multi-user trust boundary or real evidence shows accidental sensitive-context sharing is a
practical problem.

The final design-review finding is closed, the temporary idea entry is removed, and the pull-based
context-links change is back to **Waiting to start** with nothing blocking implementation.

`make check-specs` and `git diff --check` pass. No product code changed.

**Needs attention:** None.

**Next:** The reviewed change is ready for `/work pull-based-context-links`.

### 2026-08-22 — Review: mail activation after its fixes

The eight earlier fixes all hold. Batched mail produces exactly one model turn, an emptied mailbox retires the work without prompting, a failed status save stops the turn instead of hiding it, mail defers to a pipeline stage that is mid-launch, a failed setup after waking cleans up completely, shutdown no longer leaves a stray agent, and a failed startup repair stops the dashboard loudly. The full test suite, the specification check, and repeated race-detector runs over the messaging, wake, shutdown, and lifecycle paths are all green, so the architecture itself is in good shape.

Five things are still open, one of which should be fixed before the next change. When a stopped agent is woken to handle mail, the system records "we have attempted this" a little too early — before it has checked that the agent's backend and model still exist. If someone has since renamed or deleted that backend in Settings, the mail opportunity is silently thrown away for a failure that did nothing, and fixing the setting afterwards does not bring it back; the recipient simply never picks up mail that was already sent to it. The four lower-priority items: mail work aimed at an agent that has since been switched to a terminal, archived, or taken over by a pipeline is never cleaned up, so the system retries it every two seconds for up to a week; releasing a reservation fails silently when newer mail has arrived, leaving a stray record until the next restart; a leftover compatibility shim from the retired mechanism is still in the shipped code; and one test's assertion cannot fail, so a guarantee it claims to check is unproven elsewhere in that test. The already-known unreachable wake-tracking code is still there and still unreachable.

**Needs attention:** Fix the early "attempted" marking before starting new work — it is the only one that can silently lose mail handling in ordinary use.

**Next:** Run `/fix` and take the findings one at a time, starting with the early attempt marking.

### 2026-08-22 — Review: verifying the mail activation fixes

All six issues from the last review are genuinely fixed, and I re-ran the original reproductions to confirm rather than taking the fix at its word. Mail no longer burns its one chance to wake an agent when the agent's backend has been removed from Settings; releasing a reservation no longer fails and leaves a stray record; work aimed at an agent that has been switched to a terminal or archived is now cleaned up instead of retried every two seconds for a week; the leftover shim and the unreachable old wake-tracking code are gone; and the test whose assertion could not fail now checks the real thing. The specifications were updated alongside the code, and the full test suite, build, and race-detector runs are green.

The fix did introduce one regression that should be corrected before moving on. The new "is this agent still eligible?" check is applied to every agent, but one of the conditions it checks — having taken part in a pipeline — was only ever meant to stop a *stopped* agent from being woken. The result is that an agent which is running normally, but has run a pipeline stage at some point in its life, now accepts mail (the sender succeeds, the unread badge updates) and is never prompted to read it. Because that pipeline record is never cleared, the agent is affected permanently. I confirmed it by running the same scenario against the code before and after the fix: before, the agent got its mail turn; after, it gets nothing.

**Needs attention:** The pipeline eligibility check needs to apply only when the recipient is stopped. Everything else is ready to move on from.

**Next:** Run `/fix` for that one finding, then this work is done.
### 2026-08-26 — Implementation: clearer agent tool results

Agents now receive a machine-readable retry class whenever an AgentDeck tool refuses a call, so they
can distinguish permanent mistakes, argument fixes, next-turn limits, and temporary host failures.
Tool results also arrive as native structured content while keeping the existing text response
unchanged for older clients. The full automated test suite and production build are green.

**Needs attention:** Live Claude and Codex compatibility still needs the existing credentialed
provider check before those adapters are claimed as verified.

**Next:** An independent reviewer should review this implementation.

### 2026-08-26 — Fix: dependent work and agent tool results

Dependent tasks now update immediately and propagate failures through chained work when a pipeline
finishes or the server restarts. Task assignments also confirm the live runtime generation before
starting, context attachment checks share one authorization rule, invalid attachments use clear
wording, and the dashboard no longer reports zero attention when its task query failed. Agent tool
retry guidance now covers the pipeline refusals those tools can actually emit, with the malformed-
argument boundary documented. The full test suite, production builds, and distributable build pass.

**Needs attention:** Live Claude and Codex compatibility still needs the existing credentialed
provider check before those adapters are claimed as verified.

**Next:** An independent reviewer should review these fixes.

### 2026-08-26 — Work: no change available

There is no active implementation and no designed change waiting to start, so no product code was
changed. The repository remains ready for the next change once one has been designed and queued.

**Needs attention:** None.

**Next:** Define and queue the next change with `/design-feature`, then run `/work` again.

### 2026-08-29 — Fix: critical review and blocked-pipeline findings

Closed all four critical defects. Diagrams can no longer retain remote URLs in inline styles;
template deep links distinguish a temporary read failure from deletion; live chat panes no longer
keep an unbounded second transcript; and a blocked stage agent's second report no longer falsely
claims it came from a stale assignment. The full automated test suites and production builds pass.

**Needs attention:** Choose the blocked-stage contract. Option 1 keeps the current Continue-only
flow and makes it explicit that a blocked report ends the attempt, so replies in the agent chat are
out of band. Option 2 adds a real in-chat continuation route, allowing an answer there to resume the
pipeline; this is a larger behavior and recovery change.

**Next:** Choose option 1 or option 2; then `/fix` can implement that decision and continue through
the remaining lower-priority findings.

### 2026-08-30 — Implementation: shared AgentDeck operating guidance

AgentDecker now has a concise resident-operator role, while every AgentDeck-launched role can use one release-matched operating skill with focused guidance for agents, coordination, and pipelines. AgentDeck installs verified owner-only Claude and Codex views at startup, keeps the overlay out of durable session configuration, migrates only the exact historical AgentDecker prompt, and continues safely without advertising the skill if installation fails. The full automated test suites, production build, and distributable build pass.

**Needs attention:** Logged-in Claude and Codex discovery still needs the existing credentialed provider check before those adapters are claimed as live-verified.

**Next:** An independent reviewer should review this implementation, then run the credentialed provider check when access is available.

### 2026-08-30 — Review: shared operating guidance and release follow-ups

The continuous unreviewed work is now reviewed. The package installation, runtime-only delivery,
exact legacy-role migration, and lifecycle wiring are structurally sound. The review found one
security defect: diagram styling can hide an external URL with CSS escaping, bypass the literal
sanitizer, and allow an untrusted diagram to make a network request. It also found two lower-priority
verification gaps: the shared knowledge overlay is not exercised across every launch, resume,
switch, and pipeline path, and migration failures are missing read/write regression fixtures. All
required automated checks and the distributable build pass.

**Needs attention:** Fix the diagram sanitizer before treating untrusted diagrams as request-free.
Two earlier usability defects also remain Must fix: a failed layout read disables persistence for
the session, and some failed pipeline pauses still offer a dead-end agent chat.

**Next:** Run `/fix`, starting with the three Must-fix defects, then add the missing lifecycle and
migration regressions.
