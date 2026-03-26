#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./order_checkout.sh
source "${SCRIPT_DIR}/order_checkout.sh"

MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-compose-mysql-1}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-123456}"
ORDER_CLOSE_CONFIG="${ORDER_CLOSE_CONFIG:-jobs/order-close/etc/order-close.yaml}"
ORDER_CLOSE_WAIT_SECONDS="${ORDER_CLOSE_WAIT_SECONDS:-30}"
INVENTORY_FAIL_TICKET_CATEGORY_ID="${INVENTORY_FAIL_TICKET_CATEGORY_ID:-}"

INVENTORY_MUTATED_SEAT_IDS=""
INVENTORY_MUTATED_CATEGORY_ID=""

mysql_exec_db() {
  local db="$1"
  local sql="$2"

  docker exec "${MYSQL_CONTAINER}" mysql -N -B -uroot "-p${MYSQL_PASSWORD}" "${db}" -e "${sql}"
}

restore_inventory_if_needed() {
  if [[ -z "${INVENTORY_MUTATED_SEAT_IDS}" ]]; then
    return
  fi

  mysql_exec_db "damai_program" "UPDATE d_seat SET seat_status = 1, freeze_token = NULL, freeze_expire_time = NULL, edit_time = NOW() WHERE id IN (${INVENTORY_MUTATED_SEAT_IDS})"
  INVENTORY_MUTATED_SEAT_IDS=""
  INVENTORY_MUTATED_CATEGORY_ID=""
}

cleanup() {
  restore_inventory_if_needed || true
}

trap cleanup EXIT

failure_preflight() {
  preflight
  require_cmd docker
  require_cmd go
}

resolve_inventory_failure_category() {
  local preorder_body="$1"
  local category_id

  if [[ -n "${INVENTORY_FAIL_TICKET_CATEGORY_ID}" ]]; then
    printf '%s' "${preorder_body}" | jq -e --arg ticket_category_id "${INVENTORY_FAIL_TICKET_CATEGORY_ID}" '.ticketCategoryVoList[] | select((.id | tostring) == $ticket_category_id)' >/dev/null || fail "configured INVENTORY_FAIL_TICKET_CATEGORY_ID not found in preorder response"
    printf '%s' "${INVENTORY_FAIL_TICKET_CATEGORY_ID}"
    return
  fi

  category_id="$(
    printf '%s' "${preorder_body}" | jq -er '.ticketCategoryVoList[1].id // .ticketCategoryVoList[0].id'
  )" || fail "missing ticket category for inventory failure scenario"

  printf '%s' "${category_id}"
}

force_inventory_insufficient() {
  local ticket_category_id="$1"
  local seat_ids

  seat_ids="$(mysql_exec_db "damai_program" "SELECT COALESCE(GROUP_CONCAT(id ORDER BY id SEPARATOR ','), '') FROM d_seat WHERE status = 1 AND program_id = ${PROGRAM_ID} AND ticket_category_id = ${ticket_category_id} AND seat_status = 1")"
  if [[ -z "${seat_ids}" ]]; then
    fail "no available seats to mutate for ticketCategoryId=${ticket_category_id}"
  fi

  INVENTORY_MUTATED_SEAT_IDS="${seat_ids}"
  INVENTORY_MUTATED_CATEGORY_ID="${ticket_category_id}"
  mysql_exec_db "damai_program" "UPDATE d_seat SET seat_status = 3, freeze_token = NULL, freeze_expire_time = NULL, edit_time = NOW() WHERE id IN (${seat_ids})"
}

run_order_close_once() {
  local log_file pid started=0

  log "触发一次 order-close 任务"
  log_file="$(mktemp)"
  go run jobs/order-close/order_close.go -f "${ORDER_CLOSE_CONFIG}" >"${log_file}" 2>&1 &
  pid=$!

  for ((i = 0; i < ORDER_CLOSE_WAIT_SECONDS; i++)); do
    if grep -q "Starting order-close job" "${log_file}"; then
      started=1
      break
    fi
    if ! kill -0 "${pid}" 2>/dev/null; then
      break
    fi
    sleep 1
  done

  if kill -0 "${pid}" 2>/dev/null; then
    kill "${pid}" 2>/dev/null || true
  fi
  wait "${pid}" 2>/dev/null || true

  if grep -q "initial order-close run failed" "${log_file}"; then
    cat "${log_file}" >&2
    rm -f "${log_file}"
    fail "order-close initial run failed"
  fi
  if [[ "${started}" -ne 1 ]]; then
    cat "${log_file}" >&2
    rm -f "${log_file}"
    fail "order-close did not finish initial run within ${ORDER_CLOSE_WAIT_SECONDS}s"
  fi

  rm -f "${log_file}"
}

