# Runtime and model capability assessment

**Date:** 2026-09-29 · **Baseline:** `367a433`
**State:** Assessment accepted 2026-09-29; retained rationale, not an authoritative specification.
The bounded design is now [ready to start](../ready-changes/centralize-launch-support.md), governed
by FS-09.R60–R63, TS-01.R36 and TS-03.R47. Implementation has not started. The promotion call-site
check also included the dashboard Switch runtime dialog, which shares the browser Terminal rule;
the baseline assessment below predates that additional consumer check.
**Origin:** Operator requested a skeptical assessment of an AI-generated architecture suggestion.

## Recommendation

Do not undertake a broad capability-architecture refactor. AgentDeck already has adapter-owned
delivery, a common model catalog, shared launch validation, and negotiated session capabilities.
The premise that launch, pipelines and mobile independently encode most provider knowledge is
not supported by the inspected code.

A small cleanup is useful: make supported launch interfaces and setting delivery available from
the existing adapter boundary, and project that information to New Agent and Settings. Retain the
existing model fields and session capabilities. This is a small-to-medium cross-layer change, not
a new subsystem: adapter metadata, a response projection, the existing server gates, two desktop
consumers, and focused tests. Its immediate benefit is removing a duplicated Terminal allowlist and
preventing Settings from offering declarations that the server rejects. It is worthwhile as one
bounded change; it does not justify reorganizing every runtime consumer.

## What exists today

| Layer | Current ownership and flow | Assessment |
|---|---|---|
| Adapter definitions | `internal/backend/adapter.go:14` owns binaries, environment handling, native resume IDs, model-switch support, hooks, effort delivery and session-option identifiers. `For`/`Types` enumerate one registry. | Preserve and extend. |
| Configured models | `internal/config/types.go:63` carries provider model string, efforts, default effort and fast support. `ValidateModelEffort`/`ValidateModelFast` in `validate.go:309` combine model declarations with adapter delivery support. | Already a canonical model representation. |
| Discovery | `config/codexmodels.go:77` reads the local Codex cache; `claudemodels.go:48` reads configured Claude selectors. `modelautosync.go:28` owns their shared add-only merge. | Provider-specific sources, common output and merge are appropriate. |
| Launch consumers | Server composition and pipeline/task paths use shared launch resolution and validation. New Agent, chat runtime selection and pipeline assignments consume model metadata; `ui/src/lib/runtimeSelection.ts:13` already shares part of selection reset logic. | No separate provider rules to replace in most selectors. |
| Session configuration | `runtime/chat.go:2430` applies model, effort, then fast after creating/loading the session, using adapter option IDs and the current option advertisement. Setting a model may replace that advertisement. | Correct centralized execution boundary. |
| Native features | `runtime/capabilities.go:59` normalizes handshake advertisements for fork, subagents, background tasks and file changes. Steering is separately read from the live peer. Start and resume negotiate again. | Already capability-driven, including Codex extensions. |
| State and controls | State exposes `runtime_capabilities`, `fast_available`, `steering_available`, and derived clone eligibility. Desktop and phone steering consume the same flag; background-task controls consume the normalized capability. | Different lifetimes and eligibility rules are intentional. |
| Phone replacement | `server/remote_home.go:231` projects model IDs/names, efforts/default and validated fast support. `ui/src/remote/WorkScreens.tsx:304` consumes it without knowing backend types. | Already the proposed direction; preserve the restricted projection. |

Paths in the table without a leading `internal/` are relative to `internal/`.

Codex discovery is useful but not a continuously refreshed authoritative provider catalog. It reads
`models_cache.json`, imports only visible entries, maps provider reasoning strings and fast speed
tiers, and checks cache/runtime version equality when a packaged runtime version is supplied.
Startup or native-source connection imports new entries; it does not update or remove existing
ones, change defaults, prove account entitlement, or start a session to discover support. This
add-only behavior intentionally preserves user configuration (FS-09.R28/R38/R53/R59).

