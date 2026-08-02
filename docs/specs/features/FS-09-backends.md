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
  every non-`codex-acp` type, are untouched. This requirement remains Codex-specific;
  configured-model discovery for Claude is R45.
- **R35** — A model entry may declare optional **effort capability**: `efforts`, a
  non-empty array of distinct non-empty provider effort-level strings, and `default_effort`, which
  must be one of them. Both are optional. A model that declares no `efforts` has no effort
  capability and AgentDeck offers no effort choice for it anywhere. AgentDeck defines no effort
  vocabulary of its own: a level is whatever string the provider accepts, so Codex's `ultra` and
  Claude's `max` coexist without translation or cross-provider normalization. `PUT /api/backends`
  rejects a blank or duplicated level, a `default_effort` outside `efforts`, and a `default_effort`
  on a model declaring no `efforts`, using the shared field-error envelope and without partially
  persisting the document.
- **R36** — A model's provider `model` string must not carry a bracketed effort suffix
  (`slug[level]`). `PUT /api/backends` rejects one with a field-level error naming `efforts` as the
  supported way to express a level, so a model's effort has exactly one source. This rejects a
  previously undocumented Codex-only encoding; a person holding such a catalog moves the level into
  `efforts` on their next save.
- **R37** — `GET /api/backends` reports each model's `efforts` and `default_effort`. New
  Agent shows an effort control only for a model that declares `efforts`, offers exactly those
  levels, preselects `default_effort` when present, and resets the selection when the backend or
  model changes — the same rule R7 applies to the model itself. Settings can add, remove, and
  reorder a model's levels and choose its default effort.
- **R38** — A `codex-acp` backend with `autosync_models` (R28) also fills a synced
  model's `efforts` and `default_effort` from the local cache's `supported_reasoning_levels[].effort`
  and `default_reasoning_level`. This stays add-only under R28's rules: it never edits an existing
  model entry, including one that already declares effort fields, and a cache without reasoning
  levels simply contributes none. Claude's configured-model sync in R45 does not discover
  effort capability, so Claude effort levels remain hand-declared.
- **R45** — A `claude-acp` backend may also set `autosync_models: true`. On dashboard
  startup after seeding, AgentDeck reads only the user-level `~/.claude/settings.json` and collects
  model selectors from `model`, every string entry in `availableModels`, and every string entry in
  the `fallbackModel` array (also tolerating the older singular string shape). Each distinct,
  non-empty selector that passes the existing model-string validation and is not already represented
  by either a model-map key or an existing entry's provider `model` string is added to that backend's
  global `models` map, keyed by and carrying the exact selector. The four family aliases use the
  labels **Claude Fable**, **Claude Opus**, **Claude Sonnet**, and **Claude Haiku**; any other selector
  uses itself as its label because the settings file carries no display metadata. Sync is add-only:
  it never edits or removes an existing entry, changes `default_model`, or writes when it finds
  nothing new. A missing, unreadable, malformed, or shape-invalid settings file is a non-fatal skip
  that never blocks startup or partially mutates the catalog. A disabled flag and every backend type
  other than `claude-acp` are untouched by this Claude source. Settings offers the same flag for
  Claude and explains that it imports configured user-level models at the next dashboard start.
- **R46** — A fresh home seeds the Claude backend with the portable provider aliases
  `fable`, `opus`, `sonnet`, and `haiku`, keyed by those same values and labelled **Claude Fable**,
  **Claude Opus**, **Claude Sonnet**, and **Claude Haiku**. `sonnet` remains the Claude default.
  These aliases intentionally resolve through the installed Claude/provider and may point to
  different concrete versions over time; the catalog does not claim a fixed version or account
  entitlement. This planned behavior supersedes only the Claude portion of R33 when shipped. As in
  R33, seeding never rewrites a pre-existing `backends.json`, adds missing aliases to an existing
  catalog, or changes an existing default.

