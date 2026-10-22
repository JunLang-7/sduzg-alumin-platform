#!/usr/bin/env bash
# classify-run-failure.sh 的回归测试。
#
# 覆盖的重点：#217 里 Nightly 的失败混着「runner 被宿主回收（SIGTERM/143）」，
# 判定必须只在「所有失败 job 都只剩中断痕迹」时才算基础设施中断，
# 一旦出现真实失败信息（例如 npm audit 高危、--- FAIL:）就必须判成 real，
# 否则自动重跑会把真实问题掩盖成绿灯。
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
classifier="$script_dir/classify-run-failure.sh"

workdir="$(mktemp -d)"
cleanup() {
  rm -rf "$workdir"
  return 0
}
trap cleanup EXIT

shutdown_marker='##[error]The runner has received a shutdown signal. This can happen when the runner service is stopped, or a manually started runner is canceled.'
termination_annotation='Process completed with exit code 143.'

failures=0

# 写一份 jobs JSON：入参是 "job 名|check_run_id|结论"。
write_jobs() {
  local file="$1"
  shift
  printf '{"jobs":[' > "$file"
  local first=1 spec name check_run_id conclusion
  for spec in "$@"; do
    IFS='|' read -r name check_run_id conclusion <<< "$spec"
    [ "$first" -eq 1 ] || printf ',' >> "$file"
    first=0
    printf '{"name":"%s","conclusion":"%s","check_run_url":"https://api.github.com/repos/o/r/check-runs/%s"}' \
      "$name" "$conclusion" "$check_run_id" >> "$file"
  done
  printf ']}' >> "$file"
}

# 写注解文件。
write_annotations() {
  local dir="$1" check_run_id="$2"
  shift 2
  mkdir -p "$dir"
  printf '[' > "$dir/$check_run_id.json"
  local first=1 message
  for message in "$@"; do
    [ "$first" -eq 1 ] || printf ',' >> "$dir/$check_run_id.json"
    first=0
    printf '{"message":"%s"}' "$message" >> "$dir/$check_run_id.json"
  done
  printf ']' >> "$dir/$check_run_id.json"
}

# 写一份 job 日志。
write_job_log() {
  local dir="$1" job_name="$2"
  shift 2
  mkdir -p "$dir"
  local file="$dir/6_$(printf '%s' "$job_name" | tr '/' '_').txt"
  : > "$file"
  local line
  for line in "$@"; do
    printf '%s\n' "$line" >> "$file"
  done
}

# 跑一次判定，返回 kind。
run_classifier() {
  local name="$1" jobs_file="$2" annotations_dir="$3" logs_dir="$4"
  local run_dir="$workdir/run-$name"
  mkdir -p "$run_dir"
  : > "$run_dir/output"
  : > "$run_dir/summary"
  GITHUB_STEP_SUMMARY="$run_dir/summary" \
    GITHUB_OUTPUT="$run_dir/output" \
    GITHUB_RUN_ID=39675304357 GITHUB_RUN_ATTEMPT=1 GITHUB_REPOSITORY=o/r \
    bash "$classifier" \
    --jobs-file "$jobs_file" \
    --annotations-dir "$annotations_dir" \
    --logs-dir "$logs_dir" \
    > "$run_dir/stdout" 2>&1
  sed -n 's/^kind=//p' "$run_dir/output" | tail -1
}

expect_kind() {
  local what="$1" expected="$2" actual="$3" case_name="$4"
  if [ "$actual" = "$expected" ]; then
    echo "  ✓ $what"
    return
  fi
  echo "  ✗ ${what}（预期 ${expected}，实际 ${actual:-空}）" >&2
  sed 's/^/    /' "$workdir/run-$case_name/stdout" >&2 || true
  failures=$((failures + 1))
}

expect_summary() {
  local what="$1" needle="$2" case_name="$3"
  if grep -qF "$needle" "$workdir/run-$case_name/summary"; then
    echo "  ✓ $what"
    return
  fi
  echo "  ✗ ${what}（摘要缺少：${needle}）" >&2
  sed 's/^/    /' "$workdir/run-$case_name/summary" >&2 || true
  failures=$((failures + 1))
}

echo 'classify-run-failure.sh 回归测试'

# 1. 典型的 runner 被回收：两个 job 都只有 shutdown 标记与 143 注解。
infra_jobs="$workdir/jobs-infra.json"
write_jobs "$infra_jobs" \
  'Full quality gate / Backend quality|7001|failure' \
  'Full quality gate / Integration and migration|7002|failure'
infra_ann="$workdir/ann-infra"
write_annotations "$infra_ann" 7001 "$termination_annotation"
write_annotations "$infra_ann" 7002 "$termination_annotation"
infra_logs="$workdir/logs-infra"
write_job_log "$infra_logs" 'Full quality gate / Backend quality' \
  'ok  	example.com/internal/router	1.190s' "$shutdown_marker" \
  '##[error]Process completed with exit code 143.'