scenario_duplicate_ticket_user() {
  local body

  log "失败场景 1/4 重复观演人"
  body="$(curl_json_expect_error "/ticket/user/add" "{\"userId\":${USER_ID},\"relName\":\"${TICKET_USER_A_NAME}\",\"idType\":1,\"idNumber\":\"${TICKET_USER_A_ID_NUMBER}\"}" "ticket user already exists")"
  print_json "${body}"
}

scenario_inventory_insufficient() {
  local preorder_body ticket_category_id body

  log "失败场景 2/4 库存不足"
  preorder_body="$(curl_json "/program/preorder/detail" "{\"id\":${PROGRAM_ID}}")"
  ticket_category_id="$(resolve_inventory_failure_category "${preorder_body}")"
  force_inventory_insufficient "${ticket_category_id}"

  body="$(curl_json "/program/preorder/detail" "{\"id\":${PROGRAM_ID}}")"
  print_json "${body}"
  assert_json_filter "${body}" "([.ticketCategoryVoList[] | select(.id == ${ticket_category_id}) | .remainNumber] | first) == 0" "expected remainNumber=0 for inventory failure category"

  body="$(curl_json_expect_error "/order/create" "{\"programId\":${PROGRAM_ID},\"ticketCategoryId\":${ticket_category_id},\"ticketUserIds\":[${TICKET_USER_ID_1},${TICKET_USER_ID_2}],\"distributionMode\":\"express\",\"takeTicketMode\":\"paper\"}" "seat inventory insufficient" 1)"
  print_json "${body}"

  restore_inventory_if_needed
}

scenario_cancel_order() {
  local body

  log "失败场景 3/4 取消订单后不可继续支付"
  fetch_preorder
  create_order
  wait_order_visible

  body="$(curl_json "/order/cancel" "{\"orderNumber\":${ORDER_NUMBER}}" 1)"
  print_json "${body}"
  assert_json_filter "${body}" 'has("success") and .success == true' "/order/cancel success != true"

  body="$(curl_json "/order/get" "{\"orderNumber\":${ORDER_NUMBER}}" 1)"
  print_json "${body}"
  assert_json_filter "${body}" '.orderStatus == 2' "expected cancelled order status after /order/cancel"

  body="$(curl_json_expect_error "/order/pay" "{\"orderNumber\":${ORDER_NUMBER},\"subject\":\"${SUBJECT}\",\"channel\":\"${PAY_CHANNEL}\"}" "order status invalid" 1)"
  print_json "${body}"
}

scenario_close_expired_order() {
  local remain_before remain_after body

  log "失败场景 4/4 超时关单"
  fetch_preorder
  remain_before="$(get_preorder_remain_number "${TICKET_CATEGORY_ID}")"
  create_order
  wait_order_visible

  mysql_exec_db "damai_order" "UPDATE d_order_00 SET order_expire_time = DATE_SUB(NOW(), INTERVAL 5 MINUTE), edit_time = NOW() WHERE order_number = ${ORDER_NUMBER}; UPDATE d_order_01 SET order_expire_time = DATE_SUB(NOW(), INTERVAL 5 MINUTE), edit_time = NOW() WHERE order_number = ${ORDER_NUMBER}"
  run_order_close_once

  body="$(curl_json "/order/get" "{\"orderNumber\":${ORDER_NUMBER}}" 1)"
  print_json "${body}"
  assert_json_filter "${body}" '.orderStatus == 2' "expected cancelled order status after order-close"

  body="$(curl_json_expect_error "/order/pay" "{\"orderNumber\":${ORDER_NUMBER},\"subject\":\"${SUBJECT}\",\"channel\":\"${PAY_CHANNEL}\"}" "order status invalid" 1)"
  print_json "${body}"

  remain_after="$(get_preorder_remain_number "${TICKET_CATEGORY_ID}")"
  if [[ "${remain_after}" != "${remain_before}" ]]; then
    fail "expected inventory restored after order-close, before=${remain_before} after=${remain_after}"
  fi
}

main() {
  failure_preflight

  log "BASE_URL=${BASE_URL}"
  log "CHANNEL_CODE=${CHANNEL_CODE}"
  log "PROGRAM_ID=${PROGRAM_ID}"
  log "MOBILE=${MOBILE}"
  log "MYSQL_CONTAINER=${MYSQL_CONTAINER}"

  register_user
  login_user
  add_ticket_user "${TICKET_USER_A_NAME}" "${TICKET_USER_A_ID_NUMBER}"
  add_ticket_user "${TICKET_USER_B_NAME}" "${TICKET_USER_B_ID_NUMBER}"
  list_ticket_users
  fetch_preorder

  scenario_duplicate_ticket_user
  scenario_inventory_insufficient
  scenario_cancel_order
  scenario_close_expired_order

  log "失败分支执行成功"
}

main "$@"
