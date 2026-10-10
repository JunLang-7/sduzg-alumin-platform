#!/usr/bin/env bash
set -euo pipefail

validator="$(cd "$(dirname "$0")" && pwd)/check-linked-issue.sh"
check() {
  local expected="$1" author="$2" author_type="$3" body="$4" actual=0
  PR_AUTHOR="$author" PR_AUTHOR_TYPE="$author_type" PR_BODY="$body" bash "$validator" >/dev/null 2>&1 || actual=$?
  if [[ "$actual" != "$expected" ]]; then
    echo "Issue policy failure: author=$author type=$author_type expected=$expected actual=$actual" >&2
    exit 1
  fi
}

check 0 'dependabot[bot]' Bot ''
check 1 'dependabot[bot]' User ''
check 1 'other[bot]' Bot ''
check 1 developer User ''
check 1 developer User 'dependabot[bot] updates dependencies on dependabot/npm'
check 1 developer User 'Closes #abc'
check 0 developer User 'Closes #111'
check 0 developer User 'fixes #111'
check 0 developer User $'Description\nResolves #111'
check 1 '' '' ''
echo 'All 10 Issue policy regression cases passed.'
