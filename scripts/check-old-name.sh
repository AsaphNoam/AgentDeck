#!/bin/sh
# TS-06.R24: no build, release, or packaging input still spells the pre-rename
# product name. The pattern is split so this script does not match itself.
set -eu
cd "$(dirname "$0")/.."
old='agent''deck'
if git ls-files -- Makefile go.mod install.sh cmd scripts .github ui/package.json ui/index.html \
    ui/remote.html ui/public ui/vite.config.ts \
  | xargs grep -il "$old" 2>/dev/null; then
  echo "check-old-name: the files above still spell the old product name" >&2
  exit 1
fi
if git ls-files | grep -iv '^docs/archive/\|^LearningArtifacts/' | grep -i "$old"; then
  echo "check-old-name: the paths above still carry the old product name" >&2
  exit 1
fi
