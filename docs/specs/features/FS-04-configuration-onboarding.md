# FS-04 — Configuration & Onboarding

**Status:** Partial
**Code:** `internal/config/`, `internal/server/config_handlers.go`, `internal/server/directory_picker.go`, `ui/src/features/settings/`, `ui/src/features/onboarding/` · **Journeys:** J2, J9
**Absorbed:** [`phase-3-config-onboarding.md`](../../archive/phases/phase-3-config-onboarding.md)

## 1. Purpose

AgentDeck is configured by small, hand-editable JSON files under `~/.agentdeck/` (or
`$AGENTDECK_HOME`): `roles/{role}.json`, `projects/{project}.json`, `backends.json`, `config.json`,
`layout.json`. This spec governs the Settings UI and REST surface that edit those files as a
convenience over the same on-disk shapes, plus the first-run onboarding wizard that gates the
dashboard until a minimum viable configuration exists. Direct JSON editing stays valid at all times;
the UI never becomes the sole writer of config. Backend/model catalog detail and credential-check
semantics live in **FS-09**; Claude/Codex configuration federation lives in **FS-08**.

## 2. Behavior

### 2.1 Roles

- **R1.** A role is `{title, system_prompt, skip_permissions}`. `skip_permissions` is tri-state:
  `null` = inherit the global `config.json` `skip_permissions`; `true` = always skip; `false` =
  always prompt. The API and UI preserve the null/true/false distinction (never coerce null to
  false).
- **R2.** `GET /api/roles` lists roles; `POST /api/roles` creates one; `PUT /api/roles/{role}`
  replaces one; `DELETE /api/roles/{role}` removes one. The Settings → Roles tab drives all four.
- **R3.** A role id is a slug matching `^[a-z0-9][a-z0-9-]{0,62}$`. The id is fixed at creation and
  cannot be renamed through the UI or a `PUT` (a `PUT` body id, if present, must equal the path id).
- **R4.** `title` is required and ≤ 120 characters; `system_prompt` may be empty. The role-edit form
  states that "editing a role affects future launches only."

### 2.2 Projects

- **R5.** A project is `{title, color, cwd, add_dirs, context_prompt}`. `color` is an RGB triple of
  integers 0–255 (display accent; first-party selection is constrained to the six presets in R39);
  `cwd` is the working directory (a leading `~` is expanded at
  launch); `add_dirs` is a list of extra accessible directories; `context_prompt` is injected into
  every agent launched in the project.
- **R6.** `GET/POST /api/projects`, `PUT /api/projects/{project}`, `DELETE /api/projects/{project}`
  mirror the role CRUD surface, driven by the Settings → Projects tab. Project ids follow the same
  slug rule and immutability as role ids (R3). Unlike roles, a project id is normally **not** typed
  by the user — it is server-derived from the title (R31).
- **R31.** On `POST /api/projects` without a `project` id (empty or absent), the server derives one
  as `slug(title)-<timestamp>`: `slug(title)` lowercases the title and collapses every run of
  non-`[a-z0-9]` characters to a single hyphen (leading/trailing hyphens trimmed, base truncated so
  the whole id satisfies R3), and `<timestamp>` is the local creation time formatted
  `YYYYMMDDThhmmssZ` lowercased — e.g. title `AgentDeck Demo` → `agentdeck-demo-20260714t202825z`.
  An explicitly supplied, valid `project` id is still honored and validated (R6), so API/CLI callers
  keep full control. The Settings and onboarding project forms no longer expose an id field; they
  always rely on server derivation. A derived id is immutable exactly like a supplied one (R3).
- **R7.** `title` is required (≤ 120), `cwd` is required, and each color channel must be 0–255. A
  `cwd` that does not exist on disk is a **non-blocking warning** (`cwd_not_found`), returned
  alongside a successful save — not a validation error.
