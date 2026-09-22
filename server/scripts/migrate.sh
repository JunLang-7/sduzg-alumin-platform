#!/bin/sh
set -eu

: "${MYSQL_HOST:=mysql}"
: "${MYSQL_PORT:=3306}"
: "${MYSQL_USER:?MYSQL_USER is required}"
: "${MYSQL_PASSWORD:?MYSQL_PASSWORD is required}"
: "${MYSQL_DATABASE:?MYSQL_DATABASE is required}"

# Official image only applies MYSQL_* when the data volume is first created.
# Teammates often have a local .env (gitignored) that no longer matches the
# volume password. Retry a few times to ride out first-boot races, then fail
# with actionable context instead of a bare mysql client error.
mysql_cmd() {
  MYSQL_PWD="$MYSQL_PASSWORD" mysql \
    --protocol=TCP \
    --host="$MYSQL_HOST" \
    --port="$MYSQL_PORT" \
    --user="$MYSQL_USER" \
    "$MYSQL_DATABASE" "$@"
}

fail() {
  echo "migrate: $*" >&2
  echo "migrate: host=${MYSQL_HOST}:${MYSQL_PORT} user=${MYSQL_USER} database=${MYSQL_DATABASE}" >&2
  echo "migrate: check .env MYSQL_USER/MYSQL_PASSWORD/MYSQL_ROOT_PASSWORD/MYSQL_DATABASE" >&2
  echo "migrate: an existing mysql_data volume keeps credentials from first init; changing .env alone does not update MySQL." >&2
  echo "migrate: dev reset (destroys data): docker compose down -v && docker compose up --build" >&2
  exit 1
}

wait_for_mysql() {
  attempt=1
  max_attempts=30
  while [ "$attempt" -le "$max_attempts" ]; do
    if mysql_cmd --batch --skip-column-names -e "SELECT 1;" >/dev/null 2>&1; then
      return 0
    fi
    echo "migrate: waiting for MySQL (${attempt}/${max_attempts}) as ${MYSQL_USER}@${MYSQL_HOST}..." >&2
    attempt=$((attempt + 1))
    sleep 2
  done
  # Show the real client error once retries are exhausted.
  mysql_cmd --batch --skip-column-names -e "SELECT 1;" || true
  fail "cannot connect to MySQL after ${max_attempts} attempts."
}

wait_for_mysql

if ! migration_table_exists="$(mysql_cmd --batch --skip-column-names -e "
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE() AND table_name = 'schema_migrations';
")"; then
  fail "cannot query MySQL."
fi

if ! mysql_cmd -e '
  CREATE TABLE IF NOT EXISTS schema_migrations (
    name VARCHAR(255) NOT NULL PRIMARY KEY,
    applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT="已执行的数据库迁移";
'; then
  fail "cannot create schema_migrations (insufficient privileges or auth failure)."
fi

# Volumes created before migration tracking was added already contain the
# pre-history schema. Mark those versions as the baseline, then run every later
# migration normally. All releases containing this runner include 001-007.
if [ "$migration_table_exists" = "0" ]; then
  if ! users_table_exists="$(mysql_cmd --batch --skip-column-names -e "
    SELECT COUNT(*)
    FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = 'users';
  ")"; then
    fail "cannot detect users table for baseline."
  fi
  if [ "$users_table_exists" != "0" ]; then
    echo "migrate: baseline 001-007 for existing volume"
    if ! mysql_cmd -e "
      INSERT IGNORE INTO schema_migrations (name) VALUES
        ('001_init_schema.sql'),
        ('002_seed_test_user.sql'),
        ('003_add_alumni_files.sql'),
        ('004_add_user_email.sql'),
        ('005_add_indexes.sql'),
        ('006_add_admin_access_control.sql'),
        ('007_fix_data_domain_encoding.sql');
    "; then
      fail "cannot write baseline rows to schema_migrations."
    fi
  fi
fi

for migration in /migrations/[0-9][0-9][0-9]_*.sql; do
  [ -f "$migration" ] || continue
  name="$(basename "$migration")"
  if ! applied="$(mysql_cmd --batch --skip-column-names -e "
    SELECT COUNT(*) FROM schema_migrations WHERE name = '$name';
  ")"; then
    fail "cannot read schema_migrations for $name."
  fi
  if [ "$applied" != "0" ]; then
    continue
  fi

  echo "Applying migration: $name"
  if ! mysql_cmd < "$migration"; then
    fail "failed applying $name"
  fi
  if ! mysql_cmd -e "INSERT IGNORE INTO schema_migrations (name) VALUES ('$name');"; then
    fail "applied $name but could not record it."
  fi
done

echo "migrate: done"
