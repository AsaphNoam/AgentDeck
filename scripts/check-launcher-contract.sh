#!/bin/sh
# Keep state-writing role launchers aligned with the workflow's safety rules.

set -u

ROOT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
errors=0
roles='design-feature work review fix usability-review investigate-bug release review-design'

fail() {
  printf 'launcher contract: %s\n' "$*" >&2
  errors=$((errors + 1))
}

for tree in .agents .claude; do
  for role in $roles; do
    file="$ROOT/$tree/skills/$role/SKILL.md"
    if [ ! -f "$file" ]; then
      fail "$tree/$role: launcher is missing"
      continue
    fi
    if ! grep -Fq 'At startup, inspect `git status` and the existing diff before any repository edit.' "$file"; then
      fail "$tree/$role: missing startup dirty-tree inspection"
    fi
    if ! grep -Eqi '\bcommit\b' "$file"; then
      fail "$tree/$role: missing commit rule"
    fi
    if ! grep -Eqi '\bclose\b' "$file"; then
      fail "$tree/$role: missing close rule"
    fi
  done
done

[ "$errors" -eq 0 ] || exit 1
