#!/usr/bin/env bash
# Run govulncheck and fail only on IDs not listed in govulncheck-allowlist.txt.
set -euo pipefail

repo_root="${GITHUB_WORKSPACE:-$(cd "$(dirname "$0")/../.." && pwd)}"
allowlist_file="${GOVULNCHECK_ALLOWLIST:-$repo_root/.github/govulncheck-allowlist.txt}"

set +e
output=$(go run golang.org/x/vuln/cmd/govulncheck@latest ./... 2>&1)
govuln_exit=$?
set -e
printf '%s\n' "$output"

allowlist=""
if [[ -f "$allowlist_file" ]]; then
  allowlist=$(grep -E '^GO-[0-9]{4}-[0-9]+$' "$allowlist_file" | tr '\n' ' ' || true)
else
  echo "govulncheck allowlist not found: $allowlist_file" >&2
fi

ids=$(printf '%s\n' "$output" \
  | grep -oE 'Vulnerability #[0-9]+: GO-[0-9]+-[0-9]+' \
  | grep -oE 'GO-[0-9]+-[0-9]+' \
  | sort -u || true)

if [[ -z "$ids" ]]; then
  # No reachable IDs parsed — keep tool/infra failures.
  exit "$govuln_exit"
fi

failing=0
while IFS= read -r id; do
  [[ -n "$id" ]] || continue
  case " $allowlist " in
    *" $id "*)
      echo "Allowlisted (see .github/govulncheck-allowlist.txt): $id"
      ;;
    *)
      echo "Vulnerability affects project code: $id" >&2
      failing=1
      ;;
  esac
done <<<"$ids"
exit "$failing"