write_job_log "$infra_logs" 'Full quality gate / Integration and migration' \
  "$shutdown_marker" '##[error]Process completed with exit code 143.'
kind="$(run_classifier infra "$infra_jobs" "$infra_ann" "$infra_logs")"
expect_kind '全部 job 被回收时判定为 infra' infra "$kind" infra
expect_summary '摘要写明可安全重跑' '可安全重跑' infra

# 2. 真实测试失败：注解与日志都不是中断痕迹。
real_jobs="$workdir/jobs-real.json"
write_jobs "$real_jobs" 'Full quality gate / Backend quality|7003|failure'
real_ann="$workdir/ann-real"
write_annotations "$real_ann" 7003 'Process completed with exit code 1.'
real_logs="$workdir/logs-real"
write_job_log "$real_logs" 'Full quality gate / Backend quality' \
  '--- FAIL: TestAlumniExport (0.02s)' 'FAIL	example.com/internal/service	1.2s' \
  '##[error]Process completed with exit code 1.'
kind="$(run_classifier real "$real_jobs" "$real_ann" "$real_logs")"
expect_kind '有 FAIL 时判定为 real' real "$kind" real
expect_summary '摘要要求人工排查' '不应自动重跑' real

# 3. infra 与 real 混合：整体必须按 real 处理。
mixed_jobs="$workdir/jobs-mixed.json"
write_jobs "$mixed_jobs" \
  'Full quality gate / Backend quality|7004|failure' \
  'Full quality gate / Security|7005|failure'
mixed_ann="$workdir/ann-mixed"
write_annotations "$mixed_ann" 7004 "$termination_annotation"
write_annotations "$mixed_ann" 7005 'Process completed with exit code 1.'
mixed_logs="$workdir/logs-mixed"
write_job_log "$mixed_logs" 'Full quality gate / Backend quality' "$shutdown_marker"
write_job_log "$mixed_logs" 'Full quality gate / Security' \
  '# npm audit report' '1 high severity vulnerability'
kind="$(run_classifier mixed "$mixed_jobs" "$mixed_ann" "$mixed_logs")"
expect_kind 'infra 与 real 混合时判定为 real' real "$kind" mixed

# 4. 中断标记与真实失败注解同时存在（例如 job 收尾时已经在报 audit 失败）。
both_jobs="$workdir/jobs-both.json"
write_jobs "$both_jobs" 'Full quality gate / Security|7006|failure'
both_ann="$workdir/ann-both"
write_annotations "$both_ann" 7006 "$termination_annotation" \
  '1 high severity vulnerability'
both_logs="$workdir/logs-both"
write_job_log "$both_logs" 'Full quality gate / Security' \
  '# npm audit report' '1 high severity vulnerability' "$shutdown_marker"
kind="$(run_classifier both "$both_jobs" "$both_ann" "$both_logs")"
expect_kind '中断标记伴随真实失败注解时判定为 real' real "$kind" both

# 5. 日志还没归档：退化为只看注解。143-only 仍按 infra，其它一律 real。
nologs_jobs="$workdir/jobs-nologs.json"
write_jobs "$nologs_jobs" \
  'Full quality gate / Backend quality|7007|failure' \
  'Full quality gate / Integration and migration|7008|failure'
nologs_ann="$workdir/ann-nologs"
write_annotations "$nologs_ann" 7007 "$termination_annotation"
write_annotations "$nologs_ann" 7008 "$termination_annotation"
nologs_dir="$workdir/logs-empty"
mkdir -p "$nologs_dir"
kind="$(run_classifier nologs "$nologs_jobs" "$nologs_ann" "$nologs_dir")"
expect_kind '没有日志但注解只有 143 时判定为 infra' infra "$kind" nologs
expect_summary '无日志时摘要注明只按注解判定' '仅按注解判定' nologs

nologs_real_jobs="$workdir/jobs-nologs-real.json"
write_jobs "$nologs_real_jobs" 'Full quality gate / Backend quality|7009|failure'
nologs_real_ann="$workdir/ann-nologs-real"
write_annotations "$nologs_real_ann" 7009 'Process completed with exit code 1.'
kind="$(run_classifier nologs-real "$nologs_real_jobs" "$nologs_real_ann" "$nologs_dir")"
expect_kind '没有日志且注解不是 143 时判定为 real' real "$kind" nologs-real

# 6. 没有失败 job 时不误判。
none_jobs="$workdir/jobs-none.json"
write_jobs "$none_jobs" 'Full quality gate / Backend quality|7010|success'
kind="$(run_classifier none "$none_jobs" "$infra_ann" "$infra_logs")"
expect_kind '没有失败 job 时判定为 none' none "$kind" none

if [ "$failures" -ne 0 ]; then
  echo "回归测试失败：$failures 项" >&2
  exit 1
fi
echo '回归测试通过'