Live ACP configuration is a second source with a different purpose: it establishes what the actual
session currently offers. Model and effort failures remain fatal to applying the requested launch;
an unavailable fast boost can fall back to normal speed. The code preserves this distinction and
refreshes the option list after model changes. A catalog flag must never replace that live check.

## Actual duplication and its cost

1. **One backend rule is duplicated across languages.**
   `internal/server/terminal.go:17` and `ui/src/lib/backendTypes.ts:23` both spell Terminal support as
   `claude-acp`. New Agent combines that browser rule with host terminal availability at
   `NewAgentModal.tsx:130`. A support change requires coordinated server/browser edits. No current
   disagreement was found; the cost is a concrete future synchronization obligation.
2. **Settings does not consume the adapter's setting support.**
   `ui/src/features/settings/ModelRow.tsx:93` offers effort declarations and a fast checkbox for
   every backend. `internal/config/validate.go:270` rejects unsupported declarations. This creates
   a predictable edit-then-reject path for OpenCode/OpenHands. It is source-established behavior,
   not a browser-observed usability finding.
3. **Provider wire construction is repeated inside runtime.**
   `chat.go:2269` and `chat.go:2314` repeat new/load parameter construction, including Claude
   metadata and Codex prompt/model omission. `deliveredModelID` and `applySessionConfig` each
   identify Codex's post-session model delivery. These are real maintenance seams, but capability
   flags alone do not solve wire encoding. A later bounded consolidation of the two builders or
   an adapter model-delivery declaration may help. Do not bundle process preparation, profile
   copying, error translation and all session serialization into a generalized adapter callback.
4. **Picker reset behavior is repeated, but mostly not provider knowledge.**
   Pipeline and phone replacement forms each handle selection/reset alongside the desktop helper.
   The phone deliberately preserves unavailable standing values and asks for a model after changing
   backend; a mechanical unification could change that flow. Share model semantics only where
   equal; do not force all controls into one widget or alter defaults as part of this change.

There is historical evidence that session parameter parity and effective setting delivery are
costly: INV §2 documents new/load drift, and TS-04.R47/R54 plus the protocol-schema tests document
the Codex model-delivery correction. That supports preserving and testing the existing shared
execution seam. It does not demonstrate a need for a plugin framework or a catalog rewrite.

## Minimal target architecture

Keep three distinct facts, each with one owner:

- **Adapter/interface support:** what AgentDeck implements for this backend and interface.
  Extend `internal/backend` with a small typed read model, conceptually
  `LaunchSupport{available, effort, fast}` for each existing interface. Declare interface support
  explicitly; derive effort/fast from the existing delivery methods, gated by that interface.
  Do not maintain a second effort/fast allowlist. Do not infer Terminal support from a nonempty
  hook map: Codex has a hook map today but lacks the verified Terminal path.
- **Configured model choices:** keep `config.Model.efforts`, `default_effort`, and `fast` exactly
  where they are. Launch controls intersect them with the selected interface's support; server
  validation stays authoritative. No new model registry or duplicate capability record is needed.
- **Actual session support and action eligibility:** retain the current normalized handshake
  capabilities, fast/steering availability and clone affordance. Live advertisements and lifecycle
  state remain authoritative at execution time. Do not fold them into static backend metadata.

Accepted API boundary: add a read-only adapter-support map to the
existing desktop backend response, keyed by registered backend type and covering all four types
so Settings can edit an unsaved backend/type. Keep it outside editable `BackendsConfig`; never save
it into `backends.json` or include it in the catalog ETag. Existing request bodies and model fields
remain intact. Keep `/api/capabilities` for host terminal-driver availability; New Agent still
combines host availability with adapter support. A missing support projection disables the relevant
optional control until refreshed rather than guessing a provider rule; server validation remains.

The phone retains its existing allowlisted response shape and replacement validator. It does not
receive backend types, paths, environment, credentials or the desktop configuration response.
Pipelines and tasks retain their current model-neutral assignments and shared launch validation.

