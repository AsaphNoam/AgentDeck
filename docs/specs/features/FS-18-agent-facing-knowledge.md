# FS-18 — Agent-Facing Chuck Knowledge

**Status:** Partial
**Code:** `internal/agentknowledge`, `internal/config`, `internal/server`, `internal/runtime`, `internal/cli` · **Journeys:** —
**Absorbed:** the Chuck knowledgebase idea in [`../../ideas.md`](../../ideas.md)

## 1. Purpose

Chuck's reusable operating knowledge currently lives mainly in the seeded `chucky` role
prompt. That knowledge becomes stale as the product changes, is unavailable to other roles, and is
not refreshed on installations where the role already exists. This feature moves shared product
expertise into a release-matched `operating-chuck` skill available to every Chuck-launched
agent, while keeping Chucky as the resident operator whose prompt defines only its purpose,
stance, and requested orchestration behavior.

This specification owns what agents observe and how the knowledge layers divide responsibility.
FS-01/FS-03/FS-07 own lifecycle and interfaces; FS-04 owns editable role configuration; FS-06 and
FS-14–FS-17 own the coordination, pipeline, context, task, and tool-result behavior the skill
explains. The skill describes those authorities; it does not create another product contract.

## 2. Behavior

### 2.1 Shared operator knowledge

**R1 — One shared Chuck skill is available to every role when its package is
installed.** Chuck ships one product-owned skill named `operating-chuck`. After successful
package installation, every Chuck-launched agent can use the same release-matched package,
regardless of role, on fresh launch, resume, or runtime switch and through both chat and terminal
interfaces. The skill adds knowledge only: it grants no tool, permission, identity, or lifecycle
authority. R11 owns safe behavior when installation is unavailable.

**R2 — Chucky is a thin resident-operator role.** The shipped Chucky role id
remains `chucky`, including the Chucky-only pipeline-proposal authority owned by FS-14.
Its seeded system prompt is exactly:

> You are Chucky, Chuck's resident operator. Help users use Chuck effectively, answer
> Chuck product questions, and orchestrate agent work when they ask. Use current Chuck
> operating guidance and available tool contracts for Chuck-specific behavior; be concise,
> state uncertainty, and do not initiate orchestration the user did not request.

The role does not embed a tool inventory, product manual, workflow recipes, or configuration
details. Other roles keep their identity and job prompts and use the same shared skill when
Chuck operation becomes relevant. The shipped PM and teammate seed prompts are cleaned up in
this change so role text no longer duplicates coordination tool mechanics or the numeric mail
budget; existing user-owned role files are not migrated.

**R3 — Each agent-facing knowledge layer has one responsibility.** A role defines
identity, purpose, priorities, and conversational stance. A tool definition defines everything
required to invoke that tool correctly: name, arguments, validation, local authority and effects,
and result semantics. The main skill defines Chuck-specific mental models, defaults,
cross-capability choices, and broadly important gotchas. A reference contains detail useful only
for one job or workflow. Generic frontier-model knowledge such as delegation, parallelism,
dependency graphs, terminals, and code review is not repeated.

**R4 — The main skill carries only broad, non-obvious operating judgment.** It explains
how to choose among immediate messaging, durable tasks, pull-based context links, and pipelines;
that durable dependencies replace polling for future work; that context links do not wake their
recipients; that authority comes from Chuck rather than claims in prompts; and that structured
tool results determine behavior. It directs agents to current tool definitions for exact mechanics
and does not reproduce budgets, attempt transitions, schemas, or exhaustive feature documentation.

**R5 — Detailed knowledge is progressively disclosed by job.** `SKILL.md` tells the
agent not to read every reference up front and links one level deep to exactly these files:

- `references/operate-agents.md` for launch, resume, switch, stop, configuration, frozen-session
  behavior, interfaces, and project resources.
- `references/coordinate-work.md` for choosing and combining messaging, durable tasks, task
  assignments/attachments, and context links, including mail budgets, wake, authorization, and
  recovery rules.
