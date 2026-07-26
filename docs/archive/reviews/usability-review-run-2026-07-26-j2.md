# Usability Review Run — 2026-07-26 J2 onboarding

**Scope:** J2 onboarding against `95db69b`, focused on the newly changed **Set up later** and
**Check again** controls plus the ordinary Backend → Project → Config → Launch path. The run used
two fresh isolated homes and did not invoke real provider sessions or credentials.

**Review surface:** release-style `sqlite_fts5` binary (`make build`); in-app browser Playwright,
DOM, screenshot, computed-style, and console APIs (browser ladder rung 1); a review-owned Claude
adapter shim backed by the repository fake ACP peer; loopback API and on-disk state checks. Evidence
is under `.review/usability-20260726-J2/evidence/`. Product code and specifications were not changed.

## Executive summary

1. **One Must-fix usability finding.** A compatible signed-in Claude adapter that rejects the
   optional `--no-color` flag as `unknown flag` rather than `unknown option` is reported as failed,
   so the ordinary onboarding gate cannot advance.
2. **Set up later passes in the real rendered UI.** One pointer click closed the modal, opened the
   empty dashboard, persisted completion, left the four-backend seeded catalog and defaults intact,
   created no project beyond `my-app`, and launched no session.
3. **Check again passes for its normal state transitions.** Missing adapter, signed-out adapter, and
   ready adapter states each refreshed in place with the expected actionable guidance or advance.
4. **The ordinary path passes after readiness succeeds.** Project creation, optional Config
   continuation, first fake-ACP agent launch, completion persistence, server restart, and reread all
   worked with no browser console error.
5. **Real credential coverage remains gated.** No authenticated provider session, API key, or login
   flow was invoked; the browser run used only the bounded fake-provider acceptance path.

## Journey results

| Step | Result | Evidence |
|---|---|---|
| J2.1 Fresh wizard render and provider guidance | **PASS** — styled 720px modal, visible provider-owned sign-in instructions, active pointer controls, zero console errors. | `J2-skip-before.png` |
| J2.2 Set up later | **PASS** — one click revealed the empty dashboard; `onboarding_complete` became true; seeded catalog/defaults remained Claude `sonnet` and Codex `gpt-5.6-sol`; only seeded `my-app` existed; session count stayed zero. | `J2-skip-after.png` |
| J2.3 Missing adapter | **PASS** — Validate changed to Check again and explained that the Claude adapter must be installed. | `J2-missing-adapter.png` |
| J2.4 Installed but signed out | **PASS** — Check again refreshed to provider-specific signed-out guidance without dismissing the wizard. | `J2-signed-out.png` |
| J2.5 Signed in with alternate optional-flag rejection wording | **FAIL — BLOCKER** — reproduced and confirmed: the provider is ready without `--no-color`, but both checks remain on generic credential failure instead of advancing. | `J2-alt-flag-failure.png`, `J2-alt-flag-failure-confirm.png` |
| J2.6 Ready provider and project creation | **PASS** — Check again advanced; `J2 Review Project` was created with a server-derived id and the selected real workspace path. | browser DOM plus persisted `projects/j2-review-project-*.json` |
| J2.7 Optional Config step | **PASS** — provider/project context rendered, Continue advanced, and the cross-feature hint styling was present. | `J2-config-step.png` |
| J2.8 First launch and completion | **PASS** — fake ACP launched one Implementer agent on Claude `sonnet`, the wizard closed, and completion persisted. | `J2-onboarding-complete.png` |
| J2.9 Restart reread | **PASS** — after a server restart and browser reload, the wizard stayed closed and the persisted agent/project/catalog rendered with zero console errors. | `J2-restart-persisted.png` |
| Real authenticated Claude/Codex readiness | **SKIPPED(credentialed provider gate not authorized)** | — |

## Findings

### BLOCKER — J2 optional flag wording wrongly fails a ready provider

```text
SEVERITY: BLOCKER
WHERE: J2 step 5 (fresh isolated home, port 46262)
REPRO: adapter rejects `auth status --no-color` with `unknown flag: --no-color`, then reports `logged in` for bare `auth status` → click Check again twice
EXPECTED: AgentDeck retries the allowlisted bare status command and advances to Project
OBSERVED: both checks show “The credential check failed” and the wizard cannot advance
EVIDENCE: .review/usability-20260726-J2/evidence/J2-alt-flag-failure.png and J2-alt-flag-failure-confirm.png
```

The running failure matches FS-04.R17/R34/A14, FS-09.A5, TS-04.R15, and INV §12. The retry at
`internal/backend/credcheck/claude.go:28-31` recognizes only one exact rejection phrase, while the
invariant requires defensive output vocabulary for optional user-CLI flags. Add a credential-probe
regression for alternate common flag-rejection wording and retry the fixed bare argv whenever the
failure identifies the optional flag as unsupported.

## Static sweeps

- **S1 serialization:** one latent type mismatch: `PUT /api/config` returns raw config without the
  computed onboarding block while the client declares a full Config response. Current J2 mutations
  ignore that body and refetch, so no runtime failure was promoted.
- **S2 CSS wiring:** no referenced-but-undefined or meaningful orphaned onboarding selector; the UI
  style checker passed. Config-step `.source-hint` styling is owned by Settings CSS and was verified
  in the rendered journey.
- **S3 external CLI variance:** the exact-string `--no-color` retry lead reproduced as the BLOCKER
  above.
- **S4 null hostility:** no J2 collection dereference lead; server boundaries normalize collections
  and onboarding guards its collection reads.
- **S5 error surfacing:** Backend validation stringifies structured HTTP failures to only
  `Error: HTTP <status>`, and initial backend-load failure leaves validation disabled without an
  explanation. These remain source risk leads because this run did not inject server/storage
  failures and did not treat them as observed findings. Set up later's error path is wired.

## Coverage notes

- The charter's generic Back-path phrase still has no matching control: the specified wizard is
  forward-only and exposes Set up later instead. This is a matrix wording gap, not an acceptance
  mismatch.
- The fake adapter never started a provider login flow or handled credentials. Its status branch
  supplied missing, signed-out, alternate-flag, and ready outcomes; its ordinary launch branch
  delegated to the repository fake ACP peer.
- Screenshots and harness state are review-owned and gitignored. The committed report plus exact
  fixture/steps above is the durable reproduction record.