- **R35.** A project additionally has an `archived` boolean, defaulting to `false` when
  absent from a newly created or existing hand-edited project file. Setting it to `true` preserves
  the project's immutable id, workspace configuration, resources, and every agent/session record;
  it moves the project and all of its agents together between the active and archived dashboard
  collections (FS-02.R29/R33). An active project can also have individually archived agents, in
  which case it remains in the active dashboard and appears in Archive as their parent.
  An archived project cannot host a running or newly launched agent; it must be reactivated before
  any of its agents can be restored or resumed.
  The project CRUD response and Settings surface expose the field, and either state can be changed
  without re-creating the project.
- **R36.** An archived project is ineligible wherever a project is selected for a new
  process. A stored `default_project` may continue to name an archived project as a dormant
  preference, but the New Agent modal, onboarding Launch step, pipeline run setup, and AgentDecker
  builder list only configured active projects and preselect the default only when it is active.
  Direct JSON edits that archive the default therefore do not create a launch trap; restoring that
  project makes the preference eligible again without rewriting `config.json`. `project.done` in
  R21 requires at least one configured active project. An explicit API/CLI submission naming an
  archived project is rejected under TS-03.R20.
- **R39** — First-party project color selection is constrained to a fixed palette of six
  predetermined accents (the canonical set and values are defined once in TS-08.R37). Each preset is
  stored and transported as its RGB triple exactly as R5/R7 define, so the persisted shape,
  `/api/projects` payloads, and channel validation (R7) are unchanged and any historically stored
  triple stays valid. Every first-party color entry point — the Settings → Projects form, the
  onboarding project step, and the dashboard project context menu (FS-02.R39) — offers only these six
  presets as selectable swatches rather than free-form channel inputs, and a newly created project
  defaults to the palette's first accent. The API continues to accept any in-range triple from
  non-UI callers; no server-side enum enforcement is added, preserving compatibility.
- **R42** — Every first-party project form that asks for a user-selected project
  directory provides a **Browse** action: Settings create/edit covers both the primary `cwd` and
  each new `add_dirs` entry, while onboarding covers its primary `cwd`. Browse opens the standard
  macOS folder picker. Choosing a directory replaces
  `cwd` or fills the pending additional-directory entry with that directory's absolute path;
  confirming the project form or explicitly adding the pending additional directory remains the
  only operation that changes the form, and confirming the project form remains the only operation
  that saves project configuration. Canceling the picker, closing it, or a browse failure leaves
  the current field values and project configuration unchanged. Path fields remain directly
  editable, including support for the existing leading `~` form, so browsing is a convenience
  rather than a new requirement or storage format.

### 2.3 Backends & models editing surface

- **R8.** The Settings → Backends tab edits the whole `backends.json` document and saves it with a
  single `PUT /api/backends`. It adds/removes backends, adds/removes models, edits backend-level and
  per-model env pairs, and picks exactly one default backend (radio) and one default model per
  backend (radio). Sensitive env values (keys matching `KEY|TOKEN|SECRET`) render masked with a
  reveal toggle.
- **R9.** Saving normalizes the document (auto-promoting a sole default where one is missing) and
  returns per-backend credential results. The catalog shape, validation invariants, credential-check
  semantics, and per-backend capabilities are specified in **FS-09**; this spec only asserts that the
  Settings UI is the editing front-end to that surface.
