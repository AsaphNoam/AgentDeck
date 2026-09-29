# Share adapter launch support with desktop controls

**State:** Waiting to start
**Why:** The operator accepted the 2026-09-29
[runtime capability assessment](../plans/runtime-capability-assessment.md) and requested the design
specifications. The original AI suggestion proposed a broader runtime/model capability layer;
repository inspection justified only this bounded extension of existing ownership.
**Relevant requirements:** FS-09.R60–R63, FS-09.A30–A32, TS-01.R36, TS-03.R47;
preserve FS-09.R13/R39/R52, TS-01.R12/R28/R35, TS-04.R47, TS-13.R15;
INV §1/§2/§3/§10/§11/§12/§17.

## Outcome

New Agent and Settings use the same adapter launch-support facts as the server. Terminal support
no longer needs a parallel browser allowlist; Settings prevents new declarations an adapter cannot
deliver while allowing explicit repair of values retained after a backend-type change.

## Included work

- Extend `internal/backend` with explicit interface support and a small derived launch-support value.
  Reuse effort/fast delivery declarations and the existing shared server validation/gates.
- Add the read-only `backend_support` projection to backend GET/PUT responses and the browser
  response schema, keeping editable config and ETags separate. Update New Agent, Settings and
  the dashboard Switch runtime dialog's existing Terminal control (the other caller of the browser
  allowlist), including missing-metadata retry and preservation of unsaved values.
- Keep model catalog/import semantics, session advertisements and state, provider protocol encoding,
  permission policy, queued sends, resume/clone/replacement behavior and the phone projection intact.
  No universal capabilities framework, new provider integration, wire-builder cleanup or data migration.

Evidence for the existing seams: `internal/backend/adapter.go` already owns effort and fast delivery;
`internal/server/terminal.go:17` and `ui/src/lib/backendTypes.ts:23` duplicate the Terminal rule;
`ui/src/features/settings/ModelRow.tsx:93` offers declarations rejected by
`internal/config/validate.go:270`. These are inspected repository behaviors, not claims about what
newer upstream CLIs can support. `internal/runtime/capabilities.go` and `chat.go` already own live
negotiation and ordered configuration; this change does not bypass or replace them. The assessment
records the fuller inventory and deferred boundaries.

Suggested sequence: adapter ownership and unchanged server gates → shared response/schema → the
desktop consumers and repair path → acceptance and closure checks. Keep all new requirements
marked planned until their implementation and applicable acceptance are complete.

## How we will know it works

FS-09.A30 verifies the current support matrix across registered adapters, real response/schema
agreement, config/ETag separation and unchanged phone fields. A31 covers New Agent, the dashboard
Terminal-switch control and existing pre-spawn refusal on launch/resume/switch. A32 covers Settings
type-change repair and metadata retry without lost drafts. Exercise the focused rendered journeys
after implementation, and run the applicable TS-06/workflow §2 closure matrix once after final edits.
Existing runtime protocol/order/queue tests protect unchanged semantics; existing credentialed
provider gates remain owed and are not satisfied by this metadata change.

## Waiting on

Nothing. Scope, additive response boundary and preservation rules were accepted by the operator.
Implementation has not started.
