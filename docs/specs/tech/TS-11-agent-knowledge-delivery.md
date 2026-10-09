# TS-11 — Agent Knowledge Packaging and Delivery

**Status:** Partial
**Code:** `internal/agentknowledge`, `internal/config`, `internal/server`, `internal/runtime`, `internal/messaging`, `internal/cli`
**Absorbed:** the Chuck knowledgebase idea in [`../../ideas.md`](../../ideas.md)

## 1. Scope

This specification owns the product-managed `operating-chuck` skill package, its release-time
source, secure cache installation, delivery to every Chuck-launched process, safe degradation
when the package is unavailable, and the exact legacy Chucky-prompt migration.

It does not create a documentation API, MCP documentation tool, mutable knowledge store, managed
role system, provider-home synchronization, or development/release-maintenance workflow. Feature
specifications and tool registrations remain authoritative; the package is release-matched
operating guidance.

## 2. Design & constraints

**R1 — One embedded source owns the complete skill package.** A new
`internal/agentknowledge` package embeds one canonical `operating-chuck` source tree:

```text
operating-chuck/
  SKILL.md
  references/operate-agents.md
  references/coordinate-work.md
  references/build-and-run-pipelines.md
```

`SKILL.md` has provider-neutral frontmatter with `name: operating-chuck` and a description that
triggers when an agent answers Chuck product questions or operates, coordinates, or supervises
Chuck work. It links references one level deep. There is no independently maintained
Claude/Codex source twin and no generated copy committed outside this package.

**R2 — Startup atomically publishes two byte-identical managed views when verification
succeeds.** On every dashboard start, Chuck attempts to install the embedded package beneath the
owner-only root `$CHUCK_HOME/cache/agent-skills/` at:

```text
.agents/skills/operating-chuck/**
.claude/skills/operating-chuck/**
```

Both views contain exactly the embedded files with byte-identical contents. Installation stages and
verifies the complete package before replacing managed files atomically; dashboard readiness is the
package-level commit boundary, so no managed launch can observe a partly installed tree. Managed
directories are `0700`, regular files are `0600`, no installed entry is a symlink, and every
resolved path stays beneath the managed cache root. The installer owns only this cache root; it
never writes a repository, personal skill directory, native provider home, or Chuck's private
Codex profile.

**R3 — retired 2026-08-29:** Startup-fatal package installation was replaced before implementation
by R10's warning-only, no-advertisement degradation contract.

**R4 — One helper composes the knowledge overlay for every process path.** A single
server-owned helper receives the startup process's verified package availability and augments the
already-composed base `LaunchSpec` for fresh launch, ordinary and wake resume, runtime switch,
pipeline launch/resume, chat, and terminal only when that availability is true. It:

- adds `$CHUCK_HOME/cache/agent-skills` once to effective `AddDirs`;
- adds reserved final-layer `CHUCK_SKILL_DIR`, pointing to the absolute managed
  `.agents/skills/operating-chuck` directory; and
- appends once: `Chuck operator knowledge is in the bundled operating-chuck skill at
  <absolute path>/SKILL.md; read it when Chuck-specific behavior matters.`

Provider-native discovery consumes the `.agents` or `.claude` view as supported. The absolute-path
instruction is the provider-neutral fallback and is authoritative if a same-named user/project
skill is also surfaced. It does not inline `SKILL.md` or any reference into the launch prompt. With
unavailable package state, the helper is a no-op for all three additions. The directory and prompt
additions use runtime-only effective fields (or the equivalent final process-parameter seam), never
the persisted `LaunchSpec.AddDirs` or `LaunchSpec.SystemPrompt` values.