- **R40** — **Add backend** opens an application dialog rather than inserting an
  incomplete card into the Settings document. The person chooses the provider type first; AgentDeck
  supplies a matching editable display-name suggestion and a usable provider starter model, so a
  selected **Codex** type never begins with a confusing **Claude** identity or an invalid empty
  model. Submit uses TS-03.R23's item-scoped create operation: the server builds the starter from the
  same provider template as fresh-home seeding and adds exactly that valid backend to the durable
  catalog without submitting, saving, replacing, or discarding unrelated whole-catalog Settings
  drafts. If the catalog was empty the new backend becomes its default; otherwise the existing
  default is preserved. If another catalog mutation commits after the Settings draft loaded, its
  later complete save is rejected and leaves that draft intact for reload/reconciliation rather than
  silently deleting the newer backend.

  Claude/Codex creation also offers **Create and use my configuration**. It durably creates the
  backend, then performs FS-08.R34's normal project-free Linked connection as the same visible
  action. Validation or the initial backend write failing leaves the catalog unchanged and keeps the
  dialog open with an actionable error. If backend creation succeeds but native connection fails,
  the valid backend remains saved and unbound, the dialog shows the specific retry action, and no
  state claims it connected. Cancel or Escape before submission changes nothing; after a backend is
  saved, closing the dialog never deletes it. The new card is merged into the editor while every
  unrelated unsaved edit remains intact. Existing backend editing stays on the complete-document
  Settings save; this change does not create a separate management screen or alter a running agent's
  frozen runtime.

### 2.4 Layout & global config

- **R10.** `config.json` holds `default_project`, `default_role`, the global `skip_permissions`
  boolean, `notifications`, and non-user-editable `version`/`port`. `GET /api/config` returns the
  config plus a computed `onboarding` block (§3); `PUT /api/config` is a partial merge of the
  user-editable subset.
- **R11.** `PUT /api/config` rejects `version` and `port` with `400 immutable`. A non-empty
  `default_project`/`default_role` must reference an existing project/role, else `400 not_found`;
  this keeps the New Agent modal and onboarding from pre-selecting a dangling default.
- **R12.** `layout.json` (card order, density, group collapse) is owned by FS-02; this spec only
  notes it is one of the seeded config files (R14) governed by the same JSON-file model.
- **R38** — The user-editable global configuration adds an optional
  `appearance_skin` preference owned by FS-12. The Settings → Appearance control reads and writes it
  through the existing configuration surface. Supported values are the unskinned Core and installed
  built-in skin ids; absence means Core. The preference affects presentation only and does not alter
  launch snapshots, projects, agents, pipelines, or onboarding readiness.

### 2.5 Composition timing

- **R13.** Editing a role, project, backend, or the global config affects **new launches only**.
  A running agent and its ordinary resume/switch paths keep the frozen launch snapshot; an explicit
  federation refresh may re-resolve only the source-owned portion described by FS-08. A newly added
  role/project/backend is selectable in the New Agent modal without a server restart (config is
  read from disk on demand).

### 2.6 Seeded configuration

- **R14.** On `dashboard start`, `SeedIfAbsent` writes a default `config.json`, `backends.json`,
  `layout.json`, the six seeded roles, and one seeded project — **only for targets absent on disk**.
  It never overwrites an existing file, so hand edits and older installs are preserved while newly
  shipped seed files appear.
- **R15.** The six seeded roles are `agentdecker`, `implementer`, `reviewer`, `researcher`, `pm`,
  and `teammate`, each with `skip_permissions: null` (inherit). The seeded project is `my-app`
  (`cwd: ~/Projects/my-app`). Because roles and a project are seeded, the onboarding role and project
  steps are already satisfied on a fresh install; the backend credential check is the operative gate
  (§3).
- **R41.** Every `dashboard start` writes the structured application log by appending to
  `$AGENTDECK_HOME/dashboard.log` (normally `~/.agentdeck/dashboard.log`), whether it runs in the
  foreground or with `--detach`. Foreground starts also mirror the same records to stderr. The log
  file is owner-only (`0600`), an existing broader mode is tightened before startup continues, and
  failure to establish the persistent log prevents the dashboard from starting.

### 2.7 Onboarding wizard

- **R16 — retired 2026-07-22:** The no-exit first-run wizard was replaced by the explicit
  **Set up later** completion path in R32. The wizard remains protected from accidental outside-click
  or Escape dismissal.
- **R17.** The Backend step edits the seeded backend for the chosen type, saves via `PUT
  /api/backends`, and only advances when the returned credential status for that backend is `ok`; a
  non-`ok` status is shown inline with provider-specific, human-readable next steps (install the
  missing adapter, run guided sign-in, or add an API key) and blocks advance.
