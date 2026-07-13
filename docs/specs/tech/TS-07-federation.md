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
