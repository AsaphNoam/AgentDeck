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
error. Behavior differences are explicitly specified by FS-05.

**R11 — Message persistence is transactional.** Messages, read state, expiry, and per-turn budget
state live in SQLite. A send either stores the message and updates its budget atomically or stores
nothing; readers return newest-first bounded results as specified by FS-06.

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
  state.db
  sessions/{agent_id}/transcript.ndjson
  cache/config-sources/**
```

The binding schemas for roles, projects, backends, and global config are defined by FS-04 and
FS-09. Federation binding/effective-view shapes are defined by TS-07. SQLite table definitions and
migration order live in `internal/state/schema.go` and execute through `migrate.go`; that executable schema is subordinate to
R1–R11 and must be reflected here when its contract changes.

## 4. Invariants

- **INV §3:** durable fields needed after restart are persisted at the mutation boundary, not
  reconstructed from mutable config.
- **INV §5:** every authoritative in-memory accumulator has a restart seeding path.
- **INV §7:** stream and SQL readers handle empty, truncated, oversized, and mid-iteration failure.
- **INV §10:** caches and indexes declare their authority and refresh boundary.
- **R12 — Migration/spec lockstep.** A migration that changes a durable shape or compatibility
  promise must update this spec (and the owning FS/API spec) in the same completed change.

## 5. Deviations & open decisions

- Full transcript indexing currently rewrites an in-memory whole-session projection at turn
  boundaries. This is correct but can be expensive for very long sessions; chunking is backlog work,
  not an alternative authority model.
- Hook-only file/command activity does not consistently advance session recency. FS-05 records the
  user-visible consequence as an open gap.
- Startup does not recursively repair permissions on an existing home tree, and role/project reads
  do not reject valid-name symlink files. Both are recorded hardening gaps in
  [`product-backlog.md`](../../product-backlog.md).

## 6. Traceability

- Config: `internal/config/atomic.go`, `seed.go`, `validate.go`, `types.go`.
- Schema/migrations: `internal/state/migrate.go`, `schema.go`, `state.go`, `running.go`, `session.go`.
- Transcript: `internal/transcript/writer.go`, `reader.go`; runtime append in
  `internal/runtime/chat.go`.
- Index/archive: `internal/index/indexer.go`, `reindex.go`, `internal/archive/archive.go`.
- Regression anchors: `TestHomeTreeIsOwnerOnly`, `TestStateDBIsOwnerOnly`,
  `TestTranscriptIsOwnerOnly`, `TestReindexPreservesFinalPartialTurn`,
  `TestEmptyArchiveMarshalsResultsArray`, `TestSearchFallbackFiltersMetadata`.