- **R18.** The Project step creates the user's first real project (preferred over the seeded
  `my-app`, whose `cwd` may not exist). Its id is server-derived (R31) and read back from the create
  response, then carried into the Launch step's default selection.
- **R19.** The Config step is the optional Claude/Codex federation entry point (FS-08). It is
  client-side and skippable; it is not tracked in the server-side onboarding flags, so a returning
  user resumes past it with Continue.
- **R20.** The Launch step launches the first agent (`POST /api/sessions`, interface `chat`) and, on
  success, sets `onboarding_complete: true` via `PUT /api/config`, then closes the wizard.

- **R43** `(planned)` — Settings expose the install-wide task concurrency budget: how many agent
  runtimes AgentDeck may bring up for dependent work at one time, defaulting to four (FS-16.R7, R21).
  It is a single positive integer, validated and persisted like every other setting on this screen,
  and it never counts or affects agents a person launches or resumes themselves.

## 3. States & transitions

- **R21.** `GET /api/config` computes an `onboarding` block with per-step `{done, detail}` for
  `backend`, `project`, and `role`, and a top-level `satisfied` = backend.done && project.done &&
  role.done. `role.done`/`project.done` mean ≥ 1 role/project exists on disk. `backend.done` means
  `backends.json` parses at version 2 with a default backend whose default model's credential check
  returns `ok`.
- **R22.** If `onboarding_complete` is already `true`, `satisfied` is forced true regardless of the
  computed steps, so a user who once completed setup is never re-gated by a later transient
  credential failure.
- **R23.** Before the wizard opens, the gate is satisfied for rendering purposes when either the
  server reports `satisfied` or `onboarding_complete` is true. Once an unsatisfied gate opens the
  wizard, that mounted wizard stays latched through Backend, Project, Config, and Launch even if a
  config poll reports `satisfied`; only successful Launch completion sets its session-local
  "dismissed" flag and closes it. Once dismissed, the dashboard renders and the wizard does not
  reappear during that mounted browser session.
- **R24.** The backend credential result feeding `backend.done` is cached for 60s and invalidated
  whenever `backends.json` is saved or `onboarding_complete` is written, so edits re-evaluate the
  gate promptly.

## 4. Edge cases & errors

- **R25.** Creating a role/project whose id already exists returns `409 already_exists`. `PUT`/`DELETE`
  on an absent id returns `404 not_found`.
- **R26.** Validation failures return `400` with the envelope `{error:"validation_failed", errors:[{field,code,message}]}`.
  A malformed request body is reported as a `bad_request` field error in the same envelope.
- **R27.** Ids that would escape the config directory (slashes, dots, encoded `%2e`, uppercase,
  whitespace) are rejected by the slug rule before any filesystem access — no path traversal reaches
  disk.
- **R28.** `DELETE` of a role/project referenced by a **running** agent returns `409 in_use` with the
  offending agent ids and a hint to retry with `?force=true`. The Settings UI surfaces this as a
  confirm dialog listing the agents; on confirm it re-issues the delete with `force=true`. `force`
  deletes only the definition — running agents keep their already-composed config and are unaffected.
  Any non-409 delete failure (offline/500) is surfaced as an error toast, never a silent no-op.
- **R37.** Settings destructive actions use core application dialogs rather than browser
  `confirm()` (FS-12.R26). Deleting a role or project, archiving a project, and the in-use (409)
  force follow-up (R28) each open a consequence-aware confirmation dialog. The in-use dialog lists
  the offending running agent ids and, on confirm, re-issues the delete with `force=true`. Cancel
  performs no action; each confirmed action issues the same request as today.
- **R29.** In the Launch step, if the agent launches but the follow-up `onboarding_complete` write
  fails, the wizard stays visible, keeps the launched agent, and surfaces the write error (it does
  not silently claim completion).
- **R30.** A `cwd` that does not exist yet is accepted on save with a `cwd_not_found` warning shown
  next to the field (R7); the seeded `my-app` project's missing `cwd` is only explained after a
  launch against it fails (see Deviations).
