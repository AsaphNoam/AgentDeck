#!/usr/bin/env bash
#
# install.sh — install AgentDeck from GitHub Releases on an Apple-silicon Mac.
#
# This bootstrap downloads a pre-built, self-contained release (the AgentDeck
# binary, a private Node runtime, and the official Claude/Codex ACP adapters),
# verifies its SHA-256, and hands off to the bundled binary's own verified,
# atomic install transaction (`agentdeck release install`). It never compiles Go,
# runs npm, builds the UI, or installs anything globally (FS-10.R1–R3, TS-06.R17).
#
# Requirements: macOS on Apple silicon (arm64) and the standard command-line
# tools curl, shasum, tar, and mktemp. Go, Node, npm, Homebrew, and admin rights
# are NOT required. Release archives are checksum-verified but intentionally
# unsigned and unnotarized: macOS may ask you to approve an unidentified
# developer on first open (FS-10.R9, TS-05.R12).
#
# Usage:
#   ./install.sh [--version X.Y.Z]
#
# Environment overrides:
#   AGENTDECK_VERSION   pin a version (same as --version)
#   AGENTDECK_REPO      GitHub owner/repo (default: AsaphNoam/AgentDeck)
#   AGENTDECK_APP_ROOT  application root (default: ~/Library/Application Support/AgentDeck)

set -euo pipefail

REPO="${AGENTDECK_REPO:-AsaphNoam/AgentDeck}"
VERSION="${AGENTDECK_VERSION:-}"
TARGET="darwin-arm64"

die() { echo "error: $*" >&2; exit 1; }

# --- Parse arguments -------------------------------------------------------
while [ "$#" -gt 0 ]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --version=*) VERSION="${1#*=}"; shift ;;
    -h|--help) sed -n '2,30p' "$0"; exit 0 ;;
    *) die "unknown argument: $1 (try --help)" ;;
  esac
done

# --- Platform guard (FS-10.R1) --------------------------------------------
os="$(uname -s)"
arch="$(uname -m)"
if [ "$os" != "Darwin" ] || [ "$arch" != "arm64" ]; then
  die "AgentDeck's release installer currently supports only macOS on Apple silicon (arm64).
Detected: ${os}/${arch}. Build from source instead: https://github.com/${REPO}#build-from-source"
fi

# --- Tool check ------------------------------------------------------------
for tool in curl shasum tar mktemp; do
  command -v "$tool" >/dev/null 2>&1 || die "required tool not found: ${tool}"
done

api="https://api.github.com/repos/${REPO}"
dl="https://github.com/${REPO}/releases/download"

# --- Resolve version -------------------------------------------------------
if [ -z "$VERSION" ]; then
  echo "==> Resolving latest release"
  tag="$(curl -fsSL "${api}/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d'"' -f4)" \
    || die "could not reach GitHub to resolve the latest release (check your network)"
  [ -n "$tag" ] || die "no published release found for ${REPO}"
  VERSION="${tag#v}"
fi
tag="v${VERSION}"
archive_name="agentdeck-${VERSION}-${TARGET}.tar.gz"

# --- Download to a same-run staging dir ------------------------------------
staging="$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-install.XXXXXX")"
cleanup() { rm -rf "$staging"; }
trap cleanup EXIT

echo "==> Downloading AgentDeck ${VERSION} (${TARGET})"
curl -fSL --proto '=https' "${dl}/${tag}/${archive_name}" -o "${staging}/${archive_name}" \
  || die "download failed for ${archive_name} (release ${tag} may not exist for this platform)"
curl -fsSL --proto '=https' "${dl}/${tag}/manifest.json" -o "${staging}/manifest.json" \
  || die "download failed for the release manifest of ${tag}"

# --- Verify checksum before touching anything (TS-05.R12) ------------------
want="$(grep '"sha256"' "${staging}/manifest.json" | head -1 | cut -d'"' -f4)"
[ -n "$want" ] || die "release manifest is missing a sha256 field"
got="$(shasum -a 256 "${staging}/${archive_name}" | cut -d' ' -f1)"
if [ "$want" != "$got" ]; then
  die "checksum mismatch for ${archive_name}
  expected ${want}
  got      ${got}
Refusing to install a corrupt or tampered download; your current installation is unchanged."
fi
echo "==> Checksum verified"

# --- Extract just enough to run the bundled binary -------------------------
# The bundled binary re-verifies the archive and performs the real atomic,
# staged install into the application root (INV §2: one verified transaction).
tar -xzf "${staging}/${archive_name}" -C "${staging}"
bundled="${staging}/agentdeck-${VERSION}-${TARGET}/libexec/agentdeck"
[ -x "$bundled" ] || die "release archive is missing its bundled binary (corrupt download)"

echo "==> Installing"
"$bundled" release install \
  --archive "${staging}/${archive_name}" \
  --manifest "${staging}/manifest.json"

# --- Report the stable command --------------------------------------------
app_root="${AGENTDECK_APP_ROOT:-$HOME/Library/Application Support/AgentDeck}"
shim="${app_root}/bin/agentdeck"
echo
echo "AgentDeck ${VERSION} is installed."
echo "  command: ${shim}"
echo
echo "Next:"
echo "  \"${shim}\" auth claude       # sign in to a provider when ready"
echo "  \"${shim}\" dashboard start   # start the dashboard on 127.0.0.1"
echo "  \"${shim}\" dashboard open    # open the UI in your browser"
if ! command -v agentdeck >/dev/null 2>&1; then
  echo
  echo "note: ${app_root}/bin is not on your PATH. Add it to use 'agentdeck' directly,"
  echo "      or run the absolute command path above."
fi
