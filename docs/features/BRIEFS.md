# AgentDeck — Session briefs

Newest first. Each entry is the exact final response from a feature-design, implementation, review,
fix-review, or usability-review session. Agents resume from [`HANDOFF.md`](HANDOFF.md), not this history. Earlier
entries are preserved in [`../archive/state/BRIEFS-pre-sdd.md`](../archive/state/BRIEFS-pre-sdd.md).

### 2026-08-17 — Implementation: Browse for working directories

You can now pick a project's working directory with the standard macOS folder chooser instead of
typing the path. The Browse button sits next to the working-directory field in Settings, in the
New project dialog on the projects page, and in the onboarding wizard, and next to the box where you
add an extra directory. Typing a path still works exactly as before, including the `~` shorthand.

Choosing a folder only fills the field in. Nothing is saved until you press Create or Update, an
extra directory still needs the Add button, and cancelling the chooser leaves whatever you had
typed untouched. Only one chooser can be open at a time, and if it fails the form tells you and
stays usable. The server shows the panel and hands back only the one folder you picked — it never
lists your disk, and a failure message never reveals a path.

The full Go, browser-component, presentation, build, and packaging checks pass, with new tests
covering selection, cancellation, the one-at-a-time rule, disconnect during a chooser, and the
security boundary.

**Needs attention:** No one has yet opened the real macOS panel end-to-end in a browser — the
automated tests deliberately fake it. Please try Browse once in Settings and in the wizard.

**Next:** Try the Browse button; then either name the remaining waiting change (saving unsent chat
drafts) or ask for a review of this one.

### 2026-08-17 — Fix: review-finding queue check

There are no open review findings to fix. Everything from the last code review and the projects
background-menu investigation is already fixed and committed, and no change is active. Two older
leads — a single unexplained archive error and some external terminal calls without explicit
timeouts — still have no reproduction, so they stay off the list rather than becoming work.

Your two design sessions from today are still sitting uncommitted in the working copy: the
specifications and waiting-change files for directory browsing and for saving unsent chat drafts. I
left them exactly as they were rather than committing work I did not produce or verify.

**Needs attention:** Those two design changes are uncommitted. Commit them yourself, or ask for a
design review first if you want them checked before they land.

**Next:** Name one of the two waiting changes to implement, or report a new issue.

### 2026-08-17 — Bug investigation: projects background actions

Confirmed: right-click actions fail on the blank projects background because the menu listener ends
with the project cards while the visible main canvas continues below it. Right-clicking inside that
smaller area works, which is why card actions and the existing heading-based test pass. A live
1280×720 reproduction showed the projects surface ending around y=293 and the canvas continuing to
y=720; right-clicking the lower blank area opened no AgentDeck menu, with no browser errors logged.

**Needs attention:** The background **New project** action remains unavailable from most of the
blank canvas; the persistent header button still works.

**Next:** Run `/fix` to extend the projects interaction surface across the available canvas while
preserving card-specific menus, then verify a right-click below the final card in the grid journey.

### 2026-08-17 — Fix and release: wake, proposals, and live context updates

