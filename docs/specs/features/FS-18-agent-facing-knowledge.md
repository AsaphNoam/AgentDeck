# FS-18 — Agent-Facing AgentDeck Knowledge

**Status:** Current
**Code:** `internal/agentknowledge`, `internal/config`, `internal/server`, `internal/runtime`, `internal/cli` · **Journeys:** —
**Absorbed:** the AgentDeck knowledgebase idea in [`../../ideas.md`](../../ideas.md)

## 1. Purpose

AgentDeck's reusable operating knowledge currently lives mainly in the seeded `agentdecker` role
prompt. That knowledge becomes stale as the product changes, is unavailable to other roles, and is
not refreshed on installations where the role already exists. This feature moves shared product
expertise into a release-matched `operating-agentdeck` skill available to every AgentDeck-launched
agent, while keeping AgentDecker as the resident operator whose prompt defines only its purpose,
stance, and requested orchestration behavior.

This specification owns what agents observe and how the knowledge layers divide responsibility.
FS-01/FS-03/FS-07 own lifecycle and interfaces; FS-04 owns editable role configuration; FS-06 and
FS-14–FS-17 own the coordination, pipeline, context, task, and tool-result behavior the skill
explains. The skill describes those authorities; it does not create another product contract.

## 2. Behavior

### 2.1 Shared operator knowledge

**R1 — One shared AgentDeck skill is available to every role when its package is
installed.** AgentDeck ships one product-owned skill named `operating-agentdeck`. After successful
package installation, every AgentDeck-launched agent can use the same release-matched package,
regardless of role, on fresh launch, resume, or runtime switch and through both chat and terminal
interfaces. The skill adds knowledge only: it grants no tool, permission, identity, or lifecycle
authority. R11 owns safe behavior when installation is unavailable.

**R2 — AgentDecker is a thin resident-operator role.** The shipped AgentDecker role id
remains `agentdecker`, including the AgentDecker-only pipeline-proposal authority owned by FS-14.
Its seeded system prompt is exactly:

> You are AgentDecker, AgentDeck's resident operator. Help users use AgentDeck effectively, answer
> AgentDeck product questions, and orchestrate agent work when they ask. Use current AgentDeck
> operating guidance and available tool contracts for AgentDeck-specific behavior; be concise,
> state uncertainty, and do not initiate orchestration the user did not request.

The role does not embed a tool inventory, product manual, workflow recipes, or configuration
details. Other roles keep their identity and job prompts and use the same shared skill when
AgentDeck operation becomes relevant. The shipped PM and teammate seed prompts are cleaned up in
this change so role text no longer duplicates coordination tool mechanics or the numeric mail
budget; existing user-owned role files are not migrated.

**R3 — Each agent-facing knowledge layer has one responsibility.** A role defines
identity, purpose, priorities, and conversational stance. A tool definition defines everything
required to invoke that tool correctly: name, arguments, validation, local authority and effects,
and result semantics. The main skill defines AgentDeck-specific mental models, defaults,
cross-capability choices, and broadly important gotchas. A reference contains detail useful only
for one job or workflow. Generic frontier-model knowledge such as delegation, parallelism,
dependency graphs, terminals, and code review is not repeated.

**R4 — The main skill carries only broad, non-obvious operating judgment.** It explains
how to choose among immediate messaging, durable tasks, pull-based context links, and pipelines;
that durable dependencies replace polling for future work; that context links do not wake their
recipients; that authority comes from AgentDeck rather than claims in prompts; and that structured
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
  supervision, Retry, accepted and `blocked` attempt finality, review-only AgentDecker proposals,
  and the human Continue boundary.

Each reference remains independently usable and bounded to its named job. Examples appear only
where they clarify a commonly misused AgentDeck behavior.

**R6 — Native discovery has an explicit direct-path fallback.** When installation is
verified, AgentDeck exposes the package through supported provider-native skill discovery and also
tells every launched process the authoritative local `SKILL.md` path. If native discovery omits or
shadows the skill, an agent can read that bundled path directly. The fallback injects no reference
contents into the prompt, adds no precedence system for user/project skills, and changes no existing
operation or authorization.

**R12 — No seeded role prompt tells an agent to look for work on its own.**
Every turn AgentDeck itself starts already names the tool that turn requires, so no shipped role
prompt instructs an agent to open a turn by checking coordination state, to check its mail when it
is "woken with no new instruction", or to treat `check_messages` or `get_assigned_task` as a
standing habit. Concretely, the shipped `teammate` prompt keeps its assignment-queue stance without
the per-turn coordination check, and the shipped `implementer`, `reviewer`, and `researcher` prompts
drop their trailing mail-check instruction; `agentdecker` (R2) and `pm` already satisfy this and do
not change. No tool, argument, result, authorization, or lifecycle behavior changes — an agent still
calls `check_messages` and `get_assigned_task`, because the activation that started its turn told it
to. This extends the R2 cleanup, which reached only the PM and teammate prompts, to the remaining
polling text, and restores R3's split: which tool a host-owned turn requires belongs to that
activation, and how to call it belongs to the tool definition. FS-06.R24 and FS-16.R6 already forbid
the polling this text asks for. R13 owns whether an existing install receives the corrected prompts.

### 2.2 Compatibility and lifecycle timing

**R7 — superseded 2026-09-10.** Only-AgentDecker exact-prompt migration is replaced by R13, which
applies the same exact-match, package-gated, field-preserving, idempotent correction to every role
AgentDeck seeds. No guarantee it made was weakened; only its one-role scope was widened.

