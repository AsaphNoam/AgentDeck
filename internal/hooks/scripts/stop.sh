#!/bin/sh
# Stop hook → idle/turn complete (techspec §4.2). Fires at the END OF EACH TURN,
# not on CLI exit, so it never clears the running row server-side (§4.3).
dir="$(dirname "$0")"
cat >/dev/null 2>&1
exec "$dir/_post.sh" Stop idle "detail=turn complete" "last_trace=Stop"