**R5 — The overlay changes no frozen user configuration.** The helper runs after launch,
resume, or switch has selected its base role/project/backend configuration. It may add the stable
managed root, reserved environment entry, and prompt pointer to a pre-feature snapshot, but it never
re-resolves or rewrites that snapshot's role prompt, project prompt, user `add_dirs`, backend/model,
effort, permissions, or federation launch object. New and old sessions therefore receive current
product-managed files at the stable path when startup availability is true, while preserving their
user-owned frozen configuration. When availability is false they receive no knowledge overlay. The
helper deduplicates its three additions, including across repeated resume/switch cycles and the
one-shot switch primer. Session metadata and snapshots always persist the unmodified frozen base
fields, so a later unavailable startup cannot recover an overlay from durable state.

**R6 — superseded 2026-09-10.** The single-role, single-digest migration is generalized to a table
by R13, which keeps every guarantee below and widens only its scope: the superseded prompt text is
still not retained as a second knowledge source, parsed, normalized, or fuzzy-matched.

**R13 — One migration helper reads a code-owned digest table covering every
seeded role.** `Store.MigrateSupersededRolePrompts` is one role-agnostic entry point over
`supersededRolePromptDigests`, a code-owned table mapping each seeded role id to the digests of
prompts previously shipped for that id. The replacement text is read from `seedRoles()` rather than
restated, so the current prompt has exactly one authority and the table cannot drift from it
(INV §2, INV §10). The per-role body keeps R6's semantics: package `Available=true` gates the whole
pass; the role is read, SHA-256 is computed over its stored system-prompt bytes, and only an exact
match replaces the field through the ordinary atomic role writer. Roles are visited in sorted order,
and per-role read, decode, or write failure is joined into the returned error while the pass
continues to the next role rather than aborting (INV §7, INV §8); `prepareAgentKnowledge` keeps its
single warn-and-continue call site rather than growing one call per role, and the returned count is
the number actually corrected. Each role id maps to a list of digests so an install several releases
behind is still corrected. A table entry naming a role `seedRoles()` does not seed is reported rather
than skipped silently, and an entry whose digest equals that role's *current* seeded prompt would
silently no-op, so a test re-derives every digest from the fixture bytes in
`internal/config/testdata/superseded_*_prompt.txt` and fails on either defect (INV §10, INV §17).

**R7 — Tool definitions retain local mechanics; the skill owns cross-tool judgment.**
All existing agent-facing tool names, argument and result shapes, validation, authority, effects,
`isError`, text blocks, `structuredContent`, and retry classifications remain unchanged. The
`create_task` description no longer owns the cross-workflow instruction to avoid polling; that rule
moves to the main skill. The `report_pipeline_stage_result` definition states only the local call
semantic that an accepted result is final for the current attempt; the pipeline reference owns the
meaning of `blocked`, human Continue, and proposals. The stale messaging package comment that
advertises only three tools and superseded wake behavior is corrected. No skill file carries an
argument schema or duplicates the full registration inventory.

**R8 — Core and reference content are bounded by ownership.** The main `SKILL.md`
contains only the Chuck-wide choices in FS-18.R3–R4: message versus task versus context link
versus pipeline, durable dependencies instead of polling, pull-only/non-waking context, Chuck-
derived authority, structured-result behavior, and routing to tool definitions for exact mechanics.
`coordinate-work.md` owns coordination-only details including messaging budgets.
`build-and-run-pipelines.md` owns pipeline-only details including accepted/`blocked` attempt
finality, human Continue, and review-only Chucky proposals. `operate-agents.md` owns lifecycle,
configuration, interface, and project-resource detail. Examples are limited to commonly misused
behavior; planned, experimental, secret-bearing, credential-specific, and unverifiable claims are
excluded. As an alignment cleanup, the fresh PM and teammate seed prompts remove duplicated
coordination tool mechanics and the numeric mail budget; existing user-owned role files remain
untouched.

**R9 — retired 2026-08-29:** Release-maintenance classification was removed from the shipped
operator package before implementation. It belongs directly in `/release` or the repository
development/release skill that workflow uses.

