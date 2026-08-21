# TS-02 — Data & persistence

**Status:** Current
**Code:** `internal/config`, `internal/state`, `internal/transcript`, `internal/index`, `internal/archive`, `internal/configsource`
**Absorbed:** exact source mapping in the [phase archive manifest](../../archive/phases/README.md)

## 1. Scope

This spec owns durable data boundaries, file formats, SQLite ownership and migrations, transcript
storage, and rebuildable indexes. Product-visible archive behavior is in FS-05; federation-specific
source bindings and cache rules are in TS-07.

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
worker cannot mutate a later claim. For `mail`, startup recovers pre-side-effect `claimed` rows to
`pending`, or deletes them when another pending mail row already coalesces that agent, while
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
R1–R23 and must be reflected here when its contract changes.

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
- Regression anchors: `TestHomeTreeIsOwnerOnly`, `TestStateDBIsOwnerOnly`,
  `TestTranscriptIsOwnerOnly`, `TestReindexPreservesFinalPartialTurn`,
  `TestEmptyArchiveMarshalsResultsArray`, `TestSearchFallbackFiltersMetadata`,
  `TestArchiveProjectReportsCompensationFailureAndPublishesDurableFallback`.
