# Phase 3 — Implementation Tech Spec: Config CRUD & Onboarding

**Mirrors:** `docs/phases/phase-3-config-onboarding.md` (phase PRD)
**Master PRD refs:** §3.2 (roles), §3.3 (projects), §3.4 (backends), F4, F5, F6, F12
**Depends on:** Phase 0 (file store, GET stubs), Phase 1 (launch endpoint + composition), Phase 2 (UI shell, agent store, SSE client)
**Status:** ready to build after Phase 2
**Audience:** the engineer implementing the phase. Every design decision is resolved here; no further design work should be required.

---

## 0. Codex review findings — address while building this phase

> Recorded 2026-06-28 from a cross-phase Codex review. Resolve each as you build the
> referenced subphase; delete the entry once implemented and verified green.

- **Advisory — force-delete UI expects 409 detail, but the delete mutation surfaces a plain error
  (§4.1 / §5).** The `DELETE /api/roles/{role}` (and projects) in-use response is a **structured 409**
  (`{ error, message, hint }`, §5 lines ~236/241) whose `hint` tells the UI to retry with
  `?force=true`. If the delete mutation hook throws a plain `Error` (message only), the structured
  detail is lost and the force-delete affordance can't distinguish 409-in-use from other failures.
  **Resolution:** the delete mutation must parse the 409 JSON body and propagate the status + structured
  detail (at least `hint`) so the UI can offer the `?force=true` retry instead of a generic error toast.

