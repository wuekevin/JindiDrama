#!/usr/bin/env bash
set -euo pipefail

export GIT_NO_REPLACE_OBJECTS=1
if [[ "$(git rev-parse --is-shallow-repository)" != false ]]; then
  echo 'Distribution source verification requires a complete Git history.' >&2
  exit 1
fi

restricted_commit=$(git log --full-history -1 --format=%H HEAD -- \
  ':(top,icase,literal)design/reference' ':(top,icase,literal)design/diff' \
  ':(top,icase,literal)reference-starter' ':(top,icase,literal)docs/API-LEGAL-GATES.md' \
  ':(top,icase,literal)bmad/MASTER-PROMPT.md')
if [[ -n "$restricted_commit" ]]; then
  echo "Distribution source reaches restricted reference materials (commit $restricted_commit)." >&2
  exit 1
fi

echo 'Distribution source history is clear of the restricted reference paths.'
