#!/bin/sh
# Mutation evidence for the Review findings fix-model contract (INV §17).

set -u

ROOT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
checker="$ROOT/scripts/check-specs.sh"
errors=0

fail() {
  printf 'review finding contract test: %s\n' "$*" >&2
  errors=$((errors + 1))
}

work=$(mktemp -d) || exit 1
trap 'rm -rf "$work"' EXIT

write_handoff() {
  file=$1
  shift
  mkdir -p "$(dirname "$file")"
  {
    printf '# Fixture\n\n## Review findings\n\n'
    printf '%s\n' "$@"
  } > "$file"
}

expect_pass() {
  name=$1
  file=$2
  if ! output=$("$checker" --file "$file" 2>&1); then
    fail "$name: valid finding failed: $output"
  fi
}

expect_failure() {
  name=$1
  expected=$2
  file=$3
  if output=$("$checker" --file "$file" 2>&1); then
    fail "$name: invalid finding passed"
    return
  fi
  case "$output" in
    *"$expected"*) ;;
    *) fail "$name: expected '$expected'; got: $output" ;;
  esac
}

file="$work/valid/HANDOFF.md"
write_handoff "$file" \
  '- **Must fix** — Localized. **Fix model:** trivial/easy — Claude Sonnet or Codex Luna.' \
  '- **Worth fixing** — Coordinated. **Fix model:** medium — Codex Terra or Claude Opus.' \
  '- **Must fix** — Architectural. **Fix model:** difficult — Codex Sol.'
expect_pass 'valid mappings' "$file"

file="$work/missing/HANDOFF.md"
write_handoff "$file" '- **Must fix** — Missing recommendation.'
expect_failure 'missing recommendation' 'exactly one **Fix model:** recommendation' "$file"

file="$work/mismatch/HANDOFF.md"
write_handoff "$file" \
  '- **Must fix** — Wrong mapping. **Fix model:** trivial/easy — Codex Sol.'
expect_failure 'invalid mapping' 'invalid fix-model band or model mapping' "$file"

file="$work/duplicate/HANDOFF.md"
write_handoff "$file" \
  '- **Must fix** — Duplicate. **Fix model:** medium — Codex Terra or Claude Opus. **Fix model:** difficult — Codex Sol.'
expect_failure 'duplicate recommendation' 'exactly one **Fix model:** recommendation' "$file"

[ "$errors" -eq 0 ] || exit 1