In Settings, unsupported new effort/fast declarations would no longer be offered. If an existing
draft contains them, keep them visible with an explanation and a way to remove them; never silently
erase data on a type change. This is the only proposed visible behavior adjustment beyond sourcing
existing launch affordances from the server. The operator accepted this scope on 2026-09-29.

## Migration boundaries and exclusions

| Area | Boundary |
|---|---|
| Provider discovery | Keep Codex cache decoding, Claude settings decoding, version compatibility and existing add-only rules. Moving these pure readers into the adapter package gains little and risks package cycles. |
| Provider protocol | Keep option IDs, wire metadata, environment overlays, credential guidance, Codex home isolation, native resume IDs and provider-specific flags specialized. Preserve the generic runtime's ordered application and error handling. |
| Queued sends | Host-owned turn arbitration, not a provider queue capability. Never delegate this to an advertised provider queue without a separate behavior change. |
| Steering | Already live-advertised; preserve active-turn checks and host-owned fallback semantics. No static `codex supports steering` rule. |
| Permissions | Preserve frozen AgentDeck skip policy, pending-request handling and provider outcome mapping. Do not unify provider-native autonomy modes or absorb the separate live-permission idea. |
| Resume/replacement/clone | Preserve native resume vs primer rules, transactional replacement, frozen launch settings and state eligibility. A support flag does not grant permission to perform an action or establish cross-provider session compatibility. |
| Session capability storage | Preserve snapshot vs live lifetimes and revalidation. Do not migrate existing state/API fields simply to make their names uniform. |
| New upstream features | Adapter-only changes are realistic for another model/value using existing semantics. A new operation, lifecycle or UI meaning still requires consumer work and explicit acceptance criteria. |

No plugin loader, negotiation protocol beyond existing ACP, policy engine, generic capability bag,
universal permission enum, generalized speed tiers, capability polling service, database migration,
automatic rewrite of old models, or new provider support belongs in this change.

## Migration and verification

1. The bounded behavior and additive read-only API projection are confirmed and specified in
   FS-09 and TS-01/03; use TS-04's existing delivery contract. No new FS/TS family is needed. Keep
   FS-03 and TS-13 semantics unchanged. Relevant invariants: INV §1/§2/§3/§10/§11/§12/§17.
2. Define adapter launch support using existing delivery declarations. Route the current server
   Terminal gate through it, preserving rejection codes and all four backends' behavior.
3. Project support into the desktop response; switch New Agent and Settings to consume it. Keep
   existing model, pipeline, phone and session contracts. Delete the browser Terminal allowlist.
4. Verify the completed slice once with the applicable TS-06 matrix. Focused acceptance must prove:
   all registered adapters keep their current supported interface/effort/fast combinations;
   unsupported launches are rejected before spawn; Settings cannot create an unsupported declaration
   through its controls and can still repair a stale draft; the real serialized response reaches the
   browser schema; the phone still exposes only its allowed fields; and no response-only field is
   written back into configuration. Check the Settings repair path and New Agent in the rendered UI.
5. Preserve existing protocol-schema, new/load parity, model→effort→fast ordering, changed-option-list,
   missing-capability, ignored-setting, native-resume and queue/steer tests. If a later change touches
   wire construction, make it a separate scoped change and run the applicable pinned-provider gates;
   fake ACP emission is not proof of provider execution.

The principal risk is confusing configured support with live availability or authorization. Other
risks are changing frozen settings, saving derived metadata, accidentally broadening phone data,
silently clearing Settings drafts, and overgeneralizing wire formats. The boundaries above avoid
those without adding a new runtime layer.

This assessment inspected repository code, specifications and regression coverage. It did not
execute providers, measure maintenance time, or validate the UI in a browser. Existing credentialed
provider gates in HANDOFF remain owed; no statement here claims newly verified upstream support.
