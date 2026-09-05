# TS-02 — Data & persistence

**Status:** Current
**Code:** `internal/config`, `internal/state`, `internal/transcript`, `internal/index`, `internal/archive`, `internal/configsource`, `internal/contextref`
**Absorbed:** exact source mapping in the [phase archive manifest](../../archive/phases/README.md)

## 1. Scope

This spec owns durable data boundaries, file formats, SQLite ownership and migrations, transcript
storage, context-reference/grant state, and rebuildable indexes. Product-visible archive behavior
is in FS-05; federation-specific source bindings and cache rules are in TS-07. TS-11 owns the
planned disposable agent-knowledge cache and explicitly adds no JSON or SQLite authority.

## 2. Design & constraints

**R1 — Persistence is split by writer.** Human-editable configuration is JSON under
`$AGENTDECK_HOME` (default `~/.agentdeck`); machine state is SQLite; AgentDeck's chat runtime appends
normalized events to `sessions/{agent_id}/transcript.ndjson`. External CLI transcripts may also be
read and indexed, but are never treated as AgentDeck's only transcript authority.

**R2 — The server is the sole SQLite writer.** All writes flow through `internal/state`; other
packages call its methods and do not open `state.db` independently. Readers tolerate missing rows
and return typed errors rather than fabricating state.

**R3 — New and rewritten data is atomic and owner-only.** JSON updates use write-temp,
fsync/close, rename semantics in the config store. AgentDeck creates its home and data directories
as `0700` and creates/rewrites config, transcript, token, cache, and database files as `0600`.
Startup explicitly tightens the home directory and database; it does not recursively repair every
pre-existing descendant.

**R4 — Config file schemas are versioned.** `config.json` and `layout.json` are version 1;
`backends.json` is version 2; `config-sources.json` is version 1. Unknown future versions fail with
an actionable error. Seed operations create missing files only and never overwrite user content.

**R5 — Slug-addressed files are syntax-validated.** Role and project ids are validated before path
construction, including URL-decoded values, so dots and separators cannot construct an out-of-root
path. Existing role/project symlink files are followed by ordinary file reads; closing that
same-user symlink boundary requires a security delta and adversarial tests.

**R6 — SQLite changes are forward-only migrations.** Migrations run in order in a transaction,
are recorded in `schema_migrations`, and are idempotent on an already-current database. Code may
read older rows only through migration-compatible defaults; it must not rewrite migration history.

**R7 — Stable identity and runtime state are separate.** The `agents` identity row is durable and
keyed by `agent_id`. The `running` row is ephemeral current-process state keyed by the same id and
contains the live session/process/interface data. Session snapshots freeze launch-time composition
needed for resume and switch.

**R8 — Transcript append is ordered and durable.** Every normalized event has a per-agent monotonic
sequence and is appended as one JSON object per line. Readers skip a malformed or oversized record
when safe, report scanner/I/O errors, and always return arrays rather than JSON `null`.

**R9 — Search data is derived and repairable.** `sessions_fts`, tracked-file, tracked-command, and
rollup projections can be rebuilt from durable identity/session/transcript inputs. Reindex preserves
the final partial turn and replaces a session's indexed content atomically enough that readers never
observe a knowingly mixed generation.

**R10 — Both FTS capabilities are supported.** Release binaries compile with `sqlite_fts5`; the
untagged test/build path must degrade to metadata `LIKE` search without surfacing a missing-module
error. When that build opens an existing FTS5 virtual table it cannot load, index write boundaries
skip only the derived FTS metadata/transcript document while still committing authoritative session
metadata, event counters, and turn rollups. A missing FTS5 module never blocks agent lifecycle.
One state-owned capability detector classifies the current `sessions_fts` object as full-text,
plain fallback, or unavailable so index writes and Archive presentation cannot disagree. Behavior
differences are explicitly specified by FS-01.R29 and FS-05.

