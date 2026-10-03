# Use installed Claude and Codex providers

**State:** Waiting to start
**Why:** Human request following the 2026-10-01 Codex 0.154.0/cache 0.159.2 warning and Claude
5.5 investigation: users update providers independently of AgentDeck and must not wait for a new
application bundle just to select a model their installed provider already supports.
**Relevant requirements:** FS-09.R68–R74/A37–A44; FS-10.R20–R22/A10–A12;
TS-03.R52–R53; TS-04.R70–R74; TS-06.R28–R29; INV §§1–5, 8, 10–12, 14–15, 17.

## Outcome

AgentDeck uses the person's installed Claude/Codex CLI for future work while keeping its managed
ACP adapters, SDKs and Node. Updating the provider at its existing launcher makes the next process
use it, without changing a running agent or its conversation. Settings/New Agent show the selected
executable and last checked version; Refresh provider rechecks it and imports local model candidates
when autosync is enabled. Errors distinguish install, sign-in, provider update and actual adapter
incompatibility, with preserved input and explicit retry.

## Included work

- One executable resolver across process starts, Claude terminal, authentication and readiness.
  Preserve model > backend > ambient overrides using existing keys; otherwise discover on the user
  PATH and fixed macOS install locations, excluding managed runtime directories. Resolve each start.
- Remove private PATH/provider-version authority from the release wrapper; launch managed adapters
  by absolute private Node/entrypoint paths. Keep immutable dependency inventory and patches.
- Remove Codex's exact cache/runtime version gate; retain schema validation, opt-in and add-only
  import. Claude keeps configured-selector import. Exact model strings remain user configurable.
- Scoped response-only runtime metadata, an explicit bounded desktop refresh endpoint, advanced
  executable editing, accurate installation/update guidance and typed lifecycle recovery.
- Cover clean installs and upgrades from bundled defaults, personal auth versus isolated Codex
  session-home ownership, and application rollback (an old binary retains its old policy).
- Update README/setup and shipped operator knowledge during implementation. The open Claude 5.5
  compatibility finding is resolved by verified local-provider use, not merely by specifying it.

No automatic provider installation/update, bundled fallback, version allowlist, prompt/launch replay,
model substitution, per-chat provider pin, new remote configuration authority, dependency stripping,
or OpenCode/OpenHands changes. No native-session migration or guarantee of downgrade compatibility.
The retired FS-09.R64–R67 recovery draft must not be implemented.

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

Bundled-default plus optional local mode keeps the demonstrated model delay and adds two execution
policies. Automatic fallback adds silent version/session ambiguity and still needs compatibility
testing. Removing managed adapters as well would expose AgentDeck's actual protocol contract and
patches to independent upgrades. Local providers with managed integration components is the chosen
boundary; verify it with fixed-adapter tests rather than a guessed maximum provider version.

## How we will know it works

FS-09.A37–A43 and FS-10.A10–A12 specify executable-marker lifecycle/install/auth tests, bounded
probes, no-exec reads, concurrency and metadata-free saves, missing-provider recovery and the
rendered update → refresh → choose model journey. Apply normal TS-06 closure checks.

FS-09.A44/TS-06.R29 additionally require authorized real-provider receipts: hold AgentDeck,
adapter/SDK/Node fixed and run the reference CLI and a newer stable CLI through existing features.
Record exact versions and outcomes, including resume, models/effort, permissions, cancellation,
MCP/roles/skills and advertised Codex features. These receipts are owed, not already passed; they
gate shipping this change, not each future user update. Do not update the person's global CLI for
tests. An actual adapter incompatibility must be investigated, not hidden with bundle fallback.

## Waiting on

Nothing to start implementation. Real-provider verification needs its normal explicit authorization
and credentials before shipping. Leave other queued changes and findings untouched; coordinate
identifier changes with the separate rename unit only if it lands first.
