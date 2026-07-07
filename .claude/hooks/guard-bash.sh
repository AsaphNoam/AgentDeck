#!/bin/sh
# PreToolUse[Bash]: workflow guards for shell commands.
# Trunk-based repo: pushing to origin requires an explicit human request
# (AGENT-WORKFLOW §6), so any push is downgraded to an ask.
command -v jq >/dev/null 2>&1 || exit 0
cmd=$(jq -r '.tool_input.command // empty' 2>/dev/null) || exit 0
case "$cmd" in
*git\ push*|*git*-C*push*)
  printf '%s\n' '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"Trunk-based repo: pushing to origin requires an explicit human request (AGENT-WORKFLOW §6). Only proceed if the user asked for this push in this session."}}'
  ;;
esac
exit 0
