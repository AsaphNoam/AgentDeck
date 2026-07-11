# Phase 7 — Implementation Tech Spec: configuration federation, OpenHands & OpenCode

**Mirrors:** `docs/features/phase-7-additional-features.md` (phase PRD)
**Features:** F16 (Claude/Codex config federation), F14 (OpenCode backend), F15 (OpenHands backend); extends F7 (switch-runtime matrix)
**Depends on:** Phase 1 (chat runtime), Phase 3 (config/onboarding), Phase 6 (`BackendAdapter` seam, switch-runtime, terminal gates)
**Audience:** the engineer implementing this phase. Prescriptive; no further design decisions required.

---

## 1. Overview & scope recap

Add a configuration-federation layer for **Claude Code** and **Codex**. AgentDeck stores bindings
and overrides, resolves a provider-native effective view, and lets each CLI consume its own native
instruction/tooling surfaces. This replaces the underspecified one-time F16 import that appeared in
the PRD but had no implementation section or REST surface in the original tech spec.

The already-built backend slice adds **OpenCode** (`opencode acp`) and **OpenHands**
(`openhands acp`) through the existing ACP chat runtime. Per-agent differences remain captured in a
`BackendAdapter`; federation is a launch-composition/config concern and must not add branches to the
ACP transport, event mapping, persistence, SSE, or permission gate.

Hard constraints:

- **No new agent runtime.** Federation adds config-source endpoints and resolver packages, not a
  fifth process or a second ACP path.
- **No external writes.** Linked/mirrored modes never change Claude/Codex files. AgentDeck-owned
  overlays, bindings, caches and detached imports live under `AGENTDECK_HOME` only.
- **Native first.** Do not copy setup assets or reimplement their full merge semantics when the CLI
  can discover them from its real `cwd`/home. Parse only the subset AgentDeck must display, select,
  validate, fingerprint or override.
- **No secret ingestion.** Never read auth-store contents. Redact literal `env`, token/header and
  credential values before a result can enter an API response, cache, log, error or snapshot.
- **Terminal interface is rejected** for both new types at launch AND switch validation with
  `422 terminal_unavailable` — mirroring the Codex-terminal decision (no verified hook path means
  a statusless agent, which drops the spec).
- **Never write into the user's own CLI config** (`opencode.json`, `~/.openhands/config.toml`,
  `settings.json`). All injection is env-var- or per-launch-artifact-based and torn down with the
  agent registration.
- Everything unverifiable without credentials/CLIs is **GATED** (same class as the Phase 1
  real-CLI and Codex acceptances) and listed in §8.4; the fakeacp-backed paths must be green
  regardless.

External surface facts this spec relies on (researched 2026-07; re-verify in 7.4):

| | OpenCode | OpenHands |
|---|---|---|
| Binary / ACP mode | `opencode acp` | `openhands acp` |
| Model id form | `provider/model` (e.g. `anthropic/claude-sonnet-4-5`) | `LLM_MODEL` env / `[llm]` TOML |
| Auth | `opencode auth login` → `~/.local/share/opencode/auth.json`; provider env vars | `LLM_API_KEY` (+`LLM_BASE_URL`) env; `~/.openhands/settings.json` |
| Yolo | `permission` config block; `OPENCODE_CONFIG_CONTENT` env carries a full config JSON | always-approve approval mode (ACP-exposed); `--always-approve` in TUI |
| MCP config | `opencode.json(c)` `mcp` block (JSON) | `config.toml [mcp]` (TOML) |
| Interactive resume | `--continue` / `--session <id>` (TUI) | `--resume <id>` (TUI) |

Configuration-federation facts to re-verify against pinned CLI versions in 7.8:

| Surface | Claude Code | Codex |
|---|---|---|
| User config | `~/.claude/settings.json` and user `.claude/` assets | `${CODEX_HOME:-~/.codex}/config.toml`, profile files; user skills under `~/.agents/skills` |
| Project config | `.claude/settings.json`, `.claude/settings.local.json`, `CLAUDE.md`, `.claude/{rules,skills,agents}` | trusted `.codex/config.toml` layers, `AGENTS.md`, `.agents/skills` |
| High-level fields | `model`, `availableModels`, `fallbackModel`, `effortLevel`, provider env metadata | `model`, `model_provider`, `model_providers`, `model_reasoning_effort`, `model_verbosity`, `model_catalog_json`, profile |
| Setup/tooling | instructions/imports, rules, skills, subagents, hooks, plugins, `.mcp.json`/user MCP | AGENTS chain, skills, agents/config layers, rules/hooks/plugins, `mcp_servers` |
| Precedence | managed → CLI → local → project → user | CLI → project (closest) → selected profile → user → system → built-in, subject to managed requirements |

