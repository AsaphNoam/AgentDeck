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

- **R15 — The installed product is Deckhand.** (planned) The release installs the `deckhand`
  command into a Deckhand-named install tree (`~/Library/Application Support/Deckhand` by default,
  `$DECKHAND_APP_ROOT` to override), publishes `deckhand-<version>-<target>.tar.gz`, and names its
  manifest component `deckhand`. `agentdeck` is not installed, aliased, or kept on PATH; a person
  who typed it gets their shell's ordinary command-not-found. Everything R1–R14 promises about
  fresh install, private runtime, provider sign-in, explicit update and rollback holds unchanged
  under the new name.
- **R16 — An existing AgentDeck install moves over by running the Deckhand installer once.**
  (planned) There is no in-place `agentdeck update` path onto Deckhand: the update command in an
  installed AgentDeck resolves releases from the pre-rename GitHub repository, and this
  specification does not depend on that repository redirecting. The Deckhand installer is a normal
  fresh install (R2) that additionally detects an AgentDeck install tree and reports that it found
  one. It never modifies, moves, or deletes that tree — the previous install stays runnable as a
  fallback — and documentation gives the exact command to remove it once the person is satisfied.
- **R17 — First start migrates the state directory once.** (planned) When `deckhand` starts and
  `$DECKHAND_HOME` (default `~/.deckhand`) does not exist while an AgentDeck home does
  (`$AGENTDECK_HOME` if set, else `~/.agentdeck`), it moves that directory to the Deckhand home and
  reports the source path, the destination path, and that the move happened. Everything inside
  comes across unchanged and keeps working: the SQLite state database with its agents, sessions,
  tasks, pipelines and context links; transcripts; `backends.json`, `config.json`,
  `config-sources.json` and `layout.json`; project resources; and owned worktrees. `AGENTDECK_HOME`
  is read for this one purpose and has no other effect. Once a Deckhand home exists the check does
  not run again.
- **R18 — Migration refuses rather than guesses.** (planned) It does not run, and start proceeds
  against the Deckhand home alone while saying why, when: both homes already exist (it never merges
  two states, and names which one it is using); a dashboard is running against either home; the
  AgentDeck home is unreadable, is not a directory, or is not the owner's; or the destination cannot
  be created. A refusal is reported with the path and a retryable action, never swallowed, and never
  leaves the person guessing which state they are running on.
- **R19 — The environment is renamed with the product.** (planned) Every variable the product
  defines or injects is `DECKHAND_*` — `DECKHAND_HOME`, `DECKHAND_APP_ROOT`, `DECKHAND_HOOK_URL`,
  `DECKHAND_HOOK_TOKEN`, `DECKHAND_AGENT_ID`, `DECKHAND_INTERFACE`, `DECKHAND_SKILL_DIR`,
  `DECKHAND_PROJECT_RESOURCES`, `DECKHAND_LOG_LEVEL`, `DECKHAND_CODEX_VERSION`, the provider
  login-command overrides, and the installer's own variables. No `AGENTDECK_*` variable is honored
  except `AGENTDECK_HOME` under R17.

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

- **A7** (R15, R19) — (planned) A fresh install on a clean macOS arm64 home produces a runnable
  `deckhand --version` and dashboard, installs nothing named `agentdeck` on PATH or in the install
  tree, and the launched agent environment contains only `DECKHAND_*` variables. *Verified:*
  fresh-home installer integration test extended to assert the absent old command, plus a
  launch-environment test asserting no `AGENTDECK_` prefix is injected.
- **A8** (R17, R18) — (planned) A populated `~/.agentdeck` containing agents, transcripts,
  config files, project resources and a worktree becomes `~/.deckhand` on first start with every
  one of those readable afterward and the dashboard serving the same agents; a home that already
  exists at both paths, a running dashboard, and an unreadable source each refuse with a named path
  and leave both directories untouched. *Verified:* state-migration integration tests covering the
  success path and each refusal branch.
- **A9** (R16) — (planned) Release documentation states that moving from AgentDeck is a one-time
  installer run rather than `agentdeck update`, that the previous install tree is left in place, and
  gives the exact command to remove it. *Verified:* release-documentation review against this
  specification.

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
- Release transaction/layout: `internal/release/`; bootstrap and assembly:
  `scripts/release/{install,assemble}.sh`; command UX: `internal/cli/{release,update,auth}.go`.
- Regression coverage: `internal/release/{archive,install,wrapper}_test.go`; release CLI and
  fresh-home bootstrap coverage: `internal/cli/{release,update,auth,installer}_test.go`; release
  publication: `.github/workflows/release.yml`; product documentation: `README.md`.
- Rename identity, install tree, and state migration (R15–R19): TS-06.R24, TS-02.R32–R33.