**R10 — Installation failure is warning-only and suppresses every availability
signal.** The installer returns one immutable process-local availability result to server
construction. Secure-path, publication, or verification failure returns `Available=false`, emits a
bounded warning to the ordinary startup log/stderr sinks, and permits dashboard startup. The shared
composition helper then adds no managed `AddDirs`, `CHUCK_SKILL_DIR`, or package-use prompt for
any launch path, even if a prior cache remains on disk. Chuck neither claims nor natively
advertises the package for that dashboard process, and R13's migration does not run. The next
dashboard start retries installation and then migration; there is no background repair, network
fetch, telemetry, or hot-reload loop.

**R11 (planned) — Operating guidance chooses actions without duplicating their protocol.** The
embedded skill replaces internal-MCP/tool wording with the FS-17.R13 action names and directs agents
to `chuck action describe <action>` for exact fields and results. It retains cross-action
judgment, budgets, authority, recovery, and pipeline/task boundaries, but does not copy schemas,
credentials, URLs, or the full action catalog into resident guidance.
This replacement ships atomically with the eventual direct-action cutover; internal-MCP wording
remains correct and unchanged while FS-17.R20 is unmet.

**R12 (planned) — Basic action discovery does not depend on skill installation.** Every chat runtime
overlay carries one short stable pointer to `$CHUCK_ACTION_CLI action ...`; mail activation names
`check_messages`, task activation names `get_assigned_task`, and pipeline assignment names
`report_pipeline_stage_result`. If managed-skill installation degrades under R10, these exact next
steps remain available without claiming the broader knowledge package is installed. Terminal
launches receive neither signal (FS-17.R15/R18, TS-04.R36–R37).
No direct-action pointer is injected while the migration is paused.

**R14 — retired 2026-10-03:** Special retirement/prompt migration replaced by ordinary publication R18.
**R18** — Rename `SkillName`, embedded directory and frontmatter to `operating-chuck`.
Reuse `internal/agentknowledge/package.go`'s existing verified whole-root publication (R9/R10),
which stages both provider views and replaces the managed root; do not add an old-name detector,
cleanup retry loop or per-directory migration. Files outside that owned root remain untouched.
FS-18.R18's authored wording changes together with the seed constants; the existing prompt-refresh
mechanism remains but receives no rename-specific legacy-role conversion or digest table extension.
Prepared customized roles belong to the offline cutover (TS-02.R41). Verify the current package
using the existing publication tests (INV §2, §7, §15).

**R15 — Shared standing context extends the existing runtime-only overlay.** For
FS-18.R15, `server.applyKnowledgeOverlay` remains the sole composition seam for fresh launch,
ordinary/wake resume, runtime switch and pipeline launch/resume in chat and terminal. One
code-owned, provider-neutral instruction block beside the existing knowledge pointer supplies the
stable environment, capability-awareness, assignment, no-polling and authority guidance. It names
capabilities conditionally on their actual availability, contains no secret, runtime credential,
tool schema or numeric budget, and does not grant an operation. Role constants do not duplicate
this block. Keep it to a short paragraph or a few bullets; no new configuration field, SQLite
column, instruction-template registry or provider-specific copy is introduced.

The helper appends that block once through `RuntimeSystemPromptSuffix` before testing package
availability, and appends the verified direct-path skill-use instruction once only when available.
It preserves any other runtime suffix and composes with `RuntimeSystemPrompt` switch primers.
R4/R5's conditional directory/env/pointer rules and R10's installation degradation remain in
force; their former absence of *all* product guidance when unavailable is superseded only by this
stable block. The helper still strips an inherited reserved skill env value when the package is
unavailable. Frozen `SystemPrompt`, `AddDirs`, role/project configuration, permissions and session
metadata never acquire either managed instruction block (INV §1–§3).

Each new process is composed from its frozen base plus the current dashboard's immutable package
availability; it does not reuse a previous process's augmented suffix or skill directories. Repeat
composition within the same process generation is idempotent. A later dashboard start with failed
installation therefore exposes no stale skill path/env/directory, while retaining the stable
standing context. No turn, restart, background repair or provider-state rewrite is added. Existing
native skill discovery and the reference package stay the detailed operating source.

