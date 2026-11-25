#!/usr/bin/env bash
set -euo pipefail

checker="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/verify-distribution-source.sh"
scratch=$(mktemp -d)
trap 'rm -r -- "$scratch"' EXIT
repo="$scratch/source"
git init -q -b main "$repo"
cd "$repo"
git config user.name 'Distribution gate fixture'
git config user.email 'fixture@example.com'
git -c commit.gpgsign=false commit --allow-empty -qm 'Clean root'
bash "$checker"

expect_failure() {
  local expected="$1"
  if bash "$checker" >"$scratch/result" 2>&1; then
    echo "Expected verification to fail: $expected" >&2
    exit 1
  fi
  grep -Fq "$expected" "$scratch/result"
}

git -c commit.gpgsign=false commit --allow-empty -qm 'Second clean commit'
git clone -q --no-local --depth=1 "$repo" "$scratch/shallow"
(cd "$scratch/shallow" && expect_failure 'requires a complete Git history')

git switch -qc reference-fixture
mkdir -p Design/Reference
printf 'fixture\n' >Design/Reference/01.png
git add -- Design/Reference/01.png
git -c commit.gpgsign=false commit -qm 'Add a restricted reference'
expect_failure 'reaches restricted reference materials'
(cd Design && expect_failure 'reaches restricted reference materials')
git rm -q -- Design/Reference/01.png
git -c commit.gpgsign=false commit -qm 'Remove the file from the visible tree'
expect_failure 'reaches restricted reference materials'

git switch -q main
bash "$checker"
git -c commit.gpgsign=false merge --no-ff -qm 'Merge historical reference commit' reference-fixture
expect_failure 'reaches restricted reference materials'
echo 'Distribution source fixtures passed.'
