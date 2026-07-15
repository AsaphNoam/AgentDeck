# FS-10 — macOS installation, setup & updates

**Status:** Partial
**Code:** `(planned) scripts/release/`, `internal/cli/`, `.github/workflows/`, `README.md` · **Journeys:** J1, J2
**Absorbed:** The regular AgentDeck installer idea from `docs/ideas.md`.

## 1. Purpose

AgentDeck's MVP release path lets a friend install and run AgentDeck on an Apple-silicon Mac without
cloning this repository, compiling Go, installing Node/npm, or globally installing ACP adapters. It
also makes the first provider sign-in and later upgrades clear without taking ownership of the
person's provider credentials or AgentDeck configuration.

## 2. Behavior

- **R1** `(planned)` — The MVP release installer supports **macOS arm64 only**. It detects another
  operating system or architecture before downloading or changing an installation and explains that
  only an Apple-silicon Mac is currently supported. It requires only standard macOS command-line
  tools documented by the installer; a source checkout, Go, Node, npm, Homebrew, and administrator
  privileges are not prerequisites.
- **R2** `(planned)` — A documented GitHub Releases installer installs a selected release, or the
  current release when no version is selected. It clearly reports the installed version and the
  command that starts AgentDeck. Re-running it for the same version is safe and does not duplicate
  the installation or overwrite AgentDeck user state.
- **R3** `(planned)` — The release contains a self-contained private runtime: the AgentDeck binary,
  a compatible Node runtime, and the reviewed official Claude and Codex ACP adapter packages. The
  `agentdeck` command finds these private components itself; it does not require or alter global
  Node/npm packages, global `claude-agent-acp`/`codex-acp` commands, or the user's shell PATH beyond
  the one AgentDeck command shim.
- **R4** `(planned)` — Installation files live separately from AgentDeck's configuration, sessions,
  and credentials. Installing, updating, rolling back, or uninstalling the application runtime never
  overwrites `$AGENTDECK_HOME` (normally `~/.agentdeck`) or provider-owned configuration. A person
  can keep using a source build independently of the release installation.
- **R5** `(planned)` — An interactive fresh install checks the default Claude backend's readiness.
  If sign-in is needed, it offers to run the bundled provider sign-in flow in the current terminal.
  `agentdeck auth claude` and `agentdeck auth codex` provide the same guided, provider-specific flow
  later. Declining, cancelling, or failing sign-in leaves a working installation and directs the
  person to retry from the dashboard/onboarding or with the same command; it never records or prints
  credentials itself.
- **R6** `(planned)` — At the end of an interactive install, AgentDeck starts the dashboard in the
  background and opens the loopback dashboard in the default browser. `--no-start` and
  non-interactive installation suppress that action. If startup fails, the installer reports that
  installation succeeded, gives the exact start command and log location, and does not claim the
  dashboard opened.
- **R7** `(planned)` — Updates are explicit. AgentDeck never checks for, downloads, or applies an
  update in the background. `agentdeck update` reports the available release and asks before
  installing it; `--yes` permits non-interactive use, `--check` only reports availability, and
  `--rollback` explicitly returns to the immediately preceding installed release. Updating keeps the
  prior runtime usable until activation succeeds. A dashboard already running from the old release
  continues until the person explicitly restarts it.
- **R8** `(planned)` — An interrupted, corrupt, incompatible, or insufficiently verified release
  download leaves the selected current runtime and all user data intact. The installer/update command
  explains whether it failed before download, verification, unpacking, activation, provider sign-in,
  or dashboard startup, with one next action.
- **R9** `(planned)` — MVP release artifacts are distributed through GitHub Releases with published
  SHA-256 checksums. They are deliberately neither code-signed nor notarized. Documentation warns
  that macOS may require the person to approve an unidentified developer on first open; AgentDeck
  never attempts to bypass Gatekeeper or asks for an administrator password.

## 3. States & transitions

- **R10** `(planned)` — A release runtime is either absent, staged, current, previous, or retained.
  Only a fully downloaded and checksum-verified staged runtime can become current. Switching current
  records the old current runtime as previous; rollback switches only between those two known-good
  installed runtimes. Failed staging leaves the current/previous relationship unchanged.
- **R11** `(planned)` — Provider readiness is independent of application installation: `ready`,
  `sign-in needed`, `sign-in cancelled`, and `sign-in failed` are actionable outcomes, not install
  failures. The existing onboarding gate remains the authority for whether a first agent can launch
  (FS-04.R16–R24 and FS-09.R30).

## 4. Edge cases & errors

- **R12** `(planned)` — If no suitable writable command location is already on PATH, the interactive
  installer asks before adding one idempotent AgentDeck-owned PATH entry to the user's zsh startup
  file. Refusal keeps the install valid and prints the absolute command path; it does not edit a
  shell profile silently. Non-interactive installation never edits shell profiles.
- **R13** `(planned)` — If another installation/update is active, a second one exits without
  changing the selected runtime. A failed update never stops a running dashboard, deletes an older
  runtime, or makes `agentdeck` resolve to a partial directory.
- **R14** `(planned)` — A missing network connection, unavailable GitHub release, unsupported
  provider login flow, or unavailable browser is reported separately from integrity failures. The
  command gives a retryable action and retains the working installation where one exists.

## 5. Acceptance criteria

- **A1** `(planned)` — On a clean macOS arm64 home with no Go, Node, npm, or global ACP adapter on
  PATH, the documented release installer produces a runnable `agentdeck --version` and dashboard.
  *Verified:* automated fresh-home installer integration test plus manual J1 release-install run.
- **A2** `(planned)` — The installed command resolves its Node runtime and both official ACP adapter
  entry points from the selected private runtime, without changing global package locations or
  requiring them on PATH. *Verified:* release-layout/wrapper integration tests.
- **A3** `(planned)` — A fresh interactive install offers default-provider sign-in, while declined,
  cancelled, failed, and successful sign-in each leave the installer outcome truthful and route the
  person to onboarding or the running dashboard. *Verified:* fake-provider command tests and manual
  J2 credential branches; successful real-provider sign-in is credential-gated.
- **A4** `(planned)` — A successful explicit update activates the new version without modifying
  `$AGENTDECK_HOME`; a simulated download/checksum/unpack interruption preserves the previous
  command; `agentdeck update --rollback` restores it. *Verified:* installer/update integration tests.
- **A5** `(planned)` — `--no-start` and non-interactive installation neither launch a dashboard nor
  edit a shell profile; interactive installation starts and opens the dashboard only after the
  runtime activates. *Verified:* CLI integration tests and manual J1 run.
- **A6** `(planned)` — The release page and install documentation state the macOS-arm64 limit,
  checksum verification, no-signing/no-notarization choice, Gatekeeper approval possibility, provider
  sign-in requirement, and explicit update/rollback commands. *Verified:* release-documentation
  review against this specification.

## 6. Deviations & open decisions

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
- Existing dashboard lifecycle commands: `internal/cli/dashboard.go`; source installer:
  `install.sh`; product documentation: `README.md`.
