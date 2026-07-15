# FS-11 — Project shared resources

**Status:** Partial
**Code:** `internal/config/`, `internal/server/launch.go`, `internal/runtime/`, `ui/src/features/settings/` · **Journeys:** —
**Absorbed:** —

## 1. Purpose

Each AgentDeck project needs an AgentDeck-owned place where its agents can leave and reuse working
material: feature specifications, implementation guides, project-specific instructions, research,
test harnesses, and validation results. That material is project-scoped and local to the user's
machine, but intentionally lives outside the project working tree so it cannot become an accidental
repository change or commit.

## 2. Behavior

- **R1** `(planned)` — Every project has one stable shared-resources directory at
  `$AGENTDECK_HOME/project-resources/{project-id}/`. It is keyed by the immutable project id, not
  by the display title or `cwd`, so renaming a project title or moving its repository leaves the
  resource location unchanged.
- **R2** `(planned)` — AgentDeck creates the directory when a project is created and also creates
  it lazily for pre-existing projects before their first launch. The directory is empty by default;
  AgentDeck does not seed, scan, index, synchronize, or otherwise interpret its contents.
- **R3** `(planned)` — Every new agent launch receives the canonical absolute resource-directory
  path as `AGENTDECK_PROJECT_RESOURCES` and an explicit composed instruction that the directory is
  the project’s shared place for agent-created material, is outside the repository, and may be read
  or written by project agents. It is also included once in the agent's additional accessible
  directories. The agent’s working directory remains the configured project `cwd`.
- **R4** `(planned)` — The project Settings view shows the resource-directory path as a copyable,
  read-only value and explains that it is outside the repository. It does not expose a control for
  choosing a different location in this change.
- **R5** `(planned)` — Deleting a project definition never deletes its shared-resources directory.
  AgentDeck reports the retained path in the successful delete response/UI confirmation so the user
  can remove it deliberately if desired. Recreating a project with a different id creates a new,
  empty directory; it never adopts a directory based only on title or `cwd`.

## 3. States & transitions

- **R6** `(planned)` — Project creation validates and ensures the corresponding resource directory
  before committing the project definition. A failure to create the directory fails the create
  request without leaving a project definition behind. Launch similarly ensures the directory before
  process start; failure prevents the launch and names the path.
- **R7** `(planned)` — The resource path is included in the frozen additional-directory and prompt
  composition of a running or archived agent. Editing project metadata cannot redirect an existing
  agent; every future launch uses the stable path for its project id.

## 4. Edge cases & errors

- **R8** `(planned)` — Project ids pass the existing slug validation before a resource path is
  constructed. A resource-path component cannot contain separators, dots, or another project’s id.
- **R9** `(planned)` — An existing directory is reused. A missing, non-directory, unreadable, or
  unwritable target produces an actionable error and AgentDeck must not launch an agent that was not
  told a usable shared location.
- **R10** `(planned)` — Resource contents are opaque user data: they are never returned by the
  dashboard API, emitted over SSE, included in transcripts, or logged merely because the directory
  exists. Agents remain responsible for any secrets they choose to place there under the ordinary
  local-machine trust boundary.

## 5. Acceptance criteria

- **A1** `(planned)` — Creating a project produces an owner-only empty directory under a test
  AgentDeck home, and launching a pre-existing project creates the same stable directory lazily.
  *Verified by:* config/server integration tests.
- **A2** `(planned)` — A launched fake agent sees the canonical path in
  `AGENTDECK_PROJECT_RESOURCES` and in its composed instruction while its `cwd` remains the project
  working directory. *Verified by:* launch-composition tests.
- **A3** `(planned)` — Project title/`cwd` edits do not change the resource path; deleting the
  project leaves contents intact and a new project id does not reuse them. *Verified by:* project
  CRUD/resource-lifecycle tests.
- **A4** `(planned)` — Invalid ids and unusable filesystem targets fail before process launch; the
  directory and its files remain absent from list/API/SSE/log payloads. *Verified by:* adversarial
  path/mode and server-response tests.
- **A5** `(planned)` — Settings displays and copies the project resource path but offers no mutable
  location field. *Verified by:* Settings UI tests and a usability journey.

## 6. Deviations & open decisions

- This is planned only. Existing AgentDeck projects currently have no AgentDeck-owned shared
  resources directory and agent launches receive no such path or instruction.
- Retention after project deletion is intentional: project resources can contain useful work and
  are not safe to erase as a side effect of removing configuration. A future explicit cleanup tool
  may be designed separately.
- Repository-resident directories (including hidden or ignored directories), cloud synchronization,
  content browsing/search, access control beyond the current same-user model, and configurable
  locations are out of scope.

## 7. Traceability

- Project identity/configuration: FS-00.R6, FS-04.R5–R7 and FS-04.R25–R28.
- Launch composition and frozen configuration: FS-00.R10, FS-01.R3/R10/R12.
- Persistence and filesystem safeguards: TS-02.R1/R3/R5 and TS-05.R5–R7.