**R11 — Message persistence is transactional.** Messages, read state, expiry, and per-turn budget
state live in SQLite. A send either stores the message and updates its budget atomically or stores
nothing; readers return newest-first bounded results as specified by FS-06.

**R13** — Project resources are opaque filesystem data at
`$AGENTDECK_HOME/project-resources/{project-id}/`, not JSON configuration, SQLite state, a cache,
or an index. `internal/config` owns one shared helper that validates the project id, returns the
absolute path, and ensures the parent and leaf directories exist as owner-only directories. It
rejects a parent or leaf that is a non-directory or symlink. To prove an existing leaf is writable,
it creates and removes one private zero-byte probe; otherwise it never lists, reads, writes, deletes,
or repairs resource contents. Project creation calls this helper after validation and before writing
the project JSON, so a resource-creation failure leaves no new definition. New launch, resume, and
switch call the same helper before registration or process start; the immutable project id lets their
existing frozen `add_dirs` and system-prompt snapshot carry the identical path without a database
migration.

**R14 — Annotation events are ordinary transcript records.** A new normalized event kind
`annotation` (payload: per-annotation anchor `seq`, diff path/side/line range when present, clipped
excerpt, instruction; optional overall instruction; target descriptor) appends through the existing
per-agent transcript writer and sequence allocation, for active and inactive sessions alike. Its
instruction and excerpt text joins the session's indexed content (R9) so FS-13.R10 search works;
payload fields are append-only once shipped.

**R15 — User-originated mail rows.** The messages table accepts rows whose sender is the
reserved user identity (FS-06.R21), written only through `internal/state` by the server-side
annotation delivery path. The insert updates unread/indicator state in the same transaction but
touches no `turn_budget` row. FS-06.R8 retention applies unchanged; any needed column arrives as a
forward-only migration (R6) and must not repurpose existing sender fields.

**R16 — Turn-document search projection.** `sessions_fts` stores multiple derived
documents per agent, distinguished by an unindexed stable document id: one replaceable metadata
document plus immutable completed-turn, explicit non-turn-flush, and migrated legacy documents.
Only the current turn is accumulated in memory; a flush inserts a new document and never reads or
rewrites earlier transcript documents. Live indexing and reindex use the same searchable-event and
document-flush helpers. Raw `transcript.ndjson` remains authoritative; the untagged build keeps its
metadata-only fallback and does not depend on FTS transcript documents.

**R17 — Pipeline configuration and state use the existing authority split.** Version-1
templates live in owner-only, atomically rewritten `pipelines/{id}.json`; durable run snapshots,
attempt lineage/reports, current named values/provenance, and start idempotency records live in
forward-only SQLite tables written only through `internal/state`. Pipeline-table foreign keys may
cascade within a deleted run but must not cascade into `agents`, `sessions`, transcripts, or archive
projections. The migration uses non-null JSON defaults/collection decoding, indexes for active-run
and agent-attempt lookup, and a schema-version guard test. Durable AgentDecker proposal records are
part of the same SQLite-owned set and are specified by R22. TS-09 owns the logical shapes.

**R18 — Effort is an additive catalog field and frozen execution data.**
`backends.json` stays **version 2**: `efforts` and `default_effort` are optional per-model keys and
the decoder ignores unknown keys, so a catalog written by a newer build still loads in an older one,
which simply resolves no effort — FS-09.R41's documented fallback rather than a corrupt read. No
migration touches the file, and seeding continues never to rewrite an existing catalog.
The resolved effort is frozen in a new forward-only `sessions.effort TEXT NOT NULL DEFAULT ''`
column beside the existing `model`, so resume and switch read it exactly as they read the model and
an unbound backend needs no federation object. Empty means "none resolved"; existing rows adopt that
value without interpretation. The federation `launch_config_json` object (v8) keeps its own
requested-versus-resolved effort record for provenance and is not the authority for what ran.
Every pipeline attempt likewise stores `effort TEXT NOT NULL DEFAULT ''` beside its backend/model;
continuation and recovery execute that attempt's frozen identity rather than re-reading a run
assignment. Empty retains the same "none resolved" meaning for existing attempts.

