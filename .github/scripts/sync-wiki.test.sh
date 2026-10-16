#!/usr/bin/env bash
# 只操作临时本地仓库，不连接 GitHub，不需要访问令牌。
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

fixture="$workdir/source"
remote="$workdir/wiki.git"
mkdir -p "$fixture/.github/scripts" "$fixture/docs/release-notes" "$workdir/seed"
cp "$repo_root/.github/scripts/sync-wiki.sh" "$fixture/.github/scripts/"
cp "$repo_root/docs/api.md" "$repo_root/docs/deployment.md" "$repo_root/docs/平台使用说明.md" "$fixture/docs/"
cp "$repo_root/docs/release-notes/README.md" "$fixture/docs/release-notes/"
source_checksum="$(git hash-object "$fixture/docs/api.md")"

# 保持测试不依赖开发者本地 Git 模板、hooks 或提交签名配置。
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_TEMPLATE_DIR="$workdir/templates"
mkdir -p "$GIT_TEMPLATE_DIR"
git init --quiet --bare --initial-branch=main "$remote"
git -C "$workdir/seed" init --quiet --initial-branch=main
printf '# Existing wiki\n' >"$workdir/seed/Existing.md"
git -C "$workdir/seed" add Existing.md
git -C "$workdir/seed" -c user.name=test -c user.email=test@example.com commit --quiet -m 'seed wiki'
git -C "$workdir/seed" push --quiet "$remote" HEAD:main

export WIKI_REMOTE_URL="file://$remote"
export WIKI_REPO_URL='https://example.com/owner/platform'
export GITHUB_SHA='test-source-sha'
script="$fixture/.github/scripts/sync-wiki.sh"

bash "$script" --dry-run >"$workdir/dry-run.log"
grep -Fq 'docs/api.md → 接口文档.md' "$workdir/dry-run.log"
before="$(git --git-dir="$remote" rev-parse HEAD)"
bash "$script" >"$workdir/first.log"
after="$(git --git-dir="$remote" rev-parse HEAD)"
[[ "$before" != "$after" ]]
for page in 平台使用说明 部署说明 接口文档 版本发布记录 Home _Sidebar Existing; do
  git --git-dir="$remote" cat-file -e "HEAD:$page.md"
done
for page in Home _Sidebar; do
  git --git-dir="$remote" show "HEAD:$page.md" >"$workdir/$page.md"
  grep -Fq '[接口文档](接口文档)' "$workdir/$page.md"
done

git --git-dir="$remote" show HEAD:接口文档.md >"$workdir/api.md"
for link in \
  '[返回文档目录](Home)' \
  '[部署说明](部署说明)' \
  '[平台使用说明](平台使用说明)' \
  '[路由注册](https://example.com/owner/platform/blob/main/server/internal/router/router.go)' \
  '[请求与响应类型](https://example.com/owner/platform/tree/main/server/internal/dto)' \
  '[后端开发说明](https://example.com/owner/platform/blob/main/server/README.md)' \
  '[通用约定](#通用约定)'; do
  grep -Fq "$link" "$workdir/api.md"
done
if grep -Eq '\]\((README\.md|deployment\.md|平台使用说明\.md|\.\./server/)' "$workdir/api.md"; then
  echo '接口文档仍包含不适用于 Wiki 的仓库相对链接' >&2
  exit 1
fi
[[ "$source_checksum" == "$(git hash-object "$fixture/docs/api.md")" ]]
for mapping in '平台使用说明:平台使用说明' 'deployment:部署说明' 'release-notes/README:版本发布记录'; do
  git --git-dir="$remote" show "HEAD:${mapping#*:}.md" >"$workdir/page.md"
  cmp "$fixture/docs/${mapping%%:*}.md" "$workdir/page.md"
done

# 重复同步不产生无意义提交；源文件更新后应生成新提交。
bash "$script" >"$workdir/repeat.log"
[[ "$after" == "$(git --git-dir="$remote" rev-parse HEAD)" ]]
grep -Fq 'Wiki 内容无变化' "$workdir/repeat.log"
printf '\nWiki regression marker\n' >>"$fixture/docs/api.md"
bash "$script" >"$workdir/update.log"
updated="$(git --git-dir="$remote" rev-parse HEAD)"
[[ "$after" != "$updated" ]]
git --git-dir="$remote" show HEAD:接口文档.md >"$workdir/updated-api.md"
grep -Fq 'Wiki regression marker' "$workdir/updated-api.md"

# 缺失文件默认报错且不推送；发布任务可显式跳过并保留已有页面。
rm "$fixture/docs/api.md"
status=0
bash "$script" >"$workdir/missing.log" 2>&1 || status=$?
[[ "$status" == 1 ]]
grep -Fq '缺少源文件 docs/api.md' "$workdir/missing.log"
[[ "$updated" == "$(git --git-dir="$remote" rev-parse HEAD)" ]]
bash "$script" --skip-missing >"$workdir/skip.log" 2>&1
grep -Fq '已跳过对应页面：docs/api.md' "$workdir/skip.log"
[[ "$updated" == "$(git --git-dir="$remote" rev-parse HEAD)" ]]

status=0
bash "$script" --invalid-option >"$workdir/invalid.log" 2>&1 || status=$?
[[ "$status" == 1 ]]
echo 'Wiki 同步回归测试通过：页面、导航、链接、源文件保留、幂等、更新、缺失文件与参数校验。'
