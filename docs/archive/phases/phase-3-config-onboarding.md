# Phase 3 — Config CRUD & onboarding

**Status:** ready to build after Phase 2
**Features:** F5 (projects & roles), F6 (backend & model config), F12 (onboarding)
**Depends on:** Phases 1, 2
**Enables:** a usable first-run experience; parallelizable with Phases 4 and 5

---

## 1. Goal

Make AgentDeck configurable and approachable without hand-editing JSON. Add Settings UIs for roles, projects, and backends/models, and a guided first-run onboarding that blocks the dashboard until a minimum viable config exists (≥1 backend with valid credentials, ≥1 project, then the first agent launch). Also deliver the full New Agent modal (the launch flow's UI front-end to Phase 1's launch endpoint).

---

## 2. Scope

### In scope
- Roles CRUD (F5): create/edit/delete role definitions (`title`, `system_prompt`, `skip_permissions` override).
- Projects CRUD (F5): create/edit/delete (`title`, `color`, `cwd`, `add_dirs`, `context_prompt`).
- Backends/models config (F6): edit `backends.json`; per-model API key + endpoint (`env`) overrides; mark default backend and default model per backend; credential validation on save where possible.
- New Agent modal (F4 UI): name (auto-suggested), role, project, backend, model, interface → calls Phase 1 launch.
- Onboarding (F12): guided steps gating the dashboard on a fresh `~/.agentdeck/`.

### Out of scope
- Switch-runtime on a live agent (Phase 6/F7) — distinct from editing definitions.
- Archive/resume (Phase 4), messaging (Phase 5).

---

## 3. Detailed requirements

### 3.1 Roles & projects CRUD (F5)
- Settings UI listing existing roles/projects with create/edit/delete; backed by `roles/` and `projects/` files.
- Direct JSON edit remains valid — the UI is a convenience over the same files.
- Editing a role/project affects **future** launches only; existing agents keep their composed config until restarted (composition happens at launch, Phase 1).
- A newly added role/project is selectable in the New Agent modal **without a server restart** (read from disk on demand).

### 3.2 Backends & models (F6)
- Settings UI over `backends.json` (master PRD §3.4 schema, `version: 2`).
- Per-model `env` overrides (e.g. `OPENAI_API_KEY`, `OPENAI_BASE_URL`) over backend-level `env`.
- Exactly one default backend; exactly one default model per backend (enforce on save).
- Validate credentials where feasible (e.g. a lightweight auth/ping check) and surface pass/fail.
- `GET /api/backends`, `PUT /api/backends`.

### 3.3 New Agent modal (F4 UI)
- Fields: name (auto-suggested, editable), role, project, backend, model (filtered to chosen backend), interface (chat default; terminal shown but may be disabled until Phase 6).
- Submits to `POST /api/sessions`; on success the card appears (Phase 2) and chat is openable.

### 3.4 Onboarding (F12)
- On a fresh/empty `~/.agentdeck/` (or missing minimum viable config), block the main dashboard and walk the user through:
  1. Configure ≥1 backend with valid credentials.
  2. Create the first project.
  3. Launch the first agent.
- Once minimum viable config exists, onboarding does not reappear.

---

## 4. REST surface added

```
GET  /api/roles        POST /api/roles      PUT /api/roles/{role}     DELETE /api/roles/{role}
GET  /api/projects     POST /api/projects   PUT /api/projects/{p}     DELETE /api/projects/{p}
GET  /api/backends     PUT /api/backends
GET  /api/config       (onboarding/minimum-viable-config check)
```

(`GET` variants of roles/projects/backends were stubbed in Phase 0; this phase adds the write paths and validation.)

---

## 5. Acceptance criteria

- [ ] Adding a custom role makes it selectable in the New Agent modal without restarting the server.
- [ ] Configuring a second model with a custom endpoint routes that model's calls to the override URL (verifiable in Phase 1 launch composition / request logs).
- [ ] Marking a different backend default updates the modal's default selection.
- [ ] Saving a backend with invalid credentials surfaces a validation failure rather than silently accepting.
- [ ] A fresh install with empty `~/.agentdeck/` is walked through backend → project → first running agent, and the dashboard is blocked until that minimum config exists.
- [ ] The New Agent modal produces an agent identical to the CLI launch form.

---

## 6. Open questions
- Per-backend credential validation method (auth ping vs. trial request) — depends on each CLI's capabilities.
- Permission model granularity (master PRD §9): expose `skip_permissions` per-role here; per-tool policy is a possible later extension.