**R19 — Codex's isolated runtime profile is private, managed filesystem state.**
`$AGENTDECK_HOME/codex/` is an owner-only Codex profile for `codex-acp` children (TS-04.R20/R21).
It contains the child's own session/history store plus a one-way managed mirror of personal Codex
setup. A private owner-only manifest records only the destination paths AgentDeck refreshed, so a
later source removal removes that private copy without deleting session/history data or any
unmanaged Codex state. An explicit setup allowlist, rather than a runtime-state denylist, excludes
personal session indexes, databases and WAL sidecars, logs, snapshots, temp files, and future
unrecognized runtime entries. The personal `CODEX_HOME` is never a writable destination and no
source symlink is created. The setup mirror is refreshed before each child start, not watched
continuously; a failed refresh retains the previous published profile and manifest.

**R20.** Archive state is durable at its owning boundary. A project JSON gains
`archived: false` by default; an absent key in an existing hand-edited file decodes as false. A
forward-only SQLite migration adds `agents.archived INTEGER NOT NULL DEFAULT 0` plus the project/
archive lookup index; an agent's archive flag is authoritative for its inclusion in Archive and is
not copied into its immutable session snapshot or derived search projection. The server alone changes
that flag through `internal/state`, and an archived flag is never committed while a `running` row
exists. Under TS-01.R13's exclusive project claim, project archive stops relevant pipeline runs and
processes, writes all agent archive flags in one SQLite transaction, then atomically writes the
project JSON; no process-start path can register a running row anywhere in that stop-to-commit window.
A failed final config write compensates the flag transaction before releasing the claim and returning
an error; processes already stopped remain stopped. If that atomic compensation also fails, the
project remains active, the server re-reads and publishes the resulting durable agent
flags before releasing the claim, and the error reports both the project-publication and compensation
failures; any agents that remain archived are coherent independent agent archives rather than hidden
partial state. Restore reverses only the requested project or agent flag/config state. Existing
`layout.json` and a `default_project` that names the project are unchanged: the default becomes a
dormant preference under FS-04.R36, while project cards and scoped dashboards add no new persisted
layout or migration.

**R21 — Appearance is an additive version-1 config preference.**
`config.json` gains optional string `appearance_skin`; an absent or empty value means Core, while
the first supported skin id is `sky-grove`. The Settings path writes `sky-grove` for that skin and
clears/omits the field for Core through the existing config store, so R3's owner-only atomic rewrite
and ordinary hand editing apply. No SQLite row, migration, cache file, project/session field, seed
rewrite, or config-version bump is
introduced. `internal/config` owns the finite write-time validation set; a syntactically valid but
unknown value from a hand edit remains readable so the UI can fall back and explain it rather than
classifying the whole version-1 document as corrupt. Older files decode to Core without rewrite;
Core is never inserted into the manifest as if it were a skin id.

**R22 — AgentDecker proposal records are authoritative, consumable, and
bounded.** A forward-only `pipeline_proposals` table is the durable authority for the Pipelines
approval surface: content-addressed `proposal_id`, kind, digest, non-null canonical `payload_json`,
`created_at`, and `consumed_at`. A record is committed before its MCP tool reports success. Because
`proposal_id` is content-addressed, an id conflict is the same record being proposed again: the
payload and digest are already identical, and the conflict refreshes `created_at` and clears
`consumed_at` so exactly one pending offer exists — the payload a caller received is always the
payload the approval surface holds. `consumed_at` is empty until the exact mutation the proposal
describes commits and is then set once; existing rows adopt the empty value and stay pending, and
only pending records are listed. Each write applies a newest-first retention bound in
the same transaction, so a never-approved backlog cannot grow without limit. The table carries no
foreign key: proposal records neither cascade into nor out of runs, templates, agents, sessions,
transcripts, or archive projections, so deleting a run or template leaves them alone. TS-09 owns the
logical payload shapes and the lifecycle's product meaning.

