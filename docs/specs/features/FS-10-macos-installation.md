# FS-10 — macOS installation, setup & updates

**Status:** Partial
**Code:** `scripts/release/`, `internal/release/`, `internal/cli/`, `.github/workflows/release.yml`, `README.md` · **Journeys:** J1, J2
**Absorbed:** The regular AgentDeck installer idea from `docs/ideas.md`.

## 1. Purpose

AgentDeck's MVP release path lets a friend install and run AgentDeck on an Apple-silicon Mac without
cloning this repository, compiling Go, installing Node/npm, or globally installing ACP adapters. It
also makes the first provider sign-in and later upgrades clear without taking ownership of the
person's provider credentials or AgentDeck configuration.

## 2. Behavior

- **R1** — The MVP release installer supports **macOS arm64 only**. It detects another
  operating system or architecture before downloading or changing an installation and explains that
  only an Apple-silicon Mac is currently supported. It requires only standard macOS command-line
  tools documented by the installer; a source checkout, Go, Node, npm, Homebrew, and administrator
  privileges are not prerequisites.
- **R2** — A documented GitHub Releases installer installs a selected release, or the
  current release when no version is selected. It clearly reports the installed version and the
  command that starts AgentDeck. Re-running it for the same version is safe and does not duplicate
  the installation or overwrite AgentDeck user state.
- **R3** — The release contains a self-contained private runtime: the AgentDeck binary,
  a compatible Node runtime, and the reviewed official Claude and Codex ACP adapter packages. The
  `agentdeck` command finds these private components itself; it does not require or alter global
  Node/npm packages, global `claude-agent-acp`/`codex-acp` commands, or the user's shell PATH beyond
  the one AgentDeck command shim.
- **R4** — Installation files live separately from AgentDeck's configuration, sessions,
  and credentials. Installing, updating, rolling back, or uninstalling the application runtime never
  overwrites `$AGENTDECK_HOME` (normally `~/.agentdeck`) or provider-owned configuration. A person
  can keep using a source build independently of the release installation.
- **R5** — An interactive fresh install checks the default Claude backend's readiness.
  If sign-in is needed, it offers to run the bundled provider sign-in flow in the current terminal.
  `agentdeck auth claude` and `agentdeck auth codex` provide the same guided, provider-specific flow
  later. Declining, cancelling, or failing sign-in leaves a working installation and directs the
  person to retry from the dashboard/onboarding or with the same command; it never records or prints
  credentials itself.
- **R6** — At the end of an interactive install, AgentDeck starts the dashboard in the
  background and opens the loopback dashboard in the default browser. `--no-start` and
  non-interactive installation suppress that action. If startup fails, the installer reports that
  installation succeeded, gives the exact start command and log location, and does not claim the
  dashboard opened.
- **R7** — Updates are explicit. AgentDeck never checks for, downloads, or applies an
  update in the background. `agentdeck update` reports the available release and asks before
  installing it; `--yes` permits non-interactive use, `--check` only reports availability, and
  `--rollback` explicitly returns to the immediately preceding installed release. Updating keeps the
  prior runtime usable until activation succeeds. A dashboard already running from the old release
  continues until the person explicitly restarts it.
- **R8** — An interrupted, corrupt, incompatible, or insufficiently verified release
  download leaves the selected current runtime and all user data intact. The installer/update command
  explains whether it failed before download, verification, unpacking, activation, provider sign-in,
  or dashboard startup, with one next action.
- **R9** — MVP release artifacts are distributed through GitHub Releases with published
  SHA-256 checksums. They are deliberately neither code-signed nor notarized. Documentation warns
  that macOS may require the person to approve an unidentified developer on first open; AgentDeck
  never attempts to bypass Gatekeeper or asks for an administrator password.

