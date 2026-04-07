#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8081}"
CHANNEL_CODE="${CHANNEL_CODE:-0001}"
SHOW_TIME_ID="${SHOW_TIME_ID:-30001}"
PROGRAM_ID="${PROGRAM_ID:-}"
PASSWORD="${PASSWORD:-123456}"
SUBJECT="${SUBJECT:-大麦演出票}"
PAY_CHANNEL="${PAY_CHANNEL:-mock}"
RUN_ID="${RUN_ID:-$(date +%s%N)}"
MOBILE="${MOBILE:-139$(printf '%08d' "$((10#${RUN_ID} % 100000000))")}"
TICKET_CATEGORY_ID="${TICKET_CATEGORY_ID:-}"
ORDER_PROGRESS_WAIT_SECONDS="${ORDER_PROGRESS_WAIT_SECONDS:-15}"

TICKET_USER_A_NAME="${TICKET_USER_A_NAME:-张三}"
TICKET_USER_A_ID_NUMBER="${TICKET_USER_A_ID_NUMBER:-110101199001011234}"
TICKET_USER_B_NAME="${TICKET_USER_B_NAME:-李四}"
TICKET_USER_B_ID_NUMBER="${TICKET_USER_B_ID_NUMBER:-110101199202021234}"

USER_ID=""
TOKEN=""
PURCHASE_TOKEN=""
TICKET_USER_ID_1=""
TICKET_USER_ID_2=""
ORDER_NUMBER=""
POLL_ORDER_STATUS=""
POLL_LAST_BODY=""
CURL_LAST_STATUS=""
CURL_LAST_BODY=""

log() {
  printf '[order-checkout] %s\n' "$*"
}

fail() {
  printf '[order-checkout] ERROR: %s\n' "$*" >&2
  exit 1
}

print_json() {
  local body="$1"

  if printf '%s\n' "${body}" | jq '.' >/dev/null 2>&1; then
    printf '%s\n' "${body}" | jq
    return
  fi

  printf '%s\n' "${body}"
}

require_cmd() {
  local name="$1"

  command -v "${name}" >/dev/null || fail "missing dependency: ${name}"
}

assert_json_filter() {
  local body="$1"
  local filter="$2"
  local message="$3"

  printf '%s' "${body}" | jq -e "${filter}" >/dev/null || fail "${message}"
}

extract_required() {
  local body="$1"
  local filter="$2"
  local label="$3"
  local value

  value="$(
    printf '%s' "${body}" | jq -r "if (${filter}) == null then empty else (${filter}) end"
  )" || fail "missing ${label}"
  [[ -n "${value}" ]] || fail "missing ${label}"
  printf '%s' "${value}"
}

curl_request() {
  local path="$1"
  local payload="$2"
  local auth_required="${3:-0}"
  local tmp_file http_code body
  local curl_args=(
    -sS
    -o
    ""
    -w
    "%{http_code}"
    -X
    POST
    "${BASE_URL}${path}"
    -H
    "Content-Type: application/json"
    -d
    "${payload}"
  )

  tmp_file="$(mktemp)"
  curl_args[2]="${tmp_file}"

  if [[ "${auth_required}" == "1" ]]; then
    curl_args+=(
      -H
      "Authorization: Bearer ${TOKEN}"
      -H
      "X-Channel-Code: ${CHANNEL_CODE}"
    )
  fi

  if ! http_code="$(curl "${curl_args[@]}")"; then
    rm -f "${tmp_file}"
    fail "request failed: ${path}"
  fi

  body="$(cat "${tmp_file}")"
  rm -f "${tmp_file}"

  CURL_LAST_STATUS="${http_code}"
  CURL_LAST_BODY="${body}"
}

curl_json() {
  local path="$1"
  local payload="$2"
  local auth_required="${3:-0}"

  curl_request "${path}" "${payload}" "${auth_required}"

  if [[ ! "${CURL_LAST_STATUS}" =~ ^2 ]]; then
    print_json "${CURL_LAST_BODY}" >&2 || true
    fail "unexpected http status ${CURL_LAST_STATUS} for ${path}"
  fi

  printf '%s' "${CURL_LAST_BODY}"
}

curl_json_expect_error() {
  local path="$1"
  local payload="$2"
  local expected_message="${3:-}"
  local auth_required="${4:-0}"
  local expected_status="${5:-}"

  curl_request "${path}" "${payload}" "${auth_required}"

  if [[ "${CURL_LAST_STATUS}" =~ ^2 ]]; then
    print_json "${CURL_LAST_BODY}" >&2 || true
    fail "expected non-2xx http status for ${path}, got ${CURL_LAST_STATUS}"
  fi

  if [[ -n "${expected_status}" && "${CURL_LAST_STATUS}" != "${expected_status}" ]]; then
    print_json "${CURL_LAST_BODY}" >&2 || true
    fail "unexpected http status ${CURL_LAST_STATUS} for ${path}, expected ${expected_status}"
  fi

  if [[ -n "${expected_message}" && "${CURL_LAST_BODY}" != *"${expected_message}"* ]]; then
    print_json "${CURL_LAST_BODY}" >&2 || true
    fail "expected error body to contain '${expected_message}' for ${path}"
  fi

  printf '%s' "${CURL_LAST_BODY}"
}

preorder_payload() {
  printf '{"showTimeId":%s}' "${SHOW_TIME_ID}"
}

get_preorder_body() {
  curl_json "/program/preorder/detail" "$(preorder_payload)"
}

get_preorder_remain_number() {
  local ticket_category_id="$1"
  local body remain

  body="$(get_preorder_body)"
  remain="$(
    printf '%s' "${body}" | jq -er --arg ticket_category_id "${ticket_category_id}" '.ticketCategoryVoList[] | select((.id | tostring) == $ticket_category_id) | .remainNumber'
  )" || fail "missing remainNumber for ticketCategoryId=${ticket_category_id}"

  printf '%s' "${remain}"
}