**R23 — Activations are durable, payload-free operational control.** A forward-only
migration adds:

```sql
CREATE TABLE activations (
  activation_id TEXT PRIMARY KEY,
  agent_id      TEXT NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
  kind          TEXT NOT NULL,
  state         TEXT NOT NULL,
  claim_token   TEXT NOT NULL DEFAULT '',
  created_at    TEXT NOT NULL,
  claimed_at    TEXT,
  attempted_at  TEXT
);
CREATE UNIQUE INDEX idx_activations_one_pending_mail
  ON activations(agent_id) WHERE state = 'pending' AND kind = 'mail';
CREATE INDEX idx_activations_pending
  ON activations(state, created_at);
```

The only accepted `kind` in this change is `mail`; its accepted states are `pending`, `claimed`, and
`attempted`. Types and state methods validate both vocabularies rather than treating arbitrary
strings as executable work. The row contains no source payload and has no association with a live
pid/session/generation. Message insertion by either MCP or the reserved user sender and
`EnsurePendingMailActivation(agent_id)` share one SQLite transaction, with the mail-only partial
unique index as the final concurrency guard. `delivered_via` remains readable message provenance
for compatibility but no longer owns activation/wake scheduling. The table name and stable identity
reserve a shared control-plane concept; the mail-only index is not a universal `(agent_id, kind)`
coalescing contract. Before another kind is valid, its owning specification must define whether it
needs a stable source/work id, multiple independent pending rows for one agent, or another
kind-specific uniqueness key, and add the required forward migration/index.

Claim and transition updates match both `activation_id` and a newly minted `claim_token`; a delayed
worker cannot mutate a later claim. For `mail`, a live pre-attempt release and startup recovery both
delete a claimed row when another pending mail row already coalesces that agent, otherwise restore it to
`pending`; an ineligible recipient deletes its matching pre-attempt claim, while
`attempted` rows are recognized as non-replayable and deleted. The live mail handler likewise
deletes attempted rows after their bounded handoff. This is reconciliation metadata, not retained
history: it has no user-configurable TTL, archive copy, transcript/FTS projection, API/SSE
representation, or cascade into messages. A pending `mail` row with no unread source mail may be
deleted without inference. Future kinds own their retry and durable-start transition; they do not
inherit this cleanup policy merely by using the table.

The migration creates one pending `mail` activation for each agent that has unread legacy mail still
marked `delivered_via = 'pending'`, coalescing all such rows. Legacy `nudge`, `poll`,
`wake_attempted`, and `wake_failed` rows are not backfilled because they already crossed or consumed
the old attempt boundary; reactivating them on upgrade would duplicate work. Message read/hard
retention and turn-budget rows remain unchanged.

**R24 — Context references, direct grants, and personal preferences
have separate durable rows.** One forward-only migration adds the logical shape below (the
executable migration may use equivalent check constraints and indexes):

