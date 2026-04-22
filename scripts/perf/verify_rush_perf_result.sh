#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

source "${ROOT_DIR}/scripts/perf/lib/common.sh"
source "${ROOT_DIR}/scripts/perf/lib/mysql.sh"

SHOW_TIME_ID="${SHOW_TIME_ID:-30001}"
TICKET_CATEGORY_ID="${TICKET_CATEGORY_ID:-40001}"

MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-compose-mysql-1}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-123456}"

MYSQL_DB_PROGRAM="${MYSQL_DB_PROGRAM:-livepass_program}"
MYSQL_DB_ORDER="${MYSQL_DB_ORDER:-livepass_order}"

main() {
  perf_require_cmd docker

  local seat_sold_count
  local ticket_category_total
  local ticket_category_remain
  local seat_guard_count
  local order_ticket_count
  local expected_sold_count

  seat_sold_count="$(perf_mysql_query "${MYSQL_DB_PROGRAM}" "SELECT COUNT(*) FROM d_seat WHERE show_time_id = ${SHOW_TIME_ID} AND ticket_category_id = ${TICKET_CATEGORY_ID} AND seat_status = 3 AND status = 1")"
  ticket_category_total="$(perf_mysql_query "${MYSQL_DB_PROGRAM}" "SELECT total_number FROM d_ticket_category WHERE id = ${TICKET_CATEGORY_ID} AND show_time_id = ${SHOW_TIME_ID} AND status = 1 LIMIT 1")"
  ticket_category_remain="$(perf_mysql_query "${MYSQL_DB_PROGRAM}" "SELECT remain_number FROM d_ticket_category WHERE id = ${TICKET_CATEGORY_ID} AND show_time_id = ${SHOW_TIME_ID} AND status = 1 LIMIT 1")"
  seat_guard_count="$(perf_mysql_sum_matching_tables "${MYSQL_DB_ORDER}" "d_order_seat_guard" "show_time_id = ${SHOW_TIME_ID} AND status = 1")"
  order_ticket_count="$(perf_mysql_sum_matching_tables "${MYSQL_DB_ORDER}" "d_order_ticket_user%" "show_time_id = ${SHOW_TIME_ID} AND status = 1")"
  expected_sold_count=$((ticket_category_total - ticket_category_remain))

  printf 'showTimeId=%s\n' "${SHOW_TIME_ID}"
  printf 'ticketCategoryId=%s\n' "${TICKET_CATEGORY_ID}"
  printf 'ticketCategoryTotal=%s\n' "${ticket_category_total}"
  printf 'ticketCategoryRemain=%s\n' "${ticket_category_remain}"
  printf 'expectedSoldCount=%s\n' "${expected_sold_count}"
  printf 'seatSoldCount=%s\n' "${seat_sold_count}"
  printf 'orderSeatGuardCount=%s\n' "${seat_guard_count}"
  printf 'orderTicketCount=%s\n' "${order_ticket_count}"

  if [[ "${expected_sold_count}" != "${seat_sold_count}" ]]; then
    perf_fail "seat sold count mismatch: expected=${expected_sold_count} actual=${seat_sold_count}"
  fi

  if [[ "${seat_sold_count}" != "${seat_guard_count}" ]]; then
    perf_fail "seat guard count mismatch: sold=${seat_sold_count} guard=${seat_guard_count}"
  fi

  perf_log "rush perf result verification passed"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
