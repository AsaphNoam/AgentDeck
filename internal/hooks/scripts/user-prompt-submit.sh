#!/bin/sh
# UserPromptSubmit hook → busy/thinking (techspec §4.2).
dir="$(dirname "$0")"
cat >/dev/null 2>&1
exec "$dir/_post.sh" UserPromptSubmit busy "detail=thinking" "last_trace=UserPromptSubmit"
