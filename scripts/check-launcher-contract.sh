#!/bin/sh
# Keep state-writing role launchers aligned with the workflow's safety rules.

set -u

ROOT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
# --root checks a fixture tree instead of this repository; the mutation test in
# check-launcher-contract-test.sh uses it to prove each assertion can fail.
if [ "${1-}" = "--root" ]; then
  ROOT=$(CDPATH= cd "${2-}" && pwd) || exit 2
fi
workflow="$ROOT/docs/features/AGENT-WORKFLOW.md"
errors=0
roles='design-feature work review fix usability-review investigate-bug release review-design'

fail() {
  printf 'launcher contract: %s\n' "$*" >&2
  errors=$((errors + 1))
}

# Rules wrap across lines, so match them against the file as one whitespace-normalized
# line. A rule stated only in part, or stated in reverse, does not match.
has_rule() {
  tr '\n' ' ' < "$1" | tr -s ' ' | grep -Fq "$2"
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
      if ! has_rule "$file" 'choose any available review unit named by the handoff'; then
        fail "$tree/$role: review selection does not authorize any available unit"
      fi
      if ! has_rule "$file" 'Chronology does not constrain selection, and other role queues do not block review.'; then
        fail "$tree/$role: review selection is chronologically or cross-queue gated"
      fi
      if ! has_rule "$file" 'if no eligible unit is pending, report that and do not make an empty commit.'; then
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
       ! has_rule "$file" 'Other role queues and idea age do not constrain selection.'; then
      fail "$tree/$role: design selection is ordered or cross-queue gated"
    fi
    if [ "$role" = design-feature ] &&
       ! has_rule "$file" 'names an entry in any section of `docs/ideas.md`'; then
      fail "$tree/$role: a named idea outside the queue sections is unrecognized"
    fi
    if [ "$role" = design-feature ] &&
       ! has_rule "$file" 'automatic selection is limited to the available or resumable ideas'; then
      fail "$tree/$role: unnamed design selection is not limited to available or resumable ideas"
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
if ! has_rule "$workflow" 'If a review finds no eligible unit, report that and make no empty state commit.'; then
  fail 'workflow: no-op reviews can create review obligations'
fi
if ! has_rule "$workflow" 'Use the idea named by the user from any `docs/ideas.md` section'; then
  fail 'workflow: a named idea outside the queue sections is unrecognized'
fi
if ! has_rule "$workflow" 'automatic selection is limited to the available or resumable ideas'; then
  fail 'workflow: unnamed design selection is not limited to available or resumable ideas'
fi
if ! grep -Fq 'no queue or chronology blocks another.' "$workflow"; then
  fail 'workflow: role queues can block each other'
fi
if grep -Eq 'earliest eligible|later eligible unit out of order|sole eligible review unit' "$workflow"; then
  fail 'workflow: stale global ordering rule'
fi

[ "$errors" -eq 0 ] || exit 1