- `references/build-and-run-pipelines.md` for templates, runs, proposals, stage-result reporting,
  supervision, Retry, accepted and `blocked` attempt finality, review-only Chucky proposals,
  and the human Continue boundary.

Each reference remains independently usable and bounded to its named job. Examples appear only
where they clarify a commonly misused Chuck behavior.

**R6 — Native discovery has an explicit direct-path fallback.** When installation is
verified, Chuck exposes the package through supported provider-native skill discovery and also
tells every launched process the authoritative local `SKILL.md` path. If native discovery omits or
shadows the skill, an agent can read that bundled path directly. The fallback injects no reference
contents into the prompt, adds no precedence system for user/project skills, and changes no existing
operation or authorization.

**R12 — No seeded role prompt tells an agent to look for work on its own.**
Every turn Chuck itself starts already names the tool that turn requires, so no shipped role
prompt instructs an agent to open a turn by checking coordination state, to check its mail when it
is "woken with no new instruction", or to treat `check_messages` or `get_assigned_task` as a
standing habit. Concretely, the shipped `teammate` prompt keeps its assignment-queue stance without
the per-turn coordination check, and the shipped `implementer`, `reviewer`, and `researcher` prompts
drop their trailing mail-check instruction; `chucky` (R2) and `pm` already satisfy this and do
not change. No tool, argument, result, authorization, or lifecycle behavior changes — an agent still
calls `check_messages` and `get_assigned_task`, because the activation that started its turn told it
to. This extends the R2 cleanup, which reached only the PM and teammate prompts, to the remaining
polling text, and restores R3's split: which tool a host-owned turn requires belongs to that
activation, and how to call it belongs to the tool definition. FS-06.R24 and FS-16.R6 already forbid
the polling this text asks for. R13 owns whether an existing install receives the corrected prompts.

### 2.2 Compatibility and lifecycle timing

**R7 — superseded 2026-09-10.** Only-Chucky exact-prompt migration is replaced by R13, which
applies the same exact-match, package-gated, field-preserving, idempotent correction to every role
Chuck seeds. No guarantee it made was weakened; only its one-role scope was widened.

**R13 — The same exact-match correction reaches every seeded role, not only
Chucky.** A dashboard start that verified the package replaces only the `system_prompt` field
of a seeded role whose stored prompt bytes exactly equal a prompt Chuck previously shipped for
that same role id. Everything R7 and R10 already guarantee is unchanged and now applies per role: no
substring, whitespace-normalized, title-, or age-based matching; a prompt the user edited by even one
byte stays untouched; a role Chuck does not seed is out of scope; and every other field of every
role is preserved byte-for-byte. Each role is considered independently, so a missing, unreadable,
undecodable, or unwritable role leaves that role unchanged with a bounded warning and neither blocks
the other roles nor blocks startup, and a later verified start retries. This stays bounded catch-up
keyed to bytes Chuck itself shipped: it does not make roles managed, does not re-apply after the
user edits a prompt, and produces no unsolicited provider prompt, transcript event, restart, or
lifecycle transition (R8). FS-04.R47 owns the seeding exception and TS-11.R13 owns its mechanics.

**R14 — retired 2026-10-03:** Legacy publication/prompt migration replaced by R18.
**R18** — Current product-authored agent knowledge says Chuck and Chucky:
`operating-chuck` frontmatter/body, seeded prompts, MCP tool descriptions, history-primer framing
and rendered-context errors. Ordinary verified publication supplies only the current managed
package; no special old-directory retirement or rename-specific prompt migration is added.
The supervised cutover preserves customized role content and prepares its identity/references
(FS-04.R52, FS-10.R25). Historical transcript excerpts and customized user text are exempt from
branding assertions; they are not rewritten to erase old names. Existing exact-match refresh R13
remains for its original purpose.