**R16 — Four seed definitions reuse exact-prompt migration without deleting legacy roles.**
For FS-04.R50–R51 and FS-18.R16, `config.seedRoles` and `SeedIfAbsent` expose only
the resident operator, implementer, reviewer and researcher. Keep their prompt source in the
existing seed constants; the implementation uses the FS-18.R16 content contracts rather than a
second copy of the prompt in fixtures or provider configuration. The default role, Role JSON
shape and permission inheritance are unchanged. Seed-count assertions enumerate the four-role
contract independently; user-authored role CRUD remains unrestricted.

Remove `pm`/`teammate` from the seed map and remove retired-role entries from the active
`supersededRolePromptDigests` map. Preserve their on-disk files, references and historical identity;
there is no delete/remap migration or compatibility alias. Retain the invariant that a migration
entry for a non-seeded role is an error, rather than weakening it to tolerate a stale table. Add
the immediately preceding shipped prompt bytes for each retained role to the historical fixtures
and digest lists, keeping prior supported digests. Never add the new prompt's own digest. Retired
fixtures may remain as preservation-test evidence but never activate a migration. Reuse R13's
single package-gated migration pass and its per-role failure isolation. An empty or one-byte-edited
custom prompt stays unchanged; existing PM/Teammate defaults and task/pipeline references continue
to resolve through ordinary role reads. Frozen session personas are not refreshed on resume.

The separately approved resident-role/product rename owns spelling and identity migration. If it
lands first, use its canonical resident-role id and retain its preservation rules; this change
neither reinstates the old id nor adds a second operator. Update active seed inventory and prompt
documentation at implementation closure; historical archives remain historical.

**R17 — Prompt checks distinguish delivery, migration and model behavior.** `(planned)`
FS-04.A30–A31 and FS-18.A11–A13 are proved in three layers:

- Extend existing config, CLI knowledge and server-overlay fixtures for four-role fresh seeding,
  retained legacy roles/references, exact retained-role migration, custom/empty prompts, independent
  I/O/decode failures, unavailable→available package starts and idempotence. Replace tests that
  require a freshly seeded Teammate with retained-role migration cases plus explicit legacy-file
  preservation; do not remove their error or identity coverage. Exercise the existing lifecycle
  composer matrix with shared guidance and conditional pointer, a switch primer and frozen
  metadata, including an old snapshot lacking the new managed text.
- Prove native-provider prompt preservation through TS-04.R69's pinned adapter/SDK boundary and
  both new/load payloads. Existing terminal argv and Codex overlay tests retain their scope.
  A fake ACP echo alone cannot establish provider behavior (TS-04.R54, INV §17).
- Run the six bounded manual scenarios in FS-18.A12 against the pinned Claude and Codex providers
  with task and applicable skill held explicit, then a follow-up that omits role reminders. Record
  versions, inputs, observed role boundaries, evidence and failures; this is a qualitative acceptance
  receipt, not an accuracy benchmark. Fresh/resume prompt-adoption checks follow FS-18.A13.
  Missing credentials or unavailable upstream services leave a named manual gate open, never a
  fabricated pass. Automated content guards check prohibited polling/authority claims and source
  duplication, not arbitrary word counts or complete prompt snapshots.

Implementation closure runs the applicable TS-06.R5 matrix once after the final relevant change;
this design-only update runs spec lint, twin-skill comparison and diff checks. No rendered redesign,
new evaluation service or model-selection policy is part of the change.

**R19** — Extend the embedded `operating-chuck` package with one progressively linked
`references/think-tanks.md` for FS-18.R19. Update the verified file inventory, install/overlay
fixtures and producer-derived tool checks together. Describe canonical room versus normal session,
bounded reads, explicit staged publication, ceilings, departure, intervention and end-only judge;
tool definitions own exact arguments/results. No persona, repository instruction or provider-global
configuration duplication. The fixed activation instruction points at current room tools; mutable
room goal, phase and turn data are fetched rather than frozen into launch configuration.

