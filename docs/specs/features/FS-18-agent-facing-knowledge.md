# FS-18 — Agent-Facing AgentDeck Knowledge

**Status:** Partial
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

**R1 `(planned)` — One shared AgentDeck skill is available to every role.** AgentDeck ships one
product-owned skill named `operating-agentdeck`. Every AgentDeck-launched agent can use the same
release-matched package, regardless of role, on fresh launch, resume, or runtime switch and through
both chat and terminal interfaces. The skill adds knowledge only: it grants no tool, permission,
identity, or lifecycle authority.

**R2 `(planned)` — AgentDecker is a thin resident-operator role.** The shipped AgentDecker role id
remains `agentdecker`, including the AgentDecker-only pipeline-proposal authority owned by FS-14.
Its seeded system prompt is exactly:

> You are AgentDecker, AgentDeck's resident operator. Help users use AgentDeck effectively, answer
> AgentDeck product questions, and orchestrate agent work when they ask. Use the bundled
> operating-agentdeck skill and the currently available tool contracts for AgentDeck-specific
> behavior; be concise, state uncertainty, and do not initiate orchestration the user did not
> request.

The role does not embed a tool inventory, product manual, workflow recipes, or configuration
details. Other roles keep their existing identity and job prompts and use the same shared skill when
AgentDeck operation becomes relevant.

**R3 `(planned)` — Each agent-facing knowledge layer has one responsibility.** A role defines
identity, purpose, priorities, and conversational stance. A tool definition defines everything
required to invoke that tool correctly: name, arguments, validation, local authority and effects,
and result semantics. The main skill defines AgentDeck-specific mental models, defaults,
cross-capability choices, and broadly important gotchas. A reference contains detail useful only
for one job or workflow. Generic frontier-model knowledge such as delegation, parallelism,
dependency graphs, terminals, and code review is not repeated.

**R4 `(planned)` — The main skill carries only broad, non-obvious operating judgment.** It explains
that tool identity is derived from the launch token rather than caller claims; messaging is the
conversation/wake plane and has per-turn budgets; durable tasks replace polling for dependent or
future work; context links are pull-only and do not wake recipients; pipeline reports are final for
their current attempt, including `blocked`; human Continue creates a new attempt; AgentDecker
pipeline proposals are review records and cannot save or start work; and typed `retry.class`, not
English prose, determines retry behavior. It directs agents to current tool definitions for local
call mechanics and does not reproduce schemas or exhaustive feature documentation.

**R5 `(planned)` — Detailed knowledge is progressively disclosed by job.** `SKILL.md` tells the
agent not to read every reference up front and links one level deep to exactly these files:

- `references/operate-agents.md` for launch, resume, switch, stop, configuration, frozen-session
  behavior, interfaces, and project resources.
- `references/coordinate-work.md` for choosing and combining messaging, durable tasks, task
  assignments/attachments, and context links, including wake, authorization, and recovery rules.
- `references/build-and-run-pipelines.md` for templates, runs, proposals, stage-result reporting,
  supervision, Retry, and the human Continue boundary.
- `references/maintain-agent-knowledge.md` for reviewing shipped FS changes and deciding which
  agent-facing knowledge layer, if any, must change for a release.

Each reference remains independently usable and bounded to its named job. Examples appear only
where they clarify a commonly misused AgentDeck behavior.

**R6 `(planned)` — Native discovery has an explicit direct-path fallback.** AgentDeck exposes the
package through supported provider-native skill discovery and also tells every launched process the
authoritative local `SKILL.md` path. If native discovery omits or shadows the skill, an agent can
read that bundled path directly. The fallback injects no reference contents into the prompt, adds
no precedence system for user/project skills, and changes no existing operation or authorization.

### 2.2 Compatibility and lifecycle timing

**R7 `(planned)` — Only the exact historical AgentDecker prompt migrates.** On upgrade, AgentDeck
replaces an existing `agentdecker` system prompt only when its bytes match the one immediately
preceding shipped seed prompt. The replacement is R2's thin prompt. The write preserves title,
role id, `skip_permissions`, and every other role field. A one-byte edit, empty/custom prompt,
different role, missing role, or unreadable role is untouched; users of those roles may edit them
manually. The migration is one-time, idempotent compatibility work rather than a managed-role or
recurring synchronization mechanism.