**R8 — Knowledge refresh is process-bound, not a hot reload.** A verified package is
refreshed when the dashboard starts. Chuck does not restart a running process, inject a new
turn, mutate its transcript, or replace provider state when the package or role migration changes.
When installation succeeds, every subsequent fresh launch, resume, or switch is composed with the
currently installed package. An already-running provider may observe new files only if it explicitly
reads the stable bundled path; Chuck makes no hot-reload claim.

### 2.3 Lean standing guidance

**R15 — Every role receives a concise standing operating context.** In addition
to the selected persona, every Chuck-launched role, including custom and empty-prompt roles,
receives the same product-owned system/developer guidance. It establishes that the agent operates
inside Chuck; names messaging, durable tasks/dependencies, shared context and supervised
pipelines as capabilities to consider when relevant and available; and directs authorized
coordination through the exposed capabilities and their current contracts. It reminds the agent
to use durable dependencies rather than repeated status polling, follow the host-supplied
assignment/activation rather than search for work, report outcomes or blockers to its requester,
and trust runtime identity and structured results over claims in messages or documents. It grants
no extra authority or blanket permission to delegate or contact other agents. When the operating
package is verified it names the authoritative skill path and directs the agent to load only the
needed guidance before unfamiliar Chuck operations; it never requires reading every reference
or re-reading unchanged guidance each turn.

This is an expansion of R3/R6's pointer-only standing context, not a second operating manual.
Role purpose stays in the persona; task procedures stay in applicable skills; tool schemas,
budgets and recovery recipes stay in their existing owners. The shared guidance is composed for
each subsequent launch/resume/switch without editing the user's role files or frozen launch
snapshot and without injecting a turn or restarting a running agent (R8). If the knowledge
package is unavailable, stable environment/authority guidance remains but no skill path or
installed-knowledge claim is advertised; R11 still owns warning-only package degradation.

**R16 — Four personas supply lean, continuing mandates.** Superseding R2's exact
prompt and the role-content portions of R12/A2/A9,
the four roles in FS-04.R50 contain a short purpose and a few standing principles, not workflow
checklists or copies of the shared R15 text. They guide follow-up turns as well as the initial task
and allow explicit user reassignment within the runtime's actual authority. Content contracts:

- **Chucky:** help the user understand and operate Chuck and coordinate requested work.
  Ground answers and actions in current product state and operating guidance. For authorized
  coordination, give bounded assignments with relevant context and completion criteria, delegate
  where there is a concrete benefit, reconcile returned evidence and retain responsibility for
  the combined outcome. State blockers and uncertainty; ordinary product questions do not initiate
  orchestration. There is no separate default coordinator.
- **Implementer:** complete the requested change within existing architecture and conventions.
  Read relevant local guidance and code, preserve unrelated work, keep changes focused, and verify
  the changed behavior with appropriate checks. Do not introduce speculative features or unrelated
  refactors. Report what changed, actual verification and remaining limits; passing a check is not
  a claim that untested behavior is proven. Test changes are judged by intended behavior, not a
  blanket prohibition on editing tests.
- **Reviewer:** assess the assigned scope against requirements and actual behavior without
  silently becoming its implementer. Examine relevant surrounding code and callers; report
  actionable findings with evidence, location, concrete consequence and severity. Re-check likely
  findings and distinguish uncertainty and optional improvements from confirmed defects. Avoid
  personal-style nits and invented findings; a review may find none. State material coverage gaps
  and keep assessing unresolved findings across follow-up exchanges unless reassigned.
- **Researcher:** investigate internal code/spec/history questions and external documentation or
  research questions with the same evidence discipline. Follow relevant execution paths internally;
  prefer authoritative primary sources and check applicable versions/dates externally. Read the
  evidence behind important claims, distinguish observation from inference, surface contradictions
  and uncertainty, and give concise findings with file locations or direct source links. Scale
  effort to the question and stop when it is answered or the remaining gap is clear. Do not make
  implementation or external changes unless explicitly assigned; requested research artifacts are
  permitted within scope.

