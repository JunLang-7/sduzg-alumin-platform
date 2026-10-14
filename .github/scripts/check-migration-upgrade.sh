#!/usr/bin/env bash
# 升级路径验证（Nightly / Release 门禁）。
#
# 生产环境的数据卷不会重建：compose 只在启动时由 migrate 服务执行
# server/scripts/migrate.sh，把旧库升级到当前迁移集。本脚本在 CI 中复现该路径：
#   1. 用「上一个发布标签」的迁移集初始化数据库，模拟仍在旧版本的线上库；
#   2. 写入旧 schema 下的样例数据；
#   3. 执行生产同款 migrate.sh，升级到当前迁移集；
#   4. 断言迁移记录齐全、既有数据保留、回填与编码修复生效、脚本可重复执行。
#
# 必需环境变量：MYSQL_HOST、MYSQL_PORT、MYSQL_USER、MYSQL_PASSWORD、MYSQL_DATABASE
# 可选环境变量：BASE_TAG（显式指定基线标签）、RESET_DATABASE=1（允许重建目标库）
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

: "${MYSQL_HOST:=127.0.0.1}"
: "${MYSQL_PORT:=3306}"
: "${MYSQL_USER:?MYSQL_USER is required}"
: "${MYSQL_PASSWORD:?MYSQL_PASSWORD is required}"
: "${MYSQL_DATABASE:?MYSQL_DATABASE is required}"

migrations_dir="$repo_root/server/migrations"
migrations_rel="server/migrations"
legacy_names=('旧卷校友甲' '旧卷校友乙')

fail() {
  echo "升级路径验证失败：$*" >&2
  exit 1
}

mysql_admin() {
  MYSQL_PWD="$MYSQL_PASSWORD" mysql --protocol=TCP \
    --host="$MYSQL_HOST" --port="$MYSQL_PORT" \
    --user="$MYSQL_USER" --default-character-set=utf8mb4 "$@"
}

mysql_db() {
  mysql_admin "$MYSQL_DATABASE" "$@"
}

query() {
  mysql_db --batch --skip-column-names -e "$1"
}

assert_eq() {
  local actual="$1" expected="$2" what="$3"
  if [ "$actual" != "$expected" ]; then
    fail "$what：期望 [$expected]，实际 [$actual]"
  fi
  echo "  ✓ $what = $expected"
}

# 当前迁移集（按文件名排序，即执行顺序）。
current_files=()
while IFS= read -r path; do
  current_files+=("$(basename "$path")")
done < <(find "$migrations_dir" -maxdepth 1 -name '[0-9][0-9][0-9]_*.sql' | sort)
[ "${#current_files[@]}" -ge 2 ] || fail "当前迁移集不足两个文件，无法验证增量升级"

# 基线标签：优先 BASE_TAG，其次 HEAD^ 可达的最新标签，最后最旧的标签。
resolve_base_tag() {
  if [ -n "${BASE_TAG:-}" ]; then
    printf '%s' "$BASE_TAG"
    return 0
  fi
  if git describe --tags --abbrev=0 'HEAD^' > /dev/null 2>&1; then
    git describe --tags --abbrev=0 'HEAD^'
    return 0
  fi
  git tag --sort=v:refname | head -n 1 || true
}

base_tag="$(resolve_base_tag)"
base_files=()
if [ -n "$base_tag" ] && git rev-parse --verify --quiet "$base_tag" > /dev/null; then
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    base_files+=("$name")
  done < <(git ls-tree --name-only -r "$base_tag" "$migrations_rel/" |
    sed -n 's#.*/\([0-9][0-9][0-9]_.*\.sql\)$#\1#p' | sort)
fi

if [ "${#base_files[@]}" -eq 0 ] ||
  [ "${#base_files[@]}" -ge "${#current_files[@]}" ]; then
  echo "提示：基线 ${base_tag:-（无标签）} 未提供更旧的迁移集，退化为「除最后一个迁移外」的基线。"
  base_files=("${current_files[@]:0:${#current_files[@]}-1}")