- **R47** `(planned)` — Connecting a Claude/Codex native configuration through the normal
  one-click federation action enables `autosync_models` only on that target backend and immediately
  runs its existing provider-specific, add-only import. Codex imports only locally cached
  user-visible (`visibility:"list"`) entries and their declared effort metadata under R28/R38; Claude
  imports only valid explicit selectors from the user-level settings in R45, without inferring
  effort. Neither result claims provider support, account entitlement, availability, or a successful
  future launch. A missing or malformed local provider catalog remains a non-blocking zero-model
  result exactly as R28/R45 define: the source may still connect and its valid starter model remains.
  The merge reads the locked authoritative catalog snapshot and changes only the connected backend;
  it never invokes whole-catalog startup sync, touches another opted-in backend, overwrites an
  existing entry, or changes a default. The ordinary Settings control can later turn continuing sync
  off. TS-07.R17 owns the accepted best-effort residue when catalog persistence precedes binding.

### Adapter and capability matrix

- **R8** — All four backend types use the common ACP chat runtime and normalized transcript,
  permission, persistence, SSE, stop and resume/switch surfaces. Provider differences are confined
  to the backend adapter and launch composition; they do not create a second chat runtime.
- **R9 — retired 2026-07-15:** The Zed-era `claude-code-acp` executable and its unverified chat
  `--settings` passthrough were replaced by the official Agent Client Protocol adapter in R29.
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
- **R43** — AgentDeck launches every `codex-acp` process (launch, resume, and switch)
  with `CODEX_HOME` pointed at an AgentDeck-owned directory instead of the user's personal Codex
  home, so Codex writes its rollouts and native session index there and AgentDeck-created Codex
  conversations never appear in the user's native `codex` resume picker or Codex app history.
  AgentDeck's own transcript, archive, search, and every session created after this behavior ships
  resume from that same dedicated home. Isolation is always on for `codex-acp`; there is no per-agent
  or per-backend toggle. Sessions already written into the personal home are not moved and may no
  longer native-resume through AgentDeck after the change; the user may archive those natively.
  Claude, OpenCode, and OpenHands are unaffected.
- **R44** — Before every `codex-acp` process starts, AgentDeck refreshes its dedicated
  home from the user's effective Codex home (`${CODEX_HOME:-~/.codex}`): configuration,
  authentication, skills, agents, rules, plugins, and MCP setup are copied into the private profile,
  while Codex session/history data is never copied. The refresh is one-way: the personal home remains
  authoritative; additions, edits, and removals appear in the private profile at the next process
  start, and AgentDeck never writes through to the personal home. A rejected refresh leaves the prior
  private setup intact; a switch detects that rejection before stopping its working Codex runtime.
  AgentDeck's own
  configuration-federation discovery (FS-08) and Codex model autosync (R28) keep reading the user's
  real Codex home, so a bound source or an `autosync_models` backend behaves exactly as before.

- **R39** — Effort capability is offered only for `claude-acp` (chat and terminal) and
  `codex-acp` (chat). `opencode-acp` and `openhands-acp` expose no effort mechanism, so a model
  under one of those backends that declares `efforts` is rejected by `PUT /api/backends` with a
  field-level error rather than accepted and silently ignored at launch — capabilities remain
  explicit per backend as R26 requires.
- **R40** — For `claude-acp` chat, the chosen effort is applied immediately after the
  native session is created rather than as part of creating it, because the adapter accepts effort
  only as a post-creation session setting and AgentDeck will not write a person's native Claude
  settings files to seed it (FS-08.R7). If that application fails, AgentDeck stops the just-started
  agent and fails the launch with a bounded error; it never leaves a running agent at an effort the
  person did not choose. Codex chat and Claude terminal carry effort as part of starting the
  process, so they have no such window.

### Credential feedback

- **R16** — `PUT /api/backends` persists a structurally valid normalized document even when a
  credential probe is `failed` or `skipped`, then returns one best-effort bounded result per
  backend: `{status:"ok"|"failed"|"skipped", detail?}`. Network/tool absence cannot destroy or
  reject otherwise valid configuration.