- **R15 — The installed product is Chuck.** (planned) The release installs the `chuck`
  command into a Chuck-named install tree (`~/Library/Application Support/Chuck` by default,
  `$CHUCK_APP_ROOT` to override), publishes `chuck-<version>-<target>.tar.gz`, and names its
  manifest component `chuck`. `agentdeck` is not installed, aliased, or kept on PATH; a person
  who typed it gets their shell's ordinary command-not-found. Everything R1–R14 promises about
  fresh install, private runtime, provider sign-in, explicit update and rollback holds unchanged
  under the new name.
- **R16 — retired 2026-10-03:** Legacy-install detection replaced by supervised cutover R25.
- **R17 — retired 2026-10-03:** Automatic home migration replaced by R25–R26.
- **R18 — retired 2026-10-03:** Automatic migration refusal machinery is no longer required.
- **R19 — retired 2026-10-03:** Legacy home-variable exception removed by R26.

- **R25** `(planned)` — The sole operator moves to Chuck through one supervised cutover.
  Install Chuck normally; no legacy-install detector, migration command, startup migrator or
  `agentdeck update` bridge is shipped. Before transfer, stop dashboards and all their agent/tmux
  sessions, pause automatic work, save browser drafts, and preserve a recoverable source snapshot.
  Inventory and preserve configuration (including edited roles), agents/session history,
  transcripts, tasks/pipelines/context links, project resources and owned worktrees; starting empty
  or discarding any of these requires a separate explicit choice. Prepare the Chuck home offline,
  adapt the concrete installation's role references and relocated paths, then verify it before
  starting new sessions. Live-process continuity and automatic adoption of old tmux sessions are
  excluded; retaining history does not promise native-provider resume across the cutover.
  Release instructions identify source/destination, completion checks, recovery from the preserved
  source, phone re-pairing and optional old-install removal after success. Never run both versions
  against shared state/worktrees. This design authorizes no live transfer or deletion itself.
- **R26** `(planned)` — Chuck resolves only `$CHUCK_HOME`, default `~/.chuck`, and otherwise uses
  ordinary fresh-home startup. It does not inspect, move, merge or repair an AgentDeck home.
  Every product-defined/injected variable becomes `CHUCK_*`, including home, app root, hooks,
  agent identity, interface, skill/resources, logging, provider/login overrides and installer
  variables; no `AGENTDECK_*` input is honored. Existing data can be used only after the supervised
  preparation in R25; ordinary fresh installs require none of that preparation.

- **R20 — retired 2026-10-03:** Installed-only setup replaced by explicit-bundle alternative R23.
- **R21** `(planned)` — `agentdeck auth claude|codex` delegates to the same selected
  provider selection used by the matching backend/default model. Optional `--backend <id>` and
  `--model <id>` select a configured target; a provider/type mismatch or ambiguous backend requires
  correction rather than guessing. The command identifies its target/executable before interactive
  login, inherits that target's provider environment and writes no credentials itself. A missing CLI
  offers install guidance rather than starting login. Dashboard/onboarding still never run login;
  they show the target-specific command and Refresh provider. Fresh installation without a backend
  catalog uses ambient provider selection without creating or rewriting configuration.
- **R22 — retired 2026-10-03:** Installed-only update ownership replaced by two-source R24.
- **R23** `(planned)` — Packaged AgentDeck supplies one managed Claude provider and one managed
  Codex provider with its tested adapters/SDKs/Node, but backend selection defaults to Installed
  (FS-09.R75). Without an installed CLI, the dashboard still opens and offers official provider
  installation instructions or the backend's explicit Bundle choice; never select it automatically.
  Source builds without a managed bundle explain its unavailability. Interactive installer sign-in
  uses only the selected available executable under R21; missing provider skips login with guidance,
  not installation failure. This supersedes R5/A3's implicit bundled sign-in and qualifies R1/R3:
  a separately installed provider is not necessary if the user explicitly selects Bundle. No provider
  installer or independent bundle updater runs automatically.
