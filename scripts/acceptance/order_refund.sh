#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/order_checkout.sh"

REFUND_REASON="${REFUND_REASON:-行程变更}"
REFUND_MODE="${REFUND_MODE:-proactive}"
MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-compose-mysql-1}"
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-123456}"

REFUND_BILL_NO=""
REFUND_AMOUNT=""
ORDER_AMOUNT=""
PAY_BILL_NO=""
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

fetch_order_snapshot() {
  local expected_status="$1"
  local body

  body="$(curl_json "/order/get" "{\"orderNumber\":${ORDER_NUMBER}}" 1)"
  print_json "${body}"
  assert_json_filter "${body}" ".orderNumber == ${ORDER_NUMBER}" "unexpected order number in order detail response"
  if [[ -n "${expected_status}" ]]; then
    assert_json_filter "${body}" ".orderStatus == ${expected_status}" "unexpected order status in order detail response"
  fi
  assert_json_filter "${body}" '.orderTicketInfoVoList | length == 2' "expected exactly two order ticket snapshots"

  ORDER_AMOUNT="$(extract_required "${body}" '.orderPrice' 'orderPrice')"
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
  log "12/13 查询退款后订单详情"
  fetch_order_snapshot "4"
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

cancel_order_for_compensation() {
  local body

  log "10/14 取消未支付订单"
  body="$(curl_json "/order/cancel" "{\"orderNumber\":${ORDER_NUMBER}}" 1)"
  print_json "${body}"
  assert_json_filter "${body}" '.success == true' "expected cancel success"
}

inject_late_paid_bill() {
  local pay_time pay_bill_seed

  pay_time="$(date '+%Y-%m-%d %H:%M:%S')"
  pay_bill_seed=$((ORDER_NUMBER + 800000))
  PAY_BILL_NO="${pay_bill_seed}"

  log "12/14 注入晚到支付账单"
  docker_mysql_query damai_pay "INSERT INTO d_pay_bill (id, pay_bill_no, order_number, user_id, subject, channel, order_amount, pay_status, pay_time, create_time, edit_time, status) VALUES (${pay_bill_seed}, ${pay_bill_seed}, ${ORDER_NUMBER}, ${USER_ID}, '补偿退款验收支付', 'mock', ${ORDER_AMOUNT}, 2, '${pay_time}', '${pay_time}', '${pay_time}', 1) ON DUPLICATE KEY UPDATE pay_bill_no = VALUES(pay_bill_no), user_id = VALUES(user_id), subject = VALUES(subject), channel = VALUES(channel), order_amount = VALUES(order_amount), pay_status = 2, pay_time = VALUES(pay_time), edit_time = VALUES(edit_time), status = 1;" >/dev/null || fail "failed to inject paid bill"
  printf 'PAY_BILL_NO=%s\nORDER_AMOUNT=%s\n' "${PAY_BILL_NO}" "${ORDER_AMOUNT}"
}

check_compensation_payment() {
  local body

  log "13/14 触发支付晚到补偿退款"
  body="$(curl_json "/order/pay/check" "{\"orderNumber\":${ORDER_NUMBER}}" 1)"
  print_json "${body}"
  assert_json_filter "${body}" ".orderNumber == ${ORDER_NUMBER}" "unexpected order number in compensation pay check response"
  assert_json_filter "${body}" '.orderStatus == 4' "expected refunded order status in compensation pay check response"
  assert_json_filter "${body}" '.payStatus == 3' "expected payStatus=3 after compensation refund"
  assert_json_filter "${body}" ".payBillNo == ${PAY_BILL_NO}" "unexpected pay bill no in compensation pay check response"
}

check_compensation_storage() {
  local pay_status refund_row refund_status refund_amount

  log "14/14 校验补偿退款支付库状态"
  pay_status="$(docker_mysql_query damai_pay "SELECT pay_status FROM d_pay_bill WHERE order_number = ${ORDER_NUMBER};")" || fail "failed to query compensation pay bill"
  [[ "${pay_status}" == "3" ]] || fail "expected compensated d_pay_bill.pay_status=3, got ${pay_status}"

  refund_row="$(docker_mysql_query damai_pay "SELECT refund_bill_no, refund_amount, refund_status FROM d_refund_bill WHERE order_number = ${ORDER_NUMBER};")" || fail "failed to query compensation refund bill"
  [[ -n "${refund_row}" ]] || fail "missing compensation refund bill row for order ${ORDER_NUMBER}"

  REFUND_BILL_NO="$(printf '%s\n' "${refund_row}" | awk '{print $1}')"
  refund_amount="$(printf '%s\n' "${refund_row}" | awk '{print $2}')"
  refund_status="$(printf '%s\n' "${refund_row}" | awk '{print $3}')"

  [[ -n "${REFUND_BILL_NO}" && "${REFUND_BILL_NO}" -gt 0 ]] || fail "invalid compensation refund bill no from db: ${REFUND_BILL_NO}"
  [[ "${refund_status}" == "2" ]] || fail "expected compensation refund_status=2, got ${refund_status}"
  [[ "${refund_amount}" == "${ORDER_AMOUNT}" ]] || fail "compensation refund amount mismatch, order=${ORDER_AMOUNT} db=${refund_amount}"

  REFUND_AMOUNT="${refund_amount}"
  printf 'REFUND_BILL_NO=%s\nREFUND_AMOUNT=%s\n' "${REFUND_BILL_NO}" "${REFUND_AMOUNT}"
}

run_proactive_flow() {
  pay_order
  check_payment
  get_order

  REMAIN_BEFORE_REFUND="$(get_preorder_remain_number "${TICKET_CATEGORY_ID}")"
  log "退款前余量=${REMAIN_BEFORE_REFUND}"

  refund_order
  check_refund_payment
  check_refund_order_detail
  check_refund_storage
}

run_compensation_flow() {
  fetch_order_snapshot "1"
  cancel_order_for_compensation
  fetch_order_snapshot "2"
  inject_late_paid_bill
  check_compensation_payment
  fetch_order_snapshot "4"
  check_compensation_storage
}

main() {
  preflight
  require_cmd docker

  log "BASE_URL=${BASE_URL}"
  log "CHANNEL_CODE=${CHANNEL_CODE}"
  log "PROGRAM_ID=${PROGRAM_ID}"
  log "MOBILE=${MOBILE}"
  log "MYSQL_CONTAINER=${MYSQL_CONTAINER}"
  log "REFUND_MODE=${REFUND_MODE}"

  register_user
  login_user
  add_ticket_user "${TICKET_USER_A_NAME}" "${TICKET_USER_A_ID_NUMBER}"
  add_ticket_user "${TICKET_USER_B_NAME}" "${TICKET_USER_B_ID_NUMBER}"
  list_ticket_users
  fetch_preorder
  create_order
  case "${REFUND_MODE}" in
    proactive)
      run_proactive_flow
      log "退款主路径执行成功"
      printf 'ORDER_NUMBER=%s\nREFUND_BILL_NO=%s\nREFUND_AMOUNT=%s\nREMAIN_BEFORE_REFUND=%s\nREMAIN_AFTER_REFUND=%s\n' \
        "${ORDER_NUMBER}" \
        "${REFUND_BILL_NO}" \
        "${REFUND_AMOUNT}" \
        "${REMAIN_BEFORE_REFUND}" \
        "${REMAIN_AFTER_REFUND}"
      ;;
    compensation)
      run_compensation_flow
      log "退款补偿路径执行成功"
      printf 'ORDER_NUMBER=%s\nPAY_BILL_NO=%s\nREFUND_BILL_NO=%s\nORDER_AMOUNT=%s\nREFUND_AMOUNT=%s\n' \
        "${ORDER_NUMBER}" \
        "${PAY_BILL_NO}" \
        "${REFUND_BILL_NO}" \
        "${ORDER_AMOUNT}" \
        "${REFUND_AMOUNT}"
      ;;
    *)
      fail "unsupported REFUND_MODE=${REFUND_MODE}, expected proactive or compensation"
      ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