Primary references: [Claude settings](https://code.claude.com/docs/en/settings),
[Claude instructions](https://code.claude.com/docs/en/memory),
[Claude skills](https://code.claude.com/docs/en/skills),
[Claude subagents](https://code.claude.com/docs/en/sub-agents),
[Codex config basics](https://developers.openai.com/codex/config-basic),
[Codex config reference](https://developers.openai.com/codex/config-reference),
[Codex AGENTS.md](https://developers.openai.com/codex/guides/agents-md), and
[Codex skills](https://developers.openai.com/codex/skills).

---

## 2. Configuration federation (F16)

### 2.1 Authority and resolution model

The stored configuration and effective configuration are different types:

```text
external native layers ─┐
                        ├─ SourceResolver ─ effective backend + provenance ─ launch snapshot
AgentDeck binding/override/fallback ┘                 └─ redacted UI inventory
```

- `linked`: `config-sources.json` is only a pointer/ownership manifest. Resolution reads current
  native files; no normalized config is persisted.
- `mirrored`: same authority as linked, plus a derived last-known-good cache under
  `cache/config-sources/`. The cache is never edited, never merged back, and may be deleted.
- `detached`: no active binding. Selected non-secret values/assets are materialized under
  AgentDeck ownership and thereafter use existing config CRUD semantics.
- `backends.json` remains the fallback for unbound fields/backends. A binding declares the fields it
  owns, so seeded values cannot accidentally mask a linked external default.
- Native managed policy/requirements are constraints, never importable overrides. AgentDeck must
  not offer a control that claims to bypass them.

Resolution is per `(backend_id, project_id, source_generation)`, not globally per backend: project
layers depend on `project.cwd`. Provider-specific precedence is implemented inside the provider
resolver and returned as per-field provenance rather than flattened into a fake universal order.

### 2.2 Stored schema: `config-sources.json` v1

Add typed storage in `internal/config/sources.go`; seed no bindings (discovery is not consent):

```go
type ConfigSources struct {
    Version int                      `json:"version"` // == 1
    Sources map[string]SourceBinding `json:"sources"` // backend_id
}

type SourceBinding struct {
    Provider   string          `json:"provider"` // claude-code | codex
    Mode       string          `json:"mode"`     // linked | mirrored
    Root       string          `json:"root"`     // canonical absolute user root
    Profile    string          `json:"profile,omitempty"`
    Claims     []string        `json:"claims"` // launch_defaults, model_catalog, setup
    Overrides  SourceOverrides `json:"overrides,omitempty"`
    Approved   []string        `json:"approved_roots"`
}

type SourceOverrides struct {
    Model  *string `json:"model,omitempty"`  // empty string means native/default
    Effort *string `json:"effort,omitempty"`
}
```

`detached` is an action/result, not a persisted binding mode: detaching removes the binding after
materialization. Store roots as canonical absolute paths after preview; retain the user-facing
unexpanded input only in the preview response. `Approved` contains canonical roots the user saw
(user root, project root, and any explicit imported instruction root). A symlink target change that
escapes this set changes health to `approval_required`; the watcher does not silently follow it.

Do not put effective values, inventory, last refresh time or errors in this file. Those are derived
runtime state. Validate provider/backend pairing (`claude-acp` ↔ `claude-code`, `codex-acp` ↔
`codex`), profile syntax, claims union, path absoluteness, and one binding per backend.

### 2.3 Resolver package and provider maps

Add `internal/configsource`:

```go
type Resolver interface {
    Discover(ctx context.Context, project config.Project) []Candidate
    Preview(ctx context.Context, binding Binding, project config.Project) (Effective, Report, error)
    Resolve(ctx context.Context, binding Binding, project config.Project) (Effective, Report, error)
}
```

`Effective` contains optional `Model`, `Effort`, `Provider`, configured/catalogued `Models`, native
pass-through `Assets`, redacted environment-key metadata and `Provenance` per field. `Report`
contains canonical files read, skipped paths with reason, unknown/pass-through keys, warnings,
fingerprints and source generation. Never place raw TOML/JSON or secret-bearing values in either.
Each asset also reports `detachability: copyable | reference_only | unsupported`; do not imply that
a native-discovery-only skill/agent/rule can be detached until a provider-specific launch injection
path has passed acceptance.

Provider rules:

- **Claude:** resolve the documented managed/CLI/local/project/user precedence for the subset above.
  Inventory `CLAUDE.md` plus imports, `.claude/rules`, skills, subagents, settings/hooks/plugins and
  MCP declarations. Do not expand instruction contents into AgentDeck config; record path, scope,
  metadata and SHA-256. `availableModels` is an allowlist/catalog hint, not entitlement discovery.
- **Codex:** resolve user config, selected `$CODEX_HOME/<profile>.config.toml`, and trusted project
  `.codex/config.toml` layers root→cwd for supported fields. Inventory the AGENTS chain,
  `.agents/skills`, agents/config files, rules/hooks/plugins and MCP declarations. Respect fields
  forbidden in project config (for example provider/profile definitions). `model_catalog_json` may
  add configured models but is not an account models API.
- Unknown valid keys are `native_passthrough`; invalid syntax is an error. Use a TOML parser that
  preserves types (`github.com/pelletier/go-toml/v2`), `encoding/json`, and YAML frontmatter parsing
  already present in the module where possible. Do not shell out to either CLI for normal refresh.

The actual CLI remains the final authority. Launch composition omits a model/effort flag when the
effective choice is `inherit`, allowing native resolution; an observed model from ACP is persisted as
session state/provenance, never written back as an override.

### 2.4 Native pass-through and runtime-home boundary

Launch Claude/Codex with the selected project `cwd` and its native user configuration root so the CLI
loads project/user instructions and setup itself. AgentDeck layers only:

1. explicit launch choice / source override;
2. role + project prompt append;
3. AgentDeck identity/hook environment;
4. reserved per-session messaging MCP.

This supersedes Phase 6 §6.1's blanket rule that `CODEX_HOME` points at an isolated AgentDeck session
store. Do not hide native Codex config/auth behind an empty home. Preferred order:

1. use the user's real `CODEX_HOME` and capture ACP/native session ids without relocating the store;
2. if the pinned adapter requires isolation, construct an AgentDeck runtime home containing
   symlinks/pointers to the approved read-owned config/profile/auth entries and separate
   AgentDeck-owned session output paths;
3. if either cannot preserve native behavior, mark that surface `mirrored` and gate it in 7.8.

Never copy an entire unknown home tree. User skills remain at their native `$HOME/.agents/skills`
location. Project instructions/assets remain relative to the real `cwd`.

External MCP servers stay native. Inject messaging with the reserved id `agentdeck-messaging-<id>`;
preflight fails `409 source_conflict` if the effective native config already uses that exact id.

### 2.5 Refresh, consistency and failure policy

Add `SourceManager` owned by the server:

- `fsnotify` watches every resolved file's parent plus approved discovery directories. Debounce per
  binding/project for 250 ms; rebuild watches after atomic rename or symlink changes.
- A 30-second stat/fingerprint sweep catches missed events. A launch always calls `ResolveFresh`,
  comparing file identity/mtime/size and SHA-256 where metadata changed before composing args.
- Parse into a new immutable generation; stat inputs before and after parse and retry once if they
  changed. Publish `config_source_update` over SSE only after the generation swaps atomically.
- Linked mode caches only in memory. Mirrored mode writes a redacted normalized cache atomically
  under `cache/config-sources/<backend>/<project>.json`; permissions `0600`, directory `0700`.
- On malformed/missing/unapproved input, keep last-known-good for display with `stale:true`, but
  `ResolveFresh` returns `422 source_invalid`/`409 approval_required`; never launch from stale cache.
- Manual refresh uses the same path. Watch events do not write `config-sources.json` or
  `backends.json`, so AgentDeck cannot create a feedback loop.

High-level model/provider/effort values are frozen into the existing session launch snapshot with
binding id, profile, generation and fingerprints. Resume uses that frozen high-level snapshot by
default. Native setup assets can change between processes because the CLI owns their loading; expose
the fingerprint delta and an explicit “Resume with latest setup” choice rather than claiming content
was frozen.

Add state migration v8 with `sessions.launch_config_json TEXT NOT NULL DEFAULT '{}'`. The redacted,
versioned JSON stores requested vs resolved model/effort/provider, binding backend/provider/profile,
source generation/fingerprints and whether native defaults were inherited. Mirror the same object in
`SessionMetaData`/transcript `session_meta` so reindex can rebuild it. Existing `sessions.model` stays
the display/search compatibility projection; an ACP-observed model is separate runtime state and must
not rewrite the immutable launch object.

### 2.6 Secret and trust boundary

- Never open known auth stores (`~/.claude` credential files, Codex `auth.json`, OS keychains) for
  federation. Credential checks remain separate and return status only.
- While parsing settings, redact values under `env`, token/key/header/auth/helper fields before
  constructing diagnostics. Logs use path + key name + error class only.
- Preview is read-only and returns all canonical roots. `PUT` requires a preview token bound to the
  same provider/root/profile/fingerprints and expiring after 10 minutes; this makes consent explicit
  without trusting client-submitted paths after preview.
- Project layers load only for the AgentDeck-selected project and only when the provider's trust rule
  allows them. Managed/system policy is inventoried as `managed` without exposing protected values.
- Detached materialization excludes secret-bearing fields by construction. High-level values go to
  `backends.json`; only assets marked `copyable` go under
  `imports/config-sources/<backend>/<snapshot-id>/` with a native-relative manifest and `0600` files.
  Reference-only assets remain unchecked in the confirmation UI and are not copied. The user may
  later add AgentDeck-owned env values through the existing editor, which follows existing
  secret-storage risk.

### 2.7 REST and SSE contracts

Add routes:

```text
GET    /api/config-sources?project=<id>
POST   /api/config-sources/preview
PUT    /api/config-sources/{backend_id}
POST   /api/config-sources/{backend_id}/refresh?project=<id>
DELETE /api/config-sources/{backend_id}?detach=false|true&project=<id>
```

Preview body: `{provider, root:"auto"|<path>, profile?, mode, claims[], project}`. Response includes
`preview_token`, `expires_at`, redacted effective values, inventory, provenance, exact read/skipped
paths, warnings and fingerprints. `PUT` sends `{preview_token, overrides}`; the server rebuilds and
compares the preview before atomic persistence. `GET` returns discovery candidates and active
bindings/status, never secrets.

`config_source_update` SSE data:

```json
{"backend_id":"codex","project_id":"agentdeck","generation":4,
 "health":"ok","changed":["model","effort","skills"],"stale":false}
```

Use existing error envelopes: `400 invalid_field`, `404 source_not_found`, `409 source_changed`,
`409 source_conflict`, `409 approval_required`, `422 source_invalid`. External parse errors are
sanitized and may include line/column, never raw source snippets.

### 2.8 UI behavior

Add a **Configuration source** panel to Claude/Codex backend cards and onboarding:

- discovery → preview → explicit **Link setup**; Linked is recommended, Mirrored is labelled a
  compatibility mode, and **Import detached copy** explains that auto-sync stops;
- effective model/effort controls show `Inherited from <scope/path>`, `AgentDeck override`, or
  `Inherit CLI default`; configured model lists include an honest “not an entitlement check” note;
- inventory groups Instructions, Skills, Agents, MCP, Hooks/Rules and Plugins with source/scope,
  enabled/pass-through/unsupported status and redacted warnings; file contents are not returned;
- live SSE invalidates the config-source and effective-backend queries; stale/invalid blocks launch
  with repair, refresh, detach and override actions;
- only Claude/Codex show federation controls in this phase. OpenCode/OpenHands remain locally managed.

### 2.9 Binary-versioned product knowledge MCP

After the Claude/Codex federation work is complete, add a small server-owned knowledge package for
AgentDeck itself. Its purpose is to prevent the seeded `agentdecker` role from carrying a frozen
copy of product facts: role files are intentionally written once and must not be silently replaced.

- `internal/knowledge/docs/*.md` contains curated, product-facing Markdown topics and is compiled
  into the binary with `go:embed`. A topic is a stable filename-derived name plus its first-heading
  title. The initial inventory covers overview, launching, configuration, dashboard, interfaces,
  archive, messaging, notifications and troubleshooting.
- `internal/knowledge` exposes copy-safe `Topics`, `Get`, and `Index` operations. It reads no
  checkout files, config files, homes, source bindings or credentials at runtime. Topic lookup
  rejects path separators and unknown names; content is static for the running binary.
- Register `agentdeck_docs` on the existing messaging MCP server, through the same per-agent MCP
  registration used for live agents. Its optional `topic` argument returns the index when empty,
  exact Markdown for a known topic, or an `IsError` response that lists valid names. Do not add a
  separate HTTP documentation route or expose it to an unregistered/revoked MCP caller.
- Replace the fresh seed's long factual `agentdecker` prompt with persona, orchestration behavior,
  and a short instruction to consult `agentdeck_docs` before answering non-trivial questions about
  AgentDeck. Never rewrite a pre-existing role file; the MCP tool is independently available to
  all live registered agents.
- Topics describe only released behavior and safe operational guidance. They contain no literal
  secrets, auth-store paths or copied external-config contents, and Phase 7 work remains absent
  until it ships. Product changes update their matching topic in the same checkpoint.

---

## 3. Backend adapters

### 3.1 `opencodeACP` (`internal/backend/adapter.go`)

- `Type()` → `"opencode-acp"`; `Binary()` → `"opencode"`; `LaunchArgs(spec)` → `["acp"]`.
- **Model:** pass the configured model id through the existing ACP `session/new` model param
  (`acpmap.go::sessionNewParams` already forwards `LaunchSpec.ModelID`; the adapter does not
  translate — OpenCode model ids are already `provider/model` strings in backend config).
- **Env:** compose from backend config `env` (provider API keys). `StripEnvKeys()` returns
  `["CLAUDECODE", "OPENCODE_CONFIG", "OPENCODE_CONFIG_CONTENT"]` — the latter two so a user's
  shell-level config override never leaks into a dashboard-managed agent (we may set our own,
  §3.3).
- **Resume:** `ResolveResumeID(prev)` returns `prev` (attempt native ACP `session/load`);
  `CanSwitchModelOnResume()` → `true` (model is a per-session param). GATED: if 7.4 finds
  `session/load` unsupported, flip `ResolveResumeID` to return `""` (forces the Phase 6 primer)
  — a one-line adapter change by design.
- **Hooks:** `HookMap()` → nil, `UnsupportedHookEvents()` → all five, `HookLaunchArgs()` → nil.
  Chat status derives from the ACP stream, as for every chat agent.

### 3.2 `openhandsACP`

- `Type()` → `"openhands-acp"`; `Binary()` → `"openhands"`; `LaunchArgs(spec)` → `["acp"]`.
- **Model/auth env:** the adapter maps backend config → `LLM_MODEL=<model id>`, plus passthrough
  of `LLM_API_KEY`/`LLM_BASE_URL` from backend `env`. This is the one adapter where model
  selection rides env, not the session param — implement as an adapter method
  (`ExtraEnv(spec) []string`, new optional interface, default nil) rather than a runtime branch.
- `StripEnvKeys()` → `["CLAUDECODE", "LLM_MODEL"]` (never inherit the shell's model).
- **Resume:** as §3.1 — try native `session/load`, primer fallback via `""`; GATED verification.
  `CanSwitchModelOnResume()` → `true` (model is env-per-launch; a relaunch re-sets it).
- **Hooks:** none, same as §3.1.

### 3.3 Permission / yolo mapping

`LaunchSpec.SkipPerms` (frozen in the session snapshot, migration v7 rule — do NOT re-resolve):

- **Default (skip=false):** nothing injected. Both CLIs raise ACP permission requests; the
  existing withhold-the-response gate (`internal/runtime/permission.go`) surfaces them as
  dashboard cards. No adapter work.
- **OpenCode skip=true:** adapter `ExtraEnv` sets
  `OPENCODE_CONFIG_CONTENT={"permission":{"edit":"allow","bash":"allow","webfetch":"allow"}}`.
  Env-only: nothing on disk, torn down with the process. GATED: exact key set re-verified in 7.4.
- **OpenHands skip=true:** prefer setting the ACP session permission mode at `session/new` if the
  CLI advertises an always-approve mode in its ACP `modes` (the runtime already receives modes;
  add `LaunchSpec.SkipPerms` → mode selection in `sessionNewParams` ONLY if claude's path already
  does this — otherwise adapter `LaunchArgs` appends the CLI's approval flag). Resolution of
  which arm is 7.4's first question; until verified, ship the mode-selection arm behind the
  adapter and mark GATED.

### 3.4 Messaging MCP

`RegisterMessagingMCP` already emits an HTTP MCP entry per agent and the runtime passes
`LaunchSpec.MCPServers` into ACP `session/new` — which is exactly how ACP clients are supposed to
hand MCP servers to agents. Both new backends take that same path; **no per-agent config-file
injection**. GATED: whether each CLI honors `session/new` `mcpServers` entries of type HTTP is a
7.4 acceptance item; on rejection, the fallback is documented-not-built (same posture as the
Claude/Codex HTTP-vs-stdio gate still open in HANDOFF).

---

## 4. Backend config, seed, validation

- `internal/config/types.go`: comment on `Backend.Type` widens to
  `"claude-acp" | "codex-acp" | "opencode-acp" | "openhands-acp"`.
- `internal/config/seed.go` `DefaultBackends()`: add seeded entries `opencode`
  (type `opencode-acp`, default model `anthropic/claude-sonnet-4-5`) and `openhands`
  (type `openhands-acp`, default model `anthropic/claude-sonnet-4-5`, env keys
  `LLM_API_KEY`/`LLM_BASE_URL` present-but-empty). Seeds must not change the default backend id
  (`defaultBackendID()` stays `"claude"`).
- Backend PUT validation: unknown `type` already fails via `backend.For` at the runtime gate;
  add an explicit 400 `invalid_field` at `PUT /api/backends` for types outside the four-value
  union so Settings errors early, not at launch.
- **Credential checks** (`internal/backend/credcheck/`): add `opencode.go` (binary on PATH +
  `~/.local/share/opencode/auth.json` exists or a provider key in backend env) and
  `openhands.go` (binary on PATH + `LLM_API_KEY` in backend env or `~/.openhands/settings.json`
  exists). Same shape/timeout as `claude.go`; checks run in the existing (currently serial)
  `PUT /api/backends` validation path.

---

## 5. Backend server seam (`internal/server`)

- `composeLaunch` / `composeResumeSpec` / `validateSwitchTarget`: generalize the codex-terminal
  rejection — replace `backend.Type == "codex-acp"` with a helper
  `terminalSupported(backendType) bool` (true only for `claude-acp`) used in ALL THREE composers
  (this is the classic launch/resume/switch drift hot spot; one helper, three call sites).
- No other composer changes: `LaunchSpec` fields, hook-registration composition
  (`composeHookRegistration` finds no hook map → no settings file, no flag), MCP registration,
  and teardown (`teardownAgentRegistration`) are already adapter-driven and backend-agnostic.
- Switch-runtime: no endpoint changes. The matrix widens automatically — cross-backend swaps hit
  the primer path when `resolveResumeId` yields no native id.

---

## 6. Backend UI (`ui/src`)

- `schemas/backends.ts`: `z.enum(["claude-acp","codex-acp","opencode-acp","openhands-acp"])` —
  the single source of the union; everything else derives from it.
- Add `lib/backendTypes.ts`: `BACKEND_TYPE_LABELS: Record<BackendType, string>`
  (`Claude`, `Codex / OpenAI`, `OpenCode`, `OpenHands`) — replaces the per-component inlined
  ternaries in `BackendStep.tsx` / `BackendsEditor.tsx` / `NewAgentModal.tsx` (three-way drift
  risk otherwise).
- `features/onboarding/steps/BackendStep.tsx` + `features/settings/BackendsEditor.tsx`: options
  render from the enum + label map; per-type conditional fields: `openhands-acp` shows
  `LLM_API_KEY`/`LLM_BASE_URL` env inputs (mirroring the existing `codex-acp` conditional
  fields); `opencode-acp` needs none (auth is CLI-side login). **Merge-over-seed discipline
  applies** (INVARIANTS: forms over seeded config merge, never replace).
- `NewAgentModal.tsx`: interface picker hides/disables Terminal when the selected backend type
  is not `claude-acp` (drive from a `terminalSupported` mirror of §5, or from `/api/capabilities`
  if it grows a per-backend field — do the simple client mirror; capabilities growth is out of
  scope).
- Rebuild + `make embed` for the dist refresh.

---

## 7. Error handling & edge cases

- Binary not installed: the chat runtime's spawn failure already maps to the launch error path;
  ensure the message names the binary (existing behavior via exec error) — no new code expected,
  covered by a test.
- Terminal request for new types: `422 terminal_unavailable` (existing code constant), asserted
  at launch, resume-with-override, and switch.
- Unknown backend type at launch: existing `backend.For` miss → current error path; new 400 at
  config PUT (§4) prevents most of these earlier.
- Env hygiene: `StripEnvKeys` per §3 so dashboard agents never inherit shell-level
  model/config overrides.

---

## 8. Subphase plan (incremental / quota-limited implementation)

Every subphase ends GREEN: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`
(+ `cd ui && npm run build` when ui/ is touched). Never commit on red.

### Subphase 7.1 — Adapters + config + gates (Go core)
- **Goal:** Both backends launch/stream/stop/resume through the chat runtime against fakeacp.
- **Deliverables:** `opencodeACP` + `openhandsACP` adapters (§3.1–3.2) incl. the `ExtraEnv`
  optional interface; seed entries + type-union validation (§4); `terminalSupported` helper
  replacing the codex literal in all three composers (§5); fakeacp e2e per backend
  (launch→prompt→stream→stop→native-resume), terminal-rejection tests for both types across
  launch/resume/switch (tasks 1–4).
- **Depends on:** Phase 6 complete (or 6.7 skipped).
- **Done when (checkpoint):** GREEN (Go-only); `TestOpenCodeChatE2E`, `TestOpenHandsChatE2E`,
  `TestNewBackendTerminalRejected` pass both tag variants.
- **Resume note:** At start, `backend.For` knows two types and the composers hard-code
  `"codex-acp"` in the terminal gates. Start with the adapters, then the gate helper, then seeds.
- **Size:** M

### Subphase 7.2 — Permissions, credchecks, switch matrix
- **Goal:** Yolo mapping, credential validation, and cross-backend switching are wired and tested.
- **Deliverables:** skip=true env/mode injection per §3.3 (fakeacp asserts the spawned env /
  session-mode); `credcheck/opencode.go` + `credcheck/openhands.go` (§4) with fs/env-faked tests;
  switch-runtime integration test claude→opencode primer path (§5); PUT /api/backends 400 on
  unknown type (tasks 5–7).
- **Depends on:** 7.1.
- **Done when (checkpoint):** GREEN (Go-only); `TestSkipPermissionsEnvOpenCode`,
  `TestSwitchClaudeToOpenCodePrimer` pass.
- **Resume note:** At start, both backends launch permission-gated only; skip=true is a no-op
  and Settings Validate errors for the new types.
- **Size:** M

### Subphase 7.3 — UI plumbing
- **Goal:** All four backends are creatable, validatable, and launchable from the UI.
- **Deliverables:** enum + `BACKEND_TYPE_LABELS`; BackendStep/BackendsEditor per-type fields with
  merge-over-seed tests; NewAgentModal terminal-option gating; embedded dist refresh (tasks 8–10).
- **Depends on:** 7.1 (types exist server-side); parallel-safe with 7.2.
- **Done when (checkpoint):** GREEN incl. `cd ui && npm run test` + `npm run build` + `make embed`.
- **Resume note:** At start, the zod enum still rejects the new types, so the UI cannot even
  render a seeded opencode backend — do the schema first.
- **Size:** S

### Subphase 7.4 — GATED live acceptance (human-credentialed)
- **Goal:** Verify every GATED assumption against real CLIs; correct adapters where wrong.
- **Deliverables:** `//go:build acceptance` tests per backend (handshake, one streamed turn,
  permission round-trip, stop, resume-or-primer verdict, `mcpServers` honor verdict); HANDOFF
  acceptance-gate entries resolved or refined; adapter one-liners flipped per verdicts
  (§3.1 resume, §3.3 arms, §3.4) (tasks 11–12).
- **Depends on:** 7.1–7.3; human with `opencode` + `openhands` installed and provider keys.
- **Done when (checkpoint):** GREEN; acceptance runs recorded in HANDOFF (pass or documented
  deviation per CLI).
- **Resume note:** Everything is fakeacp-green before this; nothing in 7.4 may regress the fake
  paths. If credentials never arrive, the phase ships with the gates documented (Codex precedent).
- **Size:** S (code) + human time

### Subphase 7.5 — Federation schema + provider resolvers

- **Goal:** Resolve a redacted, provenance-bearing effective Claude/Codex view from fixture trees
  without changing any external file.
- **Deliverables:** `config-sources.json` typed store/validation (§2.2); `configsource` interface,
  Claude JSON/instruction inventory and Codex TOML/AGENTS inventory (§2.3); canonical-path approval,
  fingerprints and centralized redaction (§2.6); fixture matrices for precedence, unknown fields,
  imports/symlinks, profiles, project trust and malformed sources (tasks 13–16).
- **Depends on:** 7.1–7.3; independent of gated 7.4.
- **Done when (checkpoint):** GREEN (Go-only); `TestClaudeResolverPrecedence`,
  `TestCodexResolverPrecedence`, `TestResolverNeverReturnsSecrets`, and
  `TestBindingDoesNotWriteSource` pass under both Go test variants.
- **Resume note:** F16 has no implementation in the pre-revision tree. Start with pure resolvers and
  fixture roots; do not add watchers/endpoints until their results are deterministic and redacted.
- **Size:** M

### Subphase 7.6 — Source manager + API + launch integration

- **Goal:** Linked/mirrored bindings refresh safely and every new launch consumes a fresh effective
  snapshot while resume defaults remain frozen.
- **Deliverables:** `SourceManager`, fsnotify + 30s sweep + atomic generation/cache (§2.5); preview
  token and source routes (§2.7); effective resolver wired into launch/resume/switch composition;
  native `cwd`/home pass-through and Codex-home correction (§2.4); session provenance/fingerprints;
  `config_source_update` SSE; reserved MCP collision preflight (tasks 17–21).
- **Depends on:** 7.5.
- **Done when (checkpoint):** GREEN (Go-only); tests prove atomic rename and missed-event recovery,
  launch-time freshness, stale-invalid launch blocking, frozen resume, `config_refresh:true`, preview
  TOCTOU rejection, no source writes and no secrets in API/cache/log/snapshot.
- **Resume note:** Extend `POST /api/sessions/{id}/resume` optional body with
  `config_refresh:true` for “Resume with latest setup”; absent/false preserves the frozen high-level
  snapshot. Source changes never rewrite active session state.
- **Size:** L

### Subphase 7.7 — Federation onboarding + Settings UI

- **Goal:** A user can preview/link, understand provenance and health, override/reset fields,
  refresh, detach and relink without reading config files manually.
- **Deliverables:** schemas/API hooks; onboarding source step; Settings source panel/inventory;
  inherited/override/default states in model/effort controls; stale/approval/conflict repair UX; SSE
  query invalidation; detached-import confirmation; embedded dist (tasks 22–24).
- **Depends on:** 7.6.
- **Done when (checkpoint):** full GREEN plus UI tests for preview-before-write, provenance labels,
  external-update refresh, invalid-source launch gate, reset-to-inherit and detach semantics.
- **Resume note:** Never render source contents or secret values. Paths and field names are enough
  for diagnostics; configured models carry the entitlement caveat.
- **Size:** M

### Subphase 7.8 — GATED live federation acceptance

- **Goal:** Re-verify provider mappings and native pass-through against pinned real Claude/Codex CLIs.
- **Deliverables:** acceptance-tagged temp-home/project cases plus an opt-in real-user-config smoke
  test that is read-only; verify model/effort precedence, instruction/skill/agent/MCP visibility,
  settings reload/restart boundaries, Codex native-home behavior and redaction; update provider maps
  and reference date from observed results (tasks 25–26).
- **Depends on:** 7.5–7.7; human with installed/authenticated CLIs and disposable test config roots.
- **Done when (checkpoint):** GREEN; live results recorded in HANDOFF, with any unsupported surface
  moved to mirrored or explicitly reported rather than silently copied.
- **Size:** S (code) + human time

### Subphase 7.9 — Binary-versioned AgentDeck knowledge MCP

- **Goal:** Live agents can retrieve concise, authoritative AgentDeck product guidance that matches
  the running binary, without AgentDeck overwriting a user's seeded role prompt or reading mutable
  repository documentation at runtime.
- **Deliverables:** embedded topic package and tests (§2.9); registered `agentdeck_docs` MCP tool
  with index, known-topic, unknown-topic and revoked-caller coverage; fresh AgentDecker seed-prompt
  reduction; release-facing topic set covering every shipped product surface (tasks 27–30).
- **Depends on:** 7.5–7.8. It intentionally follows completed Claude/Codex federation so the
  published configuration guidance describes the shipped, verified behavior rather than a plan.
- **Done when (checkpoint):** full GREEN; tests prove the binary serves its embedded topic index,
  rejects traversal/unknown topics and unregistered callers, never emits sentinel secret values,
  and leaves existing role files untouched during seeding.
- **Resume note:** This is an AgentDeck-owned, read-only MCP capability, not another configuration
  source. Do not copy the old knowledge branch's docs without revalidating every product claim.
- **Size:** S

---

## 9. Implementation task breakdown

1. `internal/backend/adapter.go`: `opencodeACP`, `openhandsACP`, `ExtraEnv` optional interface;
   `internal/runtime/chat.go` consumes `ExtraEnv` via the existing adapter lookup (additive).
2. `internal/config/seed.go` + `types.go`: seeds, union comment.
3. `internal/server`: `terminalSupported` helper; replace three codex literals.
4. fakeacp e2e ×2 + terminal-rejection tests ×3 paths.
5. §3.3 skip mapping + spawned-env assertions.
6. `credcheck/{opencode,openhands}.go` + tests.
7. Switch primer integration test + backends-PUT type validation.
8. `ui/src/schemas/backends.ts` + `lib/backendTypes.ts`.
9. BackendStep / BackendsEditor / NewAgentModal updates + tests.
10. `make embed` dist refresh.
11. Acceptance-tagged tests ×2 backends.
12. HANDOFF gate resolution + adapter corrections.
13. `internal/config/sources.go`: v1 store, path/provider/claims validation, atomic writes.
14. `internal/configsource`: shared types, redaction, fingerprinting and approved-root policy.
15. Claude resolver + fixture precedence/setup inventory tests.
16. Codex resolver + fixture precedence/profile/project-trust/setup inventory tests.
17. `SourceManager`: watch, sweep, immutable generations, mirrored cache and SSE publication.
18. Source discovery/preview/bind/refresh/detach handlers with expiring preview tokens.
19. State migration v8 + launch/resume/switch effective resolution, frozen provenance and refresh option.
20. Preserve native Claude/Codex homes/cwd; replace Phase 6 isolated-Codex-home assumption.
21. Reserved messaging MCP collision preflight + failure/redaction integration tests.
22. UI schemas and API hooks; onboarding discovery/preview/link flow.
23. Settings source/provenance/inventory/health/override/detach UI + tests.
24. SSE query invalidation, launch gates, dist build and `make embed`.
25. Acceptance fixtures against pinned CLI versions using disposable homes/projects.
26. Opt-in read-only smoke against real user sources; record/reconcile verdicts.
27. `internal/knowledge`: embedded topic index/lookup package and unit tests.
28. `internal/messaging`: registered `agentdeck_docs` MCP tool with registered-caller and error-path tests.
29. `internal/config/seed.go`: fresh AgentDecker persona prompt points to the tool without changing
    existing role files.
30. Curated release-matched topics plus secret/future-feature content sweep.

## 10. Testing strategy

- All protocol-level behavior against **fakeacp** (env-driven; extend scenarios only if a needed
  behavior — e.g. reject `session/load` — isn't already scriptable).
- One `_test.go` per touched source file; regression names describe the defect class
  (`TestNewBackendTerminalRejected`, `TestOpenHandsEnvNeverInheritsShellModel`).
- UI: Vitest + MSW for editor merge-preserve and modal gating; no live server.
- Federation fixture trees use temp homes/projects and assert source trees are byte-identical after
  every preview/resolve/refresh/detach-without-materialization test.
- Table-test provider precedence and provenance separately; never validate only the flattened value.
- Property/fuzz tests feed nested secret-like keys and malformed JSON/TOML/frontmatter through every
  report/error/cache/snapshot encoder and assert sentinel secret bytes never escape.
- Watch tests cover in-place writes, atomic rename, delete/recreate, symlink target change and a
  deliberately dropped event recovered by the sweep/launch freshness check.
- Concurrency test resolves while files change and proves readers see generation N or N+1, never a
  mixed view. API tests prove preview-token expiry and fingerprint/TOCTOU rejection.
- Live behavior exclusively behind `//go:build acceptance` (§8.4 and §8.8).
- Knowledge tests invoke the MCP tool through a registered and a revoked session, assert a
  deterministic index and topic body, and scan every result for injected secret sentinels. Seed
  tests prove an existing `agentdecker` role remains byte-identical after an upgrade.

## 11. Resolved decisions (answers to PRD §6)

- **Authority:** linked/mirrored sources are one-way, external-authoritative. Detached snapshot is
  AgentDeck-authoritative. No two-way merge or external writes.
- **Pointers vs copies:** native CLI discovery/pointers are the default; a redacted cache is derived
  compatibility state only. Setup assets are not normalized into an AgentDeck universal schema.
- **Freshness:** watch + periodic sweep improves UI latency; synchronous launch resolution is the
  correctness boundary. Stale last-known-good is display-only.
- **Sessions:** high-level values freeze per session; source fingerprints record setup drift. Native
  assets may be reread when a CLI process starts, and explicit refreshed resume makes that visible.
- **Models:** only configured/allowlisted/catalogued models are shown. Missing means inherit native
  default; it is never replaced with a guessed id or marketed as account entitlement discovery.
- **Secrets:** auth stores are never opened, and secret-bearing settings are redacted before leaving
  the resolver. Detached import excludes them.

- **Native resume:** attempt `session/load` for both; adapters return `""` → primer on a failed
  7.4 verdict. The primer path is the guaranteed floor.
- **Yolo:** OpenCode via `OPENCODE_CONFIG_CONTENT` env (nothing on disk); OpenHands via ACP
  session mode, flag-arm fallback — final arm picked in 7.4.
- **MCP:** ride ACP `session/new` `mcpServers` (the protocol-intended path); no config-file
  injection; rejection verdict documented, fallback not built this phase.
- **Env hygiene:** strip `CLAUDECODE` (both, inherited guard), `OPENCODE_CONFIG*` (OpenCode),
  `LLM_MODEL` (OpenHands) from inherited env before adapter-composed values are applied.
