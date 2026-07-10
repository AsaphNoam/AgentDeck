# Phase 7 — Implementation Tech Spec: OpenHands & OpenCode backends

**Mirrors:** `docs/features/phase-7-additional-features.md` (phase PRD)
**Features:** F14 (OpenCode backend), F15 (OpenHands backend); extends F7 (switch-runtime matrix)
**Depends on:** Phase 1 (chat runtime), Phase 6 (`BackendAdapter` seam, switch-runtime, terminal gates)
**Audience:** the engineer implementing this phase. Prescriptive; no further design decisions required.

---

## 1. Overview & scope recap

Add two chat backends — **OpenCode** (`opencode acp`) and **OpenHands** (`openhands acp`) — through
the existing ACP chat runtime. Both CLIs implement the Agent Client Protocol natively over stdio,
the same wire the runtime already speaks to `claude-code-acp`/`codex-acp`, so **no runtime changes
are permitted**: every per-agent difference is captured in a `BackendAdapter`
(`internal/backend/adapter.go`), per the Phase 6 §6.3 isolation rule. The chat runtime, transport,
`acpmap.go`, state, persistence, SSE, and permission gate stay byte-identical.

Hard constraints:

- **No new runtime, no new endpoints.** The REST surface is unchanged; only accepted values widen.
- **Terminal interface is rejected** for both new types at launch AND switch validation with
  `422 terminal_unavailable` — mirroring the Codex-terminal decision (no verified hook path means
  a statusless agent, which drops the spec).
- **Never write into the user's own CLI config** (`opencode.json`, `~/.openhands/config.toml`,
  `settings.json`). All injection is env-var- or per-launch-artifact-based and torn down with the
  agent registration.
- Everything unverifiable without credentials/CLIs is **GATED** (same class as the Phase 1
  real-CLI and Codex acceptances) and listed in §12; the fakeacp-backed paths must be green
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

---

## 2. Backend adapters

### 2.1 `opencodeACP` (`internal/backend/adapter.go`)

- `Type()` → `"opencode-acp"`; `Binary()` → `"opencode"`; `LaunchArgs(spec)` → `["acp"]`.
- **Model:** pass the configured model id through the existing ACP `session/new` model param
  (`acpmap.go::sessionNewParams` already forwards `LaunchSpec.ModelID`; the adapter does not
  translate — OpenCode model ids are already `provider/model` strings in backend config).
- **Env:** compose from backend config `env` (provider API keys). `StripEnvKeys()` returns
  `["CLAUDECODE", "OPENCODE_CONFIG", "OPENCODE_CONFIG_CONTENT"]` — the latter two so a user's
  shell-level config override never leaks into a dashboard-managed agent (we may set our own,
  §2.3).
- **Resume:** `ResolveResumeID(prev)` returns `prev` (attempt native ACP `session/load`);
  `CanSwitchModelOnResume()` → `true` (model is a per-session param). GATED: if 7.4 finds
  `session/load` unsupported, flip `ResolveResumeID` to return `""` (forces the Phase 6 primer)
  — a one-line adapter change by design.
- **Hooks:** `HookMap()` → nil, `UnsupportedHookEvents()` → all five, `HookLaunchArgs()` → nil.
  Chat status derives from the ACP stream, as for every chat agent.

### 2.2 `openhandsACP`

- `Type()` → `"openhands-acp"`; `Binary()` → `"openhands"`; `LaunchArgs(spec)` → `["acp"]`.
- **Model/auth env:** the adapter maps backend config → `LLM_MODEL=<model id>`, plus passthrough
  of `LLM_API_KEY`/`LLM_BASE_URL` from backend `env`. This is the one adapter where model
  selection rides env, not the session param — implement as an adapter method
  (`ExtraEnv(spec) []string`, new optional interface, default nil) rather than a runtime branch.
