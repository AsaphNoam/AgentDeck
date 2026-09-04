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
      if ! grep -Fq 'Administrative commits never become' "$file"; then
        fail "$tree/$role: missing administrative-commit exclusion"
      fi
      if ! grep -Fq 'Chronology does not' "$file"; then
        fail "$tree/$role: review selection is chronologically gated"
      fi
      if ! grep -Fq 'make an empty commit.' "$file"; then
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
    if [ "$role" = fix ] &&
       ! grep -Fq 'Chronology and other role queues' "$file"; then
      fail "$tree/$role: fix selection is gated by another unit"
    fi
    if [ "$role" = work ] &&
       ! grep -Fq 'add that completed change to the' "$file"; then
      fail "$tree/$role: completed work does not declare its review unit"
    fi
    if [ "$role" = work ] &&
       ! grep -Fq 'Several available changes are not a reason to ask' "$file"; then
      fail "$tree/$role: multiple work units require unnecessary ordering"
    fi
    if [ "$role" = design-feature ] &&
       ! grep -Fq 'idea age do not constrain selection.' "$file"; then
      fail "$tree/$role: design selection is ordered or cross-queue gated"
    fi
    if [ "$role" = review-design ] &&
       ! grep -Fq 'Several waiting changes and other role queues do not' "$file"; then
      fail "$tree/$role: design review selection is cross-queue gated"
    fi
    if grep -Eq 'earliest eligible|later unit out of order|sole eligible' "$file"; then
      fail "$tree/$role: stale global ordering rule"
    fi
  done
done

for role in $roles; do
  if ! cmp -s "$ROOT/.agents/skills/$role/SKILL.md" "$ROOT/.claude/skills/$role/SKILL.md"; then
    fail "$role: Claude and Codex launchers differ"
  fi
done

if ! grep -Fq 'Independent role queues; one selected unit per run.' "$workflow"; then
  fail 'workflow: missing independent role queues'
fi
if ! grep -Fq 'never enter the review queue.' "$workflow"; then
  fail 'workflow: missing administrative-commit exclusion'
fi
if ! grep -Fq 'does not become a new default review unit.' "$workflow"; then
  fail 'workflow: finding fixes can re-enter the review queue'
fi
if ! grep -Fq 'make no empty state commit.' "$workflow"; then
  fail 'workflow: no-op reviews can create review obligations'
fi
if ! grep -Fq 'no queue or chronology blocks another.' "$workflow"; then
  fail 'workflow: role queues can block each other'
fi
if grep -Eq 'earliest eligible|later eligible unit out of order|sole eligible review unit' "$workflow"; then
  fail 'workflow: stale global ordering rule'
fi

[ "$errors" -eq 0 ] || exit 1
