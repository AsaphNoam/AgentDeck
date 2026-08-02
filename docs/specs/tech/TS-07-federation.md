# TS-07 — Configuration federation

**Status:** Partial
**Code:** `internal/configsource`, `internal/server/config_sources.go`, launch/resume/switch composition, `ui/src/features/settings/ConfigSourcePanel.tsx`
**Absorbed:** exact Phase 7.5–7.8 mapping in the [phase archive manifest](../../archive/phases/README.md)

## 1. Scope

This spec owns the architecture for read-only discovery and composition of Claude Code/Codex native
configuration: authority, provider resolvers, preview consent, binding persistence, effective views,
freshness/watch behavior, cache, launch freezing, and redaction. FS-08 owns user-visible behavior.

## 2. Design & constraints

**R1 — Federation is one-way.** Native provider files remain authoritative in linked and mirrored
modes. AgentDeck persists a binding, approved roots, and explicit overrides; provenance,
fingerprints, generation, and health are derived manager/snapshot/cache state. An optional
owner-only mirror is disposable cache. AgentDeck never writes the native source.

**R2 — Resolution is provider-native and pure.** Claude and Codex resolvers implement their real
precedence and inventory rules from an explicit source root/project/profile plus an approved-root
set. Resolution reads bounded allowlisted files, follows imports only inside approval, and returns a
redacted effective model plus a sanitized report; it performs no writes.

**R3 — Discovery does not grant consent.** Discovery returns candidate roots/metadata. Preview
resolves a selected candidate/mode/project and mints a short-lived single-purpose token binding the
source root, profile, mode, fingerprints, and redacted result. Bind consumes a matching unexpired
token; source-root/profile/mode changes require re-preview.

**R4 — Consent is backend/source-root scoped; resolution is project-aware.** A backend binding can
serve different AgentDeck projects. Every resolution admits the currently selected canonical
project root for that resolution, without persisting it as new source-root consent or requiring a
new preview merely because the AgentDeck project changed.

**R5 — Effective composition has explicit precedence.** Launch-explicit values beat AgentDeck
binding overrides; overrides beat provider-native resolved values; absent explicit/override values
are natively inherited by omitting the ACP model where required. Every effective field carries
provenance (`explicit`, `override`, native source, default/absent).

**R6 — A launch freezes what it used.** Before a new launch or resume-with-refresh, the manager
resolves fresh and rejects invalid/approval-conflict state. The redacted effective object and
fingerprints are stored in the session snapshot. Normal resume and runtime/backend switch carry the
frozen snapshot unless resume explicitly requests config refresh.

**R7 — Watchers are acceleration; launch freshness is correctness.** Bindings populate immutable
manager generations. Startup hydrates every persisted binding before watch/sweep starts. fsnotify
with debounce handles prompt edits; a periodic sweep covers missed events. Every launch still
performs a fresh resolve independent of watcher timing.

**R8 — Health transitions are explicit.** Resolution produces healthy, stale, source-invalid,
approval-required, or source-conflict states with sanitized changed-field metadata. On refresh
failure the last known redacted generation may remain visible as stale, but cannot silently pass a
fresh launch gate.

**R9 — Mirrored data is disposable cache.** A mirror contains only copyable/redacted effective
material, is owner-only, generation-addressed, and can be regenerated from native authority.
Reference-only/unsupported assets are inventory metadata, not copied content.

**R10 — API/UI never expose source contents or secret values.** Effective view exposes high-level
values, provenance, field/key names, asset kinds/paths/hashes, health, and changed keys. It does not
return native file bodies, environment values, credential material, hook bodies, or tokens after use.

**R11 `(planned)` — Detached import materializes only copyable contracts.** Detach will make an
AgentDeck-owned copy only for fields/assets with a verified injection path. Until then
`detach=true` returns `501 not_implemented`; ordinary unbind works.

**R12 `(planned)` — Provider compatibility is acceptance-gated.** Pinned real Claude/Codex versions
must prove discovery, precedence, native model inheritance, refresh, launch, and resume before the
release claims complete federation compatibility.

**R14 — The stored effort override becomes resolver input.** The binding's effort
override, already resolved and redacted into the effective view, is handed to TS-01.R12's single
`resolveEffort` as the tier below an explicit launch effort and above the model's `default_effort`
— the same position the model override already occupies. Federation gains no second resolution
order and no new persisted field: `config-sources.json` and the frozen launch object keep their
current shapes, and the value they already carry simply stops being inert. A bound launch whose
override names a level the selected model does not declare fails under FS-09.R42 like any other
undeclared level; the source is never rewritten to repair it (R4).

