# Phase 4 — Implementation Tech Spec: Persistence — transcript archive, full-text search, resume, file/command tracking

**Mirrors:** `docs/phases/phase-4-persistence-archive.md` (phase PRD)
**Master PRD:** `agent-dashboard-prd.md` (source of truth — F9, F10, §3.5 `sessions/` layout, §4.1 `Runtime.Resume`, §6 REST surface)
**Builds on:** Phase 1 (Runtime interface, normalized transcript `Event`s, `LaunchSpec` + composition, `state.db` agent/running/status rows), Phase 2 (state manager, SSE bus, `state_update`/`new_message`, `GET /api/sessions/{id}/transcript` minimal in-memory endpoint)
**Status:** ready to implement after Phase 2
**Audience:** the engineer implementing Phase 4. This document is complete enough to implement with essentially no further design decisions. Where the PRD leaves something open (phase §6, master PRD §9), this spec pins a concrete decision — see §12.

---

## 0. Codex review findings — address while building this phase

> Recorded 2026-06-28 from a cross-phase Codex review. Resolve each as you build the
> referenced subphase; delete the entry once implemented and verified green.

- **BLOCKING — the indexer can wipe durable FTS content from a process-local accumulator (§2.3 / §4.6).**
  §4.6 has the indexer accumulate a session's searchable text **in memory** and replace its
  `sessions_fts` row wholesale per `turn_end` (FTS5 updates are delete+insert, §2.3 line ~162). After a
  **server restart or a resume**, that accumulator (`Indexer.content`) is empty/partial — it only holds
  what the current process streamed — so the first `OnTurnEnd` **replaces the durable FTS row with only
  the new turn's text, wiping every previously-indexed turn**. **Resolution:** never let a fresh process's
  accumulator overwrite durable content. Either (a) on first touch after `Start`/`Resume`, hydrate the
  per-session accumulator from the durable source — the persisted FTS `content`, or by replaying
  `transcript.ndjson` up to `last_seq` — before the first wholesale replace; or (b) make `OnTurnEnd` a
  read-modify-write that appends the new turn to the existing row rather than reconstructing it from
  memory. (Treat the raw `transcript.ndjson` as authoritative, per §2.4/§2.6.)

