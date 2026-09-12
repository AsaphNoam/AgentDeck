# AgentDeck — archived handoff state through 2026-09-12

Settled changelog entries moved out of [`../../features/HANDOFF.md`](../../features/HANDOFF.md) to
keep its session-start header inside budget. Nothing here is live state; the handoff carries the
resumable position.

## Changelog

**Changelog — 2026-09-12 (design-feature):** Designed the **Deckhand** rename: one cut through every
identity, `~/.agentdeck` migrated by `os.Rename` at first start, `agentdecker` → **FirstMate**, clean
CLI swap, dual-read annotation prefix, browser-key copy-forward. Existing installs move over by
running the installer once, not `agentdeck update` — GitHub documents rename redirects only for web
links and git clone/fetch/push. MCP tool names were never branded and do not change. Added
FS-00.R16, FS-04.R48/A28, FS-10.R15–R19/A7–A9, FS-13.R24/A15, FS-18.R14/A10, TS-02.R32–R33,
TS-04.R52, TS-06.R24, TS-08.R58, TS-11.R14; seven specs moved Current → Partial with the index.
Promoted to `docs/ready-changes/rename-product-to-deckhand.md`. Checks pass; no product code changed.

**Changelog — 2026-09-12 (review):** Reviewed `fix-model-recommendations` across its workflow,
mirrored role launchers, validation scripts, mutation tests, and build wiring. The change consistently
records one recommendation per grouped fix unit at the level of its most difficult open fix, and the
tests independently reject missing, duplicate, per-item, mismatched, or weakened routing. No findings;
the unit is closed. Focused finding-contract, launcher-contract, mutation, handoff-spec, and diff checks
pass. Invariant classes 2, 4, 7, 10, and 17 apply; classes 1, 3, 5, 6, 8, 9, and 11–16 have no applicable
surface.

**Changelog — 2026-09-12 (fix):** Hardened the New Agent modal tests to wait for the Launch
button to become enabled before clicking it, closing a CI timing race around asynchronously loaded
role and project state. All 450 UI tests pass; product behavior is unchanged.
