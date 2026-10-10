#!/usr/bin/env bash
set -euo pipefail

# Identity comes from GitHub's pull_request.user metadata, never PR prose.
if [[ "${PR_AUTHOR:-}" == 'dependabot[bot]' && "${PR_AUTHOR_TYPE:-}" == 'Bot' ]]; then
  echo 'Dependabot PR: Issue reference exemption applies; all other checks remain required.'
  exit 0
fi

shopt -s nocasematch
pattern='(close[sd]?|fix(e[sd])?|resolve[sd]?)[[:space:]]+#[0-9]+'
if ! [[ "${PR_BODY:-}" =~ $pattern ]]; then
  echo 'PR 描述必须使用 Closes #编号、Fixes #编号 或 Resolves #编号关联 Issue。' >&2
  exit 1
fi