- **R24** `(planned)` — Installed providers update through their own installer/package manager;
  bundled providers update only with AgentDeck releases. An application update changes its managed
  stack but preserves each backend's mode and saved overrides; it never updates the user's CLI or
  automatically reverts Installed to Bundle. A running dashboard keeps its own release's bundle;
  restarting into the new application release makes that release's bundle available for subsequent
  starts. Old immutable release directories may remain for application rollback, not a provider
  version picker. Neither changing mode nor rolling back AgentDeck migrates/downgrades native
  sessions. Rollback to a pre-policy binary restores that binary's old selection behavior and must
  be disclosed. Existing configs lacking mode adopt Installed without rewriting user state.

## 3. States & transitions

- **R10** — A release runtime is either absent, staged, current, previous, or retained.
  Only a fully downloaded and checksum-verified staged runtime can become current. Switching current
  records the old current runtime as previous; rollback switches only between those two known-good
  installed runtimes. Failed staging leaves the current/previous relationship unchanged.
- **R11** — Provider readiness is independent of application installation: `ready`,
  `sign-in needed`, `sign-in cancelled`, and `sign-in failed` are actionable outcomes, not install
  failures. The existing onboarding gate remains the authority for whether a first agent can launch
  (FS-04.R16–R24 and FS-09.R30).

## 4. Edge cases & errors

- **R12** — If no suitable writable command location is already on PATH, the interactive
  installer asks before adding one idempotent AgentDeck-owned PATH entry to the user's zsh startup
  file. Refusal keeps the install valid and prints the absolute command path; it does not edit a
  shell profile silently. Non-interactive installation never edits shell profiles.
- **R13** — If another installation/update is active, a second one exits without
  changing the selected runtime. A failed update never stops a running dashboard, deletes an older
  runtime, or makes `agentdeck` resolve to a partial directory.
- **R14** — A missing network connection, unavailable GitHub release, unsupported
  provider login flow, or unavailable browser is reported separately from integrity failures. The
  command gives a retryable action and retains the working installation where one exists.

## 5. Acceptance criteria

- **A1** — On a clean macOS arm64 home with no Go, Node, npm, or global ACP adapter on
  PATH, the documented release installer produces a runnable `agentdeck --version` and dashboard.
  *Verified:* automated fresh-home installer integration test plus manual J1 release-install run.
- **A2** — The installed command resolves its Node runtime and both official ACP adapter
  entry points from the selected private runtime, without changing global package locations or
  requiring them on PATH. *Verified:* release-layout/wrapper integration tests.
- **A3** — A fresh interactive install offers default-provider sign-in, while declined,
  cancelled, failed, and successful sign-in each leave the installer outcome truthful and route the
  person to onboarding or the running dashboard. *Verified:* fake-provider command tests and manual
  J2 credential branches; successful real-provider sign-in is credential-gated.
- **A4** — A successful explicit update activates the new version without modifying
  `$AGENTDECK_HOME`; a simulated download/checksum/unpack interruption preserves the previous
  command; `agentdeck update --rollback` restores it. *Verified:* installer/update integration tests.
- **A5** — `--no-start` and non-interactive installation neither launch a dashboard nor
  edit a shell profile, including after the installer re-executes under its operation lock;
  interactive installation starts and opens the dashboard only after the runtime activates.
  *Verified:* CLI integration tests (including a pseudo-terminal lock-re-exec test) and manual J1 run.
- **A6** — The release page and install documentation state the macOS-arm64 limit,
  checksum verification, no-signing/no-notarization choice, Gatekeeper approval possibility, provider
  sign-in requirement, and explicit update/rollback commands. *Verified:* release-documentation
  review against this specification.

- **A7** (R15, R26) — (planned) A fresh install on a clean macOS arm64 home produces a runnable
  `chuck --version` and dashboard, installs nothing named `agentdeck` on PATH or in the install
  tree, and the launched agent environment contains only `CHUCK_*` product variables. *Verified:*
  fresh-home installer integration test extended to assert the absent old command, plus a
  launch-environment test asserting no `AGENTDECK_` prefix is injected.