preflight() {
  require_cmd curl
  require_cmd jq
}

register_user() {
  local body

  log "1/11 注册用户"
  body="$(curl_json "/user/register" "{\"mobile\":\"${MOBILE}\",\"password\":\"${PASSWORD}\",\"confirmPassword\":\"${PASSWORD}\"}")"
  print_json "${body}"
  assert_json_filter "${body}" 'has("success") and .success == true' "/user/register success != true"
}

login_user() {
  local body

  log "2/11 登录用户"
  body="$(curl_json "/user/login" "{\"code\":\"${CHANNEL_CODE}\",\"mobile\":\"${MOBILE}\",\"password\":\"${PASSWORD}\"}")"
  USER_ID="$(extract_required "${body}" '.userId' 'userId')"
  TOKEN="$(extract_required "${body}" '.token' 'token')"
  print_json "${body}"
  printf 'USER_ID=%s\nTOKEN=%s\n' "${USER_ID}" "${TOKEN}"
}

add_ticket_user() {
  local rel_name="$1"
  local id_number="$2"
  local body

  log "新增观演人 ${rel_name}"
  body="$(curl_json "/ticket/user/add" "{\"userId\":${USER_ID},\"relName\":\"${rel_name}\",\"idType\":1,\"idNumber\":\"${id_number}\"}")"
  print_json "${body}"
  assert_json_filter "${body}" 'has("success") and .success == true' "/ticket/user/add success != true"
}

list_ticket_users() {
  local body

  log "4/11 查询用户与观演人列表"
  body="$(curl_json "/user/get/user/ticket/list" "{\"userId\":${USER_ID}}")"
  print_json "${body}"
  assert_json_filter "${body}" '.ticketUserVoList | length >= 2' "expected at least two ticket users"

  TICKET_USER_ID_1="$(
    printf '%s' "${body}" | jq -er --arg id_number "${TICKET_USER_A_ID_NUMBER}" '.ticketUserVoList[] | select(.idNumber == $id_number) | .id'
  )" || fail "missing ticket user id for ${TICKET_USER_A_NAME}"
  TICKET_USER_ID_2="$(
    printf '%s' "${body}" | jq -er --arg id_number "${TICKET_USER_B_ID_NUMBER}" '.ticketUserVoList[] | select(.idNumber == $id_number) | .id'
  )" || fail "missing ticket user id for ${TICKET_USER_B_NAME}"

  printf 'TICKET_USER_ID_1=%s\nTICKET_USER_ID_2=%s\n' "${TICKET_USER_ID_1}" "${TICKET_USER_ID_2}"
}