- **R32** — An unsatisfied onboarding wizard has an explicit **Set up later** action.
  It sets `onboarding_complete: true`, closes the wizard, and opens the dashboard without creating
  a project, changing a backend/model catalog, or launching an agent. The wizard remains modal and
  cannot be dismissed by outside click or Escape; its ordinary completion path remains **Backend →
  Project → Config → Launch**. If writing completion fails, it stays open and reports the failure.
- **R33** — The Backend step presents backend type and credential choices, but no
  editable AgentDeck model id or provider model string. It uses the chosen backend's existing
  default model; people edit model catalogs and defaults later in Settings → Backends.
- **R34** — Claude and Codex onboarding gives provider-specific guidance to sign in
  outside AgentDeck, then offers **Check again** to refresh readiness. AgentDeck does not launch,
  proxy, display, receive, or store a native sign-in flow or credential. Unready, unavailable, and
  failed readiness results leave the wizard open with retryable guidance and the Set up later action.
  Codex retains OpenAI API-key configuration as an alternative to native sign-in; it is not required
  for a successfully signed-in Codex CLI.

## 5. Acceptance criteria

- **A1.** Role create/edit/delete round-trips through the API preserving all fields including
  tri-state `skip_permissions`. *Verified:* `TestRolesCRUDRoundTrip`.
- **A2.** A newly created role is selectable in the New Agent flow without a server restart.
  *Verified:* `TestSelectableWithoutRestart` (and journey J9).
- **A3.** Project create/edit/delete round-trips; an unknown `cwd` yields a warning, not an error.
  *Verified:* `TestProjectsCRUDRoundTrip`, `TestProjectsCwdNotFoundIsWarningNotError`.
- **A4.** Invalid input (missing title, bad slug, out-of-range color) returns the `400` field-error
  envelope. *Verified:* `TestRolesValidationFailures`, `TestProjectsValidationFailures`.
- **A5.** Path-traversal ids are rejected before disk access, including percent-encoded dots.
  *Verified:* `TestPathTraversalRejected`, `TestPathTraversalEncodedDots`.
- **A6.** Deleting a role/project in use returns `409 in_use` with agent ids; `force=true` completes
  the delete without affecting running agents. *Verified:* `TestRolesInUseGuard`,
  `TestProjectsInUseGuard`.
- **A7.** The six roles and the `my-app` project seed on a fresh home and are never clobbered on a
  populated one. *Verified:* `TestRolesSeeded`, `TestProjectsSeeded`, `TestBackendsSeeded`,
  `TestSeedIfAbsentNoClobber`.
- **A8.** The onboarding gate is unsatisfied on an empty store, satisfied when all steps pass, held
  open by bad backend credentials, and overridden by `onboarding_complete`. *Verified:*
  `TestGetConfigEmptyStoreNotSatisfied`, `TestGetConfigSatisfiedWhenAllStepsDone`,
  `TestGetConfigBadCredsMakesBackendNotDone`, `TestGetConfigOnboardingCompleteOverridesGate`.
- **A9.** `PUT /api/config` merges only the user-editable subset, rejects `version`/`port`, and
  persists notification settings. *Verified:* `TestPutConfigMergesFields`,
  `TestPutConfigRejectsImmutableFields`, `TestPutConfigPersistsNotificationSettings`.
- **A10 — retired 2026-07-22:** Replaced by A12–A14, which cover both the ordinary guided launch
  and explicit Set up later paths.
- **A11.** Creating a project without an id derives a valid, immutable `slug(title)-<timestamp>` id
  from the title; an explicitly supplied id is still honored and validated. *Verified:*
  `TestGenerateProjectID`, `TestProjectsAutoGeneratedID`.
- **A12** — The ordinary onboarding path keeps the wizard mounted across a false→true
  config refresh and walks a fresh install from ready backend credentials through project creation
  to a first running agent. *Verified:* `ui/src/features/onboarding/OnboardingGate.test.tsx` and
  journey J2.