```sql
CREATE TABLE context_references (
  context_ref_id      TEXT PRIMARY KEY,
  source_kind         TEXT NOT NULL,
  source_agent_id     TEXT NOT NULL DEFAULT '',
  first_seq           INTEGER,
  last_seq            INTEGER,
  pipeline_attempt_id TEXT NOT NULL DEFAULT '',
  created_at          TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_context_ref_transcript
  ON context_references(source_agent_id, first_seq, last_seq)
  WHERE source_kind = 'transcript_span';
CREATE UNIQUE INDEX idx_context_ref_pipeline_report
  ON context_references(pipeline_attempt_id)
  WHERE source_kind = 'pipeline_attempt_report';

CREATE TABLE context_grants (
  grant_id             TEXT PRIMARY KEY,
  context_ref_id       TEXT NOT NULL REFERENCES context_references(context_ref_id),
  granted_by_agent_id  TEXT NOT NULL,
  granted_to_agent_id  TEXT NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
  label                TEXT NOT NULL DEFAULT '',
  description          TEXT NOT NULL DEFAULT '',
  created_at           TEXT NOT NULL,
  updated_at           TEXT NOT NULL,
  revoked_at           TEXT,
  UNIQUE(context_ref_id, granted_by_agent_id, granted_to_agent_id)
);
CREATE INDEX idx_context_grants_recipient
  ON context_grants(granted_to_agent_id, revoked_at, updated_at DESC, grant_id);

CREATE TABLE context_grant_preferences (
  grant_id    TEXT PRIMARY KEY REFERENCES context_grants(grant_id) ON DELETE CASCADE,
  hidden_at   TEXT
);
```

State types validate the closed source-kind vocabulary and its kind-specific locator shape: a
transcript span has one source agent and positive ordered sequence bounds and no attempt id; a
pipeline report has one attempt id and no transcript locator. The partial unique indexes are the
concurrency guard that canonicalization returns one reference for one locator; label, description,
grant, target, personal state, and creator are deliberately absent from that key. Reference rows
contain no copied transcript/report content and have no foreign key to source agents, sessions,
pipeline runs, or attempts, so deletion leaves an honest tombstone rather than cascading or aliasing
the id. References and grants have no owner-management or automatic-retention path in this feature;
an active direct grant persists until grantor revocation or the defensive recipient cascade. This is
the deliberate FS-15 §6 lifecycle policy, not missing janitor wiring.

One active-or-revoked grant row exists per reference/grantor/recipient triple. Sharing that triple
again updates its presentation, clears `revoked_at`, and clears the recipient's hidden preference in
one transaction rather than adding duplicate list entries. Grantor ids are retained logical
provenance without a foreign key. A recipient-row deletion defensively cascades only its grants and
preferences; this feature adds no agent-deletion product operation. Revocation changes only the
grant row; hiding changes only the preference row. Every
mutation matches the caller-derived grantor/recipient identity as applicable, and multi-row
canonicalize-plus-grant operations are atomic (INV §5/§15). No backfill is needed.

- **R25** — One forward-only migration adds the durable task domain: `tasks`,
  `task_arms`, `task_attachments`, and a `work_results` registration unique per source, plus a
  nullable `source_id` on `activations` with a partial unique index for the `dependency` kind, which is
  the kind-specific uniqueness key R23 requires before a second kind is valid. Arms and attachments
  cascade from their task; agent ids, pipeline run ids, and context reference ids stay logical
  references without cascades, so deleting an agent, run, or reference never deletes task history and
  deleting a task never reaches into agent identity, transcripts, the archive, a pipeline run, or a
  canonical reference. Shapes and ownership are specified in TS-10.

- **R26** — One forward-only migration adds the partial read index used by the
  pipeline delegated-agent projection (TS-09.R28):

  ```sql
  CREATE INDEX idx_tasks_project_creator_created
    ON tasks(project, created_by_agent_id, created_by_generation, created_at DESC, task_id)
    WHERE created_by_kind = 'agent' AND assigned_agent_id IS NOT NULL;
  ```

  It adds no column, row, foreign key, write authority, or retention behavior. The predicate matches
  the projection exactly: person-created work and agent-created work that has no assigned agent can
  never produce a delegated-agent card. Migration and query-plan tests prove that the targeted read
  uses this index and does not regress to scanning every task retained by the project.

- **R27** — One forward-only migration adds the `project_worktrees` ownership table
  (shape and rules in TS-12.R2/§3). The `project` column is a logical, non-cascading reference
  exactly like R25's task references: deleting or archiving a project never cascades here, and only
  the explicit consented deletion flow removes a row. The project config files additionally accept
  optional `base_branch` and `setup_command` fields (FS-04.R45); absent fields read as empty and
  legacy files stay valid unchanged.

