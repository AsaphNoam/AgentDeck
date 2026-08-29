# TS-11 — Agent Knowledge Packaging and Delivery

**Status:** Partial
**Code:** `internal/agentknowledge`, `internal/config`, `internal/server`, `internal/runtime`, `internal/messaging`, `internal/cli`
**Absorbed:** the AgentDeck knowledgebase idea in [`../../ideas.md`](../../ideas.md)

## 1. Scope

This specification owns the product-managed `operating-agentdeck` skill package, its release-time
source, secure cache installation, delivery to every AgentDeck-launched process, safe degradation
when the package is unavailable, and the exact legacy AgentDecker-prompt migration.

It does not create a documentation API, MCP documentation tool, mutable knowledge store, managed
role system, provider-home synchronization, or development/release-maintenance workflow. Feature
specifications and tool registrations remain authoritative; the package is release-matched
operating guidance.

## 2. Design & constraints

**R1 `(planned)` — One embedded source owns the complete skill package.** A new
`internal/agentknowledge` package embeds one canonical `operating-agentdeck` source tree:

```text
operating-agentdeck/
  SKILL.md
  references/operate-agents.md
  references/coordinate-work.md
  references/build-and-run-pipelines.md
```

`SKILL.md` has provider-neutral frontmatter with `name: operating-agentdeck` and a description that
triggers when an agent answers AgentDeck product questions or operates, coordinates, or supervises
AgentDeck work. It links references one level deep. There is no independently maintained
Claude/Codex source twin and no generated copy committed outside this package.

**R2 `(planned)` — Startup atomically publishes two byte-identical managed views when verification
succeeds.** On every dashboard start, AgentDeck attempts to install the embedded package beneath the
owner-only root `$AGENTDECK_HOME/cache/agent-skills/` at:

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

**R3 — retired 2026-08-29:** Startup-fatal package installation was replaced before implementation
by R10's warning-only, no-advertisement degradation contract.

**R4 `(planned)` — One helper composes the knowledge overlay for every process path.** A single
server-owned helper receives the startup process's verified package availability and augments the
already-composed base `LaunchSpec` for fresh launch, ordinary and wake resume, runtime switch,
pipeline launch/resume, chat, and terminal only when that availability is true. It:

- adds `$AGENTDECK_HOME/cache/agent-skills` once to effective `AddDirs`;
- adds reserved final-layer `AGENTDECK_SKILL_DIR`, pointing to the absolute managed
  `.agents/skills/operating-agentdeck` directory; and
- appends once: `AgentDeck operator knowledge is in the bundled operating-agentdeck skill at
  <absolute path>/SKILL.md; read it when AgentDeck-specific behavior matters.`

Provider-native discovery consumes the `.agents` or `.claude` view as supported. The absolute-path
instruction is the provider-neutral fallback and is authoritative if a same-named user/project
skill is also surfaced. It does not inline `SKILL.md` or any reference into the launch prompt. With
unavailable package state, the helper is a no-op for all three additions. The directory and prompt
additions use runtime-only effective fields (or the equivalent final process-parameter seam), never
the persisted `LaunchSpec.AddDirs` or `LaunchSpec.SystemPrompt` values.

**R5 `(planned)` — The overlay changes no frozen user configuration.** The helper runs after launch,
resume, or switch has selected its base role/project/backend configuration. It may add the stable
managed root, reserved environment entry, and prompt pointer to a pre-feature snapshot, but it never
re-resolves or rewrites that snapshot's role prompt, project prompt, user `add_dirs`, backend/model,
effort, permissions, or federation launch object. New and old sessions therefore receive current
product-managed files at the stable path when startup availability is true, while preserving their
user-owned frozen configuration. When availability is false they receive no knowledge overlay. The
helper deduplicates its three additions, including across repeated resume/switch cycles and the
one-shot switch primer. Session metadata and snapshots always persist the unmodified frozen base
fields, so a later unavailable startup cannot recover an overlay from durable state.

**R6 `(planned)` — Legacy role migration uses one exact code-owned digest after package
verification.** After ordinary `SeedIfAbsent` handling, startup attempts and verifies package
installation first. Only `Available=true` permits reading the `agentdecker` role, computing SHA-256
over its stored system-prompt bytes, and replacing that field when it equals the single digest of
the immediately preceding shipped seed prompt. The ordinary atomic role writer preserves every
other field. Unavailable package state leaves the role untouched; a later start retries. Read,
decode, or write failure likewise leaves the role unchanged, emits a bounded warning, and does not
block startup. This one-time convenience never broadens normal configuration readiness. The legacy
prompt text is not retained as a second knowledge source, parsed, normalized, or fuzzy-matched.