- **A13** — Set up later completes onboarding and reveals the dashboard without a
  project create, backend save, catalog/default-model change, or session launch; a completion-write
  failure remains visible and retryable. *Verified:* `OnboardingGate.test.tsx` and journey J2.
- **A14** — Claude/Codex provider-specific sign-in guidance and Check again safely
  report unready/unavailable/failed/ready states; Codex can instead validate an API key. The Backend
  step contains neither an editable model id nor provider model string. *Verified:* `BackendStep.test.tsx`,
  credential-check tests, and journey J2 with a fake provider.
- **A15.** Project archive state round-trips through the project config API and survives
  restart; an older project file without the field is active by default. — config/server/UI regressions.
- **A16.** When `default_project` names an archived project, launch selectors show only
  active projects with no archived default preselected, onboarding reports no project ready when no
  active project exists, and restoring the default makes it eligible again without changing
  `config.json`. — config/readiness and launch-selector regressions; J2, J5, J14.
- **A17.** (R37) The role/project delete, force-delete-in-use, and project-archive dialogs render
  their consequence and agent list, issue the same requests on confirm, and no-op on Cancel. —
  `RolesEditor`/`ProjectsEditor` component tests and the FS-12.A8 guard.
- **A18** — (R38) The appearance preference round-trips through the global
  configuration surface, survives restart, defaults to Core when absent, and rejects or safely
  recovers from unsupported values as required by FS-12.R32. — configuration/API regressions and
  FS-12.A9–A11.
- **A19** — (R39) The Settings/onboarding project forms and the dashboard project menu
  present the six-preset swatch picker with no free-form channel inputs; selecting a preset persists
  its RGB triple through the existing project API and round-trips unchanged, and a newly created
  project defaults to the first preset. — `ProjectForm` and `ProjectDashboard` component tests; J2, J5.
- **A20** — (R40) Add backend is cancellable before submission; choosing a type produces
  only that provider's matching suggested name and canonical valid starter model; and item-scoped
  create adds exactly one backend without persisting, replacing, or discarding an unrelated dirty
  Settings draft. **Create and use my configuration** returns a connected Claude/Codex backend only
  after FS-08.A11 succeeds. A connection failure instead leaves the new backend saved, unbound,
  visible, and retryable; closing after that failure preserves it. *Verify by* dialog/editor tests
  for dirty-draft preservation, validation/initial-write failure, cancel, create-only success,
  create-and-connect success, create-and-connect failure/retry, and ordinary later edit/delete.
- **A21.** (R41) Foreground logging appends each JSON record to `dashboard.log` while mirroring it to
  the terminal, repeated starts preserve prior records, and the file is created or tightened to
  `0600`; detached-child logging uses its redirected stderr as the single file sink so records are
  not duplicated. — `TestDashboardLoggerForegroundPersistsAndMirrors`,
  `TestDashboardLoggerAppendsAndTightensPermissions`, `TestDashboardLoggerDaemonUsesRedirectedStderr`.
- **A22.** (R42) Settings project create/edit can browse for `cwd` and each new
  `add_dirs` entry, onboarding can browse for `cwd`, and every surface can select, cancel without
  changing the existing value, and still type or edit a path manually. A selected additional
  directory is not part of the form until Add is invoked, and selection alone never writes project
  configuration. — project-form and onboarding component tests plus the project-creation journeys
  in J2 and J9.

- **A23** `(planned)` (R43) — The task concurrency budget round-trips through settings with its
  default of four, rejects a non-positive value without persisting it, and is read back by the
  dispatcher: settings UI and API tests.

## 6. Deviations & open decisions

- **Invalid seeded-project `cwd`.** An invalid seeded-project `cwd` is still explained only after a
  launch fails rather than by preflight; reverse that part by adding a stricter preflight.
