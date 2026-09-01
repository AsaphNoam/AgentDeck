# FS-19 — Worktree projects

**Status:** Partial
**Code:** `internal/server/`, `internal/config/`, `internal/state/`, `ui/src/features/dashboard/` · **Journeys:** —
**Absorbed:** —

## 1. Purpose

AgentDeck makes isolated parallel coding work effortless by leaning into its existing model: the
project is the workspace. A **worktree project** is an ordinary project whose working directory is a
Git worktree that AgentDeck created, bootstrapped, and tracks. Spinning up a parallel stream of work
is creating one more project; everything inside a project — agents, tasks, pipeline stages — shares
its checkout, and isolation exists between projects. The checkout is disposable infrastructure: the
branch, commits, project, agent identities, and conversations are the durable things.

## 2. Behavior

### 2.1 Creating a worktree project

- **R1 (planned).** Any active project whose expanded `cwd` resolves inside a Git working tree
  offers a **New worktree project** action. It creates, as one visible operation: a new branch off
  the base branch (R2), a fresh Git worktree for that branch under AgentDeck's owned worktree area
  (R4), and a new active project whose `cwd` is that worktree and whose `color`, `context_prompt`,
  `add_dirs`, `base_branch`, and `setup_command` are copied from the source project. The creation
  form asks only for what varies: a **Title** (pre-filled from the source title), an editable
  **Branch** pre-filled as `agentdeck/<slug(title)>`, and an editable **Base** pre-filled from the
  source project's effective base branch. Editing the base to another branch is the explicit way to
  stack on feature work; nothing ever stacks implicitly. The action creates a project only; it does
  not launch an agent.
- **R2 (planned).** Every project has an optional `base_branch` (FS-04.R45). When it is empty, the
  effective base is the repository's default branch, auto-detected at use time. Independent new
  work always branches from the effective base, keeping logical task relationships separate from
  Git ancestry.
- **R3 (planned).** When the new project has a non-empty `setup_command`, AgentDeck runs it
  non-interactively inside the fresh worktree immediately after creation, before reporting the
  project ready. A failing or timed-out setup command does **not** undo or block creation: the
  project exists and is launchable, and the failure surfaces as a visible warning carrying the
  captured output. Setup is AgentDeck's job at checkout creation; it is never delegated to the
  first coding agent.
- **R4 (planned).** AgentDeck-owned worktrees live under `$AGENTDECK_HOME/worktrees/{project-id}/`,
  keyed by the new project's immutable id. For each worktree it creates, AgentDeck durably records
  ownership: the checkout path, the branch, and the source repository. A project whose `cwd` merely
  lies inside a worktree AgentDeck did not create is **external**; AgentDeck never deletes an
  external checkout and never offers to.

### 2.2 Living in a worktree project

- **R5 (planned).** Agents, tasks, and pipeline stages change nothing: they launch into the
  project's `cwd` through the existing composition seam, so every agent in a worktree project
  shares its checkout, and stop/resume continues in the same checkout via the existing frozen
  launch snapshot. Intentional sharing **is** membership in the same project; parallel isolation
  **is** two projects.
- **R6 (planned).** A worktree project is visibly a worktree project: its dashboard card and its
  scoped project header show the branch name, so parallel streams stay distinguishable at a glance
  (presentation entry points in FS-02.R60).

### 2.3 Disposable checkout, durable branch

- **R7 (planned).** When a launch in a worktree project finds the owned checkout directory missing,
  AgentDeck recreates the worktree from the recorded branch, re-runs the project's `setup_command`
  under R3's warning semantics, and proceeds with the launch, reporting that recreation happened.
  It never silently substitutes the base branch: if the recorded branch no longer exists, the
  launch fails with an actionable error naming the branch and the recovery options.
- **R8 (planned).** Archiving a worktree project with an owned checkout offers — never forces —
  deleting that checkout. The dialog defaults to keeping it, states that the branch and commits are
  kept either way, and shows whether the checkout currently holds uncommitted changes (the only
  thing deletion can lose); if that state cannot be determined, the dialog says so instead of
  claiming the checkout is clean. Accepting removes the checkout directory and its Git worktree
  registration only. Deleting the project definition of a project that still has an owned checkout
  presents the same offer. No other path — agent stop, agent archive, crash, server restart, or
  retention — ever deletes a checkout.

## 3. States & transitions

A worktree project is an ordinary project in every existing lifecycle: it archives, reactivates,
and deletes under FS-04.R35/R36. Its extra state is the owned checkout, which is either **present**,
**missing** (recreated on next launch, R7), or **removed by explicit choice** (R8, after which the
project is archived or deleted; the branch remains in the repository). Ownership never changes:
a checkout is owned from creation, and external checkouts never become owned.

## 4. Edge cases & errors

- **R9 (planned).** The **New worktree project** action is absent when the source project's `cwd`
  is not inside a Git working tree, and unavailable on archived projects. When Git is not
  installed or the repository cannot be read, creation fails with an actionable error before
  anything is created.
- **R10 (planned).** Creation is all-or-nothing ahead of setup: if the branch already exists, the
  worktree path collides, or any Git step fails, the operation reports the specific error, and no
  project, branch, or directory is left behind. The setup command (R3) is the only step whose
  failure leaves the created project in place.
- **R11 (planned).** Forking a worktree project is allowed and behaves identically: the new branch
  still comes off the effective base branch by default, not off the source project's branch.

## 5. Acceptance criteria

- **A1 (planned).** From a repo-backed project, the fork action creates branch + owned worktree +
  project copying the source fields, the new card shows its branch, and an agent launched there
  runs with `cwd` inside the new checkout. Verified by integration test plus a usability journey.
- **A2 (planned).** A failing `setup_command` still yields a launchable project and a visible
  warning containing the captured output. Verified by test.
- **A3 (planned).** With the owned checkout deleted out-of-band, the next launch recreates it on
  the recorded branch, re-runs setup, reports the recreation, and the agent starts; with the branch
  also deleted, the launch fails with the actionable error and creates nothing. Verified by test.
- **A4 (planned).** The archive dialog on a dirty owned checkout shows the uncommitted-changes
  state, defaults to keeping, keeps on decline, and on accept removes the checkout and its
  registration while the branch and commits survive in the source repository. Verified by test
  plus manual gate.
- **A5 (planned).** A project whose `cwd` is a user-created worktree is treated as external: no
  deletion offer at archive or delete, and no AgentDeck code path removes it. Verified by test.
- **A6 (planned).** A non-repo project shows no fork action; a fork attempt with a colliding branch
  name fails actionably and leaves no partial project, branch, or directory. Verified by test.
- **A7 (planned).** Two agents in one worktree project share one checkout; two sibling forks get
  distinct checkouts and branches from the same base. Verified by test.

## 6. Deviations & open decisions

- The unscoped New-project modal does not offer creating a worktree in v1; forking an existing
  repo-backed project is the only creation path.
- `add_dirs` pass through unchanged on fork, even when they point into Git repositories;
  multi-repo isolation is explicitly out of scope.
- No merge/PR automation, no cross-project diff UI, and no automatic project creation for tasks.

## 7. Traceability

To be recorded at implementation.
