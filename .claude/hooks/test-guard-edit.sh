#!/bin/sh
set -eu

command -v jq >/dev/null 2>&1 || {
  echo "test-guard-edit.sh: jq is required" >&2
  exit 1
}

root=$(mktemp -d "${TMPDIR:-/tmp}/guard-edit-test.XXXXXX")
trap 'rm -rf "$root"' EXIT HUP INT TERM
mkdir -p "$root/docs/features"
hook=$(pwd)/.claude/hooks/guard-edit.sh

run_hook() {
  CLAUDE_PROJECT_DIR="$root" "$hook"
}

assert_empty() {
  [ -z "$1" ] || {
    echo "expected allow, got: $1" >&2
    exit 1
  }
}

assert_denied() {
  case "$1" in
    *'"permissionDecision":"deny"'*) ;;
    *)
      echo "expected deny, got: $1" >&2
      exit 1
      ;;
  esac
}

cat >"$root/docs/features/HANDOFF.md" <<'EOF'
# handoff
## Current position
small
## Active change
small
## Later
ignored
EOF

normal=$(printf '%s' '{"tool_input":{"file_path":"docs/features/HANDOFF.md","old_string":"small","new_string":"tiny"}}' | run_hook)
assert_empty "$normal"

briefs=$(printf '%s' '{"tool_input":{"file_path":"docs/features/BRIEFS.md","content":"replacement"}}' | run_hook)
assert_empty "$briefs"

generated_relative=$(printf '%s' '{"tool_input":{"file_path":"internal/server/ui/dist/generated.js","content":"x"}}' | run_hook)
assert_denied "$generated_relative"
generated_absolute=$(printf '%s' "{\"tool_input\":{\"file_path\":\"$root/internal/server/ui/dist/generated.js\",\"content\":\"x\"}}" | run_hook)
assert_denied "$generated_absolute"
cache_relative=$(printf '%s' '{"tool_input":{"file_path":".gocache/object","content":"x"}}' | run_hook)
assert_denied "$cache_relative"
cache_absolute=$(printf '%s' "{\"tool_input\":{\"file_path\":\"$root/.gocache/object\",\"content\":\"x\"}}" | run_hook)
assert_denied "$cache_absolute"

# A non-canonical spelling of a protected path must reach the same decision.
generated_traversal=$(printf '%s' '{"tool_input":{"file_path":"internal/server/ui/../ui/dist/generated.js","content":"x"}}' | run_hook)
assert_denied "$generated_traversal"
generated_traversal_absolute=$(printf '%s' "{\"tool_input\":{\"file_path\":\"$root/internal/./server/ui/../ui/dist/generated.js\",\"content\":\"x\"}}" | run_hook)
assert_denied "$generated_traversal_absolute"
cache_traversal=$(printf '%s' '{"tool_input":{"file_path":"ui/../.gocache/object","content":"x"}}' | run_hook)
assert_denied "$cache_traversal"
cache_traversal_absolute=$(printf '%s' "{\"tool_input\":{\"file_path\":\"$root/ui/../.gocache/object\",\"content\":\"x\"}}" | run_hook)
assert_denied "$cache_traversal_absolute"

# A path that only looks like a protected one after traversal must stay allowed.
outside_dist=$(printf '%s' '{"tool_input":{"file_path":"internal/server/ui/dist/../../src/app.tsx","content":"x"}}' | run_hook)
assert_empty "$outside_dist"

long=$(awk 'BEGIN { for (i = 0; i < 9000; i++) printf "x" }')
cat >"$root/docs/features/HANDOFF.md" <<EOF
# handoff
## Current position
$long
## Active change
small
## Later
ignored
EOF

growth_payload=$(jq -cn --arg p docs/features/HANDOFF.md --arg old small --arg new longer '{tool_input:{file_path:$p,old_string:$old,new_string:$new}}')
growth=$(printf '%s' "$growth_payload" | run_hook)
assert_denied "$growth"

growth_traversal_payload=$(jq -cn --arg p docs/specs/../features/HANDOFF.md --arg old small --arg new longer '{tool_input:{file_path:$p,old_string:$old,new_string:$new}}')
growth_traversal=$(printf '%s' "$growth_traversal_payload" | run_hook)
assert_denied "$growth_traversal"

shrink_payload=$(jq -cn --arg p docs/features/HANDOFF.md --arg old "$long" --arg new x '{tool_input:{file_path:$p,old_string:$old,new_string:$new}}')
shrink=$(printf '%s' "$shrink_payload" | run_hook)
assert_empty "$shrink"

replace_all_payload=$(jq -cn --arg p docs/features/HANDOFF.md --arg old small --arg new "$long" \
  '{tool_input:{file_path:$p,old_string:$old,new_string:$new,replace_all:true}}')
replace_all=$(printf '%s' "$replace_all_payload" | run_hook)
assert_denied "$replace_all"

echo "guard-edit fixture tests passed"
