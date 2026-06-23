# Phase 4 — Implementation Tech Spec: Persistence — transcript archive, full-text search, resume, file/command tracking

**Mirrors:** `docs/phases/phase-4-persistence-archive.md` (phase PRD)
**Master PRD:** `agent-dashboard-prd.md` (source of truth — F9, F10, §3.5 `sessions/` layout, §4.1 `Runtime.Resume`, §6 REST surface)
**Builds on:** Phase 1 (Runtime interface, normalized transcript `Event`s, status/running files, `LaunchSpec` + composition), Phase 2 (state manager, SSE bus, `state_update`/`new_message`, `GET /api/sessions/{id}/transcript` minimal in-memory endpoint)
**Status:** ready to implement after Phase 2
**Audience:** the engineer implementing Phase 4. This document is complete enough to implement with essentially no further design decisions. Where the PRD leaves something open (phase §6, master PRD §9), this spec pins a concrete decision — see §12.

---

## 1. Overview & scope recap

### 1.1 What this phase delivers

Phase 1/2 produced a transcript that **streams** (`new_message` on the SSE bus) but lives only in memory — kill the server and the conversation is gone; a stopped agent still shows as a card (Phase 2 §3.5) but has no recoverable history. Phase 4 makes work **durable and recoverable**:

1. **Transcript persistence (F9):** every normalized transcript `Event` (Phase 1 §4.2) is appended to `~/.agentdeck/sessions/{agent_id}/transcript.ndjson` as it streams, crash-safely (survive a kill mid-turn with no loss of already-streamed content). A small `session.json` per session records the composed config needed to reconstruct the chat panel and to resume.
2. **Archive + search (F9):** `GET /api/archive?q=...` lists **all** sessions — active and inactive — with name/role/project/timestamps, and full-text-searches over name, role, project, and transcript content.
3. **Resume (F9):** `POST /api/sessions/{id}/resume` restores history + composed config, implements the **chat-runtime `Runtime.Resume`** (stubbed in Phase 1), preserves the stable `agent_id` while writing a fresh ephemeral `session_id`, re-attaches a runtime, and reappears as a live card. The CLI launch of an existing identity resumes instead of duplicating.
4. **File & command tracking (F10):** per-agent **Files** and **Commands** views derived from the persisted transcript (tool calls), searchable and copyable, with diff linkage where a `diff` event exists.

This phase also delivers the **non-destructive resume machinery** that Phase 6 (switch interface/backend/model on a live agent, F7) reuses verbatim.

### 1.2 In scope

- A `transcript` package (Go): a per-agent NDJSON append writer that the chat runtime feeds in parallel with the SSE bus; crash-safe append; a reader that replays a transcript into `Event`s.
- `session.json` per session: composed config snapshot + denormalized index fields (name/role/project/timestamps) for cheap archive listing.
- Archive listing + full-text search: `GET /api/archive?q=...` with a **scan-on-query** strategy plus a maintained lightweight per-session **summary index** for the fast-path (listing without `q`), and a documented growth path to a real inverted index.
- Resume: real `ChatRuntime.Resume`; `POST /api/sessions/{id}/resume`; CLI resume-not-duplicate; upgrade of Phase 2's `GET /api/sessions/{id}/transcript` to read **persisted** history (live or archived).
- File/command derivation: `GET /api/sessions/{id}/files`, `GET /api/sessions/{id}/commands`, derived from transcript events (no separate hand-maintained index file — see §12.5), with diff linkage.

### 1.3 Out of scope (and how it slots in later)

| Item | Status this phase | Lands in |
|------|-------------------|----------|
| Switch interface/backend/model on a live agent (F7) | Out — but its resume machinery is built here and reused | Phase 6 |
| Terminal runtime `Resume` | Out — `TerminalRuntime` is still a `501` stub; only `ChatRuntime.Resume` is real | Phase 6 |
| MCP messaging server registration on resume | Out — resume leaves a documented hook (`LaunchSpec.ExtraArgs`/`mcpServers`); not wired | Phase 5 |
| Messaging, nudger, notifications | Out | Phase 5 |
| Hooks-driven file/command capture for **chat** agents | Out — chat agents derive files/commands from the ACP transcript (master PRD §4.4: chat agents may skip redundant hook writes); hook path is the terminal-runtime mechanism | Phase 6 |
| Archive UI polish (read-only transcript viewer styling, infinite scroll) | Minimal viable list + search box + open-result; reuses the Phase 2 `TranscriptView` renderers in read-only mode | this phase (minimal), polish later |

### 1.4 Where this plugs into existing code

- The **chat runtime** (Phase 1 §4) already emits normalized `Event`s to the bus. Phase 4 adds a second sink: a `transcript.Writer` per agent that the runtime also feeds. This is the only change to runtime hot-path code.
- The **state manager / SSE bus** (Phase 2) are unchanged: a resumed agent writes `running/`+`status/` exactly like a fresh launch, so it reappears as a card through the normal `state_update` path. No bus changes.
- Phase 2's in-memory `GET /api/sessions/{id}/transcript` is **upgraded** to read the persisted NDJSON, removing the "retains nothing → empty array" caveat.

---

## 2. Technology choices

All server-side; Go 1.22+, single binary (Phase 0 constraint). Continue the project rule: prefer the standard library; add a dependency only where the stdlib is genuinely insufficient. **No new dependencies are introduced this phase** (the search path is deliberately scan-based to avoid an indexing-engine dependency at this scale — see §2.2, §12.1).

### 2.1 Transcript on-disk format — NDJSON append log

**Decision: one append-only NDJSON file per session, `sessions/{agent_id}/transcript.ndjson`, one normalized `Event` per line.** (Phase PRD §3.1 and master PRD §9 both recommend NDJSON append; this spec locks it in.)

| Option | Verdict | Why |
|--------|---------|-----|
| **Single growing JSON array** (`[ev, ev, …]`) | **Rejected** | Appending requires rewriting the closing `]` (read-modify-write of the file tail) or holding the whole array in memory and re-serializing on every event. A kill mid-write leaves a syntactically invalid file (`…,` with no `]`), so the *entire* transcript becomes unparseable — the exact crash-safety failure we must avoid. Streaming reads are impossible (you must parse the whole array). |
| **NDJSON append log** (one JSON object per line) | **Chosen** | Append is a pure `O(1)` `write(2)` of `marshal(ev)+"\n"` — no rewrite, no whole-file buffer. A crash mid-write loses **at most the last partial line**; every prior complete line is intact and independently parseable (just skip a trailing partial line on read — §8.1). Streaming/tailing reads are natural (`bufio.Scanner` line-by-line, exactly the Phase 1 reader we already trust). Each line is self-describing (it *is* an `Event`), so the format needs no schema versioning beyond the `Event` envelope's append-only rule. |
| **SQLite / embedded DB** | **Rejected** | Violates the project's "no database, plain JSON files, user owns all data" constraint (master PRD §7, MAP.md). Adds a cgo or pure-Go dependency. Overkill at the expected scale (§9). |

**Crash-safety mechanics (the load-bearing part):**

