# TS-12 — Worktree lifecycle

**Status:** Partial
**Code:** `internal/worktree/` (planned), `internal/server/`, `internal/config/`, `internal/state/`
**Absorbed:** —

## 1. Scope

The architecture behind FS-19 worktree projects: the Git execution boundary, the durable ownership
record, fork orchestration, disposable-checkout recreation, explicit deletion, and the API surface
the dashboard consumes. Out of scope: task/pipeline semantics (unchanged — they inherit the
project's cwd through the existing composition seam), multi-repo coordination, and any merge or
branch automation beyond creating the fork branch.

## 2. Design & constraints

- **R1 (planned) — One Git execution boundary.** All Git operations live in one package
  (`internal/worktree`), invoked argv-only via `exec.Command` (never a shell), rooted with
  `git -C <path>`, with bounded per-command timeouts, `stdin` closed, and `GIT_TERMINAL_PROMPT=0`
  so no operation can hang on a prompt. A missing `git` binary or an unparseable output is an
  actionable error, and parsing tolerates version variance (INV §12) by using plumbing commands
  (`rev-parse`, `symbolic-ref`, `status --porcelain`, `worktree list --porcelain`) rather than
  scraping porcelain UI output.
- **R2 (planned) — Ownership is a SQLite record.** Machine-created worktree ownership lives in
  `state.db` (TS-02.R1's writer split; a hand-editable JSON field could forge ownership and widen
  the deletion surface). A `project_worktrees` row — see §3 — exists exactly for checkouts
  AgentDeck created; its presence is the sole ownership test (FS-19.R4). The `project` column is a
  logical, non-cascading reference like TS-02.R25's task references: deleting a project never
  cascades here, and the deletion flow removes the row explicitly.
- **R3 (planned) — Fork ordering is crash-safe toward under-ownership.** Fork executes: validate
  (source project, repo, branch/base, target path) → create branch and worktree (the external
  effect) → insert the ownership row → write the new project file → run setup (FS-19.R3). Any
  failure before setup rolls back in reverse: remove the project file, delete the row, remove the
  worktree, delete the still-commitless branch — satisfying FS-19.R10's all-or-nothing rule. A
  crash inside the window can only leave a checkout that is *not* recorded as owned; per INV §15
  the record that authorizes a future deletion is never written ahead of the artifact it describes,
  so every crash residue is inert and conservatively treated as external.
- **R4 (planned) — Checkout resolution is one shared helper on every start path.** A single
  `ensureWorktreeCheckout(project)` helper consults the ownership record and recreates a missing
  owned checkout from its recorded branch (FS-19.R7) before any process starts. It is called from
  `composeLaunch`, `composeResumeSpec`, `composeSwitchSpec`, and pipeline `ValidateStage` —
  replacing the two currently duplicated cwd stat checks (`internal/server/launch.go`,
  `internal/server/pipeline_lifecycle.go`) with one seam, per INV §2 and TS-01.R6/R9. Resume and
  switch keep the frozen `snap.Cwd`; the helper only re-materializes the directory the frozen path
  names, never re-derives a different path. Recreation reports itself in the start response and
  status detail; a recorded branch that no longer exists fails the start with an actionable error.
- **R5 (planned) — Setup runs non-interactively with launch-equivalent env.** `setup_command` runs
  as `/bin/sh -c` in the checkout with the same inherited server environment agents get, `stdin`
  closed, combined output captured with a bounded tail (64 KiB), and a 10-minute timeout. Result,
  timestamp, and captured tail are stored on the ownership row so the warning (FS-19.R3/A2)
  survives the creating request; the fork and start responses carry it too.
- **R6 (planned) — Status queries are cheap where they are frequent.** The projects list enriches
  each project from the DB only (`worktree: {owned, branch}` — no Git subprocesses on the list
  path). `repo_backed`, needed for action visibility (FS-02.R60), comes from a bounded
  `rev-parse --is-inside-work-tree` check memoized per expanded cwd with a short TTL and
  invalidation on project edit. Expensive facts (dirty state for the archive dialog, FS-19.R8) are
  computed only on the on-demand per-project endpoint (§3); a failed or timed-out check reports
  "undeterminable", never a fabricated clean state (INV §8).
- **R7 (planned) — Deletion is gated, verified, and ordered.** Checkout deletion runs only inside
  the existing project-archive claim window (`beginProjectArchive`, TS-01.R13) or the
  project-delete path after the same claim, with all processes stopped. Before removing anything,
  the flow verifies the recorded path is the canonical `$AGENTDECK_HOME/worktrees/{project-id}`
  location, symlink-free, and currently registered to the recorded repository via
  `git worktree list`; any mismatch aborts with an error instead of deleting. Removal uses
  `git worktree remove --force` (the dialog already disclosed dirty state and the person consented,
  FS-19.R8) and only then deletes the ownership row, so a crash between the two leaves a row whose
  checkout is missing — the R4 recreation case — rather than an unrecorded owned checkout. External
  checkouts have no row and therefore no deletion path at all.
- **R8 (planned) — Owned paths follow the established filesystem rules.** The `worktrees/` root
  and `{project-id}` construction follow the project-resources pattern: slug-validated id before
  path construction, owner-only (0700) root, symlink rejection on root and leaf
  (FS-11.R8/R9, TS-02.R13). No AgentDeck code path ever removes a directory outside the owned
  `worktrees/` root.
- **R9 (planned) — Base detection order is fixed.** Effective base = project `base_branch` when
  set; else `origin/HEAD`'s target (via `symbolic-ref`); else the source repository's current
  branch; a detached HEAD with no `origin/HEAD` is an actionable fork error. Detection runs at use
  time (fork, FS-19.R2), never cached durably.
- **R10 (planned) — Git refs are the atomic claim.** Concurrent forks are not serialized by
  AgentDeck; branch-ref creation is the atomic claim (INV §5), so the second identical fork fails
  on the existing branch and rolls back per R3. Server-derived project ids make checkout-path
  collisions unreachable; the path is still pre-checked and fails closed.

## 3. Interfaces & data shapes

Migration (next version, forward-only, TS-02.R6):

```sql
CREATE TABLE project_worktrees (
  project        TEXT PRIMARY KEY,   -- immutable project id; logical reference, no cascade
  repo_path      TEXT NOT NULL,      -- expanded absolute path of the source repository
  branch         TEXT NOT NULL,
  checkout_path  TEXT NOT NULL,      -- canonical $AGENTDECK_HOME/worktrees/{project}
  created_at     TEXT NOT NULL,
  setup_ok       INTEGER,            -- NULL until first setup run
  setup_at       TEXT,
  setup_output   TEXT                -- bounded tail of the last setup run
);
```

Endpoints (error envelope and status codes per TS-03):

- `POST /api/projects/{project}/worktree-fork` `{title, branch, base}` → `201` with the new
  project id and any setup warning; Git/validation failures are `422` with the specific reason
  (FS-19.R9/R10). Archived source projects are rejected.
- `GET /api/projects/{project}/worktree` → `{owned, repo_backed, branch, base, dirty,
  dirty_known, setup: {ok, at, output}}`; computed on demand (R6), feeds the fork form and the
  archive dialog.
- The archive and delete endpoints accept an optional `delete_checkout` boolean, honored only for
  owned checkouts (R7); its absence never deletes.
- `GET /api/projects` gains per-project `worktree: {owned, branch}` and `repo_backed` (R6).

Project config file additions (`base_branch`, `setup_command`) are owned by FS-04.R45 and
TS-02's config authority; both are optional and absent-tolerant.

## 4. Invariants

- **INV §2 / TS-01.R6/R9** — checkout resolution joins the shared composition helpers (R4); the
  seam count goes down, not up.
- **INV §4** — the checkout deliberately opts out of create/teardown symmetry, like FS-11 project
  resources: creation has no automatic teardown on any exit path; the only teardown is the
  explicit, consented deletion flow (R7). This deviation is the FS-19.R8 product decision.
- **INV §5** — branch-ref creation is the fork's atomic claim (R10).
- **INV §7** — status enrichment isolates per-project Git failures; one unreadable repo degrades
  that project's fields, never the list.
- **INV §12** — plumbing-only Git parsing with timeout and prompt suppression (R1).
- **INV §14** — owned paths are validated and symlink-rejected before any create or delete (R8).
- **INV §15** — ownership rows are written after the checkout exists and deleted after it is
  removed, so no crash window can authorize deleting something AgentDeck did not create (R3/R7).

## 5. Deviations & open decisions

- Setup uses `/bin/sh -c` with the server's inherited environment — matching what launched agents
  see — rather than the user's login shell; shell-profile-dependent tooling may need an absolute
  path in `setup_command`.
- The `repo_backed` memoization TTL is an implementation constant; it trades a short staleness
  window for a subprocess-free list path.

## 6. Traceability

To be recorded at implementation.