These are content requirements, not byte-exact prompts. Keep each role to a short paragraph or a
few bullets, ordinarily about 80–130 words, without adding model-specific delegation defaults,
invented credentials, obligatory praise, output bureaucracy or compulsory clarification before
routine work. The user's task and relevant skills supply detailed workflows. R12's no-polling
and no-self-assigned-work constraints remain in force for all seeded prompts.

**R17 — Chuck guidance supplements native provider instructions.** Normal
persona/context delivery preserves the provider's native coding-agent instructions while adding
the Chuck shared context, role and project guidance in the provider-supported governing
instruction layer. This applies to Claude chat and terminal and Codex chat; it introduces no new
runtime or role permission. Launch and resume must request additive delivery rather than silently
replace the provider's native prompt. Running sessions are not hot-rewritten; provider-owned
prompt snapshots on resume must be verified and any delayed adoption recorded honestly, without
discarding conversation history or claiming a changed effective prompt merely because a request
was sent.

**R19** (planned) — Agents receive Think Tank operating judgment through the same verified shared
skill, with `references/think-tanks.md` linked for that job. This extends R5's three-file inventory;
it does not make every agent load every reference. Explain room authority versus private sessions,
explicit contribution/leave, incremental reads, ceilings rather than quotas, independent openings,
intervention and final-only judge. Current tool definitions own invocation details. Guidance grants
no ability to create/end/delete rooms, edit membership, poll progress or manufacture consensus.

## 3. States & transitions

- **Package:** absent or older cache → dashboard startup attempts to install the current complete
  package → success makes it available to later process composition; failure logs a warning and
  leaves it unavailable for that dashboard process without blocking Chuck.
- **Role:** absent, non-matching prompt, or unavailable package → unchanged; verified package plus
  exact superseded prompt → that role's current shipped prompt; later startup → unchanged. Under R13
  these three transitions apply independently to each seeded role.
- **Process:** running with its existing context → no unsolicited change; next launch, resume, or
  switch → current package is discoverable only when that dashboard process verified installation.

## 4. Edge cases & errors

**R9 — retired 2026-08-29:** Startup-fatal package installation was replaced before implementation
by R11's safe-degradation boundary; operating knowledge is not a Chuck availability dependency.

**R10 — Matching and documentation ownership fail closed.** Prompt migration never uses
substring, whitespace-normalized, title-, age-, or role-id-only matching. A skill or reference may
name an operation only according to its governing shipped FS/TS and current tool definition;
changing prose alone cannot add or change a tool, REST/CLI operation, permission, lifecycle state,
or interface contract.

**R11 — Installation failure degrades without advertising stale or unverified
knowledge.** If the bundled package cannot be securely installed and verified, Chuck starts and
logs a clear warning, but that dashboard process exposes no native discovery directory, direct-path
instruction, or `CHUCK_SKILL_DIR` value and makes no claim that `operating-chuck` is
available. It also leaves an exact historical Chucky prompt unmigrated. Launch, resume, switch,
chat, terminal, and pipeline behavior otherwise continue unchanged. A later dashboard start retries
installation and then the exact migration normally.

## 5. Acceptance criteria

**A1** (R1, R6, R8) — After successful installation, fresh launch, ordinary and wake
resume, runtime switch, and pipeline-started work expose one release-matched
`operating-chuck` package to chat and terminal processes. Composition tests prove the managed
directory and direct pointer are added once through a shared path.

**A2** (R2–R5) — The seeded role prompt is byte-equal to R2 and contains no product
manual. `SKILL.md` contains only the R4 decisions, routes exactly the three R5 references, and
neither duplicates tool schemas nor requires all references to be loaded. Messaging budgets appear
only in `coordinate-work.md`; pipeline-only `blocked`/Continue/proposal guidance appears only in
`build-and-run-pipelines.md`, while tool definitions retain only local invocation mechanics. Fresh
PM and teammate seed prompts retain their job stance without repeating coordination tool mechanics
or the numeric mail budget.

**A3** (R3, R6, R10–R11) — Tool names, arguments, authority, effects, and results are
unchanged by skill availability. After successful installation, native discovery may be absent or
shadowed while the direct bundled path remains readable; after failed installation, neither route
is advertised. No role/skill text grants an unavailable operation.