fi

echo "升级路径验证：基线标签=${base_tag:-（无）}"
echo "  旧迁移集（${#base_files[@]}）：${base_files[*]}"
echo "  当前迁移集（${#current_files[@]}）：${current_files[*]}"

# 1) 模拟旧版本线上库。
existing_tables="$(mysql_admin --batch --skip-column-names -e "
  SELECT COUNT(*) FROM information_schema.tables
  WHERE table_schema = '$MYSQL_DATABASE' AND table_type = 'BASE TABLE';")" ||
  fail "无法检查目标数据库状态"
if [ "$existing_tables" != "0" ]; then
  if [ "${RESET_DATABASE:-0}" = "1" ]; then
    echo "重建数据库 $MYSQL_DATABASE（RESET_DATABASE=1）"
    mysql_admin -e "DROP DATABASE IF EXISTS \`$MYSQL_DATABASE\`;" ||
      fail "无法重建数据库 $MYSQL_DATABASE"
  else
    fail "数据库 $MYSQL_DATABASE 已有 $existing_tables 张表；确认可重建时请设置 RESET_DATABASE=1"
  fi
fi

mysql_admin -e "CREATE DATABASE IF NOT EXISTS \`$MYSQL_DATABASE\`
  DEFAULT CHARACTER SET utf8mb4 DEFAULT COLLATE utf8mb4_unicode_ci;" ||
  fail "无法创建数据库 $MYSQL_DATABASE"

for name in "${base_files[@]}"; do
  [ -f "$migrations_dir/$name" ] || fail "旧迁移文件不存在：$name"
  echo "  应用旧迁移：$name"
  mysql_db < "$migrations_dir/$name" || fail "应用旧迁移失败：$name"
done

# 2) 旧 schema 下的样例数据：升级前写入，用于验证回填与数据保留。
mysql_db <<'SQL' || fail "写入旧数据失败"
SET NAMES utf8mb4;

INSERT INTO alumni_profiles
  (name, grade, class_name, cohort, counselor, mentor, major,
   training_mode, industry, work_unit, position, mobile, gender)
VALUES
  ('旧卷校友甲', '2018', 'MPA2班', '2018', '王导员', '赵导师', '公共管理',
   '非全日制', '事业单位', '某某区教育局', '副主任', '13900001111', '女'),
  ('旧卷校友乙', '2019', 'MPA3班', '2019', '孙导员', '钱导师', '行政管理',
   '全日制', '高校', '某某大学', '讲师', '13900002222', '男');

INSERT INTO users (account, password_hash, role, alumni_id, real_name, mobile, status)
SELECT 'legacy-alumni', users.password_hash, 'alumni', alumni_profiles.id,
       '旧卷校友甲', '13900001111', 'active'
FROM users
JOIN alumni_profiles ON alumni_profiles.name = '旧卷校友甲'
WHERE users.account = 'admin';
SQL

profiles_before="$(query "SELECT COUNT(*) FROM alumni_profiles;")"
echo "  升级前 alumni_profiles 行数：$profiles_before"

