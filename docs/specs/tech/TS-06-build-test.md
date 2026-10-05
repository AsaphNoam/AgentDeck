# TS-06 — Build, test & delivery

**Status:** Partial
**Code:** `Makefile`, `go.mod`, `ui`, `internal/server/ui`, `install.sh`, `scripts/`, `internal/cli/`, `.github/workflows`
**Absorbed:** build/test sections in the [phase archive manifest](../../archive/phases/README.md) and contributor guidance formerly duplicated in [`CLAUDE.md`](../../../CLAUDE.md)

## 1. Scope

This spec owns supported toolchains, build tags, UI embedding, release/install constraints, required
verification, spec linting, and test conventions.

## 2. Design & constraints

**R1 — retired 2026-07-15:** This described the source-build toolchain before the planned private
Node runtime in R13. Source-build requirements remain part of R13.

**R2 — Release builds enable FTS5.** Every distributed Go build uses the `sqlite_fts5` tag. The
untagged path remains supported solely as the tested metadata-search fallback; a release command
without the tag is a defect.

**R3 — The UI is embedded, not hand-edited.** `ui/src` is the source. `make embed` builds the Vite
app and copies `ui/dist` into `internal/server/ui/dist`; agents never edit the embedded output.

**R4 — Standard targets have stable meaning.** `make build` creates the tagged binary; `make test`
runs spec lint plus both Go variants; `make dist` builds UI, refreshes embed output, and builds the
tagged binary; `make check-specs` runs the mechanical spec contract.

**R5 — Required checks match the work but are never selective.** A product-code change runs both Go
test variants and any affected UI build/tests; concurrency hot spots add focused race tests. A docs-only
spec/workflow change runs spec lint and link/reference checks plus any build/test needed to
validate claims it changed. Failures may not be hidden by removing or weakening tests.

**R6 — Acceptance tests name the requirement they prove.** New or materially touched tests that prove
a feature acceptance item include an exact `FS-nn.Ak` comment. Specs point back to load-bearing
tests/code; behavior/architecture commits carry relevant IDs in the subject or `Spec:` trailer.

**R7 — Spec lint enforces mechanics, review enforces truth.** Automated checks validate filenames,
headers/status, local R/A uniqueness, index parity, planned/current consistency, relative links,
citations, conflict markers, and tool-wrapper artifacts. They do not infer semantic completeness.

**R8 — CI repeats shared checks from a clean clone.** Pushes to `main` and pull requests run spec
lint, both Go variants, `go vet`, UI install/tests/build. CI uses read-only repository permissions
and cancels superseded runs; it does not rewrite embedded tracked output.

**R9 — Tests isolate user state and external providers.** Tests use temporary
`CHUCK_HOME`, deterministic fake ACP peers, in-process HTTP handlers, and fixtures. Credentialed
real-CLI acceptance is an explicit manual gate and never silently substitutes for automated tests.

**R10 — retired 2026-07-15:** This single-binary delivery assumption is superseded for the planned
macOS release by R13–R21. Source-built Chuck remains a single Go binary.

**R12 — Source installs pin the official Claude adapter.** When `INSTALL_ACP=1`,
`install.sh` installs the exact reviewed `@agentclientprotocol/claude-agent-acp` version and checks
for its Node 22 runtime floor. Ordinary source builds require Node 20.19 or newer for the UI
toolchain and do not mutate global adapter installations unless explicitly requested.

**R13** — The release build has two supported delivery forms with separate contracts:
source builds follow `go.mod` and the Node 20.19-or-newer UI/CI baseline, while the GitHub Releases
MVP targets only `darwin/arm64` and ships a private Node 22-or-newer runtime. Every release binary is
built with `sqlite_fts5`; an untagged binary is never packaged as a release runtime.

**R14** — Release assembly is deterministic from a versioned packaging manifest and
lockfile that pin the Node distribution, `@agentclientprotocol/claude-agent-acp`,
`@agentclientprotocol/codex-acp`, `@openai/codex`, and their runtime dependency closure. The release
job verifies those pinned inputs before it creates an archive; an installer never runs npm, resolves a
package range, builds the UI, or compiles Go on a recipient's Mac. A version-locked patch may amend
one pinned adapter only when its filename identifies that exact package version, clean-install
application is fail-closed, the required output is checked before packaging, and the manifest's
component version carries a Chuck patch suffix rather than presenting upstream bytes.

**R15** — A release archive contains only this versioned layout:

