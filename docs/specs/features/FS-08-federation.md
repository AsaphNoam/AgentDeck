# FS-08 — Configuration federation

**Status:** Partial
**Code:** `internal/config/sources.go`, `internal/configsource/`, `internal/server/config_sources.go`, `ui/src/features/settings/ConfigSourcePanel.tsx`, `ui/src/features/onboarding/steps/SourceStep.tsx` · **Journeys:** J2, J9
**Absorbed:** exact Phase 7.5–7.8 mapping in the [phase archive manifest](../../archive/phases/README.md)

## 1. Purpose

Configuration federation lets an AgentDeck Claude or Codex backend use the user's existing native
CLI setup without creating a second writable copy. Native files remain authoritative; AgentDeck
stores an explicit binding, optional model/effort overrides, approved roots, and redacted
provenance. The feature covers discovery, preview and consent, linked/mirrored operation,
freshness, launch/resume semantics, and the Settings/onboarding experience.

Federation is optional. An unbound backend continues to use `backends.json` exactly as described by
FS-04 and FS-09. OpenCode and OpenHands do not participate in federation.

## 2. Behavior

### Discovery, preview, and consent

- **R1** — Only a backend whose type is `claude-acp` or `codex-acp` can have a configuration-source
  binding. Their provider ids are respectively `claude-code` and `codex`; a provider/backend
  mismatch is rejected rather than coerced.
- **R2** — `GET /api/config-sources?project=<id>` returns redacted discovery candidates and active
  bindings for the selected project. Standard discovery examines `~/.claude/settings.json` for
  Claude and `${CODEX_HOME:-~/.codex}/config.toml` for Codex. Discovery alone grants no read
  authority and persists nothing.
- **R3** — `POST /api/config-sources/preview` accepts a provider, project, mode, optional claims, and either
  `root:"auto"` or an explicit root. It resolves read-only and returns an effective view, a report,
  and an opaque preview token expiring after ten minutes. The supported claims are
  `launch_defaults`, `model_catalog`, and `setup`.
- **R4** — A preview report contains paths/scopes/kinds read, skipped paths with reasons, unknown
  key names classified as native pass-through, fingerprints, approved roots, warnings, and a
  source digest. The effective view may contain model, fallback model, effort, verbosity, provider,
  configured models, setup-asset metadata, environment-key names, native MCP server ids, and
  per-field provenance. It never contains source-file contents, credential values, headers, auth
  stores, or literal environment values.
- **R5** — `PUT /api/config-sources/{backend_id}` accepts only a one-use preview token plus optional
  model/effort overrides. The server reconstructs the provider, mode, paths, profile, claims, and
  approved roots from the token rather than trusting new client paths. It rejects an unknown,
  spent, or expired token and rejects a source whose digest changed after preview.
- **R6** — Every source read is constrained to a canonical root the user approved during preview,
  plus the currently selected project's canonical root. A symlink that resolves outside those
  roots produces `approval_required`; AgentDeck never silently expands the approved boundary.
- **R7** — Preview, resolve, refresh, bind, and unbind never write a native source file. The stored
  `config-sources.json` manifest and any AgentDeck cache live beneath `AGENTDECK_HOME` with
  owner-only permissions.

### Binding modes and effective view

- **R8** — Two binding modes ship. In `linked` mode the native tree is read directly and remains
  authoritative. In `mirrored` mode the same authority rule applies, but AgentDeck also maintains
  an owner-only redacted cache as disposable compatibility/display state. Cache failure never
  changes source authority or makes a stale launch valid.
- **R9** `(planned)` — Detached import is the only mode in which AgentDeck would materialize and
  own a copy. It is not shipped: `DELETE ...?detach=true` returns `501 not_implemented`, and the UI
  labels detached import/copy unavailable. Plain unbind (`detach=false`) is supported.
- **R10** — A binding may explicitly override `model` and `effort`. Empty/reset values mean inherit.
  Settings identifies an AgentDeck override versus an inherited native value and provides
  **Apply override** and **Reset to inherit** actions.
- **R11** — Claude resolution recognizes user, project, local, and managed settings; inventories
  `CLAUDE.md` imports plus user/project rules, skills, agents, hooks, plugins, and MCP declarations.
  Managed settings are applied after ordinary native layers and AgentDeck overrides, so managed
  policy remains authoritative.
- **R12** — Codex resolution recognizes user configuration, an explicitly selected profile, and a
  trusted project's `.codex/config.toml`; it inventories `AGENTS.md`, user/project skills and
  agents, and declared rules, hooks, plugins, MCP servers, and configured model catalog entries.
  Untrusted project configuration is reported as skipped rather than applied.
