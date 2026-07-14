# FS-09 — Backends and model catalog

**Status:** Partial
**Code:** `internal/backend/`, `internal/config/{types,seed,validate}.go`, `internal/server/{config_handlers,terminal,launch,resume,switch}.go`, `ui/src/features/{settings,onboarding,launch}/` · **Journeys:** J2, J7, J9
**Absorbed:** [`agent-dashboard-prd.md`](../../archive/agent-dashboard-prd.md) F6 and the [phase archive manifest](../../archive/phases/README.md)

## 1. Purpose

AgentDeck supervises agent CLIs through named backend definitions. A backend selects one of four
ACP adapter types, owns a model catalog and defaults, may supply environment settings, and declares
the capability boundary AgentDeck can honestly offer. This spec covers backend/model configuration,
credential feedback, launch behavior and the shipped Claude, Codex, OpenCode and OpenHands matrix.
Configuration-source federation for Claude/Codex is FS-08.

## 2. Behavior

### Catalog and configuration

- **R1** — `backends.json` version 2 is a map of user-chosen backend ids to
  `{name,type,default,default_model,models,env}`. Each model is
  `{name,model,env}`; the map key is AgentDeck's selectable model id and `model` is the provider/CLI
  model string sent at launch.
- **R2** — The only valid backend types are `claude-acp`, `codex-acp`, `opencode-acp`, and
  `openhands-acp`. Unknown types are rejected on `PUT /api/backends` with a field-level
  `unknown_backend_type` error before they can reach launch.
- **R3** — A non-empty catalog has exactly one default backend. More than one is invalid; when none
  is marked, the lexicographically first backend id is promoted. Every backend has at least one
  model and a valid default model; an omitted default model is similarly promoted to the
  lexicographically first model id.
- **R4** — Every model's provider string is non-empty. Backend-level environment entries apply to
  all its models and model-level entries override the same keys. Saved configuration affects future
  launches; a running or frozen archived session is not hot-mutated.
- **R5** — A fresh home seeds four definitions without overwriting existing user files:
  `claude` (`claude-acp`, default), `codex` (`codex-acp`), `opencode`
  (`opencode-acp`), and `openhands` (`openhands-acp`). OpenCode's seeded model is provider-qualified;
  OpenHands exposes empty `LLM_API_KEY`/`LLM_BASE_URL` fields for user configuration.
- **R6** — Settings can add/remove/edit backend definitions and models, select defaults, choose all
  four types, and edit backend/model environment values. Onboarding offers the same type union while
  merge-preserving the rest of the seeded catalog.
- **R7** — New Agent lists configured backends and models, defaults to the configured default
  backend/model, and resets the selected model to that backend's default when the backend changes.
  The launch API rejects an unknown backend or model instead of guessing.
- **R28** — A `codex-acp` backend may set `autosync_models: true`. On dashboard startup (after
  seeding), AgentDeck reads the Codex CLI's local model cache
  (`${CODEX_HOME:-~/.codex}/models_cache.json`) and **adds** every user-visible model
  (`visibility:"list"`) not already present to that backend's `models` map, keyed by the Codex model
  slug, with the slug as the provider string and the catalog `display_name` as the label. Sync is
  **add-only**: it never edits or removes an existing model entry, never changes `default_model`, and
  writes nothing when it finds nothing new. A missing, unreadable, or unparseable cache is a
  non-fatal skip that never blocks startup or mutates the catalog. Backends without the flag, and
  every non-`codex-acp` type, are untouched. Claude has no equivalent on-disk catalog (its list is
  compiled into the CLI binary) and is intentionally out of scope.

### Adapter and capability matrix

- **R8** — All four backend types use the common ACP chat runtime and normalized transcript,
  permission, persistence, SSE, stop and resume/switch surfaces. Provider differences are confined
  to the backend adapter and launch composition; they do not create a second chat runtime.
- **R9** — `claude-acp` launches `claude-code-acp`, strips inherited `CLAUDECODE`, supports native
  same-backend resume/model switch, and can register the composed Claude hook settings through
  `--settings`.
- **R10** — `codex-acp` launches `codex-acp`, attempts native same-backend resume/model switch, and
  uses ACP-derived chat status. Its real hook-settings registration remains unverified and no hook
  settings argv is injected.
- **R11** — `opencode-acp` launches `opencode acp`, uses provider-qualified model ids, strips
  inherited `CLAUDECODE`, `OPENCODE_CONFIG`, and `OPENCODE_CONFIG_CONTENT`, and has no lifecycle-hook
  registration. With effective skip-permissions true, AgentDeck injects an ephemeral
  `OPENCODE_CONFIG_CONTENT` permission block; with it false, ordinary ACP permission requests use
  the shared gate.