- **R17 — retired 2026-07-15:** The direct system `claude auth status` probe was replaced by the
  selected adapter's bundled-Claude probe; current provider readiness behavior is R34.
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
- **R41** — Launch resolves effort in one precedence order: an explicitly requested
  effort; else a bound configuration source's stored effort override (FS-08.R10); else the selected
  model's `default_effort`; else nothing is sent and the CLI selects its own level. This mirrors the
  model-inheritance rule in FS-08.R17 rather than inventing a level. The resolved effort is frozen
  into the session snapshot beside the model, so resume and switch restore it and the archive
  records what actually ran.
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
  removed according to R10–R12 and R29 before backend/model/hook values are applied. Other host environment
  variables remain inherited subject to the standing env-inheritance decision in FS-00/TS-05.
- **R26** — OpenCode/OpenHands expose no terminal mode or native hook surface merely because their
  chat adapter exists. Capabilities remain explicit per backend and interface.
- **R27** `(planned)` — `OPENCODE_PATH` and `OPENHANDS_PATH` select the executable consistently for
  both credential probing and launch, and a missing/rejected CLI fails with backend-specific
  installation or incompatible-flag guidance instead of a raw transport-closed error.
- **R42** — A requested effort the selected model does not declare fails launch, switch
  runtime, and pipeline run start with a field-level error naming the effort field and the levels
  that model does declare, before any process starts — exactly as an unknown model does under R7.
  AgentDeck never substitutes a different level and never silently drops the request. A model whose
  declared levels are later edited does not retroactively change a frozen running or archived
  session (R4).
- **R29** — `claude-acp` launches `claude-agent-acp` from the pinned official
  `@agentclientprotocol/claude-agent-acp` package, strips inherited `CLAUDECODE`, supplies the
  composed system prompt, model, additional directories and MCP registrations through the
  adapter's ACP session metadata, and preserves native same-backend resume/model switch. Chat
  status remains ACP-derived; Claude terminal launches continue to invoke the interactive Claude
  executable directly with their generated `--settings` hook file.
- **R30 — retired 2026-07-22:** Credential readiness limited Codex to `OPENAI_API_KEY`. Replaced
  by the native-sign-in-or-API-key behavior in R34.
- **R31** — A hand-edited `backends.json` that is syntactically valid but structurally incomplete
  (including a missing `backends` map or a backend with no models) is treated as unreadable on every
  read path. `GET /api/backends` returns the in-memory default catalog just as it does for a missing
  or malformed file; the logged diagnostic names `backends.json`. The file is not overwritten until
  the person explicitly saves a valid catalog.
- **R32** — `codex-acp` receives AgentDeck's frozen composed role/project prompt through its
  documented `CODEX_CONFIG` session-config overlay as `developer_instructions`, on both new and
  resumed chats. AgentDeck preserves unrelated keys from a valid existing `CODEX_CONFIG` object and
  places a pre-existing string `developer_instructions` before the composed prompt. Malformed,
  non-object config, or a non-string existing `developer_instructions` fails that launch with a
  bounded configuration error; AgentDeck never silently drops the selected role.
- **R33** — Fresh homes seed Claude's default model as `sonnet` (labelled as the current
  Claude Sonnet) and Codex's as `gpt-5.6-sol` (labelled `GPT-5.6-Sol`). Seed updates never rewrite an
  existing `backends.json`, change an existing default, or replace a person’s model entry.
- **R34** — Claude and Codex readiness recognize provider-native sign-in as well as the
  applicable configured API-key path. AgentDeck does not start or proxy native login; it only probes
  the resulting readiness and returns a bounded ready/unready/unavailable/failed outcome. For Codex,
  a valid native login and a valid `OPENAI_API_KEY` are independent acceptable readiness paths.

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
- **A5** (R16, R18–R19, R34) — Saves persist independently of best-effort probe status; backend-specific
  probes, merged env and sanitized timeout/missing-auth outcomes are covered. *Verified by*
  `TestMergeEnv`, `TestOpenCodeProber`, `TestOpenHandsProber`,
  `TestClaudeProberRetriesWithoutNoColor`, and config-handler/onboarding credential tests.