- **R13** — Setup assets are inventory/reference metadata only (`path`, scope, kind, fingerprint,
  status, detachability). AgentDeck does not translate Claude setup into Codex setup, copy it, or
  claim that a configured model is enabled for the user's account.
- **R14** — Unknown native keys remain owned by the CLI and are reported as `native_passthrough`;
  unsupported content is never rewritten or presented as successfully imported.

### Freshness and launch semantics

- **R15** — A successful resolution installs an immutable per-backend/per-project generation.
  File-system events are debounced, a 30-second sweep recovers missed events, and
  `config_source_update` SSE announces generation, health, changed high-level fields, and stale
  state so the UI can invalidate its project-scoped source query.
- **R16** — Every new launch synchronously resolves the source again. A prior last-known-good view
  may remain visible, but `source_invalid`, missing/unapproved content, or another failed fresh
  resolution blocks the dependent launch; stale cache is never used as launch input.
- **R17** — For a bound launch, an explicit launch model wins over a stored source override. If no
  explicit model exists, a source override is used. If neither exists, AgentDeck omits the model
  from the ACP request so the native CLI selects its own default rather than receiving a guessed
  AgentDeck model.
- **R18** — A bound launch freezes a redacted versioned object in the session snapshot: backend,
  provider, profile, mode, requested/resolved high-level values, source digest/fingerprints, and
  whether the model was natively inherited. No secret values are frozen.
- **R19** — Resume and runtime switch use that frozen federation object by default. A resume request
  with `config_refresh:true` explicitly resolves the current source and freezes the new result.
  Source changes never hot-mutate an already running agent.
- **R20** — AgentDeck passes the real project working directory and native user home through to the
  CLI, allowing provider-native project instructions and setup to remain discoverable without
  AgentDeck copying them.
- **R21** — A native declaration named `agentdeck-messaging` conflicts with AgentDeck's reserved
  per-session messaging MCP id. Launch fails `409 source_conflict`; neither declaration is silently
  overwritten.

### User experience

- **R22** — First-run onboarding contains an optional, always-skippable source step after project
  creation. It reuses the Settings source panel for the backend selected earlier. OpenCode and
  OpenHands show an explanation and continue without federation controls.
- **R23** — Settings exposes federation only for Claude/Codex backends. The user selects a project,
  discovers and previews native setup, then binds Linked (recommended) or Mirrored
  (compatibility). A bound source shows mode, root, health/staleness, model/effort provenance,
  configured models with a “not an entitlement check” note, setup inventory, override controls,
  refresh/load-effective-view, and unlink.
- **R24** — The UI renders paths, field names, scope, status, and configured environment-key names,
  but never renders source contents or secret values. A stale/invalid/approval-required source
  shows a repair message offering refresh after correction or unlink.

## 3. States & transitions

- **R25** — An unbound backend uses `backends.json`. Preview leaves it unbound. A successful token
  consumption moves it to bound/healthy or bound/stale-invalid if the post-persist fresh resolve
  fails; the persisted binding remains visible so the user can repair or unlink it.
- **R26** — A healthy generation has `health:"ok"`, `stale:false`. A failed later resolution keeps
  the last-known-good display generation but changes health to `source_invalid` or
  `approval_required` and sets `stale:true`. A successful refresh creates a new healthy generation.
- **R27** — Unlink deletes the binding, drops its in-memory generations, returns `204`, and restores
  ordinary `backends.json` behavior for future launches. It does not modify native files or running
  sessions.

## 4. Edge cases & errors

- **R28** — Unknown provider/mode/project/backend, provider/backend mismatch, invalid claims,
  non-canonical paths, invalid profiles, or malformed JSON/TOML fail with a structured error and do
  not expose raw source values.
- **R29** — Source errors map to stable codes: missing source/binding → `404 source_not_found`;
  digest changed → `409 source_changed`; reserved MCP id → `409 source_conflict`; expired/unapproved
  consent → `409 approval_required`; malformed/invalid source → `422 source_invalid`.
- **R30** — Empty lists in source API responses are arrays rather than `null`; source reports are
  deterministic enough for digest/fingerprint comparison and UI rendering.

## 5. Acceptance criteria

- **A1** (R1–R7) — Preview and token-confirmed binding are read-only, provider/backend checked,
  owner-only on disk, and reject expired/spent/TOCTOU tokens. *Verified by*
  `TestConfigSourcePreviewBindRefreshDelete`, `TestConfigSourceTOCTOURejectedAtBind`,
  `TestPreviewTokenExpiry`, `TestBindingDoesNotWriteSource`, and `TestConfigSourcesRoundTripAndOwnerOnlyAtomicWrite`.