- **Federation Config step scope.** The wizard's Config step and Settings' config-source panel are
  the entry points to Claude/Codex federation, which has its own gated/deferred behavior (e.g.
  detached import returns `501`). Those specifics are owned by **FS-08**, not this spec.
- The onboarding role/project steps are effectively always pre-satisfied on a fresh install because
  roles and a project seed unconditionally (R15); the operative gate is the backend credential check.
  This is intended, not a mismatch.
- **Confirmed archived-project boundary.** R35–R36 preserve project data and a dormant archived
  default while excluding archived projects from launch selectors. Launch, resume, restore, and
  pipeline rejection use TS-03.R20; storage and transition ordering use TS-02.R20.

## 7. Traceability

- **Config types & seeding:** `internal/config/types.go`, `internal/config/seed.go`
  (`SeedIfAbsent`, `seedRoles`, `seedProject`, `DefaultConfig`).
- **Validation & id derivation:** `internal/config/validate.go` (`ValidSlug`, `ValidateRole`,
  `ValidateProject`, `GenerateProjectID`); `handlePostProject` in
  `internal/server/config_handlers.go` derives the id when none is supplied (R31).
- **Handlers:** `internal/server/config_handlers.go` (role/project/backends/config handlers,
  `computeOnboarding`, `computeBackendStep`, `cachedCredCheck`), `onboardingCacheTTL` in
  `internal/server/server.go`.
- **UI:** `ui/src/features/settings/` (`SettingsPage`, `RolesEditor`, `RoleForm`, `ProjectsEditor`,
  `ProjectForm`, `BackendsEditor`, `AppearanceEditor`), `ui/src/features/onboarding/` (`OnboardingGate`,
  `OnboardingWizard`, `steps/`).
- **Archive eligibility:** `internal/server/config_handlers.go` (`computeOnboarding`),
  `ui/src/features/settings/ProjectsEditor.tsx`, `NewAgentModal.tsx`, `LaunchStep.tsx`, and
  `RunStartForm.tsx`.
- **Set up later & provider guidance:** `OnboardingWizard` owns the completion escape hatch (R32);
  `steps/BackendStep.tsx` holds the provider sign-in guidance and Check again (R33, R34), naming the
  `agentdeck auth` selectors that `internal/backend/providerauth` owns.
- **Dashboard logging (R41/A21):** `internal/cli/dashboard.go` establishes the shared persistent
  logger; `internal/cli/cli_test.go` covers foreground mirroring, append/permission behavior,
  unavailable paths, and the detached child's single redirected sink.
- **Key regression tests:** `internal/server/config_handlers_test.go`,
  `internal/server/config_endpoint_test.go`, `internal/config/config_test.go`
  (`TestSeedIfAbsentNoClobber`), `ui/src/features/onboarding/OnboardingGate.test.tsx` (A12, A13),
  `ui/src/features/onboarding/steps/BackendStep.test.tsx` (A14), and
  `ProjectsEditor.test.tsx` "shows archived state and restores the project from Settings",
  `internal/config/appconfig_test.go`, and `AppearanceEditor.test.tsx` (A18).
- **Backend creation (R40/A20):** `internal/server/backend_create.go`,
  `internal/config/seed.go` (`StarterBackend`),
  `ui/src/features/settings/AddBackendDialog.tsx`, and the merge into
  `ui/src/features/settings/BackendsEditor.tsx`; pinned by
  `internal/server/backend_create_test.go`, `internal/config/backendcreate_test.go`, and
  `BackendsEditor.test.tsx`.
- **Directory browsing (R42/A22):** one shared control,
  `ui/src/components/ui/BrowseDirectoryButton.tsx`, consumed by `ProjectForm.tsx` (cwd and the
  pending `add_dirs` entry) and `steps/ProjectStep.tsx`; `pickDirectory` in `ui/src/api/config.ts`
  over `internal/server/directory_picker.go` (TS-03.R26, TS-05.R15). Pinned by the three browse
  cases in `ProjectsEditor.test.tsx` and one in `ProjectStep.test.tsx`.
