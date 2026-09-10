# AgentDeck — handoff state settled through 2026-09-09

Archived under AGENT-WORKFLOW §16.7 to keep the live handoff header inside its session-start budget.
Everything below stood in the live handoff's changelog after the 2026-09-09 bug-investigation, fix,
and review sessions and is settled history. The live file is
[`../../features/HANDOFF.md`](../../features/HANDOFF.md). Earlier epochs are in
[`HANDOFF-through-2026-09-07.md`](HANDOFF-through-2026-09-07.md),
[`HANDOFF-through-2026-09-06.md`](HANDOFF-through-2026-09-06.md),
[`HANDOFF-through-2026-09-03.md`](HANDOFF-through-2026-09-03.md), and
[`HANDOFF-pre-sdd.md`](HANDOFF-pre-sdd.md).

## Changelog entries

**Changelog — 2026-09-09 (fix):** Closed BR-3's Must-fix and its unit (INV §1, INV §11). ACP lets an
adapter restore native context by replaying prior `session/update` frames during `session/load`, and
resume held no gate over that call, so every replayed frame was sequenced, persisted, published live,
and allowed to drive agent status — which is why an open chat visibly scrolled through old work on
wake. Resume now holds a replay claim from callback installation until `session/load` returns:
transcript events are suppressed for that window, replace-only live state (available commands,
context usage) is still accepted, and the suppressed count is logged. `session/new` replays nothing,
so launch is unchanged. New `TS-04.R50` records the rule. `fakeacp` gained a `FAKEACP_LOAD_HISTORY`
scenario and `TestResumeSuppressesProviderHistoryReplay`, confirmed to fail pre-fix with exactly the
reported symptom. No UI change was needed — append and bottom-follow are correct once the runtime
stops mislabeling history as live. Both Go variants, a focused `-race` run on resume, vet, build, and
spec checks pass. Settled 2026-09-08 entries moved to the `v0.4.2` state archive for header budget.

**Changelog — 2026-09-09 (fix):** Closed BR-2's immediate Must-fix. The release-private Codex CLI
is pinned to 0.153.4 throughout the manifest, lockfile, assembly checks, and release fixtures. The
release wrapper now defaults `CODEX_PATH` to that exact direct private executable, so codex-acp
1.1.2 cannot silently launch its nested 0.144.x dependency; an explicit environment override is
still preserved. TS-06.R22 and wrapper coverage record the executable-authority contract. Focused
release/CLI tests and the full closure matrix pass; no credentialed Astra prompt was sent, so that
acceptance gate remains open. The systemic model-catalog/runtime gap remains a Worth-fixing finding,
with `bump-pinned-acp-adapters.md` queued as its structural follow-up.

**Changelog — 2026-09-09 (bug investigation):** Confirmed BR-2 — model discovery reads the personal
Codex cache while execution uses the older release-private CLI, so an imported `gpt-6-astra` was
selectable but unusable. Recorded one immediate Must-fix (now closed) and one systemic Worth-fixing;
no provider prompt was sent and no reproduction test was committed. Full trace under **Bug
investigation reports**.

**Changelog — 2026-09-09 (review):** Re-reviewed the `chat-session-configuration` finding-fix range
`edb909a..8a01ebf`. All six original findings are materially addressed; one Worth-fixing protocol
edge remains (empty rebuilt option list, recorded under **Review findings**). Retracted the fix run's
Claude Must-fix: a credentialed prompt probe observed the requested model in SDK `system/init.model`,
the assistant message, and `modelUsage` for Haiku and Sonnet even though ACP still reported stale
`opus`, so adapter-local `currentValue` is configuration evidence, not an execution oracle. The
provider-oracle postmortem finding records that recurrence. Checks pass; the full provider acceptance
matrix remains open.

## Changelog entries — 2026-09-10 pinned adapter bump (review, work)

**Changelog — 2026-09-10 (review):** Reviewed `bump-pinned-acp-adapters.md`. The package pins,
lockfile, source-install pin, release fixtures, single-Codex assembly gate, and executable probe are
consistent with the release-runtime requirements. One Worth-fixing specification finding remains:
the steering requirement still says the current pins predate the extension and several ACP
compatibility statements still label the retired versions as pinned. The unit stays open for fix.
The invariant sweep applied §§2, 10, 11, 12, and 17; §§1 and 3–9 and 13–16 had no surface in this
dependency-and-packaging diff. Both Go variants, spec checks, the UI production build, and the
distributable rebuild pass. Credentialed provider journeys were not authorized or run and remain an
open acceptance gate.

**Changelog — 2026-09-10 (work):** Finished `bump-pinned-acp-adapters.md`. The release runtime now
pins Claude ACP 0.75.1 and Codex ACP 1.10.0; the Codex adapter dedupes onto the direct Codex 0.153.4
pin, and assembly now rejects a second nested Codex package. Version fixtures and the source-install
Claude pin were refreshed. Static inspection confirms the existing protocol, configuration,
permission, MCP, usage, command, and steering surfaces. The full automated matrix and distributable
build pass. No credentialed provider journey was authorized or run, so that acceptance gate remains
open and none of those live behaviors is claimed verified.