fetch_preorder() {
  local body

  log "5/11 查询预下单详情"
  body="$(get_preorder_body)"
  print_json "${body}"
  assert_json_filter "${body}" ".showTimeId == ${SHOW_TIME_ID}" "unexpected showTimeId in preorder response"
  assert_json_filter "${body}" '.ticketCategoryVoList | length > 0' "missing ticket categories in preorder response"
  assert_json_filter "${body}" '.permitChooseSeat == 0' "expected permitChooseSeat=0"

  PROGRAM_ID="$(extract_required "${body}" '.programId' 'programId')"
  if [[ -n "${TICKET_CATEGORY_ID}" ]]; then
    printf '%s' "${body}" | jq -e --arg ticket_category_id "${TICKET_CATEGORY_ID}" '.ticketCategoryVoList[] | select((.id | tostring) == $ticket_category_id)' >/dev/null || fail "configured TICKET_CATEGORY_ID not found in preorder response"
  else
    TICKET_CATEGORY_ID="$(extract_required "${body}" '.ticketCategoryVoList[0].id' 'ticketCategoryId')"
  fi

  printf 'PROGRAM_ID=%s\nSHOW_TIME_ID=%s\nTICKET_CATEGORY_ID=%s\n' "${PROGRAM_ID}" "${SHOW_TIME_ID}" "${TICKET_CATEGORY_ID}"
}

create_purchase_token() {
  local body

  log "6/11 申请购买令牌"
  body="$(curl_json "/order/purchase/token" "{\"showTimeId\":${SHOW_TIME_ID},\"ticketCategoryId\":${TICKET_CATEGORY_ID},\"ticketUserIds\":[${TICKET_USER_ID_1},${TICKET_USER_ID_2}],\"distributionMode\":\"express\",\"takeTicketMode\":\"paper\"}" 1)"
  PURCHASE_TOKEN="$(extract_required "${body}" '.purchaseToken' 'purchaseToken')"
  print_json "${body}"
  printf 'PURCHASE_TOKEN=%s\n' "${PURCHASE_TOKEN}"
}

create_order() {
  local body

  log "7/11 创建订单"
  body="$(curl_json "/order/create" "{\"purchaseToken\":\"${PURCHASE_TOKEN}\"}" 1)"
  ORDER_NUMBER="$(extract_required "${body}" '.orderNumber' 'orderNumber')"
  print_json "${body}"
  printf 'ORDER_NUMBER=%s\n' "${ORDER_NUMBER}"
}

poll_order_until_done() {
  local attempt body done

  log "8/11 轮询下单进度"
  for ((attempt = 0; attempt < ORDER_PROGRESS_WAIT_SECONDS; attempt++)); do
    curl_request "/order/poll" "{\"orderNumber\":${ORDER_NUMBER}}" 1
    body="${CURL_LAST_BODY}"
    if [[ ! "${CURL_LAST_STATUS}" =~ ^2 ]]; then
      if [[ "${body}" == *"order not found"* ]]; then
        sleep 1
        continue
      fi
      print_json "${body}" >&2 || true
      fail "unexpected response while polling order: http=${CURL_LAST_STATUS}"
    fi

    print_json "${body}"
    assert_json_filter "${body}" ".orderNumber == ${ORDER_NUMBER}" "unexpected order number in poll response"
    done="$(extract_required "${body}" '.done' 'poll.done')"
    POLL_ORDER_STATUS="$(extract_required "${body}" '.orderStatus' 'poll.orderStatus')"
    POLL_LAST_BODY="${body}"
    if [[ "${done}" == "true" ]]; then
      return
    fi
    sleep 1
  done

  print_json "${POLL_LAST_BODY}" >&2 || true
  fail "order did not reach terminal state within ${ORDER_PROGRESS_WAIT_SECONDS}s"
}

