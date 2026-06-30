#!/bin/sh
# PreToolUse hook → busy, detail names the tool (techspec §4.2).
dir="$(dirname "$0")"
input="$(cat 2>/dev/null)"
tool="$(printf '%s' "$input" | jq -r '.tool_name // "tool"' 2>/dev/null)"
[ -n "$tool" ] || tool="tool"
exec "$dir/_post.sh" PreToolUse busy "detail=$tool" "last_trace=PreToolUse: $tool"
