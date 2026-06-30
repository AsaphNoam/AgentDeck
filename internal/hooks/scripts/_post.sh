#!/bin/sh
# AgentDeck hook poster (techspec §4.1). The event wrapper scripts call this with
# the AgentDeck event name, the derived status, and any extra k=v fields:
#
#   _post.sh EVENT STATE [key=value ...]
#
# It reads the per-launch AGENTDECK_* env the runtime injected, applies the
# interface gate, builds a JSON body (jq-encoded so arbitrary tool args are
# safe), and POSTs it to $AGENTDECK_HOOK_URL with the per-launch token. Failures
# never break the agent's turn — every error path is swallowed.
[ -n "$AGENTDECK_AGENT_ID" ] || exit 0
[ -n "$AGENTDECK_HOOK_URL" ] || exit 0

event="$1"
state="$2"
[ -n "$event" ] || exit 0
shift 2

# Interface gate (§4.3): for chat agents the runtime's ACP stream is the
# authoritative status producer, so the hook self-suppresses for every event the
# runtime already owns. Keeping a single status producer per agent.
if [ "$AGENTDECK_INTERFACE" = "chat" ]; then
  case "$event" in
    SessionStart|UserPromptSubmit|PreToolUse|PostToolUse|Stop) exit 0 ;;
  esac
fi

# Build the body. Every positional "k=v" becomes a string field, except
# context_pct which is emitted as a JSON number (the server's context_pct is a
# float). agent_id/event/state always win on the merge.
body="$(jq -nc \
  --arg agent_id "$AGENTDECK_AGENT_ID" \
  --arg event "$event" \
  --arg state "$state" \
  '
  ($ARGS.positional
    | map(split("=") | {key: .[0], val: (.[1:] | join("="))})
    | map(if .key == "context_pct"
          then {(.key): (.val | tonumber? // empty)}
          else {(.key): .val} end)
    | add // {})
  + {agent_id: $agent_id, event: $event, state: $state}
  ' \
  --args "$@")" || exit 0

[ -n "$body" ] || exit 0

curl -fsS --max-time 4 \
  -H "Content-Type: application/json" \
  -H "X-AgentDeck-Token: $AGENTDECK_HOOK_TOKEN" \
  --data "$body" \
  "$AGENTDECK_HOOK_URL" >/dev/null 2>&1 || true
