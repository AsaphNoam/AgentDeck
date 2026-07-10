#!/bin/sh
# PreToolUse[Bash]: workflow guards for shell commands.
# Trunk-based repo: normal pushes to origin are part of task completion
# (AGENT-WORKFLOW §6) and pass through. Only history-rewriting pushes
# still require an explicit human request (§3 destructive action).
command -v jq >/dev/null 2>&1 || exit 0
cmd=$(jq -r '.tool_input.command // empty' 2>/dev/null) || exit 0
case "$cmd" in
*git*push*--force*|*git*push*\ -f\ *|*git*push*\ -f)
  printf '%s\n' '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"Force-pushing rewrites remote history — a destructive action (AGENT-WORKFLOW §3). Only proceed if the user asked for this force-push in this session."}}'
  ;;
esac
exit 0
