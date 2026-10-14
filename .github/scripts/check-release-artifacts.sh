#!/usr/bin/env bash
# 校验 Release 产物本身可用：校验和一致、前端静态包可服务、API 二进制可执行。
# 用法：check-release-artifacts.sh [产物目录，默认 release-artifacts]
set -euo pipefail

artifacts_dir="${1:-release-artifacts}"
if [ ! -d "$artifacts_dir" ]; then
  echo "产物校验失败：目录不存在 $artifacts_dir" >&2
  exit 1
fi
cd "$artifacts_dir"

fail() {
  echo "产物校验失败：$*" >&2
  exit 1
}

[ -f SHA256SUMS ] || fail "缺少 SHA256SUMS"
[ -f sdu-alumni-web-dist.tar.gz ] || fail "缺少前端静态包"
[ -x sdu-alumni-api-linux-amd64 ] || fail "API 二进制缺失或不可执行"

echo "校验 SHA256SUMS 与实际文件一致"
sha256sum --check SHA256SUMS

echo "校验前端静态包可服务"
workdir="$(mktemp -d)"
tar -xzf sdu-alumni-web-dist.tar.gz -C "$workdir"
[ -s "$workdir/index.html" ] || fail "前端包缺少 index.html"
grep -q 'id="root"' "$workdir/index.html" || fail "index.html 缺少应用挂载点"

asset="$(find "$workdir/assets" -type f -name '*.js' 2> /dev/null | head -n 1)"
[ -n "$asset" ] || fail "前端包缺少 assets/*.js"
asset_rel="${asset#"$workdir"/}"

port="${ARTIFACT_CHECK_PORT:-18090}"
python3 -m http.server "$port" --directory "$workdir" > /dev/null 2>&1 &
server_pid=$!
cleanup() {
  kill "$server_pid" 2> /dev/null || true
  wait "$server_pid" 2> /dev/null || true
  rm -rf "$workdir"
}
trap cleanup EXIT

served=0
for _ in $(seq 1 10); do
  if curl --fail --silent "http://127.0.0.1:$port/index.html" > /dev/null; then
    served=1
    break
  fi
  sleep 1
done
[ "$served" = "1" ] || fail "index.html 无法通过 HTTP 提供"

curl --fail --silent "http://127.0.0.1:$port/index.html" | grep -q 'id="root"' ||
  fail "通过 HTTP 取得的 index.html 内容不正确"
curl --fail --silent "http://127.0.0.1:$port/$asset_rel" > /dev/null ||
  fail "静态资源无法通过 HTTP 提供：$asset_rel"

echo "产物校验通过"
ls -l
