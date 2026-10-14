#!/usr/bin/env bash
# 校验构建产物中的 API 二进制能否在无 MySQL、Redis、MinIO 时启动，
# 并通过存活与就绪检查。Nightly 与 Release 工作流共用。
set -euo pipefail

binary="${1:-}"
if [ -z "$binary" ]; then
  echo "用法：check-api-binary.sh <api 二进制路径>" >&2
  exit 2
fi
if [ ! -x "$binary" ]; then
  echo "找不到可执行的 API 二进制：$binary" >&2
  exit 2
fi

port="${CHECK_API_PORT:-18080}"
DB_ENABLED=false \
REDIS_ENABLED=false \
STORAGE_ENABLED=false \
SERVER_HOST=127.0.0.1 \
SERVER_PORT="$port" \
  "$binary" &
api_pid=$!
trap 'kill "$api_pid" 2>/dev/null || true' EXIT

live="http://127.0.0.1:${port}/api/v1/health/live"
ready="http://127.0.0.1:${port}/api/v1/health/ready"
for _ in $(seq 1 20); do
  if curl --fail --silent "$live" > /dev/null \
    && curl --fail --silent "$ready" > /dev/null; then
    echo "API 二进制通过存活与就绪检查：$binary"
    exit 0
  fi
  sleep 2
done

echo "错误：API 二进制未通过存活与就绪检查：$binary" >&2
exit 1