**R15 — Backend-catalog replacement reconciles source ownership.** After a successful complete
`backends.json` save, the server removes persisted bindings whose backend key is absent or whose
provider no longer matches the backend type, then drops their manager generations. A Settings-only
draft is not a backend identity and cannot enter the bind route; that rejection precedes preview
token consumption. Compatible bindings are retained.

**R16** `(planned)` — The ordinary source connection remains the existing preview-token/bind
protocol, composed by the client as one action: a standard auto-root `linked` preview with no
project followed immediately by its matching bind. An omitted project resolves only the provider's
user-level source during connection; the persisted binding remains backend-global and later
launches still pass their actual project to R2/R4's project-aware resolver. The regular UI never
selects a root, profile, project, claim set, or binding mode. The server keeps preview-token expiry,
one-use consumption, and fingerprint re-checking unchanged; a normal connection never weakens
consent or path approval merely to remove an intermediate screen.

**R17** `(planned)` — An enabled bind (`enable_model_sync:true`) is the sole source-connection
seam that turns on `Backend.AutoSyncModels` and performs its immediate provider-specific add-only
import. It reuses the existing Codex/Claude local readers and merge rules rather than parsing native
model files in the handler: Codex includes its declared reasoning metadata, Claude imports only
configured selectors, and either reader's missing/invalid input is a reported zero-model success.
Before writes, the handler validates the backend/provider/token and computes both the replacement
source manifest and normalized backend catalog. Backend-catalog replacement, source bind, and this
combined update share one server-side catalog mutation lock. The handler writes the source manifest
and backend catalog as a compensated local transaction: if the second owner-only atomic write fails,
it restores the first durable preimage, drops the attempted generation, and returns an error rather
than reporting a connection whose model-sync state is split. Only after both writes succeed does it
install/announce the new generation. Ordinary bind callers that omit the flag retain the existing
source-only write path.

## 3. Interfaces & data shapes

`config-sources.json` version 1 stores one binding per backend: provider/root/profile/mode, claims,
approved roots, and explicit overrides. Fingerprints/effective data are held in manager generations,
health is derived by resolution, and frozen effective data belongs to the session snapshot; mirror
cache is regenerable. Preview tokens are in-memory, expiring, and single-use.

REST/SSE routes are listed by TS-03. The `config_source_update` event identifies backend/project,
generation, health, changed field names, and stale state; it never carries source content.

## 4. Invariants

- **INV §1:** startup/rebind/reconnect repopulates watcher generations and republishes derived state.
- **INV §3:** the session freezes the effective launch object and provenance it actually used.
- **INV §10:** native source is authority; mirror/effective views are cache/projection with explicit
  refresh boundaries.
- **INV §13:** every provider path/error/effective view crosses the redaction boundary.
- **R13 — TOCTOU closure.** Bind verifies the preview-bound fingerprint/selection immediately before
  committing; a changed source requires a new preview rather than accepting stale consent.

## 5. Deviations & open decisions

- Detach is not implemented (R11); the UI shows it as unavailable and unbind remains supported.
- Custom root/profile controls are not exposed in the normal UI although the API/resolvers support
  them. Effective-view SSE invalidation and prompt watch registration have known usability gaps.
- Real-provider acceptance in R12 is credential-gated. Fake/fixture coverage proves AgentDeck's
  resolver/manager behavior, not undocumented provider compatibility.

## 6. Traceability

- Store/manager/watch: `internal/config/sources.go`, `internal/configsource/manager.go`, `watch.go`.
- Resolvers/redaction: `internal/configsource/claude.go`, `codex.go`, `types.go`, `security.go`.
- Server/composition: `internal/server/config_sources.go`, `launch.go`, `resume.go`, `switch.go`.
- UI: `ui/src/features/settings/ConfigSourcePanel.tsx`, onboarding `SourceStep.tsx`.
- Regression anchors: `TestHydrateBindingsPopulatesGenerations`, provider resolver precedence/
  symlink/redaction tests, `TestComposeLaunchFreezesFederationConfig`, UI mirrored-token and override
  tests.