- **Advisory — the New Agent modal ignores configured defaults (§4.2).** §4.2 wants Project to default
  to `config.default_project` and role to a sensible default, but the modal as specced doesn't read
  `config.default_role` / `config.default_project` (from `GET /api/config`) to preselect — it falls back
  to first-available / hardcoded `implementer`. **Resolution:** the modal (and the wizard's LaunchStep)
  must read `config.default_role` and `config.default_project` and preselect them when present, only
  falling back to first-available / `implementer` when unset.

---

## 1. Overview & scope recap

Phase 3 turns AgentDeck from a hand-edited-JSON tool into a configurable one and gives a fresh install a guided path to its first running agent. We add **write paths** over the file store (Phase 0 only added the `GET` reads), Settings UIs for roles / projects / backends, the full **New Agent modal** (the UI front-end to Phase 1's `POST /api/sessions`), and a **first-run onboarding wizard** that gates the dashboard until a minimum viable config exists.

### In scope

- **F5 — Roles CRUD:** create / edit / delete `roles/{role}.json` (`title`, `system_prompt`, `skip_permissions`).
- **F5 — Projects CRUD:** create / edit / delete `projects/{project}.json` (`title`, `color`, `cwd`, `add_dirs`, `context_prompt`).
- **F6 — Backends/models config:** `PUT /api/backends` over the whole `backends.json` document (`version: 2`); per-model `env` overrides; default-backend and default-model-per-backend invariants enforced on save; credential validation on save.
- **F4 (UI) — New Agent modal:** name (auto-suggested), role, project, backend, model (filtered to backend), interface; submits to the Phase 1 `POST /api/sessions`.
- **F12 — Onboarding wizard:** blocks the dashboard on a fresh/empty `~/.agentdeck/` and walks backend → project → first agent; never reappears once min config exists.
- **Read-from-disk-on-demand:** a newly added role/project/model is selectable in the New Agent modal **without a server restart**.

### Out of scope (do not build here)

- **Switch-runtime on a live agent (F7) — Phase 6.** Editing a role/project/backend definition is *not* switching a live agent; this spec only changes definitions and affects **future** launches. Existing agents keep their launch-time composed config until restarted.
- Archive / resume (Phase 4), messaging (Phase 5), terminal runtime (Phase 6), per-type notification mute (Phase 5 — see §10).
- Changing the `POST /api/sessions` launch contract itself (owned by Phase 1). We only call it.

---

## 2. Technology choices

### 2.1 Frontend — forms & validation

- **Form state + validation:** **React Hook Form** (`react-hook-form`) + **Zod** (`zod`) via `@hookform/resolvers/zod`. Rationale: RHF is uncontrolled-by-default (cheap re-renders for the larger backends editor), Zod gives one schema reused for (a) client-side validation and (b) typed parsing of API responses. Define each entity's Zod schema once in `web/src/schemas/` and infer the TS type from it (`z.infer`).
- **Modals / dialogs / wizard shell:** **Radix UI primitives** (`@radix-ui/react-dialog`, `@radix-ui/react-select`, `@radix-ui/react-tabs`). Headless + accessible; the project already renders in a local browser with no design-system dependency. The onboarding wizard is a non-dismissible `Dialog` (no overlay-click / Esc close).
- **Data fetching / cache:** **TanStack Query** (`@tanstack/react-query`). Config lists (roles, projects, backends, `/api/config`) are server state with cache invalidation on mutation — exactly its model. After any successful mutation, invalidate the relevant query key so the New Agent modal and Settings re-read from disk (this is the mechanism that satisfies "selectable without restart" on the client; see §3.6 for the server half).
- **Color picker (project accent):** a minimal RGB input trio (three number inputs 0–255) plus a swatch preview — no extra dependency. Stored as `[r,g,b]` per master PRD §3.3.
- **No new global state lib.** Reuse the Phase 2 agent store; config state lives in TanStack Query cache.

### 2.2 Backend — validation helpers (Go)

- **Struct validation:** hand-written validators in a new `internal/config/validate.go`. The rule set is small and invariant-heavy (defaults, references, slug format); a generic validation library buys little. Each validator returns a `[]FieldError{ Field, Code, Message }` (see §5.6 error shape).
- **Slug rule (role/project ids):** `^[a-z0-9][a-z0-9-]{0,62}$`. The id is the filename stem; reject anything that would escape the directory (no `/`, `.`, `..`, whitespace, uppercase). Shared helper `config.ValidSlug(s string) bool`.
- **Atomic writes:** reuse the Phase 0 file-store write-temp-then-rename. CRUD handlers must go through the file-store package, never `os.WriteFile` directly.
- **Credential validation (per backend type):** a small `internal/backend/credcheck` package exposing `Check(ctx, backendType string, model ModelSpec, env map[string]string) CredResult`. `CredResult{ Status: "ok"|"failed"|"skipped", Detail string }`. Strategy is **auth ping, not a billed trial request** — see §3.5 for the concrete probe per backend type. The check is bounded by a context timeout (default **6s**) and never blocks the save fatally (see §3.5 + §6).

---

## 3. Backend design

All write paths live behind the existing Go HTTP server (`127.0.0.1` only). New package layout:

```
internal/
  config/
    roles.go        role CRUD handlers
    projects.go     project CRUD handlers
    backends.go     backends PUT handler + invariant enforcement
    config.go       GET /api/config min-viable-config check
    validate.go     shared validators, slug rule, FieldError type
  backend/
    credcheck/
      credcheck.go  Check(...) dispatch by backend type
      claude.go     claude-acp probe
      codex.go      codex-acp probe
```

### 3.1 Roles CRUD (write paths over `roles/`)

Schema (master PRD §3.2), file `roles/{role}.json`:

```jsonc
{ "title": "Reviewer", "system_prompt": "...", "skip_permissions": null }
```

- **POST `/api/roles`** — body includes the desired `role` id (slug) plus fields. Validate slug; reject if `roles/{role}.json` already exists (409). `skip_permissions` is `true | false | null` (null = inherit global `config.skip_permissions`). Write via file store. Return 201 + the stored object.
- **PUT `/api/roles/{role}`** — full replace of the role's fields (the id in the path is canonical; ignore any `role` in the body, or 400 on mismatch). 404 if absent. Validate, write, return 200 + object.
- **DELETE `/api/roles/{role}`** — 404 if absent. **In-use guard** (§6): if any agent in `running/` references this role, reject with 409 unless `?force=true`; with force, delete the definition and leave running agents untouched (they already composed their config at launch). Return 204 on success.

Validation rules: `title` non-empty (≤ 120 chars); `system_prompt` may be empty string but not missing; `skip_permissions ∈ {true,false,null}`.

### 3.2 Projects CRUD (write paths over `projects/`)

Schema (master PRD §3.3), file `projects/{project}.json`:

```jsonc
{ "title": "My App", "color": [100,180,255], "cwd": "~/Projects/my-app", "add_dirs": [], "context_prompt": "..." }
```

- **POST `/api/projects`** — body includes `project` id (slug) + fields. 409 if exists. Validate, write, 201 + object.
- **PUT `/api/projects/{p}`** — full replace, 404 if absent, 200 + object.
- **DELETE `/api/projects/{p}`** — same in-use guard as roles (409 if referenced by a `running/` agent unless `?force=true`), 204.

Validation rules:
- `title` non-empty (≤ 120).
- `color` exactly 3 ints, each 0–255 (default `[128,128,128]` if omitted).
- `cwd` non-empty; store the user-entered string verbatim (keep `~` — expansion happens at launch composition in Phase 1, not here). Validate that, after `~` expansion, the path exists and is a directory; if not, return a **warning-level** field error code `cwd_not_found` that the UI surfaces but the save **still succeeds** (a project may point at a dir created later). This is the one validator that warns rather than blocks.
- `add_dirs` array of strings (may be empty); same `~`-verbatim treatment, no existence check.
- `context_prompt` may be empty.

### 3.3 Backends config — `PUT /api/backends` with invariants

`backends.json` is a single document (master PRD §3.4, `version: 2`). The PUT replaces the **entire** document (the UI always sends the whole thing — there is no per-backend endpoint). On save, enforce these invariants server-side (do not trust the client):

1. **`version` must be `2`.** Reject otherwise (400, code `unsupported_version`).
2. **Exactly one default backend.** Across `backends[*].default == true` there must be exactly one. If the client sends zero, and there is ≥1 backend, **auto-promote** the first backend (stable map-key order, sorted) to default and return it in the response (not an error — onboarding sends one backend and may omit the flag). If the client sends more than one, return 400 (`multiple_default_backends`).
3. **Exactly one default model per backend.** For each backend, `default_model` must be a non-empty key present in that backend's `models`. If `default_model` is missing/empty and the backend has ≥1 model, auto-promote the first model key (sorted) and return it. If `default_model` names a non-existent model, 400 (`unknown_default_model`).
4. **Every backend has ≥1 model.** A backend with an empty `models` map is rejected (400, `backend_without_models`).
5. **`type` must be a known backend type:** `claude-acp` or `codex-acp`. Unknown type → 400 (`unknown_backend_type`).
6. **Model `model` field non-empty** (the provider model string passed to the CLI).

**Per-model env override semantics (composition contract for Phase 1):** the effective env for a launch is `merge(backend.env, model.env)` where `model.env` keys **override** `backend.env` keys (shallow merge, model wins). This spec only *stores* `env`; Phase 1 composes it at launch. We document and unit-test the merge here so backend storage and launch composition agree. `env` values are stored **as-is in plaintext** in `backends.json` (consistent with the local-first, no-secret-vault design of the master PRD — the file is `chmod 600` by the file store; see §6).

Handler flow for `PUT /api/backends`:
1. Parse body into `BackendsConfig` struct.
2. Run structural validation (invariants 1–6). On failure → 400 with `FieldError[]`.
3. Apply auto-promotions (defaults) → this is the *normalized* document.
4. Run credential validation (§3.5) for each `(backend, default_model)` pair — and any model whose `env` changed vs. the on-disk version, to validate new keys. Collect per-backend `CredResult`.
5. **Save policy:** write the normalized document **regardless of cred-check outcome** (a user may want to save now and fix creds later), but include the `credentials` results in the response so the UI surfaces failures. Onboarding (§4.3) treats a `failed`/`skipped` default-model cred-check as "not yet satisfied" and will not advance — but the bytes are still persisted so the user doesn't lose work.
6. Return 200 + normalized document + `credentials` map.

### 3.4 Read-from-disk-on-demand (no restart)

The server **must not** cache roles/projects/backends in memory across requests for the CRUD + launch path. Every `GET /api/roles`, `GET /api/projects`, `GET /api/backends`, and every `POST /api/sessions` launch composition reads the relevant directory/file from disk at request time via the file store. This is what makes a newly created role/project/model immediately selectable (acceptance criterion). If Phase 0 introduced any in-memory config cache, this phase replaces reads with disk reads or invalidates the cache on every write. The client half (TanStack Query invalidation) is in §2.1.

### 3.5 Credential validation strategy (resolved per backend type)

**Decision: auth ping, not a trial completion request.** A trial request bills the user and is slow; an auth/whoami-style ping is cheap and answers the real question ("are these credentials accepted?"). Each probe runs the backend's CLI in a non-interactive auth-check mode with the candidate `env` injected, under a 6s context timeout, and maps exit/output to `CredResult`.

- **`claude-acp` (Claude Code):** invoke the Claude Code CLI in a non-interactive auth-status mode with the composed env. Concrete probe: run the CLI's auth/whoami subcommand (e.g. `claude auth status` / equivalent non-interactive flag for the pinned version) and treat exit code 0 + non-error output as `ok`. If the CLI binary is not found on PATH → `skipped` with detail `cli_not_installed`. If it runs but reports unauthenticated → `failed` with the CLI's message. (Claude Code commonly authenticates via an interactive login session rather than an env key; in that case the probe checks the *existing logged-in session* rather than a key, which is exactly the credential we need to validate.)
- **`codex-acp` (Codex):** the credential is an API key (`OPENAI_API_KEY`, optionally `OPENAI_BASE_URL`) carried in `env`. Concrete probe: a single lightweight authenticated **models-list** GET against `${OPENAI_BASE_URL:-https://api.openai.com}/v1/models` with `Authorization: Bearer $OPENAI_API_KEY`, 6s timeout. HTTP 200 → `ok`; 401/403 → `failed` (detail = `invalid_api_key`); network/DNS error or non-2xx/4xx → `skipped` (detail = the error; we don't want a flaky network to hard-block a save). This is an auth ping (no tokens billed), not a completion. If no `OPENAI_API_KEY` is present in the merged env → `skipped` (detail `no_api_key`).

`credcheck.Check` dispatches on `backend.type`; unknown types return `skipped`. The whole step is best-effort and never panics; a probe that times out yields `skipped` (detail `timeout`), not `failed`.

### 3.6 Minimum-viable-config check — `GET /api/config`

Drives onboarding gating (§4.3). Reads disk on demand and returns the user-facing config plus a computed `onboarding` block:

**Min-viable-config is satisfied iff ALL of:**
1. `backends.json` parses, `version == 2`, and has ≥1 backend with a valid `default_model` whose latest cred-check is `ok`. (The server re-runs the cred-check for the default backend's default model here, cached for ~60s to avoid hammering the probe on every dashboard poll.)
2. ≥1 project exists in `projects/`.
3. ≥1 role exists in `roles/` (Phase 0 seeds 4, so this is normally true; included for completeness on a wiped store).

The check does **not** require an agent to have ever launched — "launch the first agent" is the wizard's final step but the *gate* lifts as soon as backend+project+role exist with valid creds, so a user can reach the dashboard and launch from there. Once satisfied, the wizard never reappears (the client persists a `onboarding_complete` flag in `config.json`; see §4.3 + §6 for the interrupted-onboarding behavior).

---

## 4. Frontend design

All under `web/src/`. New routes/components plug into the Phase 2 shell.

```
web/src/
  schemas/        roleSchema.ts, projectSchema.ts, backendsSchema.ts, configSchema.ts (Zod)
  api/            config.ts (typed fetchers + mutations, wraps fetch + Zod parse)
  features/
    settings/
      SettingsPage.tsx        tabbed: Roles | Projects | Backends
      RolesEditor.tsx
      RoleForm.tsx
      ProjectsEditor.tsx
      ProjectForm.tsx
      BackendsEditor.tsx
      ModelRow.tsx
    launch/
      NewAgentModal.tsx
      useSuggestedName.ts
    onboarding/
      OnboardingGate.tsx      reads /api/config, blocks dashboard
      OnboardingWizard.tsx    3 steps
      steps/BackendStep.tsx ProjectStep.tsx LaunchStep.tsx
```

### 4.1 Settings UI structure

A `SettingsPage` with three Radix `Tabs`:

- **Roles tab (`RolesEditor`):** list of role cards (title, truncated system prompt, `skip_permissions` badge). "New role" opens `RoleForm` in a Dialog; click a row to edit; per-row delete. `RoleForm` fields: id (slug, only editable on create — read-only on edit since the filename is the id), `title`, `system_prompt` (textarea), `skip_permissions` (tri-state select: Inherit global / Always skip / Always prompt → `null|true|false`). Inline note: "Editing a role affects future launches only."
- **Projects tab (`ProjectsEditor`):** list of project rows with the color swatch. `ProjectForm` fields: id (slug, create-only), `title`, `color` (RGB trio + swatch), `cwd` (text, with a "directory not found" warning rendered from the `cwd_not_found` field warning but not blocking save), `add_dirs` (add/remove string list), `context_prompt` (textarea).
- **Backends tab (`BackendsEditor`):** edits the whole `backends.json`. Per backend: `name`, `type` (select: Claude / Codex → `claude-acp`/`codex-acp`), a "Default backend" radio (exactly one across all backends, enforced in UI and re-enforced server-side), backend-level `env` key/value editor, and a model table. Each `ModelRow`: model key, display `name`, provider `model` string, per-model `env` overrides (key/value), and a "Default model" radio scoped to that backend. A "Validate credentials" button per backend triggers the save's cred-check display; the save response's `credentials[backendId]` result renders as a pass/fail chip. Add/remove backend and add/remove model controls. Save sends the full normalized document to `PUT /api/backends`; on 200, re-render from the normalized response (so auto-promoted defaults reflect immediately) and toast any `failed` cred-checks.

All three editors invalidate their TanStack Query keys on successful mutation; the New Agent modal reads the same keys, so new entities appear without restart.

### 4.2 New Agent modal (F4 UI)

Radix `Dialog`. Fields, in order:

- **Name** — text, prefilled by `useSuggestedName(role, project)` → `"{Capitalized role}-{project}"` style auto-suggestion (e.g. `Implementer-my-app`), editable. Re-suggests when role/project change *only while the field is untouched*; once the user edits the name, stop auto-overwriting.
- **Role** — select, options from `GET /api/roles` (disk-on-demand).
- **Project** — select, options from `GET /api/projects`; defaults to `config.default_project`.
- **Backend** — select, options from `GET /api/backends`; defaults to the default backend.
- **Model** — select, **filtered to the chosen backend's `models`**; defaults to that backend's `default_model`. Resets when backend changes.
- **Interface** — segmented control: `chat` (default, selected) and `terminal`. **Terminal is rendered but disabled** with a tooltip "Available in a later release" (terminal runtime is Phase 6). Only `chat` is submittable this phase.

Submit → `POST /api/sessions { name, role, project, backend, model, interface }` (the Phase 1 contract; we add `name` if the launch endpoint accepts it, otherwise call rename after launch — confirm against Phase 1, but the modal owns the field). On success: close modal, the Phase 2 card appears via SSE `state_update`, chat is openable. On error: surface the launch error inline; do not close. The modal is reused as the wizard's final "Launch" step (§4.3) with role/project preselected.

### 4.3 Onboarding wizard (F12)

`OnboardingGate` wraps the dashboard route. On mount (and on `/api/config` cache refresh) it reads `onboarding.satisfied`:

- **If not satisfied:** render `OnboardingWizard` as a **non-dismissible** full-screen Dialog over a blurred/blocked dashboard. No Esc, no overlay-click close. The dashboard is unreachable until the gate lifts.
- **If satisfied:** render the dashboard; the wizard never mounts.

Three steps, each gated on the previous:

1. **BackendStep** — embeds a focused single-backend form (subset of `BackendsEditor`): pick type (Claude / Codex), enter `env` creds if needed, set default model. "Validate & continue" runs `PUT /api/backends` and only advances when the response's default-model cred-check is `ok`. A `failed`/`skipped` result blocks advance and shows the detail; the document is still saved (so a reload resumes here, not from scratch).
2. **ProjectStep** — create the first project (subset of `ProjectForm`: title, cwd, optional context). `POST /api/projects`. `cwd_not_found` is a non-blocking warning here too (the user may be setting up a dir). Advances on 201.
3. **LaunchStep** — the New Agent modal body with backend/project preselected and role defaulted to `implementer`. Launch → `POST /api/sessions`. On success the wizard sets `onboarding_complete: true` in `config.json` (via a small `PATCH`/`PUT /api/config` write — see §5.5) and dismisses; the dashboard mounts with the new agent's card.

**"Never reappears":** the gate is satisfied when **either** `config.onboarding_complete == true` **or** the computed min-viable-config (backend+project+role with ok creds) holds. The explicit `onboarding_complete` flag covers the edge where a user later deletes their only project — we don't want to trap a returning user back in the wizard; but the computed check covers a truly fresh store where the flag is absent. (See §6 for the partial-interruption case.)

---

## 5. API contracts

All under `http://127.0.0.1:{port}/api`. JSON request/response. `Content-Type: application/json`.

### 5.1 Roles

**`POST /api/roles`**
```jsonc
// request
{ "role": "security-reviewer", "title": "Security Reviewer",
  "system_prompt": "Audit for vulns.", "skip_permissions": false }
// 201 response
{ "role": "security-reviewer", "title": "Security Reviewer",
  "system_prompt": "Audit for vulns.", "skip_permissions": false }
```
- 400 invalid slug / missing title (validation error shape §5.6)
- 409 `{ "error": "already_exists", "message": "role 'security-reviewer' exists" }`

**`PUT /api/roles/{role}`** — body = role fields (no `role` key, or it must equal the path). 200 + stored object. 404 `{ "error": "not_found" }`. 400 validation.

**`DELETE /api/roles/{role}`** — 204 no body. 404 not_found. 409 in-use:
```jsonc
{ "error": "in_use",
  "message": "role 'reviewer' is used by 2 running agents",
  "agents": ["a_8f3c12", "a_1b2c3d"],
  "hint": "retry with ?force=true to delete the definition; running agents are unaffected" }
```
`DELETE /api/roles/{role}?force=true` → 204 even if in use.

### 5.2 Projects

**`POST /api/projects`**
```jsonc
// request
{ "project": "billing", "title": "Billing", "color": [200,120,60],
  "cwd": "~/Projects/billing", "add_dirs": [], "context_prompt": "Stripe-backed." }
// 201 response — same object; may include warnings
{ "project": "billing", "title": "Billing", "color": [200,120,60],
  "cwd": "~/Projects/billing", "add_dirs": [], "context_prompt": "Stripe-backed.",
  "warnings": [ { "field": "cwd", "code": "cwd_not_found",
                  "message": "directory ~/Projects/billing does not exist yet" } ] }
```
- 400 invalid slug / bad color / missing title / empty cwd (§5.6)
- 409 already_exists

**`PUT /api/projects/{p}`** — full replace. 200 + object (+ optional `warnings`). 404 / 400.

**`DELETE /api/projects/{p}`** — 204. 404. 409 in-use (same shape as roles, `"project '...' is used by N running agents"`). `?force=true` → 204.

### 5.3 Backends

**`GET /api/backends`** — returns the stored `backends.json` document (master PRD §3.4 shape).

**`PUT /api/backends`** — request = the **entire** `backends.json` document.
```jsonc
// 200 response: normalized document + cred-check results
{ "version": 2,
  "backends": { /* normalized: defaults auto-promoted */ },
  "credentials": {
    "claude": { "status": "ok",     "detail": "" },
    "codex":  { "status": "failed", "detail": "invalid_api_key" }
  } }
```
- 400 invariant violation (§5.6 with codes: `unsupported_version`, `multiple_default_backends`, `unknown_default_model`, `backend_without_models`, `unknown_backend_type`). The document is **not** saved on a 400.
- 200 is returned even when some `credentials[*].status` is `failed`/`skipped` — the bytes are persisted; the caller decides what to do (onboarding blocks; Settings toasts).

### 5.4 Config / min-viable check

**`GET /api/config`**
```jsonc
// 200
{ "version": 1, "port": 4317,
  "default_project": "my-app", "default_role": "implementer",
  "skip_permissions": false,
  "onboarding_complete": false,
  "onboarding": {
    "satisfied": false,
    "steps": {
      "backend":  { "done": true,  "detail": "claude default model creds ok" },
      "project":  { "done": false, "detail": "no projects defined" },
      "role":     { "done": true,  "detail": "4 roles" }
    } } }
```
`onboarding.satisfied` = (backend.done && project.done && role.done) OR `onboarding_complete == true`.

### 5.5 Config write (onboarding completion)

**`PUT /api/config`** — partial-merge write of the user-editable subset of `config.json`. This phase only needs to set `onboarding_complete` and optionally `default_project`/`default_role`.
```jsonc
// request (only provided keys are merged)
{ "onboarding_complete": true, "default_project": "billing" }
// 200 → full updated config (same shape as GET, minus the computed onboarding block)
```
Reject attempts to change `version` or `port` here (400) — those are not user-config-editable in this phase.

### 5.6 Validation error shape (shared)

Any 400 from a CRUD/PUT handler:
```jsonc
{ "error": "validation_failed",
  "errors": [
    { "field": "title", "code": "required",      "message": "title is required" },
    { "field": "role",  "code": "invalid_slug",  "message": "must match ^[a-z0-9][a-z0-9-]{0,62}$" }
  ] }
```
`code` is a stable machine token; `message` is human text. `warnings` (non-blocking, e.g. `cwd_not_found`) use the same `{field,code,message}` element shape but ride on a **2xx** response, never block.

### 5.7 Status code summary

| Endpoint | Success | Errors |
|---|---|---|
| POST roles/projects | 201 | 400 validation, 409 already_exists |
| PUT roles/projects | 200 | 400 validation, 404 not_found |
| DELETE roles/projects | 204 | 404 not_found, 409 in_use (unless `?force=true`) |
| PUT backends | 200 (even w/ failed creds) | 400 invariant violation |
| GET config | 200 | — |
| PUT config | 200 | 400 (immutable field) |

---

## 6. Edge cases & error handling

- **Deleting a role/project in use by a running agent.** Default = refuse with 409 + the list of `running/` agent ids referencing it (lookup: scan `running/` ids, join to `agents/{id}.json` `role`/`project`). `?force=true` deletes the *definition* only; running agents already composed their config at launch (Phase 1) and keep running unaffected. UI shows a confirm dialog listing the affected agents before sending `force`.
- **Editing (PUT) a role/project in use.** Allowed, no guard — edits only affect future launches by contract. UI shows the "future launches only" note.
- **Invalid backend save.** Structural invariant violation → 400, nothing written, UI keeps the user's unsaved edits and highlights the offending field via the `FieldError[]`. Distinguish this from a *valid-but-bad-creds* save (200 with `credentials[*].status: "failed"`) which **is** persisted — the UI must not treat a failed cred-check as a lost save.
- **Cred-check flakiness.** Network/timeout/CLI-missing → `skipped`, never `failed`. A `skipped` default-model check does **not** lift the onboarding gate (we require `ok`), but Settings saves still persist. This prevents an offline user from being permanently blocked in Settings while still keeping onboarding honest about "valid credentials."
- **Partial onboarding interrupted (closed mid-wizard).** Each step persists its artifact immediately (backend doc saved on step 1, project on step 2). On relaunch, `GET /api/config` recomputes which steps are `done` and the wizard resumes at the first not-done step — no progress lost, no duplicate creation. `onboarding_complete` is only set after a successful first launch; until then the gate is driven by the computed min-viable check, so a user who configured backend+project but never launched still lands on the dashboard (gate satisfied) yet can launch normally.
- **Concurrent edits to `backends.json` / config.** Single local user, single server; writes are atomic (temp+rename). Last-write-wins is acceptable. No optimistic-concurrency token in this phase.
- **Slug collision / path traversal.** `ValidSlug` rejects `/`, `.`, `..`, whitespace, uppercase — a malicious or fat-fingered id can never write outside `roles/`/`projects/`.
- **Secrets at rest.** `env` (API keys) stored plaintext in `backends.json`, consistent with the master PRD's no-vault local-first model; the file store sets file mode `0600`. The UI masks `*_KEY` / `*_TOKEN` env values by default with a reveal toggle, and never logs them. Cred-check probes must not echo the key into server logs.
- **Deleting the default project mid-life.** Allowed (with force if running). `config.default_project` may then dangle; the New Agent modal falls back to the first available project and `GET /api/config` reports `project.done` based on *any* project existing, so the dashboard stays reachable as long as ≥1 project remains. If the *last* project is deleted, `onboarding.satisfied` could flip false — but `onboarding_complete: true` (set at first launch) keeps the wizard from re-triggering; instead the New Agent modal shows an empty-projects state prompting creation.

---

## Subphase plan (incremental / quota-limited implementation)

**Invariant:** every subphase ends at a GREEN checkpoint — `go build ./...` passes (and `npm run build` in `web/` for UI subphases) and all existing tests pass — so work is never half-done and a fresh agent can resume cold at the next subphase without inheriting partial work.

### Subphase 3.1 — Validators + Roles & Projects write paths
- **Goal:** Ship CRUD write endpoints for roles and projects over the config file store, reading from disk on demand.
- **Deliverables:** `internal/config/validate.go` (`ValidSlug` per §3.1/§3.2, `FieldError`, role/project validators — task 1); `POST/PUT/DELETE /api/roles[/{role}]` with the in-use guard against `running/` (§3.1, §5.1 — task 2); `POST/PUT/DELETE /api/projects[/{p}]` with the non-blocking `cwd_not_found` warning + in-use guard (§3.2, §5.2 — task 3); routes wired; all reads hit disk per request (§3.4). Shared `validation_failed` error shape per §5.6.
- **Depends on:** Phase 0 file store (atomic temp+rename, `AGENTDECK_HOME`) and its `GET` stubs for `roles/`/`projects/`.
- **Done when (checkpoint):** `go build ./...` passes; new Go unit tests for validators and a roles+projects POST→GET→PUT→DELETE round-trip pass (§8 CRUD + validation-failure + in-use-guard cases); existing tests green.
- **Resume note:** start from Phase 0 file store with only `GET` roles/projects/backends stubs. Begin at `validate.go`, then roles handlers, then projects handlers.
- **Size:** M.

### Subphase 3.2 — credcheck package + Backends PUT with invariants
- **Goal:** Persist the whole `backends.json` document through `PUT /api/backends`, enforcing default-backend/default-model invariants and running best-effort auth-ping credential validation.
- **Deliverables:** `internal/backend/credcheck/` — `credcheck.go` dispatch, `claude.go` (auth-status probe), `codex.go` (`/v1/models` auth ping), 6s context timeout, `CredResult{ok|failed|skipped}` (§3.5 — task 4); `internal/config/backends.go` `PUT /api/backends`: parse → invariants 1–6 → auto-promote defaults → cred-check default models → persist normalized doc regardless of cred outcome → return doc + `credentials` map (§3.3, §5.3 — task 5); backends validators in `validate.go`; per-model env merge (`merge(backend.env, model.env)`, model wins) documented + unit-tested.
- **Depends on:** Subphase 3.1 (`validate.go`, `FieldError`, file-store write pattern).
- **Done when (checkpoint):** `go build ./...` passes; invariant tests (zero-default auto-promote, multiple-default 400, unknown/empty model 400, unknown type 400, unsupported version 400-nothing-written), env-merge test, and cred-check tests with a mocked probe transport (ok/failed/skipped; doc persisted on `failed`) pass; existing tests green.
- **Resume note:** start with 3.1's roles/projects endpoints live and `validate.go` present. Begin at `credcheck/`, then `backends.go` PUT.
- **Size:** M.

### Subphase 3.3 — Config endpoints + disk-on-demand audit (backend close-out)
- **Goal:** Expose the onboarding gate data and config completion write, and confirm no stale in-memory config cache defeats disk-on-demand.
- **Deliverables:** `internal/config/config.go` — extend `GET /api/config` with the computed `onboarding` block (min-viable check §3.6, ~60s cred-check memo) and add `PUT /api/config` partial merge for `onboarding_complete`/defaults, rejecting `version`/`port` changes (§3.6, §5.4, §5.5 — task 6); disk-on-demand audit removing/invalidating any Phase 0 in-memory config cache on the CRUD + launch-composition read paths (§3.4 — task 7).
- **Depends on:** Subphase 3.2 (cred-check used by the min-viable backend step) and 3.1 (roles/projects existence checks).
- **Done when (checkpoint):** `go build ./...` passes; min-viable tests (empty store `satisfied:false` with correct per-step `done`; backend-ok-creds+project+role → `satisfied:true`; bad-creds default model → `backend.done:false`), `PUT /api/config` immutable-field 400, and selectable-without-restart tests (POST role then same-process GET returns it) pass; existing tests green. Backend surface complete.
- **Resume note:** start with all roles/projects/backends write endpoints live. Begin at `config.go` `GET` onboarding block, then `PUT /api/config`, then the cache audit.
- **Size:** S.

### Subphase 3.4 — Frontend scaffolding + Settings (Roles & Projects editors)
- **Goal:** Stand up the config API/query layer and ship the Roles and Projects Settings tabs.
- **Deliverables:** `web/src/schemas/` Zod schemas (role/project/backends/config) + `web/src/api/config.ts` typed fetchers/mutations + TanStack Query keys & invalidation wiring (§2.1, §4 — task 8); `SettingsPage.tsx` tabs, `RolesEditor`/`RoleForm` (create-only slug, tri-state `skip_permissions`), `ProjectsEditor`/`ProjectForm` (RGB color trio + swatch, `cwd_not_found` warning render) routed into the Phase 2 shell (§4.1 — task 9).
- **Depends on:** Subphases 3.1 & 3.3 (roles/projects endpoints + `GET /api/config`); Phase 2 shell (routing, agent store).
- **Done when (checkpoint):** `npm run build` in `web/` passes; `go build ./...` still passes; Vitest+MSW tests for the Roles/Projects editors (create invalidates query so the entity appears) pass; existing tests green.
- **Resume note:** backend endpoints are all live and tested. Begin at `schemas/` + `api/config.ts`, then the Settings tabs; reuse Phase 2 routing.
- **Size:** M.

### Subphase 3.5 — Backends editor + New Agent modal
- **Goal:** Ship the full backends editor and the New Agent modal (the UI front-end to the Phase 1 launch).
- **Deliverables:** `BackendsEditor`/`ModelRow` — exactly-one-default radios (UI-enforced, re-enforced server-side), backend/per-model `env` editors with masked `*_KEY`/`*_TOKEN` + reveal, per-backend validate + cred chip, full-document `PUT /api/backends` re-rendering from the normalized response (§4.1 — task 10); `NewAgentModal` + `useSuggestedName`, backend-filtered model select resetting on backend change, disabled terminal interface, submit to `POST /api/sessions` (§4.2 — task 11).
- **Depends on:** Subphase 3.4 (schemas, API layer, Settings shell) and 3.2 (`PUT /api/backends`); Phase 1 `POST /api/sessions`.
- **Done when (checkpoint):** `npm run build` passes; `go build ./...` passes; Vitest tests (model filters to backend + resets on backend change; name auto-suggests until edited; terminal disabled; failed cred-check renders chip without data-loss toast; secret masking) pass; existing tests green.
- **Resume note:** Settings shell, schemas, and API layer exist from 3.4. Begin at `BackendsEditor`, then `NewAgentModal`.
- **Size:** M.

### Subphase 3.6 — Onboarding wizard gate + wire-up/polish
- **Goal:** Gate the dashboard behind the onboarding wizard until min-viable-config exists, reusing the editors and modal.
- **Deliverables:** `OnboardingGate` (reads `GET /api/config` `onboarding.satisfied`, blocks dashboard, non-dismissible) + `OnboardingWizard` 3 steps (BackendStep / ProjectStep / LaunchStep) reusing the editors/modal, resume-from-first-not-done-step, set `onboarding_complete` via `PUT /api/config` on first launch (§4.3 — task 12); empty states, toasts, error surfacing (§4 — task 13); remaining frontend tests (§8 — task 14).
- **Depends on:** Subphases 3.4 & 3.5 (editors + modal reused as wizard steps) and 3.3 (`GET /api/config` onboarding block).
- **Done when (checkpoint):** `npm run build` passes; `go build ./...` passes; Vitest gating tests (`satisfied:false` → wizard renders & dashboard blocked, Esc/overlay-click no-op; `satisfied:true` → dashboard, no wizard; backend-done/project-not-done → resumes on project step) and remaining §8 frontend tests pass; existing tests green. Phase 3 acceptance criteria (phase PRD §5) met.
- **Resume note:** all backend endpoints and Settings/modal UI exist. Begin at `OnboardingGate`, then `OnboardingWizard` steps, then polish/tests.
- **Size:** M.

## 7. Implementation task breakdown (ordered)

1. **Shared validators** — `internal/config/validate.go`: `ValidSlug`, `FieldError`, role/project/backends validators with the rules in §3. Unit-test in isolation.
2. **Roles write paths** — `POST/PUT/DELETE /api/roles[/{role}]` handlers over the file store, incl. in-use guard. Wire routes.
3. **Projects write paths** — `POST/PUT/DELETE /api/projects[/{p}]`, incl. `cwd_not_found` warning + in-use guard.
4. **credcheck package** — `internal/backend/credcheck` with `claude.go` (auth-status probe) and `codex.go` (models-list auth ping), context-timeout-bounded, returning `CredResult`.
5. **Backends PUT** — `PUT /api/backends`: parse → invariants → auto-promote defaults → cred-check default models → persist normalized doc → return doc + `credentials`.
6. **Config endpoints** — extend `GET /api/config` with the computed `onboarding` block (with ~60s cred-check cache); add `PUT /api/config` partial merge (`onboarding_complete`, defaults).
7. **Disk-on-demand audit** — ensure roles/projects/backends reads (incl. launch composition) hit disk per request; remove/invalidate any Phase 0 in-memory cache.
8. **Frontend scaffolding** — Zod schemas, typed API layer (`web/src/api/config.ts`), TanStack Query keys + mutation/invalidation wiring.
9. **Settings — Roles & Projects editors** — `SettingsPage` tabs, `RolesEditor`/`RoleForm`, `ProjectsEditor`/`ProjectForm` with create-only slug, color trio, warnings rendering.
10. **Settings — Backends editor** — `BackendsEditor`/`ModelRow`, default radios (UI enforcement), env editors with masked secrets, per-backend validate + cred chip, full-document save.
11. **New Agent modal** — `NewAgentModal` + `useSuggestedName`, backend-filtered models, disabled terminal interface; submit to `POST /api/sessions`.
12. **Onboarding** — `OnboardingGate` (reads `/api/config`, blocks dashboard), `OnboardingWizard` 3 steps reusing the editors/modal, resume-from-step logic, set `onboarding_complete` on first launch.
13. **Wire-up & polish** — empty states (no roles/projects), toasts, secret-masking, error surfacing; route Settings into the Phase 2 shell.
14. **Tests** — §8.

### Open questions resolved before coding

- Pin the exact Claude Code auth-status subcommand and Codex `/v1/models` base URL against the versions pinned in Phase 1 (§3.5 gives the strategy; confirm the exact flag/path at implementation time — strategy is fixed, only the literal token may shift with CLI version).
- Confirm `POST /api/sessions` accepts a `name` field (Phase 1). If not, modal calls launch then rename. Modal contract unchanged either way.

---

## 8. Testing strategy

**Backend (Go), using `AGENTDECK_HOME` pointed at a temp dir:**

- **CRUD round-trips:** POST→GET→PUT→GET→DELETE→GET for roles and projects; assert the file on disk matches and the GET reflects each step. Atomic-write check (no partial files).
- **Validation failures:** invalid slug, missing title, bad color (4 ints / out of range), empty cwd → 400 with the expected `error code` in `errors[]`. `cwd_not_found` rides on a 201/200 as a `warning`, not a 400.
- **Invariant enforcement (backends):** zero defaults → auto-promoted (assert normalized response); multiple defaults → 400; `default_model` unknown → 400; empty `models` → 400; unknown `type` → 400; unsupported `version` → 400 (and nothing written). Per-model env override merge (`merge(backend.env, model.env)`, model wins) unit-tested against the documented Phase 1 composition contract.
- **Cred-check:** mock the probe transport — `ok` (200 models-list), `failed` (401), `skipped` (timeout / CLI missing / no key). Assert PUT persists the doc on `failed` and returns `credentials[*].status` correctly.
- **In-use guard:** seed a `running/{id}.json` referencing a role/project; DELETE → 409 with the agent id listed; DELETE `?force=true` → 204 and the running file untouched.
- **Min-viable-config:** empty store → `onboarding.satisfied == false`, correct per-step `done`; add backend(ok creds)+project+role → `satisfied == true`. Bad-creds default model → `backend.done == false`.
- **Selectable-without-restart:** create a role via POST, then `GET /api/roles` in the **same** server process returns it (no restart) — proves disk-on-demand. Same for a new project and a new model appearing for launch composition.

**Frontend (Vitest + React Testing Library, MSW for the API):**

- New Agent modal: model select filters to chosen backend; changing backend resets model to that backend's default; name auto-suggests until edited then stops; terminal interface disabled.
- Mutations invalidate queries: after creating a role in Settings (mocked 201), the New Agent modal's role list includes it without a reload.
- Onboarding gating: `/api/config` `satisfied:false` → wizard renders, dashboard blocked (non-dismissible: Esc/overlay-click do nothing); `satisfied:true` → dashboard renders, no wizard. Resume: backend-done/project-not-done → wizard opens on the project step.
- Backends editor: exactly-one-default radios; failed cred-check renders a chip but the save is treated as persisted (not an error toast that implies data loss); secret env values masked with reveal.

**End-to-end (manual / acceptance, mirrors phase PRD §5):**

- Fresh empty `~/.agentdeck/` → wizard walks backend → project → first running agent; dashboard blocked until then; relaunch does not re-show the wizard.
- Add a custom role → appears in the modal with no restart.
- Configure a second model with a custom `OPENAI_BASE_URL` → Phase 1 launch routes that model to the override URL (verify in launch/request logs).
- Marking a different backend default → modal's default backend selection follows.
- Save a backend with a deliberately bad key → cred-check `failed` surfaced, not silently accepted; onboarding won't advance, but Settings still persisted the edit.

---

## 9. Interfaces consumed / produced

**Consumes (depends on):**
- **Phase 1 launch:** `POST /api/sessions {name?, role, project, backend, model, interface}` — the New Agent modal and onboarding LaunchStep call this unchanged. Phase 1 owns composition (`project.cwd` + `project.context_prompt` + `role.system_prompt` + `backend/model` + merged env); this phase produces the definitions that composition reads at launch.
- **Phase 0 file store:** typed read/write/list/delete with atomic writes and `AGENTDECK_HOME`; the `GET` stubs for roles/projects/backends (we add the write paths).
- **Phase 2 shell:** routing, the agent store, SSE client. The new card appears via the Phase 2 `state_update` after a modal launch; Settings + onboarding mount inside the Phase 2 app shell.

**Produces (for downstream phases):**
- Authoritative, validated `roles/`, `projects/`, and `backends.json` that **Phase 1 composes at launch** and every later phase reads. The per-model `env` override semantics (`model.env` over `backend.env`) and default-backend/default-model invariants are guaranteed here so launch composition (Phase 1) and switch-runtime (Phase 6) can rely on them.
- `config.onboarding_complete` and the `GET /api/config` min-viable contract used by the dashboard gate.
- `skip_permissions` per-role override (§10) consumed by Phase 1's permission gating.

---

## 10. Resolved decisions

- **Credential validation method (per backend type): auth ping, not a trial/billed request.** `claude-acp` → CLI non-interactive auth-status check (validates the logged-in session; `skipped` if CLI absent). `codex-acp` → authenticated `GET /v1/models` against `OPENAI_BASE_URL` (default `https://api.openai.com`) with the `OPENAI_API_KEY` from the merged env: 200 → `ok`, 401/403 → `failed`, network/timeout/no-key → `skipped`. 6s timeout, best-effort, never panics, never blocks the save (only blocks onboarding advancement). (PRD §6 open question resolved.)
- **Save policy on bad creds: persist anyway, report status.** A 400 means *structurally invalid* (nothing written); a 200 with `credentials[*].status: "failed"` means *saved but creds rejected*. Onboarding requires `ok` to advance; Settings just surfaces the result. This keeps users from losing edits while keeping "valid credentials" honest for the gate.
- **Default invariants enforced server-side with auto-promotion.** Exactly one default backend and exactly one default model per backend, enforced on `PUT /api/backends`; missing defaults are auto-promoted (sorted-first) and echoed back; conflicting/unknown defaults are 400. The client never gets to persist an inconsistent default set.
- **`skip_permissions` is exposed per-role** (tri-state: `null` inherit global / `true` / `false`) here, consumed by Phase 1 permission gating. Finer per-tool policy is explicitly deferred (master PRD §9).
- **Per-type notification mute is out of scope for Phase 3** — it belongs to notifications (F11, Phase 5). Noted here only to mark the boundary; this phase does not add notification settings.
- **Onboarding "never reappears" = computed gate OR explicit `onboarding_complete` flag.** The flag (set on first successful launch) protects returning users from being re-trapped if they later delete their only project; the computed check covers a genuinely fresh store before the flag exists.
- **Read-from-disk-on-demand** for roles/projects/backends on every relevant request (CRUD + launch composition) — the mechanism behind "selectable without restart"; no server-side config cache except a short (~60s) cred-check memo to avoid re-probing on every dashboard poll.