**R20 (shipped 2026-10-09) — Pipeline guidance belongs to the frozen agent prompt.** For FS-14.R87–R89,
the server's ordinary base launch composer accepts only the bounded per-run instructions resolved
under TS-09.R58 and appends the labelled block under TS-09.R59 once for a newly created standing
owner or dedicated coordinator. Unlike the current product-managed knowledge overlay, this is
user-authored, constant configuration for that agent and persists in `LaunchSpec.SystemPrompt`.
`applyKnowledgeOverlay` continues to own only product operating context, package discovery and tool
approval composition; it neither rereads live templates nor moves pipeline guidance into its
runtime-only suffix. Resume/wake/switch and native-session fallback preserve the ordinary frozen
base, while switch primers remain process-only. Existing Claude native-preset append and Codex
developer-instruction delivery carry the resulting composed prompt unchanged. No new provider
capability, regular-message bootstrap, role-file migration or provider-global config write is used.
Refresh the pipeline authoring reference for the optional field when shipping; FS-14.A54–A55
exercise actual provider parameters and frozen recovery independently of the composition helper.

## 3. Interfaces & data shapes

The new agent-facing delivery contracts are:

```text
skill name: operating-chuck
managed root: $CHUCK_HOME/cache/agent-skills
direct package: $CHUCK_HOME/cache/agent-skills/.agents/skills/operating-chuck
reserved env: CHUCK_SKILL_DIR=<direct package>
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
- **INV §3:** R15's shared context and skill pointer stay in process-only fields rather than
  frozen persona/project configuration or session metadata.
- **INV §7:** R16 retains per-role migration failures and never deletes an existing legacy role.
- **INV §4:** a process teardown removes only generation-scoped runtime artifacts and never deletes
  the installed cache or user configuration.
- **INV §6:** chat, terminal, manual, wake, switch, and pipeline paths join the same conditional
  delivery contract before availability is claimed.
- **INV §8:** installation and migration failures are bounded and actionable; an unavailable or
  unverified skill is never advertised as present.
- **INV §10:** embedded source is authoritative, installed trees are disposable projections, and
  byte-for-byte verification prevents source and provider-view drift.
- **INV §15:** the complete verified package is committed before any launch can consume it; a
  failed commit suppresses the overlay rather than suppressing Chuck startup.
- **INV §17:** R13's digest table is proven against the shipped seed constants rather than against a
  restated copy of them, so a stale or self-matching entry fails a test instead of silently
  migrating nothing.

## 5. Deviations & open decisions

- No external API or schema changes. `CHUCK_SKILL_DIR`, the managed path, and the bounded prompt
  pointer are the only new agent-facing delivery interfaces.
- R15–R17 extend the existing prompt overlay and seed contents only; Claude's additive wire shape
  is owned by TS-04.R69. The direct-action migration remains paused and is not a prerequisite.
- No migration touches a customized role, and no managed/read-only role architecture is introduced.
- No runtime copies the skill into provider homes or repositories. Native discovery is a convenience;
  the direct installed path is the compatibility fallback.
- Development and release-maintenance classification is excluded from this product package and
  belongs to `/release` or its repository development skill.

## 6. Traceability

Anchors: embedded assets and verified publication in `internal/agentknowledge`; exact digest
migration in `internal/config/seed.go`; `server.applyKnowledgeOverlay`; the runtime-only effective
launch fields consumed by ACP and terminal runtimes; local tool definitions in
`internal/messaging/messaging.go`; and `cli.prepareAgentKnowledge`. R13's table and per-role pass
replace the single-digest path at `internal/config/seed.go` and keep its one call site in
`internal/cli/dashboard.go`. Governing seams: FS-18;
FS-04.R13–R15/R47; TS-01.R5–R6/R9;
TS-02.R3–R5; TS-04.R6–R7/R14/R17/R28–R31; TS-06.R3–R7/R11; TS-09; TS-10; FS-17; and INV §1, §2,
§4, §6, §8, §10, §15, and §17.