- **A8 — retired 2026-10-03:** General migration matrix replaced by bounded A13–A14.
- **A9 — retired 2026-10-03:** Cutover documentation is covered by A14.
- **A13** `(planned)` (R26) — With a populated old home and `AGENTDECK_HOME` set, startup uses
  only the chosen Chuck home and leaves the old home untouched; an absent Chuck home seeds normally.
  *Verify by* focused home-resolution/startup tests, not a migration failure matrix.
- **A14** `(planned)` (R25) — Rehearse the documented cutover on a disposable copy representative
  of the operator's installation. Confirm retained records/transcripts and customized roles are
  readable, resources and owned Git worktrees resolve, new FirstMate sessions launch, and the
  preserved source can still be used for recovery after Chuck is stopped. Review stop/pause,
  draft preservation, phone re-pairing and cleanup instructions. Record what was checked and any
  native-resume limitation; do not claim a real installation was migrated from this rehearsal.
  *Verify by* one supervised rehearsal receipt and release-documentation review.

- **A10** `(planned)` (R21, R23–R24) — A fresh install with no installed provider produces a working dashboard,
  truthful provider-install/explicit-Bundle guidance and no automatic native-provider spawn; with a user CLI it uses that
  CLI despite bundle copies until Bundle is selected. Install/login cancellation is non-destructive. *Verify by*
  release-layout/installer/onboarding fixtures and a focused rendered missing-provider journey.
- **A11** `(planned)` (R21) — Default, sole-provider, ambiguous, explicit backend/model and
  missing-catalog auth selection follow TS-04.R71; login/status/launch child markers and provider
  homes agree, and unsupported status remains skipped. *Verify by* CLI/readiness integration tests
  and the authorized real-provider check in FS-09.A42.
- **A12** `(planned)` (R24) — Upgrading an existing mode-less config from the bundled-default release uses a fake external
  provider on the next process; a missing provider blocks only its launch; explicit modes, overrides and
  user data survive update/rollback unchanged. Old immutable release directories are not patched.
  *Verify by* installed-version transition tests and documentation review that distinguishes
  application rollback from provider/session rollback and pre-policy selection behavior.

## 6. Deviations & open decisions

- R21/R23–R24/A10–A12 are planned successors, not shipped behavior. R5/A3's bundled sign-in remains
  the current implementation until this change lands; implementation must update the opening setup
  copy and acceptance evidence together with provider selection.

- This MVP intentionally excludes Intel macOS, Windows, Linux, Homebrew, signing, notarization,
  auto-updates, launch-at-login, global adapter installation, and automatic migration of a source
  installation. Each is a future product decision, not an implied compatibility promise.
- A GitHub Release checksum detects accidental corruption and many delivery mistakes, but because
  this MVP has no signing/notarization it does not independently prove publisher identity. The
  distribution trust boundary is explicit in TS-05.R12.

## 7. Traceability

- Existing first-run/config authority: FS-04.R14–R24; provider adapter and credential behavior:
  FS-09.R24–R30 and TS-04.R13.
- Release assembly, private runtime, installer/update transaction, and verification: TS-06.R13–R21.
- Release-file integrity, credential handling, and macOS trust boundary: TS-05.R12.
- Release transaction/layout: `internal/release/`; bootstrap and assembly:
  `scripts/release/{install,assemble}.sh`; command UX: `internal/cli/{release,update,auth}.go`.
- Regression coverage: `internal/release/{archive,install,wrapper}_test.go`; release CLI and
  fresh-home bootstrap coverage: `internal/cli/{release,update,auth,installer}_test.go`; release
  publication: `.github/workflows/release.yml`; product documentation: `README.md`.
- Rename identity and supervised cutover (R15, R25–R26): TS-06.R24, TS-02.R40–R41.