- Open the file `O_APPEND|O_CREATE|O_WRONLY`. `O_APPEND` makes each `write` atomic with respect to the file offset, so concurrent/interleaved writes never corrupt each other's offsets.
- **Marshal the whole record into one `[]byte` (including the trailing `\n`) and issue a single `Write`.** A record is never written field-by-field, so a crash can only ever truncate at a record boundary or mid-record — and a mid-record truncation is a *partial last line* the reader discards (§8.1). It can never corrupt an earlier record.
- **Durability tier: `fsync` on turn boundaries, not per event.** We `f.Sync()` after writing a `turn_end` or `error` record, and on `Stop`/`Resume`/graceful shutdown — not after every `assistant_text` delta (that would fsync dozens of times per turn and stall streaming). The OS page cache holds in-flight deltas; a process *crash* (not a power loss) loses nothing because the bytes are already in the kernel buffer. This satisfies the acceptance criterion "killing the server mid-turn does not lose already-streamed transcript content" (a `kill` is a process crash; the kernel flushes the page cache to disk). Power-loss durability of the final in-flight deltas is explicitly not guaranteed and is acceptable for a local dev tool (§12.6).
- **Write-temp-rename is NOT used for the transcript** (it is used for `session.json` and all the small JSON files, per Phase 0 convention). Append logs and temp-rename are incompatible; the append-atomicity above is the correct crash-safety primitive for a log.

### 2.2 Full-text search approach — scan-on-query + maintained summary index

**Decision: a two-tier scheme.**

1. **Summary index (maintained):** a tiny per-session `session.json` (§3.3) holds the denormalized **listing** fields (name, role, project, created/updated timestamps, counts). Listing the archive (`GET /api/archive` with no `q`) reads only these small files — no transcript I/O. This file is rewritten (temp+rename) on session create, on rename, and on `turn_end` (to bump `updated_at` and counts).
2. **Scan-on-query (full-text):** when `q` is present, search runs as a **bounded concurrent scan**: match `q` against each session's `session.json` metadata first (cheap), and against the `transcript.ndjson` content second (line scan, early-exit on first match per session). Results are ranked (§4.4) and truncated.

| Option | Verdict | Why |
|--------|---------|-----|
| **Scan-on-query only** | **Chosen for now** | At the expected scale (tens to low-hundreds of sessions, each transcript typically < a few MB — §9), a concurrent grep-style scan answers a query in well under the interactive budget. Zero index to maintain, zero staleness, zero new dependency. The PRD explicitly blesses "on-the-fly scan acceptable at small scale." |
| **Maintained inverted index (Bleve/etc.)** | **Deferred (documented growth path, §12.1)** | Correct at large scale but pulls a real search-engine dependency and an index-staleness/rebuild story we don't need yet. The growth trigger and migration plan are written down so it's a localized later change (the search function is the only call site). |
| **Maintained summary index for *listing*** | **Chosen (the cheap half)** | Listing must be instant and is queried on every archive open; scanning every transcript just to list names would be wasteful. The summary `session.json` is the right granularity and is needed for resume anyway. |

**Concretely:** search = (always-fast metadata listing from `session.json`) + (on-demand transcript content scan only when `q` is non-empty), with a hard cap on scanned bytes per session and a global result cap (§4.4, §8.4).

### 2.3 Library / stdlib table

| Concern | Choice | Rationale |
|---------|--------|-----------|
| Append writer | `os.OpenFile(O_APPEND\|O_CREATE\|O_WRONLY, 0o644)` + single `Write` per record | §2.1; stdlib, atomic appends. |
| Transcript read / replay | `bufio.Scanner` with the same **8 MiB** max-token buffer as Phase 1 (§2 of Phase 1 techspec) + `encoding/json` per line | A `diff`/`tool_result` record can be large; reuse the proven Phase 1 framing so live and persisted parsing share a code path. |
| Small JSON files (`session.json`, summary) | `encoding/json` + Phase 0 write-temp-rename helper | Atomic small-file writes; consistent with the rest of the store. |
| Concurrent scan | stdlib `errgroup`-style worker pool hand-rolled with `sync.WaitGroup` + a bounded semaphore channel (e.g. `GOMAXPROCS` workers) | Bounds open file descriptors and CPU during search; no dependency (`golang.org/x/sync` optional — prefer hand-rolled to keep zero new deps). |
| Case-insensitive substring / token match | `strings.ToLower` + `strings.Contains` (whitespace-tokenized `q`; all tokens must match — §4.3) | Sufficient for substring/phrase FTS at this scale; no regex engine, no analyzer. |
| Diff linkage | reuse Phase 1 `DiffData` already in the transcript | No new storage; files derive from events (§6, §12.5). |

> **Buffer enlargement is load-bearing here too** (same warning as Phase 1 §2): the persisted-transcript reader must use the 8 MiB scanner cap, or replaying a session that contains a large diff frame silently truncates that record. The reader test (§10) must replay a transcript containing a > 64 KiB line.

---

## 3. Transcript persistence design

### 3.1 `sessions/{agent_id}/` file layout

```
~/.agentdeck/sessions/{agent_id}/
  transcript.ndjson     append-only log; one normalized Event per line (the chat history)
  session.json          composed-config snapshot + denormalized listing/index fields
  meta.lock             (optional) advisory single-writer marker while a runtime is attached
```

- **`transcript.ndjson`** — the durable transcript. One line per Phase 1 `Event` (§3.2 below). Never rewritten, only appended; survives stop and resume (a resumed session **continues appending to the same file** — §5.4, so the full logical history is one file across resumes).
- **`session.json`** — written once at session creation, updated (temp+rename) on rename and on each `turn_end`. Carries (a) the **composed config** to reconstruct the chat header and to resume, and (b) **denormalized listing fields** (name/role/project/timestamps/counts) so the archive lists without touching the transcript.
- **`meta.lock`** — optional best-effort advisory file containing the attached runtime's pid; lets us detect "is a writer currently attached" and guard double-resume (§8.2). Not a hard lock (local single-user); see §8.2.

> The directory is created on first `Start` of an agent (Phase 1 currently does not create `sessions/`; Phase 4 adds the `transcript.Writer.Open` call into the launch/Start path — §3.5).

### 3.2 NDJSON record schema (one per transcript event)

**Each line is exactly the Phase 1 normalized `Event` envelope, marshaled as compact JSON + `\n`.** No wrapping, no re-shaping — persistence and the SSE `new_message` payload are byte-identical objects, so a single serializer feeds both sinks and the replay reader reconstructs the exact stream the chat panel originally saw.

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

Per-`type` `data` payloads are exactly the Phase 1 structs (`AssistantTextData`, `ToolCallData`, `ToolResultData`, `DiffData`, `PermissionRequestData`, `TurnEndData`, `ErrorData`). **Persistence retains the normalized events only** (not the raw ACP stream); raw retention is available behind a flag for debugging (§12.2). Because each line is self-describing, the file needs no header and no schema-version field beyond the `Event` envelope's append-only evolution rule (Phase 1 §11: `agent_id`/`seq`/`type`/`ts`/`data` permanent; new `type`s additive; payload fields append-only).

**Two persisted record refinements over the live stream:**

