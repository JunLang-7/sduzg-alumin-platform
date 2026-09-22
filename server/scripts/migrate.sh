#!/bin/sh
set -eu

: "${MYSQL_HOST:=mysql}"
: "${MYSQL_PORT:=3306}"
: "${MYSQL_USER:?MYSQL_USER is required}"
: "${MYSQL_PASSWORD:?MYSQL_PASSWORD is required}"
: "${MYSQL_DATABASE:?MYSQL_DATABASE is required}"

# Metadata is declared inside each migration file so this runner never
# hardcodes migration filenames. New files only need the usual NNN_*.sql
# plus optional headers:
#   -- migrate: proves users     # table that implies this migration ran
#   -- migrate: requires data_domains  # must exist if this row is marked applied

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
  echo "migrate: check .env MYSQL_USER/MYSQL_PASSWORD/MYSQL_DATABASE" >&2
  echo "migrate: an existing mysql_data volume keeps credentials from first init; changing .env alone does not update MySQL." >&2
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
  mysql_cmd --batch --skip-column-names -e "SELECT 1;" || true
  fail "cannot connect to MySQL after ${max_attempts} attempts."
}

table_exists() {
  mysql_cmd --batch --skip-column-names -e "
    SELECT COUNT(*)
    FROM information_schema.tables
    WHERE table_schema = DATABASE() AND table_name = '$1';
  "
}

# Print space-separated tables from migration headers (all matching lines):
#   -- migrate: proves users,alumni_profiles
#   -- migrate: requires data_domains
meta_tables() {
  file="$1"
  kind="$2"
  sed -n "s/^--[[:space:]]*migrate:[[:space:]]*${kind}[[:space:]]*//p" "$file" \
    | tr ',' ' ' \
    | tr '\n' ' '
}

wait_for_mysql

if ! migration_table_exists="$(table_exists schema_migrations)"; then
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

# If a migration is marked applied but its proves/requires tables are missing,
# clear the mark so the apply loop can rerun it (fixes stale 006/007 marks).
repair_stale_marks() {
  for migration in /migrations/[0-9][0-9][0-9]_*.sql; do
    [ -f "$migration" ] || continue
    name="$(basename "$migration")"
    proves="$(meta_tables "$migration" proves)"
    requires="$(meta_tables "$migration" requires)"
    [ -n "$proves$requires" ] || continue

    if ! applied="$(mysql_cmd --batch --skip-column-names -e "
      SELECT COUNT(*) FROM schema_migrations WHERE name = '$name';
    ")"; then
      fail "cannot read schema_migrations for $name."
    fi
    [ "$applied" != "0" ] || continue

    stale=0
    for table in $proves $requires; do
      if ! exists="$(table_exists "$table")"; then
        fail "cannot probe table '$table' for $name."
      fi
      if [ "$exists" = "0" ]; then
        stale=1
        break
      fi
    done

    if [ "$stale" = "1" ]; then
      echo "migrate: $name marked applied but probe table missing; clearing mark"
      if ! mysql_cmd -e "DELETE FROM schema_migrations WHERE name = '$name';"; then
        fail "cannot clear stale mark for $name."
      fi
    fi
  done
}

# Pre-tracking volumes: mark migrations applied when their proves-table exists.
# Files without proves inherit the previous file's baseline state (seeds, DDL
# tweaks between checkpoints). New migrations never need editing this script.
baseline_from_schema() {
  [ "$migration_table_exists" = "0" ] || return 0

  prev_marked=0
  for migration in /migrations/[0-9][0-9][0-9]_*.sql; do
    [ -f "$migration" ] || continue
    name="$(basename "$migration")"
    proves="$(meta_tables "$migration" proves)"

    if [ -n "$proves" ]; then
      mark=1
      for table in $proves; do
        if ! exists="$(table_exists "$table")"; then
          fail "cannot probe proves table '$table' for $name."
        fi
        if [ "$exists" = "0" ]; then
          mark=0
          break
        fi
      done
    else
      mark="$prev_marked"
    fi

    if [ "$mark" = "1" ]; then
      echo "migrate: baseline $name"
      if ! mysql_cmd -e "INSERT IGNORE INTO schema_migrations (name) VALUES ('$name');"; then
        fail "cannot baseline $name."
      fi
      prev_marked=1
    else
      prev_marked=0
    fi
  done
}

repair_stale_marks
baseline_from_schema

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