**A4** (R5) — Package checks prove all three linked references exist, are readable
directly from the core skill, name their intended jobs, and do not embed the other references
wholesale. No development/release-maintenance reference ships in the package.

**A5** (R10, R13) — The exact superseded-prompt fixture migrates once, including when its
non-prompt fields were customized; successful migration preserves those fields byte-for-byte.
Fixtures with a one-byte prompt edit, empty/custom prompt, another role, a missing role, a role
file that cannot be decoded, and a read or write I/O error remain unchanged. An unavailable
package also leaves the exact fixture unchanged; a later verified startup migrates it once. A9
covers the same criteria for the other seeded roles.

**A6 — retired 2026-08-29:** The startup-fatal installation check was replaced by A8 before
implementation.

**A7** (R1–R6) — After successful installation, with pinned Claude and Codex providers,
an ordinary non-Chucky role can identify the native skill and open one routed reference; a
provider with native discovery disabled can reach the same file through the direct pointer.
Chucky answers a Chuck question from the skill and creates no orchestration action until
asked.

**A8** (R8, R11) — Installation or verification failure emits the startup warning and
still permits ordinary launch/resume/switch behavior, while composition exposes none of the skill
directory, pointer, or environment variable and leaves an exact historical Chucky prompt
unchanged. A successful later startup restores the full R1/R6 overlay and performs the exact
migration once. Package refresh or role migration causes no unsolicited provider prompt, transcript
event, restart, or lifecycle transition.

**A10 — retired 2026-10-03:** Rename-specific publication migration replaced by A14.
**A14** (R18) — Dashboard startup publishes verified `operating-chuck` through the
ordinary managed-root replacement, with no old skill in that active root. Product-authored skill,
seed, tool-description, primer-framing and error constants contain no old branding; historical
excerpts/custom prompts are preserved. *Verify by* the existing package-publication fixture and a
bounded assertion over those authored surfaces, including startup from a prepared cutover home.

**A9** (R12, R13) — The four corrected seed prompts contain no instruction to
open a turn by checking coordination state, to check mail when woken without an instruction, or to
call `check_messages`/`get_assigned_task` as a habit, and `teammate` still states its
assignment-queue stance while `implementer`/`reviewer`/`researcher` keep every other bullet.
*Verified:* `TestSeededPromptsDoNotInstructPolling`, whose banned-phrase list is enumerated from
this requirement rather than copied from the constants under test, and which the pre-change prompt
bytes in `internal/config/testdata/superseded_*_prompt.txt` would fail. Migration is proven per role
against those same fixtures: an exact previously shipped prompt migrates to the current text with all
other fields preserved byte-for-byte; a one-byte edit, an empty or custom prompt, another role's
superseded prompt, a role Chuck does not seed, a missing role, an undecodable role file, and a
read or write I/O error each leave that role unchanged; a failure on one role still migrates the
remaining ones and does not fail startup; an unavailable package leaves the roles unchanged and a
later verified start migrates them once; and re-running the pass is idempotent. A drift guard
re-derives every shipped digest from the fixture bytes and fails if the table names an unseeded role
or lists a role's current prompt as superseded. *Verified:*
`TestSupersededDigestTableMatchesSeededRoles`, `TestMigrateSupersededRolePromptsExactOnly`,
`TestMigrateSupersededRolePromptsIsolatesPerRoleFailure`,
`TestMigrateSupersededRolePromptsMissingRolesAndIdempotence`,
`TestMigrateSupersededRolePromptsSkipsUnseededRole`,
`TestMigrateSupersededRolePromptsReportsUnseededTableEntry`, and
`TestPrepareAgentKnowledgeFailureThenRetry`.

