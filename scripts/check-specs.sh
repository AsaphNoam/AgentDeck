#!/bin/sh
# Mechanical integrity checks for the authoritative specs. Semantic agreement
# between specs and code remains a review responsibility.

set -u

ROOT=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
SPECS="$ROOT/docs/specs"
INDEX="$SPECS/README.md"
errors=0

fail() {
  printf 'spec check: %s\n' "$*" >&2
  errors=$((errors + 1))
}

usage() {
  printf 'usage: %s [--file path]\n' "$0" >&2
  exit 2
}

definitions() {
  # Accepted house styles include "- **R1**", "**R1.**", and
  # "**R1 — Short title.**". Only a line-leading item is a definition.
  sed -n 's/^[[:space:]-]*\*\*\([RA][0-9][0-9]*\)\([^0-9].*\)$/\1/p' "$1"
}

spec_for_id() {
  id=$1
  case "$id" in
    FS-*) dir="$SPECS/features" ;;
    TS-*) dir="$SPECS/tech" ;;
    *) return 1 ;;
  esac
  for candidate in "$dir"/"$id"-*.md; do
    if [ -f "$candidate" ]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

check_links() {
  file=$1
  links=$(awk '
    {
      rest = $0
      while (match(rest, /\]\([^)]*\)/)) {
        print substr(rest, RSTART + 2, RLENGTH - 3)
        rest = substr(rest, RSTART + RLENGTH)
      }
    }
  ' "$file")

  old_ifs=$IFS
  IFS='
'
  for link in $links; do
    case "$link" in
      ''|'#'*|http:*|https:*|mailto:*) continue ;;
    esac
    target=${link%%#*}
    target=${target#<}
    target=${target%>}
    [ -n "$target" ] || continue
    if [ ! -e "$(dirname "$file")/$target" ]; then
      fail "${file#"$ROOT"/}: broken relative link: $link"
    fi
  done
  IFS=$old_ifs
}

check_refs() {
  file=$1

  ids=$(grep -Eo '(FS|TS)-[0-9][0-9]' "$file" 2>/dev/null | sort -u)
  for id in $ids; do
    target=$(spec_for_id "$id")
    if [ -z "$target" ]; then
      fail "${file#"$ROOT"/}: cites missing spec $id"
    fi
  done

  refs=$(grep -Eo '(FS|TS)-[0-9][0-9]\.[RA][0-9]+' "$file" 2>/dev/null | sort -u)
  for ref in $refs; do
    id=${ref%%.*}
    item=${ref#*.}
    target=$(spec_for_id "$id")
    if [ -z "$target" ]; then
      # The missing spec was already reported by the bare-ID check.
      continue
    fi
    if ! definitions "$target" | grep -qx "$item"; then
      fail "${file#"$ROOT"/}: citation $ref does not resolve"
    fi
  done
}

check_spec_file() {
  file=$1
  name=$(basename "$file")
  id=$(printf '%s\n' "$name" | cut -c 1-5)
  first=$(sed -n '1p' "$file")

  case "$first" in
    "# $id — "*) ;;
    *) fail "${file#"$ROOT"/}: H1 must start '# $id — '" ;;
  esac

  status_lines=$(grep -c '^\*\*Status:\*\*' "$file" 2>/dev/null || true)
  valid_status_lines=$(grep -Ec '^\*\*Status:\*\* (Current|Partial|Draft)$' "$file" 2>/dev/null || true)
  if [ "$status_lines" -ne 1 ] || [ "$valid_status_lines" -ne 1 ]; then
    fail "${file#"$ROOT"/}: declare exactly one Status: Current, Partial, or Draft"
  else
    status=$(sed -n 's/^\*\*Status:\*\* //p' "$file")
    case "$status" in
      Current)
        if grep -q '(planned)' "$file"; then
          fail "${file#"$ROOT"/}: Current specs cannot contain (planned) items"
        fi
        ;;
      Partial)
        if ! grep -q '(planned)' "$file"; then
          fail "${file#"$ROOT"/}: Partial specs must identify unshipped items with (planned)"
        fi
        ;;
    esac
  fi

  duplicates=$(definitions "$file" | sort | uniq -d)
  if [ -n "$duplicates" ]; then
    for item in $duplicates; do
      fail "${file#"$ROOT"/}: duplicate requirement ID $item"
    done
  fi

  if ! grep -Eq '^## .*Deviations & open decisions' "$file"; then
    fail "${file#"$ROOT"/}: missing Deviations & open decisions section"
  fi
  if ! grep -Eq '^## .*Traceability' "$file"; then
    fail "${file#"$ROOT"/}: missing Traceability section"
  fi

  case "$id" in
    FS-00) ;;
    FS-*)
      if ! definitions "$file" | grep -q '^A[0-9][0-9]*$'; then
        fail "${file#"$ROOT"/}: feature specs must define acceptance criteria"
      fi
      ;;
  esac
}

