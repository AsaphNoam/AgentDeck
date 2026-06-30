#!/bin/sh
# SessionStart hook → idle (techspec §4.2). The CLI passes its hook payload as
# JSON on stdin; we forward the session_id when present so the server can refresh
# the running row.
dir="$(dirname "$0")"
input="$(cat 2>/dev/null)"
sid="$(printf '%s' "$input" | jq -r '.session_id // empty' 2>/dev/null)"
set -- SessionStart idle "detail=session started" "last_trace=SessionStart"
[ -n "$sid" ] && set -- "$@" "session_id=$sid"
exec "$dir/_post.sh" "$@"