- **A2** (R4, R6, R11–R14, R24) — Claude and Codex fixture trees resolve precedence, provenance,
  inventory, project trust and symlink boundaries without returning secret sentinels. *Verified by*
  `TestClaudeResolverPrecedence`, `TestClaudeResolverReadOnlyAndRedacted`,
  `TestCodexResolverPrecedence`, `TestCodexResolverSkipsUntrustedProject`,
  `TestResolverNeverReturnsSecrets`, and the two unapproved-symlink tests.
- **A3** (R8, R15–R16, R25–R27) — Mirrored cache is redacted, watcher/sweep recover external
  changes, invalid sources retain display state but block launch, refresh heals, and unlink returns
  to unbound behavior. *Verified by* `TestMirroredCacheIsRedactedAndOwnerOnly`,
  `TestWatchReresolvesOnFilesystemEvent`, `TestSweepRecoversMissedEvent`,
  `TestResolveFreshInvalidSourceBlocksButKeepsLastKnownGood`, and
  `TestComposeLaunchBlocksInvalidSource`.
- **A4** (R10, R17–R21) — Launch precedence, native model inheritance, frozen redacted provenance,
  frozen resume, explicit model override, and the reserved MCP collision are pinned. *Verified by*
  `TestComposeLaunchFreezesFederationConfig`, `TestComposeLaunchExplicitModelOverridesSource`,
  `TestComposeResumeSpecCarriesFrozenLaunchConfig`, and
  `TestComposeLaunchRejectsReservedMCPCollision`.
- **A5** (R22–R24) — Onboarding is optional/provider-aware and Settings supports both binding modes,
  provenance/inventory, overrides/reset, refresh, repair and unlink without rendering contents.
  *Verified by* `SourceStep.test.tsx` and `ConfigSourcePanel.test.tsx`.
- **A6** (R9) — Detached materialization is visibly unavailable rather than silently degrading to
  unlink. *Verified by* `TestConfigSourceDetachNotImplemented` and the disabled detached controls in
  `ConfigSourcePanel.test.tsx`.
- **A7** `(GATED — real CLI credentials; Phase 7.8)` — Against pinned authenticated Claude Code and
  Codex CLIs with disposable homes/projects, verify native model/effort precedence, project
  instructions, skills/agents/MCP visibility, settings reload boundaries, Codex home/trust
  behavior, read-only operation, and redaction. Until recorded, fixture-backed resolver and launch
  behavior is shipped but live native-pass-through compatibility is not claimed.

## 6. Deviations & open decisions

- **Detached import is planned, not shipped (R9).** Every discovered setup asset is currently
  `reference_only`; no verified provider-specific launch-injection path can honor an independent
  copy. Implementing detach requires a new spec delta defining exactly which values/assets become
  AgentDeck-owned and how launches consume them.
- **Custom root/profile UI is incomplete.** The API accepts an explicit root and profile, but the
  current Settings/onboarding panel discovers only `root:"auto"` and has no profile picker.
- **Effective view is loaded on demand.** `GET /api/config-sources` returns candidates and binding
  health, not the full cached effective object. Settings displays effective model/inventory after
  preview or explicit refresh.
- **Live provider verification is gated (A7).** Provider mappings are fixture-tested but must be
  reconciled against pinned real Claude/Codex versions before making compatibility guarantees.
- **Binary-versioned `agentdeck_docs` knowledge MCP is not shipped.** Legacy Phase 7.9 describes it,
  but it is separate from configuration federation and requires its own feature-spec delta before
  implementation; current agents receive only the existing messaging MCP tools.

## 7. Traceability

- **Persistence/schema:** `internal/config/sources.go`; migration v8 in `internal/state/schema.go`;
  frozen snapshot access in `internal/state/session.go`.
- **Resolution/security/freshness:** `internal/configsource/{claude,codex,manager,watch,security}.go`.
- **API/composition:** `internal/server/config_sources.go`, `launch.go`, `resume.go`, `switch.go`,
  `messaging_registration.go`.
- **UI:** `ui/src/schemas/configSources.ts`, `ui/src/api/configSources.ts`,
  `ui/src/features/settings/ConfigSourcePanel.tsx`,
  `ui/src/features/onboarding/steps/SourceStep.tsx`, `ui/src/api/sse.ts`.
- **Regression anchors:** `internal/configsource/*_test.go`, `internal/config/sources_test.go`,
  `internal/server/config_sources_test.go`, `ConfigSourcePanel.test.tsx`, `SourceStep.test.tsx`.
