#!/usr/bin/env bash
# 判定一次失败运行里的失败 job 是「runner 被回收」这类基础设施中断，还是真实失败。
#
# 背景（#217）：托管 runner 被宿主回收时，job 来不及自己报错，只会收到 SIGTERM，日志结尾固定是
#   The runner has received a shutdown signal. This can happen when the runner
#   service is stopped, or a manually started runner is canceled.
#   Process completed with exit code 143.
# 这类失败重跑即可，不是代码问题；但也不能把真实失败误判成基础设施抖动，
# 所以只有「所有失败 job 都只剩中断痕迹」时才判定为 infra。
#
# 用法：
#   classify-run-failure.sh [--jobs-file FILE] [--annotations-dir DIR] [--logs-dir DIR]
#
# 默认通过 gh 拉取当前运行（GITHUB_REPOSITORY / GITHUB_RUN_ID / GITHUB_RUN_ATTEMPT）。
# 三个参数用于离线回归测试：直接喂 jobs JSON、注解目录（<check_run_id>.json）
# 与已解压的日志目录。
#
# 输出：
#   - 标准输出：逐 job 判定结果
#   - $GITHUB_STEP_SUMMARY：Markdown 摘要（若已设置）
#   - $GITHUB_OUTPUT：kind=infra|real|none（若已设置）
# 判定本身不会以非零退出：是否重跑由调用方根据 kind 决定。
set -euo pipefail

shutdown_marker="received a shutdown signal"
# SIGTERM 收尾时 runner 写给失败步骤的注解，是中断留下的唯一注解。
termination_message="Process completed with exit code 143."

jobs_file=""
annotations_dir=""
logs_dir=""

usage() {
  cat >&2 <<'EOF'
用法：classify-run-failure.sh [--jobs-file FILE] [--annotations-dir DIR] [--logs-dir DIR]

  --jobs-file FILE        jobs API 返回的 JSON（默认通过 gh 获取）
  --annotations-dir DIR   目录下按 <check_run_id>.json 存放注解（默认通过 gh 获取）
  --logs-dir DIR          已解压的运行日志目录（默认通过 gh 获取并解压）
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --jobs-file)
      jobs_file="${2:?--jobs-file 需要参数}"
      shift 2
      ;;
    --annotations-dir)
      annotations_dir="${2:?--annotations-dir 需要参数}"
      shift 2
      ;;
    --logs-dir)
      logs_dir="${2:?--logs-dir 需要参数}"
      shift 2
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      echo "未知参数：$1" >&2
      usage
      exit 2
      ;;
  esac
done

repo="${GITHUB_REPOSITORY:-}"
run_id="${GITHUB_RUN_ID:-}"
run_attempt="${GITHUB_RUN_ATTEMPT:-1}"
summary_file="${GITHUB_STEP_SUMMARY:-/dev/null}"

workdir=""
cleanup() {
  if [ -n "$workdir" ]; then
    rm -rf "$workdir"
  fi
  return 0
}
trap cleanup EXIT

need_workdir() {
  if [ -z "$workdir" ]; then
    workdir="$(mktemp -d)"
  fi
}

# 日志与注解按 job 名归档时，路径分隔符会被替换成下划线。
sanitize_name() {
  printf '%s' "$1" | tr '/' '_'
}

emit_summary() {
  local kind="$1" text="$2" rows="$3" note="$4"
  {
    echo '## Nightly 失败判定'
    echo
    echo "运行：[\`$run_id\`](https://github.com/$repo/actions/runs/$run_id)（attempt ${run_attempt}）"
    echo
    echo "**判定：$kind** — $text"
    echo
    echo '| 失败 job | 判定 | 依据 |'
    echo '| --- | --- | --- |'
    printf '%s' "$rows"
    if [ -n "$note" ]; then
      echo
      echo "$note"
    fi
  } >> "$summary_file"
  if [ -n "${GITHUB_OUTPUT:-}" ]; then
    echo "kind=$kind" >> "$GITHUB_OUTPUT"
  fi
}

if [ -z "$jobs_file" ]; then
  if [ -z "$repo" ] || [ -z "$run_id" ]; then
    echo '缺少 GITHUB_REPOSITORY / GITHUB_RUN_ID，或改用 --jobs-file 等离线参数。' >&2
    exit 2
  fi
  need_workdir
  jobs_file="$workdir/jobs.json"
  # --slurp 不能和 --jq 同用，因此把分页结果统一收敛成一个 {"jobs": [...]}。
  gh api --paginate --slurp \
    "repos/$repo/actions/runs/$run_id/attempts/$run_attempt/jobs?per_page=100" \
    | jq '{jobs: [.[] | .jobs[]]}' > "$jobs_file"
fi

if [ ! -f "$jobs_file" ]; then
  echo "找不到 jobs 文件：$jobs_file" >&2
  exit 2
fi

failed_jobs="$(jq -r '.jobs[] | select(.conclusion == "failure") | .name' "$jobs_file")"

if [ -z "$failed_jobs" ]; then
  echo '没有失败的 job，无需判定。'
  emit_summary 'none' '没有失败的 job。' '' ''
  exit 0
fi