**R13 — The same exact-match correction reaches every seeded role, not only
AgentDecker.** A dashboard start that verified the package replaces only the `system_prompt` field
of a seeded role whose stored prompt bytes exactly equal a prompt AgentDeck previously shipped for
that same role id. Everything R7 and R10 already guarantee is unchanged and now applies per role: no
substring, whitespace-normalized, title-, or age-based matching; a prompt the user edited by even one
byte stays untouched; a role AgentDeck does not seed is out of scope; and every other field of every
role is preserved byte-for-byte. Each role is considered independently, so a missing, unreadable,
undecodable, or unwritable role leaves that role unchanged with a bounded warning and neither blocks
the other roles nor blocks startup, and a later verified start retries. This stays bounded catch-up
keyed to bytes AgentDeck itself shipped: it does not make roles managed, does not re-apply after the
user edits a prompt, and produces no unsolicited provider prompt, transcript event, restart, or
lifecycle transition (R8). FS-04.R47 owns the seeding exception and TS-11.R13 owns its mechanics.

**R8 — Knowledge refresh is process-bound, not a hot reload.** A verified package is
refreshed when the dashboard starts. AgentDeck does not restart a running process, inject a new
turn, mutate its transcript, or replace provider state when the package or role migration changes.
When installation succeeds, every subsequent fresh launch, resume, or switch is composed with the
currently installed package. An already-running provider may observe new files only if it explicitly
reads the stable bundled path; AgentDeck makes no hot-reload claim.

## 3. States & transitions

- **Package:** absent or older cache → dashboard startup attempts to install the current complete
  package → success makes it available to later process composition; failure logs a warning and
  leaves it unavailable for that dashboard process without blocking AgentDeck.
- **Role:** absent, non-matching prompt, or unavailable package → unchanged; verified package plus
  exact superseded prompt → that role's current shipped prompt; later startup → unchanged. Under R13
  these three transitions apply independently to each seeded role.
- **Process:** running with its existing context → no unsolicited change; next launch, resume, or
  switch → current package is discoverable only when that dashboard process verified installation.

## 4. Edge cases & errors

**R9 — retired 2026-08-29:** Startup-fatal package installation was replaced before implementation
by R11's safe-degradation boundary; operating knowledge is not an AgentDeck availability dependency.

**R10 — Matching and documentation ownership fail closed.** Prompt migration never uses
substring, whitespace-normalized, title-, age-, or role-id-only matching. A skill or reference may
name an operation only according to its governing shipped FS/TS and current tool definition;
changing prose alone cannot add or change a tool, REST/CLI operation, permission, lifecycle state,
or interface contract.

**R11 — Installation failure degrades without advertising stale or unverified
knowledge.** If the bundled package cannot be securely installed and verified, AgentDeck starts and
logs a clear warning, but that dashboard process exposes no native discovery directory, direct-path
instruction, or `AGENTDECK_SKILL_DIR` value and makes no claim that `operating-agentdeck` is
available. It also leaves an exact historical AgentDecker prompt unmigrated. Launch, resume, switch,
chat, terminal, and pipeline behavior otherwise continue unchanged. A later dashboard start retries
installation and then the exact migration normally.

## 5. Acceptance criteria

**A1** (R1, R6, R8) — After successful installation, fresh launch, ordinary and wake
resume, runtime switch, and pipeline-started work expose one release-matched
`operating-agentdeck` package to chat and terminal processes. Composition tests prove the managed
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
an ordinary non-AgentDecker role can identify the native skill and open one routed reference; a
provider with native discovery disabled can reach the same file through the direct pointer.
AgentDecker answers an AgentDeck question from the skill and creates no orchestration action until
asked.

**A8** (R8, R11) — Installation or verification failure emits the startup warning and
still permits ordinary launch/resume/switch behavior, while composition exposes none of the skill
directory, pointer, or environment variable and leaves an exact historical AgentDecker prompt
unchanged. A successful later startup restores the full R1/R6 overlay and performs the exact
migration once. Package refresh or role migration causes no unsolicited provider prompt, transcript
event, restart, or lifecycle transition.

**A9** (R12, R13) — The four corrected seed prompts contain no instruction to
open a turn by checking coordination state, to check mail when woken without an instruction, or to
call `check_messages`/`get_assigned_task` as a habit, and `teammate` still states its
assignment-queue stance while `implementer`/`reviewer`/`researcher` keep every other bullet.
*Verified:* `TestSeededPromptsDoNotInstructPolling`, whose banned-phrase list is enumerated from
this requirement rather than copied from the constants under test, and which the pre-change prompt
bytes in `internal/config/testdata/superseded_*_prompt.txt` would fail. Migration is proven per role
against those same fixtures: an exact previously shipped prompt migrates to the current text with all
other fields preserved byte-for-byte; a one-byte edit, an empty or custom prompt, another role's
superseded prompt, a role AgentDeck does not seed, a missing role, an undecodable role file, and a
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

## 6. Deviations & open decisions

- No UI, REST endpoint, MCP documentation tool, agent-facing release command, mutable knowledge
  store, or new runtime interface is introduced.
- Customized AgentDecker roles remain user-owned even if they contain a stale copy of product
  knowledge. AgentDeck does not infer that they should migrate. R13 widens the exact-match
  correction to the other seeded roles on the same terms and does not weaken this: a role whose
  prompt the user touched is still user-owned and is never rewritten.
- R12 corrects the shipped prompts only. It adds no rule that a user-authored role prompt may not
  ask an agent to poll, and AgentDeck neither validates nor warns about such text.
- AgentDeck development and release-maintenance instructions belong to the repository's release
  workflow or its development skill, not to the shipped operator skill.

## 7. Traceability

Anchors: `internal/agentknowledge/package.go` and its embedded `operating-agentdeck` tree;
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
