# Simple backend creation and global configuration linking

**State:** Waiting to start
**Why:** Replace the incomplete inline backend draft and exposed save-first source-link gate with a
provider-first, item-scoped create and global native-configuration connection experience.
**Relevant requirements:** FS-04.R40/A20, FS-08.R2–R5/R8/R23/R33/R34/A10–A11,
FS-09.R28/R38/R45/R47/A20, TS-03.R22–R23, TS-07.R15–R18,
INV §1/§2/§3/§8/§10/§11/§13/§15

## Outcome

People create one valid provider backend without committing unrelated Settings drafts. Claude/Codex
creation offers one **Create and use my configuration** action that saves the backend, links native
configuration globally, and immediately performs the provider-honest add-only import for that target.
A failed connection leaves the saved backend unbound and retryable. The normal flow is Linked only;
existing Mirrored bindings/API compatibility remain, but Mirrored is not offered as recovery.

## Included work

- Item-scoped `POST /api/backends` plus a provider-first dialog with matching editable name and the
  canonical valid starter model, preserving unrelated whole-catalog drafts.
- One-visible-action Claude/Codex create-and-connect, plus the same project-free global connection
  for an existing saved backend; the actual project still resolves at launch.
- Linked-only normal connection. Existing mirrored bindings and explicit API mode compatibility stay,
  without presenting Mirrored as a fallback it cannot provide.
- Immediate target-only provider import and continuing autosync: Codex local list-visible cache
  entries and declared effort metadata; Claude explicit user-level configured selectors and no
  inferred effort. Neither is presented as support, availability, or entitlement evidence.
- Best-effort catalog-first then binding persistence. Returned source-write errors attempt catalog
  restoration and never claim connection; interruption may leave only an unbound autosync-enabled or
  add-only-enriched backend, and stable-id retry converges safely.
- Global API/client schemas, mocks, backend/source query invalidation, source SSE, and post-commit-only
  generation publication.

## How we will know it works

FS-04.A20, FS-08.A11, and FS-09.A20 pass through focused server/UI regressions. Cover create-only and
create-and-connect for both providers; an unrelated dirty Settings draft; cancel and initial-write
failure; zero-project connection; discovery/preview/bind/source-write failure leaving the saved
backend unbound; exact replay and ordinary retry; target-only import and zero-model success; existing
models/defaults and other backends unchanged; existing Mirrored binding/API compatibility without a
fallback control; concurrent item-create/full-save/bind serialization; returned-write restoration;
accepted restart residue and convergence; no premature generation/SSE; both backend/source query
invalidation; and launch from a later actual project. Run `make check-specs`, `make test`, `make
build`, `cd ui && npm test && npm run build`, and `make dist`; use a browser pass to create/connect
Claude and Codex, retry a failed connection, and launch through the global binding from a later
project.

## Waiting on

None.