assert_poll_status() {
  local expected_status="$1"

  if [[ "${POLL_ORDER_STATUS}" != "${expected_status}" ]]; then
    print_json "${POLL_LAST_BODY}" >&2 || true
    fail "unexpected terminal poll status ${POLL_ORDER_STATUS}, expected ${expected_status}"
  fi
}

fetch_order_snapshot() {
  local expected_status="${1:-}"
  local expected_ticket_count="${2:-2}"
  local body

  body="$(curl_json "/order/get" "{\"orderNumber\":${ORDER_NUMBER}}" 1)"
  print_json "${body}"
  assert_json_filter "${body}" ".orderNumber == ${ORDER_NUMBER}" "unexpected order number in order detail response"
  if [[ -n "${expected_status}" ]]; then
    assert_json_filter "${body}" ".orderStatus == ${expected_status}" "unexpected order status in order detail response"
  fi
  if [[ -n "${expected_ticket_count}" ]]; then
    assert_json_filter "${body}" ".orderTicketInfoVoList | length == ${expected_ticket_count}" "unexpected order ticket snapshot count"
  fi

  printf '%s' "${body}"
}

pay_order() {
  local body

  log "9/11 模拟支付"
  body="$(curl_json "/order/pay" "{\"orderNumber\":${ORDER_NUMBER},\"subject\":\"${SUBJECT}\",\"channel\":\"${PAY_CHANNEL}\"}" 1)"
  print_json "${body}"
  assert_json_filter "${body}" ".orderNumber == ${ORDER_NUMBER}" "unexpected order number in pay response"
  assert_json_filter "${body}" '.payBillNo > 0' "missing payBillNo in pay response"
  assert_json_filter "${body}" '.payStatus == 2' "expected payStatus=2 after pay"
  assert_json_filter "${body}" '.orderStatus == 3' "expected orderStatus=3 after pay"
}

check_payment() {
  local body

  log "10/11 查询支付状态"
  body="$(curl_json "/order/pay/check" "{\"orderNumber\":${ORDER_NUMBER}}" 1)"
  print_json "${body}"
  assert_json_filter "${body}" ".orderNumber == ${ORDER_NUMBER}" "unexpected order number in pay check response"
  assert_json_filter "${body}" '.payStatus == 2' "expected payStatus=2 in pay check response"
  assert_json_filter "${body}" '.orderStatus == 3' "expected orderStatus=3 in pay check response"
}

get_order() {
  local body

  log "11/11 查询订单详情"
  body="$(fetch_order_snapshot "3" "2")"
  assert_json_filter "${body}" 'all(.orderTicketInfoVoList[]; (.seatId > 0 and .seatRow > 0 and .seatCol > 0))' "expected allocated seats in order detail response"
}

main() {
  preflight

  log "BASE_URL=${BASE_URL}"
  log "CHANNEL_CODE=${CHANNEL_CODE}"
  log "SHOW_TIME_ID=${SHOW_TIME_ID}"
  log "MOBILE=${MOBILE}"

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
  get_order

  log "主路径执行成功"
  printf 'USER_ID=%s\nTOKEN=%s\nPROGRAM_ID=%s\nSHOW_TIME_ID=%s\nTICKET_USER_ID_1=%s\nTICKET_USER_ID_2=%s\nTICKET_CATEGORY_ID=%s\nPURCHASE_TOKEN=%s\nORDER_NUMBER=%s\n' \
    "${USER_ID}" \
    "${TOKEN}" \
    "${PROGRAM_ID}" \
    "${SHOW_TIME_ID}" \
    "${TICKET_USER_ID_1}" \
    "${TICKET_USER_ID_2}" \
    "${TICKET_CATEGORY_ID}" \
    "${PURCHASE_TOKEN}" \
    "${ORDER_NUMBER}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
