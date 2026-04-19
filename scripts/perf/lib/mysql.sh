#!/usr/bin/env bash
set -euo pipefail

perf_mysql_exec() {
  docker exec -i "${MYSQL_CONTAINER}" \
    mysql \
    --default-character-set=utf8mb4 \
    -u"${MYSQL_USER}" \
    "-p${MYSQL_PASSWORD}" \
    "$@"
}

perf_mysql_query() {
  local db="$1"
  local sql="$2"

  perf_mysql_exec -N -s "${db}" -e "${sql}"
}

perf_mysql_import_file() {
  local db="$1"
  local sql_file="$2"

  [[ -f "${sql_file}" ]] || perf_fail "sql file not found: ${sql_file}"
  perf_mysql_exec "${db}" < "${sql_file}"
}

perf_mysql_sum_matching_tables() {
  local db="$1"
  local table_like="$2"
  local where_clause="$3"
  local total=0
  local table_names=()
  local table_name
  local count

  mapfile -t table_names < <(perf_mysql_query "${db}" "SELECT table_name FROM information_schema.tables WHERE table_schema = '${db}' AND table_name LIKE '${table_like}' ORDER BY table_name")

  for table_name in "${table_names[@]}"; do
    [[ -n "${table_name}" ]] || continue
    count="$(perf_mysql_query "${db}" "SELECT COUNT(*) FROM \`${table_name}\` WHERE ${where_clause}")"
    count="${count:-0}"
    total=$((total + count))
  done

  printf '%s\n' "${total}"
}