```text
chuck-<version>-darwin-arm64/
  bin/chuck                     # wrapper
  libexec/chuck                 # FTS5 Go binary
  runtime/node/bin/node
  runtime/node_modules/@agentclientprotocol/{claude-agent-acp,codex-acp}/dist/index.js
  runtime/node_modules/@anthropic-ai/claude-agent-sdk-darwin-arm64/claude     # bundled Claude
  runtime/node_modules/@openai/codex-darwin-arm64/vendor/aarch64-apple-darwin/bin/codex
  runtime/                      # pinned adapter dependency closure
  manifest.json                 # version, target, component versions and archive identity
```

The wrapper exports `CHUCK_RUNTIME_ROOT=<version>/runtime` and executes `libexec/chuck`.
It changes neither PATH nor any provider executable override (R30). Managed adapters launch as the
private `node` running their entrypoint by absolute path, never a globally installed Node or ACP
adapter. Source builds have no managed root and keep their existing PATH behavior.

**R16** — The installer places immutable version directories below
`~/Library/Application Support/Chuck/versions/`, keeps the selected version through a `current`
pointer, and exposes one stable user command shim. That application root is distinct from
`CHUCK_HOME`; release assembly, install, update, rollback, and uninstall must never write user
configuration, state, transcripts, or credentials there.

**R17** — A GitHub Release publishes the archive, a SHA-256 checksum, and a small
machine-readable manifest naming the exact version, `darwin-arm64` target, archive filename, size,
and checksum. The installer and updater download to a same-filesystem staging directory, verify the
checksum and internal manifest/layout before activation, then atomically install the version and
switch `current`. No partial directory is reachable through the stable command.

**R18** — Release activation retains the immediately preceding verified version as
`previous`. `chuck update --rollback` atomically restores that version. A failed update, failed
rollback, or an installer interrupted before activation leaves the old `current` pointer intact;
activation never signals or replaces a running dashboard process.

**R19** — `chuck update` is the only update mechanism. It obtains release metadata
only when explicitly invoked, supports check-only/non-interactive confirmation behavior from
FS-10.R7, and performs no background check, download, telemetry, or update. Concurrent installer or
update invocations serialize around one install root; a contender exits without changing it.

**R20** — Guided authentication is implemented as a CLI delegation boundary, not an
installer credential protocol. `chuck auth claude|codex` resolves the selected private adapter
and its compatible provider login path, attaches it to the caller's terminal, and returns a bounded
success/cancel/failure result. It accepts no credential value flags, writes no credential material to
the application runtime, and does not log child stdout/stderr except sanitized actionable failure
detail. Interactive install may invoke this command; non-interactive install never does.

**R21** — Release CI verifies archive contents, FTS5 tagging, pinned component versions,
private-wrapper resolution, checksum rejection, fresh-home installation, explicit update/rollback,
no-start/non-interactive behavior, and preservation of a pre-existing `CHUCK_HOME`. It runs the
automated portion on a macOS arm64 runner or equivalent arm64 macOS environment. Credentialed Claude
and Codex login/chat checks remain manual gates and cannot be represented as release CI success.

**R22** — The release runtime declares and lockfiles the exact direct `@openai/codex`
dependency whose platform binary is the bundled Codex, and validates both bundled native providers
(running each `--version` without Node) alongside both ACP adapters before packaging. The manifest
records the bundled `claude` and `codex` versions. The lockfile and assembled runtime resolve exactly
one `@openai/codex` package at that direct version; assembly rejects a nested second copy. Since
2026-10-04 the wrapper no longer exports a default `CODEX_PATH` or `AGENTDECK_CODEX_VERSION`: the
bundle is used only when a backend explicitly selects it (R30, TS-04.R75). Source and release
command-tree tests prove `chuck auth claude|codex` is present; release tests prove an explicit
Bundle choice resolves both providers under the published root without a global install. Existing
installed release directories remain immutable: a command absent from an older version requires an
explicit reinstall/update to a newer release.

**R23 (planned) — The action client is the exact running Chuck binary.** Source and release
launches resolve `os.Executable()` to an absolute path and inject that immutable/current-version
path for chat actions; they do not depend on `PATH`, a global install, or a second artifact. Source,
archive, installed-version, retained-rollback, and update tests invoke representative action help and
transport behavior through that path. Release remains blocked until pinned credentialed Claude,
Codex, OpenCode, and OpenHands sessions pass FS-17.A9; fake providers cannot satisfy that gate.
Implementation is also blocked until the exact packaged Codex and ACP adapter prove FS-17.R20 under
the default sandbox. Enabling broad network access or substituting a filesystem transport does not
pass this build gate.

