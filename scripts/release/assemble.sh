#!/usr/bin/env bash
# Assemble the self-contained darwin/arm64 release runtime. This is a release
# builder tool, not the end-user installer: it requires Go, npm, and a verified
# Node distribution archive supplied by CI or the release operator.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-${1:-}}"
NODE_TARBALL="${NODE_TARBALL:-}"
NODE_VERSION="${NODE_VERSION:-22.22.0}"
# Published by nodejs.org for node-v22.22.0-darwin-arm64.tar.gz. This pin keeps
# the private runtime input reproducible; changing it is a deliberate release
# dependency update and requires refreshing the matching Node archive.
NODE_SHA256="5ed4db0fcf1eaf84d91ad12462631d73bf4576c1377e192d222e48026a902640"
CLAUDE_ACP_VERSION="0.59.0"
CODEX_ACP_VERSION="1.1.2"
TARGET="darwin-arm64"
OUT_DIR="${OUT_DIR:-$ROOT/dist/release}"

die() { echo "error: $*" >&2; exit 1; }
[ -n "$VERSION" ] || die "set VERSION or pass the release version as the first argument"
[ -n "$NODE_TARBALL" ] || die "set NODE_TARBALL to a verified Node ${NODE_VERSION} darwin-arm64 tarball"
[ -f "$NODE_TARBALL" ] || die "NODE_TARBALL does not exist: $NODE_TARBALL"
command -v go >/dev/null || die "Go is required to assemble a release"
command -v npm >/dev/null || die "npm is required to build the embedded UI"

work="$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-release.XXXXXX")"
cleanup() { rm -rf "$work"; }
trap cleanup EXIT

echo "==> Building embedded AgentDeck ${VERSION}"
make dist VERSION="$VERSION"

stage="$work/agentdeck-${VERSION}-${TARGET}"
mkdir -p "$stage/libexec" "$stage/runtime"
cp bin/agentdeck "$stage/libexec/agentdeck"

echo "==> Installing private Node ${NODE_VERSION}"
node_sum="$(shasum -a 256 "$NODE_TARBALL" | awk '{print $1}')"
[ "$node_sum" = "$NODE_SHA256" ] || die "NODE_TARBALL checksum does not match the pinned Node ${NODE_VERSION} distribution"
tar -xzf "$NODE_TARBALL" -C "$work"
node_source="$work/node-v${NODE_VERSION}-${TARGET}"
[ -x "$node_source/bin/node" ] || die "NODE_TARBALL is not the Node ${NODE_VERSION} ${TARGET} distribution"
mv "$node_source" "$stage/runtime/node"

echo "==> Installing pinned ACP adapters"
cp scripts/release/package.json scripts/release/package-lock.json "$stage/runtime/"
"$stage/runtime/node/bin/npm" ci --omit=dev --prefix "$stage/runtime"
[ -x "$stage/runtime/node_modules/.bin/claude-agent-acp" ] || die "Claude ACP adapter was not installed"
[ -x "$stage/runtime/node_modules/.bin/codex-acp" ] || die "Codex ACP adapter was not installed"

"$stage/libexec/agentdeck" release wrapper --dir "$stage"
"$stage/libexec/agentdeck" release manifest --dir "$stage" --version "$VERSION" \
  --node "$NODE_VERSION" --claude-acp "$CLAUDE_ACP_VERSION" --codex-acp "$CODEX_ACP_VERSION"

mkdir -p "$OUT_DIR"
"$stage/libexec/agentdeck" release package --dir "$stage" --output-dir "$OUT_DIR" --version "$VERSION"
echo "==> Release assets written to ${OUT_DIR}"
