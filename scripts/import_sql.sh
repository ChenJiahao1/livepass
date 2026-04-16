#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-compose-mysql-1}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-${MYSQL_ROOT_PASSWORD:-123456}}"
IMPORT_DOMAINS="${IMPORT_DOMAINS:-user,program,order,pay}"

MYSQL_DB_USER="${MYSQL_DB_USER:-damai_user}"
MYSQL_DB_PROGRAM="${MYSQL_DB_PROGRAM:-damai_program}"
MYSQL_DB_ORDER="${MYSQL_DB_ORDER:-damai_order}"
MYSQL_DB_PAY="${MYSQL_DB_PAY:-damai_pay}"
MYSQL_DB_AGENTS="${MYSQL_DB_AGENTS:-damai_agents}"

USER_SQL_FILES=(
  "sql/user/d_user.sql"
  "sql/user/d_user_mobile.sql"
  "sql/user/d_user_email.sql"
  "sql/user/d_ticket_user.sql"
)

PROGRAM_SQL_FILES=(
  "sql/program/d_program_category.sql"
  "sql/program/d_program_group.sql"
  "sql/program/d_program.sql"
  "sql/program/d_program_show_time.sql"
  "sql/program/d_delay_task_outbox.sql"
  "sql/program/d_seat.sql"
  "sql/program/d_ticket_category.sql"
  "sql/program/dev_seed.sql"
)

ORDER_SQL_FILES=(
  "sql/order/sharding/d_order_shards.sql"
  "sql/order/sharding/d_order_ticket_user_shards.sql"
  "sql/order/sharding/d_order_user_guard.sql"
  "sql/order/sharding/d_order_viewer_guard.sql"
  "sql/order/sharding/d_order_seat_guard.sql"
  "sql/order/sharding/d_delay_task_outbox.sql"
)

PAY_SQL_FILES=(
  "sql/pay/d_pay_bill.sql"
  "sql/pay/d_refund_bill.sql"
)

AGENTS_SQL_FILES=(
  "sql/agents/agent_threads.sql"
  "sql/agents/agent_messages.sql"
  "sql/agents/agent_runs.sql"
)

log() {
  printf '[import-sql] %s\n' "$*"
}

fail() {
  printf '[import-sql] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  local name="$1"

  command -v "${name}" >/dev/null || fail "missing dependency: ${name}"
}

mysql_exec() {
  docker exec "${MYSQL_CONTAINER}" \
    mysql \
    --default-character-set=utf8mb4 \
    -u"${MYSQL_USER}" \
    "-p${MYSQL_PASSWORD}" \
    "$@"
}

ensure_database() {
  local database="$1"

  mysql_exec -e "CREATE DATABASE IF NOT EXISTS \`${database}\` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"
}

import_file() {
  local database="$1"
  local relative_path="$2"
  local absolute_path="${REPO_ROOT}/${relative_path}"

  [[ -f "${absolute_path}" ]] || fail "sql file not found: ${relative_path}"

  log "import ${relative_path} -> ${database}"
  docker exec -i "${MYSQL_CONTAINER}" \
    mysql \
    --default-character-set=utf8mb4 \
    -u"${MYSQL_USER}" \
    "-p${MYSQL_PASSWORD}" \
    "${database}" < "${absolute_path}"
}

import_domain() {
  local domain="$1"
  local database="$2"
  shift 2
  local files=("$@")

  ensure_database "${database}"
  for relative_path in "${files[@]}"; do
    import_file "${database}" "${relative_path}"
  done
}

main() {
  local domain raw_domain

  require_cmd docker

  docker ps --format '{{.Names}}' | grep -Fx "${MYSQL_CONTAINER}" >/dev/null || fail "mysql container not running: ${MYSQL_CONTAINER}"

  IFS=',' read -r -a requested_domains <<< "${IMPORT_DOMAINS}"
  for raw_domain in "${requested_domains[@]}"; do
    domain="${raw_domain//[[:space:]]/}"
    case "${domain}" in
      user)
        import_domain "user" "${MYSQL_DB_USER}" "${USER_SQL_FILES[@]}"
        ;;
      program)
        import_domain "program" "${MYSQL_DB_PROGRAM}" "${PROGRAM_SQL_FILES[@]}"
        ;;
      order)
        import_domain "order" "${MYSQL_DB_ORDER}" "${ORDER_SQL_FILES[@]}"
        ;;
      pay)
        import_domain "pay" "${MYSQL_DB_PAY}" "${PAY_SQL_FILES[@]}"
        ;;
      agents)
        import_domain "agents" "${MYSQL_DB_AGENTS}" "${AGENTS_SQL_FILES[@]}"
        ;;
      "")
        ;;
      *)
        fail "unknown domain: ${domain}"
        ;;
    esac
  done

  log "import completed"
}

main "$@"
