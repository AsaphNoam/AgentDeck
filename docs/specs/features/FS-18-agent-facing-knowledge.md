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

### 2.2 Compatibility and lifecycle timing

**R7 — Only the exact historical AgentDecker prompt migrates, and only after package
verification succeeds.** On upgrade, AgentDeck replaces an existing `agentdecker` system prompt
only when the current dashboard start verified the package and the prompt bytes match the one
immediately preceding shipped seed prompt. The replacement is R2's thin prompt. The write preserves
title, role id, `skip_permissions`, and every other role field. A one-byte edit, empty/custom prompt,
different role, missing role, unreadable role, or unavailable package is untouched; users of those
roles may edit them manually. A later successful dashboard start retries the exact comparison and
migration. The migration is one-time, idempotent compatibility work rather than a managed-role or
recurring synchronization mechanism.

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
  exact historical prompt → R2 prompt; later startup → unchanged.
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

**A5** (R7, R10) — The exact legacy-prompt fixture migrates once, including when its
non-prompt fields were customized; successful migration preserves those fields byte-for-byte.
Fixtures with a one-byte prompt edit, empty/custom prompt, another role, a missing role, and a
read or write error remain unchanged. An unavailable package also leaves the exact fixture unchanged;
a later verified startup migrates it once.

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

## 6. Deviations & open decisions

- No UI, REST endpoint, MCP documentation tool, agent-facing release command, mutable knowledge
  store, or new runtime interface is introduced.
- Customized AgentDecker roles remain user-owned even if they contain a stale copy of product
  knowledge. AgentDeck does not infer that they should migrate.
- AgentDeck development and release-maintenance instructions belong to the repository's release
  workflow or its development skill, not to the shipped operator skill.

## 7. Traceability

Anchors: `internal/agentknowledge/package.go` and its embedded `operating-agentdeck` tree;
`config.MigrateLegacyAgentDecker`; `server.applyKnowledgeOverlay`; runtime-only launch fields and
`StartSystemPrompt`/`StartAddDirs`/`StartEnv`; `cli.prepareAgentKnowledge`; and the local tool
registrations in `internal/messaging/messaging.go`. Acceptance coverage lives in
`internal/agentknowledge/package_test.go`, `internal/config/config_test.go`,
`internal/cli/knowledge_test.go`, `internal/server/knowledge_overlay_test.go`, and runtime/terminal
parameter tests. Pinned credentialed provider discovery remains a manual release gate when logged-in
Claude and Codex providers are available. Governing requirements: FS-04.R1–R4/R13–R15; FS-06;
FS-14–FS-17; TS-11; and INV §1, §2, §4, §6, §8, §10, §11, and §15.