- **A6** `(GATED — real CLI credentials; Phase 7.4)` — With authenticated `opencode` and
  `openhands` CLIs, verify ACP handshake and one streamed turn, permission round-trip and
  skip-permissions behavior, stop, native resume or documented primer fallback, provider/model/env
  mapping, and HTTP `mcpServers` registration. Until recorded, these backends pass fake-ACP tests but
  real-CLI compatibility is not claimed.
- **A7** `(GATED — real CLI credentials)` — Re-run live Codex chat launch/turn/stop/resume and the
  official Claude adapter plus Claude/Codex/OpenCode/OpenHands HTTP messaging-MCP registration
  against pinned versions before a release claims those external compatibility paths.
- **A8** (R28) — A `codex-acp` backend with `autosync_models` gains newly available user-visible
  Codex models on startup without duplicating or overwriting existing entries, changing the default,
  or including hidden models; a disabled flag, a non-codex backend, and a missing cache leave the
  catalog unchanged. *Verified by* `TestSyncCodexModelsAddsVisibleModels`,
  `TestSyncCodexModelsPreservesExistingAndDefault`, `TestSyncCodexModelsRespectsFlagAndType`, and
  `TestReadCodexModelCatalog`.
- **A9** (R29, R34) — The Claude adapter resolves to `claude-agent-acp`; its ACP v1
  initialize and session metadata shapes remain covered by the shared fake-adapter tests; and the
  credential probe delegates through `--cli`, including the no-color compatibility retry. The
  pinned package's real initialize/session/turn behavior remains part of the credentialed A7 gate.
  *Verified by* adapter, runtime parameter, and Claude credential-probe tests plus the gated real
  adapter acceptance suite.
- **A10** (R31) — Missing backend collections and null model maps fall back to a usable default
  catalog at the API boundary, while the editor also safely handles null collections:
  `TestReadBackendsRejectsIncompleteDocument`, `TestGetBackendsFallsBackForIncompleteDocument`, and
  `BackendsEditor.test.tsx`.
- **A11** (R32) — Codex new/resume process environments carry the frozen composed prompt in a
  merged `CODEX_CONFIG` `developer_instructions` value, without an unsupported ACP `systemPrompt`
  parameter; malformed overlays fail before spawning. *Verified by* runtime parameter and
  process-environment regression tests. A real authenticated Codex turn/resume remains part of A7.
- **A12** — A fresh backend catalog has the Claude `sonnet` and Codex `gpt-5.6-sol`
  defaults, while a pre-existing catalog remains byte-for-byte unchanged by seeding. *Verified by*
  seed/config tests.
- **A13** — Fake provider-native Claude/Codex readiness produces bounded outcomes
  without AgentDeck starting a login process or receiving credential bytes; Codex API-key readiness
  remains supported. *Verified by* credential-check tests and FS-04.A14's UI test.

- **A14** (R35–R38) — A catalog declaring per-model effort levels validates and round
  trips; a blank/duplicate level, an out-of-range `default_effort`, a `default_effort` without
  `efforts`, and a bracketed provider model string each fail with a named field and nothing is
  persisted; `GET /api/backends` reports the levels; New Agent offers exactly the selected model's
  levels, preselects its default, hides the control for a model declaring none, and resets on model
  change; and `autosync_models` fills levels for a newly synced model while leaving an existing
  entry's hand-declared levels and the catalog default untouched. *Verify by* backend validation and
  config-handler tests, Codex model-cache sync tests, `BackendsEditor.test.tsx`, and
  `NewAgentModal.test.tsx`.
- **A15** (R39–R42) — A Codex chat launch at a declared level reaches the adapter
  carrying that level; a Claude chat launch applies it after session creation, and an injected
  failure of that application leaves no running agent and returns a bounded error; a Claude terminal
  launch carries the level in its argv; an `opencode-acp`/`openhands-acp` model declaring `efforts`
  is rejected at save; an undeclared level is rejected at launch, switch runtime, and pipeline run
  start with no process started; and precedence resolves explicit over source override over
  `default_effort` over omitted. *Verify by* runtime parameter/process-environment tests, a fake-ACP
  post-session failure regression, launch/switch/pipeline validation tests, and the federation
  precedence tests named in FS-08.A8.