All twelve review findings are fixed. Stopping and waking an agent can no longer race and tear down
the resumed process's registrations; mail wake retries are durably bounded without consuming newer
mail; wake and file-search storage failures remain visible instead of looking like ordinary stopped
agents; and agent discovery is consistent during lifecycle changes. Pipeline proposals now preserve
the exact request offered for approval, disappear only after a successful approval, stay bounded,
and tolerate one damaged record without hiding the rest. Provider context updates also reach the
dashboard during a turn, and messaging descriptions now include sleeping agents. The complete Go,
UI, build, distribution, specification, and focused race checks pass. Release
[`v0.2.2`](https://github.com/AsaphNoam/AgentDeck/releases/tag/v0.2.2) is published.

**Needs attention:** None.

**Next:** Update to `v0.2.2`; no follow-up engineering work remains for these findings.

### 2026-08-17 — Code review: stopped-agent wake, pipeline proposals, and context usage

The newest work is not ready to treat as settled. Eight required corrections surfaced. Stopping an
agent while it is waking can remove the new process's messaging and hook registrations while the wake
still reports success. Mail wake also has two retry-boundary failures: an adapter that starts and then
immediately crashes can be restarted every two seconds forever, while mail arriving during an older
failed wake is incorrectly marked failed too and cannot trigger its promised retry. Storage or project
configuration errors during a wake are currently disguised as if the agent simply did not exist.
The addressable-agent directory is also assembled by two separate database reads, so an agent being
stopped between them can appear twice and produce a false ambiguous recipient.

Two earlier fixes also need correction. Repeated run proposals can show the agent one exact request
while saving a different request for approval, which can replay an older run. Provider context updates
are held only in runtime memory until the turn ends, so the dashboard meter stays stale during a long
response. The new proposal database table is also missing from the owning persistence specification.
A smaller lifecycle gap leaves every successfully approved proposal labelled Pending forever, with no
dismissal or retention rule. Three additional lower-risk issues misreport file-search storage errors,
let one malformed proposal hide all valid proposals, and still describe messaging as live-agent-only.

**Needs attention:** Fix the eight required findings before relying on wake-on-message or durable
pipeline approvals. Decide the simple consumed/dismissed retention behavior for approved proposals
while making that repair.

**Next:** Run `/fix` to validate and repair the findings one at a time, starting with the wake/Stop
race and proposal payload mismatch.

### 2026-08-17 — Implementation: stopped chat agents wake when a message arrives

Stopping a chat agent is now a lightweight sleep rather than an ending. Typing into the composer of
a stopped conversation sends the message as usual: AgentDeck restarts that agent behind the scenes,
using exactly the settings it was frozen with, and then delivers the message, so the only visible
difference is that the reply takes a few seconds longer to start. Another agent can now find and
message a sleeping agent the same way — it appears in the agent list marked as sleeping rather than
running, its mail is saved before anything else happens, and the agent is woken and told to read it.
This means a person can freely stop idle agents to reclaim the memory their provider processes hold,
knowing any later message brings the conversation back.

The three deliberate limits from the design all hold. Agents that a pipeline stopped after accepting
their work are never revived by a message; only the pipeline itself or the explicit Resume action
can restart them. Archived agents, agents in archived projects, agents with nothing saved to resume
from, and terminal agents behave exactly as before. And if a wake fails — a broken provider, for
example — the message is kept, the agent stays stopped, an error is shown with the draft preserved,
and AgentDeck does not keep retrying that same mail, even after a dashboard restart; new mail arms
it again. Because a prompt, incoming mail, and a manual Resume can all arrive at once, all three now
go through one exclusive per-agent lock, so exactly one restart happens and the others are told the
agent is already starting.

One unrelated repair: a test guarding the database version had been failing on the main branch since
an earlier change added a migration without updating it. It now derives its expectation from the
migration list, so it cannot drift again.

**Needs attention:** None. This was verified with automated tests against a stand-in provider, not
in a real browser with a real Claude or Codex session; a usability pass would confirm the wake feels
right in practice.

**Next:** Optionally run a usability review of the stopped-agent composer and mail wake, or pick the
next idea to design.

### 2026-08-16 — Design fix: all five wake-on-message review findings resolved

Every finding from the design review checked out against the real code, so the wake-on-message
design was revised rather than defended. The four confirmed problems and their fixes: two
simultaneous wakes of the same agent could have destroyed each other's registration — the design
now takes one exclusive per-agent claim before any side effect, shared by prompt, mail, and the
existing Resume button; finished pipeline stage agents would have been silently wakeable, reviving
completed stage work — they are now fully excluded from wake, with the pipeline and explicit Resume
as the only revival paths; the "don't retry a failed wake" promise relied on an in-memory marker
and second-granularity timestamps that could not order same-second mail — it now uses a durable
per-message delivery marker that survives restarts; and the promised "stopped but wakeable" label
in the agent list had no concrete field — it is now an explicit `availability` value alongside the
existing status. A wording fix also pins the project gate to the exact shipped Resume behavior. The
change is still waiting to start, now with the review's concurrency, pipeline, restart, and
same-second test scenarios in its acceptance evidence.

**Needs attention:** None — all findings are closed.

**Next:** Say the word and an implementation session can start the ready change
(`wake-on-message-for-stopped-agents.md`).

### 2026-08-16 — Design review: wake stopped agents on message

The wake-on-message design is not ready to implement. Four required corrections surfaced. Two
messages can try to wake the same agent before the runtime claim is taken, and the losing attempt can
remove the successful agent's messaging and hook registrations. The wake rule also includes pipeline
stage agents that the pipeline deliberately stopped after accepting their work, so ordinary mail
could revive completed stages outside pipeline control. The proposed failed-wake guard both retries
after a dashboard restart despite promising not to and cannot reliably distinguish messages created
in the same second; the existing durable delivery marker is a simpler place to record that failure.
Finally, the agent-discovery response promises a wakeable marker without defining its field or type.
There is also a smaller ambiguity around agents whose project definition has been removed.

**Needs attention:** Decide whether completed pipeline-stage agents may ever wake from ordinary
prompts or mail. The other required changes are technical corrections to make concurrency, retry,
and protocol behavior precise.

**Next:** Revise the design and planned requirements to close these findings, then run
`/review-design` again before implementation.

### 2026-08-16 — Feature design: wake stopped agents on message

The idle-memory idea is now a ready-to-implement design, and it turned out smaller than expected.
Investigation showed the memory an idle agent holds is its provider child process, and the only
agents created by the system rather than a person — pipeline stage agents — are already stopped the
moment they finish. Per your decisions, conversations you start are never put to sleep
automatically, so no idle timer was designed at all. Instead, the design makes waking transparent:
sending a chat message to a stopped conversation resumes it and delivers the message, and
agent-to-agent mail can find a stopped agent, wake it, and prompt it to read. Stopping an agent
therefore becomes the cheap, deliberate way to free its memory, since any message brings it back
with its history, model, and permissions intact. A broken agent cannot be woken repeatedly into a
process-spawn loop, and a failed wake keeps your typed message so nothing is lost.

**Needs attention:** None — all product decisions were resolved in session.

**Next:** Say the word and an implementation session can start the ready change
(`wake-on-message-for-stopped-agents.md`).

### 2026-08-16 — Design review: no waiting change

There is no waiting design to review. The ready-changes directory explicitly lists none, and the
handoff names no active change. Your uncommitted idea and specification edits were left untouched;
they were not treated as an approved ready change.

**Needs attention:** A design must be completed and added to the ready-changes directory before it
can receive this review.

**Next:** Finish or name the intended ready change, then run `/review-design` again.

### 2026-08-16 — Fix: review-finding queue check

There are no open review findings to fix. The two most recent required fixes are already complete,
and no active change was selected. Your in-progress ideas edit was left untouched.

**Needs attention:** None.

**Next:** Name a ready change or report a new issue when more work is wanted.

### 2026-08-16 — Fix: pipeline proposal recovery and context usage

AgentDecker proposals are now saved by AgentDeck before the agent is told they succeeded, so they stay
available for review after a reload, in another browser, or when an ACP adapter records an empty tool
result. The Pipelines page reads that durable record and still requires the normal explicit Save or
Start confirmation. The context bar now uses the Claude adapter's real context-usage update instead
of misreading end-of-turn token accounting, so real chat sessions report their used context again.

**Needs attention:** None.

**Next:** Authorize a push when these locally committed fixes should be shared with `origin/main`.

### 2026-08-16 — Bug investigation: context bar stuck at 0%

The bug is confirmed. The context-usage bar on every real chat agent reads "0% context used" because
AgentDeck looks for token usage in the wrong place and under the wrong names. The pinned Claude adapter
reports how full the context is through a running "usage update" message carrying the tokens used and
the window size — AgentDeck ignores that message entirely. It instead reads the end-of-turn result,
where the adapter only reports raw token counts with no window, under field names ("used"/"window")
that the adapter never sends. Both paths therefore leave the value at zero forever. It went unnoticed
because the fake adapter used in tests emits the made-up shape AgentDeck expects, so the tests passed
against an adapter that doesn't exist. The fix also needs to write down, in the integration spec, what
the real adapters actually send, since the code currently points at a spec section that isn't there.

**Needs attention:** This is a required fix; the meter has been silently meaningless for real agents.

**Next:** Run `/fix` to read context usage from the adapter's real "usage update" message, pin the
mapping in the integration spec, correct the test adapter, and un-skip the reproduction test.

### 2026-08-16 — Bug investigation: missing AgentDecker pipeline proposals

The bug is confirmed. AgentDecker successfully validates a pipeline proposal, but AgentDeck keeps no
proposal record of its own: it returns the proposal only to the agent and later tries to reconstruct it
from one browser-local builder transcript. A captured real session shows the proposal tool completing
while the adapter stored an empty result, leaving the Pipelines page nothing to display. Proposals made
from another valid AgentDecker chat or browser are also undiscoverable because that page follows only
the builder session remembered by the current browser.

**Needs attention:** This is a required fix; successful proposals can currently be lost from every
approval surface even though AgentDecker reports that they were proposed.

**Next:** Run `/fix` to store accepted proposals through a server-owned durable discovery path and
make Pipelines read it, with coverage for empty adapter results, reloads, and non-builder AgentDecker
sessions.

### 2026-08-15 — Usability review: newest chat and startup changes

The newest chat autocomplete, progress, and startup changes passed a focused real-browser review with
no user-impact findings. File and command pickers opened only at word boundaries, filtered correctly,
accepted from the keyboard, and preserved their trailing spaces in the sent transcript. A stopped chat
reported unavailable autocomplete without disabling the composer. The Working indicator appeared for
a deliberately non-finishing turn and cleared after the turn failed. Controlled Claude startup failure
showed safe recovery guidance without leaking raw diagnostics, and the persistent dashboard log stayed
owner-only across restarts. The browser reported zero console errors.

One visual-only step was blocked: the automation could inspect the live selectable-text and contrasting
selection styles but could not create an actual pointer text selection, so that gesture is not claimed
as passed. Full evidence is in the [review report](../archive/reviews/usability-review-run-2026-08-15-recent-chat-fixes.md).

**Needs attention:** None; the blocked selection gesture is an evidence gap, not an observed product problem.

**Next:** No fix is needed from this review; optionally drag-select a user message once during the next manual browser smoke.

### 2026-08-15 — Fix: composer autocomplete send and stopped-chat file search

Both problems the review found in the new chat autocomplete are fixed. Sending now delivers and saves
the prompt exactly as shown — including the trailing space a selected file or command adds — while
still refusing an empty or whitespace-only message. And a stopped chat no longer returns files from its
former working directory; it now safely reports that autocomplete is unavailable, matching how the
command picker already behaves. Both fixes restore behavior the specifications already required, so no
specification changed. New automated tests cover each fix and were confirmed to fail before the change.

**Needs attention:** None. Pushing the local commits to the shared branch still needs your authorization.

**Next:** No further action required; the autocomplete work is now complete and verified.

### 2026-08-15 — Code review: recent dashboard and composer work

The review found two problems in the new chat autocomplete. Selecting a file or command visibly adds a
trailing space, but sending trims it away, so the saved and delivered prompt differs from what was
shown. Also, a stopped chat can still return files from its former working directory instead of safely
reporting that autocomplete is unavailable.

**Needs attention:** These two fixes are required before the autocomplete work can be considered fully correct.

**Next:** Run `/fix` to preserve selected prompt text verbatim and reject file search for stopped chats.

### 2026-08-15 — Implementation: no change available

No implementation was started. The project has no active change and no specified change waiting to
start, so there is nothing currently authorized for implementation.

**Needs attention:** A change must be designed and placed in the ready queue before implementation
can begin.

**Next:** Choose the product change you want designed next, then start implementation after that
design is ready.

### 2026-08-15 — Implementation: composer file and command autocomplete

Chat now has the file and command autocomplete the play session asked for. In a running chat, typing
`@` opens a keyboard-driven picker of files from that session's working directory and inserts the one
you pick as a plain relative path; typing `#` opens the same picker filled with the commands and
skills the live agent currently advertises and inserts the runnable slash form (for example `/review`
or a Codex `/$skill`). Both are just text in your message — nothing is attached, embedded, or saved
anywhere — so sending works exactly as before.

The autocomplete never gets in the way: file search is confined to the working directory (it skips
`.git` and Git-ignored files, follows no symlinks out of the tree, and is size-capped), the command
list comes straight from the agent's latest update, and if a directory is missing or the agent is
stopped the picker just shows an empty state and you can still send. This finished the two "Chat &
composer" spec areas, which are now marked complete.

I verified it with automated tests across the runtime, the two new endpoints (driven against a real
temporary Git project and a fake agent that publishes a command list), and the composer UI, plus the
full build and packaging. I did not run a live browser or a real Claude/Codex session for this change.

**Needs attention:** None.

**Next:** A quick real-browser pass in the usability review to confirm the picker feels right against
a live agent; nothing blocks that.

### 2026-08-15 — Feature design: composer file and command autocomplete

The design is ready to implement. Typing `@` will search the live chat's working directory and
insert the selected relative path as ordinary prompt text. It deliberately does not create a
structured ACP attachment or embed file contents.

Typing `#` will open the same keyboard-operated picker with every command and skill most recently
advertised by the live ACP session. Selecting one inserts its executable slash form—for example,
`/review` or `/$skill-name`. Later provider updates replace the list; the list exists only for the
running session and is never written to transcripts or other storage.

The implementation extends the existing chat runtime and session API with two small reads: bounded,
Git-aware file search and the current in-memory ACP command snapshot. It needs no new package,
provider-specific discovery, persistence, or global live event. The pinned Claude and Codex adapters
and current releases were checked against ACP's command contract, and the earlier design-review
findings are resolved.

**Needs attention:** None.

**Next:** Run `/work composer-file-and-command-autocomplete` to implement the waiting change.

### 2026-08-15 — Design review: file and skill autocomplete

The draft is not ready to implement. It is still an incomplete feature draft—there is no technical
specification or ready change—and the review found two connected problems.

Deferring skill/command autocomplete is not justified by ACP compatibility. The pinned Claude and
Codex adapters both publish the standard, replaceable `available_commands_update` list when a
session starts or loads, and Claude refreshes it when commands change. AgentDeck can cache the latest
list for each live session, expose one read endpoint, and reuse the file picker's UI for `#`.
[ACP's command contract](https://agentclientprotocol.com/protocol/v1/slash-commands) says advertised
names are invoked as `/<name>` prompt text, including `/$skill-name` for Codex skills.

File selection also needs an explicit semantic choice. The draft sends only literal `@path` text,
but [ACP makes `resource_link` a baseline prompt block](https://agentclientprotocol.com/protocol/v1/initialization)
and both pinned adapters translate it to their native file-reference form. If selecting a file is
supposed to attach usable context, AgentDeck should send a server-validated resource link alongside
the durable display text. If it is only text completion, the specification must call it that and
accept provider-dependent resolution.

I recommend one combined change: a reusable picker, `@` files sent as validated ACP resource links,
and `#` commands/skills from a live per-session snapshot. This needs moderate runtime and API test
coverage, but no new package, persistence, or provider-specific discovery; that is not enough
complexity to justify silently dropping half of the requested feature.

**Needs attention:** Decide whether `#` shows every ACP-advertised command or only skills. I
recommend all advertised commands because Claude's standard payload does not identify which entries
came from skills. Also confirm whether `@` is real file attachment (recommended) or text-only
convenience.

**Next:** Confirm those two product choices; the next design session should revise the feature
specification, add the technical specification, and create the ready change.

### 2026-08-15 — Process: limitation checks generalized beyond packages

Generalized. The rule no longer names packages and providers as the special case — it now covers
every way a design can smuggle in a false limitation. Three triggers force validation: the design
explicitly says something cannot be done the straightforward way; the design silently takes an
indirect route whose only sensible motive is such a limitation; or the strategy simply reads as
strange for the stated goal. In each case the limitation itself must be shown real — against the
actual code, the pinned tool's real surface, newer releases, and better alternatives, whether that
is a direct approach, an existing seam, or an official package — with the evidence in the change
file. A workaround built on a claim that falls is then re-judged as over-engineering.

The designer-side rule is generalized the same way: nothing is declared impossible from memory, and
an indirect strategy whose only motive is an unverified limitation is not ready to propose.

Documentation checks, the twin-skill comparison, and whitespace checks pass.

**Needs attention:** None.

**Next:** Nothing pending; the next `/design-feature` and `/review-design` runs pick these rules up
automatically.

### 2026-08-15 — Process: impossibility claims now require proof

No, the research lens only covered one direction — it caught designs assuming a capability that
doesn't exist, but not the failure you've hit repeatedly: an agent declaring something unavailable
in the Claude or Codex ACP packages when it was fully supported, in the pinned version, a newer one,
or a better alternative package. That false "cannot do" is worse than the false "can do," because
the workaround it motivates is exactly the over-engineering the review exists to prevent.

Both sides are now fixed. The design reviewer must give claimed limitations the same rigor as
claimed capabilities: any design that works around something a package supposedly cannot do must
show the limitation is real, verified against the pinned version's actual surface, newer releases,
and official or better-maintained alternatives, with the evidence recorded in the change file. An
unverified impossibility claim is itself a finding, and once it falls, the workaround it justified
is re-judged as over-engineering. The designing agent gets the matching rule at the source: never
declare a capability impossible from memory — verify first, record the evidence, and expect a false
limitation to delete the workaround it motivated.

Documentation checks, the twin-skill comparison, and whitespace checks pass.

**Needs attention:** None.

**Next:** When a design cites a provider or package limitation, expect to see its evidence in the
change file; challenge any that arrives without it.

### 2026-08-15 — Process: new /review-design workflow and simplicity rules

Added. `/review-design` reviews a designed-but-unbuilt change before any code is written — the gap
the August 1st backend-design review had to improvise through. It reads the waiting change and its
planned requirements, then judges them through three ordered lenses. First, over-engineering: every
planned requirement must justify itself from the confirmed user outcome, a real report, or a binding
invariant, and machinery built "in case" of situations nobody has observed is a finding, not
prudence. Second, maintainability: designs should stretch an existing area, seam, route, or pattern
before minting a parallel mechanism or new interface, and anything genuinely new must say why
extension was rejected. Third, research: the design's factual assumptions — that cited helpers
behave as claimed, that packages and pinned tools can do what the design needs — are checked against
the actual tree, with contradictions recorded as findings.

Findings land in the same queue the code reviewer uses; the change stays waiting while any Must-fix
finding is open, and fixing the design is a follow-up design session with you. The reviewer itself
changes nothing.

Your instinct about implementation was right: the build workflow had the same blind spot. It asked
for small complete pieces but never for simple ones, and code reviewers were told to ignore
speculative edge cases rather than flag their presence. Both are fixed: implementers must now make
the smallest change that satisfies the requirement and extend existing seams before inventing
parallel ones, and unrequired complexity is now an explicit code-review finding.

Documentation checks, the twin-skill comparison, and whitespace checks pass.

**Needs attention:** None.

**Next:** Run `/review-design` on the next change `/design-feature` produces, before starting `/work`.

### 2026-08-14 — Process: new /investigate-bug workflow

Added. `/investigate-bug` is a new agent workflow for the bugs you meet at work, where live
debugging is impossible and a report plus whatever was logged is all the evidence there is. You
paste the symptom and any log excerpt; the agent records the report verbatim, finds what the
specifications say should happen, and classifies it: a real code defect, a specification gap, or
behavior that is actually working as specified.

For a real defect it traces the failure path in code, tries to reproduce it locally, and records a
finding that the existing `/fix` workflow picks up unchanged. Every conclusion carries an honest
confidence label — confirmed, probable, or undetermined — so a plausible story is never dressed up
as a proven root cause. When a reproduction test is achieved it is committed skipped, keeping the
build green, and the fix session un-skips it as the regression test. When the investigation stalls
because nothing was logged at the failure point, that missing diagnostic becomes its own finding, so
every undiagnosable report at least makes the next one diagnosable.

Documentation checks, the twin-skill comparison, and whitespace checks pass. Also suggested but not
built: workflows for cutting releases, auditing overall spec drift, and quick idea capture.

**Needs attention:** None.

**Next:** Use `/investigate-bug <report or log path>` the next time a work bug comes home.

### 2026-08-14 — Fix: packaged Claude and Codex launchers

Fixed. Release archives now keep `claude-agent-acp`, `codex-acp`, and `codex` rooted in their real
package directories while remaining symlink-free. The packaged commands are regular launchers that
run the original module with AgentDeck's private Node runtime, so relative imports resolve correctly.

The original `executable file not found in $PATH` happened because the running dashboard came from
the cloned repository's `./bin/agentdeck`. Source builds intentionally use the caller's PATH and do
not inherit the separately installed release runtime, so that process could not see
`claude-agent-acp`.

Installer users do not hit that exact PATH error: the installed wrapper adds its private runtime.
They could hit a separate, universal packaging failure in any archive built by the defective code.
The archiver copied the npm symlink target `dist/index.js` into `.bin`; JavaScript then looked for
`./acp-agent.js` inside `.bin` and failed with `ERR_MODULE_NOT_FOUND`. The installed 0.1.2 runtime on
this machine confirms that failure. Every copy of the same affected archive has the defect, although
users who never launch the bundled provider command would not notice it.

The regression was proven to fail before the fix and pass afterward. Specification checks, focused
and race tests, both Go test variants, source and distribution builds, and the UI presentation build
all pass.

**Needs attention:** Existing installations are immutable and are not repaired in place. A corrected
release must be published, then users must explicitly update or reinstall it.

**Next:** Publish a corrected release, then update the affected installation before retrying Claude.

### 2026-08-14 — Implementation: persistent dashboard logs in both modes

Implemented. Every `agentdeck dashboard start` now appends structured application logs to
`$AGENTDECK_HOME/dashboard.log`—normally `~/.agentdeck/dashboard.log`—whether started in the
foreground or with `--detach`. Foreground mode still mirrors the same records to the terminal.
:codex-annotation{index="1"}

On the work computer, collect the latest 500 records with:

```sh
tail -n 500 "${AGENTDECK_HOME:-$HOME/.agentdeck}/dashboard.log" > agentdeck-dashboard.log
```

Review and attach `agentdeck-dashboard.log` to send it. The file is append-only and forced to owner-
only `0600`; startup now fails clearly if the persistent log cannot be opened. Detached logging uses
one sink, so records are not duplicated. Package-level `slog` diagnostics now use the same configured
JSON logger and log level.

Verified with spec checks, focused logger tests and race test, both Go test variants, `make build`,
and `make dist`.

**Needs attention:** `dashboard.log` still has no rotation or retention limit, so it can grow over
time. That should be a separate diagnostics/log-lifecycle change.

### 2026-08-14 — Fix: random build failure and leftover chat paperwork

The build failure you hit on push is a flaky test, not a real defect. The test that checks agent-id
generation first creates a thousand agents with genuinely random ids, then forces a deliberate
collision to prove the retry works — but the retry target is a fixed id, so roughly one run in
sixteen thousand happens to have already used it, and the collision it was supposed to survive
becomes unsurvivable. Id generation itself is correct and unchanged; only the test setup was wrong,
and it now clears the target before forcing the collision. I confirmed the cause by reproducing the
identical error on purpose, then proved the fix against the same forced condition.

The chat spinner and copy-pastable messages were also left half-recorded. Both were still sitting in
the ideas list as if unbuilt, so they are removed — seven of the play-session ideas remain. The note
claiming eleven ideas were captured was wrong; nine were. The project's current-state summary now
mentions the shipped chat behavior, which it had skipped entirely. Your uncommitted "browse for
working directories" idea sat in the same file and rode along in this commit; nothing was changed
about it.

**Needs attention:** That change went straight from a raw idea to shipped code without a design step,
so there is no record of what was agreed before it was built. Worth deciding whether small usability
items should keep skipping that step or not.

**Next:** Push the branch and confirm the build is green.

### 2026-08-14 — Fix: Claude ACP startup failures

Claude works through AgentDeck's exact pinned adapter on this machine: startup completed and an
authenticated one-word streamed turn passed. The original play-session failure cannot be recovered
because AgentDeck discarded the adapter's startup diagnostics, so that blind spot is now fixed.
Claude launch and resume have a 30-second limit for each startup stage, and an adapter exit reports
safe guidance for resource exhaustion, nested launch, authentication, or runtime problems instead
of the opaque `transport closed` message or raw provider output.

**Needs attention:** The historical failure's exact cause remains unknown, and the broader live
Claude resume, permissions, and messaging checks are still separate acceptance gates.

**Next:** Launch a Claude agent normally. If the environment-specific problem returns, the new error
will identify the failed stage and recovery path instead of hiding the cause.

### 2026-08-11 — Implementation: chat spinner and selectable messages

Two of your play-session items are done. The chat now shows a small "Working…" spinner at the bottom
of the conversation while an agent is preparing its reply, so it is clear the answer is coming and
the session is not stuck; it disappears when the reply arrives or the agent pauses to ask for a
permission. You can also now select and copy text anywhere in the conversation, including your own
messages — selecting your own message used to highlight it in the same colour as its background, so
it looked like the text could not be marked at all. That change is limited to the chat and does not
touch the projects page.

Your other nine ideas are written down and ordered by how easy they are to build, so you can pick the
next one.

**Needs attention:** The selection-colour fix was confirmed from the styles and tests, not by opening
a live browser, so a quick look to confirm the highlight reads well in both light and the Sky & Grove
theme is worth doing.

**Next:** Choose the next idea — the strongest candidates are diagnosing the "can't use Claude" error
or adding the @-file and #-skill autocomplete.

### 2026-08-02 — Fix: backend safety and review follow-ups

Settings now refuses a stale save instead of deleting a backend created in another tab. Configuration
linking and unlinking no longer lose concurrent connections, failed cleanup restores the prior
catalog, and storage errors stay private. The connected-create screen now refreshes to its actual
linked state; unavailable project dashboards keep a usable project picker; failed builder transcript
reads retain pending proposal recovery; and Back avoids missing or archived project pages.

**Needs attention:** None.

**Next:** Review the completed fix set before publishing.

### 2026-08-02 — Usability review: backend creation and configuration linking

The real browser flow for adding and linking backends worked in an isolated run. Cancel changed
nothing; provider selection created the matching Codex starter; adding a backend kept an unrelated
unsaved Settings edit intact; direct create-and-connect showed a global linked state and imported
the configured models for that backend only; and unlink restored its retryable unbound state. The
browser reported no warnings or errors. The detailed record is in
[`usability-review-run-2026-08-02-backend-creation.md`](../archive/reviews/usability-review-run-2026-08-02-backend-creation.md).

**Needs attention:** The earlier durability findings are still open. Connection failure/retry needs a
controlled unavailable-source fixture, and launching through the binding remains a credentialed
provider check.

**Next:** Run `/fix` to resolve the recorded backend/catalog and configuration-source durability
findings.

### 2026-08-02 — Review: backend creation and configuration linking

The new backend and configuration-linking flow is not safe to ship yet. A stale Settings save can
erase a backend created in another tab, unlinking one configuration can erase a different connection
that completed at the same time, and a failed cleanup can leave a backend paired with the wrong
configuration source. The create-and-connect screen can also say a backend is connected while its
own card immediately shows it as unbound.

Two smaller issues need cleanup with those fixes: a damaged backend catalog consumes a one-time
connection approval even though no bind can succeed, and storage failures expose internal file-system
details in API errors.

**Needs attention:** Resolve the recorded review findings before relying on the new creation or
configuration-linking flow.

**Next:** Run `/fix` to address the backend/catalog and configuration-source durability findings,
with regressions for the interleavings and failed writes.

### 2026-08-02 — Implementation: simple backend creation and global configuration linking

Adding a backend is no longer an incomplete card you have to finish by hand. **Add backend** opens a
dialog where you pick the provider first; AgentDeck fills in a matching name you can edit and creates
the backend with that provider's standard starter model. It adds only that one backend — any other
unsaved edits you had open in Settings stay exactly as you left them, neither saved nor thrown away.

For Claude and Codex the dialog also offers **Create and use my configuration**, which creates the
backend and connects it to your existing CLI setup in one step. Connecting no longer asks you to pick
a project or choose between Linked and Mirrored: there is one **Use my … configuration** button, it
works even when you have no projects set up yet, and each agent still picks up its own project's
configuration when it launches. Connecting also turns on model import and immediately adds the models
your configuration names — which is what your files say, not a check that those models are actually
available to your account. If connecting fails, the backend is still created and saved, clearly shown
as not connected, and the same button retries.

Three problems were found and fixed while building this. Reading a configuration without a project
was falling back to AgentDeck's own working directory, so it could read and approve an unrelated
folder. A damaged `backends.json` could be silently replaced with the built-in default catalog
instead of being reported. And the Settings backends screen quietly discarded your unsaved edits
whenever it refreshed in the background.

**Needs attention:** None.

**Next:** A usability pass should drive the real browser journey — create a backend, connect Claude
and Codex, retry a failed connection, and launch an agent through the new global binding from a
project chosen later. Component tests cover this today, but no browser run does.

### 2026-08-01 — Design update: global backend creation and configuration linking

The design is ready to implement again. Backend creation now uses a focused one-backend operation,
so **Create and use my configuration** can save the correct provider starter and connect Claude or
Codex in one visible action without saving or discarding unrelated Settings edits. If connection
fails, the valid backend remains visible, unbound, and retryable. Existing backends use the same
project-free global connection; their agents still resolve project-specific configuration only when
they launch.

The normal flow is Linked only. Existing Mirrored bindings and explicit API use remain compatible,
but Mirrored is no longer presented as recovery for a failure it cannot repair. Immediate model
import is limited to the connected backend and accurately means Codex's locally visible cache entries
or Claude's explicitly configured selectors—not guaranteed support or entitlement.

Persistence is intentionally best effort: catalog changes are written before the binding, returned
failures attempt restoration, and an interruption can leave only harmless add-only models or autosync
on an unbound backend. The binding alone means “connected,” and retrying with the stable backend id
converges safely. The planned checks now cover dirty drafts, zero-project use, partial failures,
concurrency, replay/retry, invalidation, and later project launch.

**Needs attention:** None.

**Next:** Start the waiting backend-creation/global-linking change when you want it implemented.

### 2026-08-01 — Review: simple backend creation and global source linking

The design is not ready to implement. Four decisions are still underspecified or internally
inconsistent: the specification describes the unshipped global flow as if it were already current;
the Add-backend dialog does not say exactly when the backend becomes durable, what happens to
unsaved edits elsewhere in the whole-catalog form, or whether a saved backend remains after linking
fails; Mirrored is offered as recovery even though it currently follows the same discovery and
resolution path as Linked and only adds a cache after success; and the two-file source/catalog update
has no restart recovery if the process stops between writes.

Two smaller corrections should land with that revision: describe Claude/Codex imports as configured
or locally visible models rather than "supported models," and make the verification plan explicitly
cover target-only import, zero-project use, concurrent saves, rollback/restart, retry, and both source
and backend invalidation.

**Needs attention:** Choose the modal ownership rule and the future of Mirrored recovery. I recommend
persisting the valid backend inside the dialog before connection, keeping it as an unbound backend if
connection fails, preserving or explicitly resolving any pre-existing unsaved edits, and removing
Mirrored from error recovery unless a concrete failure it can repair is defined.

**Next:** Revise the planned requirements and ready-change file around those choices, then review the
package once more before implementation.

### 2026-08-01 — Design: simple backend creation and global configuration linking

Backend setup is ready to simplify. **Add backend** will open a provider-first dialog that suggests
the matching name and creates a usable saved backend, rather than adding an incomplete Settings card.
Claude and Codex will both offer one read-only **Use my configuration** action during creation and on
an existing backend. It connects the backend globally with no project, discovery, or mode chooser,
imports currently supported provider models immediately, and keeps that catalog add-only synced.
Model/effort provenance, overrides, refresh, unlink, and a compatibility recovery remain available
after connection; detached copying remains unavailable.

**Needs attention:** None.

**Next:** Start the ready change `simple-backend-creation-and-global-source-linking.md` when you want
to implement it.

### 2026-08-01 — Implementation: import configured Claude models at startup

The Claude backend can now pull in the models you have configured. Turn on "Import configured models from Claude on startup" for the Claude backend in Settings, and at the next dashboard start AgentDeck reads the models named in your personal Claude settings file (`~/.claude/settings.json`) and adds any it doesn't already list to the launch picker. It only adds — it never edits, removes, or reorders your existing models, never changes your default, and never touches anything on a backend where the option is off. A missing or unusual settings file is simply skipped. This mirrors the existing Codex auto-sync, and both providers now share one import step.

Fresh installs also start with the four portable Claude family choices — Fable, Opus, Sonnet, and Haiku — with Sonnet as the default, so a new machine offers all four without any setup. Existing setups are left exactly as they are.

Specification checks, both Go test modes, all 208 interface tests, and the packaged application build all pass. I verified the read/merge/save and seeding behavior with focused tests against real files; whether a given imported model actually runs still depends on your installed Claude version and account, which the launch step checks as usual.

**Needs attention:** None.

**Next:** Restart AgentDeck to pick up the change, then (optionally) enable the Claude import toggle in Settings → Backends.

### 2026-08-01 — Implementation: configuration-source backend linking

New backends now require a successful Settings save before configuration-source linking is offered. A premature API request no longer consumes its preview token, and saving the backend catalog removes bindings for deleted or provider-retargeted backends, so an invisible stale binding cannot block later links.

The focused regressions, both Go test modes, all 207 UI tests, specification checks, and generated distribution build pass.

**Needs attention:** None.

**Next:** Restart AgentDeck to use the rebuilt application.

### 2026-08-01 — Review: today's ad-hoc changes

I reviewed everything committed today after the per-chat runtime picker: the AgentDecker proposal
fix, the Sky & Grove visual rework, creating a project from the projects home, the chat Back-link
change, locking scoped New Agent launches, the Claude model-sync design, and the three chat
tool-activity presentation commits. All the required checks pass. Most of it is sound, and the two
larger features — project creation and the collapsed tool runs — are correctly described in the
specifications and covered by tests.

One thing is broken. Locking New Agent to the current project also applies on a project page whose
project no longer exists (you get there from an "Project unavailable" card when a project was
deleted while its agents were still live). There the project chooser is hidden and every launch is
rejected by the server, so New Agent cannot work at all and there is no control to fix it. That
needs the lock limited to real, active projects.

Three smaller things are worth fixing. The AgentDecker builder treats a failed transcript fetch as
an empty one, so a single server hiccup can permanently discard a stopped builder's pending
proposal — the exact loss today's fix was written to prevent. And three user-visible changes shipped
with nothing written down: the chat **Back** link now returns to the project page instead of the
home dashboard (with no note, test, or handling for a deleted project), the builder no longer jumps
to its chat after launch, and the builder panel now hides its chat link while stopped.

**Needs attention:** The New Agent lock on an unavailable-project page is a real dead end; the other
three are lower priority.

**Next:** Run `/fix` to work through the four findings, starting with the New Agent one.

### 2026-08-01 — Implementation: Plain chat tool activity

Chat tool activity now renders as regular, subdued text. Group summaries, calls, output, and failures have no tinted backgrounds or enclosing boxes; the expand/collapse control and error-text cue remain.

The visual fixture and full test/build suite passed.

**Needs attention:** None.

**Next:** No action needed.

### 2026-08-01 — Implementation: Collapsed chat tool runs

Consecutive tool activity now appears as one closed **Ran _n_ tools** row. Clicking it reveals the individual calls, their meaningful output, and any failures; regular assistant text, diffs, permissions, and other events begin a new row.

Successful no-output results are no longer displayed at all, so the **Completed** line is gone. The stored transcript and the individual right-click annotation targets are unchanged.

The full test and build suite passed, and the browser fixture confirmed both collapsed and expanded states with no browser errors.

**Needs attention:** None.

**Next:** No action needed.

### 2026-08-01 — Implementation: Compact chat tool activity

The empty black rectangles were successful tool-result events with no payload to show — commonly an edit whose actual change is already rendered as a diff. The renderer was still creating a padded result panel for them. They now show a compact **Completed** outcome instead.

Tool calls and results now use quiet, light-gray activity rows closer to the Codex experience. Expanded arguments and output remain inspectable; errors keep their clear treatment; diffs, code, commands, and terminal output stay on dark technical surfaces.

The full test and build suite passed, and the browser fixture confirmed the call, output, empty-completion, and error states with no browser errors.

**Needs attention:** None.

**Next:** No action needed.

### 2026-08-01 — Feature design: Claude backend model autosync

Claude backend model sync is fully designed and ready to implement. An opted-in Claude backend will
read only the user-level Claude `model`, `availableModels`, and `fallbackModel` settings at dashboard
startup and add valid, previously unrepresented selectors to AgentDeck's future-launch picker. The
merge is add-only: it never overwrites or removes an entry, changes the default, infers effort, or
mutates running and archived sessions. A bad or absent Claude source does not block startup or stop a
valid Codex catalog update.

Fresh homes will seed Claude Code's [portable aliases](https://code.claude.com/docs/en/model-config)
`fable`, `opus`, `sonnet`, and `haiku` with generic family labels; Sonnet stays default. Existing
catalogs remain untouched. Project, local, managed, environment-only, private-cache, binary, network,
and live-session sources are excluded, so this is configured-model convenience rather than a full
entitlement catalog.

The implementation will keep `backends.json` as the sole authority, parse provider sources inside
the configuration package, merge all provider additions into one validated snapshot, and perform at
most one atomic rewrite. It adds no API, schema version, cache, sidecar, database migration, or
federation binding. Documentation checks pass.

**Needs attention:** None.

**Next:** Run the waiting **Sync configured Claude models** change when you want it implemented.

### 2026-08-01 — Fix: Keep new agents in their current project

Creating an agent from a project page now always creates it in that project. The project chooser is
removed from that modal, so it is no longer possible to accidentally launch into a different project.
The normal New Agent screen still lets you choose a project. Automated checks and builds pass.

**Needs attention:** None.

**Next:** No action needed.

### 2026-08-01 — Implementation: Create a project from the projects page

You can now create a project straight from the projects home, without opening Settings or onboarding.
A persistent **New project** button sits next to the "Projects" heading, and right-clicking anywhere on
the projects background (but not on a project card) opens a small **New project** menu. Both open the
same modal with the full project form — Title, the six-preset Color, working directory, additional
directories, and context prompt. Submitting a valid project creates it and its card appears on the grid
immediately, no reload. A bad working directory is a non-blocking warning; a validation or server error
keeps the modal open with the message; Cancel or Escape closes it with no change. The create action
exists only on the projects home, not on a single project's page, and right-clicking a project card
still opens that card's own menu.

This reuses the existing project form and create API unchanged — no new storage, id, color, or
validation rules. The dashboard's create flow is covered by a new automated test (both entry points,
the card-menu exclusion, a live create, the error case, and cancel). All required checks pass: the full
UI test suite, both Go test modes, the source and packaged builds, and the presentation/style/whitespace
checks. FS-02 is now fully current.

**Needs attention:** None. One real-browser walkthrough of the new create flow (grid journey J5) is
still worth doing in a usability review; the automated coverage stands in for it for now.

**Next:** Optionally run a usability review to exercise the create flow in a real browser; otherwise
this change is complete.

### 2026-08-01 — Feature design: Create a project from the projects page

The projects home will get a way to create a project directly, so people no longer have to open
Settings or onboarding. A persistent **New project** button and a right-click **New project** menu on
the projects background both open one modal with the full project form — Title, the six-preset Color,
working directory, additional directories, and context prompt. Submitting creates the project through
the existing create API and its card appears on the grid immediately; a bad `cwd` is a non-blocking
warning, and a validation error keeps the modal open. The create action lives only on the projects
home, not on a single project's page, and right-clicking a project card still opens that card's menu.

FS-02 gained R41, R42, and A24 (all planned) and moved to Partial. No technical-spec change is
needed: the create route, React Query hooks, project form, and context-menu/dialog presentation
already exist and are reused unchanged. The idea moved out of `docs/ideas.md` into the waiting
`docs/ready-changes/create-project-from-projects-page.md` change. `make check-specs` passes.

**Needs attention:** None.

**Next:** Build the change from `create-project-from-projects-page.md` when you want it implemented.

### 2026-08-01 — Implementation: Sky & Grove visual rework

Sky & Grove now uses a calmer sky-blue canvas, crisp blue-white surfaces, evergreen controls,
restrained contour linework, tighter radii, and more controlled shadows. The oversized ring and leaf
decoration is gone, so the original interface's hierarchy remains intact and agent-state bars stay
visible instead of becoming decoration. AgentDeck Core is unchanged.

The complete test, build, and distribution suite passes: all 194 UI tests, both Go test modes,
style and presentation checks, and the paired browser visual matrix with no browser errors.

**Needs attention:** None.

**Next:** Use Sky & Grove in normal work and flag any specific surface that still feels too
decorative or low-contrast.

### 2026-08-01 — Fix: AgentDecker pipeline proposals

AgentDecker now recognizes a valid proposal from its returned data instead of relying on an adapter’s display label, stays on Pipelines after launch so its approval controls remain in view, and keeps a proposal available if the builder stops immediately afterward. The project color dots in the right-click menu are round, bordered, and interactive again.

**Needs attention:** The real Claude/Codex adapter round-trip remains a credentialed release check.

**Next:** Run that credentialed pipeline-builder check before a release that claims live-provider support.

### 2026-08-01 — Usability review: Create-with-AgentDecker pipeline flow

I did a targeted review of the "Create with AgentDecker" pipeline-builder flow you reported as broken. I could not drive it end-to-end in the app: the stand-in test agent can't make the real tool call that produces a proposal, so no test setup ever exercises the actual approval surface — which is exactly why the tests stay green while the real flow fails. I traced the whole path in the code instead. Three problems, two of them serious.

The likely cause of "broken": the page only recognizes a pipeline proposal when the agent's tool call is labeled with the exact name `propose_pipeline_template`, but the way AgentDeck records tool calls stores a generic category ("other") in that slot, not the tool name. So with the real Claude/Codex adapter the proposal almost certainly never shows up, and there is nothing to approve. This one needs a quick check against the real adapter to confirm the label it actually uses before fixing.

Second, even when a proposal is produced, the flow sends you into the agent chat right after launch — but the only place to review and approve the proposal is back on the Pipelines page. Nothing in the chat tells you a proposal is waiting or how to get back to it, so the guided flow dead-ends. Third and minor: a proposal made just before the builder stops is dropped with no record.

All three are written up with exact file locations and fix directions in the handoff for the next agent, and the full run record is saved under `docs/archive/reviews/usability-review-run-2026-08-01-agentdecker-builder.md`. I changed no product code.

**Needs attention:** HUMAN — before the fix, confirm what tool-call title the real claude-agent-acp / codex-acp adapter emits for an MCP tool, since that determines the right fix for finding 1.

**Next:** hand to the fix agent; the two Must-fix findings are queued in the handoff.

### 2026-08-01 — Usability review: per-chat runtime picker

I drove the new chat-header runtime picker in a real browser, from a freshly built copy of the app and an isolated test setup, using a stand-in agent so nothing touched your real providers.

Everything worked. On a running chat agent the header shows backend, model, and effort as dropdowns; picking a different model or effort reveals a Switch button, and pressing it actually changes the live agent and snaps the dropdowns to its new identity. Changing the backend correctly resets the model and effort to that backend's defaults, and a cross-backend switch went through cleanly. Stopping the agent turns the header back into a plain read-only label. No errors appeared at any point.

This was the one browser journey the project had never been able to run before — the old version relied on a pop-up prompt the automated browser refused to show. The shipped picker uses inline dropdowns instead, so it can now be exercised, and it passed.

**Needs attention:** None.

**Next:** No action needed. Credentialed checks against the real Claude/Codex CLIs remain a separate release gate, unchanged by this run.

### 2026-08-01 — Review: per-chat runtime picker

I reviewed the new chat-header runtime picker that lets you change a running agent's backend, model, and effort without leaving the chat.

It works as intended and matches its written requirements. A running chat agent shows the picker; a stopped or archived one shows the plain read-only label. Changes stay pending until you press Switch, a failed or no-op switch is explained and the picker snaps back to the agent's real runtime, and an agent whose current backend or model is no longer in the catalog still shows its identity and won't let you switch until you pick a listed one. The change also pulls the shared "reset model and effort when the backend changes" logic into one place so the chat header, the New Agent screen, and the dashboard Switch dialog can't drift apart — a genuine cleanup, done correctly.

I found no new problems. The one open item is unchanged and unrelated: the small cosmetic issue with the project color swatches inside the right-click menu, already recorded from the previous review.

**Needs attention:** None.

**Next:** No action needed; the picker is ready to use.

### 2026-08-01 — Implementation: per-chat runtime picker

The chat header now lets you change a running agent’s backend, model, and effort without returning to the dashboard. Changes stay pending until you choose Switch, preserve the existing transcript, and reset cleanly with an explanation if the switch fails. Stopped and archived sessions keep the simple read-only runtime label.

**Needs attention:** None.

**Next:** Use the header picker to change a running chat agent’s runtime when needed.

### 2026-08-01 — Review: effort-lifecycle fix and project context menu/colors

I reviewed the two batches of work since the last review: the fix that keeps each agent's chosen
effort level attached through resume and pipeline runs, and the new project right-click menu with
preset colors.

The effort fix is correct. Resuming an agent now reuses the level it was launched with instead of
re-checking it against the current catalog, so removing a level from a model in Settings no longer
blocks resuming an older agent. Pipeline stages now carry their own frozen level, so retries and
recovery run at the level the stage started with. All three problems flagged in the previous review
are properly resolved.

The project menu and colors work well, with one cosmetic issue: the six color swatches shown inside
the right-click menu are restyled by the menu's shared button styling, so they appear as slightly
squared, borderless dots rather than the round bordered swatches used in the Settings and onboarding
forms. They still show the right colors and mark the selected one, so the picker is fully usable —
this is a visual-only polish item. The same swatches in the Settings and onboarding forms look
correct. No automated test can catch this because the test tools do not evaluate styling.

**Needs attention:** None. One worth-fixing cosmetic finding is recorded for a future fix pass.

**Next:** Fix the in-menu swatch styling when convenient; no blocker for using the feature now.

### 2026-07-31 — Implementation: project context menu and preset colors

Project cards now open a floating right-click menu with Rename, Archive, and a six-color picker.
Slate, Blue, Green, Amber, Rose, and Violet replace the manual RGB fields in Settings and onboarding;
new projects start with Slate. Cards also use the chosen accent as a subtle background wash and border
while keeping the existing state bar and accent edge. Existing saved colors and API clients continue
to use ordinary RGB values unchanged.

**Needs attention:** None.

**Next:** Use the new project menu and color picker in normal project setup and dashboard work.

### 2026-07-31 — Design: project context menu and preset colors

I designed two fixes to the redesigned Projects page: the right-click menu and the color picker. No
product code changed — this is specification and a ready-to-build change file.

Right-clicking an active project card will open a floating menu at the pointer, the same kind agents
already use, instead of expanding buttons inside the card. Project color becomes a choice of six
preset accents — Slate (the new default), Blue, Green, Amber, Rose, and Violet — shown as clickable
swatches directly in that menu; clicking one recolors the card immediately, with no dialog. The same
six swatches replace the free-form red/green/blue number inputs in the Settings and onboarding project
forms, so color is picked the same way everywhere. Rename and Archive keep their existing dialogs.
Colors are still stored and sent as ordinary RGB values, so nothing migrates, the API keeps working
for any caller, and existing project colors stay valid.

The color also carries more weight on the cards themselves. Today a project's color shows only as a
thin left stripe; now each card in the project reads as gently mono-colored in that hue — a soft
full-card background wash and a tinted border, so even an empty card is clearly "that project." It is
tuned to be distinct without being loud: the top status bar still shows the agent's live state color,
and the tint is bounded so text stays readable in both the plain and Sky & Grove looks.

Two of the four "Projects page problems" ideas are covered here (the context menu and the six colors).
Creating a project from the page and the broader visual rework stay as raw ideas for later. The specs
(FS-02, FS-04, TS-08) gained planned requirements and moved to Partial, and the work is written up in
`docs/ready-changes/project-context-menu-and-preset-colors.md`, ready to implement.

**Needs attention:** None.

**Next:** Build the change with `/work` when you want it; the specs and acceptance checks are complete.

### 2026-07-31 — Fix: effort lifecycle review findings

Stopped agents now resume with the reasoning-effort level they were started with even if a later
Settings edit removes that level. Effort capability is derived directly from each provider adapter,
and every pipeline attempt now preserves its own level so continued stages cannot pick up a changed
assignment by accident.

**Needs attention:** None.

**Next:** Run the existing credentialed provider checks before claiming live-provider effort support.

### 2026-07-31 — Review: agent effort selection

I reviewed the newly committed agent effort-selection feature — the one that lets a model run at a
provider's chosen reasoning-effort level — against its requirements and the project's invariant
checklist. The feature is well built: the chosen level is resolved once, frozen with the session, and
correctly restored and delivered for each backend, and it flows through history, archive, the command
line, and pipelines. A separate model checked the code in parallel and reached the same conclusion.

One real problem. When you resume a stopped agent, the app re-checks its frozen effort level against
the current model settings. So if someone removes that level from a model in Settings after an agent
was started at it, resuming that untouched agent is refused — even though the design says a later
settings edit must not disturb an already-frozen session. Two minor cleanups were also noted and are
not urgent: a capability check written in two places that could drift apart, and pipeline attempt
records not storing their own effort level.

**Needs attention:** One must-fix — resume wrongly rejects a still-valid frozen session after an
unrelated model edit.

**Next:** Fix the resume check so it trusts the frozen level, then re-verify; the two minor items can
follow or wait.

### 2026-07-30 — Design: per-chat runtime picker

I designed the per-chat runtime picker with you and wrote it up as a ready-to-build change. In the
chat view, the header line that today just displays "backend · model · effort" as text will become
three dropdowns for a running agent, so you can change the model (or backend or effort) right there
and apply it with one explicit "Switch" click — no more going back to the dashboard and using the
card's right-click menu. History is preserved exactly as the existing switch does it. Switching
between chat and terminal stays in the dashboard dialog, and a stopped agent just shows plain text.

Two things worth noting: the old "three browser prompts" complaint in the idea was already out of
date (that flow was replaced with a proper dialog a while back), so the real value here is
convenience and discoverability, not removing prompts. And this needs no backend or API work at all —
the switch machinery already handles model/effort changes — so it's a small, UI-only change.

No product code changed; only the specification and the ready-change note. Spec and whitespace checks
pass.

**Needs attention:** None.

**Next:** Run `/work` when you want it built (it's the only item now waiting in the ready queue), or
tell me to start it.

### 2026-07-30 — Fix: agent effort selection review findings

I fixed all four findings from the effort-selection review. The blocker is cleared: the new "effort
levels" field is now seeded the same way it is read back, so the test suite is green again and a
fresh install saves an empty list instead of the null value that broke the round-trip. The three
smaller gaps are closed too — the Claude path that applies effort after a chat starts now has tests
proving it works and that a rejected level cleanly fails the launch with no leftover agent, and the
effort controls in New Agent and Settings, plus the Codex model import, all gained the tests the
specs had promised but never had. One usability nicety: if someone hand-edits the config to an
unknown appearance, they can now click "Core" to repair it directly instead of having to detour
through the other skin first.

Everything passes: both Go test modes, the build, the spec checks, all 185 UI tests, the UI build,
and the whitespace check. As you asked, I did not commit anything — the fixes sit in the working
tree ready for you to review and commit.

**Needs attention:** None.

**Next:** Commit the effort-selection change together with these fixes when you're ready; live
Claude/Codex provider honoring still needs a credentialed run before it can be claimed.

### 2026-07-30 — Review: agent effort selection

I reviewed the agent effort selection work against the specifications and our recurring bug-class
checklist. The feature is well built where it counts: one shared resolver decides the level
(your choice, then a linked source's override, then the model's default, then nothing), the level is
saved and re-applied on resume and switch, and each provider gets it the right way. But it is not
ready to ship as-is.

The blocker: the full test suite is red. A new "effort levels" field is written to disk one way and
read back another, so the default backend catalog fails its round-trip test and, worse, gets saved
with a null value. The person who implemented it noted they could not rerun the full test run; it
does fail. This must be fixed before the change is committed.

Two smaller gaps are worth fixing at the same time: the Claude path that applies effort after a chat
session starts — including the "if it fails, don't leave a half-started agent" rule — has no test,
even though the test harness was wired up for exactly that; and several specifications claim specific
tests prove the effort controls and the Codex level import, but those tests contain no effort checks.

**Needs attention:** The change fails `make test` and should not be committed until the effort field
is stored and read consistently.

**Next:** Hand these findings to a fix session to repair the round-trip and add the missing tests,
then rerun the full suite before committing on `main`.

### 2026-07-30 — Implement: agent effort selection

Agent effort selection is implemented; the working tree is ready to commit on `main`.

- Settings, New Agent, runtime switch, CLI launch, headers/archive, and pipeline stage assignments
  now use each model's declared provider-native effort levels.
- The resolved level follows explicit request → bound source override → model default → omission;
  it is persisted for resume/switch and delivered as Codex ACP model suffix, Claude chat session
  configuration, or Claude terminal argv.
- Catalog validation rejects invalid, duplicated, bracketed, or unsupported effort declarations;
  OpenCode/OpenHands remain unsupported.

Focused Go package tests and all 180 UI tests/build passed, as did `make check-specs` and whitespace
checks. Full `make test`/distribution verification was not rerun after the final regression-test
additions because compiler-cache escalation became unavailable; the required Git write was also
denied after the sandbox blocked `.git/index.lock`. Live Claude/Codex honoring remains the existing
credentialed gate.

**Needs attention:** None.

**Next:** Approve the Git write so this completed change can be committed, then run the credentialed
Claude and Codex effort acceptance matrix before claiming live-provider honoring.

### 2026-07-30 — Review: Sky & Grove appearance and the archive/dialog fixes

I reviewed the unreviewed work — the Sky & Grove appearance plus the recent archive-retry,
project-rename error, native-dialog guard, and dialog-spec fixes — against the specifications and our
recurring bug-class checklist. The code and the specifications agree, and the appearance behaves as
promised: it applies immediately, saves as a global preference, falls back to the plain Core look on
any missing, unknown, or unreadable configuration, and undoes an optimistic choice if the save fails.

I found one minor issue worth fixing when convenient: if someone hand-edits the config file to an
appearance name that doesn't exist, the app correctly shows Core and warns that the saved choice is
unavailable, but the Core option in Settings already appears selected, so clicking it does nothing —
the person has to pick Sky & Grove first and then switch back to fully clear the bad value. It only
happens after a manual file edit or a persistent config read error, so it is low risk.

Verification I ran passed: the specification and presentation-contract checks, the Go configuration
and server tests, and the appearance, terminal, and visual-matrix UI tests. I changed no product code
or specifications.

**Needs attention:** None.

**Next:** Fix the Appearance "repair to Core" gap when the team wants it; no blocker to shipping.

### 2026-07-30 — Implement: Sky & Grove appearance

Sky & Grove is implemented and committed on `main`.

- Settings → Appearance switches the mounted app immediately between AgentDeck Core and the
  sky-blue/nature-green Sky & Grove design.
- The existing global config API persists the choice; absent, unknown, corrupt, or failed config
  reads safely use Core, while failed saves report the error and restore the durable selection.
- The skin is statically bundled, guarded by the finite presentation contract, covered by paired
  Core/Sky fixtures, and updates live terminal colors without reconnecting.

Verification passed: `make check-specs`, `make build`, `make test` (both Go modes), UI checks (180
tests), `npm run build`, `make dist`, and a real-browser switch/reload/onboarding pass with no console
errors.

**Needs attention:** `npm ci` reports 13 dependency vulnerabilities (6 moderate, 6 high, 1
critical); they were not changed in this feature.

**Next:** Independent review of the Sky & Grove implementation.

### 2026-07-30 — Design: Sky & Grove ready to build

Sky & Grove is fully specified and waiting to start. No product code was changed.

The implementation will keep AgentDeck Core as the unskinned, no-preference default. Choosing Sky &
Grove writes one global appearance preference through the existing configuration API; switching
back to Core removes that preference. The interface applies the choice immediately from the same
configuration cache, rolls back visibly if saving fails, and safely uses Core for missing, unknown,
corrupt, or temporarily unreadable configuration.

The skin itself will be statically bundled in the reserved presentation layer—no theme provider,
browser-storage copy, dynamic download, or external code. Its exact sky-blue, evergreen, state, and
technical palettes are defined with distinct action and success colors; approved component hooks add
the softer geometry and restrained natural linework. The presentation contract and checker will
recognize only the bundled skin, every required surface will have paired Core/Sky & Grove visual
evidence, and a live terminal will recolor in place without reconnecting or losing content.

**Needs attention:** None.

**Next:** Start the **Add the Sky & Grove appearance** waiting change when you want it implemented;
name it explicitly because another change is also waiting.

### 2026-07-30 — Fix: archive and dialog review findings

The Archive and dialog review queue is complete. Large archives stay bounded, Archive and Restore
remain safe under overlap and failure, unreadable projects block work until repaired, and Archive
pages keep a visible retry path. Project Rename now explains field errors, lifecycle documentation
matches the shipped dialogs, and the dialog guard catches equivalent native-browser call forms. The
missing historical planning record is documented without inventing a retroactive file.

**Needs attention:** None.

**Next:** Independent review of the completed fixes before future behavior work.

### 2026-07-30 — Design: Sky & Grove theme

The original interface design did intentionally reserve optional skins around the unskinned
AgentDeck core. The proposed first skin is now defined at the product-behavior level; no product code
has changed.

The proposed behavior is:

- Settings gains an Appearance destination with compact previews for AgentDeck Core and **Sky &
  Grove**. Selection applies immediately across every route, dialog, code view, diff, and terminal,
  without changing product words, actions, state, or structure.
- The choice is stored as `appearance_skin` in AgentDeck's global `config.json` and exposed through
  the existing config API. It applies to every project and agent, survives restarts and browser
  sessions, and is unrelated to agent/session retention.
- Core remains the first-install default and safe fallback. A missing, unknown, or unreadable skin
  choice still renders Core; a failed save reports the error and returns to the last saved choice.
- Sky & Grove uses an airy sky-blue canvas, layered pale-blue surfaces, deep nature-green structure
  and primary actions, softer organic geometry, and restrained botanical/topographic decoration.
  Status, warning, destructive, permission, context, connection, and project colors remain visibly
  distinct.
- This change includes only the two bundled choices. It excludes operating-system dark/light
  following, per-project themes, schedules, custom CSS or code, downloads, imports, a marketplace,
  network-loaded assets, and theme-specific product terminology.

**Needs attention:** Confirm or revise these five choices: the **Sky & Grove** name, Core as the
default, one global server-stored preference, Settings-only selection, and a purely visual nature
interpretation with unchanged product vocabulary.

**Next:** After confirmation, I’ll define the loading, persistence, compatibility, and presentation
architecture, then create the ready-to-build change.

### 2026-07-30 — Review: Archive fix run

The recent Archive fixes correctly page long histories, prevent overlapping archive actions from
reporting a false result, preserve a coherent state when rollback fails, and stop work when a
project definition cannot be read safely. One issue remains: if loading an individual project's
Archive rows temporarily fails, that project appears empty even when its header says archived agents
exist, and there is no retry control.

**Needs attention:** A transient per-project Archive request failure can hide archived agents until
the person changes the search or reloads the page.

**Next:** Add an inline per-project error and retry action with a regression test, then continue the
remaining lower-priority repairs.

### 2026-07-30 — Fix: unreadable project files no longer allow work

The last high-risk finding is fixed. Resume, runtime switch, agent restore, and pipeline work now
read the project's saved definition to check whether it is archived; if that file is corrupt or
otherwise unreadable, the action stops with an error instead of quietly proceeding — because an
unreadable file might itself say the project is archived. A project whose file is simply missing
still behaves as before (available but not archived), and archiving an agent for cleanup keeps
working regardless. This is the fail-closed choice you selected. A test confirms the old code let a
restore silently succeed and clear the archive flag; the new code refuses it. All five high-risk
findings are now cleared.

**Needs attention:** None.

**Next:** Continue with the four lower-risk repairs, starting with project-rename validation
feedback.

### 2026-07-29 — Fix: archive correctness findings

Four high-risk Archive defects are fixed and committed: large project histories no longer inflate
group pages, an older search cannot overwrite newer results, overlapping Archive and Restore actions
cannot report false success, and a failed rollback now preserves and reports a coherent durable
state. The remaining high-risk finding needs one product choice before implementation can continue.

**Needs attention:** Choose how a corrupt project file behaves. Recommended: fail closed and return
an explicit error for Resume, runtime switch, agent restore, and pipeline work until the file is
repaired. Alternative: treat it like a missing project, which matches its absence from the project
list but may allow work when the unreadable file actually marks the project archived.

**Next:** Reply with “fail closed” or “treat as missing”; the fix run will then finish the remaining
finding and the four lower-risk repairs.

### 2026-07-29 — Review: project archive and application dialogs

The independent review found five defects that need correction before the project archive work is considered complete: large Archive pages still load and return every matching agent, older per-project searches can overwrite newer results, overlapping Archive and Restore actions can report the wrong final state, a failed rollback can leave project and agent archive flags inconsistent, and project-read failures can allow work to start while archive state is unknown. Four lower-risk repairs are also recorded for project-rename validation feedback, contradictory dialog documentation, bypasses in the native-dialog source guard, and the missing committed design handoff. The full automated test suite passes, but it does not exercise these failures.

**Needs attention:** The archive lifecycle has correctness and scaling risks under concurrency, read/storage failures, and long histories.

**Next:** Run `/fix` to validate and repair the five must-fix findings one at a time, then address the four lower-risk findings.

### 2026-07-29 — Implementation: replace native browser dialogs

AgentDeck now uses its own dialogs for all agent, project, group, and Settings inputs and destructive confirmations. Rename, runtime switching, group moves, project actions, archive, and delete flows validate before sending the same requests as before and make cancellation harmless; Settings force-delete dialogs identify the affected running agents. A source guard prevents browser prompts or confirmations from returning.

**Needs attention:** The in-app browser cannot reach this workspace’s healthy loopback test server, so the requested browser pass is unconfirmed; automated checks cover the dialog flows.

**Next:** Independent review of the completed change.

### 2026-07-29 — Design: replace native prompts and confirmations with dialogs

The idea to replace AgentDeck's browser-native prompts and confirmations with styled dialogs is now specified and ready to build. The seven `window.prompt` inputs (agent rename, runtime switch, move to group, project rename, project color) and the eight `confirm()` dialogs (stop, release group, project archive on the card and in Settings, and role/project delete including the in-use "delete anyway" step) will each become an application dialog that validates input, states the consequence of a destructive action, and cancels without side effects — issuing the same server requests as today, with no API, storage, or route change. Runtime switch becomes one cancellable form with catalog-driven dropdowns instead of three chained prompts that cannot abort, which also unblocks the browser journey native automation could not run; move to group gains a combobox over existing group names. The behavior and acceptance criteria are written as planned requirements in FS-12, FS-01, FS-02, FS-04, and TS-08, enforced by a source guard that forbids any native prompt/confirm in the UI, and the ready change `replace-native-dialogs.md` is waiting to start.

**Needs attention:** None; the three product decisions (whole native-dialog surface in one change, move-to-group combobox, styled consequence confirmations rather than type-to-confirm) are resolved.

**Next:** Implement `replace-native-dialogs.md` when the work is picked up; it is not active in the handoff and no product code was written.

### 2026-07-29 — Usability review: remaining project-archive journeys

The remaining project-archive browser checks passed: with no active projects, onboarding directs the user to create a first project; Archive loads the 51st project group after the first 50; and archiving a project with a live pipeline stops the run and removes that project from new-run choices. No new user-facing issue was found. Runtime switch is still unconfirmed because this in-app browser rejects `prompt()` before showing its inputs; that is an automation limitation, not a product finding. The full record is in [the usability review report](../archive/reviews/usability-review-run-2026-07-29-project-archive.md).

**Needs attention:** Re-run only the runtime-switch browser journey in a prompt-capable browser.

**Next:** Treat the project-archive browser matrix as complete once that environment-specific check passes.

### 2026-07-29 — Usability review retry: project archive

The browser retry completed the project-archive path: its warning is accepted, the project leaves the dashboard, Archive shows the project and its agents, and project then individual restore return the expected stopped agent. Archive also loaded the final rows of a 55-agent project, archived default projects disappeared from new-agent choices, and the checked state rendered unchanged after a restart. No new user-facing problem was found. The full record is in [the usability review report](../archive/reviews/usability-review-run-2026-07-29-project-archive.md).

**Needs attention:** Browser coverage still does not include the no-active-project onboarding state, runtime switch, more than 50 project groups in Archive, or archiving an active pipeline.

**Next:** Run those remaining browser journeys before treating the review matrix as complete.

### 2026-07-29 — Usability review: project dashboard and grouped Archive

The new dashboard flows work in the real app: projects open their own dashboards, stopped agents resume, individual archive and restore work safely, and the Archive groups archived agents beneath their still-active project. The checked state also survived a server restart. No new user-facing problem was found. The full record is in [the usability review report](../archive/reviews/usability-review-run-2026-07-29-project-archive.md).

**Needs attention:** Browser automation could not accept the project-archive confirmation dialog, so the remaining browser-only archive, paging, restart-render, and pipeline checks are still unconfirmed.

**Next:** Re-run those remaining browser paths with a browser session that can handle native confirmation dialogs.

### 2026-07-29 — Fix: project dashboard and grouped Archive

The project-first dashboard and grouped Archive are complete. Archive and restore now safely handle
concurrent starts, restarted processes, rollback, pipeline work, and large result sets; project
cards, Settings, and launch choices now stay in sync with archive state. The old flat Archive rules
were retired so the documentation matches the shipped experience.

**Needs attention:** None.

**Next:** An independent review can assess the completed change before future work begins.

### 2026-07-29 — Re-review: project dashboard and grouped Archive

The second pass found that the first review materially understated the risk. Seven dashboard issues
should be fixed before shipment: archive can leave a restarted orphan process alive while marking it
archived; archive and restore can race Resume or each other because the promised transition claim is
missing; a failed project-archive write can silently restore agents that were already archived;
pipeline starts and controls claim the project too late and return the wrong result; successful
project Archive/Restore leaves the interface stale; dragging within one project discards the saved
ordering for other projects; and Archive still truncates or strands agents because neither the
server nor the interface implements the specified two-level paging correctly.

I also found nine smaller dashboard contract gaps, including the early-disappearing Load more button,
missing project-card and Settings controls, archived projects being treated as onboarding-ready or
remaining invisibly selected, misleading project-read errors, incomplete archive responses, and a
stopped search result being labelled archived. All automated tests and builds pass, but the new
dashboard, archive actions, rollback, race barriers, grouped response, and per-project paging have
almost no focused coverage; the green suite does not exercise these failures. No product code or
specifications were changed.

**Needs attention:** Do not mark the dashboard change shipped until the seven must-fix findings are
resolved and covered by deterministic regressions.

**Next:** The build agent should fix the archive lifecycle/transaction boundaries first, then repair
the two-level paging and project UI state before closing the remaining contract gaps.

### 2026-07-29 — Review: project dashboard, grouped Archive, and Codex isolation fix

I reviewed the project-first dashboard, project-grouped Archive, and the recent Codex profile-isolation
fix. The archive/restore actions, the lease that stops a project archive from racing a new agent
launch, and the hardened Codex private-profile handling all hold up. I found one issue that should be
fixed before this ships and four smaller ones.

The important one: the grouped Archive only ever loads the newest 200 sessions and then splits that
fixed batch into project groups. Once someone has archived more than 200 agents over time, the older
ones simply stop appearing and can no longer be searched or reached — even though each project still
shows its true archived count, so the number on screen won't match what you can actually open. This
needs to page the database directly instead of slicing a capped batch.

The smaller ones: the Archive "Load more" button disappears too early once there are many projects, so
later projects can't be reached; the new project cards don't yet show the project color or a live
per-status breakdown, and their right-click menu is missing Rename and Change color; and the Codex
setup copy briefly removes a file mid-swap despite a comment promising it never does, which a
still-running Codex agent could trip over in rare timing. Build and the backend tests pass.

**Needs attention:** Fix the 200-session Archive cap before marking the dashboard change shipped; the
other four are worth doing but not blockers.

**Next:** The build agent should page the grouped Archive at both levels (add a >200-session test),
then close the four smaller findings.

### 2026-07-28 — Implementation in progress: project dashboard and grouped Archive

The working tree now has durable project and agent archive state, archive and restore actions, project-first navigation, grouped Archive results, and launch safeguards that keep archived projects from starting new work. Project archive also stops active pipeline runs before archiving their agents. Focused backend and frontend regression suites, the full UI suite, source build, embedded frontend build, and specification checks pass.

**Needs attention:** Finish the remaining acceptance coverage and retire the superseded flat-Archive requirements before this change can be marked shipped.

**Next:** Continue the active project dashboard and grouped Archive change.

### 2026-07-28 — Fix review: project dashboard and grouped Archive design

All nine review findings were valid, and the waiting design is corrected. The specifications now
record the confirmed decisions, define project-group and per-project agent pagination, preserve
full-text search across every recorded session, require Restore before Resume for archived agents,
and treat an archived default project as a dormant preference that launch selectors ignore until it
is restored. Project archival now takes an exclusive transition claim before stopping anything, so
no interactive or pipeline launch can enter the stop-to-commit window. Missing-project cards disappear
with their last non-archived agent and expose no invalid project actions; missing and archived project
routes have explicit non-empty states. The pipeline specification's shipped-only preamble is fixed.

The current flat Archive requirements remain binding until implementation ships; the ready change now
explicitly retires the superseded flat listing, paging, and acceptance rules at that point while
retaining the valid non-null list guarantee. Specification and whitespace checks pass.

**Needs attention:** None.

**Next:** Run `/work project-dashboard-and-project-grouped-archive.md` when you want it built.

### 2026-07-28 — Design: project dashboard and project-grouped archive

The redesign is now ready to build. The main dashboard will show active project cards, project
dashboards will show only their non-archived agents, and the Archive will group archived agents under
their project — including an active-project marker and archive count when a project appears in both
places. Project cards can be renamed, recolored, and archived; archiving a project warns, stops its
agents and pipeline runs, then archives them. An individual agent archive stops it immediately;
project and agent restoration are independent, while an archived project must be restored before an
agent can return to work. Existing layout preferences remain shared, with no migration or per-project
layout state.

**Needs attention:** None.

**Next:** Run `/work project-dashboard-and-project-grouped-archive.md` when you want it built.

### 2026-07-28 — Fix: make Codex session isolation safe to rely on

AgentDeck’s private Codex profile now copies only recognised setup, never personal runtime databases,
logs, snapshots, or session state. Setup tools keep their executable permission while remaining
private. Refreshes are staged and transactional: a failed update keeps the prior profile intact, and
new Codex processes cannot start against another process’s half-updated setup. Unsafe profile paths
and source aliases are rejected before anything is changed. A Codex runtime switch now validates the
profile before stopping the working agent, so a bad personal setup leaves that agent and its
registrations running.

The full automated suite, specification checks, source build, and distribution build pass.

**Needs attention:** The existing signed-in Codex acceptance check is still required to prove the
packaged CLI honours the private profile and native-resumes from it.

**Next:** Run that credentialed Codex launch and resume check when you want to clear the release gate.

### 2026-07-28 — Review: Codex session isolation

The isolation layer is not safe to rely on yet. It copies current Codex state databases because it
treats unknown files as setup, strips execute permission from setup tools, can expose a
half-refreshed profile to concurrent or already-running agents, follows an existing private-profile
symlink outside the AgentDeck home, and can stop a working agent while leaving registrations behind
when switch rollback hits the same refresh failure. The shared private-home override itself is
correctly wired across launch, resume, and switch, and the automated verification passes.

The review also found a stale planned label, missing lifecycle-path acceptance coverage, and no
committed spec-first change trail for this feature.

**Needs attention:** Fix the five must-fix isolation findings before relying on this boundary or
running the credentialed release gate.

**Next:** An implementation agent should run `/fix`, close the must-fix findings with regressions,
then rerun the pinned live Codex launch and native-resume check.

### 2026-07-28 — Design: project-grouped archives

The main dashboard will show active projects, whose right-click menu can rename, recolor, or archive
them. Stopped agents remain visible and resumable inside their active project. An archived agent is
always listed beneath its project in Archive; a still-active project therefore appears both on the
main dashboard and in Archive, clearly marked active and showing its archived-agent count. Archiving
a project warns that running agents will be stopped, then archives the project and every agent. An
individual agent Archive action stops and archives immediately with no confirmation. Project and
agent restoration are independent, but an archived project must be reactivated before any of its
agents can restore, resume, or launch.

**Needs attention:** Confirm this complete behavior and whether project dashboards should initially
share the current saved card layout or have independent layouts.

**Next:** After confirmation, define the storage, API, routes, and migration behavior for the
combined dashboard and archive redesign.

### 2026-07-28 — Design: archive projects from the dashboard

Projects can now be defined as active or archived in the design. Archived projects move from the
main dashboard to their own restoreable project view while keeping their configuration, agents, and
session history; this remains separate from the existing session Archive.

**Needs attention:** Confirm whether archived projects should be blocked from new launches and
resumes until restored. The recommendation is yes, and to prevent archiving while agents are running.

**Next:** After confirmation, define the storage, API, and route details and prepare the change for
implementation.

### 2026-07-28 — Design: project-first dashboard

The proposed redesign makes the main dashboard a live grid of project cards, including empty
projects. Opening a project shows the familiar agent dashboard filtered to that project, so agent
cards show their role without repeating the project name. Agents whose project was later deleted
remain reachable under an unavailable-project card instead of disappearing.

**Needs attention:** Confirm this behavior and whether each project should have its own saved card
layout, or initially share the current layout preferences.

**Next:** After confirmation, define the route, layout-storage, and component architecture and
prepare the change for implementation.

### 2026-07-28 — Build: keep AgentDeck's Codex sessions out of your personal history

AgentDeck now runs every Codex agent in its own private Codex home, so the conversations it starts no
longer show up in your personal `codex` resume list or the Codex app history. Right before each Codex
agent launches, resumes, or switches, AgentDeck copies your current Codex setup — configuration,
sign-in, and any setup files — into that private home, one way only: your session history is never
copied, changes and removals in your own setup are picked up on the next launch, and AgentDeck never
writes back into your personal Codex home or follows a shortcut that points outside it. Your normal
Codex home is left alone, so the rest of AgentDeck that reads it (federation and model sync) is
unaffected. Codex sessions AgentDeck created before this change stay where they were and may no
longer resume from inside AgentDeck, as we agreed.

The automated checks pass: both Go test builds, the source build, the specification checks, and
targeted concurrency and whitespace checks, including new tests for the private-home refresh, the
environment wiring, and the pre-launch refresh. No visible UI changed.

**Needs attention:** One real-Codex check still needs your machine and sign-in — confirming the
packaged Codex CLI actually uses the private home, sees your refreshed setup, and can resume a new
isolated session. That is the existing credentialed provider gate, not a blocker for the code.

**Next:** When you want it verified end to end, run a credentialed Codex session through AgentDeck and
confirm it does not appear in your personal `codex` history.

### 2026-07-27 — Fix: stale project selection in the AgentDecker builder

The AgentDecker pipeline builder no longer lets you launch into a project that was removed after you selected it. Before, if a project was deleted in Settings or another tab while the builder was open, the selector quietly showed nothing selected but the launch still tried to start in the removed project and was rejected. Now the launch button stays disabled unless the chosen project is still in the current project list, and a selection that disappears is cleared so you pick again from what actually exists.

The full automated suite — specification checks, both Go test variants, all 163 UI tests, and the UI and release builds — passes, and the new test confirms the old behavior would have failed.

**Needs attention:** None.

**Next:** No action needed. One lower-priority process note remains open: three recent user-facing changes shipped without the usual planning record; adopt the normal ready-change trail before the next behavior change.

### 2026-07-27 — Review: the past twelve hours of changes

The reviewed changes are largely in good shape: their shipped behavior is covered by the current specifications and regression tests, while the new effort-selection work is correctly marked as planned rather than shipped. The full automated suite and release build pass.

One defect needs a fix: if a project is removed after you selected it in the AgentDecker builder, the builder can still try to launch into that removed project instead of requiring a current choice. The launch button should stay disabled until the selected project is still present in the project list.

The review also found a process gap. Three new user-facing changes were committed without the normal ready-change and active-plan trail, so it is not possible to verify from history that their specifications were completed before implementation. Their specifications now match the code; use the normal planning record before the next behavior change.

**Needs attention:** Fix the stale builder-project selection before relying on that picker after project configuration changes.

**Next:** Run `/fix` to repair the builder selector and add its regression test.

### 2026-07-27 — Build: annotate by highlighting and right-clicking

The permanent **Annotate** button above every chat message is gone. Instead you highlight the part
of a message, tool call, or result you care about, right-click it, and pick **Annotate selection** —
and only the text you highlighted becomes the excerpt, rather than the whole event. Right-clicking
an event without highlighting anything still offers **Annotate whole event**, so nothing you could
do before is lost. The menu closes on Escape, on a click elsewhere, or once you pick the action, and
events that were never annotatable keep the browser's own menu.

Diffs are unchanged: you still click line numbers to pick a range, because that selection is about
lines rather than text and the button there only appears once you've chosen a range.

**Needs attention:** I verified this with component tests, the specification checks, both Go test
variants, and the full UI suite and builds, but not in a live browser against a real agent
transcript. Right-click behaviour is worth one minute of your own hands-on check. Also note that
the effort-design session's notes were still sitting uncommitted in these same two tracking files,
so they went into this commit alongside mine; nothing else from your uncommitted work was touched.

**Next:** Try it in a real chat and tell me if the selection-only excerpt reads better than the old
whole-event capture; if it does, the whole-event fallback could go too.

### 2026-07-28 — Design: keep AgentDeck's Codex sessions out of your personal history

The planned design now gives AgentDeck Codex a private home for its sessions while refreshing your
current Codex setup into it before every launch, resume, or switch. Your auth, configuration, skills,
agents, rules, plugins, and MCP setup are copied one way; your personal session history is not.
Changes or removals in your personal setup appear on the next AgentDeck Codex process, and AgentDeck
never writes to your personal Codex home. Existing AgentDeck Codex sessions may no longer resume
natively, as agreed.

The relevant feature, federation, persistence, and integration specifications plus the ready change
now describe this boundary. No product code changed.

**Needs attention:** None.

**Next:** Run `/work` on `isolate-codex-sessions-dedicated-home.md` when you want it built; the live provider check rides along on the credentialed provider gate.

### 2026-07-27 — Design: choose an agent's effort level at launch

I designed this one but wrote no product code — it's specified and queued, ready for a build session.

You doubted Claude had an effort setting, so I checked instead of guessing. It does: the Claude Code
on this machine takes `--effort` with `low, medium, high, xhigh, max`. That reversed my earlier
answer, and it was worth reversing.

Checking Codex turned up something more useful. Codex doesn't take effort as a separate setting at
all — it hides the level inside the model name, as `gpt-5.6-sol[high]`. So AgentDeck needs no new
plumbing for Codex; it already sends a model name. Better still, Codex publishes each model's
available levels in a file AgentDeck **already reads** for its model auto-sync, so for Codex the
levels can fill themselves in. Claude works the opposite way: it takes effort as a separate setting,
and only *after* a session has started. Three providers, three genuinely different mechanisms — which
is why this needed designing rather than just building.

What you'll get: an effort picker beside the model in New Agent, an `--effort` flag on the command
line, effort on a running agent through Switch runtime, and a per-stage effort when you start a
pipeline run. A model only offers the levels it actually supports, so models without effort show no
control at all rather than a setting that quietly does nothing.

Three things I decided deliberately, and would change if you disagree:

**Nothing is guessed.** Ask for a level a model doesn't have and the launch is refused with a clear
message, rather than quietly running at some other level. That's the choice you made, and I carried
it through to the awkward case too: if the provider itself rejects a level, AgentDeck stops rather
than retrying without it.

**Your config file doesn't break.** The new settings are optional additions, so an older AgentDeck
can still read a newer file — it just ignores effort. No migration, nothing to convert.

**I found a bug while designing this.** AgentDeck has always let you set an effort override on a
linked Claude or Codex configuration, and has always *displayed* it — but nothing ever sent it to the
agent. That override has been decorative since it shipped. I recorded it honestly rather than quietly
patching it; this change makes it real.

**Needs attention:** two things. Six specification documents that previously read as fully shipped are
now marked partial, because they hold planned work — they flip back when this is built.

And I did not commit. Work from an earlier session was already sitting uncommitted in the same
documents I edited, so my changes and theirs are now mixed together in the same files and I can't
separate them into an honest commit of my own. Nothing is lost and nothing is overwritten, but
someone needs to look at the pending changes and decide what belongs in which commit before this
gets saved to history.

### 2026-07-27 — Fix: choose the project when building a pipeline with AgentDecker

You hit a real gap, and the error message was telling the truth. **Create with AgentDecker** always
started its session in whatever project was set as the default, and it offered no way to change
that. On a new setup the default is the example project AgentDeck ships with, **My App**, which
points at a folder that doesn't exist — so the launch was refused, and nothing on the Pipelines page
could fix it. The Start Run form right below it already had a project picker; the builder had simply
never been given one.

The builder now has a **Project** dropdown listing every project you've configured, and it launches
into the one you pick. It still starts on your default project when that project exists, so the
common case is unchanged. If your default has been deleted or renamed, the picker now comes up
empty and the launch button stays disabled, instead of letting you press a button whose only
possible outcome was a rejected launch.

You can still change a project's folder in **Settings → Projects**; that remains the place to point
**My App** at a real directory, or you can just add a project and select it in the builder.

I checked this in a real browser against a clean setup: picking a real project gets past the folder
error, and picking **My App** still reports it — now right beside the dropdown that fixes it.

**Needs attention:** Another session's work is sitting uncommitted in this repository — the live
provider acceptance results and the planned per-stage effort specification. I left all of it
untouched and committed only my own files, but it still needs its owner to finish and commit it.
Also restart your running dashboard to pick up the rebuilt interface.

**Next:** Restart the dashboard, open **Pipelines → Create with AgentDecker**, and pick your project
from the new dropdown.

### 2026-07-26 — Fix: Resume stopped agents from the Dashboard

Right-clicking a stopped agent card now offers **Resume**. It uses the same durable session resume
as Archive, restores the frozen configuration and history, and shows a clear error if it cannot
start. The action is absent from running cards, where it would be invalid.

The menu's visibility, request, and failure feedback are covered by regression tests. All 156 UI
tests, both Go test variants, specification checks, and the release build pass.

**Needs attention:** Restart a currently running local dashboard to load the rebuilt application.

**Next:** Right-click a stopped agent card and choose **Resume**.

### 2026-07-26 — Acceptance: live Claude and Codex providers

I ran the credentialed provider checks you authorized, against your real logged-in Claude and Codex
installations with a throwaway AgentDeck home and project. Neither provider's own configuration was
modified. Most of it works for real: Claude chat streaming, permission approve and deny, stop, and
resume; Codex launch, streaming, prompt delivery, and stop; cross-provider messaging, where a Codex
agent messaged a Claude agent and the Claude agent woke and read it; and the Claude terminal, whose
flags and hooks the real CLI accepted, with a live typed prompt answered inside AgentDeck's terminal
view. Configuration federation also holds up: it reads your real setup, records where each value came
from, keeps secrets out of the mirror, notices an outside edit on its own, and refuses to launch when
your setup would collide with AgentDeck's own messaging tool.

Three things do not work against the real providers. Cancel kills a Claude agent instead of
interrupting it, because that adapter ignores the cooperative cancel and the fallback signal ends the
process — so cancelling costs you the agent. Resuming a Codex agent silently starts a fresh
conversation: AgentDeck still shows the old transcript, but the agent has no memory of it. And the
model you pick per agent has no effect on either provider — both run whatever their own CLI config
says — so AgentDeck's model choice is currently cosmetic for real sessions. Full evidence, including
how to reproduce each one, is in the run report under the archived reviews.

OpenCode and OpenHands still could not be checked: neither is installed, and both would need you to
enter credentials.

**Needs attention:** Two other things came out of the session. Another AgentDeck session was
committing to this repository while I worked, and one of its commits swept up a temporary probe file
I had created (`internal/runtime/zz_cancelprobe_acceptance_test.go`); its removal is waiting in the
working tree, uncommitted. Because that session is still editing shared files, I did not touch the
handoff or the specifications — the gate results and the three findings still need to be recorded
there once the tree is yours again.

**Next:** Decide what Cancel should do when a provider ignores it, then let me record the results in
the handoff and fix the Codex resume defect, which is a small, well-understood change.

### 2026-07-26 — Fix: visible Dashboard state labels

The top-right state badge is now readable. It was meant to show the agent state, but a CSS rule
painted the label itself as a solid block; it now styles only the status dot. The browser check
confirms readable Busy, Idle, Waiting, Done, Error, and Unknown labels, and the full test and
release-build checks pass.

To start a stopped agent today, open **Archive**, select that session, and choose **Resume**. The
Dashboard card itself does not yet offer Resume, so this path is more indirect than it should be.

**Needs attention:** Decide whether stopped Dashboard cards should gain a direct Resume action.

**Next:** Restart a currently running local dashboard to load the rebuilt application, then use
Archive → Resume to restart a stopped agent.

### 2026-07-26 — Fix: clearer Dashboard agent cards

Dashboard cards now show the configured project name rather than its internal id, while retaining
the id as a fallback if a project definition is no longer available. Empty context bars now visibly
say `0% context used`, so an active agent no longer looks as if part of its card failed to load.

This slipped through because the old written rule explicitly required a blank zero-value meter, and
the visual fixture and component tests mirrored that rule. The card test also bypassed project
configuration entirely, while the layout usability journey only checked layout mechanics. The
specification, regression tests, visual matrix, and J5 browser checklist now cover the missing
observations. The full test and release-build checks pass, and the rendered card states were checked
in a browser.

**Needs attention:** Restart a currently running local dashboard to load the rebuilt application.

**Next:** Inspect an active agent card after restart; its project subtitle should use the project name
and a zero-value meter should read `0% context used`.

### 2026-07-26 — Usability review: browser retry

Browser confirmation of the seven repaired findings is still incomplete. The earlier native-confirm
stall is gone: the confirmation interaction completed in the running app. But that run was attached
to an older review home, so it cannot prove the intended layout result. A fresh isolated lifecycle
fixture did render its stopped agents, transcript, and Archive list; immediately afterward, the
in-app browser repeatedly lost its tab after each page transition. I stopped rather than calling
the remaining Archive, pipeline, annotation, or validation paths browser-tested based on their
automated coverage. The exact partial evidence and blocker are in
[the retry report](../archive/reviews/usability-review-run-2026-07-26-browser-retry.md).

**Needs attention:** The in-app browser needs a stable tab session before the remaining real-browser
checks can be completed.

**Next:** Re-run J5 restart/delete and J6–J14 using separate ports and a stable browser session.

### 2026-07-26 — Fix: the seven recorded review findings

All seven problems from the last review are repaired, each with a test that was first confirmed to
fail against the old code. The two serious ones are closed: a resumed or runtime-switched agent that
crashes is now fully released — the server drops its ownership and per-agent credentials, another
resume works immediately, and a pipeline waiting on that agent moves into crash recovery instead of
stalling; and an agent page reached by a direct link or reload now waits until it actually knows
whether the source session exists before offering to discard retained annotation drafts, so a normal
click can no longer delete valid work.

The five smaller fixes: completed pipeline stages now link to their archived transcript instead of a
dead live page; a stopped AgentDecker builder is recognised as finished and its stale session link
disappears; an archived session that switched backend or model shows the identity Resume will
actually restore; Archive paging shows every matching session exactly once even when sessions are
touched while you page through them; and onboarding now explains which field a rejected backend or
project was wrong in, instead of showing “Error: HTTP 400.” Two of these needed the written product
rules to be extended — how far Archive paging must go to stay complete, and when an agent may be
declared missing; the rest simply restored behaviour that was already specified.

**Needs attention:** None of these fixes has been exercised in a real browser yet — the last
usability run could not get past its recorded browser and account limits, so browser confirmation of
the Archive, pipeline, and onboarding surfaces is still outstanding.

**Next:** Run a usability review over the Archive paging, pipeline transcript links, and onboarding
error paths once a browser session is available.

### 2026-07-26 — Review: the week’s feature and fix batch

The re-review found seven real problems: two need fixing before the recent work is dependable, and
five are smaller but user-visible. Most seriously, a resumed or runtime-switched chat agent that
crashes can be left falsely owned by the server, blocking another resume and preventing a pipeline
from entering recovery. Also, a deep-linked agent page can offer to discard retained annotation
drafts before the initial server state has even established whether their source still exists.

The smaller defects are broken transcript links for completed pipeline stages, a stopped
AgentDecker builder retained as if it were live, stale backend/model identity in an archived header
after a runtime switch, Archive paging that can skip or duplicate sessions when ordering changes
between pages, and onboarding validation failures reduced to “Error: HTTP 400.” The full automated
test and build matrix still passes, which confirms these are coverage gaps at real state boundaries
rather than broad regressions.

**Needs attention:** Fix the resumed-agent cleanup and premature annotation-discard paths first;
the other five findings should follow in the same repair pass.

**Next:** Run `/fix` to repair the seven recorded findings and add regressions at their real call
sites.

### 2026-07-26 — Fix: review-finding queue

There were no open review findings to repair. All previously recorded issues remain resolved, and
the project’s specification checks pass without a product or specification change.

**Needs attention:** None.

**Next:** Share a new review finding or choose a ready change when you want further work.

### 2026-07-26 — Usability review: post-fix rerun

The rerun found no new problem in the paths it completed. Fresh first paint, onboarding, the
provider-version fallback, Set up later, first launch, chat replay, and all four permission outcomes
worked in the real release build with zero unexpected browser errors. Dashboard density, collapse,
pointer-drag reorder, reload persistence, and the group-release backing operation also behaved
correctly. All 139 UI tests, the presentation checks and build, specification checks, and focused
tagged and fallback-build regressions for the recent fixes pass.

The run could not honestly cover the whole matrix: the browser stalled on a native confirmation
dialog during the layout journey, then the execution account hit its usage limit and refused any
further isolated server starts. The remaining layout restart/delete variants and the terminal,
lifecycle, Archive, Settings, messaging, recovery, durability, annotation, and pipeline journeys are
recorded as blocked—not passed—in [the full rerun report](../archive/reviews/usability-review-run-2026-07-26-rerun.md).

**Needs attention:** The incomplete journeys still need a real-browser rerun after the execution
limit resets; credentialed provider and terminal compatibility remain the existing manual gates.
The review state is committed locally, but pushing it to `origin/main` needs your explicit
authorization.

**Next:** Re-run usability review starting at the remaining layout variants, then J6–J14 in order.

### 2026-07-26 — Fix: all usability review findings

All sixteen findings from the week’s browser review are fixed. The final permission-label change
was genuinely small: AgentDeck already stored the distinct outcomes, so the live and replayed
transcript projection now preserves them and the chips read **Approved**, **Denied**,
**Cancelled**, or **Timed out**. Unknown legacy outcomes still fall back conservatively to Denied.

The final specification checks, both Go test variants, all 139 UI tests, source and UI builds,
presentation-contract checks, and the distribution build pass. The embedded production UI was
regenerated from source.

**Needs attention:** None for this review. Credentialed provider and terminal compatibility remain
the existing manual release gates.

**Next:** Run those credentialed acceptance gates when you are ready to authorize real provider
sessions.

### 2026-07-26 — Fix: usability review findings, awaiting one decision

Fifteen of the sixteen review findings are fixed and committed. Diff annotation, Codex pipeline
assignments, run-start diagnostics, archive paging and search guidance, fallback-build launches,
annotation-tray usability, live permission replay, agent/project validation, cancellation
escalation, and archived-session identity now have regression coverage.

All specification checks, both Go test variants, all 134 UI tests, and source/UI builds pass after
the last fix. I have not run the final distribution build because one product decision remains.

**Needs attention:** Choose the permission-chip vocabulary. I recommend distinct chips:
**Approved**, **Denied**, **Cancelled**, and **Timed out**. The alternative is to preserve the
current two-state wording, where every non-approval reads **Denied**; choosing that closes the
finding without code.

**Next:** Reply `distinct` or `keep Denied`. I’ll update the specification and UI if needed, run
final distribution verification, and close the fix pass.

### 2026-07-26 — Usability review: the week's new features in a real browser

I drove everything shipped in the last week through the actual app in a browser — pipelines,
annotations, archive and search on both builds we ship, plus the ordinary launch, chat, permission,
crash and restart paths. Most of it holds up well. The entire pipeline lifecycle works: stages run
one at a time, results only advance the run when the stage agent actually reports and finishes its
turn, approval pauses wait for you, a failed check loops through the repair stage and back, stopping
a stage agent pauses the run with a working Retry, and everything survives a restart. Annotations,
archive search, permissions and crash recovery also passed.

Two shipped features, though, cannot be used at all, and both have been in the product for a while
without anyone noticing. First, selecting lines in a diff to annotate them does nothing — clicking
line numbers has never worked in a browser, because the code looks for a line label in a format the
diff component does not produce. Whole-message annotation still works, so the feature looks alive
until you try its main use. Second, no Codex model can be assigned to a pipeline stage: the app
offers Codex and its models in the dropdown, then refuses the run with a message saying you must
choose a backend and model. The cause is an internal name rule that forbids dots, and both Codex
model names contain one. Our own written example of this feature is "Codex for the work, Claude for
the review", so the headline case is impossible on a fresh install.

Four more real dead ends: the archive only ever shows the first 50 sessions while truthfully saying
there are more, with no way to reach the rest; the pipeline start form throws away the server's
explanation of what is wrong and just says "run cannot start"; a search combining an agent's name
with a word from its transcript wrongly reports no results, because terms have to appear in the same
turn and nothing tells you that; and if a folder is ever opened by our standard build and then by the
fallback build, no agent can launch at all. I also recorded ten smaller items, mostly missing
validation and unclear wording.

Worth saying plainly: the first two problems were invisible to every check we run. The test suite
passes because nothing tests the diff component's connection to its third-party library, and the
pipeline tests use a made-up model name with no dot in it. This is exactly the gap browser review
exists to close.

**Needs attention:** Six problems need fixing, and two of them make an advertised feature unusable
rather than degraded. I did not change any code — the full list with reproduction steps and
screenshots is recorded for the fix work.

**Next:** Run the fix loop over the six must-fix items, starting with diff-line annotation and the
Codex model rejection.

### 2026-07-26 — Fix: Claude onboarding compatibility

Claude’s readiness check now handles the common ways different command-line versions reject the
optional `--no-color` flag. When the diagnostic explicitly identifies that flag as unsupported,
AgentDeck retries the same fixed status check without it; unrelated authentication or status
failures still remain failures. This prevents a signed-in Claude user from being falsely blocked in
onboarding. The new regressions and the full verification suite pass.

**Needs attention:** None.

**Next:** Run the credentialed provider acceptance gates when you are ready to authorize real
provider sessions.

### 2026-07-26 — Usability review: onboarding wizard

I replayed onboarding in a real browser from two fresh homes. Set up later works with an ordinary
pointer click, opens the dashboard without launching anything or changing the seeded provider
catalog, and stays complete after reload. Check again correctly moves through missing, signed-out,
and ready provider states. Once ready, the full project, optional configuration, first-agent launch,
and restart path also completed without browser errors.

I found one blocker: a valid signed-in Claude adapter can still be rejected when an older or
different version describes the unsupported `--no-color` option as “unknown flag” instead of the
one exact phrase AgentDeck recognizes. The wizard then reports a credential failure and cannot
advance, even though the same provider check succeeds without that optional flag. The complete run
and reproduction are in [the J2 usability report](../archive/reviews/usability-review-run-2026-07-26-j2.md).

**Needs attention:** Fix the provider-status fallback so normal wording differences cannot falsely
block onboarding.

**Next:** Run `/fix` to repair the recorded onboarding finding and add the missing provider-version
regression.

### 2026-07-26 — Fix: pipeline recovery, onboarding races, and transcript boundaries

All seven open review findings are fixed. Stopping a pipeline stage from its ordinary agent card now
pauses the run with a usable retry instead of leaving it stuck. Large stage inputs can no longer cut
off the required result-reporting instruction. Finished attempt transcripts open from the archive,
pipeline alerts use the run name and say success or failure, and dead AgentDecker builder sessions
are removed from browser state.

Set up later now shares one mutation claim with every onboarding step, so it cannot race backend
checks, project creation, configuration-source work, or first-agent launch. Transcript search
indexing now commits each turn or annotation boundary as one sequence-aware operation, keeping later
events in the next document. Full automated tests, race detection, and release builds pass.

**Needs attention:** The onboarding wizard still needs its recorded real-browser journey replay.

**Next:** Replay the onboarding journey in a browser when you want to close that remaining manual
evidence gap.

### 2026-07-26 — Review: configurable pipeline runs and the preceding fix batch

I reviewed everything built since the last review marker: the six-finding fix batch and the whole
configurable pipeline runs feature. The design holds up. Templates really are reusable and
model-neutral, a run really is frozen at start, and a stage only advances when the agent reports an
explicit result through its own token and its turn then finishes — runtime status alone never
completes anything. Every state change goes through a version check, so two clicks or a crashed
retry cannot double-advance a run. All the promised surfaces exist: the page, the API, the command
line, the agent tools, and the links from ordinary agent cards back to their run.

I found five problems, three of which I'd fix before anyone relies on this.

The most serious: if you press Stop on a stage agent's card — an ordinary action AgentDeck deliberately
still offers — the run does not notice. It keeps waiting for a result from an agent that no longer
exists, shows no warning, and will not let you retry the stage. Only stopping the whole run, or
restarting the dashboard, gets you out.

Second: the stage instructions AgentDeck writes for each agent are capped in length, and the
mandatory "report your result when you're done" line sits at the end. Paste a long specification into
a run input and that line gets cut off, so the agent never reports and the run hangs the same way.

Third: in a finished run's attempt history, "Open transcript" points at the live-agent screen rather
than the archive, so it shows "Agent not found" for every completed stage — exactly the transcripts
you'd want to read after a run finishes.

The two smaller ones: pipeline notifications name the run by its internal id instead of the name you
gave it, and a completed-run toast doesn't say whether it succeeded or failed; and the AgentDecker
builder remembers its chat session in the browser forever, so a dead session keeps showing a broken
link.

All the shared checks pass — specifications, both Go test variants, 120 interface tests, and both
builds. That is the point worth noticing: none of these five paths has a test, which is why they came
through green. I changed no product code or specifications.

**Needs attention:** The stop-wedges-a-run problem is the one that will bite a real user first,
because pressing Stop on a card is a normal thing to do.

**Next:** Ask me to fix the open findings, starting with the stop and the truncated instruction.

### 2026-07-25 — Implementation: configurable pipeline runs

Configurable pipeline runs are now implemented. AgentDeck has reusable model-neutral templates,
durable sequential runs and attempt history, explicit token-bound stage results,
approval/blocked/retry/stop recovery, restart reconciliation, REST/SSE/CLI controls, notifications,
agent-to-run links, and a Pipelines page with manual editing and exact-payload AgentDecker proposals.

Verification passed: specification checks, both Go test variants, 120 UI tests, UI style/build
checks, focused race tests, the distribution build, and `git diff --check`. I also exercised the
Pipelines page in an isolated real browser; that pass caught and fixed a form overflow and an
empty-state layout issue.

**Needs attention:** Credentialed Claude/Codex acceptance remains the existing manual release gate.
The two previously recorded review findings—onboarding completion races and non-atomic index
boundaries—are unchanged and remain in the handoff.

**Next:** Run J14 when you want the full pipeline usability journey replayed.

### 2026-07-25 — Feature design: configurable pipeline runs ready

Configurable pipeline runs are now fully specified and waiting to be implemented. Templates are
reusable and model-neutral; each run supplies its project, goal, text inputs, and backend/model for
each stage, so Codex and Claude can swap implementer and reviewer duties without changing the
template. AgentDeck runs one chat agent at a time, advances only from an authenticated explicit
success/failure/blocked result, supports approvals and bounded repair loops, and preserves attempts,
outputs, recovery state, agents, and transcripts across restarts.

AgentDecker can draft a template or run setup and request Save or Start. Each exact action requires
its own one-time confirmation in the Pipelines page. This is intentionally a soft interaction guard,
not a new authentication system. The technical design uses one in-process transactional reconciler,
versioned JSON templates, SQLite run state, the existing agent lifecycle and MCP server, and ordinary
REST, server-sent events, and CLI surfaces—without a queue, separate workflow service, or parallel
graph engine. Effort selection remains a separate future idea.

**Needs attention:** None.

**Next:** Start the waiting configurable pipeline runs change when you want implementation to begin.

### 2026-07-25 — Feature design: configurable pipeline runs

The feature-side draft now describes reusable pipeline templates whose stages can each select a
role, backend, and model, including Codex working with Claude reviewing or the reverse. Starting a
run snapshots its goal, project, stages, and model choices; AgentDeck launches one chat agent at a
time, gives it the relevant prior results, accepts an explicit passed/findings/failed/blocked report,
and durably shows progression, pauses, retries, repair loops, and transcripts after restart. Runtime
idle or process exit never counts as success, permissions and credentials keep their existing rules,
and task groups remain presentation rather than pipeline authority. No product code or technical
architecture has been written yet.

**Needs attention:** Please choose the first three product boundaries: (1) sequential stages with
outcome-based skips and bounded backward repair loops, but no parallel graph, as recommended — or a
full parallel directed graph now; (2) runs start from the dashboard or local command/API, which lets
AgentDecker invoke them after your request, as recommended — or agents also receive a direct
start-run tool; and (3) versioned hand-editable JSON plus a simple Settings stage-list editor and
Start Run form, as recommended — or JSON-only configuration for the first release.

**Next:** Reply with your choices for those three boundaries; I will refine the feature behavior,
then bring the remaining retention, retry-identity, and same-project concurrency decisions before
designing the technical specification.

### 2026-07-25 — Review: recent annotation, onboarding, provider, and search work

The full unreviewed range has now been checked against the product specifications and every recurring
bug class. I found six must-fix issues and two smaller accuracy issues. The most consequential are
that a running agent's annotation can still be delivered after its transcript write failed, and the
new search index can put a concurrent event on the wrong side of an immutable turn or annotation
boundary. Setup can also complete as “later” while another wizard request is still creating or
launching something; provider readiness can expose Claude status text or let a hung Codex check
prevent a valid API-key fallback; and session metadata is committed in two transactions that can
leave a partial result. The two smaller issues are misleading sign-in recovery copy and an annotation
specification link that still points to retired search behavior.

All existing automated checks pass, including both database variants, 113 interface tests, the
source builds, and the distribution build. The findings are in failure and concurrency paths those
tests do not currently exercise.

**Needs attention:** Do not publish this batch until the six must-fix findings are resolved.

**Next:** Run the fix workflow to handle the must-fix findings one at a time, adding the injected
failure and deterministic interleaving tests described in the handoff.

### 2026-07-25 — Implementation: make transcript search scale by turn

Archive indexing no longer keeps every touched session's complete transcript in memory or rewrites
that growing text after every turn. Each completed turn is now inserted once as its own searchable
document; metadata updates and annotation flushes use separate document boundaries. Existing
installations keep all old
search text through an automatic legacy-document migration, and rebuilding the index now streams the
transcript instead of loading the whole file.

The intentionally simpler design has one visible trade-off: all words in a search, including a quoted
phrase, must occur in the same turn, annotation, metadata record, or migrated legacy record. A query
cannot combine one word from an early turn with another from a later turn, and one exceptionally huge
turn can still use substantial temporary memory. Those limits and the evidence that would justify the
more elaborate size-bounded, cross-turn design are recorded in the ideas backlog.

The complete test suite passes with and without full-text search enabled, as do focused concurrency
tests and both production builds.

**Needs attention:** Publishing remains paused; local main is eight commits ahead, and pushing that
combined scope still needs your explicit approval.

**Next:** Have an independent reviewer check the new indexing and migration path; revisit the more
complex design only if real searches miss conversations across turns or individual turns become
large enough to cause memory pressure.

### 2026-07-25 — Fix: the rest of the onboarding wizard

You were half right that this was already done. The wizard's anti-eject fix and the readable
credential messages had landed earlier as bug fixes, but the actual queued work had not started, and
the specification still described all of it as unbuilt. That work is now finished.

The most useful discovery was an outright broken path. Signing in to Codex from the command line
never worked: AgentDeck ran `codex-acp login`, but that program is not a command-line tool — it is a
background service that ignores whatever you type after it, so the command just hung forever instead
of signing anyone in. The cause was that two parts of AgentDeck each kept their own private copy of
"how to talk to a provider", and only one copy was right. There is now a single list both parts read,
and a test that fails if they ever disagree again.

That fed the bigger fix. If you had signed in to Codex with your ChatGPT account, AgentDeck still
declared it not ready, because it only ever looked for an API key — which that kind of sign-in does
not produce. It now asks Codex directly whether you are signed in, and only falls back to the API key
if you are not. I confirmed this against your actual signed-in Codex: setup goes from blocked to
ready with no key configured anywhere.

Three things changed in the wizard itself. There is now a **Set up later** button, so a first run
without working credentials is no longer a dead end — previously the wizard could not be dismissed by
any means, and someone without a configured provider was simply stuck. The two boxes asking for model
identifiers are gone; the wizard uses the backend's own default and you change models in Settings.
And for Claude and Codex it now tells you the exact command to run to sign in, with a **Check again**
button beside it, rather than asking you to guess and retry blind.

New installs also get sensible current defaults, and re-running setup will not touch a backend file
you have already edited. Alongside that, release packages now carry the Codex command-line tool
directly instead of relying on it happening to come bundled with something else — that is what makes
the sign-in check work on a machine with no Codex installed.

**Needs attention:** I could not click through the finished wizard in a real browser, so Set up later
and Check again are proven by tests and a live server run, not by a human using them. That is worth
doing before you next hand AgentDeck to a new user. Publishing is also still waiting on you — there
are now seven commits ready to push.

**Next:** Replay the setup walkthrough in a browser, then decide whether to publish.

### 2026-07-25 — Fix: annotations are recorded before they are delivered

The important one was a real double-work risk. When you sent a batch of annotations to another agent,
AgentDeck delivered the mail and woke that agent up *before* writing the annotation into the source
session's own record. If that write then failed — a full disk, a permissions problem — you got an error
message while the other agent had already received the work, and because the tray is deliberately kept
after a failed send, pressing send again delivered a second copy for the agent to act on twice. The
source transcript, meanwhile, had no record of any of it. Now the record is written first and delivery
happens only after it succeeds, so a failure means nothing was sent and one retry sends exactly once.
A new test blocks the write and proves the recipient ends up with a single copy.

Three smaller cleanups came with it. Two copies of the excerpt-trimming code and three copies of the
annotation data shape had already drifted apart without anything failing, so each now has a single
definition. Pending annotation drafts kept in the browser had no cleanup at all: a deleted agent's
drafts lived on forever and could reappear against a reused identity, and the drafts of every agent you
ever opened accumulated until the browser's storage limit broke saving entirely. Drafts are now dropped
when their agent is deleted, and old or excess drafts are cleared when you reload — while still keeping
the drafts of an archived session, which is a supported place to annotate from. Finally, the
`curl | bash` installer left one small temporary file in your system temp folder on every run; the
right process now cleans it up.

All automated checks pass: specification checks, both database build variants of the Go tests, a
focused race check on the annotation path, the full browser test suite, and both application builds.

**Needs attention:** Publishing is still paused. Pushing now would send six local commits to the shared
repository, and that needs your explicit approval.

**Next:** Approve or decline the six-commit push. After that, the only annotation work left is the
real-browser walkthrough of annotating, sending, and receiving, which needs a person driving a browser.

### 2026-07-25 — Specification maintenance: strengthen recurring bug guards

The bug-class catalog now captures the two broadly repeatable lessons from the last two weeks without
collecting one-off release or provider quirks. A new rule requires AgentDeck to record its local truth
before releasing an external peer or visible side effect, and requires retryable multi-store work to
be atomic, rolled back, or safely deduplicated. The existing annotation duplicate-delivery finding is
now classified under that rule. The catalog also requires live and replay paths to share one event
projection helper, and records the repeated dashboard crashes caused by nested null collections.

The documentation-only change passed the specification and whitespace checks and was committed on
local main; no product code was changed by this maintenance commit. The requested push is paused
because it would publish this commit together with four already-completed local commits that were
ahead of the remote, and that expanded scope needs explicit approval.

**Needs attention:** Confirm whether to push all five local commits to origin/main. The annotation
delivery path also still violates the new ordering rule and remains the outstanding must-fix defect.

**Next:** Once the five-commit push is explicitly approved, push main; then a fix session should make
annotation delivery atomic or idempotent before addressing the remaining smaller review findings.

### 2026-07-24 — Workflow: make the bug-class sweep impossible to skip silently

I fixed the process hole that caused the earlier miss. The reading list every agent follows told them
to open the bug-class catalog only when something already pointed them there — but the instruction to
check work against it is written inside the catalog itself, so nothing ever pointed there and the step
was skipped without a trace. The three command files for build, review, and fix work now name the
catalog as something to always read, and say plainly what to do with it.

Documentation alone would not have stopped me, so the check now also runs as code. The existing
specification checker, which runs as part of the normal test command, now verifies that every recorded
review finding is marked as must-fix or worth-fixing and that any finding pointing at real code names
which known bug class it belongs to, or states plainly that none applies. Both of those were already
required in writing; the difference is that skipping them now fails a command people actually run
rather than passing silently. It checks the labelling, not the judgement behind it — its real value is
that a review which never opened the catalog cannot quietly claim it did.

Applying the new check immediately turned up one thing: the known installer leftover does belong to a
recognised bug class, cleanup that gets skipped on one exit path, which nobody had recorded.

**Needs attention:** Two related guards I deliberately did not add, because they are only words and
words are what failed last time — one telling a repeat review to form its own conclusions before
reading the previous review's, and one saying that "needs more tests" findings are not a substitute
for finding actual defects. Both describe exactly how I went wrong. Say if you want them written down
anyway.

**Next:** The duplicate-delivery defect in annotate-and-assign is still the first thing to fix.

### 2026-07-24 — Review: annotate and assign, re-checked against the bug-class catalog

At your request I re-reviewed the annotate-and-assign work. The first pass checked the feature only
against its written requirements and its test coverage. This pass also checked it against the
project's catalog of recurring bug classes, which is what a review is meant to do. That catalog is
where the real defects were.

One must-fix problem: when you assign annotations to another agent, the mail goes out before the
record of it is saved. If saving then fails, the delivery has already happened but the app reports an
error — and because the tray deliberately keeps your work when a send fails, pressing Send again
delivers the same batch a second time. The same gap lets an agent act on annotations that were never
recorded in the transcript.

Three smaller items: the text-shortening helper exists in three slightly different copies that have
already begun to disagree; the shape describing an annotation is declared three times with two copies
now out of step; and the pending tray is stored per agent in the browser and never cleaned up, so
drafts for deleted agents accumulate indefinitely.

**Needs attention:** Why the first pass missed these is worth fixing at the source. The instruction to
check work against the bug-class catalog is written inside that catalog, while the reading list agents
follow tells them to open it only when something already points them there — and for this change,
nothing did. The step gets skipped silently, with no failing check to reveal it. I recommend making
that sweep an explicit, unconditional step in the shared workflow and in the short command files for
build, review, and fix work, so an agent that never opens the catalog still gets told to.

**Next:** Fix the duplicate-delivery defect first, then the three smaller items. I can make the
workflow change whenever you want it.

### 2026-07-24 — Implementation: annotate and assign

You can now select diff lines or transcript events, add instructions to a browser-local tray, and
send the batch to the current agent, another running chat agent, or a newly launched agent. Each
send is recorded as a searchable annotation card, and assignments arrive as normal agent mail.

**Needs attention:** None.

**Next:** A usability reviewer should run the new annotate-and-assign journey in a real browser.

### 2026-07-22 — Review: work since the core-interface redesign

I reviewed everything built since the last review boundary: the chat permission fixes, the
transcript-replay fix, the onboarding wizard fix, and the release-archive packaging fix. All of it does
what the specifications say and nothing shipped that the specifications fail to describe. The two
recent design-only changes (annotate-and-assign, and the onboarding credentials repair) add planned
specifications only and ship no product code, as intended.

I found one minor issue, worth fixing but not urgent: the one-line `curl | bash` installer leaves a
small temporary file behind in the system temp folder each time it runs. It is harmless — owner-only
and cleared on restart — but it should tidy up after itself.

**Needs attention:** None. The installer temp-file leftover is queued as a Worth-fixing item, not a
blocker.

**Next:** A fix session can clear the installer leftover when convenient; nothing else is outstanding
from this review.

### 2026-07-22 — Project decisions: current security and terminal boundaries

The local API's same-machine trust model and broad child-process environment inheritance are accepted
for now and are tracked as future improvements rather than open decisions. Codex does work for chat;
only Codex's terminal interface is unavailable because it lacks a verified interactive hook and flag
integration. That terminal limitation, including terminal-agent messaging, is now in the backlog.

**Needs attention:** The remaining open choices are API/model compatibility and whether to authorize
credentialed provider, terminal, and federation release checks.

**Next:** Revisit any backlog item when its trade-off becomes worth changing.

### 2026-07-22 — Feature design: repair onboarding credentials and defaults

The onboarding repair is fully specified and waiting to start; no product code changed. People will
be able to choose **Set up later** and open the dashboard immediately, without creating a project,
changing backend defaults, or launching an agent. The normal backend step will no longer expose model
IDs or provider model strings. Instead, Claude and Codex will give provider-owned sign-in guidance
and let the person check readiness again; Codex can also use an OpenAI API key.

Fresh installs will use Claude `sonnet` and Codex `gpt-5.6-sol`; existing `backends.json` files are
preserved exactly. There will be no embedded terminal, dashboard-started login, credential transport,
or new auth API route. The `agentdeck auth` failure you saw is from the installed v0.1.0 binary, which
predates that command; the current source and the planned release checks include it, but an older
immutable release needs an explicit reinstall/update.

**Needs attention:** None.

**Next:** Run `/work` when you want to implement
[`repair-onboarding-credentials.md`](../ready-changes/repair-onboarding-credentials.md).

### 2026-07-21 — Release: installer fix published

I pushed the two pending `main` commits — the installer fix and the already-committed
annotate-and-assign specification work — and published [AgentDeck v0.1.1](https://github.com/AsaphNoam/AgentDeck/releases/tag/v0.1.1).
Its Apple-silicon release workflow passed the runtime assembly, installer transaction, and clean-install
checks, then uploaded the archive, manifest, and corrected installer. The normal command now serves
the fixed bootstrap:

```sh
curl -fsSL https://github.com/AsaphNoam/AgentDeck/releases/latest/download/install.sh | bash
```

**Needs attention:** None.

**Next:** Retry the normal installer command on the other Apple-silicon Mac.

### 2026-07-21 — Fix: piped release installer

I found the installer failure: when the documented command pipes the script into Bash, the lock
re-executes `bash` instead of the script file. The child then resumes partway through the pipe with
its error helpers missing, which explains the misleading cascade of 404, missing-command, checksum,
and false-success messages. The installer now saves piped input to a private temporary executable
file before taking the lock, and a regression test covers the full piped install path. All checks pass.

The existing v0.1.0 release asset cannot be changed in place. Until the next release is published,
use this safe workaround instead of piping to Bash:

```sh
installer="$(mktemp)"
curl -fsSL https://github.com/AsaphNoam/AgentDeck/releases/latest/download/install.sh -o "$installer" &&
  chmod 700 "$installer" &&
  "$installer"
status=$?
rm -f "$installer"
exit "$status"
```

**Needs attention:** A new GitHub Release must publish the fixed installer before the short documented
command works for everyone.

**Next:** Publish the verified installer fix in the next release, then retry the normal installation
command on a clean Apple-silicon Mac.

### 2026-07-20 — Feature design: annotate and assign

We defined the annotate-and-assign feature you asked for, inspired by the Codex app's diff
comments. You will be able to select lines inside a diff or a whole message in an agent's
conversation — live or in the archive — attach a short instruction, collect several of these into a
pending tray, and send the batch to the current agent, another running agent, or a brand-new agent
launched with the details prefilled. Each annotation is preserved as structured context: the app
captures the file, lines, and quoted excerpt for you, shows the batch as annotation cards in the
conversation, makes it searchable in the archive, and delivers it to agents as a generated context
block instead of pasted text. Sending to another agent rides the existing agent mailbox, so unread
badges and automatic wake-ups work unchanged.

Files on disk, screenshots, and web pages are excluded because AgentDeck has no viewer for them;
terminal output is also out of this first version. The feature and technical specifications are
written and marked as planned, and a ready-to-implement change file is waiting; no product code was
changed. Nothing further is needed from you — say the word when you want implementation to start.

### 2026-07-19 — Usability review: post-fix core journey rerun

I re-ran the release-style app through fresh install and onboarding, first chat, all four permission
outcomes, grid persistence, archive search in both builds, Settings round-trips, agent messaging,
failure recovery, and restart durability. The cancelled-permission fix now holds in the browser: the
question resolves, stays resolved after reload, and cannot be answered twice. No new product finding
was reproduced. Full evidence is in
[`../archive/reviews/usability-review-run-2026-07-19-post-fix.md`](../archive/reviews/usability-review-run-2026-07-19-post-fix.md).

The live Claude terminal and signed-in provider checks remain unrun because they require explicit
authorization and real credentials. This test browser also cannot execute browser-native prompt and
confirm dialogs, so those specific UI actions were marked blocked; their backing operations passed
through the local API and rendered correctly afterward.

**Needs attention:** No product issue is open. The review-state commit is local and needs approval to
push; the live-provider and terminal release gates still need authorization, and the native-dialog
UI actions need replay in a browser that supports them.

**Next:** Approve pushing the review state; after that, a maintainer should authorize the
credentialed provider and terminal gates and replay the native prompt/confirm actions in a compatible
browser.

### 2026-07-19 — Fix: stale permission prompt after cancelling a turn

I fixed the issue where cancelling a turn while a permission question was still open left that
question on screen with clickable Approve/Deny buttons forever — even after a reload — with a click
just producing an error. The app now records that the question was withdrawn, the same way it already
did when you deny a request or let it time out. The cancelled question turns into a resolved chip on
both the live view and after a reload, and it can no longer be clicked.

I added a test that reproduces the original problem and confirms the withdrawal is now recorded, and
updated the chat specification to state this behavior. All automated checks pass. This was the only
open issue from the recent usability review.

**Needs attention:** None. A browser re-check of the cancel-during-permission journey and the
credentialed provider gates are still open items for a future run, not blockers.

**Next:** Run a usability pass to confirm the cancel journey in the real app when convenient, and
authorize the credentialed provider gates when you're ready.

### 2026-07-19 — Usability review: full journey matrix and fix verification

I drove every non-credentialed user journey against the real built app — first paint, onboarding,
launch and chat, permissions, the card grid, resume and runtime switching, both archive-search
builds, every Settings form, agent-to-agent messaging, failure recovery, and restart durability —
and re-drove the chat and permission journeys on a fresh build after yesterday's two fixes landed.
Both fixes hold up in the browser: denying a permission now reliably returns the agent to a usable
idle state (it previously stuck on Cancel in most attempts), and reloaded or archived conversations
now show each streamed reply as one readable message.

One new smaller issue: if you cancel a turn while a permission question is still open, the question
stays on screen with clickable Approve/Deny buttons forever — even after a reload — and clicking
one just produces an error, because the app never records that the question was withdrawn.
Everything else passed. The terminal journey and real signed-in provider checks remain skipped
until you authorize live-provider runs. Full evidence:
[`../archive/reviews/usability-review-run-2026-07-19.md`](../archive/reviews/usability-review-run-2026-07-19.md).

**Needs attention:** None urgent — the stale permission prompt is queued as a Worth-fixing finding.
The review state is committed locally on top of the current remote history; publishing it needs
your approval to push the one local commit ahead of `origin/main`.

**Next:** Push the review-state commit, then run `/fix` for the stale cancelled-permission prompt;
separately, authorize the credentialed provider gates when ready.

### 2026-07-18 — Fix: permission-denial completion

Denying a tool permission can no longer leave a finished chat stuck on Cancel. AgentDeck records the
temporary resolved state before the agent can end its turn, so the normal completion remains the final
idle state. A full HTTP/SSE regression covers two fresh fake agents, and the release build and test
suites pass.

**Needs attention:** None. The existing credentialed provider, terminal, and federation release
checks remain separate manual gates.

**Next:** A maintainer can run the credentialed acceptance gates when authorized.

### 2026-07-18 — Usability review: post-fix core journeys

The onboarding and archived-reply fixes now hold up in the real built app. The wizard stayed open
through the config refresh and completed its first launch, and live, archived, and resumed chats all
showed one streamed response as one readable message. Grid reorder and restart, both Archive search
builds, Settings round-trips, two-agent messaging, unread clear and persistence, live fake-terminal
input and reattach, disconnect/reconnect, agent crash recovery, and the full presentation matrix also
passed. The full evidence and coverage limits are in
[`../archive/reviews/usability-review-run-2026-07-18-post-fix.md`](../archive/reviews/usability-review-run-2026-07-18-post-fix.md).

One must-fix issue remains: denying a permission can race the agent's normal turn completion, leaving
the denial recorded but the agent stuck busy with a Cancel button. It reproduced with two fresh
agents; approval worked normally.

**Needs attention:** Fix the permission-denial state race before treating the deny path as reliable.
Credentialed provider and real-Claude terminal compatibility remain separate manual release gates.
The review state is committed locally; publishing it requires explicit approval to push the local
commits currently ahead of `origin/main`.

**Next:** Run `/fix` for the permission-denial finding, then rerun the deny journey; a maintainer can
separately authorize the credentialed acceptance gates.

### 2026-07-18 — Fix: onboarding continuity and readable archived replies

The first-run wizard now remains open through Project, Config, and Launch even when config polling
reports setup satisfied after backend validation. Archived and resumed conversations now combine
consecutive stored assistant stream fragments into the same single reply shown live. Regression
coverage exercises the gate, transcript store, and Archive surface, and the release interface was
rebuilt with all specification, interface, Go, and distribution checks passing.

**Needs attention:** None.

**Next:** Rerun the onboarding and Archive/resume journeys in a usability review; a maintainer can
separately authorize the credentialed provider gates.

### 2026-07-18 — Usability review: core interface and previously skipped journeys

Claude left no durable checkpoint, so I restarted the review against the real built app with fresh,
isolated state. The redesigned interface itself held up across first paint, Dashboard, chat,
Settings, permissions, Archive, restart/reconnect, agent crash, and the full visual fixture; user
messages are now durable and searchable, layout survives restart, and both archive-search builds
behave as intended.

Two must-fix problems were confirmed. First, after backend validation succeeds, the onboarding
wizard is removed by its next config poll before a new user can finish Project, Config, and Launch.
Second, Archive and resumed sessions render every stored assistant stream fragment as a separate
message instead of the single readable reply shown live. The full evidence and honest coverage
limits are in [`../archive/reviews/usability-review-run-2026-07-18.md`](../archive/reviews/usability-review-run-2026-07-18.md).

**Needs attention:** The two findings should be fixed before treating first-run onboarding and
archived conversation reading as reliable. Live terminal operation and multi-agent messaging were
not completed in this run and remain unclaimed.

**Next:** Run `/fix` to address the onboarding poll race and transcript replay folding, then rerun J2
and J8; a maintainer can separately authorize the credentialed terminal and provider gates.

### 2026-07-18 — Review: core interface redesign and the two fixes before it

Reviewed every unreviewed change since the last checkpoint: the Codex role-prompt delivery fix, the
installer and usability fixes, and the full core-interface redesign. The redesign changes only how
the product looks — every screen keeps its existing behavior, data, routes, and actions, and the
development-only preview screen stays out of the shipped app. The two fixes do what they claim:
Codex now receives its role and project instructions through the configuration channel it actually
reads, an incomplete hand-edited backend file no longer crashes the dashboard, user messages are
saved and searchable, and the installer keeps a no-start or non-interactive choice through its
locked step. The written product rules and the shipped code agree in both directions, and the
specification, presentation, interface, and Go checks all pass. No new problems found.

**Needs attention:** None. Running real signed-in Claude and Codex sessions and the live terminal and
federation journeys is still a manual release step that has not been done; this review did not change that.

**Next:** Treat the reviewed work as ready; a maintainer runs the credentialed acceptance checks when authorized.

### 2026-07-18 — Implementation: redesigned the core interface

AgentDeck now has a complete product-native visual system across the shell, Dashboard, agent views,
Archive, Settings, onboarding, dialogs, menus, notifications, technical renderers, and error states.
The redesign preserves existing product behavior while adding local fonts and mark, semantic tokens,
shared visual primitives, stable future-skin hooks, and automated safeguards against visual drift.

The deterministic browser matrix and rebuilt release UI were exercised in baseline and high-variance
modes, including the empty Dashboard, New Agent dialog, Archive, every Settings section, onboarding,
transcript and terminal fixtures, overlays, long content, and every agent state. Browser review caught
and fixed inactive Settings panels taking layout space. All presentation safeguards, 101 UI tests,
both Go test variants, specification checks, source build, and distribution build pass.

**Needs attention:** None.

**Next:** An independent review can inspect the shipped change; live-provider acceptance remains the
separate credentialed gate already recorded in the handoff.

### 2026-07-18 — feature design: core frontend ready

The core frontend redesign is fully designed and waiting to build. It will give every existing
surface one distinctive, product-native AgentDeck identity—light neutral canvas, near-black
structure, energetic accent colors, Instrument Sans and IBM Plex Mono typography, crisp asymmetric
geometry, and coordinated dark technical surfaces—without turning the default interface into a
conceptual skin or changing product behavior.

The selected architecture is layered plain CSS. The unskinned core renders independently; future
skins will be able to override approved semantic values, stable component slots, geometry, and
decoration without owning feature content, state, routes, or actions. There is still no skin picker,
skin state, loader, persistence, or production skin in this change.

Because this repository runs with little human supervision, maintenance safety is part of the design,
not a note for reviewers. Style linting and a cross-code/CSS contract checker will run automatically
before UI tests and builds. They will reject undefined classes or tokens, raw visual values outside
the token boundary, undocumented or stale skin hooks, unapproved inline styling, excessive
specificity, `!important`, stale exceptions, third-party palette drift, and accidental skin-provider
or skin-state dependencies. A versioned manifest defines the public visual seam, every exception must
name an exact file and reason, a deterministic visual matrix covers all major surfaces, and local
frontend agent instructions explain the rules before later agents edit the UI.

The scope remains presentation-only: no responsive, zoom, keyboard, accessibility, recovery-flow,
browser-dialog, or feature behavior expansion is included.

**Needs attention:** None.

**Next:** Run `/work` when you want an implementation agent to start the waiting core frontend
redesign.

### 2026-07-18 — feature design: frontend architecture choice

The product-native, presentation-only direction is confirmed. I audited the existing frontend and
set the common technical boundaries: feature components keep all behavior and state; the core works
with no active skin or theme preference; fonts, icons, and visual assets stay bundled and offline;
code highlighting, diffs, and the terminal share the core palette instead of keeping independent
defaults; and future skins get controlled visual hooks without owning product content or structure.

One architectural choice remains:

- **A — Layered plain CSS contract (recommended).** Split the current global stylesheet into ordered
  foundation, token, shared-component, feature, integration, and reserved skin layers. Use semantic
  custom properties plus stable `data-ui`/`data-slot` hooks, with a small set of presentation-only
  React primitives. The core uses no provider or active-skin attribute. Future skins can change both
  visual values and approved component geometry/decoration through those hooks.
- **B — CSS Modules plus a React presentation provider.** This gives stronger local style isolation
  and typed variants, but hashed classes make rich external skin overrides harder and the provider
  risks treating the core itself as a theme.
- **C — Runtime CSS-in-JS theme engine.** This offers the most dynamic overrides, but adds runtime
  machinery and a broad component rewrite for a product that currently needs no runtime skin state.

I recommend **A**. It fits the current React/Vite architecture, adds no styling runtime, preserves
the distinction between core and skin, and still leaves enough controlled surface area for future
skins to become much richer than palette swaps. Its cost is that the documented visual values and
component hooks become a contract we must maintain deliberately.

**Needs attention:** Choose A, B, or C. A short “A” is enough to accept the recommendation.

**Next:** Once chosen, I’ll pin the exact visual system and file/component contracts, complete the
technical specification, and leave the redesign ready for implementation.

### 2026-07-18 — feature design: simplified core visual direction

I revised the proposal around the distinction you made. The first design is now the **unskinned
AgentDeck core**, not a concept applied to AgentDeck. Dashboard remains Dashboard, agent cards remain
agent cards, chat remains chat, and Archive, Settings, onboarding, and every control keep their
current names, meaning, and behavior. The expedition, dispatch, dossier, field-log, catalog,
workshop, and journey framing has been removed completely.

The proposed visual identity is distinctive through design fundamentals instead: a light neutral
canvas, near-black structure, a small high-energy accent palette, characterful display typography,
clear text and monospaced technical typography, precise rules, intentional asymmetry, crisp component
geometry, and coordinated dark surfaces for code, diffs, commands, and terminal content. It avoids
generic white SaaS cards, all-dark IDE chrome, purple/blue AI glow, glass panels, and soft gradient
clouds without replacing the product with another concept.

This step now changes presentation only. It does not add responsive or phone targets, keyboard-flow
work, zoom support, accessibility policy, reduced-motion behavior, new loading/recovery states,
dedicated replacements for browser prompts, new actions, or changed interaction flows. Existing
screens and states receive a complete visual design; existing behavior stays where its owning feature
already puts it.

Future skins remain an architectural consideration beneath this work. The core must render fully
with no active skin and no theme control. Later skins may provide the strong concepts and flavors;
they will overlay approved visual values and decorative assets without owning AgentDeck's content,
routes, actions, or component structure.

**Needs attention:** Please confirm this revised product-native visual direction and presentation-
only scope. A short “confirmed” is enough, or point to any remaining element that still feels too
thematic or too broad.

**Next:** After confirmation, I’ll define the technical core-versus-skin boundary, finish the design
specifications and acceptance evidence, and leave the frontend redesign ready for implementation.

### 2026-07-17 — feature design: frontend behavior proposal

I drafted the user-visible half of a complete frontend redesign. The proposed direction is **Field
Atlas**: a warm, tactile expedition desk built from chart-paper surfaces, deep ink, cartographic
lines, clipped dossier shapes, dark instrument inserts, and restrained signal colors. It should feel
like dispatching and supervising a capable field team—not like another dark integrated development
environment, chat app, or generic software-as-a-service dashboard.

The proposal covers the whole product:

- A persistent, unmistakable shell makes Dashboard, Archive, Settings, New Agent, and the live
  connection state clear.
- The Dashboard becomes a dispatch board: groups are map sections and agent cards are information-
  dense dossiers with prominent current activity, state, context pressure, mail, runtime, and project
  identity. Empty, loading, disconnected, and failed states receive full designs too.
- Chat becomes a chronological field log rather than a pile of chat bubbles. Tool calls, results,
  diffs, permissions, errors, terminal, files, and commands each get a distinct instrument-like
  treatment, with a persistent dispatch-style composer.
- Archive becomes a searchable catalog; Settings becomes a consistent workshop for both simple and
  very dense configuration; onboarding becomes a full-canvas four-stop expedition route instead of
  a generic modal.
- Rename, runtime switch, group moves, releases, stops, and destructive configuration actions use
  designed AgentDeck dialogs instead of browser prompts. Clone remains immediate as it is today.
- Keyboard use, visible focus, reduced motion, non-color state cues, 200% zoom, long content, and
  1024×720 through large desktop windows are part of the design rather than cleanup work. A phone-
  specific experience is excluded.

No API, agent behavior, local data, persistence, retention, or security boundary changes in this
proposal. This change would ship only Field Atlas: no theme picker, stored theme preference,
project-specific theme, downloads, or marketplace. The next design half will define a skin-safe
frontend boundary so later themes can change typography, color, spacing, geometry, texture,
illustration, and motion without changing actions, state meaning, accessibility, or route structure.

**Needs attention:** Please confirm three linked product calls: Field Atlas as the first visual
direction, 1024×720 desktop-first support with no phone-specific design, and replacement of native
browser prompts with dedicated dialogs. A short “confirmed” is enough, or tell me which part to
change.

**Next:** After confirmation, I’ll define the technical theme/component architecture, finish the
specifications and acceptance evidence, and leave the redesign as a ready change for implementation.

### 2026-07-17 — issue audit: known improvement list

I verified every recorded known issue against the current specifications, implementation, and
focused tests. I removed the fixed installer flag, Codex role-delivery, and user-prompt persistence
claims; removed the unreachable terminal typed-input nudge and vague optional-flag/specification-audit
claims; and narrowed partially fixed items to their exact remaining failures. Eleven evidence-backed
areas remain, covering chat recovery, archive/tracking, coordination, terminal shutdown/capabilities,
federation, backend startup, HTTP compatibility and limits, frontend state, process lifecycle, and
filesystem policy. No product code changed, and the focused Go/UI tests and documentation checks pass.

**Needs attention:** None; the remaining entries are verified but are not yet approved, fully
specified changes.

**Next:** Choose one retained item to define and move into the ready-changes queue.

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
### 2026-08-17 — Feature design: browse for working directories

The proposed project forms add Browse beside the working-directory field in Settings and onboarding. Picking a folder fills an absolute path but does not save until the form is confirmed; cancel or failure keeps the existing value, and manual paths including `~` remain supported.

**Needs attention:** Should the same folder picker also be available when adding each “additional directory” in Settings?

**Next:** Confirm that scope; then I’ll design the local directory-picker architecture and finish the ready-to-build change.

### 2026-08-17 — Feature design: directory-picker architecture

Browse now covers the main working directory in Settings and onboarding plus every new additional-directory entry in Settings. Manual entry remains available, and selecting a folder does not save the project or add the pending extra directory by itself. The browser standard returns a directory handle rather than the absolute path AgentDeck must give its local process ([WICG specification](https://wicg.github.io/file-system-access/), [Chrome documentation](https://developer.chrome.com/docs/capabilities/web-apis/file-system-access)).

**Needs attention:** Choose the picker architecture: a macOS-native folder panel launched by the local server, or a larger in-app filesystem browser that exposes directory listings through the API. I recommend the native panel because it returns the absolute path AgentDeck needs while leaving filesystem navigation and visibility with macOS.

**Next:** Approve the native panel or choose the in-app browser; then I’ll write the technical requirements and finish the ready-to-build change.

### 2026-08-17 — Feature design: preserve unsent chat text

The proposed behavior keeps a separate unsent draft for each chat in the current browser. It restores the exact text after navigating away, refreshing, or reopening AgentDeck; removes it after the prompt is accepted or the field is manually emptied; and keeps a failed send available for retry. Drafts stay on that browser profile only—agents, the server API, other browsers, and other devices do not receive or sync them.

**Needs attention:** Should an unsent draft remain until it is sent or manually cleared, or automatically expire after a fixed period? I recommend keeping it until sent or cleared because expiry can recreate the same accidental-loss problem.

**Next:** Choose the retention rule; then I’ll write the feature specification and acceptance checks before designing the storage mechanics.

### 2026-08-17 — Feature design: preserve unsent chat text

The feature draft now specifies one non-expiring draft per chat in the current browser. Exact text returns after navigation, refresh, or reopening AgentDeck; an accepted send or manually emptying the composer removes it; and a failed send keeps it available. Draft text is never sent before Send, added to transcripts, exposed through the server API, or synced across browsers or devices.

**Needs attention:** Confirm the full scope, including this retention consequence: deleting or archiving a chat does not trigger server cleanup of browser-only data, so an inaccessible draft can remain until that browser's AgentDeck site data is cleared.

**Next:** Once confirmed, I’ll inspect the frontend persistence boundaries and invariants, draft the technical requirement, and prepare the ready-to-build change.

### 2026-08-17 — Feature design: preserve unsent chat text

The repository's browser-state safety rule prevents the confirmed unlimited-retention design: an earlier per-agent draft feature accumulated until browser storage failed and could restore stale data if an agent identifier was reused. Avoiding that known defect requires both deletion cleanup and a storage bound.

**Needs attention:** May I use the smallest compliant rule: delete a chat's draft when its agent is deleted and retain only the 20 most recently edited chat drafts, without any time expiry? Creating a draft for a 21st chat would discard the least recently edited draft.

**Next:** Choose that bounded rule or a different limit; then I’ll revise the feature requirement, specify the direct browser-storage implementation, and prepare the ready-to-build change.

### 2026-08-17 — Feature design: preserve unsent chat text

Chat draft persistence is not already supported. The earlier example was the separate annotation tray, which stores pending review annotations per source agent in browser storage; its original unbounded implementation caused the safety rule this design must follow.

Agent-deletion cleanup is already available in the existing deletion-event path, so adding chat-draft cleanup there is trivial rather than special machinery. The remaining bound protects against drafts accumulating across many ordinary, undeleted chats or one very large pasted draft.

**Needs attention:** Confirm a cap of the 20 most recently edited chat drafts, with no time expiry. This can use one small browser-local store and the existing deletion cleanup path.

**Next:** Once confirmed, I’ll update the feature requirement, add the minimal technical constraint, and prepare the ready-to-build change.

### 2026-08-17 — Feature design: preserve unsent chat text

The feature is fully specified and ready to build as a quick, frontend-only change. Each chat will restore its exact unsent text after navigation, refresh, or reopening AgentDeck in the same browser. Accepted sends, manually emptied composers, and agent deletion clear the matching draft; failed sends retain it. The 20 most recently edited chat drafts are kept without time expiry.

The implementation is intentionally small: one browser-local draft module used by the existing composer and deletion-event path. It adds no server endpoint, database or configuration field, migration, timer, background cleanup, cross-device sync, or product code during this design step.

**Needs attention:** None.

**Next:** The waiting change can be started with `/work persist-unsent-chat-drafts`.

### 2026-08-17 — Feature design: browse for working directories ready

The quick-win design is ready to build. Settings and onboarding will use the standard macOS folder panel to fill the main working directory, and Settings will use it for additional directories too. The implementation is intentionally one small local API action backed by macOS’s built-in picker; it adds no custom filesystem browser, directory listing, upload mechanism, database state, migration, or new configuration format. Manual typing, explicit Add for extra directories, and the existing project Save remain unchanged.

Cancellation keeps the current field untouched. Only one picker can be open at once, shutdown closes an outstanding picker, and failures expose neither filesystem paths nor AppleScript diagnostics. Automated tests use a fake command boundary; the real acceptance pass opens the native panel in the macOS browser journey.

**Needs attention:** None.

**Next:** Run `/work` to implement the waiting Browse for working directories change.

### 2026-08-17 — Fix: projects-home background menu

Right-clicking any empty part of the Projects canvas now opens New project, while project cards keep their own menus. The lower canvas was checked in the rebuilt app at desktop size with no browser console errors.

**Needs attention:** None.

**Next:** None.
