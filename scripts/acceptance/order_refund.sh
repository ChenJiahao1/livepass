#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./order_checkout.sh
source "${SCRIPT_DIR}/order_checkout.sh"

REFUND_REASON="${REFUND_REASON:-行程变更}"
MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-compose-mysql-1}"
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-123456}"
REFUND_BLOCK_REASON="${REFUND_BLOCK_REASON:-秒杀活动进行中，暂不支持退票}"

REMAIN_BEFORE_REFUND=""
REMAIN_AFTER_REFUND=""
ORIGINAL_RUSH_SALE_OPEN_TIME=""
ORIGINAL_RUSH_SALE_END_TIME=""
RUSH_WINDOW_MUTATED=0

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

restore_rush_sale_window_if_needed() {
  if [[ "${RUSH_WINDOW_MUTATED}" != "1" ]]; then
    return
  fi

  docker_mysql_query damai_program "UPDATE d_program_show_time SET rush_sale_open_time = '${ORIGINAL_RUSH_SALE_OPEN_TIME}', rush_sale_end_time = '${ORIGINAL_RUSH_SALE_END_TIME}', edit_time = NOW() WHERE id = ${SHOW_TIME_ID};" >/dev/null || true
  redis-cli -h 127.0.0.1 -p 6379 DEL "cache:dProgramShowTime:id:${SHOW_TIME_ID}" >/dev/null || true
}

cleanup() {
  restore_rush_sale_window_if_needed
}

trap cleanup EXIT

force_rush_sale_window_now() {
  local current_window open_at end_at

  log "将场次秒杀窗口调整到当前时间"
  current_window="$(docker_mysql_query damai_program "SELECT DATE_FORMAT(rush_sale_open_time, '%Y-%m-%d %H:%i:%s'), DATE_FORMAT(rush_sale_end_time, '%Y-%m-%d %H:%i:%s') FROM d_program_show_time WHERE id = ${SHOW_TIME_ID};")" || fail "failed to load current rush sale window"
  ORIGINAL_RUSH_SALE_OPEN_TIME="${current_window%%$'\t'*}"
  ORIGINAL_RUSH_SALE_END_TIME="${current_window#*$'\t'}"
  open_at="$(date -d '-5 minutes' '+%F %T')"
  end_at="$(date -d '+30 minutes' '+%F %T')"
  docker_mysql_query damai_program "UPDATE d_program_show_time SET rush_sale_open_time = '${open_at}', rush_sale_end_time = '${end_at}', edit_time = '${end_at}' WHERE id = ${SHOW_TIME_ID};" >/dev/null || fail "failed to update rush sale window"
  redis-cli -h 127.0.0.1 -p 6379 DEL "cache:dProgramShowTime:id:${SHOW_TIME_ID}" >/dev/null || fail "failed to clear show time cache"
  RUSH_WINDOW_MUTATED=1
}

assert_no_refund_bill() {
  local refund_count

  refund_count="$(docker_mysql_query damai_pay "SELECT COUNT(1) FROM d_refund_bill WHERE order_number = ${ORDER_NUMBER};")" || fail "failed to query refund bill"
  [[ "${refund_count}" == "0" ]] || fail "expected no refund bill for blocked refund, got ${refund_count}"
}

main() {
  preflight
  require_cmd docker
  require_cmd redis-cli

  log "BASE_URL=${BASE_URL}"
  log "SHOW_TIME_ID=${SHOW_TIME_ID}"
  log "MOBILE=${MOBILE}"
  log "MYSQL_CONTAINER=${MYSQL_CONTAINER}"

  register_user
  login_user
  add_ticket_user "${TICKET_USER_A_NAME}" "${TICKET_USER_A_ID_NUMBER}"
  add_ticket_user "${TICKET_USER_B_NAME}" "${TICKET_USER_B_ID_NUMBER}"
  list_ticket_users
  fetch_preorder
  create_purchase_token
  create_order
  poll_order_until_done
  assert_poll_status "3"
  pay_order
  check_payment

  REMAIN_BEFORE_REFUND="$(get_preorder_remain_number "${TICKET_CATEGORY_ID}")"
  force_rush_sale_window_now

  log "10/12 发起退款，预期被秒杀窗口拒绝"
  body="$(curl_json_expect_error "/order/refund" "{\"orderNumber\":${ORDER_NUMBER},\"reason\":\"${REFUND_REASON}\"}" "${REFUND_BLOCK_REASON}" 1)"
  print_json "${body}"

  log "11/12 校验订单和支付状态未变化"
  body="$(fetch_order_snapshot "3" "2")"
  print_json "${body}"
  body="$(curl_json "/order/pay/check" "{\"orderNumber\":${ORDER_NUMBER}}" 1)"
  print_json "${body}"
  assert_json_filter "${body}" '.orderStatus == 3' "expected orderStatus=3 after blocked refund"
  assert_json_filter "${body}" '.payStatus == 2' "expected payStatus=2 after blocked refund"

  log "12/12 校验支付库与库存未发生退款副作用"
  pay_status="$(docker_mysql_query damai_pay "SELECT pay_status FROM d_pay_bill WHERE order_number = ${ORDER_NUMBER};")" || fail "failed to query pay bill"
  [[ "${pay_status}" == "2" ]] || fail "expected d_pay_bill.pay_status=2 after blocked refund, got ${pay_status}"
  assert_no_refund_bill

  REMAIN_AFTER_REFUND="$(get_preorder_remain_number "${TICKET_CATEGORY_ID}")"
  [[ "${REMAIN_AFTER_REFUND}" == "${REMAIN_BEFORE_REFUND}" ]] || fail "expected remainNumber unchanged after blocked refund, before=${REMAIN_BEFORE_REFUND} after=${REMAIN_AFTER_REFUND}"

  log "退款窗口拦截执行成功"
  printf 'ORDER_NUMBER=%s\nPROGRAM_ID=%s\nSHOW_TIME_ID=%s\nTICKET_CATEGORY_ID=%s\nREMAIN_BEFORE_REFUND=%s\nREMAIN_AFTER_REFUND=%s\n' \
    "${ORDER_NUMBER}" \
    "${PROGRAM_ID}" \
    "${SHOW_TIME_ID}" \
    "${TICKET_CATEGORY_ID}" \
    "${REMAIN_BEFORE_REFUND}" \
    "${REMAIN_AFTER_REFUND}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
