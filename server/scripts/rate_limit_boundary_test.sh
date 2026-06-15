#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SERVER_DIR="$ROOT_DIR/server"
TMP_DIR="${TMPDIR:-/tmp}/platform-rate-limit-boundary"
API_PORT_EXPLICIT="${API_PORT+x}"
API_PORT="${API_PORT:-18083}"
API_URL="http://127.0.0.1:${API_PORT}"
API_LOG="$TMP_DIR/api.log"
RUN_ID="$(date +%s)-$$"

mkdir -p "$TMP_DIR"

if [[ -f "$ROOT_DIR/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT_DIR/.env"
  set +a
fi

MYSQL_USER="${MYSQL_USER:-sdu_alumni}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-sdu_alumni_password}"
MYSQL_DATABASE="${MYSQL_DATABASE:-sdu_alumni_db}"
REDIS_PASSWORD="${REDIS_PASSWORD:-}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

require_cmd curl
require_cmd jq
require_cmd redis-cli
require_cmd vegeta

port_in_use() {
  lsof -iTCP:"$1" -sTCP:LISTEN -n -P >/dev/null 2>&1
}

choose_api_port() {
  local port
  if [[ -n "$API_PORT_EXPLICIT" ]]; then
    if port_in_use "$API_PORT"; then
      echo "port $API_PORT is already in use" >&2
      exit 1
    fi
    return
  fi

  for port in $(seq "$API_PORT" 18120); do
    if ! port_in_use "$port"; then
      API_PORT="$port"
      API_URL="http://127.0.0.1:${API_PORT}"
      API_LOG="$TMP_DIR/api-${API_PORT}.log"
      return
    fi
  done

  echo "no free API port found in range ${API_PORT}-18120" >&2
  exit 1
}

redis_cmd() {
  if [[ -n "$REDIS_PASSWORD" ]]; then
    redis-cli -h 127.0.0.1 -p 6379 -a "$REDIS_PASSWORD" --no-auth-warning "$@"
  else
    redis-cli -h 127.0.0.1 -p 6379 "$@"
  fi
}

clear_rate_keys() {
  local pattern
  for pattern in 'rate:rate_limit:*' 'rate_limit:*'; do
    while IFS= read -r key; do
      [[ -z "$key" ]] && continue
      redis_cmd del "$key" >/dev/null
    done < <(redis_cmd --scan --pattern "$pattern")
  done
}

wait_for_api() {
  for _ in {1..80}; do
    if curl -fsS "$API_URL/api/v1/health/live" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.25
  done
  echo "api did not become healthy; log follows:" >&2
  sed -n '1,160p' "$API_LOG" >&2 || true
  exit 1
}

start_api() {
  choose_api_port
  echo "starting temporary API on ${API_URL}" >&2

  (
    cd "$SERVER_DIR"
    env \
      SERVER_PORT="$API_PORT" \
      DB_ENABLED=true \
      DB_HOST=127.0.0.1 \
      DB_PORT=3307 \
      DB_USER="$MYSQL_USER" \
      DB_PASSWORD="$MYSQL_PASSWORD" \
      DB_NAME="$MYSQL_DATABASE" \
      REDIS_ENABLED=true \
      REDIS_ADDR=127.0.0.1:6379 \
      REDIS_PASSWORD="$REDIS_PASSWORD" \
      STORAGE_ENABLED=false \
      RATE_LIMIT_ENABLED=true \
      RATE_LIMIT_GLOBAL_RPM=4 \
      RATE_LIMIT_AUTH_RPM=10 \
      RATE_LIMIT_VERIFY_CODE_RPM=3 \
      RATE_LIMIT_ADMIN_RPM=30 \
      AUTH_JWT_SECRET=test-secret \
      GIN_MODE=release \
      go run ./cmd/api >"$API_LOG" 2>&1
  ) &
  API_PID=$!
  wait_for_api
}

cleanup() {
  if [[ -n "${API_PID:-}" ]]; then
    kill "$API_PID" >/dev/null 2>&1 || true
    wait "$API_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

write_target() {
  local method="$1"
  local url="$2"
  local body_file="${3:-}"
  local target_file="$4"

  {
    printf '%s %s\n' "$method" "$url"
    if [[ -n "$body_file" ]]; then
      printf 'Content-Type: application/json\n'
      printf '@%s\n' "$body_file"
    fi
  } >"$target_file"
}

attack_json() {
  local name="$1"
  local target_file="$2"
  local rate="$3"
  local duration="$4"
  local result_file="$TMP_DIR/${name}.bin"
  local json

  vegeta attack -rate="$rate" -duration="$duration" -targets="$target_file" -output="$result_file"
  json="$(vegeta report -type=json "$result_file")"
  printf '%s\n' "$json" >"$TMP_DIR/${name}.json"
  jq -c '{requests, status_codes}' <<<"$json" >&2
  printf '%s\n' "$json"
}

count_code() {
  local json="$1"
  local code="$2"
  jq -r --arg code "$code" '.status_codes[$code] // 0' <<<"$json"
}

requests_count() {
  jq -r '.requests' <<<"$1"
}

assert_eq() {
  local actual="$1"
  local expected="$2"
  local message="$3"
  if [[ "$actual" -ne "$expected" ]]; then
    echo "FAIL: $message; expected=$expected actual=$actual" >&2
    exit 1
  fi
}

assert_ge() {
  local actual="$1"
  local expected="$2"
  local message="$3"
  if [[ "$actual" -lt "$expected" ]]; then
    echo "FAIL: $message; expected >= $expected actual=$actual" >&2
    exit 1
  fi
}

assert_between() {
  local actual="$1"
  local low="$2"
  local high="$3"
  local message="$4"
  if [[ "$actual" -lt "$low" || "$actual" -gt "$high" ]]; then
    echo "FAIL: $message; expected between $low and $high actual=$actual" >&2
    exit 1
  fi
}

curl_status() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  if [[ -n "$body" ]]; then
    curl -sS -o /dev/null -w '%{http_code}' -X "$method" -H 'Content-Type: application/json' -d "$body" "$url"
  else
    curl -sS -o /dev/null -w '%{http_code}' -X "$method" "$url"
  fi
}

print_case() {
  printf '\n== %s ==\n' "$1"
}

main() {
  clear_rate_keys
  start_api

  print_case "health checks bypass rate limit"
  clear_rate_keys
  write_target GET "$API_URL/api/v1/health/live" "" "$TMP_DIR/health-targets.txt"
  health_json="$(attack_json health "$TMP_DIR/health-targets.txt" 20 1s)"
  health_requests="$(requests_count "$health_json")"
  assert_eq "$(count_code "$health_json" 200)" "$health_requests" "health requests should all return 200"
  assert_eq "$(count_code "$health_json" 429)" 0 "health requests should not be rate limited"

  print_case "auth login burst is capped below rpm"
  clear_rate_keys
  printf '{"account":"admin","password":"Admin@123456"}' >"$TMP_DIR/login-admin.json"
  write_target POST "$API_URL/api/v1/auth/login" "$TMP_DIR/login-admin.json" "$TMP_DIR/login-targets.txt"
  auth_json="$(attack_json auth-burst "$TMP_DIR/login-targets.txt" 30 1s)"
  auth_200="$(count_code "$auth_json" 200)"
  auth_429="$(count_code "$auth_json" 429)"
  assert_between "$auth_200" 3 4 "auth burst should allow only the initial burst"
  assert_ge "$auth_429" 20 "auth burst should reject most requests"

  print_case "auth login key is isolated by account"
  clear_rate_keys
  for _ in 1 2 3; do
    assert_eq "$(curl_status POST "$API_URL/api/v1/auth/login" '{"account":"admin","password":"Admin@123456"}')" 200 "admin login burst request should pass"
  done
  same_account_status="$(curl_status POST "$API_URL/api/v1/auth/login" '{"account":"admin","password":"Admin@123456"}')"
  assert_eq "$same_account_status" 429 "same account should be rate limited after burst"
  other_account_status="$(curl_status POST "$API_URL/api/v1/auth/login" "{\"account\":\"rate-limit-${RUN_ID}\",\"password\":\"Admin@123456\"}")"
  printf '{"admin_after_burst":%s,"different_account":%s}\n' "$same_account_status" "$other_account_status"
  if [[ "$other_account_status" -eq 429 ]]; then
    echo "FAIL: different account should not share the admin account limiter" >&2
    exit 1
  fi

  print_case "verify code key is isolated by target"
  clear_rate_keys
  same_target='{"target":"13800138000","purpose":"login"}'
  other_target='{"target":"13900139000","purpose":"login"}'
  first_verify="$(curl_status POST "$API_URL/api/v1/auth/verify-code/send" "$same_target")"
  if [[ "$first_verify" -eq 429 ]]; then
    echo "FAIL: first verify-code request should not be rate limited" >&2
    exit 1
  fi
  same_target_status="$(curl_status POST "$API_URL/api/v1/auth/verify-code/send" "$same_target")"
  assert_eq "$same_target_status" 429 "same target should be rate limited after burst"
  other_verify="$(curl_status POST "$API_URL/api/v1/auth/verify-code/send" "$other_target")"
  printf '{"first_target":%s,"same_target_after_burst":%s,"different_target":%s}\n' "$first_verify" "$same_target_status" "$other_verify"
  if [[ "$other_verify" -eq 429 ]]; then
    echo "FAIL: different verify-code target should not share limiter" >&2
    exit 1
  fi

  print_case "admin endpoints are rate limited"
  clear_rate_keys
  write_target GET "$API_URL/api/v1/admin/dashboard/overview" "" "$TMP_DIR/admin-targets.txt"
  admin_json="$(attack_json admin "$TMP_DIR/admin-targets.txt" 10 1s)"
  admin_429="$(count_code "$admin_json" 429)"
  assert_ge "$admin_429" 3 "admin endpoint should be rate limited after burst"

  print_case "global api limit rejects anonymous alumni requests after burst"
  clear_rate_keys
  write_target GET "$API_URL/api/v1/alumni" "" "$TMP_DIR/global-targets.txt"
  global_json="$(attack_json global "$TMP_DIR/global-targets.txt" 10 1s)"
  global_401="$(count_code "$global_json" 401)"
  global_429="$(count_code "$global_json" 429)"
  assert_between "$global_401" 4 5 "global limiter should allow only the initial burst into auth middleware"
  assert_ge "$global_429" 4 "global limiter should reject requests after burst"

  echo
  echo "PASS: rate limit boundary tests completed"
  echo "Reports saved under: $TMP_DIR"
}

main "$@"
