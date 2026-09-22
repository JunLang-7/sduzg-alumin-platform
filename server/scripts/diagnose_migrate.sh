#!/bin/sh
# Run on a machine where compose/migrate fails, from the repo root.
# Prints only non-secret diagnostics for comparing environments.
set -eu

echo "== compose =="
docker compose version 2>&1 || true

echo "== migrate logs =="
docker compose logs migrate --tail 200 2>&1 || true

echo "== mysql status =="
docker compose ps mysql migrate 2>&1 || true

echo "== resolved MYSQL_* keys (values redacted) =="
docker compose config 2>/dev/null | awk '
  /MYSQL_USER:/ || /MYSQL_DATABASE:/ { print; next }
  /MYSQL_PASSWORD:/ || /MYSQL_ROOT_PASSWORD:/ {
    split($0, a, ":")
    print a[1] ": <set,len=" length(substr($0, index($0, ":") + 2)) ">"
  }
' || true

echo "== .env keys present =="
if [ -f .env ]; then
  grep -E '^[A-Z_]+=' .env | cut -d= -f1 | sort
else
  echo "(no .env — compose defaults apply)"
fi

echo "== script line endings =="
if [ -f server/scripts/migrate.sh ]; then
  if grep -q $'\r' server/scripts/migrate.sh; then
    echo "migrate.sh has CRLF"
  else
    echo "migrate.sh is LF"
  fi
else
  echo "migrate.sh missing"
fi
