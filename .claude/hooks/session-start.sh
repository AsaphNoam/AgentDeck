#!/bin/sh
# SessionStart: inject the live handoff position so no session starts blind.
# AGENT-WORKFLOW §1: HANDOFF.md is read first, every session — this surfaces its
# header automatically; the agent must still read the full file before building.
H="${CLAUDE_PROJECT_DIR:-.}/docs/features/HANDOFF.md"
[ -f "$H" ] || exit 0
echo "AgentDeck handoff — current position (auto-injected; read docs/features/HANDOFF.md in full before building):"
awk '/^## /{n++} n==2{exit} {print}' "$H"
exit 0
