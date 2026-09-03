#!/bin/sh
# SessionStart: inject the live handoff position so no session starts blind.
# AGENT-WORKFLOW §1.1: this injection *is* the read. The agent opens other
# handoff sections by name only when this header or the task points at them.
H="${CLAUDE_PROJECT_DIR:-.}/docs/features/HANDOFF.md"
[ -f "$H" ] || exit 0
echo "AgentDeck handoff — current position and active change (auto-injected; this is the handoff read, AGENT-WORKFLOW §1.1 — do not re-read the file, open other sections by name only when this points at them):"
awk '/^## /{n++} n==3{exit} {print}' "$H"
exit 0
