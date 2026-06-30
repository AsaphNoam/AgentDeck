#!/bin/sh
# PostToolUse hook → busy, detail "<tool> done" (techspec §4.2).
dir="$(dirname "$0")"
input="$(cat 2>/dev/null)"
tool="$(printf '%s' "$input" | jq -r '.tool_name // "tool"' 2>/dev/null)"
[ -n "$tool" ] || tool="tool"
exec "$dir/_post.sh" PostToolUse busy "detail=$tool done" "last_trace=PostToolUse: $tool"