**R28 — The turn budget is an additive version-1 config value, and nothing else
in this change persists.** `config.json` gains optional integer `message_budget_per_turn`
(FS-04.R46); an absent, zero, negative, or non-numeric value means the shipped default, so an older
file decodes without rewrite and a hand edit that is syntactically valid but out of range remains
readable rather than classifying the whole version-1 document as corrupt. `internal/config` owns
that write-time validation set, and R3's owner-only atomic rewrite applies unchanged. No SQLite row,
migration, cache file, project/session field, seed rewrite, or config-version bump is introduced.

**R29 — A proposal record is a claimed state machine over one row, and
retention is blind to which state it is in.** Migration 22 adds `declined_at TEXT NOT NULL DEFAULT
''` to `pipeline_proposals` (R22), the same empty-is-unset convention migration 15 gave
`consumed_at`; existing rows adopt the empty value and stay exactly as pending as
they are. The row's states are *pending* (`consumed_at` and `declined_at` both empty), *declined*
(`declined_at` set), and *consumed* (`consumed_at` set); Delete is a hard row delete, not a fourth
state, so a deleted record leaves no tombstone and a later identical proposal inserts a new row
(FS-14.R49/R50). Every transition is a single conditional statement whose `WHERE` names the state it
expects, and the caller decides from the rows it affected rather than by reading first and writing
after (INV §5): a decline matches `consumed_at = '' AND declined_at = ''`, a delete matches
`declined_at != '' AND consumed_at = ''`, and consumption matches `consumed_at = ''` alone — which is
what lets an approved mutation consume a record another tab already declined, the ordering FS-14.R57
chose. Delete names the unconsumed state too because that same ordering leaves `declined_at` set on
a consumed row: a delete claiming a decline alone would erase the record the approval consumed
instead of losing with the durable consumed state FS-14.R49/R57 require it to report.
Zero affected rows is the loser of a race and is reported as the state the row is actually in, never
as success and never as a failure to write. Consumption stays keyed by the content-addressed id
(TS-09.R26), so no proposal id travels through the template or start API and the approval path is
unchanged apart from the state its claim may overwrite. R22's newest-first retention bound is
unchanged and deliberately state-blind: it orders by `created_at` and prunes the oldest records over
the bound in the same write transaction whether they are pending, declined, or consumed, so a
declined backlog cannot outlive the bound and no state can pin a row forever. Nothing else about
R22 changes: the table still carries no foreign key, still cascades in neither direction, and TS-09
still owns the payload shapes and the lifecycle's product meaning.

The rest of this change deliberately persists nothing: the approval exemption is derived from code
and applied as a runtime parameter (TS-01.R27), a pending permission request stays process-lifetime
state as it already is, and the awaiting-approval attention value is derived rather than stored
(TS-09.R29). The `pipeline_attempts` and run tables are untouched, so no migration is written and
no existing durable field changes meaning.

## 3. Interfaces & data shapes

The durable layout is:

```text
$AGENTDECK_HOME/
  config.json
  backends.json
  config-sources.json
  layout.json
  roles/{id}.json
  projects/{id}.json
  pipelines/{id}.json         reusable pipeline templates
  project-resources/{id}/     opaque agent/person shared material; never indexed or scanned
  codex/                      private Codex runtime profile; sessions/history remain here
  cache/codex-profile.json    managed setup-mirror manifest; never contains credentials
  state.db
  sessions/{agent_id}/transcript.ndjson
  cache/config-sources/**
```

The binding schemas for roles, projects, backends, and global config are defined by FS-04 and
FS-09. Federation binding/effective-view shapes are defined by TS-07. SQLite table definitions and
migration order live in `internal/state/schema.go` and execute through `migrate.go`; that executable schema is subordinate to
R1–R26 and must be reflected here when its contract changes.