# 注解：失败 job 的 check-run 注解。托管 runner 被回收时通常只有一条 143 收尾信息。
job_annotations() {
  local check_run_id="$1"
  if [ -n "$annotations_dir" ]; then
    if [ -f "$annotations_dir/$check_run_id.json" ]; then
      cat "$annotations_dir/$check_run_id.json"
    else
      printf '[]'
    fi
    return 0
  fi
  gh api "repos/$repo/check-runs/$check_run_id/annotations?per_page=100"
}

# 运行日志：shutdown 标记只出现在日志里，注解里拿不到。
ensure_logs() {
  if [ -n "$logs_dir" ]; then
    return 0
  fi
  need_workdir
  logs_dir="$workdir/logs"
  mkdir -p "$logs_dir"
  # 运行还在收尾时日志可能尚未归档，重试几次；仍取不到则退化为只看注解（更保守）。
  local attempt
  for attempt in 1 2 3 4 5 6; do
    if gh api "repos/$repo/actions/runs/$run_id/attempts/$run_attempt/logs" \
      > "$workdir/logs.zip" 2> /dev/null \
      && [ -s "$workdir/logs.zip" ] \
      && unzip -o -q "$workdir/logs.zip" -d "$logs_dir" 2> /dev/null \
      && [ -n "$(find "$logs_dir" -name '*.txt' -print -quit)" ]; then
      return 0
    fi
    sleep 10
  done
  rm -rf "$logs_dir"
  logs_dir=""
  echo '警告：没有取到运行日志，只能按注解判定。' >&2
}

# 该 job 在日志里的状态：shutdown（有中断标记）/ clean（有日志但无标记）/ missing（没找到日志）。
job_log_state() {
  local job_name="$1" target
  if [ -z "$logs_dir" ]; then
    printf 'missing'
    return 0
  fi
  target="$(find "$logs_dir" -name "*$(sanitize_name "$job_name").txt" -print -quit)"
  if [ -z "$target" ]; then
    target="$(find "$logs_dir" -name "*$(sanitize_name "$job_name")*" -print -quit)"
  fi
  if [ -z "$target" ]; then
    printf 'missing'
    return 0
  fi
  if grep -rqF "$shutdown_marker" "$target" 2> /dev/null; then
    printf 'shutdown'
  else
    printf 'clean'
  fi
}

# 注解里是否只剩「被 SIGTERM 收尾」这一种信息。没有注解时返回非零。
annotations_termination_only() {
  local annotations="$1" total others
  total="$(printf '%s' "$annotations" | jq -r 'length')"
  [ "$total" -gt 0 ] || return 1
  others="$(
    printf '%s' "$annotations" \
      | jq -r --arg termination "$termination_message" \
        '[.[] | .message | select(. != $termination)] | length'
  )"
  [ "$others" -eq 0 ]
}

ensure_logs

# 没有任何失败 job 的日志时（运行还没归档，或注入了空目录），判定只能看注解。
logs_found=0
verdict_kind="infra"
verdict_rows=""

while IFS= read -r job_name; do
  [ -n "$job_name" ] || continue
  check_run_id="$(
    jq -r --arg name "$job_name" \
      '[.jobs[] | select(.name == $name) | .check_run_url // ""] | first // "" | split("/") | last' \
      "$jobs_file"
  )"
  annotations='[]'
  if [ -n "$check_run_id" ]; then
    annotations="$(job_annotations "$check_run_id")"
  fi
  log_state="$(job_log_state "$job_name")"
  if [ "$log_state" != "missing" ]; then
    logs_found=1
  fi

  job_kind="real"
  case "$log_state" in
    shutdown)
      if [ "$(printf '%s' "$annotations" | jq -r 'length')" -eq 0 ] \
        || annotations_termination_only "$annotations"; then
        job_kind="infra"
        reason="日志含 runner shutdown 标记，且没有其它失败信息"
      else
        reason="日志含 runner shutdown 标记，但同时有真实失败注解"
      fi
      ;;
    missing)
      if annotations_termination_only "$annotations"; then
        job_kind="infra"
        reason="没找到该 job 日志；注解只有 exit code 143"
      else
        reason="没找到该 job 日志，且失败信息不属于中断"
      fi
      ;;
    *)
      reason="日志里没有 runner shutdown 标记"
      ;;
  esac

  if [ "$job_kind" = "real" ]; then
    verdict_kind="real"
  fi
  verdict_rows="${verdict_rows}| \`${job_name}\` | ${job_kind} | ${reason} |"$'\n'
  printf '%s\t%s\t%s\n' "$job_kind" "$job_name" "$reason"
done <<< "$failed_jobs"

logs_note=""
if [ "$logs_found" -eq 0 ]; then
  logs_note='没有可用运行日志，仅按注解判定（更保守）。'
fi

if [ "$verdict_kind" = "infra" ]; then
  verdict_text='所有失败 job 都只剩 runner 被回收（SIGTERM/143）的痕迹，属于基础设施中断，可安全重跑。'
else
  verdict_text='至少有一个失败 job 带有真实失败信息，需要人工排查，不应自动重跑。'
fi

emit_summary "$verdict_kind" "$verdict_text" "$verdict_rows" "$logs_note"
echo "判定：$verdict_kind — $verdict_text"
