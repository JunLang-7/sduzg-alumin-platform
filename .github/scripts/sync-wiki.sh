#!/usr/bin/env bash
#
# 把仓库内的指定文档同步到 GitHub Wiki。
#
# Wiki 是下游只读副本：内容以仓库为准，每次同步都会覆盖同名页面。
# 不要直接在 Wiki 上编辑，改动请提交到仓库的 docs/ 目录。
#
# 用法：
#   WIKI_REMOTE_URL='https://x-access-token:<token>@github.com/<owner>/<repo>.wiki.git' \
#     bash .github/scripts/sync-wiki.sh [--dry-run]
#
# 环境变量：
#   WIKI_REMOTE_URL  Wiki 仓库地址，必须包含可推送的认证信息。
#   WIKI_REPO_URL    可选，Home 页面里回链的仓库地址，
#                    默认 https://github.com/${GITHUB_REPOSITORY}
#
# 退出码：
#   0  同步完成，或内容无变化
#   1  参数、源文件或 git 操作出错
#   3  Wiki 仓库尚未初始化，需要先在网页端手工创建任意一页
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

usage() {
  cat <<'EOF'
用法：WIKI_REMOTE_URL=<wiki 仓库地址> bash .github/scripts/sync-wiki.sh [选项]

选项：
  --dry-run        只列出将要写入的页面，不克隆、不提交、不推送
  --skip-missing   跳过尚未存在的源文件并给出告警（发布任务使用；
                   人工执行时默认直接报错，避免页面静默漏同步）
EOF
}

dry_run=0
allow_missing=0
while (($# > 0)); do
  case "$1" in
    --dry-run) dry_run=1 ;;
    --skip-missing) allow_missing=1 ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 1
      ;;
  esac
  shift
done

# 源文件:Wiki 页面。新增页面时同时更新此处、工作流的 paths 与 CONTRIBUTING.md。
page_map=(
  "docs/平台使用说明.md:平台使用说明.md"
  "docs/deployment.md:部署说明.md"
  "docs/api.md:接口文档.md"
  "docs/release-notes/README.md:版本发布记录.md"
)

if ((dry_run)); then
  echo "将同步以下页面（源文件 → Wiki 页面）："
  for mapping in "${page_map[@]}"; do
    printf '  %s → %s\n' "${mapping%%:*}" "${mapping#*:}"
  done
  echo '并生成 Home.md（入口索引）与 _Sidebar.md（导航）'
  exit 0
fi

: "${WIKI_REMOTE_URL:?请通过 WIKI_REMOTE_URL 提供 Wiki 仓库地址}"

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

clone_log="$workdir/clone.log"
if ! git clone --depth 1 --quiet "$WIKI_REMOTE_URL" "$workdir/repo" >"$clone_log" 2>&1; then
  # 认证信息在地址里，日志必须脱敏后再输出。
  sed "s|$WIKI_REMOTE_URL|<wiki-remote>|g" "$clone_log" >&2 || true
  if grep -q 'Repository not found' "$clone_log"; then
    echo 'Wiki 仓库尚未创建：请先在仓库 Wiki 页面手工保存任意一页，再重新执行同步。' >&2
    exit 3
  fi
  echo '克隆 Wiki 仓库失败。' >&2
  exit 1
fi

wiki_dir="$workdir/repo"
source_repo_url="${WIKI_REPO_URL:-https://github.com/${GITHUB_REPOSITORY:-JunLang-7/sduzg-alumin-platform}}"

skipped=()
for mapping in "${page_map[@]}"; do
  src="$repo_root/${mapping%%:*}"
  dest="$wiki_dir/${mapping#*:}"
  if [[ ! -f "$src" ]]; then
    if ((allow_missing)); then
      skipped+=("${mapping%%:*}")
      continue
    fi
    echo "缺少源文件 ${mapping%%:*}：请同步更新 sync-wiki.sh 的页面映射。" >&2
    exit 1
  fi
  if [[ "${mapping%%:*}" == "docs/api.md" ]]; then
    # Wiki 不保留仓库目录结构：文档链接转为 Wiki 页面，代码链接回到 main。
    # 保留 #章节锚点和 API 示例中的地址，不改动仓库内的源文档。
    sed \
      -e 's|](README.md)|](Home)|g' \
      -e 's|](deployment.md)|](部署说明)|g' \
      -e 's|](平台使用说明.md)|](平台使用说明)|g' \
      -e "s|](../server/internal/router/router.go)|](${source_repo_url}/blob/main/server/internal/router/router.go)|g" \
      -e "s|](../server/internal/dto/)|](${source_repo_url}/tree/main/server/internal/dto)|g" \
      "$src" >"$dest"
  else
    cp "$src" "$dest"
  fi
done

if ((${#skipped[@]} > 0)); then
  printf '警告：以下源文件尚不存在，已跳过对应页面：%s\n' "${skipped[*]}" >&2
fi

cat >"$wiki_dir/Home.md" <<EOF
# 政管全球校友数字化平台文档

本 Wiki 由仓库的 \`docs/\` 目录自动同步生成，请勿直接在这里编辑：改动提交到仓库后，由
\`.github/workflows/wiki.yml\` 覆盖对应页面。

- [平台使用说明](平台使用说明) — 各角色的功能与操作步骤、限制条件
- [部署说明](部署说明) — Compose 启动、服务端口、环境配置、迁移与排障
- [接口文档](接口文档) — HTTP API 的认证、请求响应、权限与各模块接口
- [版本发布记录](版本发布记录) — 各版本新增功能、数据库变化与升级说明

源码、Issue 与 PR 见 ${source_repo_url}。
EOF

cat >"$wiki_dir/_Sidebar.md" <<'EOF'
- [首页](Home)
- [平台使用说明](平台使用说明)
- [部署说明](部署说明)
- [接口文档](接口文档)
- [版本发布记录](版本发布记录)
EOF

if [[ -z "$(git -C "$wiki_dir" status --porcelain)" ]]; then
  echo 'Wiki 内容无变化，跳过推送。'
  exit 0
fi

# 空仓库（远端还没有任何提交）时 HEAD 尚未指向提交，只有 symbolic-ref 可用。
branch="$(git -C "$wiki_dir" symbolic-ref --short -q HEAD || git -C "$wiki_dir" rev-parse --abbrev-ref HEAD)"
git -C "$wiki_dir" add -A
git -C "$wiki_dir" \
  -c user.name='github-actions[bot]' \
  -c user.email='41898282+github-actions[bot]@users.noreply.github.com' \
  commit --quiet -m "sync: 从仓库同步文档（${GITHUB_SHA:-local}）"
git -C "$wiki_dir" push --quiet origin "HEAD:$branch"
echo "Wiki 已更新：$(git -C "$wiki_dir" log --oneline -1)"
