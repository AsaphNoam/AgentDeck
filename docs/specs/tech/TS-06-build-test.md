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
`AGENTDECK_HOME`, deterministic fake ACP peers, in-process HTTP handlers, and fixtures. Credentialed
real-CLI acceptance is an explicit manual gate and never silently substitutes for automated tests.

**R10 — retired 2026-07-15:** This single-binary delivery assumption is superseded for the planned
macOS release by R13–R21. Source-built AgentDeck remains a single Go binary.

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
`@agentclientprotocol/codex-acp`, and their runtime dependency closure. The release job verifies
those pinned inputs before it creates an archive; an installer never runs npm, resolves a package
range, builds the UI, or compiles Go on a recipient's Mac.

**R15** — A release archive contains only this versioned layout:

```text
agentdeck-<version>-darwin-arm64/
  bin/agentdeck                 # wrapper
  libexec/agentdeck             # FTS5 Go binary
  runtime/node/bin/node
  runtime/node_modules/.bin/{claude-agent-acp,codex-acp}
  runtime/                      # pinned adapter dependency closure
  manifest.json                 # version, target, component versions and archive identity
```

The wrapper prepends only its `runtime/node/bin` and `runtime/node_modules/.bin` to the child PATH
before executing `libexec/agentdeck`, leaving the remaining user PATH available to provider tooling.
It does not use a globally installed Node or ACP adapter. Source builds retain their existing PATH
behavior.

**R16** — The installer places immutable version directories below
`~/Library/Application Support/AgentDeck/versions/`, keeps the selected version through a `current`
pointer, and exposes one stable user command shim. That application root is distinct from
`AGENTDECK_HOME`; release assembly, install, update, rollback, and uninstall must never write user
configuration, state, transcripts, or credentials there.

**R17** — A GitHub Release publishes the archive, a SHA-256 checksum, and a small
machine-readable manifest naming the exact version, `darwin-arm64` target, archive filename, size,
and checksum. The installer and updater download to a same-filesystem staging directory, verify the
checksum and internal manifest/layout before activation, then atomically install the version and
switch `current`. No partial directory is reachable through the stable command.

**R18** — Release activation retains the immediately preceding verified version as
`previous`. `agentdeck update --rollback` atomically restores that version. A failed update, failed
rollback, or an installer interrupted before activation leaves the old `current` pointer intact;
activation never signals or replaces a running dashboard process.

**R19** — `agentdeck update` is the only update mechanism. It obtains release metadata
only when explicitly invoked, supports check-only/non-interactive confirmation behavior from
FS-10.R7, and performs no background check, download, telemetry, or update. Concurrent installer or
update invocations serialize around one install root; a contender exits without changing it.

**R20** — Guided authentication is implemented as a CLI delegation boundary, not an
installer credential protocol. `agentdeck auth claude|codex` resolves the selected private adapter
and its compatible provider login path, attaches it to the caller's terminal, and returns a bounded
success/cancel/failure result. It accepts no credential value flags, writes no credential material to
the application runtime, and does not log child stdout/stderr except sanitized actionable failure
detail. Interactive install may invoke this command; non-interactive install never does.

**R21** — Release CI verifies archive contents, FTS5 tagging, pinned component versions,
private-wrapper resolution, checksum rejection, fresh-home installation, explicit update/rollback,
no-start/non-interactive behavior, and preservation of a pre-existing `AGENTDECK_HOME`. It runs the
automated portion on a macOS arm64 runner or equivalent arm64 macOS environment. Credentialed Claude
and Codex login/chat checks remain manual gates and cannot be represented as release CI success.

**R22** `(planned)` — The release runtime declares and lockfiles the exact direct `@openai/codex`
dependency required for Codex native `login status`, exposes its executable through the private
wrapper PATH, and validates that executable alongside both ACP adapters before packaging. Source and
release command-tree tests prove `agentdeck auth claude|codex` is present; release tests also prove
the private Codex readiness command resolves without a globally installed Codex CLI. Existing
installed release directories remain immutable: a command absent from an older version requires an
explicit reinstall/update to a newer release.

## 3. Interfaces & data shapes

The canonical commands are:

```sh
make check-specs
make test
cd ui && npm test && npm run build
make dist
```

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
- Spec lint: `scripts/check-specs.sh`.
- CI: `.github/workflows/ci.yml`.
- Fake integration peer: `internal/runtime/testdata/fakeacp`, server integration tests.
- Generated UI guard: `.claude/hooks/guard-edit.sh`; twin-skill/spec feedback in
  `.claude/hooks/post-edit.sh`.
