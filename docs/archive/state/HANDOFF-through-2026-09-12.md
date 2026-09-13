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

**Changelog — 2026-09-12 (review):** Reviewed `file-read-nonregular-kind` (`22d77dc`), which
classifies a target through `os.Root` before opening it. Containment holds: `Root.Stat` refuses an
escaping symlink and the post-open descriptor check still guards replacement. Four findings — no
test fails without the fix and its one non-regular case silently skips on macOS; TS-05.R21 still
names `os.OpenInRoot`; an unreadable in-root file reports as outside the workspace; `filesearch.go`
keeps the superseded containment spelling. **Fix model:** medium — Codex Terra or Claude Opus.
Classes 2, 7, 8, 10, 14, 16, 17 apply; 1, 3–6, 9, 11–13, 15 have no surface. Archived three settled
entries for header budget; the slice is still over it.

**Changelog — 2026-09-12 (design-feature):** Recorded confirmed mail decisions: unread deferred
mail survives until delivery/read then uses 24-hour cleanup; uncertain delivery remains recoverable
with stable ids on a later authorized turn; inline batches contain bounded whole messages with
durable overflow. Intervention FYIs are best effort, with no atomic change/mail requirement.
Replacement context is supplied by the standing owner through assignment or ordinary mail; no
automatic inbox/history transfer. Updated FS/TS and acceptance criteria consistently. No product
question remains; the unit stays paused only for technical delivery mechanics. Spec checks, twin
skills and diff checks pass; no product code changed.

**Changelog — 2026-09-12 (design-feature):** Completed the mail technical contract against existing
runtime and state seams: optional wake intent, shared bounded inline preparation, provider-result
confirmation, stable-id recovery without extra wakes, turn-budget reservation and deferred retention.
Added TS-01.R31–R33, TS-02.R34 and TS-04.R53; completed readiness references and moved the pipeline
unit to Waiting to start. Removed its completed source idea. Spec checks, twin-skill and diff checks
pass. No product code changed; implementation and provider acceptance remain future work.

**Changelog — 2026-09-12 (design-feature):** Drafted waking/deferred durable mail and bounded
inline delivery in FS-06.R30–R36/A20–A25, FS-00.R17 and FS-14.R78/A45. Withdrew the separate
coordinator-update queue and delivery watermark in TS-09.R50 / TS-10.R36; intervention awareness
uses ordinary deferred mail. The expanded pipeline unit is paused for unread deferred-mail
retention, feature confirmation and the matching technical contract. Spec checks, twin skills and
diff checks pass; no product code changed.

**Changelog — 2026-09-12 (fix):** Closed all four `file-read-nonregular-kind` findings. Added
socket and FIFO regressions that fail without the pre-open kind check instead of skipping on macOS,
moved the replacement hook into the real stat/open window, and split a contained-but-unopenable file
out of `path_refused` into a new `file_unreadable` refusal. Restated TS-05.R21 as
classify-through-the-root-then-open with `os.Root` authoritative over `withinRoot`; extended
FS-03.A37 and TS-03.R40. Classes 2, 8, 10, 17. File-read tests pass under `-race`; the paused
pipeline work's messaging/pipeline failures are pre-existing and unrelated.

**Changelog — 2026-09-12 (design-feature):** Revised persistent orchestration so the standing
agent always owns and reports the stage; configured dedicated coordinators are managed children and
the normal coordination contact, with durable awareness of material direct intervention. Cleanup
now automatically reconciles transient failures with persisted backoff and attention only for
persistent/unsafe conditions. Confirmed same-identity stop/resume and assignment re-delivery. Updated
FS-14.R61–R77/A35–A44, FS-16.R30–R38/A20–A24, TS-09.R35–R50, TS-10.R25–R37 and TS-05.R22;
R60 is superseded by R75. Promoted the source idea to the waiting ready change. Spec checks, twin
skills and diff checks pass; no product code changed and no implementation is active.
