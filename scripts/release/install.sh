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
#   ./install.sh [--version X.Y.Z] [--no-start] [--non-interactive]
#
# Environment overrides:
#   AGENTDECK_VERSION   pin a version (same as --version)
#   AGENTDECK_REPO      GitHub owner/repo (default: AsaphNoam/AgentDeck)
#   AGENTDECK_APP_ROOT  application root (default: ~/Library/Application Support/AgentDeck)

set -euo pipefail

REPO="${AGENTDECK_REPO:-AsaphNoam/AgentDeck}"
VERSION="${AGENTDECK_VERSION:-}"
TARGET="darwin-arm64"
NO_START=0
NONINTERACTIVE=0

die() { echo "error: $*" >&2; exit 1; }

on_path() {
  case ":${PATH}:" in *":${1}:"*) return 0 ;; *) return 1 ;; esac
}

confirm() {
  prompt="$1"
  printf '%s [y/N] ' "$prompt"
  IFS= read -r answer || return 1
  case "$answer" in y|Y|yes|YES|Yes) return 0 ;; *) return 1 ;; esac
}

# append_path_entry adds exactly one AgentDeck-owned PATH entry. The installer
# only calls it after an interactive confirmation, never in CI or a piped run
# (FS-10.R12). The root is emitted in a double-quoted shell string with the
# special characters escaped first, so a custom AGENTDECK_APP_ROOT cannot alter
# the shell profile syntax.
append_path_entry() {
  profile="${ZDOTDIR:-$HOME}/.zshrc"
  marker="# Added by AgentDeck installer"
  if [ -f "$profile" ] && grep -Fqx "$marker" "$profile"; then
    echo "==> AgentDeck PATH entry is already in ${profile}"
    return 0
  fi
  parent="$(dirname "$profile")"
  [ -d "$parent" ] || mkdir -p "$parent" || return 1
  escaped_root="$(printf '%s' "$app_root/bin" | sed 's/[\\\\$`\"]/\\\\&/g')"
  {
    printf '\n%s\n' "$marker"
    printf 'export PATH="%s:$PATH"\n' "$escaped_root"
  } >>"$profile" || return 1
  echo "==> Added ${app_root}/bin to PATH in ${profile}. Open a new terminal to use 'agentdeck'."
}

# --- Parse arguments -------------------------------------------------------
while [ "$#" -gt 0 ]; do
  case "$1" in
    --version) VERSION="${2:-}"; shift 2 ;;
    --version=*) VERSION="${1#*=}"; shift ;;
    --no-start) NO_START=1; shift ;;
    --non-interactive) NONINTERACTIVE=1; shift ;;
    -h|--help) sed -n '2,34p' "$0"; exit 0 ;;
    *) die "unknown argument: $1 (try --help)" ;;
  esac
done

# An install is interactive only with a real terminal and no override. A
# non-interactive install never prompts, never edits a shell profile, and never
# starts the dashboard (FS-10.R6, FS-10.R12).
INTERACTIVE=1
if [ ! -t 0 ] || [ "$NONINTERACTIVE" = "1" ]; then
  INTERACTIVE=0
fi

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
# The manifest may be compact one-line JSON or pretty-printed JSON. Extract the
# named field rather than relying on its position among other quoted fields.
want="$(sed -n 's/.*"sha256"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "${staging}/manifest.json" | head -1)"
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

if ! on_path "${app_root}/bin"; then
  echo
  echo "note: ${app_root}/bin is not on your PATH."
  if [ "$INTERACTIVE" = "1" ] && confirm "Add the AgentDeck command directory to ${ZDOTDIR:-$HOME}/.zshrc?"; then
    append_path_entry || echo "could not update your zsh profile; use the absolute command path above."
  else
    echo "Use the absolute command path above, or add it to your shell profile later."
  fi
fi

if [ "$INTERACTIVE" = "1" ]; then
  if "$shim" auth claude --check; then
    : # already signed in
  elif confirm "Sign in to Claude now?"; then
    if ! "$shim" auth claude; then
      echo "Claude sign-in did not complete. Installation succeeded; retry with: \"${shim}\" auth claude"
    fi
  fi
fi

if [ "$NO_START" = "1" ] || [ "$INTERACTIVE" != "1" ]; then
  echo
  echo "Start AgentDeck when ready: \"${shim}\" dashboard start --detach"
  exit 0
fi

echo
echo "==> Starting the dashboard"
if "$shim" dashboard start --detach; then
  if "$shim" dashboard open; then
    echo "Dashboard is running and opening in your browser."
  else
    echo "Dashboard is running, but the browser could not be opened. Open http://127.0.0.1:4317/ yourself."
  fi
else
  home="${AGENTDECK_HOME:-$HOME/.agentdeck}"
  echo "Installation succeeded, but the dashboard did not start."
  echo "Retry with: \"${shim}\" dashboard start --detach"
  echo "Log: ${home}/dashboard.log"
fi