**A11** (R15, R8, R11) — A fresh implementer, a custom empty-prompt role, and a
resumed legacy role each receive the shared operating context once and the correct skill pointer
when the package is available. Repeated resume/switch does not duplicate it. An unavailable
package retains only truthful environment/authority guidance and no path/availability claim.
Role files, frozen snapshots, permissions and transcripts are unchanged by the overlay. *Verify
by* launch/resume/switch composition and package-failure tests across chat and terminal paths.

**A12** `(planned)` (R15–R16) — Fixed manual scenarios exercise: a Chucky product question
without initiating coordination; requested coordination with bounded delegation and synthesis; a focused
implementation preserving unrelated edits; review of a change with one known defect and one
non-defect, followed by a fix response; an internal feature trace; and external version-specific
research with conflicting sources. Each scenario includes a later follow-up that does not repeat
the role instructions. Check the role's boundaries, actual evidence, uncertainty, relevant skill
use and final outcome against R16, without asserting universal adherence or guaranteed review
accuracy. *Verify by* a documented pinned-provider prompt evaluation on Claude and Codex, plus
content guards against polling instructions and duplicated Chuck manuals. Tests do not enforce
the advisory word target or mirror complete prompt strings.

**A13** `(planned)` (R17) — Fresh and resumed Claude chat sessions request the native coding
preset with Chuck additions; Claude terminal retains its additive flag; Codex retains its
developer-instruction composition without a base-prompt replacement. *Verify by* pinned adapter
contract and runtime parameter tests plus a credentialed fresh/resume provider check, recording
any native snapshot limitation rather than claiming delivery from model self-report alone.

**A15** (planned; R1, R3–R6, R19) — Verify the new progressively linked reference in embedded,
installed and overlay package inventories, readable on fresh/resumed normal sessions. Exercise a
room participant and end-only judge without persona-specific mechanics: each reads through room
tools and explicitly stages its own contribution, preserves ceilings and unresolved objections,
and claims no user-only authority. *Verified by:* package/overlay/tool-definition fixtures and
TS-06.R33's bounded credentialed room smoke.

## 6. Deviations & open decisions

- R15–R17 and FS-04.R50–R51 were confirmed on 2026-10-02: coordination stays in Chucky and
  existing legacy roles/references are preserved. The change introduces no permissions, model
  presets, background work or new role configuration schema. Naming follows the independently
  selected product-rename change.

- No UI, REST endpoint, MCP documentation tool, agent-facing release command, mutable knowledge
  store, or new runtime interface is introduced.
- Customized Chucky roles remain user-owned even if they contain a stale copy of product
  knowledge. Chuck does not infer that they should migrate. R13 widens the exact-match
  correction to the other seeded roles on the same terms and does not weaken this: a role whose
  prompt the user touched is still user-owned and is never rewritten.
- R12 corrects the shipped prompts only. It adds no rule that a user-authored role prompt may not
  ask an agent to poll, and Chuck neither validates nor warns about such text.
- Chuck development and release-maintenance instructions belong to the repository's release
  workflow or its development skill, not to the shipped operator skill.

## 7. Traceability

Anchors: `internal/agentknowledge/package.go` and its embedded `operating-chuck` tree;
`config.MigrateSupersededRolePrompts`; `server.applyKnowledgeOverlay`; runtime-only launch fields and
`StartSystemPrompt`/`StartAddDirs`/`StartEnv`; `cli.prepareAgentKnowledge`; and the local tool
registrations in `internal/messaging/messaging.go`. Acceptance coverage lives in
`internal/agentknowledge/package_test.go`, `internal/config/config_test.go`,
`internal/cli/knowledge_test.go`, `internal/server/knowledge_overlay_test.go`, and runtime/terminal
parameter tests. Pinned credentialed provider discovery remains a manual release gate when logged-in
Claude and Codex providers are available. R12's corrected prompt text and R13's digest table both
live in `internal/config/seed.go`. Governing requirements: FS-04.R1–R4/R13–R15/R47; FS-06.R24;
FS-14–FS-17 including FS-16.R6; TS-11 and its R13; and INV §1, §2, §4, §6, §8, §10, §11, §15,
and §17.
