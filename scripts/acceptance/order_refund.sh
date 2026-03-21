#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/order_checkout.sh"

REFUND_REASON="${REFUND_REASON:-行程变更}"
MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-compose-mysql-1}"
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-123456}"

REFUND_BILL_NO=""
REFUND_AMOUNT=""
REMAIN_BEFORE_REFUND=""
REMAIN_AFTER_REFUND=""

log() {
  printf '[order-refund] %s\n' "$*"
}

fail() {
  printf '[order-refund] ERROR: %s\n' "$*" >&2
  exit 1
}

docker_mysql_query() {
  local database="$1"
  local sql="$2"

  docker exec "${MYSQL_CONTAINER}" mysql -N -uroot -p"${MYSQL_ROOT_PASSWORD}" "${database}" -e "${sql}"
}

refund_order() {
  local body

  log "10/13 发起退款"
  body="$(curl_json "/order/refund" "{\"orderNumber\":${ORDER_NUMBER},\"reason\":\"${REFUND_REASON}\"}" 1)"
  print_json "${body}"
  assert_json_filter "${body}" ".orderNumber == ${ORDER_NUMBER}" "unexpected order number in refund response"
  assert_json_filter "${body}" '.orderStatus == 4' "expected orderStatus=4 after refund"
  assert_json_filter "${body}" '.refundAmount > 0' "expected refundAmount > 0"
  assert_json_filter "${body}" '.refundPercent > 0' "expected refundPercent > 0"
  assert_json_filter "${body}" '.refundBillNo > 0' "expected refundBillNo > 0"
  assert_json_filter "${body}" '.refundTime != ""' "expected refundTime not empty"

  REFUND_BILL_NO="$(extract_required "${body}" '.refundBillNo' 'refundBillNo')"
  REFUND_AMOUNT="$(extract_required "${body}" '.refundAmount' 'refundAmount')"
  printf 'REFUND_BILL_NO=%s\nREFUND_AMOUNT=%s\n' "${REFUND_BILL_NO}" "${REFUND_AMOUNT}"
}

check_refund_payment() {
  local body

  log "11/13 查询退款后支付状态"
  body="$(curl_json "/order/pay/check" "{\"orderNumber\":${ORDER_NUMBER}}" 1)"
  print_json "${body}"
  assert_json_filter "${body}" ".orderNumber == ${ORDER_NUMBER}" "unexpected order number in pay check response"
  assert_json_filter "${body}" '.orderStatus == 4' "expected refunded order status in pay check response"
  assert_json_filter "${body}" '.payStatus == 3' "expected payStatus=3 in pay check response"
}

check_refund_order_detail() {
  local body

  log "12/13 查询退款后订单详情"
  body="$(curl_json "/order/get" "{\"orderNumber\":${ORDER_NUMBER}}" 1)"
  print_json "${body}"
  assert_json_filter "${body}" ".orderNumber == ${ORDER_NUMBER}" "unexpected order number in refund order detail response"
  assert_json_filter "${body}" '.orderStatus == 4' "expected refunded order status in order detail response"
  assert_json_filter "${body}" '.orderTicketInfoVoList | length == 2' "expected exactly two order ticket snapshots after refund"
}

check_refund_storage() {
  local pay_status refund_row refund_status refund_amount

  log "13/13 校验支付库与库存回滚"
  pay_status="$(docker_mysql_query damai_pay "SELECT pay_status FROM d_pay_bill WHERE order_number = ${ORDER_NUMBER};")" || fail "failed to query d_pay_bill"
  [[ "${pay_status}" == "3" ]] || fail "expected d_pay_bill.pay_status=3, got ${pay_status}"

  refund_row="$(docker_mysql_query damai_pay "SELECT refund_bill_no, refund_amount, refund_status FROM d_refund_bill WHERE order_number = ${ORDER_NUMBER};")" || fail "failed to query d_refund_bill"
  [[ -n "${refund_row}" ]] || fail "missing refund bill row for order ${ORDER_NUMBER}"

  REFUND_BILL_NO="$(printf '%s\n' "${refund_row}" | awk '{print $1}')"
  refund_amount="$(printf '%s\n' "${refund_row}" | awk '{print $2}')"
  refund_status="$(printf '%s\n' "${refund_row}" | awk '{print $3}')"

  [[ -n "${REFUND_BILL_NO}" && "${REFUND_BILL_NO}" -gt 0 ]] || fail "invalid refund bill no from db: ${REFUND_BILL_NO}"
  [[ "${refund_status}" == "2" ]] || fail "expected refund_status=2, got ${refund_status}"
  [[ "${refund_amount}" == "${REFUND_AMOUNT}" ]] || fail "refund amount mismatch, api=${REFUND_AMOUNT} db=${refund_amount}"

  REMAIN_AFTER_REFUND="$(get_preorder_remain_number "${TICKET_CATEGORY_ID}")"
  [[ "${REMAIN_AFTER_REFUND}" -eq $((REMAIN_BEFORE_REFUND + 2)) ]] || fail "expected remain_after_refund=${REMAIN_BEFORE_REFUND}+2, got ${REMAIN_AFTER_REFUND}"

  printf 'REMAIN_BEFORE_REFUND=%s\nREMAIN_AFTER_REFUND=%s\n' "${REMAIN_BEFORE_REFUND}" "${REMAIN_AFTER_REFUND}"
}

main() {
  preflight
  require_cmd docker

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
  create_order
  pay_order
  check_payment
  get_order

  REMAIN_BEFORE_REFUND="$(get_preorder_remain_number "${TICKET_CATEGORY_ID}")"
  log "退款前余量=${REMAIN_BEFORE_REFUND}"

  refund_order
  check_refund_payment
  check_refund_order_detail
  check_refund_storage

  log "退款主路径执行成功"
  printf 'ORDER_NUMBER=%s\nREFUND_BILL_NO=%s\nREFUND_AMOUNT=%s\nREMAIN_BEFORE_REFUND=%s\nREMAIN_AFTER_REFUND=%s\n' \
    "${ORDER_NUMBER}" \
    "${REFUND_BILL_NO}" \
    "${REFUND_AMOUNT}" \
    "${REMAIN_BEFORE_REFUND}" \
    "${REMAIN_AFTER_REFUND}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
