# TS-11 — Agent Knowledge Packaging and Delivery

**Status:** Partial
**Code:** `internal/agentknowledge`, `internal/config`, `internal/server`, `internal/runtime`, `internal/messaging`, `internal/cli`
**Absorbed:** the AgentDeck knowledgebase idea in [`../../ideas.md`](../../ideas.md)

## 1. Scope

This specification owns the product-managed `operating-agentdeck` skill package, its release-time
source, secure cache installation, delivery to every AgentDeck-launched process, and the exact
legacy AgentDecker-prompt migration. It also defines how future release review classifies changes to
agent-facing knowledge.

It does not create a documentation API, MCP documentation tool, mutable knowledge store, managed
role system, provider-home synchronization, or `/release` workflow. Feature specifications and
tool registrations remain authoritative; the package is release-matched operating guidance.

## 2. Design & constraints

**R1 `(planned)` — One embedded source owns the complete skill package.** A new
`internal/agentknowledge` package embeds one canonical `operating-agentdeck` source tree:

```text
operating-agentdeck/
  SKILL.md
  references/operate-agents.md
  references/coordinate-work.md
  references/build-and-run-pipelines.md
  references/maintain-agent-knowledge.md
```

`SKILL.md` has provider-neutral frontmatter with `name: operating-agentdeck` and a description that
triggers when an agent answers AgentDeck product questions or operates, coordinates, supervises, or
maintains AgentDeck work. It links references one level deep. There is no independently maintained
Claude/Codex source twin and no generated copy committed outside this package.

**R2 `(planned)` — Startup publishes two byte-identical managed views.** On every dashboard start,
the embedded package is atomically installed beneath the owner-only root
`$AGENTDECK_HOME/cache/agent-skills/` at:

```text
.agents/skills/operating-agentdeck/**
.claude/skills/operating-agentdeck/**
```

Both views contain exactly the embedded files with byte-identical contents. Installation stages and
verifies the complete package before replacing managed files atomically; dashboard readiness is the
package-level commit boundary, so no managed launch can observe a partly installed tree. Managed
directories are `0700`, regular files are `0600`, no installed entry is a symlink, and every
resolved path stays beneath the managed cache root. The installer owns only this cache root; it
never writes a repository, personal skill directory, native provider home, or AgentDeck's private
Codex profile.

**R3 `(planned)` — A package-install failure prevents dashboard startup.** Secure path preparation,
publication, and verification of both views complete before the dashboard accepts launches. Any
failure preserves a prior complete view where possible, returns a bounded actionable local error,
and aborts startup. There is no background repair, network fetch, telemetry, or hot-reload loop.

**R4 `(planned)` — One helper composes the knowledge overlay for every process path.** A single
server-owned helper augments the already-composed base `LaunchSpec` for fresh launch, ordinary and
wake resume, runtime switch, pipeline launch/resume, chat, and terminal. It:

- adds `$AGENTDECK_HOME/cache/agent-skills` once to effective `AddDirs`;
- adds reserved final-layer `AGENTDECK_SKILL_DIR`, pointing to the absolute managed
  `.agents/skills/operating-agentdeck` directory; and
- appends once: `AgentDeck operator knowledge is in the bundled operating-agentdeck skill at
  <absolute path>/SKILL.md; read it when AgentDeck-specific behavior matters.`

Provider-native discovery consumes the `.agents` or `.claude` view as supported. The absolute-path
instruction is the provider-neutral fallback and is authoritative if a same-named user/project
skill is also surfaced. It does not inline `SKILL.md` or any reference into the launch prompt.

**R5 `(planned)` — The overlay changes no frozen user configuration.** The helper runs after launch,
resume, or switch has selected its base role/project/backend configuration. It may add the stable
managed root, reserved environment entry, and prompt pointer to a pre-feature snapshot, but it never
re-resolves or rewrites that snapshot's role prompt, project prompt, user `add_dirs`, backend/model,
effort, permissions, or federation launch object. New and old sessions therefore receive current
product-managed files at the stable path while preserving their user-owned frozen configuration.
The helper deduplicates its three additions, including across repeated resume/switch cycles and the
one-shot switch primer.

**R6 `(planned)` — Legacy role migration uses one exact code-owned digest.** After ordinary
`SeedIfAbsent` handling, startup reads the `agentdecker` role when present and computes SHA-256 over
the stored system-prompt bytes. Only equality with the single digest of the immediately preceding
shipped seed prompt authorizes replacing that field with FS-18.R2's exact thin prompt. The ordinary
atomic role writer preserves every other field. Read, decode, comparison, or write failure leaves
the role unchanged, emits a bounded warning, and does not block startup; this one-time convenience
never broadens normal configuration readiness. The legacy prompt text is not retained as a second
knowledge source, parsed, normalized, or fuzzy-matched.

