#!/usr/bin/env bash
# 校验 Release 产物本身可用：校验和一致、前端静态包可服务、API 二进制可执行。
# 用法：check-release-artifacts.sh [产物目录，默认 release-artifacts]
#
# 注意：本脚本启用了 pipefail，不要使用 `cmd | head` 这类会让上游进程收到 SIGPIPE
# 的管道——head 提前退出会让上游非零退出，进而让整个脚本静默失败（v0.4.0 发布时
# 就是这样失败的：`find | head` 加 `2> /dev/null` 把错误信息也吞掉了）。
set -euo pipefail

artifacts_dir="${1:-release-artifacts}"
if [ ! -d "$artifacts_dir" ]; then
  echo "产物校验失败：目录不存在 $artifacts_dir" >&2
  exit 1
fi
cd "$artifacts_dir"

workdir=""
probe_file=""
server_pid=""

fail() {
  echo "产物校验失败：$*" >&2
  exit 1
}

cleanup() {
  if [ -n "$server_pid" ]; then
    kill "$server_pid" 2> /dev/null || true
    wait "$server_pid" 2> /dev/null || true
  fi
  if [ -n "$probe_file" ]; then
    rm -f "$probe_file"
  fi
  if [ -n "$workdir" ]; then
    rm -rf "$workdir"
  fi
  return 0
}
trap cleanup EXIT

# 失败时打印临时目录内容，避免再次出现“没有任何输出就退出”。
dump_tree() {
  if [ -z "$workdir" ] || [ ! -d "$workdir" ]; then
    return 0
  fi
  echo "临时目录内容（${workdir}）：" >&2
  find "$workdir" -maxdepth 2 -type f -print 2> /dev/null | sort >&2 || true
}

[ -f SHA256SUMS ] || fail "缺少 SHA256SUMS"
[ -f sdu-alumni-web-dist.tar.gz ] || fail "缺少前端静态包"
[ -x sdu-alumni-api-linux-amd64 ] || fail "API 二进制缺失或不可执行"

echo "校验 SHA256SUMS 与实际文件一致"
sha256sum --check SHA256SUMS || fail "SHA256SUMS 与实际文件不一致"

echo "校验前端静态包可服务"
workdir="$(mktemp -d)" || fail "无法创建临时目录"
tar -xzf sdu-alumni-web-dist.tar.gz -C "$workdir" || fail "前端静态包解压失败"

if [ ! -s "$workdir/index.html" ]; then
  dump_tree
  fail "前端包缺少 index.html"
fi
grep -q 'id="root"' "$workdir/index.html" || fail "index.html 缺少应用挂载点"

# 用 glob 取第一个 js，不用 `find | head`（见文件头注释）。
asset_rel=""
for candidate in "$workdir"/assets/*.js; do
  if [ -f "$candidate" ]; then
    asset_rel="assets/${candidate##*/}"
    break
  fi
done
if [ -z "$asset_rel" ]; then
  dump_tree
  fail "前端包缺少 assets/*.js"
fi

port="${ARTIFACT_CHECK_PORT:-18090}"
probe_file="$(mktemp)"
python3 -m http.server "$port" --directory "$workdir" > /dev/null 2>&1 &
server_pid=$!

served=0
for _ in $(seq 1 10); do
  if curl --fail --silent "http://127.0.0.1:$port/index.html" > "$probe_file"; then
    served=1
    break
  fi
  sleep 1
done
if [ "$served" != "1" ]; then
  fail "index.html 无法通过 HTTP 提供（端口 ${port}）"
fi
grep -q 'id="root"' "$probe_file" ||
  fail "通过 HTTP 取得的 index.html 内容不正确"
curl --fail --silent "http://127.0.0.1:$port/$asset_rel" > /dev/null ||
  fail "静态资源无法通过 HTTP 提供：$asset_rel"

echo "产物校验通过：index.html 与 $asset_rel 均可通过 HTTP 提供"
ls -l