# 3) 执行生产同款迁移脚本（migrate.sh 固定读取 /migrations）。
prepare_migrations_mount() {
  if mkdir -p /migrations 2> /dev/null && [ -w /migrations ]; then
    rm -f /migrations/*.sql
    cp "$migrations_dir"/*.sql /migrations/
  else
    sudo mkdir -p /migrations
    sudo rm -f /migrations/*.sql
    sudo cp "$migrations_dir"/*.sql /migrations/
  fi
}

prepare_migrations_mount || fail "无法准备 /migrations 目录"

echo "执行 migrate.sh（旧库升级）"
MYSQL_HOST="$MYSQL_HOST" MYSQL_PORT="$MYSQL_PORT" \
  MYSQL_USER="$MYSQL_USER" MYSQL_PASSWORD="$MYSQL_PASSWORD" \
  MYSQL_DATABASE="$MYSQL_DATABASE" \
  bash server/scripts/migrate.sh || fail "migrate.sh 升级失败"

# 4) 断言。
echo "校验升级结果"
assert_eq "$(query "SELECT COUNT(*) FROM information_schema.tables
  WHERE table_schema = DATABASE() AND table_name = 'schema_migrations';")" \
  "1" "schema_migrations 表已创建"

missing=""
for name in "${current_files[@]}"; do
  applied="$(query "SELECT COUNT(*) FROM schema_migrations WHERE name = '$name';")"
  if [ "$applied" != "1" ]; then
    missing="$missing $name"
  fi
done
assert_eq "${missing:-none}" "none" "当前迁移集全部记录在 schema_migrations"

# proves 元数据声明的表必须真实存在（覆盖 users、alumni_files、data_domains 等）。
for name in "${current_files[@]}"; do
  tables="$(sed -n 's/^--[[:space:]]*migrate:[[:space:]]*proves[[:space:]]*//p' \
    "$migrations_dir/$name" | tr ',' ' ')"
  for table in $tables; do
    exists="$(query "SELECT COUNT(*) FROM information_schema.tables
      WHERE table_schema = DATABASE() AND table_name = '$table';")"
    assert_eq "$exists" "1" "$name 声明的表 $table 存在"
  done
done

profiles_after="$(query "SELECT COUNT(*) FROM alumni_profiles;")"
assert_eq "$profiles_after" "$profiles_before" "升级未丢失或重复校友数据"
legacy_kept="$(query "SELECT COUNT(*) FROM alumni_profiles
  WHERE name IN ('旧卷校友甲', '旧卷校友乙');")"
assert_eq "$legacy_kept" "${#legacy_names[@]}" "旧卷校友数据保留"
assert_eq "$(query "SELECT COUNT(*) FROM users
  WHERE account = 'legacy-alumni' AND alumni_id IS NOT NULL;")" \
  "1" "旧卷账号与其校友档案的关联保留"

has_domain_column="$(query "SELECT COUNT(*) FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'alumni_profiles'
    AND column_name = 'data_domain_id';")"
if [ "$has_domain_column" = "1" ]; then
  assert_eq "$(query "SELECT COUNT(*) FROM alumni_profiles
    WHERE data_domain_id IS NULL;")" "0" "数据域回填后没有空值"
  assert_eq "$(query "SELECT COUNT(*) FROM alumni_profiles p
    LEFT JOIN data_domains d ON d.id = p.data_domain_id
    WHERE d.id IS NULL;")" "0" "校友数据域外键没有孤儿行"
fi

has_domains_table="$(query "SELECT COUNT(*) FROM information_schema.tables
  WHERE table_schema = DATABASE() AND table_name = 'data_domains';")"
if [ "$has_domains_table" = "1" ]; then
  assert_eq "$(query "SELECT name FROM data_domains WHERE code = 'mpa';")" \
    "MPA专业学位研究生" "数据域中文名称无乱码"
fi

# 5) 幂等：生产环境每次启动都会执行 migrate.sh，重复执行必须无副作用。
marks_before="$(query "SELECT COUNT(*) FROM schema_migrations;")"
echo "重复执行 migrate.sh（幂等性）"
MYSQL_HOST="$MYSQL_HOST" MYSQL_PORT="$MYSQL_PORT" \
  MYSQL_USER="$MYSQL_USER" MYSQL_PASSWORD="$MYSQL_PASSWORD" \
  MYSQL_DATABASE="$MYSQL_DATABASE" \
  bash server/scripts/migrate.sh || fail "migrate.sh 重复执行失败"
assert_eq "$(query "SELECT COUNT(*) FROM schema_migrations;")" "$marks_before" \
  "重复执行未新增迁移记录"
assert_eq "$(query "SELECT COUNT(*) FROM alumni_profiles;")" "$profiles_after" \
  "重复执行未改动校友数据"

echo "升级路径验证通过：${base_tag:-（无标签）} → 当前迁移集"