check_file() {
  file=$1
  if [ ! -f "$file" ]; then
    fail "file not found: ${file#"$ROOT"/}"
    return
  fi

  if grep -En '^(<<<<<<< |>>>>>>> )|^</?(content|invoke)>[[:space:]]*$' "$file" >/dev/null 2>&1; then
    fail "${file#"$ROOT"/}: contains a conflict marker or tool-output wrapper"
  fi

  case "$(basename "$file")" in
    FS-[0-9][0-9]-*.md|TS-[0-9][0-9]-*.md) check_spec_file "$file" ;;
  esac
  check_links "$file"
  check_refs "$file"
}

check_index() {
  if [ ! -f "$INDEX" ]; then
    fail "docs/specs/README.md is missing"
    return
  fi

  rows=$(awk -F '|' '
    function trim(value) {
      sub(/^[[:space:]]+/, "", value)
      sub(/[[:space:]]+$/, "", value)
      return value
    }
    {
      id = trim($2)
      if (id !~ /^(FS|TS)-[0-9][0-9]$/) next
      field = $3
      if (match(field, /\]\([^)]*\)/)) {
        target = substr(field, RSTART + 2, RLENGTH - 3)
        status = trim($4)
        print id "|" target "|" status
      } else {
        print id "||"
      }
    }
  ' "$INDEX")

  duplicate_ids=$(printf '%s\n' "$rows" | awk -F '|' 'NF {print $1}' | sort | uniq -d)
  for id in $duplicate_ids; do
    fail "docs/specs/README.md: duplicate index ID $id"
  done

  old_ifs=$IFS
  IFS='
'
  for row in $rows; do
    id=${row%%|*}
    remainder=${row#*|}
    target=${remainder%%|*}
    declared_status=${remainder#*|}
    if [ -z "$target" ]; then
      fail "docs/specs/README.md: $id index row has no link"
      continue
    fi
    if [ ! -f "$SPECS/$target" ]; then
      fail "docs/specs/README.md: $id points to missing $target"
      continue
    fi
    target_id=$(basename "$target" | cut -c 1-5)
    if [ "$target_id" != "$id" ]; then
      fail "docs/specs/README.md: $id points to $target_id file $target"
    fi
    file_status=$(sed -n 's/^\*\*Status:\*\* //p' "$SPECS/$target")
    if [ "$declared_status" != "$file_status" ]; then
      fail "docs/specs/README.md: $id status $declared_status does not match $file_status"
    fi
  done
  IFS=$old_ifs

  for file in "$SPECS"/features/FS-[0-9][0-9]-*.md "$SPECS"/tech/TS-[0-9][0-9]-*.md; do
    [ -f "$file" ] || continue
    rel=${file#"$SPECS"/}
    count=$(printf '%s\n' "$rows" | awk -F '|' -v path="$rel" '$2 == path {n++} END {print n + 0}')
    if [ "$count" -ne 1 ]; then
      fail "$rel: expected exactly one index entry, found $count"
    fi
  done
}

if [ "$#" -eq 2 ] && [ "$1" = "--file" ]; then
  case "$2" in
    /*) selected=$2 ;;
    *) selected="$ROOT/$2" ;;
  esac
  check_file "$selected"
elif [ "$#" -eq 0 ]; then
  if [ ! -d "$SPECS" ]; then
    fail "docs/specs directory is missing"
  else
    check_index
    for file in "$INDEX" "$SPECS"/features/*.md "$SPECS"/tech/*.md; do
      [ -f "$file" ] || continue
      check_file "$file"
    done

    # Validate any traceability citation introduced in current Go/TypeScript
    # code or an active implementation plan. Generated and archived material is
    # deliberately excluded.
    for file in $(find "$ROOT/cmd" "$ROOT/internal" -type f -name '*.go' -print 2>/dev/null); do
      check_refs "$file"
    done
    if [ -d "$ROOT/ui/src" ]; then
      for file in $(find "$ROOT/ui/src" -type f \( -name '*.ts' -o -name '*.tsx' \) -print); do
        check_refs "$file"
      done
    fi
    if [ -d "$ROOT/docs/plans" ]; then
      for file in "$ROOT"/docs/plans/*.md; do
        [ -f "$file" ] || continue
        check_refs "$file"
      done
    fi
  fi
else
  usage
fi

if [ "$errors" -ne 0 ]; then
  printf 'spec check: failed with %s error(s)\n' "$errors" >&2
  exit 1
fi

printf 'spec check: ok\n'