**R7 `(planned)` — Tool definitions retain local mechanics; the skill owns cross-tool judgment.**
All existing agent-facing tool names, argument and result shapes, validation, authority, effects,
`isError`, text blocks, `structuredContent`, and retry classifications remain unchanged. The
`create_task` description no longer owns the cross-workflow instruction to avoid polling; that rule
moves to the main skill. The `report_pipeline_stage_result` definition states only the local call
semantic that an accepted result is final for the current attempt; the pipeline reference owns the
meaning of `blocked`, human Continue, and proposals. The stale messaging package comment that
advertises only three tools and superseded wake behavior is corrected. No skill file carries an
argument schema or duplicates the full registration inventory.

**R8 `(planned)` — Core and reference content are bounded by ownership.** The main `SKILL.md`
contains only the AgentDeck-wide choices in FS-18.R3–R4: message versus task versus context link
versus pipeline, durable dependencies instead of polling, pull-only/non-waking context, AgentDeck-
derived authority, structured-result behavior, and routing to tool definitions for exact mechanics.
`coordinate-work.md` owns coordination-only details including messaging budgets.
`build-and-run-pipelines.md` owns pipeline-only details including accepted/`blocked` attempt
finality, human Continue, and review-only AgentDecker proposals. `operate-agents.md` owns lifecycle,
configuration, interface, and project-resource detail. Examples are limited to commonly misused
behavior; planned, experimental, secret-bearing, credential-specific, and unverifiable claims are
excluded. As an alignment cleanup, the fresh PM and teammate seed prompts remove duplicated
coordination tool mechanics and the numeric mail budget; existing user-owned role files remain
untouched.

**R9 — retired 2026-08-29:** Release-maintenance classification was removed from the shipped
operator package before implementation. It belongs directly in `/release` or the repository
development/release skill that workflow uses.

**R10 `(planned)` — Installation failure is warning-only and suppresses every availability
signal.** The installer returns one immutable process-local availability result to server
construction. Secure-path, publication, or verification failure returns `Available=false`, emits a
bounded warning to the ordinary startup log/stderr sinks, and permits dashboard startup. The shared
composition helper then adds no managed `AddDirs`, `AGENTDECK_SKILL_DIR`, or package-use prompt for
any launch path, even if a prior cache remains on disk. AgentDeck neither claims nor natively
advertises the package for that dashboard process, and R6's migration does not run. The next
dashboard start retries installation and then migration; there is no background repair, network
fetch, telemetry, or hot-reload loop.

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
embedded and projected bytes make installation unavailable under R10 before the new view is
advertised. These paths and environment values are derived execution data, not JSON configuration,
SQLite state, REST/SSE data, or MCP arguments.

## 4. Invariants

- **INV §1:** resume and switch republish the current product-managed overlay while preserving the
  frozen user-selected side of the boundary.
- **INV §2:** R4 is the single launch/resume/switch composition helper; R7 leaves every local tool
  contract in its registration rather than rebuilding either surface elsewhere.
- **INV §4:** a process teardown removes only generation-scoped runtime artifacts and never deletes
  the installed cache or user configuration.
- **INV §6:** chat, terminal, manual, wake, switch, and pipeline paths join the same conditional
  delivery contract before availability is claimed.
- **INV §8:** installation and migration failures are bounded and actionable; an unavailable or
  unverified skill is never advertised as present.
- **INV §10:** embedded source is authoritative, installed trees are disposable projections, and
  byte-for-byte verification prevents source and provider-view drift.
- **INV §15:** the complete verified package is committed before any launch can consume it; a
  failed commit suppresses the overlay rather than suppressing AgentDeck startup.

## 5. Deviations & open decisions

- No external API or schema changes. `AGENTDECK_SKILL_DIR`, the managed path, and the bounded prompt
  pointer are the only new agent-facing delivery interfaces.
- No migration touches a customized role, and no managed/read-only role architecture is introduced.
- No runtime copies the skill into provider homes or repositories. Native discovery is a convenience;
  the direct installed path is the compatibility fallback.
- Development and release-maintenance classification is excluded from this product package and
  belongs to `/release` or its repository development skill.

## 6. Traceability

Planned anchors: embedded assets, verification, and installation in `internal/agentknowledge`;
prompt migration in `internal/config`; the shared overlay helper in `internal/server`; ACP and
terminal consumption in `internal/runtime`; local tool definitions in `internal/messaging`; and
startup ordering in `internal/cli`. Governing seams: FS-18; FS-04.R13–R15; TS-01.R5–R6/R9;
TS-02.R3–R5; TS-04.R6–R7/R14/R17/R28–R31; TS-06.R3–R7/R11; TS-09; TS-10; FS-17; and INV §1, §2,
§4, §6, §8, §10, and §15.