## 4. Invariants

- **INV §3:** durable fields needed after restart are persisted at the mutation boundary, not
  reconstructed from mutable config.
- **INV §5:** every authoritative in-memory accumulator has a restart seeding path.
- **INV §7:** stream and SQL readers handle empty, truncated, oversized, and mid-iteration failure.
- **INV §10:** caches and indexes declare their authority and refresh boundary.
- **R12 — Migration/spec lockstep.** A migration that changes a durable shape or compatibility
  promise must update this spec (and the owning FS/API spec) in the same completed change.
- **INV §2:** launch, resume, and switch use the same project-resource composition helper rather
  than independently rebuilding the environment, prompt, or additional directories.

## 5. Deviations & open decisions

- Turn-document indexing deliberately cannot satisfy a query whose terms occur only in
  different documents. That product limitation is specified by FS-05.R25–R27 and tracked for
  evidence-based reconsideration in `ideas.md`; it does not change raw transcript authority.
- Hook-only file/command activity does not consistently advance session recency. FS-05 records the
  user-visible consequence as an open gap.
- Startup does not recursively repair permissions on an existing home tree, and role/project reads
  do not reject valid-name symlink files. Both are recorded hardening gaps in
  [`ideas.md`](../../ideas.md).

## 6. Traceability

- Config: `internal/config/atomic.go`, `appconfig.go`, `appconfig_test.go`, `seed.go`, `validate.go`,
  `types.go`.
- Project resources: `internal/config` path/layout helpers; project CRUD and lifecycle
  composers in `internal/server`.
- Codex isolated profile: `internal/config` profile-refresh helper; child-env composition in
  `internal/server/{launch,resume,switch}.go`; process spawn in `internal/runtime/chat.go`.
- Schema/migrations: `internal/state/migrate.go`, `schema.go`, `state.go`, `running.go`, `session.go`.
- Archive persistence and compensation: `internal/state/agents.go`
  (`SetAgentsArchived`, `RestoreAgentArchiveStates`), `internal/server/archive_actions.go`.
- Transcript: `internal/transcript/writer.go`, `reader.go`; runtime append in
  `internal/runtime/chat.go`.
- Index/archive: `internal/index/indexer.go`, `reindex.go`, `internal/archive/archive.go`.
- Context reference persistence (R24): forward-only tables and typed state methods in
  `internal/state`, consumed through `internal/contextref`; canonicalization, cascade, tombstone,
  grant, and personal-preference regressions named by FS-15.A1–A5/A7.
- Pipeline supervision read index (R26): task migration/query-plan regressions named by
  FS-14.A17/A23 and TS-09.R28.
- Proposal decline/delete claims (R29): migration 22 in `internal/state/schema.go`,
  `DeclinePipelineProposal`/`DeletePipelineProposal`/`ListPipelineProposals` in
  `internal/state/pipelines.go`, the claim-loss sentinels in `internal/state/errors.go`, and
  `internal/state/pipeline_proposals_test.go`.
- Worktree ownership (R27): migration 20 in `internal/state/schema.go`, typed methods in
  `internal/state/worktrees.go`, path rules in `internal/config/worktrees.go`; regressions in
  `internal/state/worktrees_test.go` and `internal/config/worktrees_test.go`.
- Regression anchors: `TestHomeTreeIsOwnerOnly`, `TestStateDBIsOwnerOnly`,
  `TestTranscriptIsOwnerOnly`, `TestReindexPreservesFinalPartialTurn`,
  `TestEmptyArchiveMarshalsResultsArray`, `TestSearchFallbackFiltersMetadata`,
  `TestArchiveProjectReportsCompensationFailureAndPublishesDurableFallback`.
- Planned agent-knowledge cache ownership and atomic projection: TS-11.R1–R3.
