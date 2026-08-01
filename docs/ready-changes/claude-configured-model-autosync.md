# Sync configured Claude models

**State:** Waiting to start
**Why:** The human requested Claude backend model autosync, chose configured-only discovery, and
confirmed portable family aliases for fresh homes.
**Relevant requirements:** FS-09.R45, FS-09.R46, TS-01.R14, INV §2, INV §3, INV §7, INV §9,
INV §10, INV §11, INV §14

## Outcome

An opted-in Claude backend adds models explicitly named by the user's Claude settings to AgentDeck's
future-launch picker at dashboard startup. Fresh homes already offer Fable, Opus, Sonnet, and Haiku
through portable aliases, with Sonnet as the default.

## Included work

- Extend the existing `autosync_models` setting and Settings copy to `claude-acp`.
- Add a strict, allowlisted reader for user-level Claude `model`, `availableModels`, and
  `fallbackModel`; reuse catalog model validation and the shared add-only startup merge.
- Merge successful Claude and Codex additions into one validated snapshot and atomically rewrite
  `backends.json` only when it changes.
- Seed `fable`, `opus`, `sonnet`, and `haiku` with generic labels on fresh homes only; preserve all
  existing catalogs, defaults, entries, effort metadata, and running/frozen sessions.
- Do not add full-catalog discovery, private-cache/binary/env/project/policy/network/session reads,
  removal/provenance, effort discovery, a refresh endpoint, persistent sync status, or a schema/API
  change.

## How we will know it works

- FS-09.A18: parser, filtering, duplicate/preservation, source-isolation, combined-provider
  persistence, and Claude Settings opt-in/restart-copy regressions pass.
- FS-09.A19: fresh seed, no-clobber, onboarding, and New Agent catalog regressions pass.
- `make check-specs`, both Go test modes, affected UI tests/build, source build, and distribution
  build pass.

## Waiting on

None.