**R8 `(planned)` — Knowledge refresh is process-bound, not a hot reload.** A verified package is
refreshed when the dashboard starts. AgentDeck does not restart a running process, inject a new
turn, mutate its transcript, or replace provider state when the package or role migration changes.
Every subsequent fresh launch, resume, or switch is composed with the currently installed package.
An already-running provider may observe new files only if it explicitly reads the stable bundled
path; AgentDeck makes no hot-reload claim.

## 3. States & transitions

- **Package:** absent or older cache → dashboard startup installs the current complete package →
  later process composition exposes it; a later dashboard startup replaces it with that binary's
  complete package.
- **Role:** absent or non-matching prompt → unchanged; exact historical prompt → R2 prompt; later
  startup → unchanged.
- **Process:** running with its existing context → no unsolicited change; next launch, resume, or
  switch → current package is discoverable.

## 4. Edge cases & errors

**R9 `(planned)` — Installation fails closed while discovery degrades to the direct path.** If the
bundled package cannot be securely installed or verified, dashboard startup fails before it can
launch a managed process. Once startup has verified the package, failure of provider-native skill
discovery does not block an otherwise valid launch because R6's direct path remains available.

**R10 `(planned)` — Matching and documentation ownership fail closed.** Prompt migration never uses
substring, whitespace-normalized, title-, age-, or role-id-only matching. A skill or reference may
name an operation only according to its governing shipped FS/TS and current tool definition;
changing prose alone cannot add or change a tool, REST/CLI operation, permission, lifecycle state,
or interface contract.

## 5. Acceptance criteria

**A1 `(planned)`** (R1, R6, R8) — Fresh launch, ordinary and wake resume, runtime switch, and
pipeline-started work expose one release-matched `operating-agentdeck` package to chat and terminal
processes. Composition tests prove the managed directory and direct pointer are added once through a
shared path.

**A2 `(planned)`** (R2–R5) — The seeded role prompt is byte-equal to R2 and contains no product
manual. `SKILL.md` contains the R4 decisions, routes exactly the four R5 references, and neither
duplicates tool schemas nor requires all references to be loaded.

**A3 `(planned)`** (R3, R6, R9–R10) — Tool names, arguments, authority, effects, and results are
unchanged when native skill discovery is absent or shadowed; the direct bundled path remains
readable, and no role/skill text grants an unavailable operation.

**A4 `(planned)`** (R5) — Package checks prove every linked reference exists, is readable directly
from the core skill, names its intended job, and does not embed the other references wholesale.

**A5 `(planned)`** (R7, R10) — The exact legacy-prompt fixture migrates once, including when its
non-prompt fields were customized; successful migration preserves those fields byte-for-byte.
Fixtures with a one-byte prompt edit, empty/custom prompt, another role, a missing role, and a
read/comparison error remain unchanged.

**A6 `(planned)`** (R8–R9) — Package installation failure prevents startup while preserving any
previous complete cache. A package refresh or role migration causes no unsolicited provider prompt,
transcript event, restart, or lifecycle transition; the next process composition exposes the
current package.

**A7 `(planned)`** (R1–R6) — With pinned Claude and Codex providers, an ordinary non-AgentDecker
role can identify the native skill and open one routed reference; a provider with native discovery
disabled can reach the same file through the direct pointer. AgentDecker answers an AgentDeck
question from the skill and creates no orchestration action until asked.

## 6. Deviations & open decisions

- No UI, REST endpoint, MCP documentation tool, agent-facing release command, mutable knowledge
  store, or new runtime interface is introduced.
- Customized AgentDecker roles remain user-owned even if they contain a stale copy of product
  knowledge. AgentDeck does not infer that they should migrate.
- The future `/release` workflow is outside this feature. The package contains only the maintenance
  decision rubric that workflow will consume.

## 7. Traceability

Planned anchors: the embedded skill and installer in `internal/agentknowledge`; exact role
compatibility in `internal/config`; shared lifecycle composition in `internal/server`; chat and
terminal delivery in `internal/runtime`; startup ordering in `internal/cli`; and current tool
contracts in `internal/messaging`. Governing requirements: FS-04.R1–R4/R13–R15; FS-06; FS-14–FS-17;
TS-11; and INV §1, §2, §4, §6, §8, §10, §11, and §15.
