#!/usr/bin/env bash
set -euo pipefail

perf_log() {
  printf '[perf] %s\n' "$*"
}

perf_fail() {
  printf '[perf] ERROR: %s\n' "$*" >&2
  exit 1
}

perf_require_cmd() {
  local name="$1"
  command -v "${name}" >/dev/null 2>&1 || perf_fail "missing dependency: ${name}"
}

perf_ensure_dir() {
  local dir="$1"
  mkdir -p "${dir}"
}

perf_json_escape() {
  local value="${1:-}"
  printf '%s' "${value}" | jq -Rr @json
}

perf_calc_ticket_count() {
  local idx="$1"

  awk \
    -v seed="${RANDOM_SEED}" \
    -v current_idx="${idx}" \
    -v min_count="${MIN_TICKET_COUNT}" \
    -v max_count="${MAX_TICKET_COUNT}" \
    'BEGIN {
      srand(seed + current_idx)
      printf "%d", int(rand() * (max_count - min_count + 1)) + min_count
    }'
}
