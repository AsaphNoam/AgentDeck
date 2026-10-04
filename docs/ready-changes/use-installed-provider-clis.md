# Use installed providers by default, with an explicit AgentDeck bundle choice

**State:** In progress
**Why:** Human request following the 2026-10-01 Codex 0.154.0/cache 0.159.2 warning and Claude
5.5 investigation: users update providers independently of AgentDeck and must not wait for a new
application bundle just to select a model their installed provider already supports. Follow-up
approval retains one explicit tested bundle per provider while capping compatibility maintenance.
**Relevant requirements:** FS-09.R68/R70–R72/R74–R78, A37–A38/A40/A42–A43/A45–A47;
FS-10.R21/R23–R24/A10–A12; TS-03.R52–R54; TS-04.R71–R73/R75–R77;
TS-06.R30–R31; INV §§1–5, 8, 10–12, 14–15, 17.

## Outcome

AgentDeck uses the person's installed Claude/Codex CLI for future work while keeping its managed
ACP adapters, SDKs and Node. Each backend can explicitly select the single Claude or Codex bundle
in the running AgentDeck release. No per-chat bundles or historical version picker. Updating an
Installed provider at its existing launcher makes the next process
use it, without changing a running agent or its conversation. Settings/New Agent show the selected
executable and last checked version; Refresh provider rechecks it and imports local model candidates
when autosync is enabled. Errors distinguish install, sign-in, provider update and actual adapter
incompatibility, with preserved input and explicit retry. Bundle mode follows application releases;
neither source switches automatically or promises native-session downgrade compatibility.

## Included work

- Add backend `provider_mode: installed|bundled`, omission meaning Installed. One resolver across
  process starts, Claude terminal, auth and readiness selects mode first. Installed uses existing
  override precedence then discovery; Bundle uses the release's native executable and ignores but
  preserves those overrides. Saving uses the existing editor/ETag flow. No new launch-time picker.
- Remove private PATH/provider-version authority from the release wrapper; launch managed adapters
  by absolute private Node/entrypoint paths. Reuse the existing provider packages as one selectable
  managed bundle each; keep immutable inventory and patches, not another package manager.
- Remove Codex's exact cache/runtime version gate; retain schema validation, opt-in and add-only
  import. Claude keeps configured-selector import. Exact model strings remain user configurable.
- Scoped response-only runtime metadata, an explicit bounded desktop refresh endpoint, advanced
  executable editing, accurate installation/update guidance and typed lifecycle recovery.
- Cover clean installs and upgrades from bundled defaults, personal auth versus isolated Codex
  session-home ownership, and application rollback (an old binary retains its old policy).
- Update README/setup and shipped operator knowledge during implementation. The open Claude 5.5
  compatibility finding is resolved by verified local-provider use, not merely by specifying it.

No automatic provider installation/update or source fallback/reversion, version allowlist, prompt
replay, model substitution, per-chat pin, new remote configuration authority, dependency stripping,
independent bundle updater or OpenCode/OpenHands changes. No native-session downgrade migration.
Retired requirements are tombstones, not work to implement. New source choice is explicit and
persistent, not the earlier temporary-local/automatic-return-to-bundle design.

Existing seams: `composeChildEnv` and the lifecycle callers in `internal/server/`, terminal drivers,
`internal/backend/providerauth/`, backend catalog handlers/import helpers, and `scripts/release/` wrapper
generation. Extend these; do not create a parallel runtime manager. The refresh endpoint is needed
because a read must not spawn a CLI and a settings save must not be required for a read-only recheck.

### Decision evidence and limits

The Claude report proves a stale embedded CLI blocked a model despite a sufficiently new installed
CLI. The Codex cache warning follows AgentDeck's exact-equality policy, not an observed protocol
failure. Executable override seams already exist (`CLAUDE_CODE_EXECUTABLE`, `CODEX_PATH`).

The historical audit found no documented provider-only upgrade regression in the inspected
AgentDeck history (2026-07-15–2026-10-03), but pinned releases and missing cross-version receipts
make that weak negative evidence. Reviewed Codex releases 0.154.0–0.159.2 did not establish a
macOS ACP compatibility break; OpenAI describes old-client/new-server compatibility as intentional
in [Unlocking the Codex harness](https://openai.com/index/unlocking-the-codex-harness/).
The [Claude changelog](https://github.com/anthropics/claude-code/blob/main/CHANGELOG.md), through
2.1.288, does record provider regressions/fixes around gateway auth, approvals, proxy requests,
MCP negotiation and resume. These establish that upgrades are not risk-free, not that the SDK and
CLI must move in lockstep. No reliable failure frequency follows from this sample.

Peer research gives no universal version policy. [Conductor](https://www.conductor.build/docs/faq)
bundles for compatibility and supports system executables; [Nodeterm's integration](https://github.com/eneskirca/nodeterm/blob/main/src/core/claude-cli.ts)
uses local CLI probes and targeted guards; [Zed](https://zed.dev/docs/ai/external-agents) distributes
agent packages independently. Terminal orchestration is not proof of SDK compatibility. The approved
choice retains local freshness plus a tested manual alternative, without taking on Zed-style runtime
distribution or historical-version support. One adapter stack per provider serves both modes.

## How we will know it works

FS-09.A37–A38/A40/A42–A43/A45–A47 and FS-10.A10–A12 specify executable-marker lifecycle/install/auth tests, bounded
probes, no-exec reads, concurrency and metadata-free saves, missing-provider recovery and the
rendered update → refresh → choose model journey. Apply normal TS-06 closure checks.

FS-09.A46/TS-06.R31 cap new real-provider verification at four combinations: two providers ×
(current bundle, one current stable Installed CLI). Run the named smoke, not an exhaustive feature,
model, historical-version, skin or account matrix; reuse identical receipts. Two combined browser
journeys in one skin suffice; shared tests/build closure cover the rest. Perform the one-time
structured-feature dependency audit in TS-04.R77, especially adapter-advertised Steer over an old
CLI. Reuse existing effective capability fields; no frontend semver tables, compatibility registry,
old-feature emulation or speculative version probes. Bumps rerun only the affected provider smoke;
unrelated changes and user CLI updates do not trigger recurring certification. A reproduced failure
adds a focused fixture. A larger necessary compatibility mechanism requires a separate scope decision.
Real receipts remain owed until authorized runs pass; do not modify users' global providers.

## Waiting on

Nothing to start implementation. Real-provider verification needs its normal explicit authorization
and credentials before shipping. Leave other queued changes and findings untouched; coordinate
identifier changes with the separate rename unit only if it lands first.