- **R12** — `openhands-acp` launches `openhands acp`, carries the selected provider model in
  `LLM_MODEL`, strips inherited `CLAUDECODE` and `LLM_MODEL`, and has no lifecycle-hook registration.
  The shared ACP permission gate auto-approves requests when effective skip-permissions is true;
  no unverified CLI-side always-approve flag/mode is injected.
- **R13** — Chat is supported for every backend. Terminal launch, resume and switch are supported
  only for `claude-acp`; all other types return `422 terminal_unavailable`. The New Agent UI hides
  or disables Terminal for those types rather than offering a combination the server rejects.
- **R14** — A same-backend resume supplies the prior native session id for all four adapters; a
  cross-backend switch has no compatible native id and uses AgentDeck's bounded history-primer
  handoff on the same stable `agent_id`.
- **R15** — Every chat launch receives AgentDeck's scoped HTTP messaging MCP entry through the ACP
  `mcpServers` session parameter. Whether each real CLI/version accepts that registration is an
  external compatibility gate, not inferred from fake-ACP success.

### Credential feedback

- **R16** — `PUT /api/backends` persists a structurally valid normalized document even when a
  credential probe is `failed` or `skipped`, then returns one best-effort bounded result per
  backend: `{status:"ok"|"failed"|"skipped", detail?}`. Network/tool absence cannot destroy or
  reject otherwise valid configuration.
- **R17** — Claude probes `claude auth status` (retrying without `--no-color` for older CLIs);
  Codex checks `OPENAI_API_KEY` against `${OPENAI_BASE_URL:-https://api.openai.com}/v1/models`;
  OpenCode requires its executable plus either its standard auth file or a provider API key;
  OpenHands requires its executable plus `LLM_API_KEY` or its standard settings file.
- **R18** — Credential probes use the merged backend/model environment, have a six-second deadline,
  sanitize returned output, and classify missing CLIs/keys, timeouts, network errors and unfamiliar
  responses as `skipped` rather than inventing success.
- **R19** — The onboarding backend step is complete only when the current default backend/default
  model probe returns `ok`; its result is cached for 60 seconds and invalidated by a backend save.

## 3. States & transitions

- **R20** — Saving Settings transitions the submitted catalog through validation → deterministic
  default normalization → atomic persistence → independent credential results. Validation failure
  leaves the previous document intact.
- **R21** — Launch selects a backend id and model id, resolves the provider model string and merged
  environment, strips adapter-forbidden inherited variables, adds adapter-owned values, and starts
  the adapter argv. Stop and crash cleanup remain common runtime behavior.
- **R22** — Switch within one backend/model family attempts native resume. Switching to a different
  backend uses primer handoff. A failed target resume rolls back through the lifecycle rules in
  FS-01 rather than changing the backend catalog.

## 4. Edge cases & errors

- **R23** — Malformed version, multiple defaults, a backend with no models, an unknown default
  model, unknown type, or empty provider model string returns the shared field-error envelope; the
  server does not partially persist the invalid document.
- **R24** — A missing executable fails launch rather than creating a running agent; a credential
  result of `skipped` is not proof that launch will work. Backend-specific recovery copy is a known
  deviation in §6.
- **R25** — Ambient adapter-specific environment that could override AgentDeck composition is
  removed according to R9–R12 before backend/model/hook values are applied. Other host environment
  variables remain inherited subject to the standing env-inheritance decision in FS-00/TS-05.
- **R26** — OpenCode/OpenHands expose no terminal mode or native hook surface merely because their
  chat adapter exists. Capabilities remain explicit per backend and interface.
- **R27** `(planned)` — `OPENCODE_PATH` and `OPENHANDS_PATH` select the executable consistently for
  both credential probing and launch, and a missing/rejected CLI fails with backend-specific
  installation or incompatible-flag guidance instead of a raw transport-closed error.

## 5. Acceptance criteria

- **A1** (R1–R7, R20, R23) — Four types validate, deterministic defaults normalize, invalid
  catalogs fail without persistence, seeds include all four definitions, and Settings/onboarding
  preserve and expose the entire union. *Verified by* the `TestValidateBackendsConfig_*` suite, seed/config
  tests, `BackendsEditor.test.tsx`, and `BackendStep.test.tsx`.
- **A2** (R8–R12, R21) — Fake ACP launch→prompt→stream→stop/resume works for OpenCode/OpenHands;
  argv/env mapping is adapter-specific and skip-permissions behavior is pinned. *Verified by*
  `TestOpenCodeChatE2E`, `TestOpenHandsChatE2E`, `TestNewBackendAdapters`,
  `TestSkipPermissionsEnvOpenCode`, and `TestOpenHandsExtraEnvCarriesModel`.
- **A3** (R13, R26) — Codex, OpenCode and OpenHands terminal launch/resume/switch requests are
  rejected `422`, while the UI never presents their Terminal choice. *Verified by*
  `TestCodexTerminalRejected`, `TestNewBackendTerminalRejected`, and New Agent modal tests.
