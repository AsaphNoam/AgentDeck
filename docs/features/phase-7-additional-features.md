# Phase 7 — Additional features: backend import, OpenHands & OpenCode

**Status:** selected Phase 7 candidate (from the future-phase bucket) — ready to build after Phase 6
**Features:** F16 (import backend models/config from Claude Code and Codex), F14 (OpenCode backend), F15 (OpenHands backend); extends F7 (switch-runtime backend matrix)
**Depends on:** Phases 1, 2, 6 (chat runtime, adapter seam, switch-runtime)
**Enables:** remaining future-phase candidates

---

## 1. Goal

First, remove backend setup guesswork by letting AgentDeck import backend models/defaults from the
user's existing **Claude Code** and **Codex** configs as a one-time action from Settings/Onboarding.

Widen AgentDeck from a two-backend (Claude, Codex) dashboard to a four-backend one by adding
**OpenCode** (opencode.ai, `opencode` CLI) and **OpenHands** (openhands.dev, `openhands` CLI) as
first-class chat backends. Both CLIs natively speak ACP over stdio (`opencode acp`,
`openhands acp`), so they ride the existing chat runtime unchanged — the work is two new backend
adapters, config/seed/UI plumbing, and extending the switch-runtime matrix, not a new runtime.

---

## 2. Scope

### In scope
- One-time import of backend defaults/models from existing Claude Code and Codex user config so a
  fresh AgentDeck setup can start from real local values instead of seeded placeholders.
- OpenCode chat backend: `opencode acp` through the existing ACP chat runtime; launch, stream,
  cancel, stop, resume on a stable `agent_id`.
- OpenHands chat backend: `openhands acp` through the same runtime, same lifecycle.
- Backend adapters capturing all per-agent differences: binary/args, env passthrough, model
  selection, resume mechanism, permission/yolo mapping, hook capability (none), MCP registration.
- Config: seeded backend entries, type validation, credential checks for both.
- UI: backend type union, onboarding + settings editors, launch modal labels.
- Switch-runtime: the new backends join the backend-swap matrix (cross-backend swaps use the
  Phase 6 history primer).
- Gated live acceptance against the real CLIs (same class as the Phase 1 / Codex gates).

### Out of scope
- Continuous two-way sync with Claude Code/Codex config after import; Phase 7 only covers explicit
  one-time import into AgentDeck's own config.
- Terminal (PTY) interface for OpenHands/OpenCode — no verified hook-registration path, so both
  are rejected with `422 terminal_unavailable` exactly like Codex terminal (Phase 6 decision).
- Agent-to-agent messaging for the new backends beyond what the existing HTTP MCP registration
  provides; any stdio-MCP fallback work.
- OpenCode `serve`/HTTP-server mode and OpenHands headless (`--headless`) mode as alternative
  transports — ACP is the only integration path this phase.
- Other future-phase candidates (activity map, templates, notes, triage filters).

---

## 3. Detailed requirements

### 3.1 Backend import from Claude Code / Codex (F16)
- Settings and onboarding expose an explicit "Import from Claude/Codex" action.
- Import reads each tool's user config, extracts backend/model defaults plus any non-secret backend
  parameters AgentDeck understands, and writes a normalized AgentDeck `backends.json`.
- AgentDeck remains the source of truth after import; imported values can be edited locally
  without mutating Claude/Codex config files.
- Secret material is never silently copied unless the user explicitly opted into importing env
  vars/keys.
- If an external config is missing, malformed, or only partially understood, the import is
  best-effort and reports exactly what was imported vs skipped.

### 3.2 OpenCode backend (F14)
- New backend type `opencode-acp`, seeded backend id `opencode`, binary `opencode`, launch args
  `["acp"]`; runs through the existing chat runtime with no runtime branching.
- Model selection via OpenCode's `provider/model` ids (e.g. `anthropic/claude-sonnet-4-5`).
- `skip_permissions` maps to OpenCode's per-tool `permission` config (injected, never written
  into the user's `opencode.json`).
- Native resume of the same logical session where ACP `session/load` supports it; otherwise the
  Phase 6 primer path.

### 3.3 OpenHands backend (F15)
- New backend type `openhands-acp`, seeded backend id `openhands`, binary `openhands`, launch
  args `["acp"]`.
- Model/auth via OpenHands env (`LLM_MODEL`, `LLM_API_KEY`, `LLM_BASE_URL`) composed from backend
  config env, not the user's `config.toml`.
- `skip_permissions` maps to OpenHands' always-approve approval mode over ACP; default mode keeps
  per-action prompts flowing through the existing permission gate.
- Resume as in 3.1: native where supported, primer otherwise.

### 3.4 Shared integration rules
- Everything agent-specific lives in a `BackendAdapter` (Phase 6 §6.3 rule); the chat runtime,
  state, persistence, and SSE paths stay backend-agnostic.
- Permission requests flow through the existing ACP withhold-the-response gate; both backends'
  approval prompts appear as normal dashboard permission cards.
- Hooks: neither CLI has a Claude-shaped hook surface; adapters report no hook support and chat
  status derives from the ACP stream (as it already does for chat agents).
- Terminal interface for both types is rejected at launch and switch validation.
- Credential checks (Settings "Validate") for both backends; a fresh machine without the CLI
  installed gets a clear, non-blocking error.

### 3.5 Switch-runtime matrix (extends F7)
- Same-backend model swap for the new types follows each adapter's `CanSwitchModelOnResume`.
- Cross-backend swaps (any of the four ↔ any other) preserve history via native resume or the
  Phase 6 primer; no new switch endpoint semantics.

---

## 4. REST surface added

```
(none — no new endpoints)
```

The existing config CRUD (`PUT /api/backends`), launch (`POST /api/sessions`), resume, and
switch-runtime endpoints gain two accepted backend types; `422 terminal_unavailable` covers the
rejected terminal combinations.

---

## 5. Acceptance criteria

- [ ] A user can add/validate an OpenCode backend and launch a chat agent that streams a turn.
- [ ] A user can add/validate an OpenHands backend and launch a chat agent that streams a turn.
- [ ] A user can import Claude Code and Codex backend defaults/models into AgentDeck in one action
      from onboarding or settings.
- [ ] Stop → resume continues the same logical session on the same `agent_id` for both backends.
- [ ] A permission request from either backend surfaces as a dashboard permission card and can be
      approved/denied.
- [ ] `skip_permissions` launches run without permission prompts on both backends.
- [ ] Switching backend Claude → OpenCode (and back) preserves history via the primer path.
- [ ] Requesting a terminal-interface launch for either new backend fails with a clear
      `terminal_unavailable` error; the UI never offers the combination.
- [ ] Onboarding and Settings list all four backend types; seeded config round-trips untouched.

---

## 6. Open questions (for the techspec)
- Does each CLI's ACP implementation support `session/load` (native resume) — and if not, does
  `ResolveResumeID` return empty to force the primer path?
- Exact yolo mapping: OpenCode config injection (`OPENCODE_CONFIG_CONTENT`?) vs a session mode;
  OpenHands always-approve as ACP session mode vs launch flag.
- Do the CLIs accept the `mcpServers` entries the runtime already passes in `session/new` (for
  the messaging MCP), or does registration need per-agent config injection?
- Which env vars must be stripped (nested-session guards, `CLAUDECODE`-class issues) for each CLI?
