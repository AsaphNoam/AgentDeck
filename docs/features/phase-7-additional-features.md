# Phase 7 — Additional features: configuration federation, OpenHands & OpenCode

**Status:** active — backend work 7.1–7.3 complete; configuration federation added to remaining scope
**Features:** F16 (link/sync/import Claude Code and Codex configuration), F14 (OpenCode backend), F15 (OpenHands backend); extends F7 (switch-runtime backend matrix) and the seeded AgentDecker guide
**Depends on:** Phases 1, 2, 3, 6 (chat runtime, config UI, adapter seam, switch-runtime)
**Enables:** one setup shared with the native CLIs instead of a third drifting copy

---

## 1. Goal

Make AgentDeck a supervisor over the user's existing **Claude Code** and **Codex** setup, not a
competing configuration silo. A user can link either backend to its native configuration and have
new AgentDeck launches automatically see changes to model/provider/effort defaults and native setup
assets such as instructions, skills, subagents, rules, hooks, and MCP servers.

The preferred design is **federation by reference**: Claude Code or Codex remains authoritative,
AgentDeck stores only a source binding, explicit AgentDeck overrides, provenance, and fingerprints.
Where a surface cannot safely remain live-linked, AgentDeck may maintain a rebuildable mirror. The
original one-time import remains available as a detached snapshot for users who want AgentDeck to
own and edit an independent copy.

Phase 7 also widens AgentDeck from two backends (Claude, Codex) to four by adding **OpenCode** and
**OpenHands** as first-class chat backends through the existing ACP runtime.

---

## 2. Scope

### In scope

- Discover the standard user and selected-project configuration roots for Claude Code and Codex,
  with an advanced option to link a different root/profile.
- Three explicit ownership modes per Claude/Codex backend:
  - **Linked (recommended):** the native CLI files are authoritative; AgentDeck resolves them at
    read/launch time and does not persist a normalized copy as configuration.
  - **Mirrored:** the native files remain authoritative, while AgentDeck maintains a disposable,
    auto-refreshed normalized cache for a surface that cannot be consumed by reference.
  - **Detached snapshot:** a one-time, user-confirmed import into AgentDeck-owned config/assets;
    later external changes do not apply.
- Auto-refresh linked/mirrored sources through file watching plus a periodic reconciliation pass;
  every launch performs a synchronous freshness check so a missed watch event cannot launch stale.
- High-level launch settings: configured model/default, model catalog or allowlist when present,
  provider/base URL metadata, reasoning/effort, and selected profile. “Available models” means
  configured/allowlisted/catalogued models, not a promise to enumerate account entitlements.
- Native setup surfaces: Claude `CLAUDE.md`, `.claude/rules/`, skills, subagents, settings/hooks,
  plugins and MCP declarations; Codex `AGENTS.md`, `.agents/skills/`, `.codex/config.toml`, profiles,
  agents, rules/hooks, plugins and MCP declarations. The UI inventories their source and status.
- A binary-versioned AgentDeck knowledge base, exposed to live agents through the existing MCP
  registration as `agentdeck_docs`. It supplies a topic index plus named, version-matched Markdown
  topics covering the shipped product (launching, configuration, dashboard, interfaces, archive,
  messaging, notifications and troubleshooting). The seeded AgentDecker role keeps its persona and
  orchestration guidance short and points agents to this tool for product facts.
- Preserve each CLI's documented user/project/local/managed precedence and project trust rules.
- Explicit per-field AgentDeck overrides layered above the linked baseline without modifying the
  external files; the UI distinguishes inherited values from overrides and can reset an override.
- Immutable launch provenance: each session records requested values, resolved high-level values,
  selected source/profile and content fingerprints (never secret values).
- Existing OpenCode/OpenHands backend, permission, credential, UI and switch-runtime work.

### Out of scope

- Two-way sync or writing into Claude Code/Codex configuration. AgentDeck never edits external
  settings, instructions, skills, agent definitions, MCP declarations, hooks, profiles, or secrets.
- Translating a Claude skill/agent/config into a Codex skill/agent/config (or the reverse). Each CLI
  consumes its own native surfaces; shared files or symlinks remain a user/repository choice.
- Copying credentials, auth stores, literal secret environment values, telemetry, usage history,
  conversation state, UI preferences, or managed enterprise policy into AgentDeck.