- **A16** `(planned)` `(GATED — real CLI credentials)` — Against pinned authenticated Codex and
  Claude CLIs, confirm that a chosen level is actually honored by the running agent for Codex chat,
  Claude chat, and Claude terminal, and that an undeclared level surfaces as AgentDeck's rejection
  rather than a provider-side failure. Until recorded, effort mapping is fixture-tested against the
  pinned adapters but live provider honoring is not claimed.
- **A17** (R43, R44) — A `codex-acp` launch, resume, and switch set the child
  `CODEX_HOME` to the AgentDeck-owned directory, overriding ambient/backend/model values, so the
  rollout and native session index are written there and nothing is added to the user's personal
  Codex `session_index.jsonl`. Before each child starts, its private profile refreshes copied
  authentication, configuration, skills, agents, rules, plugins, and MCP setup from the personal
  home; source additions, edits, and removals arrive on the next process start, no source path is
  symlinked or written, and session/history data is excluded. A new isolated session resumes by
  loading its native id from that same private store; federation resolution and autosync still read
  the user's real Codex home. *Verify by* launch/resume/switch env-composition, profile-refresh,
  and home-provisioning tests. Real `codex-acp` honoring of `CODEX_HOME`, profile setup visibility,
  native resume, and the native CLI/app history boundary are external compatibility gates recorded
  under A7 (cf. TS-04.R21).
- **A18** (R45) — An opted-in `claude-acp` backend gains valid, previously
  unrepresented selectors from the user-level `model`, `availableModels`, and array or legacy-string
  `fallbackModel` settings at dashboard startup; alias labels are friendly and other labels preserve
  the selector. Existing entries are unchanged even when their map key differs from their provider
  string, the default is unchanged, removed source values remain in the catalog, and source values
  never gain inferred effort levels. A disabled flag, non-Claude backend, missing/malformed/wrong-shape
  file, invalid selector, duplicate selector, project/local/managed setting, private Claude cache,
  and environment-only model setting add nothing. *Verify by* focused config sync/persistence tests
  and `BackendsEditor` tests for the Claude-specific opt-in copy and restart timing.
- **A19** (R46) — A fresh backend catalog contains exactly the four Claude family
  aliases with generic family labels and `sonnet` as its default, while a pre-existing catalog
  remains byte-for-byte unchanged by seeding. The launch picker exposes all four without claiming
  their resolved versions or entitlements. *Verify by* seed/config tests and the onboarding/New
  Agent catalog fixtures.
- **A20** `(planned)` (R47) — A normal successful Claude/Codex source connection enables model
  autosync and immediately imports only into its target backend: Codex adds local-cache
  `visibility:"list"` models plus declared effort metadata, while Claude adds only valid explicit
  user-level configured selectors and no inferred effort. Existing entries/defaults and every other
  backend remain unchanged; absent/malformed local input reports zero additions without failing an
  otherwise valid connection; and UI/result copy makes no support or entitlement claim. A permitted
  unbound best-effort residue remains add-only and converges without duplication on retry. *Verify by*
  focused source-connect/model-sync server tests and Settings tests for both providers, target-only
  behavior, add-only/default preservation, zero-model success, residue, and retry.

## 6. Deviations & open decisions

- **Claude configured-model sync is deliberately not full discovery (R45).** Claude exposes
  no stable local full-catalog contract equivalent to Codex's cache. The planned sync therefore
  reads only explicit user-level settings, does not scan the executable or private account caches,
  does not inspect environment values, and does not issue a network, credential, adapter, or session
  probe. Imported selectors can still be unavailable for the installed version, provider, or
  account; launch remains the authoritative compatibility check. Project, local, and managed values
  stay project/policy-scoped instead of leaking into the global backend catalog.
