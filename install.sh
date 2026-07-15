#!/usr/bin/env bash
#
# install.sh — build the AgentDeck UI + Go binary and install `agentdeck` on PATH.
#
# Steps:
#   1. Build the React/Vite UI -> ui/dist.
#   2. Copy ui/dist into the Go embed location (internal/server/ui/dist).
#   3. Build the Go binary with version ldflags.
#   4. Install the binary into an on-PATH bin dir.
#   5. Seed ~/.agentdeck on first run (the binary seeds lazily on `dashboard start`).
#
# Prereqs: Go 1.25+, Node 20+, npm. Node is build-time only unless the optional
# Claude ACP adapter is installed; that adapter requires Node 22+ at runtime.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

BINARY="agentdeck"
PKG="github.com/agentdeck/agentdeck"
VERSION_PKG="${PKG}/internal/version"
EMBED_DIR="internal/server/ui/dist"

# Pinned official ACP adapter for the Claude chat runtime (TS-04.R13). The Go
# runtime targets the ACP protocol version this adapter negotiates; bump this pin
# deliberately and re-run the gated real-provider acceptance checks (FS-09.A7).
# Install it (Node required) with: INSTALL_ACP=1 ./install.sh
CLAUDE_ACP_PKG="@agentclientprotocol/claude-agent-acp"
CLAUDE_ACP_VERSION="0.59.0"

VERSION="${VERSION:-0.1.0}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

LDFLAGS="-X ${VERSION_PKG}.Version=${VERSION} \
-X ${VERSION_PKG}.Commit=${COMMIT} \
-X ${VERSION_PKG}.Date=${DATE}"

echo "==> Checking prerequisites"
command -v go   >/dev/null 2>&1 || { echo "error: Go 1.25+ is required"; exit 1; }
command -v node >/dev/null 2>&1 || { echo "error: Node 20+ is required"; exit 1; }
command -v npm  >/dev/null 2>&1 || { echo "error: npm is required"; exit 1; }

NODE_MAJOR="$(node -p 'process.versions.node.split(".")[0]')"
if [ "${NODE_MAJOR}" -lt 20 ]; then
  echo "error: Node 20+ is required to build AgentDeck"
  exit 1
fi

# Optional: install the pinned ACP adapter so chat agents can launch. Off by
# default (the Go binary builds + the test suite passes without it); the real-CLI
# acceptance test needs it plus a logged-in Claude account.
if [ "${INSTALL_ACP:-0}" = "1" ]; then
  if [ "${NODE_MAJOR}" -lt 22 ]; then
    echo "error: ${CLAUDE_ACP_PKG}@${CLAUDE_ACP_VERSION} requires Node 22+"
    exit 1
  fi
  echo "==> Installing ${CLAUDE_ACP_PKG}@${CLAUDE_ACP_VERSION}"
  npm install -g "${CLAUDE_ACP_PKG}@${CLAUDE_ACP_VERSION}"
fi

echo "==> Building UI (ui/dist)"
( cd ui && npm ci && npm run build )

echo "==> Embedding UI into ${EMBED_DIR}"
rm -rf "${EMBED_DIR}"
mkdir -p "${EMBED_DIR}"
cp -R ui/dist/. "${EMBED_DIR}/"

echo "==> Building ${BINARY} (version ${VERSION}, commit ${COMMIT})"
mkdir -p bin
# -tags sqlite_fts5 is required: the archive search path uses FTS5 MATCH/snippet/
# bm25, which error at runtime on the untagged plain-table fallback.
go build -tags sqlite_fts5 -ldflags "${LDFLAGS}" -o "bin/${BINARY}" ./cmd/agentdeck

# Choose an install dir on PATH, preferring a user-writable location.
INSTALL_DIR="${INSTALL_DIR:-}"
if [ -z "${INSTALL_DIR}" ]; then
  if [ -d "${HOME}/.local/bin" ] || mkdir -p "${HOME}/.local/bin" 2>/dev/null; then
    INSTALL_DIR="${HOME}/.local/bin"
  else
    INSTALL_DIR="/usr/local/bin"
  fi
fi

echo "==> Installing to ${INSTALL_DIR}/${BINARY}"
mkdir -p "${INSTALL_DIR}"
if [ -w "${INSTALL_DIR}" ]; then
  install -m 0755 "bin/${BINARY}" "${INSTALL_DIR}/${BINARY}"
else
  sudo install -m 0755 "bin/${BINARY}" "${INSTALL_DIR}/${BINARY}"
fi

echo
echo "Installed ${BINARY} $("${INSTALL_DIR}/${BINARY}" --version 2>/dev/null || echo '')"
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *) echo "note: ${INSTALL_DIR} is not on your PATH; add it to use 'agentdeck' directly." ;;
esac
echo
echo "Next:"
echo "  agentdeck dashboard start    # seeds ~/.agentdeck on first run, binds 127.0.0.1:4317"
echo "  agentdeck dashboard open     # open the UI in your browser"
