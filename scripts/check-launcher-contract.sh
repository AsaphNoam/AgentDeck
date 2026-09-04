#!/bin/sh
# Keep state-writing role launchers aligned with the workflow's safety rules.

set -u

ROOT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
workflow="$ROOT/docs/features/AGENT-WORKFLOW.md"
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
    if [ "$role" = review ]; then
      if ! grep -Fq 'Administrative commits never become default review' "$file"; then
        fail "$tree/$role: missing administrative-commit exclusion"
      fi
      if ! grep -Fq 'Do not review a later unit out of order.' "$file"; then
        fail "$tree/$role: missing ordered-unit selection"
      fi
      if ! grep -Fq 'do not make an empty commit.' "$file"; then
        fail "$tree/$role: missing no-op review exit"
      fi
      if ! grep -Fq 'Findings keep that same unit open' "$file"; then
        fail "$tree/$role: findings can split their originating unit"
      fi
    fi
    if [ "$role" = fix ] &&
       ! grep -Fq 'it does not create a new default review unit.' "$file"; then
      fail "$tree/$role: finding fix creates a review-loop risk"
    fi
    if [ "$role" = work ] &&
       ! grep -Fq 'record that completed change as the sole' "$file"; then
      fail "$tree/$role: completed work does not declare its review unit"
    fi
  done
done

for role in $roles; do
  if ! cmp -s "$ROOT/.agents/skills/$role/SKILL.md" "$ROOT/.claude/skills/$role/SKILL.md"; then
    fail "$role: Claude and Codex launchers differ"
  fi
done

if ! grep -Fq 'A unit is one substantive change from implementation through the' "$workflow"; then
  fail 'workflow: missing substantive change-unit boundary'
fi
if ! grep -Fq 'never enter the default review queue.' "$workflow"; then
  fail 'workflow: missing administrative-commit exclusion'
fi
if ! grep -Fq 'does not become a new default review unit.' "$workflow"; then
  fail 'workflow: finding fixes can re-enter the review queue'
fi
if ! grep -Fq 'make no empty state commit.' "$workflow"; then
  fail 'workflow: no-op reviews can create review obligations'
fi

[ "$errors" -eq 0 ] || exit 1