**R24 — The rename is one cut through build, release, and distribution identity.** The Go
module follows the renamed Chuck repository under the existing GitHub owner, with every internal
import path and version `-ldflags -X` target tracking it; the command directory becomes `cmd/chuck`;
the built binary, the wrapper shim at `bin/chuck`, the FTS5 binary at `libexec/chuck`, the
manifest's required layout and its component key, the archive name
`chuck-<version>-<target>.tar.gz`, the staging and versioned release directory names, the
assembled runtime's package name, the install tree default
`~/Library/Application Support/Chuck`, the installer's `CHUCK_*` variables, and the release
workflow's smoke test all rename together. There is no transitional release that publishes both
names and no compatibility alias, because FS-10.R25 moves the sole existing install through a fresh
installer run rather than through an in-place update, so no already-installed client has to parse a
manifest or resolve an archive under the new name. The release repository constant moves to the
renamed GitHub repository; renaming that repository is an operational step, and this specification
deliberately does not rely on GitHub redirecting REST API calls or release-asset downloads, because
GitHub documents redirects only for web links and git clone/fetch/push. Verification is a
repository-wide assertion that no build, release, or packaging artifact still spells the old name,
run in CI alongside the existing archive-content checks (INV §10).

**R26 `(planned)` — The Codex ACP capability bump is reproducible and provider-verified.** Release
inputs pin `@agentclientprotocol/codex-acp` exactly at `1.12.0` and direct `@openai/codex` exactly at
`0.154.0`; lockfile, assembly constants, manifest expectations, wrapper/archive fixtures and release
documentation move together. Assembly still proves one installed Codex package, hashes the reviewed
unpatched adapter bundle, applies the version-named no-consumption steering patch with zero fuzz,
hashes the complete patched output, and reports the component as `1.12.0+chuck.1`. A source
comparison test proves the patch is still semantically required instead of assuming its old offset.

Automated contract fixtures exercise canonical and absent capability advertisements; root/nested
reasoning; native child lifecycle and legacy fallback; background task reconstruction, targeted stop
and provider restart; fork success, pagination, cyclic/oversized cursor refusal and compensating
delete; canonical tool names; matching/stale/truncated file reports; and MCP elicitation completion.
The fake derives closed request/update shapes from the reviewed 1.12.0 protocol, not Chuck's
mapper. Before release, one credentialed Codex receipt covers chat, model/effort/fast application,
MCP, steer idle fallback, thought delivery, one native subagent, one background command and targeted
stop, clone/fork with multi-page history, canonical tool name, file report, stop/resume and load. It
records the exact adapter/CLI versions and cannot be replaced by fake-ACP success (INV §12/§17).

**R27 — Remote control pins its Tailscale dependency and toolchain.** `tailscale.com` is
required at one exact version, and `go.mod`'s `go` directive rises to that module's minimum (≥ 1.26.6
as of v1.102.5); CI, release, and documented source toolchains move together (CI and release read
`go-version-file: go.mod`; an older local toolchain switches through `GOTOOLCHAIN=auto`). The
release binary's growth is accepted: the stripped `sqlite_fts5` build measured 15.9 MB before and
39.8 MB after (+22.8 MiB, 2026-09-28, with `webpush-go` v1.4.0 and `rsc.io/qr` v0.2.0 also
pinned). A bump re-verifies the TS-13 evidence surface —
`ListenTLS` prerequisites, `WhoIs` fields, `StatusWithoutPeers` auth URL, and `Close` behavior —
before landing. Automated tests exercise the remote chain through a fake listener and fake `WhoIs`
and never contact a real tailnet or push service; FS-20's manual gates own those.

**R28 — retired 2026-10-03:** Dependency-only bundle policy replaced by explicit choice in R30.
**R29 — retired 2026-10-03:** Broad compatibility gate replaced by finite verification in R31.

**R30 — One current managed bundle per provider, one shared integration stack.**
Preserve the immutable Node/ACP/SDK closure, verified manifest and Codex steering patch. Expose the
release's existing Claude native dependency and directly pinned Codex as the single supported
Bundle choice for each provider; identify deterministic entrypoints/versions in the verified layout.
Reuse them rather than adding downloads, duplicate fallback packages, alternate adapter versions,
or a bundle registry. Old release directories stay immutable for application rollback only.