- Guaranteeing that a locally named model is enabled for the user's account; native CLI validation
  and launch errors remain authoritative.
- Hot-mutating a running agent. High-level changes apply to the next new launch; explicit resume
  keeps its frozen model/provider/effort snapshot. Native instruction/setup files may be reread by
  the CLI when a process starts or resumes, according to that CLI's own behavior.
- Terminal (PTY) interface for OpenHands/OpenCode and alternative non-ACP transports.
- Treating repository documentation or an installed user's role prompt as the runtime source of
  truth. Product knowledge ships in the binary; AgentDeck does not overwrite an existing user-owned
  role just to refresh its guidance.

---

## 3. Detailed requirements

### 3.1 Configuration federation from Claude Code / Codex (F16)

- Onboarding offers **Use my Claude Code setup** and **Use my Codex setup** after a read-only
  preview. Settings exposes source mode, resolved root/profile, last refresh, health, provenance,
  discovered assets, overrides, **Refresh now**, **Detach**, and **Relink**.
- Enabling a link is explicit and shows every root AgentDeck will read. It never silently enables a
  newly discovered custom root, follows a changed symlink target, or expands to another project.
- Linked is the default recommendation. Mirrored is used only when the adapter cannot pass a
  surface through natively; the cache is labelled derived, can be deleted/rebuilt, and is never a
  conflict peer. Detached snapshot is the only mode in which AgentDeck becomes authoritative.
- AgentDeck delegates native setup loading to the launched CLI whenever possible: launch with the
  real project working directory and native user config environment, then add only AgentDeck's
  per-session overlays (identity, messaging MCP, role/project prompt, and explicit user overrides).
- A resolver parses only the documented subset needed for preview, model controls and provenance.
  Unknown keys are preserved externally and reported as native/pass-through, not dropped or
  rewritten. Unsupported fields never make the whole source look successfully imported.
- External changes invalidate the effective view and notify the UI. A parse failure retains the
  last-known-good value for display only, marks the binding stale/invalid, and blocks a new launch
  that depends on that source until it is fixed, detached, or explicitly overridden.
- Name collisions follow the native CLI's precedence. AgentDeck's injected messaging MCP uses a
  reserved per-session id; a conflicting external id produces a preflight error instead of being
  overwritten. Managed policy always wins over AgentDeck overrides.
- Secrets are neither returned by the preview/status APIs nor stored in the mirror/session
  snapshot. Secret-bearing settings are represented as redacted key names or “configured”.

### 3.2 Effective configuration and session semantics

- Effective config is resolved in this order: managed/native constraints → AgentDeck explicit
  launch choice → AgentDeck stored override → native project/local layer → selected native profile
  → native user layer → AgentDeck seed fallback. The provider resolver must preserve any
  provider-specific exception to this simplified ordering and expose provenance per field.
- “Inherit CLI default” is a first-class model/effort choice. AgentDeck must not convert a missing
  external model into a guessed model id. When the runtime reports the actual model, store it as
  observed state without turning it into an override.
- New launches resolve the latest valid source. Resume and same-session model switches retain the
  frozen high-level snapshot unless the user explicitly chooses **Resume with latest setup**;
  source fingerprints still show whether native setup assets changed.
- A project can use linked user defaults while its own checked-in `.claude`, `.codex`,
  `.agents/skills`, `CLAUDE.md`, and `AGENTS.md` layers remain project-relative. Switching the
  selected AgentDeck project therefore changes the effective project layer without rebinding the
  user's global source.

### 3.3 Binary-versioned product knowledge

- Ship a small, curated set of AgentDeck product topics inside the executable, not from the
  checkout or a mutable config directory. A released binary therefore serves documentation for the
  behavior it actually contains.
- Add `agentdeck_docs` to the MCP tools registered for each live agent. Calling it without a topic
  returns the stable topic index; a valid topic returns that Markdown; an unknown topic returns an
  actionable tool error that includes the valid names. It must use the existing per-agent MCP
  registration rather than add an unauthenticated documentation endpoint.
- Keep documentation contents product-facing and non-secret. It may name supported configuration
  files and user-visible workflows, but must not embed credential values, source-file contents, or
  future/unshipped behavior. Every release-changing product surface updates the relevant topic in
  the same change.
