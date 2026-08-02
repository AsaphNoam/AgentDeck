# Simple backend creation and global configuration linking

**State:** Waiting to start
**Why:** Replace the newly added save-first source-link gate with a simple provider-first backend
creation and native configuration connection experience.
**Relevant requirements:** FS-04.R40/A20, FS-08.R2–R5/R23/R34/A11, FS-09.R28/R38/R45/R47/A20,
TS-03.R22, TS-07.R16–R17, INV §1/§8/§10/§11/§13/§15

## Outcome

People add a valid provider backend in a modal and can connect their Claude/Codex setup with one
read-only action. The connection is global to the backend, imports the provider's supported models
immediately, and keeps that model catalog synced without a project/mode/discovery prerequisite.

## Included work

- Provider-first Add backend dialog with matching editable name and valid starter model.
- One-click global Claude/Codex connection in the dialog and for existing backends.
- Automatic enablement plus immediate, add-only Codex/Claude model import after a successful normal
  connection.
- Compatibility-mode recovery only after normal connection failure; existing post-link provenance,
  overrides, refresh, and unlink remain.
- API/client conversion from required project-scoped source setup to the global flow, with compensated
  source/catalog persistence and query/SSE invalidation.

## How we will know it works

FS-04.A20, FS-08.A11, and FS-09.A20 pass through focused server/UI regressions. Run `make
check-specs`, `make test`, `make build`, `cd ui && npm test && npm run build`, and `make dist`; use a
browser pass to create and connect each provider, retry a failed connection, and confirm a later
project launches through the global binding.

## Waiting on

None.