- `StripEnvKeys()` → `["CLAUDECODE", "LLM_MODEL"]` (never inherit the shell's model).
- **Resume:** as §2.1 — try native `session/load`, primer fallback via `""`; GATED verification.
  `CanSwitchModelOnResume()` → `true` (model is env-per-launch; a relaunch re-sets it).
- **Hooks:** none, same as §2.1.

### 2.3 Permission / yolo mapping

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

### 2.4 Messaging MCP

`RegisterMessagingMCP` already emits an HTTP MCP entry per agent and the runtime passes
`LaunchSpec.MCPServers` into ACP `session/new` — which is exactly how ACP clients are supposed to
hand MCP servers to agents. Both new backends take that same path; **no per-agent config-file
injection**. GATED: whether each CLI honors `session/new` `mcpServers` entries of type HTTP is a
7.4 acceptance item; on rejection, the fallback is documented-not-built (same posture as the
Claude/Codex HTTP-vs-stdio gate still open in HANDOFF).

---

## 3. Config, seed, validation

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

## 4. Server seam (`internal/server`)

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

## 5. UI (`ui/src`)

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
  is not `claude-acp` (drive from a `terminalSupported` mirror of §4, or from `/api/capabilities`
  if it grows a per-backend field — do the simple client mirror; capabilities growth is out of
  scope).
- Rebuild + `make embed` for the dist refresh.

---

## 6. Error handling & edge cases

- Binary not installed: the chat runtime's spawn failure already maps to the launch error path;
  ensure the message names the binary (existing behavior via exec error) — no new code expected,
  covered by a test.
- Terminal request for new types: `422 terminal_unavailable` (existing code constant), asserted
  at launch, resume-with-override, and switch.
- Unknown backend type at launch: existing `backend.For` miss → current error path; new 400 at
  config PUT (§3) prevents most of these earlier.
- Env hygiene: `StripEnvKeys` per §2 so dashboard agents never inherit shell-level
  model/config overrides.

---

## 7. Subphase plan (incremental / quota-limited implementation)

Every subphase ends GREEN: `go build ./...`, `go test ./...`, `go test -tags sqlite_fts5 ./...`
(+ `cd ui && npm run build` when ui/ is touched). Never commit on red.

### Subphase 7.1 — Adapters + config + gates (Go core)
- **Goal:** Both backends launch/stream/stop/resume through the chat runtime against fakeacp.
- **Deliverables:** `opencodeACP` + `openhandsACP` adapters (§2.1–2.2) incl. the `ExtraEnv`
  optional interface; seed entries + type-union validation (§3); `terminalSupported` helper
  replacing the codex literal in all three composers (§4); fakeacp e2e per backend
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
- **Deliverables:** skip=true env/mode injection per §2.3 (fakeacp asserts the spawned env /
  session-mode); `credcheck/opencode.go` + `credcheck/openhands.go` (§3) with fs/env-faked tests;
  switch-runtime integration test claude→opencode primer path (§4); PUT /api/backends 400 on
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
  "Blocked on human" entries resolved or refined; adapter one-liners flipped per verdicts
  (§2.1 resume, §2.3 arms, §2.4) (tasks 11–12).
- **Depends on:** 7.1–7.3; human with `opencode` + `openhands` installed and provider keys.
- **Done when (checkpoint):** GREEN; acceptance runs recorded in HANDOFF (pass or documented
  deviation per CLI).
- **Resume note:** Everything is fakeacp-green before this; nothing in 7.4 may regress the fake
  paths. If credentials never arrive, the phase ships with the gates documented (Codex precedent).
- **Size:** S (code) + human time

---

## 8. Implementation task breakdown

1. `internal/backend/adapter.go`: `opencodeACP`, `openhandsACP`, `ExtraEnv` optional interface;
   `internal/runtime/chat.go` consumes `ExtraEnv` via the existing adapter lookup (additive).
2. `internal/config/seed.go` + `types.go`: seeds, union comment.
3. `internal/server`: `terminalSupported` helper; replace three codex literals.
4. fakeacp e2e ×2 + terminal-rejection tests ×3 paths.
5. §2.3 skip mapping + spawned-env assertions.
6. `credcheck/{opencode,openhands}.go` + tests.
7. Switch primer integration test + backends-PUT type validation.
8. `ui/src/schemas/backends.ts` + `lib/backendTypes.ts`.
9. BackendStep / BackendsEditor / NewAgentModal updates + tests.
10. `make embed` dist refresh.
11. Acceptance-tagged tests ×2 backends.
12. HANDOFF gate resolution + adapter corrections.

## 9. Testing strategy

- All protocol-level behavior against **fakeacp** (env-driven; extend scenarios only if a needed
  behavior — e.g. reject `session/load` — isn't already scriptable).
- One `_test.go` per touched source file; regression names describe the defect class
  (`TestNewBackendTerminalRejected`, `TestOpenHandsEnvNeverInheritsShellModel`).
- UI: Vitest + MSW for editor merge-preserve and modal gating; no live server.
- Live behavior exclusively behind `//go:build acceptance` (§7.4).

## 10. Resolved decisions (answers to PRD §6)

- **Native resume:** attempt `session/load` for both; adapters return `""` → primer on a failed
  7.4 verdict. The primer path is the guaranteed floor.
- **Yolo:** OpenCode via `OPENCODE_CONFIG_CONTENT` env (nothing on disk); OpenHands via ACP
  session mode, flag-arm fallback — final arm picked in 7.4.
- **MCP:** ride ACP `session/new` `mcpServers` (the protocol-intended path); no config-file
  injection; rejection verdict documented, fallback not built this phase.
- **Env hygiene:** strip `CLAUDECODE` (both, inherited guard), `OPENCODE_CONFIG*` (OpenCode),
  `LLM_MODEL` (OpenHands) from inherited env before adapter-composed values are applied.
