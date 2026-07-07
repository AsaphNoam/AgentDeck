#!/bin/sh
# PostToolUse[Edit|Write]: cheap, repo-specific feedback on the exact mistakes
# past reviews kept re-catching (see docs/phases/INVARIANTS.md). Exit 2 feeds
# stderr back to the agent; the edit itself has already been applied.
command -v jq >/dev/null 2>&1 || exit 0
fp=$(jq -r '.tool_input.file_path // empty' 2>/dev/null) || exit 0
[ -n "$fp" ] && [ -f "$fp" ] || exit 0
msgs=""

case "$fp" in
*.go)
  if command -v gofmt >/dev/null 2>&1 && [ -n "$(gofmt -l "$fp" 2>/dev/null)" ]; then
    msgs="${msgs}gofmt: $fp is not gofmt-formatted — run gofmt -w on it.
"
  fi
  # INVARIANTS §7: rows.Err() unchecked after a scan loop recurred 4x in reviews.
  if grep -q 'rows\.Next()' "$fp" && ! grep -q 'rows\.Err()' "$fp"; then
    msgs="${msgs}invariant §7: $fp has a rows.Next() loop but never checks rows.Err() — rows.Close() is cleanup, rows.Err() is the only iteration-failure signal; a mid-scan failure silently truncates the list (this exact bug recurred 4 times in reviews).
"
  fi
  # INVARIANTS §7: bufio.Scanner aborts the whole stream on one oversized line.
  if grep -q 'bufio\.NewScanner' "$fp"; then
    msgs="${msgs}invariant §7 note: $fp uses bufio.NewScanner — it aborts the entire stream on one oversized line (ErrTooLong). Transcript lines here can exceed 8 MiB; oversized records must be skipped, not fatal (past BLOCKING in internal/transcript/reader.go). Verify the buffer/skip handling.
"
  fi
  ;;
esac

# Twin-skill drift guard: .claude/skills and .agents/skills already drifted once.
case "$fp" in
*/.claude/skills/*)
  twin=$(printf '%s' "$fp" | sed 's|/\.claude/skills/|/.agents/skills/|')
  msgs="${msgs}skills are twinned: apply the matching edit to $twin (.claude and .agents copies intentionally differ only in the Claude co-author trailer instruction).
"
  ;;
*/.agents/skills/*)
  twin=$(printf '%s' "$fp" | sed 's|/\.agents/skills/|/.claude/skills/|')
  msgs="${msgs}skills are twinned: apply the matching edit to $twin (.claude and .agents copies intentionally differ only in the Claude co-author trailer instruction).
"
  ;;
esac

if [ -n "$msgs" ]; then
  printf '%s' "$msgs" >&2
  exit 2
fi
exit 0
