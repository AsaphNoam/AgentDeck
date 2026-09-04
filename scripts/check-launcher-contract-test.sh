#!/bin/sh
# Mutation evidence for check-launcher-contract.sh (INV §17): a launcher or workflow
# copy that states the opposite of a guarded rule must fail the check. Each case
# reproduces the pre-fix wording the assertion is supposed to reject.

set -u

ROOT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
checker="$ROOT/scripts/check-launcher-contract.sh"
errors=0

fail() {
  printf 'launcher contract test: %s\n' "$*" >&2
  errors=$((errors + 1))
}

work=$(mktemp -d) || exit 1
trap 'rm -rf "$work"' EXIT

fixture() {
  dir="$work/$1"
  mkdir -p "$dir/docs/features" "$dir/.agents/skills" "$dir/.claude/skills"
  cp -R "$ROOT/.agents/skills/." "$dir/.agents/skills/"
  cp -R "$ROOT/.claude/skills/." "$dir/.claude/skills/"
  cp "$ROOT/docs/features/AGENT-WORKFLOW.md" "$dir/docs/features/"
  printf '%s\n' "$dir"
}

mutate() {
  target="$1/$2"
  sed "$3" "$target" > "$target.mutated" && mv "$target.mutated" "$target"
}

# Both trees carry the same wording, so mutate them together and leave the twin
# comparison out of the evidence.
mutate_launcher() {
  mutate "$1" ".agents/skills/$2/SKILL.md" "$3"
  mutate "$1" ".claude/skills/$2/SKILL.md" "$3"
}

expect_failure() {
  name=$1
  expected=$2
  dir=$3
  if output=$("$checker" --root "$dir" 2>&1); then
    fail "$name: mutated tree still passes the contract check"
    return
  fi
  case "$output" in
    *"$expected"*) ;;
    *) fail "$name: expected '$expected'; got: $output" ;;
  esac
}

baseline=$(fixture baseline)
if ! output=$("$checker" --root "$baseline" 2>&1); then
  fail "baseline: unmutated tree fails the contract check: $output"
fi

# The review launcher tells a no-op run to make the empty commit it must refuse.
dir=$(fixture review-noop)
mutate_launcher "$dir" review 's/report that and do not$/report that and/'
expect_failure 'review no-op reversal' 'missing no-op review exit' "$dir"

# The review launcher keeps the chronology sentence but orders unit selection.
dir=$(fixture review-selection)
mutate_launcher "$dir" review 's/choose any available review unit named$/choose the first review unit named/'
expect_failure 'review ordered selection' 'review selection does not authorize any available unit' "$dir"

# The design launcher recognizes a named idea only in the two queue sections.
dir=$(fixture design-named)
mutate_launcher "$dir" design-feature 's/names an entry in any section of `docs\/ideas.md`/names an entry in `New ideas` or `Ideas being defined`/'
expect_failure 'design named-idea narrowing' 'a named idea outside the queue sections is unrecognized' "$dir"

# The design launcher drops the limit on unnamed automatic selection.
dir=$(fixture design-auto)
mutate_launcher "$dir" design-feature 's/automatic selection is limited to the available or resumable ideas://'
expect_failure 'design unbounded automatic selection' 'unnamed design selection is not limited' "$dir"

# The workflow tells a no-op review to commit anyway.
dir=$(fixture workflow-noop)
mutate "$dir" docs/features/AGENT-WORKFLOW.md 's/make no empty state commit\./make an empty state commit./'
expect_failure 'workflow no-op reversal' 'no-op reviews can create review obligations' "$dir"

# The workflow narrows a named idea back to the two queue sections.
dir=$(fixture workflow-named)
mutate "$dir" docs/features/AGENT-WORKFLOW.md 's/named by the user from any `docs\/ideas.md` section/named by the user from `New ideas` or `Ideas being defined`/'
expect_failure 'workflow named-idea narrowing' 'a named idea outside the queue sections is unrecognized' "$dir"

# The workflow drops the limit on unnamed automatic selection.
dir=$(fixture workflow-auto)
mutate "$dir" docs/features/AGENT-WORKFLOW.md 's/automatic selection is limited to the available or resumable ideas: //'
expect_failure 'workflow unbounded automatic selection' 'unnamed design selection is not limited' "$dir"

[ "$errors" -eq 0 ] || exit 1