- **Permission outcomes are persisted.** The live stream emits `permission_request`; the *resolution* (approve/deny/timeout/auto-approve) must also be in the transcript so a replayed/archived view shows the outcome, not a dangling open gate. We persist the resolution by appending the `permission_request` record **with its terminal fields filled in once resolved** is not possible on an append log — so instead we append a small follow-up record reusing the existing `tool_result`-style correlation, or set the outcome on the `PermissionRequestData`'s persisted copy at resolve time. **Decision (§12.3): append a dedicated terminal record.** We add a `permission_resolved` event `type` (additive, allowed by the append-only rule):

  ```jsonc
  { "agent_id":"a_8f3c12","seq":12,"type":"permission_resolved","ts":"...",
    "data":{ "tool_call_id":"tc_4","decision":"approve|deny|timeout|auto_approve" } }
  ```

  The live runtime emits this on the bus too (so the open chat panel collapses the gate to a resolved chip — Phase 2 §7.3 already shows a resolved chip; this just makes it event-driven and persisted). This is the one new transcript event type Phase 4 introduces; it is forward-compatible (Phase 2 clients that don't know it ignore it).

- **`session_meta` first record.** The very first line written to a fresh `transcript.ndjson` is a `session_meta` record capturing the composed config snapshot, so the transcript is self-contained for reconstruction even if `session.json` is lost:

  ```jsonc
  { "agent_id":"a_8f3c12","seq":0,"type":"session_meta","ts":"...",
    "data":{ "name":"Atlas","role":"implementer","project":"my-app","backend":"claude",
             "model":"sonnet-4-6","interface":"chat","cwd":"/Users/…/my-app",
             "system_prompt_sha":"…","created_at":"…","resumed_at":null } }
  ```

  `seq:0` is reserved for `session_meta`; transcript events start at `seq:1` (Phase 1 contract). On resume we append a *new* `session_meta` with `resumed_at` set and the fresh ephemeral context, but `seq` continues monotonically (it is not reset).

### 3.3 `session.json` (composed config + listing fields)

```jsonc
// sessions/{agent_id}/session.json
{
  "agent_id": "a_8f3c12",
  "name": "Atlas",
  "role": "implementer",
  "project": "my-app",
  "backend": "claude",
  "model": "sonnet-4-6",
  "interface": "chat",
  "group": "auth-migration",

  // composed config snapshot (frozen at launch; what resume re-applies — §5.2)
  "cwd": "/Users/asaaph/Projects/my-app",
  "add_dirs": [],
  "system_prompt": "…project context…\n\n…role persona…",   // the exact composed prompt (§6.2 of Phase 1)
  "env_keys": ["OPENAI_BASE_URL"],                           // names only; never persist secret values (§8.7)

  // ephemeral runtime ids across resumes (latest only; full history is in the transcript session_meta records)
  "last_session_id": "claude-sess-xyz",

  // listing / index fields (denormalized for the archive — §4)
  "created_at": "2026-06-22T10:00:00Z",
  "updated_at": "2026-06-22T11:32:08Z",     // last turn_end / activity
  "turn_count": 14,
  "event_count": 312,
  "files_touched": 7,                        // derived count, refreshed on turn_end (cheap rollup, §6.4)
  "commands_run": 4,
  "active": false                            // mirrors "is running/{id}.json present" at last write (best-effort; truth is running/)
}
```

- Written (temp+rename) at: session creation (from `LaunchSpec` + identity), on `/rename`, and on every `turn_end`.
- **Composed config is frozen here at launch** (master PRD invariant: edits to role/project affect future launches only). Resume re-applies *this snapshot*, not the current `roles/`/`projects/` files — see §5.2 and §12.4.
- **Secrets are never written.** Only env *key names* are stored (`env_keys`); values are re-resolved from `backends.json` at resume time (§8.7).
- `active` is a best-effort hint; the authoritative "is this agent running" signal remains `running/{id}.json` (the archive merges both — §4.2).

### 3.4 What's persisted to reconstruct the chat panel + composed config

Reconstructing the **chat panel** (Phase 2 `TranscriptView`) requires replaying the transcript events in order; each renderer (`AssistantText`, `ToolCall`, `ToolResult`, `DiffBlock`, `PermissionPrompt`→resolved chip, `TurnError`) consumes the exact `Event` shapes it consumed live. Therefore persisting the normalized `Event` stream verbatim is **sufficient and necessary**: nothing extra is needed for the panel.

Reconstructing the **composed config** (chat header: name/model/context; and resume input) comes from `session.json` (and its `session_meta` mirror in the transcript as a fallback). `context_pct` is not in the transcript line-by-line; the last-known value is recoverable from the most recent `turn_end` record's `context_pct` and is restored to `status/{id}.json` on resume (§5.3).

### 3.5 Wiring the writer into the runtime (the only hot-path change)

```go
// internal/transcript/writer.go
type Writer struct {
    f   *os.File      // O_APPEND|O_CREATE|O_WRONLY on transcript.ndjson
    mu  sync.Mutex    // serialize Append (single agent, low contention)
    dir string
}

func Open(home, agentID string) (*Writer, error)   // mkdir sessions/{id}/, open append, write session_meta if new
func (w *Writer) Append(ev runtime.Event) error     // marshal ev + "\n", single Write; fsync on turn_end/error
func (w *Writer) Sync() error                        // explicit fsync (Stop/Resume/shutdown)
func (w *Writer) Close() error
```

- `ChatRuntime.Start` (Phase 1 §4.1) gains: after `session/new` succeeds and before returning, `transcript.Open(home, agentID)` and write the `session_meta` record + `session.json`.
- `ChatRuntime.dispatch` (Phase 1 §4.3), at the point it currently does `hub.Publish(ev)` / `bus.Publish("new_message", ev)`, **also** calls `writer.Append(ev)`. Order: **append first, then publish.** Persisting before publishing guarantees that anything a client ever saw is already on disk (no "streamed but not persisted" window), directly satisfying the crash-mid-turn acceptance criterion.
- On `turn_end`/`error`: `writer.Sync()` then update `session.json` (counts, `updated_at`).
- `Stop` (Phase 1 §8.5): `writer.Sync()` + `writer.Close()` *before* deleting `running/`. **`sessions/` is left intact** (this is the F9 "stop keeps history" requirement); only `running/{id}.json` is removed (status is left at `done`, per Phase 1 §7.5). The session is now an *inactive archive entry*.

---

## 4. Archive + search design

Package: `internal/archive` (Go). Type: `Archive` (reads the file store; holds no long-lived state beyond an optional cache — §8.4).

### 4.1 What "a session" is (active + inactive)

A session exists iff `sessions/{agent_id}/` exists (equivalently, `agents/{agent_id}.json` exists — they are created together at launch). The archive lists **every** such session regardless of whether `running/{id}.json` is present:

- **Active** = `running/{id}.json` exists (or a live handle is in the registry).
- **Inactive** = stopped: `sessions/` + `agents/` present, no `running/`.

This is why the archive is a superset of the Phase 2 dashboard (which shows active + recently-stopped cards): the archive is the durable, searchable record of all of them.

### 4.2 Listing (no `q`) — fast path from summaries

`GET /api/archive` (no `q`) builds the list from `session.json` files only:

1. `readdir` `sessions/`. For each `{agent_id}` dir, read `session.json` (one small file).
2. Overlay liveness: `active = running/{id}.json exists`. (Authoritative, overrides the cached `active` hint in `session.json`.)
3. Sort by `updated_at` desc (most recent first); paginate (§7.1).

No transcript is read on the listing path. With N sessions this is N small-file reads, trivially fast for the expected N (§9).

### 4.3 Search (`q` present) — metadata + transcript scan

`GET /api/archive?q=foo bar`:

1. **Tokenize** `q` on whitespace → lowercased tokens `[t1, t2, …]`. Match semantics: **AND** (a session matches iff *every* token appears somewhere in its searchable text). Quoted `"exact phrase"` is treated as a single token (substring match preserving spaces) — supports "findable by a distinctive phrase from its transcript" (acceptance).
2. **Metadata pass (cheap, all sessions):** for each session, build the metadata haystack = lowercased `name + role + project + group + model + backend`. If all tokens match here → it's a hit (with `matched_in: ["metadata"]`); record it but **still** allow transcript match to add a snippet.
3. **Transcript pass (bounded, only sessions not already fully matched by metadata, or to extract a snippet):** scan `transcript.ndjson` line by line, lowercasing the textual content of each event (`assistant_text.delta`, `tool_call.name`+`args`, `tool_result.content`, `diff.path`+`new_text`, `permission_request.reason`, `session_meta`). Track which tokens have been seen; **early-exit** the moment all tokens are satisfied. Capture a **snippet** (the matching line's text, trimmed to ~160 chars with the match centered) for display.
4. A session is a **result** iff all tokens matched across the union of metadata + transcript text.
5. **Concurrency:** scan sessions with a bounded worker pool (§2.3) so total search latency ≈ (slowest single session scan), not the sum. Cap bytes scanned per session (§8.4) and total results (§7.1).

> Searchable text union = name, role, project (PRD-required) **plus** transcript content (PRD-required) plus model/backend/group (cheap bonus, all in metadata). The PRD's required surface (name/role/project/transcript) is fully covered.

### 4.4 Ranking

Results sort by a simple deterministic score (no TF-IDF needed at this scale):

1. **Metadata matches rank above transcript-only matches** (a name/role/project hit is a stronger signal than a buried transcript mention).
2. Within a tier, by `updated_at` desc.
3. Ties broken by `agent_id` for stability.

Each result carries `matched_in` (`["metadata"]`, `["transcript"]`, or both) and a `snippet` (transcript match line, else empty) so the UI can show *why* it matched.

### 4.5 Archive UI (minimal viable)

- Route `/archive` (added to the Phase 2 router). A search box (debounced 250ms → `GET /api/archive?q=`), and a result list: each row shows name, role · project, backend·model, created/updated, a state chip (active/inactive), and the snippet if present.
- Clicking a result:
  - **Inactive** → opens a **read-only** transcript view: reuse the Phase 2 `TranscriptView` + renderers, fed by `GET /api/sessions/{id}/transcript` (now persisted-backed, §7.4), with the composer disabled and a prominent **Resume** button.
  - **Active** → navigates to the live `/agent/:id` chat panel (it's already a running card).
- This reuses Phase 2's renderer registry and store; the only new UI is the archive list + search box + the read-only/Resume affordance.

---

## 5. Resume design

The acceptance-critical, cross-phase deliverable. Resume must restore history + config, mint a fresh ephemeral `session_id` on the **same** stable `agent_id`, re-attach a chat runtime, and reappear as a live card — and Phase 6's switch-runtime reuses this exact path.

### 5.1 `POST /api/sessions/{id}/resume` flow

```
client/CLI → POST /api/sessions/{id}/resume   (optional body: { interface?, backend?, model? } — see §5.5)
      │
API layer:
  1. Load agents/{id}.json (identity). 404 if absent.
  2. If a live handle already exists in the registry for {id} → 409 conflict
     ("already running"; resume is for inactive sessions) — §8.2.
  3. Load sessions/{id}/session.json (composed-config snapshot). If missing,
     fall back to reconstructing from the transcript's latest session_meta record. If both
     missing → 422 ("no persisted session to resume").
  4. Build a LaunchSpec from the snapshot (NOT from current roles/projects — §5.2, §12.4),
     applying any interface/backend/model override from the body (this is the seam Phase 6 uses; §5.5).
  5. Re-resolve secrets: env values from backends.json by env_keys (§8.7).
      │
      ▼
Registry → runtimeFor(spec.Agent.Interface) → Runtime.Resume(ctx, spec, lastSessionID)
      │
ChatRuntime.Resume (§5.3):
  6. Spawn the CLI (process group), ACP initialize handshake.
  7. session/load (resume the prior CLI session) IF the adapter supports it AND lastSessionID
     is still valid; else session/new (fresh CLI session) — either way a FRESH ephemeral
     sessionId is produced (§5.4). The AgentDeck transcript is the source of truth for history
     either way (§12.4), so a failed CLI-side resume degrades gracefully to new + replayed context.
  8. Re-open the SAME transcript.ndjson in append mode (seq continues from max+1, §5.4);
     append a new session_meta record with resumed_at + the new ephemeral sessionId.
  9. Write running/{id}.json (NEW session_id, new pid) + status/{id}.json
     (restore last-known context_pct from the latest turn_end; state="idle", detail="resumed").
 10. Register the Handle. Return.
      │
      ▼
State manager (Phase 2) sees the new running/ + status/ files → emits state_update →
the agent reappears as a LIVE card with prior transcript intact (client fetches
GET /api/sessions/{id}/transcript → full persisted history).
```

### 5.2 Restoring history + composed config

- **History:** lives entirely in `transcript.ndjson` and is reloaded by the client via the upgraded `GET /api/sessions/{id}/transcript` (§7.4) — the server does not need to re-stream it. The chat panel repaints the full prior transcript, then live `new_message` deltas from the resumed turn append on top.
- **Composed config:** taken from `session.json` (the **frozen launch snapshot**), not recomputed from current `roles/`/`projects/`. This preserves the master-PRD invariant "a running agent's spec is frozen; edits affect future launches only" — and means a resume a week later behaves exactly as the original launch even if the role/project files have since changed. (Documented in §12.4.)

### 5.3 `ChatRuntime.Resume` (implements the Phase 1 stub)

Signature is the one fixed in Phase 1 §3.1: `Resume(ctx, spec LaunchSpec, sessionID string) (*Handle, error)`. It is `ChatRuntime.Start` minus identity-minting, plus history continuity:

| Step | `Start` (Phase 1) | `Resume` (Phase 4) |
|------|-------------------|--------------------|
| agent_id | minted by launch flow | **reused** (already exists; never changes) |
| transcript file | created (`session_meta` seq 0) | **re-opened in append mode**; new `session_meta` appended; `seq` continues |
| CLI session | `session/new` → new `sessionId` | `session/load(sessionID)` if supported & valid, else `session/new` → always a **fresh** `sessionId` |
| running/ | written (pid, new session_id) | written (pid, **new** session_id — old one is dead) |
| status/ | `idle`, context_pct 0 | `idle`, context_pct **restored** from last `turn_end` |
| handle | registered | registered |

- **`session/load` vs `session/new`:** the ACP adapter *may* expose `session/load` to resume the CLI's own session state (so the model keeps its native context). We try it when `spec` carries a `lastSessionID` and the adapter advertised the capability in `initialize`; on any failure we fall back to `session/new`. **Either way, AgentDeck's `transcript.ndjson` is authoritative for the displayed history**, so a CLI that cannot natively resume still shows the user their full prior conversation; the model simply starts that turn without its prior native KV-context (acceptable, documented §12.4). A fresh `sessionId` is recorded in `running/` regardless — the old ephemeral id is invalid after the prior process exited.
- **Re-attach is non-destructive:** nothing in `sessions/` is overwritten; the transcript only grows. This is exactly what makes Phase 6 switch-runtime safe (stop old runtime → `Resume` with new interface/backend/model on the same `agent_id` and same transcript).

### 5.4 Stable `agent_id` + fresh ephemeral `session_id` + seq continuity

- `agent_id` is never touched on resume (loaded from `agents/{id}.json`).
- A **fresh** `session_id` is written to `running/{id}.json` on every Start *and* Resume (master PRD invariant; MAP.md load-bearing concept). The old `session_id` is only of historical interest (recorded in the per-resume `session_meta` transcript records).
- **`seq` continuity:** on `Open` of an existing transcript, the writer scans the last line to recover the max `seq` (cheap: read the tail, or track it in `session.json.event_count`/a `last_seq` field) and resumes the per-agent monotonic counter at `max+1`. This keeps `seq` globally monotonic across resumes so Phase 2's gap detection still works across a resume boundary.

### 5.5 The Phase-6 seam (switch-runtime)

`POST .../resume` accepts an optional body `{ interface?, backend?, model? }`. When present, the launch flow overrides those fields in the rebuilt `LaunchSpec` before calling `Resume`. **Phase 4 ships this with no override (resume = resume as-was).** Phase 6's `POST /api/sessions/{id}/switch-runtime` is implemented as: `Stop` the current runtime → `Resume` with the override body. Thus Phase 6 adds *no new resume machinery* — it reuses this endpoint's internals. The override fields are validated this phase (unknown backend/model/interface → 422) but only the "no override" path is exercised by Phase 4 tests; the override path is covered by Phase 6.

### 5.6 CLI: launch of an existing identity resumes, not duplicates

Phase 1's CLI (`agentdeck <role>@<project>`) always created a new agent. Phase 4 changes the CLI launch resolution:

- The CLI computes a **launch key** = the user's intent. If the user passes an explicit existing identity (a new flag `--resume <agent_id>` or `agentdeck resume <agent_id>`), the CLI calls `POST /api/sessions/{id}/resume` instead of `POST /api/sessions`.
- For the bare `agentdeck <role>@<project>` form: if there is an **inactive** session with the same `role@project` (and same name if `--name` given), the CLI **prompts/defaults to resume the most recent matching inactive session** rather than spawning a duplicate. Default behavior: resume the single most-recent inactive match; if there are multiple and it's ambiguous, list them and require `--resume <agent_id>` (or `--new` to force a fresh launch). `--new` always forces `POST /api/sessions`.
- This satisfies "CLI launch of an existing identity resumes rather than duplicates" without surprising the user when they genuinely want a fresh agent (`--new`).

---

## 6. File & command tracking design (F10)

### 6.1 Source of truth: derive from the transcript, no separate index file

**Decision (§12.5): Files and Commands are *derived* from the persisted transcript on read, not stored in a separate maintained index.** The transcript already contains every `tool_call`/`tool_result`/`diff` event; a second hand-maintained index would be a redundant, drift-prone copy. Derivation is a single linear scan of `transcript.ndjson` (the same scan the search path already does), cheap at this scale, and *always consistent* with the transcript by construction. A small rollup count is cached in `session.json` for the archive listing (§3.3) but the authoritative detail comes from deriving on demand.

### 6.2 Capturing edited files (from tool calls + diffs)

A "file edit" is detected from transcript events:

- A `tool_call` whose `name` is a known file-writing tool (`Edit`, `Write`, `MultiEdit`, `NotebookEdit`, `Create`, … — a configurable set, default per the Claude Code tool vocabulary) with a `path`/`file_path` in its `args`.
- A `diff` event (Phase 1 `DiffData`) — the strongest signal, since it carries `path` + the patch and is what enables diff linkage.

Derivation builds a **per-file rollup**:

```jsonc
// one entry in GET /api/sessions/{id}/files
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

Paths are normalized to **cwd-relative** where possible (using `session.json.cwd`) for stable display, with the absolute path retained internally; identical paths collapse into one entry (the rollup).

### 6.3 Capturing run commands (from tool calls)

A "command" is detected from a `tool_call` whose `name` is a shell-running tool (`Bash`, `Shell`, `Run`, `Terminal`, … — configurable, default Claude Code vocabulary), reading the command string from `args.command` (fallbacks: `args.cmd`, `args.script`). Each occurrence is one entry (commands are not deduped — running the same command twice is two meaningful events):

```jsonc
// one entry in GET /api/sessions/{id}/commands
{
  "command": "npm test -- --watch=false",
  "seq": 57, "ts": "…",
  "tool_call_id": "tc_7",
  "exit_status": "completed",      // from the correlated tool_result.status, if present
  "exit_error": ""                 // from tool_result.error, if failed
}
```

Correlation: the `command` comes from the `tool_call`; the `exit_status`/`exit_error` from the correlated `tool_result` (matched by `tool_call_id`). If no result yet (turn in flight), `exit_status: "in_progress"`.

### 6.4 Cheap rollup counts for the archive

On each `turn_end`, the runtime updates `session.json.files_touched` / `commands_run`. To stay cheap it maintains running counts on the handle during the live session (incrementing as edit/command tool_calls stream) rather than rescanning; the authoritative detailed lists are always re-derived on the `/files` and `/commands` endpoints. For archived (no live handle) sessions the counts in `session.json` are used for listing and the detail endpoints derive from the transcript.

### 6.5 Diff linkage

The `/files` entries carry `diff_refs` = `(seq, tool_call_id)` pointers. The UI's "view diff" action fetches the specific `diff` event(s) for that file. Implementation: the `/files` response includes the `diff_refs`; the client retrieves the diff payloads either from the already-loaded transcript (the chat panel / read-only view has them) or via a targeted read. Since the read-only transcript view already renders `diff` events through `DiffBlock` (Phase 2 §7.1), "Files → view diff" simply scrolls/links to the corresponding `diff` block in the transcript by `seq`. No diff is recomputed or duplicated; linkage is by reference into the single transcript.

### 6.6 Searchable + copyable (acceptance)

- **Files** and **Commands** tabs (added to the Phase 2 chat panel as two new tabs alongside the transcript) render the derived lists with a per-tab filter box (client-side substring filter — the lists are small per agent).
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
      "active": false,                          // authoritative: running/{id}.json present
      "matched_in": ["transcript"],             // ["metadata"] | ["transcript"] | both; omitted when q empty
      "snippet": "…added a null check to parseUser() before…"   // present for transcript matches
    }
  ]
}
```

- No `q` → listing path (§4.2): `matched_in`/`snippet` omitted; sorted by `updated_at` desc.
- With `q` → search path (§4.3–4.4).
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
  - `404` (`not_found`) — no `agents/{id}.json`.
  - `409` (`conflict`) — agent already running (a live handle exists / `running/{id}.json` present). Resume is for inactive sessions; to change a live agent use Phase 6 switch-runtime.
  - `422` (`validation`) — no persisted session to resume (no `session.json` and no `session_meta` in transcript), or an override field names an unknown backend/model/interface.
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
Derived from the transcript (§6.2). Sorted by `last_ts` desc (most recently touched first). `404` if no such agent (no `agents/{id}.json` / `sessions/{id}/`). Works for both active and archived sessions.

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
Derived from the transcript (§6.3). Sorted by `seq` desc (most recent first; chronological order preserved within the same scan). `404` if no such agent.

### 7.5 `GET /api/sessions/{id}/transcript` — upgraded (persisted-backed)

Supersedes the Phase 2 in-memory version. Now reads `sessions/{id}/transcript.ndjson` and returns the full persisted event stream (still usable for reconnect repaint and now also for archive read-only view + resume repaint).

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

`POST /api/sessions` (launch), `GET /api/sessions`, `GET /api/sessions/{id}`, `prompt`, `cancel`, `stop`, `rename`, `permission`, `GET /api/events`, `GET/PUT /api/layout`. No bus or SSE changes this phase.

---

## 8. Edge cases & error handling

### 8.1 Corrupt / partial transcript

- **Partial trailing line** (crash mid-`Write` despite single-Write atomicity, e.g. truncated by `kill -9` between the kernel accepting part of the buffer — rare with `O_APPEND` single writes but handled): the reader uses `bufio.Scanner`; the final token without a trailing `\n` that fails `json.Unmarshal` is **dropped silently** (logged at debug). Every prior complete line is intact. This is the core crash-safety guarantee.
- **A corrupt line in the middle** (should not happen with atomic appends, but defensive): on `json.Unmarshal` failure for a non-final line, log the (truncated) raw line and **skip it, continue scanning** — exactly the Phase 1 §8.3 resync behavior. One bad line never aborts a transcript replay.
- **Missing `session.json`** but present transcript: reconstruct listing/config fields from the latest `session_meta` record (§3.2). Missing both → the session is unlistable beyond its `agent_id`; surface it in the archive with degraded fields and a `"degraded": true` hint rather than hiding it.
- **`seq` recovery on a corrupt tail:** if the last line is partial, the writer recovers max `seq` from the last *valid* line; the dropped partial line's `seq` is simply re-used by the next append (no harm — it was never delivered).

### 8.2 Resume of an already-running agent

- A live handle in the registry (or `running/{id}.json` present with an alive pid) → `POST .../resume` returns `409`. Resume targets inactive sessions only.
- **Stale `running/` with a dead pid** (server crashed leaving a ghost): Phase 1 §8.5 already reconciles stale `running/` on server start (deletes it, sets status). So a resume after a crash sees no live handle and proceeds normally. If a resume request races startup reconciliation, the `meta.lock` advisory check (pid liveness) is the tiebreaker; a dead pid → proceed, a live pid → `409`.
- **Double resume race** (two concurrent resume requests for the same id): the registry's `Start`/`Resume` path takes the registry lock and checks for an existing handle (Phase 1 §8.6 double-start guard); the second request gets `409`.

### 8.3 Very large transcripts

- **Reading:** the `/transcript` endpoint streams the file line-by-line; for a very large transcript the client can request `?since_seq=` to avoid refetching the whole thing on reconnect. (A future enhancement could paginate from the tail; not needed at expected scale — §9.)
- **Writing:** append is `O(1)` regardless of file size, so a long-running agent's writes never slow down.
- **Search:** the transcript scan caps **bytes scanned per session** (default 16 MiB; configurable `ARCHIVE_SCAN_CAP`) and early-exits on full token match. A session whose transcript exceeds the cap without matching is marked `"truncated_scan": true` in that result's debug field (not surfaced in the normal UI) so we don't silently claim "no match" on a huge unscanned tail. At expected scale this cap is rarely hit.
- **Memory:** no operation loads a whole transcript into memory except the `/transcript` endpoint's JSON response (bounded by `limit`); search and files/commands derivation are streaming line scans.

### 8.4 Search performance

- **Listing** is N small-file reads — fast and constant-ish.
- **Search** is bounded-concurrency line scans with early-exit and per-session byte caps (§8.3). Worst case (a `q` that matches nothing, forcing a full scan of every session) is bounded by `min(transcript_size, scan_cap)` summed over sessions, parallelized across workers. At the expected scale (§9) this is comfortably interactive.
- **Optional in-process cache (off by default):** a small LRU keyed by `(agent_id, file_mtime)` caching the lowercased searchable blob per session can cut repeated-query latency; gated behind `ARCHIVE_CACHE=1` because correctness without it is the priority and the file `mtime` is the only invalidation signal needed. The growth path (§12.1) supersedes this if scale demands.

### 8.5 Concurrent write during resume / read during write

- A session being **read** (search/transcript/files) while it is **live and appending**: `O_APPEND` writes are atomic; the reader either sees a complete final line or a partial one it drops (§8.1). No locking needed between reader and writer — the append log is naturally read-concurrent.
- **Resume re-opening the transcript:** the prior runtime is stopped (process dead, `Stop` called `Sync`+`Close`) before resume opens the file for append, so there is at most one writer per transcript at a time. The `meta.lock` documents the current writer's pid as a best-effort guard.

### 8.6 Stop semantics (keeps `sessions/`, removes `running/`)

Restated for completeness (the F9 requirement): `POST /api/sessions/{id}/stop` (Phase 1 §7.5) is amended so that before deleting `running/{id}.json` it calls `writer.Sync()` + `writer.Close()`. `sessions/{id}/` is **never** deleted by stop. The session immediately becomes an inactive archive entry, fully searchable and resumable. `status/{id}.json` is left at `done` (Phase 1 convention) so the archive can show a final state and the Phase 2 card shows "stopped".

### 8.7 Secrets

`session.json` and `session_meta` store env **key names only** (`env_keys`), never values. On resume, env values are re-resolved from `backends.json` via `composeEnv` (Phase 1 §6.2) using the snapshot's backend/model. This prevents API keys leaking into the on-disk transcript/archive (which is plain-text and user-readable). The raw-retention debug flag (§12.2) carries the same prohibition — raw ACP frames are scrubbed of known secret env values is not feasible, so the raw flag is documented as **debug-only, do not enable with real credentials in the transcript** (§12.2).

---

## 9. Scale assumptions (sizing the decisions)

The decisions in §2 and §8 are sized for the realistic local-tool scale, stated so a reviewer can check the math:

- **Sessions:** tens to a few hundred over the tool's lifetime on one machine (one developer). Listing N small `session.json` files is trivial at N≈hundreds.
- **Transcript size:** a typical coding session is hundreds to low-thousands of events; even verbose sessions are single-digit MB. A pathological multi-hour session might reach tens of MB — still within the per-session scan cap and fast to append.
- **Search latency budget:** interactive (< ~300ms for typical, < ~1s worst case for an all-miss full scan), met by concurrent capped scans without an index.
- **Growth trigger** (when to build the real index, §12.1): when either total transcript bytes across sessions exceed ~1 GB or a no-match search exceeds ~1s on the target machine.

---

## 10. Implementation task breakdown (ordered)

Each step is small and independently testable. The transcript writer/reader and a fixture transcript come first so everything downstream is TDD'd against deterministic data.

**Persistence core:**
1. `internal/transcript`: `Event` reuse (Phase 1 type), `Writer` (`Open`/`Append`/`Sync`/`Close`, `O_APPEND`, single-Write, fsync-on-turn_end), `session_meta` first record, `seq` recovery on reopen. Unit tests: append→read round-trip; reopen continues seq; **partial-trailing-line dropped**; **mid-file bad line skipped**; a > 64 KiB line round-trips (buffer cap).
2. `transcript.Reader` + replay: stream lines → `[]Event`; `since_seq` filter; skip `seq:0` meta by default. Tests against a fixture transcript including a large diff line.
3. `permission_resolved` event type (additive): runtime emits it on resolve (approve/deny/timeout/auto); persisted + published. Update Phase 1 permission paths to append it.
4. `session.json` writer/reader (`internal/session`): build from `LaunchSpec`+identity at create; update on rename + turn_end (counts, updated_at, last_seq, last context_pct). Atomic temp+rename. Secrets → key names only.

**Wire into the runtime:**
5. `ChatRuntime.Start`: open the transcript writer, write `session_meta` + `session.json`. `dispatch`: **append before publish**. `turn_end`/`error`: `Sync` + update `session.json` + maintain live file/command rollup counts.
6. `ChatRuntime.Stop`: `Sync`+`Close` writer before deleting `running/`; leave `sessions/` intact.

**Resume:**
7. `ChatRuntime.Resume` (real): spawn + handshake; `session/load` with fallback to `session/new`; reopen transcript in append mode + new `session_meta`(resumed_at); write running/ (fresh session_id) + status/ (restored context_pct). Registry wires it (remove the Phase 1 stub).
8. `POST /api/sessions/{id}/resume` handler: identity/already-running/snapshot checks; build `LaunchSpec` from snapshot (+optional override seam); call `Resume`; return `{agent,running,status,resumed}`. Errors per §7.2.
9. CLI resume-not-duplicate: `--resume <id>` / `agentdeck resume <id>`; bare-form most-recent-inactive-match → resume, `--new` to force fresh, ambiguity → list + require `--resume`.

**Archive + search:**
10. `internal/archive`: listing (summary scan, liveness overlay, sort, paginate); search (tokenize, metadata pass, bounded concurrent transcript scan with early-exit + snippet + per-session byte cap; rank; cap results).
11. `GET /api/archive?q=` handler (§7.1). Tests: list active+inactive; findable-by-phrase (transcript hit); metadata hit; AND semantics; pagination; snippet.

**Files / commands:**
12. `internal/derive`: single-scan derivation of files rollup (+diff_refs) and commands list (+exit status correlation) from a transcript. Configurable tool-name sets.
13. `GET /api/sessions/{id}/files` + `GET /api/sessions/{id}/commands` handlers (§7.3–7.4).

**Endpoint upgrade + UI:**
14. Upgrade `GET /api/sessions/{id}/transcript` to read persisted NDJSON (replace the Phase 2 in-memory impl); `since_seq`/`include_meta`.
15. Frontend: `/archive` route (search box + result list + snippet + state chip); read-only transcript view (reuse `TranscriptView`, disable composer, show Resume button) → `POST .../resume`; **Files** and **Commands** tabs in the chat panel (derived lists, per-row copy, filter, diff link → scroll to `diff` block by seq).
16. Wiring + manual verification against a real `claude-code-acp`: run a turn that edits files + runs commands, stop, confirm archive lists it, search finds it by a transcript phrase, resume restores history and continues, files/commands appear and copy.

---

## 11. Testing strategy

### 11.1 Crash-mid-turn → no data loss (acceptance-critical)

- **Unit (writer):** append K records, then *do not* call Sync/Close; reopen the file and read — all K records present (page cache → kernel; a `kill` is a process crash, not power loss). Then write a deliberately **partial** final line (truncate the file mid-record), reopen, assert the partial line is dropped and the K prior records are intact.
- **Integration (fake CLI):** reuse Phase 1's `crash_midturn` fake-ACP scenario (emits a chunk then `os.Exit(1)`). Assert: after the crash, `transcript.ndjson` contains every event that was published before the crash (compare against the events the SSE subscriber received). This is the literal "killing the server mid-turn does not lose already-streamed content" check — and "append before publish" (§3.5) guarantees persisted ⊇ delivered.
- **Integration (server kill):** drive a real or fake turn, `kill -9` the server process mid-turn, restart, `GET /transcript` → assert all pre-kill events present.

### 11.2 Findable-by-phrase (acceptance-critical)

- Persist a transcript containing a distinctive phrase only in an `assistant_text` delta (not in metadata). `GET /api/archive?q="distinctive phrase"` → the session is in `results` with `matched_in:["transcript"]` and a `snippet` containing the phrase.
- Negative: a phrase present in *no* session → `total:0`.
- Metadata hit: `q` = the role/project name → match with `matched_in:["metadata"]`.
- AND semantics: two tokens each present but in different sessions → neither matches; both in one session → that one matches.
- Active + inactive both listed (no `q`): start one agent (active), stop another (inactive) → both appear; `active` flags correct.

### 11.3 Resume-with-history-intact (acceptance-critical)

- Launch (fake CLI), run a turn producing several events, `Stop`. Assert no live handle, `sessions/` intact, `running/` gone.
- `POST .../resume` → assert: `200`; `agent_id` unchanged; `running.session_id` **differs** from the pre-stop one; a card reappears (state manager emits `state_update`); `GET /transcript` returns the **full prior** events plus the new `session_meta`(resumed_at); a subsequent prompt appends with `seq` continuing monotonically past the pre-resume max.
- `409` on resume of an already-running agent.
- `422` on resume with no persisted session.
- CLI: bare form with one inactive match resumes (same `agent_id`); `--new` forces a fresh `agent_id`; ambiguous match → error listing candidates.

### 11.4 Files/commands appear & copyable (acceptance-critical)

- Persist a transcript with 3 file-edit tool_calls/diffs (one path edited twice → rollup `edit_count:2`, 2 distinct paths) and 2 Bash tool_calls. `GET /files` → 2 entries (3 edits rolled into 2 paths) with correct counts and `has_diff`/`diff_refs`; `GET /commands` → 2 entries with `command` strings and `exit_status` from correlated results.
- Diff linkage: a file with a `diff` event has non-empty `diff_refs` pointing at the right `seq`/`tool_call_id`; a file edited via a non-diff tool has `has_diff:false`.
- Frontend (Vitest/RTL): Files/Commands tabs render the lists; copy button copies the path/command; filter box narrows the list; "view diff" link targets the right transcript `seq`.

### 11.5 Other

- **Reader robustness:** fixture transcripts with (a) a > 64 KiB line, (b) a mid-file malformed line, (c) a partial trailing line — all replay correctly per §8.1.
- **`session.json` updates:** counts/`updated_at`/`last_seq` advance on `turn_end`; rename updates `name` without touching the transcript.
- **Search caps:** a synthetic oversized transcript hits the byte cap → result flagged `truncated_scan` (debug), no crash.
- **Concurrency / `-race`:** concurrent search (readers) while a session is live-appending (writer) — no race, reader drops only any partial tail.
- **Secrets:** assert no env *value* ever appears in `session.json` or the transcript (only `env_keys`).

---

## 12. Resolved decisions (phase §6 / master PRD §9 open questions)

### 12.1 Search index strategy — scan-on-query now, documented growth path

- **Now:** **scan-on-query** for full-text (metadata pass + bounded concurrent transcript line-scan with early-exit and per-session byte cap), plus a **maintained per-session summary** (`session.json`) for instant listing. No search-engine dependency. Justified by scale (§9) and the PRD's blessing of on-the-fly scan at small scale.
- **Growth path (when §9's trigger fires):** introduce a maintained inverted index (candidate: `github.com/blevesearch/bleve/v2`, pure-Go) stored at `~/.agentdeck/index/`, updated incrementally on `turn_end` and rebuilt on demand (`agentdeck reindex`). The **only call site to change** is `archive.Search`; the API contract (§7.1) is unchanged. Index staleness is bounded by indexing on `turn_end`; a self-heal rebuild runs if the index version mismatches. This is written down so the migration is a localized, contract-preserving change.

### 12.2 Transcript format — NDJSON append; normalized retained; raw behind a flag

- **Format:** **NDJSON append log** (`transcript.ndjson`), one normalized `Event` per line (§2.1). Justified vs single-JSON (crash-safety, O(1) append, streaming reads) and vs a DB (project constraint).
- **Normalized vs raw:** **retain normalized events only** by default — they are the contract every consumer (chat panel, archive, files/commands, resume) needs, and they are backend-agnostic (Phase 1 §4.2). **Raw ACP frames are retained only behind `AGENTDECK_RAW_TRANSCRIPT=1`**, written to a sibling `sessions/{id}/raw.ndjson` (one raw stdio frame per line) for protocol debugging. The raw file is **debug-only**: it may contain secrets/credentials in unredacted tool args/env echoes, so it is off by default and documented "do not enable with real credentials" (§8.7). Nothing in the product reads `raw.ndjson`; it exists purely for diagnosing ACP-mapping bugs.

### 12.3 Permission outcomes — dedicated `permission_resolved` record

A new additive transcript event `type: "permission_resolved"` (`{tool_call_id, decision: approve|deny|timeout|auto_approve}`) is appended when a gate is resolved, so archived/replayed transcripts show the outcome rather than a dangling open request. It is emitted on the bus too (Phase 2 client collapses the gate to a resolved chip event-driven). Forward-compatible: older clients ignore unknown types.

### 12.4 Resume uses the frozen launch snapshot; transcript is authoritative for history

- Resume rebuilds `LaunchSpec` from `session.json`'s **frozen composed-config snapshot**, *not* from current `roles/`/`projects/` files — preserving the master-PRD invariant "a running agent's spec is frozen; edits affect future launches only." A resume any time later reproduces the original composition.
- **AgentDeck's `transcript.ndjson` is the source of truth for displayed history.** The CLI's native `session/load` is attempted (to preserve the model's native context) but is best-effort: on failure we fall back to `session/new`, and the user still sees their full prior conversation (replayed from our transcript). This decouples resume correctness from the adapter's session-persistence support.

### 12.5 Files/commands — derived from the transcript, not a separate index

Files and Commands are **derived on demand** from the persisted transcript (a single line scan), with cheap rollup *counts* cached in `session.json` for archive listing. No separate hand-maintained `files.json`/`commands.json` (which would drift from the transcript). Always-consistent by construction; cheap at this scale. The tool-name sets that identify "file edit" vs "run command" tools are configurable, defaulting to the Claude Code vocabulary (Edit/Write/MultiEdit/… and Bash/Shell/…).

### 12.6 Durability tier — fsync on turn boundaries

`fsync` on `turn_end`/`error`/`Stop`/`Resume`/shutdown, not per event. This guarantees no loss on a process **crash** (`kill`) — the acceptance bar — while not stalling streaming with dozens of fsyncs per turn. Power-loss durability of the last few in-flight deltas is explicitly out of scope for a local dev tool. Tunable to per-event fsync via `AGENTDECK_FSYNC=always` if a user wants it.

---

## Appendix A — Acceptance checklist mapping

| PRD acceptance (phase §5) | Covered by |
|---------------------------|-----------|
| Stopped agent appears in archive with name/role/project/timestamps | §3.3 `session.json`, §4.2 listing, §7.1; §11.2 |
| Findable by a distinctive transcript phrase via `?q=` | §4.3 transcript scan + snippet, §7.1; §11.2 |
| Resume restores full transcript + config, re-attaches runtime continuing the same logical session | §5 (Resume flow + `ChatRuntime.Resume`), §7.2; §11.3 |
| After 3 file edits + 2 commands, all 5 appear in Files/Commands tabs and are copyable | §6 derivation, §7.3–7.4, §6.6; §11.4 |
| Edited files link to diffs where a diff exists | §6.5 `diff_refs`; §11.4 |
| Killing the server mid-turn does not lose already-streamed content | §2.1 crash-safety + §3.5 append-before-publish; §11.1 |

## Appendix B — Interfaces produced for later phases

1. **NDJSON transcript format + `transcript.Writer`/`Reader`** (§3.2, §3.5). **Phase 6 switch-runtime** reads/continues the *same* `transcript.ndjson` across an interface/backend/model switch (the log only grows; nothing is rewritten), so a switch is non-destructive by construction. The `permission_resolved` and `session_meta` record types are part of this frozen format.
2. **Resume machinery** (§5): `ChatRuntime.Resume` + `POST /api/sessions/{id}/resume` with the optional `{interface,backend,model}` override body. **Phase 6** implements `POST .../switch-runtime` as `Stop` → `Resume(override)` — reusing this endpoint's internals with **no new resume code**. The frozen-snapshot rule (§12.4) means a switch re-composes only the overridden fields, keeping the rest of the spec stable.
3. **`session.json` snapshot** (§3.3): the frozen composed-config record. Phase 6's switch overrides `backend`/`model`/`interface` against this snapshot; Phase 5's resume hook adds `mcpServers` via the same `LaunchSpec` seam (`ExtraArgs`/`mcpServers`, Phase 1 §3.1) without changing the snapshot shape.
4. **Persisted-backed `GET /api/sessions/{id}/transcript`** (§7.5): every later phase that needs history (archive view, resume repaint, switch-runtime repaint) reads it; it replaces the Phase 2 in-memory stopgap.
5. **Archive/search API** (§7.1): a stable contract whose implementation can swap scan→index (§12.1) without changing callers.
