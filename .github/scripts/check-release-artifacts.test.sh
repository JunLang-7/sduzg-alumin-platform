#!/usr/bin/env bash
# check-release-artifacts.sh 的回归测试。
#
# 重点覆盖 v0.4.0 发布时踩到的缺陷：产物包内 assets 数量很多时，
# `find | head` 会让 find 收到 SIGPIPE，在 pipefail 下静默失败。
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
checker="$script_dir/check-release-artifacts.sh"

workdir="$(mktemp -d)"
cleanup() {
  rm -rf "$workdir"
  return 0
}
trap cleanup EXIT

build_artifacts() {
  local dir="$1" asset_count="$2" with_index="${3:-1}"
  mkdir -p "$dir/dist/assets"
  if [ "$with_index" = "1" ]; then
    printf '<div id="root"></div>\n' > "$dir/dist/index.html"
  fi
  local i=0
  while [ "$i" -lt "$asset_count" ]; do
    printf 'console.log(%s);\n' "$i" > "$dir/dist/assets/chunk-$i.js"
    i=$((i + 1))
  done
  tar -czf "$dir/sdu-alumni-web-dist.tar.gz" -C "$dir/dist" .
  printf '#!/bin/sh\nexit 0\n' > "$dir/sdu-alumni-api-linux-amd64"
  chmod +x "$dir/sdu-alumni-api-linux-amd64"
  (
    cd "$dir"
    sha256sum sdu-alumni-api-linux-amd64 sdu-alumni-web-dist.tar.gz > SHA256SUMS
  )
}

failures=0

expect_ok() {
  local what="$1" dir="$2" port="$3"
  if ARTIFACT_CHECK_PORT="$port" bash "$checker" "$dir" > "$dir/out.log" 2>&1; then
    echo "  ✓ $what"
    return
  fi
  echo "  ✗ ${what}（预期通过，实际退出非零）" >&2
  sed 's/^/    /' "$dir/out.log" >&2
  failures=$((failures + 1))
}

expect_fail() {
  local what="$1" dir="$2" port="$3" needle="$4"
  if ARTIFACT_CHECK_PORT="$port" bash "$checker" "$dir" > "$dir/out.log" 2>&1; then
    echo "  ✗ ${what}（预期失败，实际通过）" >&2
    failures=$((failures + 1))
    return
  fi
  if grep -q "$needle" "$dir/out.log"; then
    echo "  ✓ $what"
    return
  fi
  echo "  ✗ ${what}（失败但缺少可诊断信息）" >&2
  sed 's/^/    /' "$dir/out.log" >&2
  failures=$((failures + 1))
}

echo "check-release-artifacts.sh 回归测试"

# assets 很多（>200）时最容易触发 head 提前退出导致的 SIGPIPE。
many="$workdir/many"
build_artifacts "$many" 240
expect_ok "assets 很多时通过" "$many" 18100

single="$workdir/single"
build_artifacts "$single" 1
expect_ok "单个 asset 时通过" "$single" 18101

no_index="$workdir/no-index"
build_artifacts "$no_index" 3 0
expect_fail "缺少 index.html 时报错并列出目录内容" "$no_index" 18102 "前端包缺少 index.html"

corrupt="$workdir/corrupt"
build_artifacts "$corrupt" 2
printf 'tampered' >> "$corrupt/sdu-alumni-web-dist.tar.gz"
expect_fail "校验和不一致时报错" "$corrupt" 18103 "SHA256SUMS 与实际文件不一致"

if [ "$failures" -ne 0 ]; then
  echo "回归测试失败：$failures 项" >&2
  exit 1
fi
echo "回归测试通过"
