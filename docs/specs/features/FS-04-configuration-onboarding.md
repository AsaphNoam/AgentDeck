# FS-04 — Configuration & Onboarding

**Status:** Current
**Code:** `internal/config/`, `internal/server/config_handlers.go`, `ui/src/features/settings/`, `ui/src/features/onboarding/` · **Journeys:** J2, J9
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
  integers 0–255 (display accent); `cwd` is the working directory (a leading `~` is expanded at
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

### 2.7 Onboarding wizard

- **R16.** When the computed config is not satisfied and `onboarding_complete` is not set, the
  dashboard is replaced by a modal wizard that cannot be dismissed by clicking outside or pressing
  Escape. Its steps are **Backend → Project → Config → Launch**. The only in-wizard dismissal is the
  explicit Skip setup control (R32).
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
- **R32.** The wizard offers an explicit **Skip setup** control, present on every step, that lets a
  user leave onboarding without launching an agent. Activating it writes `onboarding_complete: true`
  via `PUT /api/config` and, on success, dismisses the wizard to the dashboard. Because
  `onboarding_complete` forces `satisfied` true (R22), a skipped user is not re-gated by the
  computed steps and can finish configuration later in Settings — the dashboard may render with no
  backend whose credentials pass. If the `onboarding_complete` write fails, the wizard stays open
  and surfaces the error (as in R29), so skipping is never silently claimed.

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
- **R29.** In the Launch step, if the agent launches but the follow-up `onboarding_complete` write
  fails, the wizard stays visible, keeps the launched agent, and surfaces the write error (it does
  not silently claim completion).
- **R30.** A `cwd` that does not exist yet is accepted on save with a `cwd_not_found` warning shown
  next to the field (R7); the seeded `my-app` project's missing `cwd` is only explained after a
  launch against it fails (see Deviations).

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
- **A10.** The onboarding wizard walks a fresh install from backend credentials → project → first
  running agent, gating the dashboard throughout, including across a false→true config refresh
  while Project is visible. *Verified:* `ui/src/features/onboarding/OnboardingGate.test.tsx` and
  journey J2.
- **A11.** Creating a project without an id derives a valid, immutable `slug(title)-<timestamp>` id
  from the title; an explicitly supplied id is still honored and validated. *Verified:*
  `TestGenerateProjectID`, `TestProjectsAutoGeneratedID`.
- **A12.** The wizard's Skip setup control writes `onboarding_complete` and dismisses to the
  dashboard without launching an agent; a failed write keeps the wizard open with the error
  surfaced. *Verified:* `ui/src/features/onboarding/OnboardingGate.test.tsx` (skip dismisses to
  dashboard / skip write-failure keeps wizard) and journey J2.

## 6. Deviations & open decisions

- **Prompt-based confirmation UI.** The delete-in-use flow
  and other mutating confirmations use the browser's native `confirm()`/`prompt()` rather than
  dedicated dialogs, and an invalid seeded-project `cwd` is explained only after a launch fails
  rather than by preflight. This is the recorded "Immediate/prompt-based UI" product choice; reverse
  by adding dedicated dialogs and stricter preflight.
- **Federation Config step scope.** The wizard's Config step and Settings' config-source panel are
  the entry points to Claude/Codex federation, which has its own gated/deferred behavior (e.g.
  detached import returns `501`). Those specifics are owned by **FS-08**, not this spec.
- The onboarding role/project steps are effectively always pre-satisfied on a fresh install because
  roles and a project seed unconditionally (R15); the operative gate is the backend credential check.
  This is intended, not a mismatch.

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
  `ProjectForm`, `BackendsEditor`), `ui/src/features/onboarding/` (`OnboardingGate`,
  `OnboardingWizard`, `steps/`).
- **Key regression tests:** `internal/server/config_handlers_test.go`,
  `internal/server/config_endpoint_test.go`, `internal/config/config_test.go`
  (`TestSeedIfAbsentNoClobber`), `ui/src/features/onboarding/OnboardingGate.test.tsx`.