- **BLOCKING (cross-project) — the FTS5 build tag is missing from the normal build/install paths
  (§2.5, line ~188).** This spec assumes `-tags sqlite_fts5`, but the real build/install paths do **not**
  set it: `Makefile` (the `build:` target, `go build … ./cmd/agentdeck`) and `install.sh` (its `go build`
  line) build untagged. Once Phase 4 lands the FTS5 schema + `MATCH` SQL, an **installed binary fails** at
  `CREATE VIRTUAL TABLE … USING fts5` (migration) and/or at `GET /api/archive?q=…`. **Resolution when this
  phase lands:** add `-tags sqlite_fts5` to the `Makefile` build target, `install.sh`, and any `go test`
  invocation that exercises FTS, and document the tag as required to build AgentDeck. *(Recorded only —
  build files left unchanged for now, since FTS5 isn't in the schema yet.)*

- **Advisory — resume does not mint a fresh MCP registration (§5.1 step 4 / §5.2 / §5.7).** Resume
  correctly rebuilds the `LaunchSpec` from the **frozen `sessions` snapshot** (§5.2, *not* from current
  config). But the messaging-MCP seam carries a **per-launch hook token** (Phase 1 §6.4) that is
  ephemeral/in-memory and is therefore **not** in the persisted snapshot — so §5.7's "passes the same
  LaunchSpec MCP seam" has nothing valid to pass. **Resolution:** on resume, re-mint a fresh hook token
  and re-inject the messaging MCP server spec (`command = os.Executable()`, args with the **new** token +
  `agent_id`) into the rebuilt `LaunchSpec`, exactly like `Start` does — don't rely on the frozen
  snapshot's `MCPServers`.

---

## 1. Overview & scope recap

### 1.1 What this phase delivers

Phase 1/2 produced a transcript that **streams** (`new_message` on the SSE bus) but lives only in memory — kill the server and the conversation is gone; a stopped agent still shows as a card (Phase 2 §3.5) but has no recoverable history. Phase 4 makes work **durable and recoverable**:

1. **Transcript persistence (F9):** every normalized transcript `Event` (Phase 1 §4.2) is appended to `~/.agentdeck/sessions/{agent_id}/transcript.ndjson` as it streams, crash-safely (survive a kill mid-turn with no loss of already-streamed content). In parallel, the server upserts session/transcript **metadata and searchable content into `state.db`** so the archive can list and full-text-search without touching the raw files.
2. **Archive + search (F9):** `GET /api/archive?q=...` lists **all** sessions — active and inactive — with name/role/project/timestamps, and full-text-searches over name, role, project, and transcript content. Both the listing and the search are **`state.db` queries** — the search uses a **SQLite FTS5** virtual table.
3. **Resume (F9):** `POST /api/sessions/{id}/resume` restores identity/metadata from `state.db` + history from `sessions/`, implements the **chat-runtime `Runtime.Resume`** (stubbed in Phase 1), preserves the stable `agent_id` while writing a fresh ephemeral `session_id` to the running row in `state.db`, re-attaches a runtime, registers the in-process MCP server, and reappears as a live card. The CLI launch of an existing identity resumes instead of duplicating.
4. **File & command tracking (F10):** per-agent **Files** and **Commands** views, captured from chat-runtime tool calls and `POST /api/hook` into **rows in `state.db`**, searchable and copyable, with diff linkage where a `diff` event exists.

This phase also delivers the **non-destructive resume machinery** that Phase 6 (switch interface/backend/model on a live agent, F7) reuses verbatim.

### 1.2 In scope

- A `transcript` package (Go): a per-agent NDJSON append writer (the durable raw log) that the chat runtime feeds in parallel with the SSE bus; crash-safe append; a reader that replays a transcript into `Event`s.
- `state.db` schema for Phase 4: session/transcript **metadata** tables, an **FTS5 virtual table** over searchable content, and **file/command** rows — all keyed to `agent_id`, written by the server as the sole writer.
- An **indexer** that, as events stream (and on demand for backfill), upserts metadata + FTS5 content + file/command rows into `state.db` from the normalized events, so `state.db` is rebuildable from the raw `sessions/` logs.
- Archive listing + full-text search: `GET /api/archive?q=...` backed entirely by `state.db` (listing = metadata query; search = FTS5 `MATCH` query).
- Resume: real `ChatRuntime.Resume`; `POST /api/sessions/{id}/resume`; CLI resume-not-duplicate; upgrade of Phase 2's `GET /api/sessions/{id}/transcript` to read **persisted** history from the raw `sessions/` log (live or archived).
- File/command tracking: `GET /api/sessions/{id}/files`, `GET /api/sessions/{id}/commands`, served from `state.db` rows, with diff linkage.

### 1.3 Out of scope (and how it slots in later)

| Item | Status this phase | Lands in |
|------|-------------------|----------|
| Switch interface/backend/model on a live agent (F7) | Out — but its resume machinery is built here and reused | Phase 6 |
| Terminal runtime `Resume` | Out — `TerminalRuntime` is still a `501` stub; only `ChatRuntime.Resume` is real | Phase 6 |
| Messaging, nudger, notifications | Out (the in-process MCP server is *registered* on resume here; its messaging tools land in Phase 5) | Phase 5 |
| Hooks-driven file/command capture for **chat** agents | Chat agents derive files/commands from the ACP transcript (master PRD §4.4: chat agents may skip redundant hook writes); the `POST /api/hook` path is the terminal-runtime capture mechanism, wired into the same `state.db` tables here | Phase 6 (terminal producer) |
| Archive UI polish (read-only transcript viewer styling, infinite scroll) | Minimal viable list + search box + open-result; reuses the Phase 2 `TranscriptView` renderers in read-only mode | this phase (minimal), polish later |

### 1.4 Where this plugs into existing code

- The **chat runtime** (Phase 1 §4) already emits normalized `Event`s to the bus. Phase 4 adds two sinks fed in lockstep: a `transcript.Writer` per agent (the durable raw NDJSON log) and an indexer call that upserts the event's searchable content + any file/command rows into `state.db`. This is the only change to runtime hot-path code.
- The **state manager / SSE bus** (Phase 2) are unchanged: a resumed agent writes its running/status rows to `state.db` exactly like a fresh launch, so it reappears as a card through the normal `state_update` path. No bus changes.
- Phase 2's in-memory `GET /api/sessions/{id}/transcript` is **upgraded** to read the persisted NDJSON, removing the "retains nothing → empty array" caveat.

---

## 2. Technology choices

All server-side; Go 1.22+, single binary (Phase 0 constraint). The storage split follows the project rule (`docs/architecture-decisions.md`): **human-edited config stays in plain JSON files; machine-generated state lives in `state.db` (SQLite), with the Go server as the sole writer.** Phase 4 adds the session/transcript metadata, FTS5 index, and file/command rows to that DB, and keeps the agent CLI's raw transcript stream as durable append files under `sessions/`.

### 2.1 Transcript on-disk format — NDJSON append log (durable source)

**Decision: one append-only NDJSON file per session, `sessions/{agent_id}/transcript.ndjson`, one normalized `Event` per line.** This raw log is the **durable source of truth for displayed history** and the input from which `state.db` is (re)built. (Phase PRD §3.1 and master PRD §9 both recommend NDJSON append; this spec locks it in.)

| Option | Verdict | Why |
|--------|---------|-----|
| **Single growing JSON array** (`[ev, ev, …]`) | **Rejected** | Appending requires rewriting the closing `]` (read-modify-write of the file tail) or holding the whole array in memory and re-serializing on every event. A kill mid-write leaves a syntactically invalid file (`…,` with no `]`), so the *entire* transcript becomes unparseable — the exact crash-safety failure we must avoid. Streaming reads are impossible (you must parse the whole array). |
| **NDJSON append log** (one JSON object per line) | **Chosen for the raw source** | Append is a pure `O(1)` `write(2)` of `marshal(ev)+"\n"` — no rewrite, no whole-file buffer. A crash mid-write loses **at most the last partial line**; every prior complete line is intact and independently parseable (just skip a trailing partial line on read — §8.1). Streaming/tailing reads are natural (`bufio.Scanner` line-by-line, exactly the Phase 1 reader we already trust). Each line is self-describing (it *is* an `Event`), so the format needs no schema versioning beyond the `Event` envelope's append-only rule. |

The queryable index and metadata do **not** live in this file — they live in `state.db` (§2.2). The raw log stays a plain append log precisely so the runtime hot path never does anything fancier than an `O(1)` write; everything searchable is derived into the DB by the indexer.

**Crash-safety mechanics for the raw append log (the load-bearing part):**

- Open the file `O_APPEND|O_CREATE|O_WRONLY`. `O_APPEND` makes each `write` atomic with respect to the file offset, so concurrent/interleaved writes never corrupt each other's offsets.
- **Marshal the whole record into one `[]byte` (including the trailing `\n`) and issue a single `Write`.** A record is never written field-by-field, so a crash can only ever truncate at a record boundary or mid-record — and a mid-record truncation is a *partial last line* the reader discards (§8.1). It can never corrupt an earlier record.
- **Durability tier: `fsync` on turn boundaries, not per event.** We `f.Sync()` after writing a `turn_end` or `error` record, and on `Stop`/`Resume`/graceful shutdown — not after every `assistant_text` delta (that would fsync dozens of times per turn and stall streaming). The OS page cache holds in-flight deltas; a process *crash* (not a power loss) loses nothing because the bytes are already in the kernel buffer. This satisfies the acceptance criterion "killing the server mid-turn does not lose already-streamed transcript content" (a `kill` is a process crash; the kernel flushes the page cache to disk). Power-loss durability of the final in-flight deltas is explicitly not guaranteed and is acceptable for a local dev tool (§12.6).
- **Write-temp-rename is NOT used for the raw transcript** (it is the right tool only for the small human-edited JSON config files, per Phase 0 convention). Append logs and temp-rename are incompatible; the append-atomicity above is the correct crash-safety primitive for a log.

### 2.2 Full-text search approach — SQLite FTS5 in `state.db`

**Decision: full-text search is a SQLite FTS5 virtual table in `state.db`, populated by the indexer from the raw `sessions/` logs.** Listing the archive and searching it are both `state.db` queries; no transcript files are read on the archive path. The raw NDJSON logs remain the durable source, and `state.db` is a **rebuildable index** over them.

| Option | Verdict | Why |
|--------|---------|-----|
| **SQLite FTS5** (`github.com/mattn/go-sqlite3`) | **Chosen** | Real full-text search (tokenized, ranked via `bm25()`) over name/role/project/transcript content with a single `MATCH` query; constant-cost listing via an indexed metadata table; transactional upserts; rebuildable from the raw logs. The server is the sole writer (architecture-decisions D1/D2), so SQLite is safe with no multi-process contention. One file under `~/.agentdeck/`, fully local. |
| **Hand-rolled per-query scan of every transcript file** | **Rejected** | Reading and tokenizing every `transcript.ndjson` on each query does not scale with session count and re-derives the same work on every keystroke. FTS5 maintains the inverted index once on write. |
| **Separately maintained on-disk index file (Bleve/etc.)** | **Rejected** | Pulls a search-engine dependency and a bespoke staleness/rebuild story for an index that SQLite already gives us transactionally alongside the metadata we need anyway. |

**Concretely:** the indexer keeps an FTS5 row of searchable content per session in sync as events stream; `GET /api/archive` with no `q` is a metadata `SELECT` ordered by `updated_at`, and with `q` it is an FTS5 `MATCH` joined back to metadata, ranked by `bm25()` (§4). Because `state.db` is derived from the raw logs, it can be dropped and rebuilt by replaying `sessions/` (§2.4, `agentdeck reindex`).

### 2.3 `state.db` schema for Phase 4

The server opens `state.db` in **WAL mode** (`PRAGMA journal_mode=WAL`) — see §2.5 for the crash-safety rationale. Phase 1/2 already created the `agents`, `running`, and `status` tables; Phase 4 adds the session/transcript metadata, the FTS5 index, and the file/command tables. All are keyed to the stable `agent_id`.

```sql
-- session/transcript metadata: one row per agent_id (the archive listing source)
CREATE TABLE IF NOT EXISTS sessions (
  agent_id        TEXT PRIMARY KEY,          -- stable identity (matches agents.agent_id)
  name            TEXT NOT NULL,
  role            TEXT NOT NULL,
  project         TEXT NOT NULL,
  backend         TEXT NOT NULL,
  model           TEXT NOT NULL,
  interface       TEXT NOT NULL,             -- 'chat' | 'terminal'
  grp             TEXT,                       -- "group" is reserved in SQL; column is grp
  cwd             TEXT NOT NULL,
  system_prompt   TEXT NOT NULL,             -- the exact composed prompt (Phase 1 §6.2)
  env_keys        TEXT NOT NULL DEFAULT '[]',-- JSON array of env KEY NAMES only; never values (§8.7)
  last_session_id TEXT,                       -- latest ephemeral CLI session id (resume history is in session_meta events)
  last_seq        INTEGER NOT NULL DEFAULT 0, -- max persisted seq, for resume continuity (§5.4)
  last_context_pct REAL NOT NULL DEFAULT 0,   -- last-known context_pct, restored on resume (§5.3)
  created_at      TEXT NOT NULL,              -- RFC3339 UTC
  updated_at      TEXT NOT NULL,              -- last turn_end / activity
  turn_count      INTEGER NOT NULL DEFAULT 0,
  event_count     INTEGER NOT NULL DEFAULT 0,
  files_touched   INTEGER NOT NULL DEFAULT 0, -- rollup count for listing (detail in tracked_files)
  commands_run    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions(updated_at DESC);

-- FTS5 full-text index over searchable session content.
-- content='' = contentless (external content): we store the searchable text ourselves
-- and keep it in sync from the raw logs; the table is rebuildable.
CREATE VIRTUAL TABLE IF NOT EXISTS sessions_fts USING fts5(
  agent_id UNINDEXED,   -- carried so a MATCH row joins back to sessions; not tokenized
  name,                 -- metadata fields are indexed so a name/role/project hit ranks as metadata
  role,
  project,
  grp,
  model,
  backend,
  content,              -- accumulated transcript text (assistant_text, tool names+args, tool_result, diff path+new_text, permission reason)
  tokenize = 'unicode61 remove_diacritics 2'
);

-- per-file rollup (F10 Files tab); one row per (agent_id, path)
CREATE TABLE IF NOT EXISTS tracked_files (
  agent_id     TEXT NOT NULL,
  path         TEXT NOT NULL,                 -- cwd-relative where resolvable, else absolute
  abs_path     TEXT NOT NULL,
  edit_count   INTEGER NOT NULL DEFAULT 0,
  first_seq    INTEGER NOT NULL,
  last_seq     INTEGER NOT NULL,
  first_ts     TEXT NOT NULL,
  last_ts      TEXT NOT NULL,
  has_diff     INTEGER NOT NULL DEFAULT 0,    -- 0/1
  diff_refs    TEXT NOT NULL DEFAULT '[]',    -- JSON array of {seq, tool_call_id} into the transcript (§6.5)
  PRIMARY KEY (agent_id, path)
);
CREATE INDEX IF NOT EXISTS idx_files_agent_ts ON tracked_files(agent_id, last_ts DESC);

-- per-command occurrence (F10 Commands tab); one row per tool_call (not deduped)
CREATE TABLE IF NOT EXISTS tracked_commands (
  agent_id     TEXT NOT NULL,
  seq          INTEGER NOT NULL,
  ts           TEXT NOT NULL,
  tool_call_id TEXT NOT NULL,
  command      TEXT NOT NULL,
  exit_status  TEXT NOT NULL DEFAULT 'in_progress', -- in_progress|completed|failed
  exit_error   TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (agent_id, seq)
);
CREATE INDEX IF NOT EXISTS idx_commands_agent_seq ON tracked_commands(agent_id, seq DESC);
```

Notes:
- **`sessions_fts` is contentless (`content=''`-style external content):** the indexer writes the searchable text into it and updates it as content accumulates; nothing is duplicated into a shadow content table beyond what we choose to store. The whole table is rebuildable by replaying the raw logs (§2.4).
- **`content` accumulation:** the indexer appends each event's searchable text to the session's FTS `content`. Because FTS5 rows are replaced wholesale on update, the indexer keeps the accumulating `content` per live session in memory and rewrites the FTS row on a debounce (per `turn_end`), or for archived/backfill sessions builds it in one pass — see §4.6.
- **Secrets never enter the DB:** `env_keys` is key-names-only JSON; values are re-resolved from `backends.json` at resume (§8.7). The same prohibition applies to the FTS `content` (tool args may echo env — the indexer stores normalized event text, not raw frames; raw frames are debug-only, §12.2).

### 2.4 Raw logs are the source; `state.db` is the rebuildable index

The split is the spine of this phase, so state it crisply:

- **`sessions/{agent_id}/transcript.ndjson` (raw NDJSON append log) = the durable source.** It is what survives a crash mid-turn, what `GET /api/sessions/{id}/transcript` replays for the chat panel, archive read-only view, and resume repaint, and what the files/commands derivation and the FTS index are built from.
- **`state.db` (SQLite, FTS5 + metadata + file/command rows) = the queryable, rebuildable index.** It exists to make listing, full-text search, and the Files/Commands tabs fast and transactional. It carries no information that cannot be reconstructed by replaying the raw logs.

This means a corrupted or deleted `state.db` is recoverable: `agentdeck reindex` truncates the Phase 4 tables and replays every `sessions/{agent_id}/transcript.ndjson`, re-upserting metadata, FTS content, and file/command rows. Conversely, losing a raw log loses that session's history — which is why the raw log gets the append-only crash-safety treatment (§2.1) and `state.db` gets WAL (§2.5).

### 2.5 WAL crash-safety for `state.db`

`state.db` is opened with `PRAGMA journal_mode=WAL` and `PRAGMA synchronous=NORMAL`:

- **WAL** lets readers (archive listing/search, files/commands endpoints) proceed concurrently with the single writer (the server's indexer), and makes a process **crash** recover cleanly: an interrupted transaction is rolled back from the WAL on the next open, never leaving a half-written index. This is the DB analogue of the raw log's append-atomicity — it is what makes the index safe to write on the hot path.
- **`synchronous=NORMAL`** matches the raw log's durability tier (§2.1): no loss on a process crash; the last in-flight transaction may be lost only on power loss, which is out of scope for a local dev tool (§12.6). Index writes batch per `turn_end` so they are cheap.
- Because the **server is the sole writer** (architecture-decisions D1/D2: status arrives over HTTP, not via other processes touching the DB), there is no multi-process write contention to reconcile — the precondition that makes SQLite-as-authoritative safe here.

### 2.6 Library / stdlib table

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Raw append writer | `os.OpenFile(O_APPEND\|O_CREATE\|O_WRONLY, 0o644)` + single `Write` per record | §2.1; stdlib, atomic appends. |
| Transcript read / replay | `bufio.Scanner` with the same **8 MiB** max-token buffer as Phase 1 (§2 of Phase 1 techspec) + `encoding/json` per line | A `diff`/`tool_result` record can be large; reuse the proven Phase 1 framing so live and persisted parsing share a code path. |
| `state.db` access | `github.com/mattn/go-sqlite3` (the project's existing SQLite driver) via `database/sql`; FTS5 enabled by build tag `sqlite_fts5` | §2.2; FTS5 ships with `mattn/go-sqlite3` when built with the `sqlite_fts5` tag (CI/build sets it). |
| Small JSON config files | `encoding/json` + Phase 0 write-temp-rename helper | Atomic small-file writes for the human-edited config (`roles/`, `projects/`, `backends.json`, …); state lives in `state.db`, not files. |
| Diff linkage | reuse Phase 1 `DiffData` already in the transcript; store `(seq, tool_call_id)` refs in `tracked_files.diff_refs` | No new storage; files rows reference the raw transcript (§6.5). |

> **Buffer enlargement is load-bearing for the raw reader** (same warning as Phase 1 §2): the persisted-transcript reader and the indexer's backfill pass must use the 8 MiB scanner cap, or replaying a session that contains a large diff frame silently truncates that record. The reader test (§10) must replay a transcript containing a > 64 KiB line.

---

## 3. Transcript persistence design

### 3.1 `sessions/{agent_id}/` file layout

```
~/.agentdeck/sessions/{agent_id}/
  transcript.ndjson     append-only log; one normalized Event per line (the durable raw history)
  raw.ndjson            (optional, debug-only) raw ACP frames behind AGENTDECK_RAW_TRANSCRIPT=1 (§12.2)
```

- **`transcript.ndjson`** — the durable transcript and the source `state.db` is built from. One line per Phase 1 `Event` (§3.2 below). Never rewritten, only appended; survives stop and resume (a resumed session **continues appending to the same file** — §5.4, so the full logical history is one file across resumes).
- Session/transcript **metadata** (name/role/project/timestamps/counts/composed config) is **not** a file here — it is the `sessions` row in `state.db` (§2.3), upserted by the indexer. There is no `session.json` and no `running/` marker file; the running registry is the `running` row in `state.db` (Phase 1/2).

> The directory is created on first `Start` of an agent; Phase 4 adds the `transcript.Writer.Open` call into the launch/Start path (§3.5) and the first `state.db` `sessions`-row upsert.

### 3.2 NDJSON record schema (one per transcript event)

**Each line is exactly the Phase 1 normalized `Event` envelope, marshaled as compact JSON + `\n`.** No wrapping, no re-shaping — the raw persisted line and the SSE `new_message` payload are byte-identical objects, so a single serializer feeds both the raw log and the bus, and the replay reader reconstructs the exact stream the chat panel originally saw. The indexer reads these same normalized events to populate `state.db`.

The envelope (Phase 1 §3.1), restated for this spec's completeness:

```jsonc
// one line of transcript.ndjson
{
  "agent_id": "a_8f3c12",
  "seq": 7,                       // per-agent monotonic; persisted so resume continues from max+1 (§5.4)
  "type": "assistant_text",       // assistant_text|tool_call|tool_result|diff|permission_request|turn_end|error
  "ts": "2026-06-22T10:04:12Z",   // RFC3339 UTC
  "data": { "delta": "Sure, I'll " }   // type-specific payload (Phase 1 §4.2 *Data structs)
}
```

Per-`type` `data` payloads are exactly the Phase 1 structs (`AssistantTextData`, `ToolCallData`, `ToolResultData`, `DiffData`, `PermissionRequestData`, `TurnEndData`, `ErrorData`). **The raw log retains the normalized events only** (not the raw ACP stream); raw retention is available behind a flag for debugging (§12.2). Because each line is self-describing, the file needs no header and no schema-version field beyond the `Event` envelope's append-only evolution rule (Phase 1 §11: `agent_id`/`seq`/`type`/`ts`/`data` permanent; new `type`s additive; payload fields append-only).

**Two persisted record refinements over the live stream:**

- **Permission outcomes are persisted.** The live stream emits `permission_request`; the *resolution* (approve/deny/timeout/auto-approve) must also be in the transcript so a replayed/archived view shows the outcome, not a dangling open gate. Because the raw log is append-only we cannot edit the original record in place, so we append a dedicated terminal record (§12.3). We add a `permission_resolved` event `type` (additive, allowed by the append-only rule):

  ```jsonc
  { "agent_id":"a_8f3c12","seq":12,"type":"permission_resolved","ts":"...",
    "data":{ "tool_call_id":"tc_4","decision":"approve|deny|timeout|auto_approve" } }
  ```

  The live runtime emits this on the bus too (so the open chat panel collapses the gate to a resolved chip — Phase 2 §7.3 already shows a resolved chip; this just makes it event-driven and persisted). This is the one new transcript event type Phase 4 introduces; it is forward-compatible (Phase 2 clients that don't know it ignore it).

- **`session_meta` first record.** The very first line written to a fresh `transcript.ndjson` is a `session_meta` record capturing the composed config snapshot, so the raw log is **self-contained for reconstruction** — `state.db` can be rebuilt from the logs alone, including the metadata row:

  ```jsonc
  { "agent_id":"a_8f3c12","seq":0,"type":"session_meta","ts":"...",
    "data":{ "name":"Atlas","role":"implementer","project":"my-app","backend":"claude",
             "model":"sonnet-4-6","interface":"chat","cwd":"/Users/…/my-app",
             "system_prompt_sha":"…","created_at":"…","resumed_at":null } }
  ```

  `seq:0` is reserved for `session_meta`; transcript events start at `seq:1` (Phase 1 contract). On resume we append a *new* `session_meta` with `resumed_at` set and the fresh ephemeral context, but `seq` continues monotonically (it is not reset). The indexer reads this record to upsert the `sessions` row's identity/config columns during a backfill/reindex.

### 3.3 The `state.db` session row (composed config + listing fields)

The metadata that the archive lists and that resume re-applies is the `sessions` row (§2.3 schema). Logically:

```jsonc
// state.db sessions row (illustrative JSON view of one row)
{
  "agent_id": "a_8f3c12",
  "name": "Atlas",
  "role": "implementer",
  "project": "my-app",
  "backend": "claude",
  "model": "sonnet-4-6",
  "interface": "chat",
  "grp": "auth-migration",

  // composed config snapshot (frozen at launch; what resume re-applies — §5.2)
  "cwd": "/Users/asaaph/Projects/my-app",
  "system_prompt": "…project context…\n\n…role persona…",   // the exact composed prompt (Phase 1 §6.2)
  "env_keys": ["OPENAI_BASE_URL"],                           // names only; never persist secret values (§8.7)

  // ephemeral runtime ids across resumes (latest only; full history is in the transcript session_meta records)
  "last_session_id": "claude-sess-xyz",
  "last_seq": 312,
  "last_context_pct": 0.42,

  // listing / index fields (queried for the archive — §4)
  "created_at": "2026-06-22T10:00:00Z",
  "updated_at": "2026-06-22T11:32:08Z",     // last turn_end / activity
  "turn_count": 14,
  "event_count": 312,
  "files_touched": 7,                        // rollup; detail in tracked_files (§6.4)
  "commands_run": 4
}
```

- Upserted at: session creation (from `LaunchSpec` + identity), on `/rename`, and on every `turn_end` (counts, `updated_at`, `last_seq`, `last_context_pct`).
- **Composed config is frozen here at launch** (master PRD invariant: edits to role/project affect future launches only). Resume re-applies *this snapshot*, not the current `roles/`/`projects/` files — see §5.2 and §12.4.
- **Secrets are never written.** Only env *key names* are stored (`env_keys`); values are re-resolved from `backends.json` at resume time (§8.7).
- **Liveness is not a column on this row.** Whether an agent is running is the presence of its `running` row in `state.db` (Phase 1/2). The archive joins `sessions` against `running` to compute `active` (§4.2).

### 3.4 What's persisted to reconstruct the chat panel + composed config

Reconstructing the **chat panel** (Phase 2 `TranscriptView`) requires replaying the raw transcript events in order; each renderer (`AssistantText`, `ToolCall`, `ToolResult`, `DiffBlock`, `PermissionPrompt`→resolved chip, `TurnError`) consumes the exact `Event` shapes it consumed live. Therefore persisting the normalized `Event` stream verbatim in `transcript.ndjson` is **sufficient and necessary**: nothing extra is needed for the panel.

Reconstructing the **composed config** (chat header: name/model/context; and resume input) comes from the `state.db` `sessions` row (with the transcript's `session_meta` record as the rebuild source). `context_pct` is not in the transcript line-by-line; the last-known value is stored in `sessions.last_context_pct` (updated from each `turn_end`) and restored to the `status` row on resume (§5.3).

### 3.5 Wiring the writer + indexer into the runtime (the only hot-path change)

```go
// internal/transcript/writer.go
type Writer struct {
    f   *os.File      // O_APPEND|O_CREATE|O_WRONLY on transcript.ndjson
    mu  sync.Mutex    // serialize Append (single agent, low contention)
    dir string
}

func Open(home, agentID string) (*Writer, error)   // mkdir sessions/{id}/, open append (write session_meta if new)
func (w *Writer) Append(ev runtime.Event) error     // marshal ev + "\n", single Write; fsync on turn_end/error
func (w *Writer) Sync() error                        // explicit fsync (Stop/Resume/shutdown)
func (w *Writer) Close() error

// internal/index/indexer.go — upserts state.db from normalized events (server is sole writer)
type Indexer struct{ db *sql.DB }
func (ix *Indexer) OnEvent(agentID string, ev runtime.Event) error  // accumulate FTS content; insert file/command rows
func (ix *Indexer) OnTurnEnd(agentID string, st TurnRollup) error    // flush FTS row; update sessions counts/updated_at/last_seq
func (ix *Indexer) UpsertSessionMeta(agentID string, m SessionMeta) error
```

- `ChatRuntime.Start` (Phase 1 §4.1) gains: after `session/new` succeeds and before returning, `transcript.Open(home, agentID)` writes the `session_meta` record, and the indexer upserts the `sessions` row (from `LaunchSpec` + identity).
- `ChatRuntime.dispatch` (Phase 1 §4.3), at the point it currently does `hub.Publish(ev)` / `bus.Publish("new_message", ev)`, **also** calls `writer.Append(ev)` then `indexer.OnEvent(agentID, ev)`. Order: **append to the raw log first, then publish, with index upsert alongside.** Persisting to the durable raw log before publishing guarantees that anything a client ever saw is already on disk (no "streamed but not persisted" window), directly satisfying the crash-mid-turn acceptance criterion. The index upsert is a `state.db` write in the same WAL transaction batch; if it lags or fails, the raw log is still authoritative and a reindex recovers it (§2.4).
- On `turn_end`/`error`: `writer.Sync()`, then `indexer.OnTurnEnd` flushes the accumulated FTS `content` row and updates the `sessions` row (counts, `updated_at`, `last_seq`, `last_context_pct`) and any file/command rollups — all in one DB transaction.
- `Stop` (Phase 1 §8.5): `writer.Sync()` + `writer.Close()`, and the indexer flushes any pending FTS content, *before* deleting the `running` row. **`sessions/` and the `sessions` row are left intact** (this is the F9 "stop keeps history" requirement); only the `running` row is removed (status left at `done`, per Phase 1 §7.5). The session is now an *inactive archive entry*.

---

## 4. Archive + search design

Package: `internal/archive` (Go). Type: `Archive` — runs `state.db` queries; holds no long-lived state (the DB is authoritative).

### 4.1 What "a session" is (active + inactive)

A session exists iff a `sessions` row exists in `state.db` (created together with the `agents` row at launch). The archive lists **every** such session regardless of whether a `running` row is present:

- **Active** = a `running` row for that `agent_id` exists (equivalently, a live handle is in the registry).
- **Inactive** = stopped: `sessions` (and `agents`) row present, no `running` row.

This is why the archive is a superset of the Phase 2 dashboard (which shows active + recently-stopped cards): the archive is the durable, searchable record of all of them.

### 4.2 Listing (no `q`) — `state.db` metadata query

`GET /api/archive` (no `q`) is a single query against `sessions` left-joined to `running` for liveness:

```sql
SELECT s.*, (r.agent_id IS NOT NULL) AS active
FROM sessions s
LEFT JOIN running r ON r.agent_id = s.agent_id
ORDER BY s.updated_at DESC
LIMIT ?1 OFFSET ?2;             -- §7.1 pagination
```

No transcript file is read on the listing path. `active` is computed authoritatively from the `running` row (not cached on the session row). With N sessions this is one indexed query, trivially fast for the expected N (§9).

### 4.3 Search (`q` present) — FTS5 `MATCH`

`GET /api/archive?q=foo bar`:

1. **Build the FTS query.** Whitespace tokens are combined with **AND** semantics (FTS5's default for space-separated terms is AND). A quoted `"exact phrase"` becomes an FTS5 phrase query (`"exact phrase"`). The raw `q` is sanitized (escape FTS5 syntax characters) and column filters are not exposed to the user — the `MATCH` runs across all indexed columns.
2. **Run the join + rank:**

   ```sql
   SELECT s.*, (r.agent_id IS NOT NULL) AS active,
          f.content_snippet AS snippet,
          f.rank AS score
   FROM (
     SELECT agent_id,
            snippet(sessions_fts, 6, '', '', '…', 12) AS content_snippet,  -- col 6 = content
            bm25(sessions_fts, 8.0,4.0,4.0,2.0,1.0,1.0,1.0) AS rank          -- weight metadata cols above content
     FROM sessions_fts
     WHERE sessions_fts MATCH ?1
   ) f
   JOIN sessions s  ON s.agent_id = f.agent_id
   LEFT JOIN running r ON r.agent_id = s.agent_id
   ORDER BY f.rank        -- bm25 returns lower = better
   LIMIT ?2 OFFSET ?3;
   ```

3. **`matched_in`** is computed from which columns the term hit (metadata columns vs `content`); the server inspects the match (or runs a cheap second `MATCH` restricted to metadata columns: `{name role project grp model backend} : q`) to label `["metadata"]`, `["transcript"]`, or both.
4. **`snippet`** comes from FTS5's `snippet()` over the `content` column, centered on the match (§7.1).

> Searchable text union = name, role, project (PRD-required) **plus** transcript content (PRD-required) plus model/backend/group (cheap bonus). All are indexed columns of `sessions_fts`. The PRD's required surface (name/role/project/transcript) is fully covered.

### 4.4 Ranking

Ranking is FTS5 `bm25()` with **column weights that float metadata matches above buried transcript mentions** (name/role/project weighted ~8×/4×, `content` 1×). Within similar relevance, the API tie-breaks by `updated_at` desc, then `agent_id` for stability (apply in the Go layer after the SQL `ORDER BY f.rank` when scores tie). Each result carries `matched_in` (`["metadata"]`, `["transcript"]`, or both) and a `snippet` (from the content match, else empty) so the UI can show *why* it matched.

### 4.5 Archive UI (minimal viable)

- Route `/archive` (added to the Phase 2 router). A search box (debounced 250ms → `GET /api/archive?q=`), and a result list: each row shows name, role · project, backend·model, created/updated, a state chip (active/inactive), and the snippet if present.
- Clicking a result:
  - **Inactive** → opens a **read-only** transcript view: reuse the Phase 2 `TranscriptView` + renderers, fed by `GET /api/sessions/{id}/transcript` (persisted-backed, reads the raw log, §7.4), with the composer disabled and a prominent **Resume** button.
  - **Active** → navigates to the live `/agent/:id` chat panel (it's already a running card).
- This reuses Phase 2's renderer registry and store; the only new UI is the archive list + search box + the read-only/Resume affordance.

### 4.6 Keeping the FTS content in sync (the indexer)

The FTS `content` for a session is the concatenation of the searchable text of its events. FTS5 updates a row by delete+insert, so the indexer avoids rewriting the row per delta:

- **Live sessions:** the indexer accumulates the turn's searchable text in memory and writes/replaces the session's `sessions_fts` row once per `turn_end` (`OnTurnEnd`), inside the same transaction that bumps the `sessions` counters. Mid-turn streamed deltas are searchable as of the turn boundary — acceptable, and the raw log is always authoritative if a finer-grained query is ever needed.
- **Backfill / reindex:** `agentdeck reindex` (and first-run migration for any pre-existing raw logs) replays each `transcript.ndjson` in one pass, building the full `content` and file/command rows, then writes each session's FTS row once. This is the recovery path that makes `state.db` rebuildable from the raw source (§2.4).

The searchable text extracted per event type: `assistant_text.delta`, `tool_call.name`+stringified `args`, `tool_result.content`, `diff.path`+`diff.new_text`, `permission_request.reason`, and the `session_meta` identity fields. Raw secrets are never extracted (the indexer reads normalized events, not raw frames; §8.7).

---

## 5. Resume design

The acceptance-critical, cross-phase deliverable. Resume must restore history + config, mint a fresh ephemeral `session_id` on the **same** stable `agent_id`, re-attach a chat runtime, register the in-process MCP server, and reappear as a live card — and Phase 6's switch-runtime reuses this exact path.

### 5.1 `POST /api/sessions/{id}/resume` flow

```
client/CLI → POST /api/sessions/{id}/resume   (optional body: { interface?, backend?, model? } — see §5.5)
      │
API layer:
  1. Load the agents row for {id} (identity) from state.db. 404 if absent.
  2. If a live handle already exists in the registry for {id} (running row present) → 409 conflict
     ("already running"; resume is for inactive sessions) — §8.2.
  3. Load the sessions row (composed-config snapshot) from state.db. If missing,
     rebuild it from the transcript's latest session_meta record. If both
     missing → 422 ("no persisted session to resume").
  4. Build a LaunchSpec from the snapshot (NOT from current roles/projects — §5.2, §12.4),
     applying any interface/backend/model override from the body (the seam Phase 6 uses; §5.5).
  5. Re-resolve secrets: env values from backends.json by env_keys (§8.7).
      │
      ▼
Registry → runtimeFor(spec.Agent.Interface) → Runtime.Resume(ctx, spec, lastSessionID)
      │
ChatRuntime.Resume (§5.3):
  6. Spawn the CLI (process group), ACP initialize handshake.
  7. session/load (resume the prior CLI session) IF the adapter supports it AND lastSessionID
     is still valid; else session/new (fresh CLI session) — either way a FRESH ephemeral
     sessionId is produced (§5.4). The raw transcript log is the source of truth for history
     either way (§12.4), so a failed CLI-side resume degrades gracefully to new + replayed context.
  8. Re-open the SAME transcript.ndjson in append mode (seq continues from last_seq+1, §5.4);
     append a new session_meta record with resumed_at + the new ephemeral sessionId.
  9. Upsert the running row (NEW session_id, new pid) + status row in state.db
     (restore last_context_pct from the sessions row; state="idle", detail="resumed").
 10. Register the Handle and the in-process MCP server (§5.7). Return.
      │
      ▼
State manager (Phase 2) sees the new running + status rows → emits state_update →
the agent reappears as a LIVE card with prior transcript intact (client fetches
GET /api/sessions/{id}/transcript → full persisted history from the raw log).
```

### 5.2 Restoring history + composed config

- **History:** lives entirely in `transcript.ndjson` (the raw source) and is reloaded by the client via the upgraded `GET /api/sessions/{id}/transcript` (§7.4) — the server does not need to re-stream it. The chat panel repaints the full prior transcript, then live `new_message` deltas from the resumed turn append on top.
- **Composed config:** taken from the `state.db` `sessions` row (the **frozen launch snapshot**), not recomputed from current `roles/`/`projects/`. This preserves the master-PRD invariant "a running agent's spec is frozen; edits affect future launches only" — and means a resume a week later behaves exactly as the original launch even if the role/project files have since changed. (Documented in §12.4.)

### 5.3 `ChatRuntime.Resume` (implements the Phase 1 stub)

Signature is the one fixed in Phase 1 §3.1: `Resume(ctx, spec LaunchSpec, sessionID string) (*Handle, error)`. It is `ChatRuntime.Start` minus identity-minting, plus history continuity:

| Step | `Start` (Phase 1) | `Resume` (Phase 4) |
|------|-------------------|--------------------|
| agent_id | minted by launch flow | **reused** (already exists; never changes) |
| transcript file | created (`session_meta` seq 0) | **re-opened in append mode**; new `session_meta` appended; `seq` continues |
| CLI session | `session/new` → new `sessionId` | `session/load(sessionID)` if supported & valid, else `session/new` → always a **fresh** `sessionId` |
| running row | written (pid, new session_id) | written (pid, **new** session_id — old one is dead) |
| status row | `idle`, context_pct 0 | `idle`, context_pct **restored** from `sessions.last_context_pct` |
| handle | registered | registered |

- **`session/load` vs `session/new`:** the ACP adapter *may* expose `session/load` to resume the CLI's own session state (so the model keeps its native context). We try it when `spec` carries a `lastSessionID` and the adapter advertised the capability in `initialize`; on any failure we fall back to `session/new`. **Either way, the raw `transcript.ndjson` is authoritative for the displayed history**, so a CLI that cannot natively resume still shows the user their full prior conversation; the model simply starts that turn without its prior native KV-context (acceptable, documented §12.4). A fresh `sessionId` is recorded in the `running` row regardless — the old ephemeral id is invalid after the prior process exited.
- **Re-attach is non-destructive:** nothing in `sessions/` is overwritten; the raw log only grows, and the `state.db` rows are upserted (not rebuilt). This is exactly what makes Phase 6 switch-runtime safe (stop old runtime → `Resume` with new interface/backend/model on the same `agent_id` and same transcript).

### 5.4 Stable `agent_id` + fresh ephemeral `session_id` + seq continuity

- `agent_id` is never touched on resume (loaded from the `agents` row).
- A **fresh** `session_id` is written to the `running` row on every Start *and* Resume (master PRD invariant). The old `session_id` is only of historical interest (recorded in the per-resume `session_meta` transcript records and in `sessions.last_session_id`).
- **`seq` continuity:** on `Open` of an existing transcript, the writer resumes the per-agent monotonic counter at `last_seq+1`. `last_seq` is read from the `sessions` row (kept current on every `turn_end`); as a fallback (e.g. mid-rebuild) the writer recovers max `seq` by scanning the raw log's tail. This keeps `seq` globally monotonic across resumes so Phase 2's gap detection still works across a resume boundary.

### 5.5 The Phase-6 seam (switch-runtime)

`POST .../resume` accepts an optional body `{ interface?, backend?, model? }`. When present, the launch flow overrides those fields in the rebuilt `LaunchSpec` before calling `Resume`. **Phase 4 ships this with no override (resume = resume as-was).** Phase 6's `POST /api/sessions/{id}/switch-runtime` is implemented as: `Stop` the current runtime → `Resume` with the override body. Thus Phase 6 adds *no new resume machinery* — it reuses this endpoint's internals. The override fields are validated this phase (unknown backend/model/interface → 422) but only the "no override" path is exercised by Phase 4 tests; the override path is covered by Phase 6.

### 5.6 CLI: launch of an existing identity resumes, not duplicates

Phase 1's CLI (`agentdeck <role>@<project>`) always created a new agent. Phase 4 changes the CLI launch resolution:

- The CLI computes a **launch key** = the user's intent. If the user passes an explicit existing identity (a new flag `--resume <agent_id>` or `agentdeck resume <agent_id>`), the CLI calls `POST /api/sessions/{id}/resume` instead of `POST /api/sessions`.
- For the bare `agentdeck <role>@<project>` form: if there is an **inactive** session with the same `role@project` (and same name if `--name` given), the CLI **prompts/defaults to resume the most recent matching inactive session** rather than spawning a duplicate. Default behavior: resume the single most-recent inactive match; if there are multiple and it's ambiguous, list them and require `--resume <agent_id>` (or `--new` to force a fresh launch). `--new` always forces `POST /api/sessions`.
- This satisfies "CLI launch of an existing identity resumes rather than duplicates" without surprising the user when they genuinely want a fresh agent (`--new`).

### 5.7 Registering the in-process MCP server on resume

A resumed agent must be reachable for agent-to-agent messaging exactly like a freshly launched one. Resume therefore registers the agent with the **in-process Go MCP server** (architecture-decisions D3: the messaging server is hosted inside the Go binary, no runtime Node), passing the same `LaunchSpec` MCP seam (`mcpServers`/`ExtraArgs`, Phase 1 §3.1) the launch path uses. Phase 4 wires the **registration** (so the resumed agent is listed and addressable); the messaging *tools* (`list_agents`/`send_message`/`check_messages`) are Phase 5. Because registration is in-process, it is a function call against the same `state.db` the rest of the server reads — no serialization boundary.

---

## 6. File & command tracking design (F10)

### 6.1 Source of truth: rows in `state.db`, derived from events

**Decision (§12.5): Files and Commands are captured into `state.db` rows (`tracked_files`, `tracked_commands`) by the indexer as events stream, and re-derivable from the raw transcript on demand.** Every `tool_call`/`tool_result`/`diff` event the runtime emits — and every `POST /api/hook` from a terminal-runtime producer — feeds the same tables. The endpoints (`/files`, `/commands`) are plain `state.db` queries. Because the rows are derived from the raw log, a reindex rebuilds them exactly (§2.4), so there is no drift-prone separate file to maintain.

### 6.2 Capturing edited files (from tool calls + diffs)

A "file edit" is detected from transcript events (chat runtime) or `POST /api/hook` payloads (terminal runtime, Phase 6 producer):

- A `tool_call` whose `name` is a known file-writing tool (`Edit`, `Write`, `MultiEdit`, `NotebookEdit`, `Create`, … — a configurable set, default per the Claude Code tool vocabulary) with a `path`/`file_path` in its `args`.
- A `diff` event (Phase 1 `DiffData`) — the strongest signal, since it carries `path` + the patch and is what enables diff linkage.

The indexer maintains a **per-file rollup** as a `tracked_files` row (upsert on `(agent_id, path)`), which the `/files` endpoint returns:

```jsonc
// one row of tracked_files, as returned by GET /api/sessions/{id}/files
{
  "path": "src/auth.ts",
  "edit_count": 3,                 // number of edit tool_calls / diffs touching this path
  "first_seq": 18, "last_seq": 142,
  "first_ts": "…", "last_ts": "…",
  "has_diff": true,                // a diff event exists → diff linkage available
  "diff_refs": [                   // pointers into the transcript for diff linkage (§6.5)
    { "seq": 19, "tool_call_id": "tc_3" },
    { "seq": 88, "tool_call_id": "tc_9" }
  ]
}
```

Paths are normalized to **cwd-relative** where possible (using the `sessions` row's `cwd`) for stable display, with the absolute path retained in `abs_path`; identical paths collapse into one row (the rollup, via the upsert).

### 6.3 Capturing run commands (from tool calls)

A "command" is detected from a `tool_call` whose `name` is a shell-running tool (`Bash`, `Shell`, `Run`, `Terminal`, … — configurable, default Claude Code vocabulary), reading the command string from `args.command` (fallbacks: `args.cmd`, `args.script`), or from a `POST /api/hook` command payload. Each occurrence is one `tracked_commands` row (commands are not deduped — running the same command twice is two meaningful events):

```jsonc
// one row of tracked_commands, as returned by GET /api/sessions/{id}/commands
{
  "command": "npm test -- --watch=false",
  "seq": 57, "ts": "…",
  "tool_call_id": "tc_7",
  "exit_status": "completed",      // from the correlated tool_result.status, if present
  "exit_error": ""                 // from tool_result.error, if failed
}
```

Correlation: the `command` comes from the `tool_call` (inserted with `exit_status: "in_progress"`); the correlated `tool_result` (matched by `tool_call_id`) updates the row's `exit_status`/`exit_error`.

### 6.4 Cheap rollup counts for the archive

On each `turn_end`, the indexer updates `sessions.files_touched` / `sessions.commands_run` from the distinct `tracked_files` paths and `tracked_commands` count for the agent (a cheap aggregate query, or maintained incrementally on the handle during the live session). The detailed lists are the `tracked_files`/`tracked_commands` rows themselves; the rollup counts exist only to make archive listing show "7 files / 4 commands" without a join per row.

### 6.5 Diff linkage

The `tracked_files` rows carry `diff_refs` = `(seq, tool_call_id)` pointers (stored as JSON, populated when a `diff` event touches that path). The UI's "view diff" action fetches the specific `diff` event(s) for that file. Implementation: the `/files` response includes the `diff_refs`; the client retrieves the diff payloads from the already-loaded transcript (the chat panel / read-only view has them, fetched from the raw log). Since the read-only transcript view already renders `diff` events through `DiffBlock` (Phase 2 §7.1), "Files → view diff" simply scrolls/links to the corresponding `diff` block in the transcript by `seq`. No diff is recomputed or duplicated; linkage is by reference into the single raw transcript.

### 6.6 Searchable + copyable (acceptance)

- **Files** and **Commands** tabs (added to the Phase 2 chat panel as two new tabs alongside the transcript) render the `state.db` rows with a per-tab filter box (client-side substring filter — the lists are small per agent).
- Every path / command row has a **copy** affordance (copy the path, or copy the full command string). The acceptance criterion "all five appear and are copyable" is met by the list + per-row copy button; "files link to diffs where available" by §6.5.

---

## 7. API contracts

Base: `http://127.0.0.1:4317/api` (port from `config.json`, default `4317`). All bodies JSON. All errors use the Phase 1 §7.7 error shape and `code` vocabulary (`validation`/422, `not_found`/404, `conflict`/409, `not_implemented`/501, `runtime_start_failed`/502, `internal`/500).

### 7.1 `GET /api/archive?q=...` — list / search sessions

Query params: `q` (optional search string; whitespace-AND, `"…"` for phrase), `limit` (default 50, max 200), `offset` (default 0), `active` (optional filter: `true`/`false`/absent=all).

Response `200`:

```jsonc
{
  "query": "null check",
  "total": 2,                       // total matches before limit/offset
  "limit": 50, "offset": 0,
  "results": [
    {
      "agent_id": "a_8f3c12",
      "name": "Atlas",
      "role": "implementer",
      "project": "my-app",
      "backend": "claude",
      "model": "sonnet-4-6",
      "interface": "chat",
      "group": "auth-migration",
      "created_at": "2026-06-22T10:00:00Z",
      "updated_at": "2026-06-22T11:32:08Z",
      "turn_count": 14,
      "files_touched": 7,
      "commands_run": 4,
      "active": false,                          // running row present for this agent_id
      "matched_in": ["transcript"],             // ["metadata"] | ["transcript"] | both; omitted when q empty
      "snippet": "…added a null check to parseUser() before…"   // present for transcript matches
    }
  ]
}
```

- No `q` → listing query (§4.2): `matched_in`/`snippet` omitted; sorted by `updated_at` desc.
- With `q` → FTS5 search (§4.3–4.4).
- Errors: `422` (e.g. `limit` out of range), `500`.

### 7.2 `POST /api/sessions/{id}/resume` — resume from archive

Request body (all optional; Phase 4 exercises the empty body):
```jsonc
{ "interface": "chat", "backend": "claude", "model": "opus-4-7" }   // overrides for Phase 6 switch-runtime (§5.5)
```

Response `200 OK` (same `{ agent, running, status }` shape as Phase 1 launch, with the **fresh** `session_id` and the preserved `agent_id`):
```jsonc
{
  "agent": { "agent_id": "a_8f3c12", "name": "Atlas", "role": "implementer", "project": "my-app",
             "backend": "claude", "model": "sonnet-4-6", "interface": "chat",
             "created_at": "2026-06-22T10:00:00Z", "group": "auth-migration" },
  "running": { "agent_id": "a_8f3c12", "pid": 51002, "session_id": "claude-sess-NEW",
               "interface": "chat", "started_at": "2026-06-23T09:15:00Z" },
  "status": { "agent_id": "a_8f3c12", "state": "idle", "detail": "resumed",
              "last_trace": "SessionStart", "busy_since": "", "context_pct": 0.42 },
  "resumed": true
}
```

- Errors:
  - `404` (`not_found`) — no `agents` row for `{id}`.
  - `409` (`conflict`) — agent already running (a live handle exists / `running` row present). Resume is for inactive sessions; to change a live agent use Phase 6 switch-runtime.
  - `422` (`validation`) — no persisted session to resume (no `sessions` row and no `session_meta` in the transcript), or an override field names an unknown backend/model/interface.
  - `501` (`not_implemented`) — override `interface: "terminal"` (terminal `Resume` is Phase 6) or override `backend` of a not-yet-implemented type.
  - `502` (`runtime_start_failed`) — CLI failed to spawn / handshake on resume.

### 7.3 `GET /api/sessions/{id}/files` — tracked files

Response `200`:
```jsonc
{
  "agent_id": "a_8f3c12",
  "files": [
    { "path": "src/auth.ts", "edit_count": 3, "first_seq": 18, "last_seq": 142,
      "first_ts": "…", "last_ts": "…", "has_diff": true,
      "diff_refs": [ { "seq": 19, "tool_call_id": "tc_3" }, { "seq": 88, "tool_call_id": "tc_9" } ] },
    { "path": "src/db.ts", "edit_count": 1, "first_seq": 60, "last_seq": 60,
      "first_ts": "…", "last_ts": "…", "has_diff": false, "diff_refs": [] }
  ]
}
```
Served from `tracked_files` (§6.2), sorted by `last_ts` desc (most recently touched first). `404` if no such agent (no `agents`/`sessions` row). Works for both active and archived sessions.

### 7.4 `GET /api/sessions/{id}/commands` — tracked commands

Response `200`:
```jsonc
{
  "agent_id": "a_8f3c12",
  "commands": [
    { "command": "npm test -- --watch=false", "seq": 57, "ts": "…", "tool_call_id": "tc_7",
      "exit_status": "completed", "exit_error": "" },
    { "command": "git status", "seq": 31, "ts": "…", "tool_call_id": "tc_5",
      "exit_status": "completed", "exit_error": "" }
  ]
}
```
Served from `tracked_commands` (§6.3), sorted by `seq` desc (most recent first). `404` if no such agent.

### 7.5 `GET /api/sessions/{id}/transcript` — upgraded (persisted-backed)

Replaces the Phase 2 in-memory version. Now reads `sessions/{id}/transcript.ndjson` (the raw source) and returns the full persisted event stream (usable for reconnect repaint and now also for the archive read-only view + resume repaint).

Query params: `since_seq` (optional; return only events with `seq > since_seq` — used for incremental reconnect catch-up), `limit` (optional cap; default unbounded for a single session at this scale).

Response `200`:
```jsonc
{ "agent_id": "a_8f3c12",
  "events": [ { "agent_id":"a_8f3c12","seq":1,"type":"assistant_text","ts":"…","data":{"delta":"…"} }, … ] }
// 404 if no such agent. If sessions/{id}/transcript.ndjson missing but agent exists → events: [].
```

- Skips the `session_meta`/`seq:0` records by default (the client doesn't render them) but includes `permission_resolved` (the client uses it to collapse gates). A `?include_meta=true` flag returns them for debugging.
- Partial trailing line (crash truncation) is silently dropped on read (§8.1).

### 7.6 Reused unchanged

`POST /api/sessions` (launch), `GET /api/sessions`, `GET /api/sessions/{id}`, `prompt`, `cancel`, `stop`, `rename`, `permission`, `POST /api/hook`, `GET /api/events`, `GET/PUT /api/layout`. No bus or SSE changes this phase. (`POST /api/hook` gains the file/command capture behavior in §6, writing the same `tracked_files`/`tracked_commands` tables.)

---

## 8. Edge cases & error handling

### 8.1 Corrupt / partial transcript

- **Partial trailing line** (crash mid-`Write` despite single-Write atomicity, e.g. truncated by `kill -9` between the kernel accepting part of the buffer — rare with `O_APPEND` single writes but handled): the reader uses `bufio.Scanner`; the final token without a trailing `\n` that fails `json.Unmarshal` is **dropped silently** (logged at debug). Every prior complete line is intact. This is the core crash-safety guarantee for the raw source.
- **A corrupt line in the middle** (should not happen with atomic appends, but defensive): on `json.Unmarshal` failure for a non-final line, log the (truncated) raw line and **skip it, continue scanning** — exactly the Phase 1 §8.3 resync behavior. One bad line never aborts a transcript replay or a reindex pass.
- **Missing `sessions` row** but present raw log: the indexer reconstructs the metadata row from the latest `session_meta` record (§3.2) on next access / reindex. Missing both → the session is unlistable beyond its `agent_id`; surface it in the archive with degraded fields and a `"degraded": true` hint rather than hiding it.
- **`state.db` lost or corrupt:** `agentdeck reindex` truncates the Phase 4 tables and replays the raw logs to rebuild metadata, FTS content, and file/command rows (§2.4). The raw logs are untouched, so no history is lost.
- **`seq` recovery on a corrupt tail:** if the last raw line is partial, the writer recovers max `seq` from the last *valid* line (or `sessions.last_seq`); the dropped partial line's `seq` is simply re-used by the next append (no harm — it was never delivered).

### 8.2 Resume of an already-running agent

- A live handle in the registry (or a `running` row present with an alive pid) → `POST .../resume` returns `409`. Resume targets inactive sessions only.
- **Stale `running` row with a dead pid** (server crashed leaving a ghost): Phase 1 §8.5 already reconciles stale `running` rows on server start (deletes them, sets status). So a resume after a crash sees no live handle and proceeds normally. If a resume request races startup reconciliation, the pid-liveness check is the tiebreaker; a dead pid → proceed, a live pid → `409`.
- **Double resume race** (two concurrent resume requests for the same id): the registry's `Start`/`Resume` path takes the registry lock and checks for an existing handle (Phase 1 §8.6 double-start guard); the second request gets `409`.

### 8.3 Very large transcripts

- **Reading:** the `/transcript` endpoint streams the raw file line-by-line; for a very large transcript the client can request `?since_seq=` to avoid refetching the whole thing on reconnect. (A future enhancement could paginate from the tail; not needed at expected scale — §9.)
- **Writing:** raw append is `O(1)` regardless of file size, so a long-running agent's writes never slow down. The FTS row update is per-`turn_end`, not per-event, so index writes stay bounded too.
- **Search:** FTS5 query cost is governed by the inverted index, not by transcript size; there is no per-query file scan to bound. A reindex of a pathologically large transcript is a one-time `O(n)` pass.
- **Memory:** no operation loads a whole transcript into memory except the `/transcript` endpoint's JSON response (bounded by `limit`) and the indexer's per-turn content accumulation (bounded by one turn); files/commands derivation is a streaming line scan during reindex.

### 8.4 Search performance

- **Listing** is one indexed `state.db` query joined to `running` — fast and constant-ish.
- **Search** is an FTS5 `MATCH` ranked by `bm25()`, joined back to `sessions` — index-time cost, comfortably interactive at the expected scale (§9). There is no full-file scan to cap.
- **Index freshness:** the FTS content is current as of the last `turn_end` (§4.6); a query during an in-flight turn won't match that turn's not-yet-flushed deltas, which is acceptable. The raw log remains authoritative if an exact-as-of-now query is ever needed.

### 8.5 Concurrent write during resume / read during write

- A session being **read** (transcript replay) while it is **live and appending** to the raw log: `O_APPEND` writes are atomic; the reader either sees a complete final line or a partial one it drops (§8.1). No locking needed between reader and writer — the append log is naturally read-concurrent.
- **`state.db` concurrency:** WAL mode (§2.5) lets the archive/files/commands read queries run concurrently with the indexer's writes; the server is the sole writer, so there is no multi-writer contention.
- **Resume re-opening the transcript:** the prior runtime is stopped (process dead, `Stop` called `Sync`+`Close`) before resume opens the file for append, so there is at most one writer per raw log at a time.

### 8.6 Stop semantics (keeps `sessions/` + `sessions` row, removes `running` row)

Restated for completeness (the F9 requirement): `POST /api/sessions/{id}/stop` (Phase 1 §7.5) is amended so that before deleting the `running` row it calls `writer.Sync()` + `writer.Close()` and flushes any pending FTS content. `sessions/{id}/` and the `sessions`/`tracked_*` rows are **never** deleted by stop. The session immediately becomes an inactive archive entry, fully searchable and resumable. The `status` row is left at `done` (Phase 1 convention) so the archive can show a final state and the Phase 2 card shows "stopped".

### 8.7 Secrets

The `sessions` row and the `session_meta` record store env **key names only** (`env_keys`), never values. On resume, env values are re-resolved from `backends.json` via `composeEnv` (Phase 1 §6.2) using the snapshot's backend/model. This prevents API keys leaking into the on-disk transcript, the archive, or the FTS content (all user-readable). The indexer extracts searchable text from **normalized** events, not raw frames, so it never indexes a raw env echo. The raw-retention debug flag (§12.2) writes raw ACP frames to a sibling `raw.ndjson` and is documented **debug-only, do not enable with real credentials in the transcript** — nothing in the product reads it.

---

## 9. Scale assumptions (sizing the decisions)

The decisions in §2 and §8 are sized for the realistic local-tool scale, stated so a reviewer can check the math:

- **Sessions:** tens to a few hundred over the tool's lifetime on one machine (one developer). Listing N rows from an indexed `state.db` table is trivial at N≈hundreds.
- **Transcript size:** a typical coding session is hundreds to low-thousands of events; even verbose sessions are single-digit MB. A pathological multi-hour session might reach tens of MB — fine for `O(1)` appends and a one-time reindex pass.
- **Search latency budget:** interactive (< ~300ms typical), met by an FTS5 inverted index regardless of total transcript bytes.
- **`state.db` size:** the FTS content roughly mirrors transcript text volume; SQLite handles single-digit-GB databases comfortably, well above the expected total. A `VACUUM` / `agentdeck reindex` reclaims space if many sessions are deleted.

---

## Subphase plan (incremental / quota-limited implementation)

The invariant for every subphase below: it ends at a GREEN checkpoint — `go build ./...` passes (with the `sqlite_fts5` build tag once §2.3 lands) and all existing tests pass — so work is never half-done and a fresh agent can resume cold at the next subphase without inheriting an in-progress change.

### Subphase 4.1 — Raw NDJSON transcript writer + reader
- **Goal:** Stand up the durable `transcript.ndjson` append log and its replay reader in isolation, before any DB or runtime wiring.
- **Deliverables:** `internal/transcript` package — `Writer` (`Open`/`Append`/`Sync`/`Close`, `O_APPEND` single-Write, fsync on `turn_end`/`error`), `session_meta` first record + `seq` recovery on reopen (§2.1, §3.2, §3.5), and `Reader`/replay (stream → `[]Event`, `since_seq` filter, skip `seq:0` by default, 8 MiB scanner cap §2.6). Adds the `permission_resolved` event type as an additive `Event` type only (§3.2, §12.3). Maps to tech-spec tasks 1, 2, and the type half of 3.
- **Depends on:** Phase 1 `runtime.Event` type + 8 MiB framing convention. No prior subphase.
- **Done when (checkpoint):** `go test ./internal/transcript/...` passes covering append→read round-trip, reopen-continues-seq, partial-trailing-line dropped, mid-file bad line skipped, and a > 64 KiB line round-trips (§11.1, §11.5). `go build ./...` passes; no runtime hot-path code touched yet, so all existing tests still pass.
- **Resume note:** Starts from Phase 1 with `Runtime.Resume` still stubbed and no persistence. Begin by creating `internal/transcript/writer.go` + `reader.go` against fixture transcripts; do not wire into the runtime here.
- **Size:** M

### Subphase 4.2 — `state.db` Phase-4 migration + indexer + reindex
- **Goal:** Add the Phase-4 schema and the indexer/backfill that upsert `state.db` from the raw logs, still without touching the runtime hot path.
- **Deliverables:** Phase-4 migration creating `sessions`, `sessions_fts` (FTS5), `tracked_files`, `tracked_commands`, opened in WAL with `synchronous=NORMAL` (§2.3, §2.5), built with the `sqlite_fts5` tag (`mattn/go-sqlite3`). `internal/index.Indexer` — `UpsertSessionMeta`, `OnEvent`, `OnTurnEnd` (§3.5, §4.6, §6). `agentdeck reindex` + first-run migration that truncates Phase-4 tables and replays every raw log (§2.4, §4.6). Maps to tech-spec tasks 4, 5, 6.
- **Depends on:** Subphase 4.1 (reader/replay + fixture transcripts feed the indexer and reindex).
- **Done when (checkpoint):** `go test ./internal/index/...` (built with `-tags sqlite_fts5`) passes: an FTS5 `MATCH` query returns the seeded session row from indexed fixture events; `OnTurnEnd` advances `sessions` counts/`updated_at`/`last_seq`; `tracked_files`/`tracked_commands` rows match fixtures; drop-DB → `reindex` → identical query results. `go build -tags sqlite_fts5 ./...` passes; existing tests still pass.
- **Resume note:** Starts with `internal/transcript` complete and tested; `state.db` has only the Phase 1/2 tables. Begin with the migration, then the indexer reading normalized events from the 4.1 reader, then the reindex command.
- **Size:** M

### Subphase 4.3 — Wire writer + indexer into the runtime; persisted-backed transcript endpoint
- **Goal:** Make live chat turns durable: feed every event to the raw log and the indexer in lockstep, and serve history from disk.
- **Deliverables:** `ChatRuntime.Start` opens the writer (`session_meta`) + upserts the `sessions` row; `dispatch` does **append → publish → index upsert**; `turn_end`/`error` → `Sync` + `OnTurnEnd` (§3.5). `ChatRuntime.Stop` `Sync`+`Close`+flush before removing the `running` row, leaving `sessions/` + rows intact (§3.5, §8.6). Emit + persist `permission_resolved` on resolve (runtime half of task 3). Upgrade `GET /api/sessions/{id}/transcript` to read the persisted NDJSON with `since_seq`/`include_meta` (§7.5). Maps to tech-spec tasks 7, 8, 15, runtime half of 3.
- **Depends on:** Subphases 4.1 + 4.2 (writer + indexer must exist and pass).
- **Done when (checkpoint):** crash-mid-turn integration test (reuse Phase 1 `crash_midturn` fake-ACP) asserts persisted ⊇ delivered events (§11.1); `GET /api/sessions/{id}/transcript` returns the full persisted stream for a stopped agent (§11.1, §11.5). `go build -tags sqlite_fts5 ./...` and the full existing suite pass.
- **Resume note:** Starts with `transcript` + `index` packages done but not called from the runtime; the transcript endpoint is still the Phase 2 in-memory stopgap. Begin in `ChatRuntime.dispatch`/`Start`/`Stop`, then swap the endpoint impl.
- **Size:** M

### Subphase 4.4 — Archive list + FTS5 search API
- **Goal:** Expose the durable, searchable archive over `state.db`.
- **Deliverables:** `internal/archive.Archive` — listing (metadata join to `running` for `active`, sort, paginate §4.2) and search (sanitized FTS query, `MATCH` join, `bm25()` weights, `snippet`, `matched_in` §4.3–4.4). `GET /api/archive?q=...&limit&offset&active` handler (§7.1). Maps to tech-spec tasks 12, 13.
- **Depends on:** Subphases 4.2 + 4.3 (the indexer populates the tables that this queries; live sessions must be indexing for active/inactive listing).
- **Done when (checkpoint):** archive tests pass — a session is findable by a distinctive transcript-only phrase with `matched_in:["transcript"]` + snippet; metadata hit yields `["metadata"]`; whitespace-AND semantics; active+inactive both listed with correct `active`; pagination; negative query → `total:0`; reindex-equivalence (§11.2). `go build -tags sqlite_fts5 ./...` and existing tests pass.
- **Resume note:** Starts with live turns persisting + indexing and the transcript endpoint reading from disk. Begin with `internal/archive` queries, then the HTTP handler.
- **Size:** M

### Subphase 4.5 — Resume (`ChatRuntime.Resume` + endpoint + CLI)
- **Goal:** Restore an inactive session to a live card on the same `agent_id` with a fresh `session_id` and intact history.
- **Deliverables:** real `ChatRuntime.Resume` (spawn + handshake; `session/load` with fallback to `session/new`; reopen transcript in append mode + new `session_meta`(resumed_at); upsert `running` row with fresh `session_id` + status row with restored `last_context_pct`; register in-process MCP server) replacing the Phase 1 stub (§5.3, §5.4, §5.7). `POST /api/sessions/{id}/resume` handler with identity/already-running/snapshot checks and the optional `{interface,backend,model}` override seam validated but only the empty-body path exercised (§5.1, §5.5, §7.2). CLI resume-not-duplicate (`--resume`/`resume <id>`/bare-form most-recent-inactive-match/`--new`) (§5.6). Maps to tech-spec tasks 9, 10, 11.
- **Depends on:** Subphase 4.3 (persisted transcript + `sessions` snapshot) and 4.4 (archive surface to resume from); the in-process MCP-server *registration* hook (messaging tools remain Phase 5).
- **Done when (checkpoint):** resume tests pass — `200` with unchanged `agent_id`, a `running.session_id` that differs from pre-stop, a reappearing card via `state_update`, full prior transcript plus the new resumed `session_meta`, a subsequent prompt continuing `seq` monotonically; `409` already-running; `422` no persisted session; CLI bare-form resume vs `--new` (§11.3). `go build -tags sqlite_fts5 ./...` and existing tests pass.
- **Resume note:** Starts with archive/search live and `Resume` still stubbed. Begin in `ChatRuntime.Resume` (mirror `Start` minus identity-minting), then the endpoint, then CLI resolution.
- **Size:** M

### Subphase 4.6 — File/command endpoints, hook capture, and UI
- **Goal:** Surface the tracked Files/Commands and ship the minimal archive + read-only/resume UI.
- **Deliverables:** `GET /api/sessions/{id}/files` + `GET /api/sessions/{id}/commands` over `tracked_files`/`tracked_commands` with configurable tool-name sets; `POST /api/hook` file/command capture into the same tables (§6, §7.3–7.4). Frontend: `/archive` route (search box + result list + snippet + state chip), read-only transcript view (reuse `TranscriptView`, disabled composer, Resume button → `POST .../resume`), and **Files**/**Commands** tabs (lists, per-row copy, filter, diff link → scroll to `diff` block by `seq` §6.5). Final wiring + manual verification against a real `claude-code-acp` (§11.4 + task 17). Maps to tech-spec tasks 14, 16, 17.
- **Depends on:** Subphase 4.5 (the read-only view's Resume button calls the resume endpoint; archive route lists what 4.4 serves).
- **Done when (checkpoint):** files/commands tests pass — 3 edits roll into 2 `tracked_files` rows with correct `has_diff`/`diff_refs`; 2 `tracked_commands` rows with `exit_status` from correlated results; `POST /api/hook` command inserts a queryable row (§11.4); frontend Vitest/RTL: tabs render, copy works, filter narrows, diff link targets the right `seq`. `go build -tags sqlite_fts5 ./...`, the full Go suite, and the UI test suite pass.
- **Resume note:** Starts with all server-side persistence/archive/resume complete and tested; only file/command endpoints, hook capture, and the frontend remain. Begin with the two Go endpoints + hook capture, then the React `/archive` route and tabs.
- **Size:** M

## 10. Implementation task breakdown (ordered)

Each step is small and independently testable. The transcript writer/reader and a fixture transcript come first so everything downstream is TDD'd against deterministic data.

**Persistence core:**
1. `internal/transcript`: `Event` reuse (Phase 1 type), `Writer` (`Open`/`Append`/`Sync`/`Close`, `O_APPEND`, single-Write, fsync-on-turn_end), `session_meta` first record, `seq` recovery on reopen. Unit tests: append→read round-trip; reopen continues seq; **partial-trailing-line dropped**; **mid-file bad line skipped**; a > 64 KiB line round-trips (buffer cap).
2. `transcript.Reader` + replay: stream lines → `[]Event`; `since_seq` filter; skip `seq:0` meta by default. Tests against a fixture transcript including a large diff line.
3. `permission_resolved` event type (additive): runtime emits it on resolve (approve/deny/timeout/auto); persisted to the raw log + published. Update Phase 1 permission paths to append it.

**`state.db` schema + indexer:**
4. Phase 4 `state.db` migration: create `sessions`, `sessions_fts` (FTS5), `tracked_files`, `tracked_commands` (§2.3); open DB in WAL. Build with the `sqlite_fts5` tag.
5. `internal/index`: `Indexer` — `UpsertSessionMeta`, `OnEvent` (accumulate FTS content; insert/update file/command rows), `OnTurnEnd` (flush FTS row; update `sessions` counts/`updated_at`/`last_seq`/`last_context_pct`). Transactional; sole-writer. Unit tests against fixture events.
6. `agentdeck reindex` (and first-run migration): truncate Phase 4 tables, replay every raw log, rebuild metadata + FTS content + file/command rows. Test: drop DB → reindex → identical query results.

**Wire into the runtime:**
7. `ChatRuntime.Start`: open the transcript writer (write `session_meta`), upsert the `sessions` row. `dispatch`: **append to raw log → publish → index upsert**. `turn_end`/`error`: `Sync` + `OnTurnEnd`.
8. `ChatRuntime.Stop`: `Sync`+`Close` writer, flush FTS content, before deleting the `running` row; leave `sessions/` + `sessions`/`tracked_*` rows intact.

**Resume:**
9. `ChatRuntime.Resume` (real): spawn + handshake; `session/load` with fallback to `session/new`; reopen transcript in append mode + new `session_meta`(resumed_at); upsert running row (fresh session_id) + status row (restored context_pct); register the in-process MCP server. Registry wires it (remove the Phase 1 stub).
10. `POST /api/sessions/{id}/resume` handler: identity/already-running/snapshot checks; build `LaunchSpec` from snapshot (+optional override seam); call `Resume`; return `{agent,running,status,resumed}`. Errors per §7.2.
11. CLI resume-not-duplicate: `--resume <id>` / `agentdeck resume <id>`; bare-form most-recent-inactive-match → resume, `--new` to force fresh, ambiguity → list + require `--resume`.

**Archive + search:**
12. `internal/archive`: listing (metadata join to `running`, sort, paginate); search (build FTS query, `MATCH` join, `bm25` rank + weights, snippet, `matched_in`, result cap).
13. `GET /api/archive?q=` handler (§7.1). Tests: list active+inactive; findable-by-phrase (FTS content hit); metadata hit; AND semantics; pagination; snippet.

**Files / commands:**
14. `GET /api/sessions/{id}/files` + `GET /api/sessions/{id}/commands` handlers (§7.3–7.4), reading `tracked_files`/`tracked_commands`. Configurable tool-name sets feeding the indexer. `POST /api/hook` file/command capture into the same tables.

**Endpoint upgrade + UI:**
15. Upgrade `GET /api/sessions/{id}/transcript` to read the persisted raw NDJSON (replace the Phase 2 in-memory impl); `since_seq`/`include_meta`.
16. Frontend: `/archive` route (search box + result list + snippet + state chip); read-only transcript view (reuse `TranscriptView`, disable composer, show Resume button) → `POST .../resume`; **Files** and **Commands** tabs in the chat panel (lists, per-row copy, filter, diff link → scroll to `diff` block by seq).
17. Wiring + manual verification against a real `claude-code-acp`: run a turn that edits files + runs commands, stop, confirm archive lists it, search finds it by a transcript phrase, resume restores history and continues, files/commands appear and copy.

---

## 11. Testing strategy

### 11.1 Crash-mid-turn → no data loss (acceptance-critical)

- **Unit (writer):** append K records, then *do not* call Sync/Close; reopen the file and read — all K records present (page cache → kernel; a `kill` is a process crash, not power loss). Then write a deliberately **partial** final line (truncate the file mid-record), reopen, assert the partial line is dropped and the K prior records are intact.
- **Integration (fake CLI):** reuse Phase 1's `crash_midturn` fake-ACP scenario (emits a chunk then `os.Exit(1)`). Assert: after the crash, `transcript.ndjson` contains every event that was published before the crash (compare against the events the SSE subscriber received). This is the literal "killing the server mid-turn does not lose already-streamed content" check — and "append to the raw log before publish" (§3.5) guarantees persisted ⊇ delivered.
- **Integration (server kill):** drive a real or fake turn, `kill -9` the server process mid-turn, restart, `GET /transcript` → assert all pre-kill events present. Then `agentdeck reindex` → assert the rebuilt `state.db` matches (the raw log is the source).

### 11.2 Findable-by-phrase (acceptance-critical)

- Persist a transcript containing a distinctive phrase only in an `assistant_text` delta (not in metadata), let the indexer flush on `turn_end`. `GET /api/archive?q="distinctive phrase"` → the session is in `results` with `matched_in:["transcript"]` and a `snippet` containing the phrase.
- Negative: a phrase present in *no* session → `total:0`.
- Metadata hit: `q` = the role/project name → match with `matched_in:["metadata"]`.
- AND semantics: two tokens each present but in different sessions → neither matches; both in one session → that one matches.
- Active + inactive both listed (no `q`): start one agent (active), stop another (inactive) → both appear; `active` flags correct (join to `running`).
- Reindex equivalence: drop `state.db`, run `agentdeck reindex`, re-run the above → identical results.

### 11.3 Resume-with-history-intact (acceptance-critical)

- Launch (fake CLI), run a turn producing several events, `Stop`. Assert no live handle, `sessions/` + `sessions` row intact, `running` row gone.
- `POST .../resume` → assert: `200`; `agent_id` unchanged; `running.session_id` **differs** from the pre-stop one; a card reappears (state manager emits `state_update`); `GET /transcript` returns the **full prior** events plus the new `session_meta`(resumed_at); a subsequent prompt appends with `seq` continuing monotonically past the pre-resume max; the agent is registered with the in-process MCP server.
- `409` on resume of an already-running agent.
- `422` on resume with no persisted session.
- CLI: bare form with one inactive match resumes (same `agent_id`); `--new` forces a fresh `agent_id`; ambiguous match → error listing candidates.

### 11.4 Files/commands appear & copyable (acceptance-critical)

- Persist a transcript with 3 file-edit tool_calls/diffs (one path edited twice → `tracked_files.edit_count:2`, 2 distinct paths) and 2 Bash tool_calls. `GET /files` → 2 rows (3 edits rolled into 2 paths) with correct counts and `has_diff`/`diff_refs`; `GET /commands` → 2 rows with `command` strings and `exit_status` from correlated results.
- Diff linkage: a file with a `diff` event has non-empty `diff_refs` pointing at the right `seq`/`tool_call_id`; a file edited via a non-diff tool has `has_diff:false`.
- `POST /api/hook` capture: a hook command payload inserts a `tracked_commands` row queryable via `/commands`.
- Frontend (Vitest/RTL): Files/Commands tabs render the lists; copy button copies the path/command; filter box narrows the list; "view diff" link targets the right transcript `seq`.

### 11.5 Other

- **Reader robustness:** fixture transcripts with (a) a > 64 KiB line, (b) a mid-file malformed line, (c) a partial trailing line — all replay (and reindex) correctly per §8.1.
- **`sessions` row updates:** counts/`updated_at`/`last_seq` advance on `turn_end`; rename updates `name` (in the row and the FTS metadata) without touching the raw log.
- **Reindex rebuild:** delete `state.db`, reindex, assert metadata/FTS/file/command rows match a freshly-indexed run.
- **Concurrency / `-race` + WAL:** concurrent archive/search reads while a session is live-appending (raw log) and indexing (DB writer) — no race, reader drops only any partial tail, DB reads see a consistent snapshot.
- **Secrets:** assert no env *value* ever appears in the raw transcript, the `sessions` row, or the FTS content (only `env_keys`).

---

## 12. Resolved decisions (phase §6 / master PRD §9 open questions)

### 12.1 Search index strategy — SQLite FTS5 in `state.db`

Full-text search is a **SQLite FTS5** virtual table in `state.db` (`mattn/go-sqlite3`, `sqlite_fts5` build tag), populated by the indexer from the raw `sessions/` logs and ranked with `bm25()` (metadata columns weighted above transcript content). Listing and search are both `state.db` queries; no transcript files are read on the archive path. `state.db` is a rebuildable index over the raw logs (`agentdeck reindex`), so the only durable artifact is the append log. This follows the storage split in `docs/architecture-decisions.md` (config in files, machine state in SQLite, server is sole writer).

### 12.2 Transcript format — NDJSON append source; normalized retained; raw behind a flag

- **Format:** the durable source is the **NDJSON append log** (`transcript.ndjson`), one normalized `Event` per line (§2.1). Justified vs single-JSON (crash-safety, O(1) append, streaming reads). The queryable index lives in `state.db`, not in the log.
- **Normalized vs raw:** **retain normalized events only** in `transcript.ndjson` — they are the contract every consumer (chat panel, archive, files/commands, resume) needs, and they are backend-agnostic (Phase 1 §4.2). **Raw ACP frames are retained only behind `AGENTDECK_RAW_TRANSCRIPT=1`**, written to a sibling `sessions/{id}/raw.ndjson` (one raw stdio frame per line) for protocol debugging. The raw file is **debug-only**: it may contain secrets/credentials in unredacted tool args/env echoes, so it is off by default and documented "do not enable with real credentials" (§8.7). Nothing in the product reads `raw.ndjson` and it is never indexed into `state.db`; it exists purely for diagnosing ACP-mapping bugs.

### 12.3 Permission outcomes — dedicated `permission_resolved` record

A new additive transcript event `type: "permission_resolved"` (`{tool_call_id, decision: approve|deny|timeout|auto_approve}`) is appended to the raw log when a gate is resolved, so archived/replayed transcripts show the outcome rather than a dangling open request. It is emitted on the bus too (Phase 2 client collapses the gate to a resolved chip event-driven). Forward-compatible: older clients ignore unknown types.

### 12.4 Resume uses the frozen launch snapshot; the raw transcript is authoritative for history

- Resume rebuilds `LaunchSpec` from the `state.db` `sessions` row's **frozen composed-config snapshot**, *not* from current `roles/`/`projects/` files — preserving the master-PRD invariant "a running agent's spec is frozen; edits affect future launches only." A resume any time later reproduces the original composition.
- **The raw `transcript.ndjson` is the source of truth for displayed history.** The CLI's native `session/load` is attempted (to preserve the model's native context) but is best-effort: on failure we fall back to `session/new`, and the user still sees their full prior conversation (replayed from the raw log). This decouples resume correctness from the adapter's session-persistence support.

### 12.5 Files/commands — rows in `state.db`, derived from the raw transcript

Files and Commands are captured into `state.db` rows (`tracked_files`, `tracked_commands`) by the indexer from chat-runtime tool calls and `POST /api/hook`, and are **re-derivable** from the raw transcript on reindex (§2.4) — so there is no separate hand-maintained `files.json`/`commands.json` to drift. The rollup *counts* are cached on the `sessions` row for archive listing. The tool-name sets that identify "file edit" vs "run command" tools are configurable, defaulting to the Claude Code vocabulary (Edit/Write/MultiEdit/… and Bash/Shell/…).

### 12.6 Durability tier — fsync on turn boundaries; WAL for `state.db`

`fsync` the raw log on `turn_end`/`error`/`Stop`/`Resume`/shutdown, not per event; `state.db` runs in WAL with `synchronous=NORMAL`. Both guarantee no loss on a process **crash** (`kill`) — the acceptance bar — while not stalling streaming with dozens of syncs per turn. Power-loss durability of the last few in-flight deltas / the last DB transaction is explicitly out of scope for a local dev tool. The raw log's fsync cadence is tunable to per-event via `AGENTDECK_FSYNC=always` if a user wants it.

---

## Appendix A — Acceptance checklist mapping

| PRD acceptance (phase §5) | Covered by |
|---------------------------|-----------|
| Stopped agent appears in archive with name/role/project/timestamps | §3.3 `sessions` row, §4.2 listing query, §7.1; §11.2 |
| Findable by a distinctive transcript phrase via `?q=` | §2.2/§4.3 FTS5 `MATCH` + snippet, §7.1; §11.2 |
| Resume restores full transcript + config, re-attaches runtime continuing the same logical session | §5 (Resume flow + `ChatRuntime.Resume`), §7.2; §11.3 |
| After 3 file edits + 2 commands, all 5 appear in Files/Commands tabs and are copyable | §6 `tracked_files`/`tracked_commands`, §7.3–7.4, §6.6; §11.4 |
| Edited files link to diffs where a diff exists | §6.5 `diff_refs`; §11.4 |
| Killing the server mid-turn does not lose already-streamed content | §2.1 raw-log crash-safety + §3.5 append-before-publish; §11.1 |

## Appendix B — Interfaces produced for later phases

1. **NDJSON transcript format + `transcript.Writer`/`Reader`** (§3.2, §3.5). **Phase 6 switch-runtime** reads/continues the *same* `transcript.ndjson` across an interface/backend/model switch (the log only grows; nothing is rewritten), so a switch is non-destructive by construction. The `permission_resolved` and `session_meta` record types are part of this frozen format.
2. **`state.db` Phase 4 schema + indexer** (§2.3, §3.5): the `sessions`/`sessions_fts`/`tracked_files`/`tracked_commands` tables and the `Indexer` that keeps them in sync from the raw logs, all rebuildable via `agentdeck reindex`. Later phases query these tables directly.
3. **Resume machinery** (§5): `ChatRuntime.Resume` + `POST /api/sessions/{id}/resume` with the optional `{interface,backend,model}` override body, plus in-process MCP-server registration. **Phase 6** implements `POST .../switch-runtime` as `Stop` → `Resume(override)` — reusing this endpoint's internals with **no new resume code**. The frozen-snapshot rule (§12.4) means a switch re-composes only the overridden fields.
4. **Frozen composed-config snapshot** (§3.3, the `sessions` row): Phase 6's switch overrides `backend`/`model`/`interface` against this snapshot; Phase 5's resume hook adds `mcpServers` via the same `LaunchSpec` seam (`ExtraArgs`/`mcpServers`, Phase 1 §3.1) without changing the snapshot shape.
5. **Persisted-backed `GET /api/sessions/{id}/transcript`** (§7.5): every later phase that needs history (archive view, resume repaint, switch-runtime repaint) reads it; it replaces the Phase 2 in-memory stopgap.
6. **Archive/search API** (§7.1): a stable contract backed by `state.db` FTS5; the implementation can evolve (e.g. richer ranking) without changing callers.