- Slim the fresh `agentdecker` seed prompt to persona, orchestration behavior, and an instruction
  to consult `agentdeck_docs` before answering non-trivial product questions. Existing role files
  remain user-owned and unchanged; the tool itself remains available to their live agents.

### 3.4 OpenCode backend (F14)

- Backend type `opencode-acp`, binary `opencode`, launch args `["acp"]`; model selection uses
  `provider/model` ids. Permission config is injected per launch and never written to user config.
- Native ACP resume where supported; otherwise the Phase 6 primer path.

### 3.5 OpenHands backend (F15)

- Backend type `openhands-acp`, binary `openhands`, launch args `["acp"]`.
- Model/auth via `LLM_MODEL`, `LLM_API_KEY`, and `LLM_BASE_URL` composed from backend config.
- Always-approve mode maps from `skip_permissions`; native resume where supported, primer otherwise.

### 3.6 Shared backend integration rules

- Agent-specific differences live in `BackendAdapter`; runtime, persistence and SSE stay generic.
- OpenCode/OpenHands permission prompts use the existing ACP permission gate; neither exposes the
  required terminal hook surface, so terminal launch/switch remains rejected.
- Cross-backend swaps preserve history through native resume or the Phase 6 primer.

---

## 4. REST surface added

- `GET /api/config-sources` — discovery, binding mode, health, fingerprints, redacted inventory and
  field provenance for each Claude/Codex source in the selected project.
- `POST /api/config-sources/preview` — read-only preview of a proposed provider/root/profile/mode;
  performs no persistence and returns exact read/skipped/error paths.
- `PUT /api/config-sources/{backend_id}` — save or replace an explicit binding and overrides after
  preview; never mutates the external source.
- `POST /api/config-sources/{backend_id}/refresh` — invalidate and synchronously resolve now.
- `DELETE /api/config-sources/{backend_id}` — remove the binding; optional `?detach=true` first
  materializes the previewed non-secret snapshot into AgentDeck-owned config.

Existing `GET/PUT /api/backends`, launch, resume and switch APIs remain. Backend reads and launch
composition use the effective resolver; writes continue to edit AgentDeck-owned fallback/overrides.

---

## 5. Acceptance criteria

- [ ] Linking a standard Claude Code or Codex setup requires one preview/confirm flow and does not
      modify any external file.
- [ ] Editing a linked model/default/effort or setup asset outside AgentDeck updates Settings and
      the next new launch without re-import; a missed watch event is caught by launch preflight.
- [ ] Claude/Codex launches see their native project instructions, skills, agents and MCP servers
      from the selected project without AgentDeck copying them.
- [ ] Settings shows inherited versus overridden values with source path/scope, and reset restores
      inheritance. “Inherit CLI default” works when no concrete model is configured.
- [ ] A detached high-level snapshot and every asset explicitly reported copyable keep working after
      the source changes or disappears; reference-only assets are excluded before confirmation.
      Linked/mirrored modes report source failure rather than silently becoming detached.
- [ ] Malformed or unsupported source content produces a redacted partial report; dependent new
      launches cannot silently use stale last-known-good config.
- [ ] Secrets/auth stores are absent from API bodies, mirrors, logs and session snapshots.
- [ ] Existing sessions keep their frozen high-level launch settings; a new launch receives the
      newly resolved values and records provenance/fingerprints.
- [ ] Existing OpenCode/OpenHands launch, permission, resume, switch and terminal-gate acceptance
      criteria remain green.
- [ ] A fresh seeded AgentDecker role can list and retrieve binary-versioned product documentation
      through `agentdeck_docs`; the served topics match the running build and never expose secrets
      or unshipped Phase 7 behavior.

---

## 6. Product decisions fixed by this revision

- External CLI config is authoritative in linked/mirrored modes; there is no two-way merge.
- Pointer/native pass-through is preferred over copying. A cache is implementation state, not a
  source of truth; detached snapshot is the explicit escape hatch.
- Setup assets remain provider-native. AgentDeck inventories and composes them but does not invent a
  lowest-common-denominator skill/agent/MCP schema.
- Auto-sync means future-launch consistency, not mutation of a process already running.
- Model discovery is honest about its boundary: configured/catalogued is not account-entitled.
- Product guidance is binary-versioned and MCP-served, while role files remain user-owned. This
  avoids silently rewriting a user's prompt just to correct stale product facts.

Implementation details and provider surface mappings are prescriptive in the mirror tech spec.
