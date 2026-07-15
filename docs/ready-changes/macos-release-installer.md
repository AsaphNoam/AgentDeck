# macOS release installer

**State:** Waiting to start
**Why:** The regular AgentDeck installer idea from `docs/ideas.md`, confirmed for a friends-only
Apple-silicon MVP.
**Relevant requirements:** FS-10.R1–R14, TS-05.R12, TS-06.R13–R21, INV §9, INV §10

## Outcome

Someone can install AgentDeck from GitHub Releases on a macOS arm64 Mac, sign in to Claude or Codex
when ready, start the dashboard, and explicitly update or roll back without a repository, Go, Node,
npm, or global ACP adapters.

## Included work

Create the versioned private runtime, GitHub Release archive/checksum/manifest, installer, stable
command shim, guided authentication commands, explicit update/check/rollback commands, release
documentation, and automated release-installer coverage. Preserve user state and keep source builds
working. Do not add Homebrew, signing, notarization, other platforms/architectures, auto-updates, or
launch-at-login.

## How we will know it works

FS-10.A1–A6 and TS-06.R21: fresh macOS-arm64 install without global Node/adapters; private adapter
resolution; truthful authentication branches; atomic update/rollback and corrupt-download recovery;
interactive/no-start behavior; release/Gatekeeper documentation review. Credentialed provider runs
remain manual gates.

## Waiting on

Nothing. The implementation should start from the confirmed macOS arm64 scope and release trust
limitation above.