- **Effort levels are declared, not discovered, for Claude.** Codex publishes its per-model levels
  in the cache `autosync_models` already reads, so R38 can fill them. Claude reports its levels only
  through the running adapter's session config options, so a Claude model's levels are hand-declared
  and can drift from what the installed CLI accepts; R42 then rejects a level AgentDeck believes is
  valid only after the person edits the catalog. Discovering Claude levels at launch would require a
  probe with no offline equivalent and is deliberately out of scope.
- **Claude chat effort has a brief post-creation window (R40).** Between session creation and the
  effort call the native session exists at the CLI's own level. AgentDeck sends no prompt in that
  window and tears the agent down if the call fails, so no turn ever runs at an unchosen level, but
  the launch cost of a failed effort application is one spawned-then-stopped process.
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
- **Official Claude adapter acceptance remains credential-gated.** Automated tests pin the ACP v1
  boundary and session metadata, but an authenticated streamed turn/resume/MCP run against the
  exact packaged version is still required before release compatibility is claimed.
- **Codex role delivery remains credential-gated.** Automated tests pin the documented
  `CODEX_CONFIG` overlay used by the installed adapter, but A7 must still confirm role adherence on
  a real new turn and native resume.
- **Model/API compatibility remains partial.** The ACP adapter may ignore AgentDeck's requested
  model in favor of its own identifiers, and older endpoints do not yet share one error envelope.

## 7. Traceability

- **Catalog/validation/seeds:** `internal/config/types.go`, `internal/config/validate.go`,
  `internal/config/seed.go`, `internal/server/config_handlers.go`.
- **Model autosync (R28/R45):** `internal/config/codexmodels.go` (`ReadCodexModelCatalog`,
  `syncCodexModels`), `internal/config/claudemodels.go` (`ReadClaudeConfiguredModels`,
  `syncClaudeModels`, `ClaudeSettingsPath`), and `internal/config/modelautosync.go` (the shared
  add-only `syncModels` merge and combined `Store.AutoSyncBackends`); invoked from `resolveConfig`
  in `internal/cli/dashboard.go`. Fresh Claude family aliases seed in `internal/config/seed.go` (R46).
- **Adapters/credentials:** `internal/backend/adapter.go`, `internal/backend/credcheck/`;
  the official Claude executable and delegated auth probe are pinned by
  `TestClaudeAdapterUsesOfficialBinary` and `TestClaudeProberRetriesWithoutNoColor`;
  provider-native readiness lives in `credcheck/native.go` over the shared
  `internal/backend/providerauth` command table (R34), covered by
  `TestCodexProberAcceptsNativeLoginWithoutAPIKey`,
  `TestCodexProberFallsBackToAPIKeyWhenNotSignedIn`, and
  `TestClassifyNativeOutputPrefersTheNegative`; seeded defaults by
  `TestSeededBackendDefaultsAreCurrentAndNeverRewritten` (R33);
  `internal/runtime/chat.go` (adapter consumption, Codex config-overlay composition, and shared ACP
  permission gate).
- **Capability/composition:** `internal/server/terminal.go`, `launch.go`, `resume.go`, `switch.go`.
- **Codex isolated profile (R43/R44):** final `CODEX_HOME` child override composed in
  `internal/server/{launch,resume,switch}.go`, one-way profile refresh under `internal/config`, spawn
  application in `internal/runtime/chat.go`; AgentDeck's own-home reads stay in
  `internal/server/config_sources.go` and `internal/config/codexmodels.go` (see TS-04.R20/R21).
- **UI:** `ui/src/schemas/backends.ts`, `ui/src/lib/backendTypes.ts`,
  `ui/src/features/settings/BackendsEditor.tsx`,
  `ui/src/features/onboarding/steps/BackendStep.tsx`,
  `ui/src/features/launch/NewAgentModal.tsx`.
- **Regression anchors:** `internal/backend/adapter_test.go`,
  `internal/backend/credcheck/credcheck_test.go`, `internal/runtime/chat_test.go`,
  `internal/server/switch_test.go`, backend config handler tests, and the UI tests above.