Replace R15/R22's implicit selection: the wrapper publishes its managed root but never overwrites
user executable overrides, exports a packaged version as catalog authority or shadows user PATH
with private provider bins. Launch adapters through absolute private Node/entrypoint paths. For an
explicit Bundle selection, resolve the existing platform package's native provider executable
(including Codex's vendor binary), including auth/Claude terminal, without requiring global Node; Installed npm CLIs
retain their user environment/Node. Reuse the existing launch abstractions rather than a second
driver. Both modes use the same managed adapter and patch set. Source builds use their documented
adapter installation and report Bundle unavailable without a managed release root.

Keep deterministic release checks, replacing assumptions of automatic private-provider execution
with explicit-mode marker tests. R20's auth ownership follows FS-10.R21/TS-04.R71; R21 includes
missing Installed and explicit Bundle setup; R26's patch/capability gates remain. Update README,
installer guidance and operator knowledge for mode/update ownership and non-guaranteed session
downgrades. No independent runtime updater, background compatibility service or package stripping.

**R31 `(planned)` — Compatibility work has a finite scope, not a growing release matrix.**
For this change, at most four real-provider combinations: Claude and Codex, each using the current
bundle and one current stable Installed CLI, with its adapter/SDK/Node fixed. Record versions,
platform, auth mode and outcomes; reuse identical existing receipts. If versions coincide, do not
download a historical version just to manufacture a difference; marker tests still prove selection.
The smoke journey covers fresh chat, native resume, explicit model/effort application, one tool
approval/denial and cancellation, plus Steer where effectively supported. Exercise an existing
role/skill and MCP action in that same journey, not separate feature suites. Other existing feature
contracts retain their focused tests and independently owed gates; this change adds no real-version
matrix for fast/clone/native children/background work/file reports beyond their integration audit
and relevant regression fixtures under TS-04.R77.

Automated tests cover the shared resolver's two modes, override preservation, bundle absence,
auth/profile parity, update transitions and marker coverage of all process-start callers. Keep
read-only GET, bounded probe, refresh/save race, metadata-free persistence and cache-inequality
fixtures. Reuse shared capability tests for supported/unsupported/unknown and failure-preserves-
input; include Steer's misleading-adapter-advertisement regression. No per-version UI branches or
duplicated full lifecycle suites for each mode. Browser acceptance is two combined journeys in one
supported skin: (1) Installed update/refresh/model selection, (2) missing/incompatible Installed →
Settings Bundle save → explicit retry → return to Installed with overrides intact. Shared component
tests and the normal build closure cover other skins; do not multiply these journeys by provider
versions, skins, models or account types.

Run normal R5 closure once after the final relevant edit. Real-provider work requires normal
authorization/credentials and must not rewrite the user's global CLI. Owed receipts remain owed,
not passes. Later adapter/provider dependency bumps rerun only the affected provider's two-point
smoke plus existing patch tests; unrelated releases do not repeat it. A reproduced regression adds
one focused fixture, not all intervening versions. There is no recurring certification duty for
user CLI releases, historical support matrix, speculative minimum/maximum allowlist or emulation
layer. If the audit reveals a genuine incompatibility needing a larger framework, stop and report
that specific blocker for a scope decision; do not expand this unit or silently waive correctness.

**R32 `(planned)` — The 2026-10 bundle refresh is reproducible and provider-verified.** Release
inputs pin `@agentclientprotocol/claude-agent-acp` exactly at `0.85.1`,
`@agentclientprotocol/codex-acp` exactly at `2.1.1` and direct `@openai/codex` exactly at `0.159.3`
(TS-04.R79); `go.mod` pins `github.com/modelcontextprotocol/go-sdk` at `v1.8.0` (TS-04.R83).
Lockfile, assembly constants, `install.sh`, manifest expectations, wrapper/archive fixtures and
release documentation move together. R26's assembly proofs carry forward against 2.1.1: one
installed Codex package whose version satisfies the adapter's declared range, a hashed unpatched
adapter bundle, the version-named steering patch applied with zero fuzz, a hashed patched output,
the `2.1.1+chuck.1` component report and the source comparison proving the patch is still needed.
The fake ACP runtime adds the reviewed 2.1.1 AIR tool-call shapes, Claude notices and the Claude
steer-backgrounds-tool marker from the inspected dists, not Chuck's mapper. Real-provider evidence
reruns R31's two-point smoke for both providers, adding one notice (where a provider emits one),
one steer during a running command and one refused model switch where the account's policy allows
staging it; missing receipts stay owed.

**R33** (planned) — Think Tank closure follows R5 once after the final relevant edit, with TS-14.R18/
§4's focused state, runtime, MCP, REST/SSE, race and UI matrix. Independent provider-frame, wire
fixture and durable-row assertions prove ownership, publication, read checkpoints, privacy and
failure behavior. Render the specified desktop journeys in every appearance. A bounded
credentialed Claude and Codex smoke covers room-tool read/submit, an ordinary approval/denial,
private Send/Steer, native resume and end-only judge; record versions/results and keep missing
receipts explicitly owed. Fake ACP and documentation checks cannot satisfy those gates. Design
closure runs only spec lint, applicable twin-skill comparisons and diff checks.

## 3. Interfaces & data shapes

The canonical commands are:

```sh
make check-specs
make test
cd ui && npm test && npm run build
make dist
```

Phone UI work checks the rendered view, which jsdom cannot see (INV §13):
`cd ui && node scripts/phone-render.mjs [transcript.json] [out.png]` serves the phone entry through
Vite with every `/api` call and the event stream stubbed, opens an agent conversation at iPhone 13
size in Playwright Chromium, and writes a full-page screenshot. It needs no Chuck server or paired
device and is a development aid, not part of closure.

The exact required checks for work/review roles are defined by
[`../../features/AGENT-WORKFLOW.md`](../../features/AGENT-WORKFLOW.md); this spec owns what each
shared target guarantees.

## 4. Invariants

- **INV §6:** build/capability claims cover every runtime variant advertised.
- **INV §7:** both FTS5 and fallback readers are tested.
- **R11 — Generated output has one source.** A generated file is updated only through its generator,
  and CI/tests detect stale or hand-edited outputs where practical.

## 5. Deviations & open decisions

- Credentialed Claude, Codex, OpenCode, and OpenHands acceptance remains manual/gated. Specs label
  affected claims rather than treating fake-provider success as real-provider certification.
- Release/install documentation has historically drifted from actual optional adapter and shell-tool
  prerequisites; README, source-install, and release-installer changes must now be reviewed against
  R12–R22.
- The macOS MVP deliberately has no signing, notarization, Homebrew formula, Intel build, Windows or
  Linux archive. TS-05.R12 records the resulting delivery-trust limitation rather than implying a
  publisher-authentication guarantee.

## 6. Traceability

- Source toolchains/targets and the optional Claude adapter: `go.mod`, `ui/package.json`, `Makefile`,
  `install.sh`.
- Release assembly/installer/update: `scripts/release/`, `internal/release/`, `internal/cli/`,
  `.github/workflows/release.yml`, `internal/cli/{installer,release,update,auth}_test.go` (FS-10).
- Symlink-free npm command packaging preserves package-relative module resolution:
  `TestCreateArchiveKeepsSymlinkedCommandTargetContext`.
- Private Codex CLI pin (R22): `scripts/release/package.json` + lockfile declare `@openai/codex`
  directly, `scripts/release/assemble.sh` validates the single resolved package and executable before packaging, and
  `requiredLayout`/`verifyInternalManifest` in `internal/release/manifest.go` enforce it; the wrapper
  supplies the same executable to the adapter through its documented `CODEX_PATH` override. Proven
  by `TestPrivateCodexResolvesWithoutGlobalInstall`, `TestRequiredLayoutAndManifestComponentsAgree`,
  and `TestAuthCommandIsPresentForEveryProvider`.
- Spec lint: `scripts/check-specs.sh`.
- Role-launcher contract: `scripts/check-launcher-contract.sh`, with
  `scripts/check-launcher-contract-test.sh` proving each guarded rule fails on a launcher or
  workflow copy that states its opposite. Both run under `make check-specs`.
- CI: `.github/workflows/ci.yml`.
- Fake integration peer: `internal/runtime/testdata/fakeacp`, server integration tests. Its
  `stress_stream` scenario (`FAKEACP_STRESS_CHUNKS`, `FAKEACP_STRESS_CHUNK_BYTES`,
  `FAKEACP_STRESS_DELAY_MS`) drives `scripts/stress-fixture`, the manual multi-tab fixture that
  reproduces browser connection-pool starvation on a same-origin dashboard; it is run by hand
  (`go run ./scripts/stress-fixture`) and has no `make` target because it needs a real browser.
- Generated UI guard: `.claude/hooks/guard-edit.sh`; twin-skill/spec feedback in
  `.claude/hooks/post-edit.sh`.