**R7 `(planned)` — Tool definitions retain local mechanics; the skill owns cross-tool judgment.**
All existing agent-facing tool names, argument and result shapes, validation, authority, effects,
`isError`, text blocks, `structuredContent`, and retry classifications remain unchanged. The
`create_task` description no longer owns the cross-workflow instruction to avoid polling; that rule
moves to the main skill. The `report_pipeline_stage_result` definition explicitly states the local
call semantic that an accepted result is final for the current attempt, including `blocked`, and
that further work requires human Continue and a new attempt. The stale messaging package comment
that advertises only three tools and superseded wake behavior is corrected. No skill file carries an
argument schema or duplicates the full registration inventory.

**R8 `(planned)` — Core and reference content are bounded by ownership.** The main `SKILL.md`
contains only FS-18.R3–R5's broadly applicable AgentDeck mental model, defaults, decision rules, and
non-obvious failure modes. It tells the agent when to load each reference and to trust current tool
definitions for individual calls. Each reference covers only its named job, may summarize shipped
behavior from its governing FS/TS, and contains examples only for commonly misused behavior. Planned,
experimental, secret-bearing, credential-specific, and unverifiable claims are excluded.

**R9 `(planned)` — Release maintenance uses a four-way classification.**
`references/maintain-agent-knowledge.md` tells a future release workflow to compare FS requirement
changes from the previous release tag to the candidate release and classify each changed behavior:

- **Tool contract:** required to invoke one tool correctly; update the tool definition and contract
  tests, plus the skill only if cross-tool judgment also changed.
- **Core skill:** a broad, non-obvious mental model, default, cross-capability decision, or failure
  mode; update `SKILL.md`.
- **Reference only:** useful only for one job or workflow; update the owning reference.
- **No agent-facing documentation:** internal implementation, tests, or refactoring changed while
  observable behavior and existing knowledge remain accurate.

One release may contain several classifications, but each documented claim has one owning layer.
This requirement creates no command, tag-selection code, CI step, publication flow, or `/release`
workflow.

## 3. Interfaces & data shapes

The new agent-facing delivery contracts are:

```text
skill name: operating-agentdeck
managed root: $AGENTDECK_HOME/cache/agent-skills
direct package: $AGENTDECK_HOME/cache/agent-skills/.agents/skills/operating-agentdeck
reserved env: AGENTDECK_SKILL_DIR=<direct package>
```

The embedded package inventory in R1 is closed for this version. Unknown installed entries,
missing required files, non-regular files, symlinks, path escapes, or any difference between the
embedded and projected bytes fail R3 before the new view is published. These paths and environment
values are derived execution data, not JSON configuration, SQLite state, REST/SSE data, or MCP
arguments.

## 4. Invariants

- **INV §1:** resume and switch republish the current product-managed overlay while preserving the
  frozen user-selected side of the boundary.
- **INV §2:** R4 is the single launch/resume/switch composition helper; R7 leaves every local tool
  contract in its registration rather than rebuilding either surface elsewhere.
- **INV §4:** a process teardown removes only generation-scoped runtime artifacts and never deletes
  the installed cache or user configuration.
- **INV §6:** chat, terminal, manual, wake, switch, and pipeline paths join the same delivery and
  verification contracts before availability is claimed.
- **INV §8:** installation and migration fail with bounded actionable errors and never claim a skill
  or role update that did not commit.
- **INV §10/§11:** embedded source is authoritative, installed trees are disposable projections,
  and byte-for-byte verification prevents source and provider-view drift.
- **INV §15:** the complete verified package is committed before any launch can consume it.

## 5. Deviations & open decisions

- No external API or schema changes. `AGENTDECK_SKILL_DIR`, the managed path, and the bounded prompt
  pointer are the only new agent-facing delivery interfaces.
- No migration touches a customized role, and no managed/read-only role architecture is introduced.
- No runtime copies the skill into provider homes or repositories. Native discovery is a convenience;
  the direct installed path is the compatibility fallback.
- The future `/release` workflow remains separate work; only its classification input is prepared.

## 6. Traceability

Planned anchors: embedded assets, verification, and installation in `internal/agentknowledge`;
prompt migration in `internal/config`; the shared overlay helper in `internal/server`; ACP and
terminal consumption in `internal/runtime`; local tool definitions in `internal/messaging`; and
startup ordering in `internal/cli`. Governing seams: FS-18; FS-04.R13–R15; TS-01.R5–R6/R9;
TS-02.R3–R5; TS-04.R6–R7/R14/R17/R28–R31; TS-06.R3–R7/R11; TS-09; TS-10; FS-17; and INV §1, §2,
§4, §6, §8, §10, §11, and §15.