- **A4** (R14, R22) — Same-backend native ids and cross-backend primer behavior preserve stable
  identity/history and reject/roll back bad transitions. *Verified by* `TestResolveResumeID`,
  `TestSwitchClaudeToOpenCodePrimer`, `TestSwitchRuntimeBackendSwapUsesPrimer`, and
  `TestSwitchRuntimeRollbackOnResumeFailure`.
- **A5** (R16–R19) — Saves persist independently of best-effort probe status; backend-specific
  probes, merged env and sanitized timeout/missing-auth outcomes are covered. *Verified by*
  `TestMergeEnv`, `TestOpenCodeProber`, `TestOpenHandsProber`,
  `TestClaudeProberRetriesWithoutNoColor`, and config-handler/onboarding credential tests.
- **A6** `(GATED — real CLI credentials; Phase 7.4)` — With authenticated `opencode` and
  `openhands` CLIs, verify ACP handshake and one streamed turn, permission round-trip and
  skip-permissions behavior, stop, native resume or documented primer fallback, provider/model/env
  mapping, and HTTP `mcpServers` registration. Until recorded, these backends pass fake-ACP tests but
  real-CLI compatibility is not claimed.
- **A7** `(GATED — real CLI credentials)` — Re-run live Codex chat launch/turn/stop/resume and
  Claude/Codex/OpenCode/OpenHands HTTP messaging-MCP registration against pinned versions before a
  release claims those external compatibility paths.
- **A8** (R28) — A `codex-acp` backend with `autosync_models` gains newly available user-visible
  Codex models on startup without duplicating or overwriting existing entries, changing the default,
  or including hidden models; a disabled flag, a non-codex backend, and a missing cache leave the
  catalog unchanged. *Verified by* `TestSyncCodexModelsAddsVisibleModels`,
  `TestSyncCodexModelsPreservesExistingAndDefault`, `TestSyncCodexModelsRespectsFlagAndType`, and
  `TestReadCodexModelCatalog`.

## 6. Deviations & open decisions

- **OpenCode/OpenHands live acceptance is gated (A6).** Their binary/ACP commands, native
  `session/load`, exact OpenCode permission keys, OpenHands CLI-side approval mode, and HTTP MCP
  acceptance are based on adapter contracts plus fake ACP tests, not a recorded authenticated run.
- **OpenHands skip-permissions is host-side.** The shared runtime auto-approves ACP permission
  requests, but an always-approve session mode/CLI flag is intentionally not sent until a real CLI
  acceptance establishes its contract.
- **Executable overrides validate but do not launch.** Credential probes honor `OPENCODE_PATH` and
  `OPENHANDS_PATH`; the adapters currently execute bare `opencode`/`openhands`. A CLI outside the
  server PATH can therefore probe successfully and still fail launch.
- **Credential probes are best-effort and storage/version-sensitive.** OpenCode/OpenHands infer
  login from fixed default files or env, while Claude parses CLI text. Alternate platform paths,
  stale files, or changed/localized CLI output can yield a misleading result; launch remains the
  authoritative check.
- **Missing/rejected CLI startup diagnostics are weak.** A missing executable or rejected optional
  flags can currently collapse into a raw/generic transport error; backend-specific installation
  and compatibility guidance is tracked usability work.
- **Model/API compatibility remains partial.** The ACP adapter may ignore AgentDeck's requested
  model in favor of its own identifiers, and older endpoints do not yet share one error envelope.

## 7. Traceability

- **Catalog/validation/seeds:** `internal/config/types.go`, `internal/config/validate.go`,
  `internal/config/seed.go`, `internal/server/config_handlers.go`.
- **Codex model autosync (R28):** `internal/config/codexmodels.go` (`ReadCodexModelCatalog`,
  `syncCodexModels`, `Store.AutoSyncBackends`); invoked from `resolveConfig` in
  `internal/cli/dashboard.go`.
- **Adapters/credentials:** `internal/backend/adapter.go`, `internal/backend/credcheck/`;
  `internal/runtime/chat.go` (adapter consumption and shared ACP permission gate).
- **Capability/composition:** `internal/server/terminal.go`, `launch.go`, `resume.go`, `switch.go`.
- **UI:** `ui/src/schemas/backends.ts`, `ui/src/lib/backendTypes.ts`,
  `ui/src/features/settings/BackendsEditor.tsx`,
  `ui/src/features/onboarding/steps/BackendStep.tsx`,
  `ui/src/features/launch/NewAgentModal.tsx`.
- **Regression anchors:** `internal/backend/adapter_test.go`,
  `internal/backend/credcheck/credcheck_test.go`, `internal/runtime/chat_test.go`,
  `internal/server/switch_test.go`, backend config handler tests, and the UI tests above.
