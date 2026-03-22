#!/usr/bin/env bash
set -euo pipefail

GATEWAY_BASE_URL="${GATEWAY_BASE_URL:-http://127.0.0.1:8081}"
CHANNEL_CODE="${CHANNEL_CODE:-0001}"
JWT="${JWT:-}"
PROGRAM_ID="${PROGRAM_ID:-10001}"
TICKET_CATEGORY_ID="${TICKET_CATEGORY_ID:-40001}"
TICKET_USER_IDS="${TICKET_USER_IDS:-}"
DISTRIBUTION_MODE="${DISTRIBUTION_MODE:-express}"
TAKE_TICKET_MODE="${TAKE_TICKET_MODE:-paper}"
WARMUP_REQUESTS="${WARMUP_REQUESTS:-1}"
CREATE_RETRIES="${CREATE_RETRIES:-5}"
ORDER_VISIBLE_WAIT_SECONDS="${ORDER_VISIBLE_WAIT_SECONDS:-20}"
RETRY_INTERVAL_SECONDS="${RETRY_INTERVAL_SECONDS:-1}"

CURL_LAST_STATUS=""
CURL_LAST_BODY=""
ORDER_NUMBER=""

log() {
  printf '[prewarm-order-ledgers] %s\n' "$*"
}

fail() {
  printf '[prewarm-order-ledgers] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  local name="$1"

  command -v "${name}" >/dev/null 2>&1 || fail "missing dependency: ${name}"
}

curl_request() {
  local path="$1"
  local payload="$2"
  local tmp_file http_code body

  tmp_file="$(mktemp)"
  if ! http_code="$(
    curl -sS \
      -o "${tmp_file}" \
      -w "%{http_code}" \
      -X POST \
      "${GATEWAY_BASE_URL}${path}" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer ${JWT}" \
      -H "X-Channel-Code: ${CHANNEL_CODE}" \
      -d "${payload}"
  )"; then
    rm -f "${tmp_file}"
    fail "request failed: ${path}"
  fi

  body="$(cat "${tmp_file}")"
  rm -f "${tmp_file}"

  CURL_LAST_STATUS="${http_code}"
  CURL_LAST_BODY="${body}"
}

create_order_payload() {
  printf '{"programId":%s,"ticketCategoryId":%s,"ticketUserIds":[%s],"distributionMode":"%s","takeTicketMode":"%s"}' \
    "${PROGRAM_ID}" \
    "${TICKET_CATEGORY_ID}" \
    "${TICKET_USER_IDS}" \
    "${DISTRIBUTION_MODE}" \
    "${TAKE_TICKET_MODE}"
}

try_create_order() {
  local payload

  payload="$(create_order_payload)"
  curl_request "/order/create" "${payload}"

  if [[ "${CURL_LAST_STATUS}" =~ ^2 ]]; then
    ORDER_NUMBER="$(printf '%s' "${CURL_LAST_BODY}" | jq -er '.orderNumber')"
    log "created warm-up order orderNumber=${ORDER_NUMBER}"
    return 0
  fi

  if [[ "${CURL_LAST_BODY}" == *"ledger not ready"* ]]; then
    return 1
  fi

  if [[ "${CURL_LAST_BODY}" == *"purchase limit"* ]]; then
    fail "warm-up hit purchase limit, reduce WARMUP_REQUESTS or use different TICKET_USER_IDS"
  fi

  fail "unexpected /order/create response: http=${CURL_LAST_STATUS} body=${CURL_LAST_BODY}"
}

wait_order_visible() {
  local attempt payload

  payload="$(printf '{"orderNumber":%s}' "${ORDER_NUMBER}")"
  for ((attempt = 1; attempt <= ORDER_VISIBLE_WAIT_SECONDS; attempt++)); do
    curl_request "/order/get" "${payload}"
    if [[ "${CURL_LAST_STATUS}" =~ ^2 ]]; then
      log "order visible orderNumber=${ORDER_NUMBER}"
      return
    fi
    if [[ "${CURL_LAST_BODY}" == *"order not found"* ]]; then
      sleep "${RETRY_INTERVAL_SECONDS}"
      continue
    fi

    fail "unexpected /order/get response: http=${CURL_LAST_STATUS} body=${CURL_LAST_BODY}"
  done

  fail "order ${ORDER_NUMBER} not visible within ${ORDER_VISIBLE_WAIT_SECONDS}s"
}

main() {
  local request attempt

  require_cmd curl
  require_cmd jq

  [[ -n "${JWT}" ]] || fail "JWT is required"
  [[ -n "${TICKET_USER_IDS}" ]] || fail "TICKET_USER_IDS is required, example: 701,702"
  [[ "${WARMUP_REQUESTS}" =~ ^[0-9]+$ ]] || fail "WARMUP_REQUESTS must be a positive integer"
  (( WARMUP_REQUESTS > 0 )) || fail "WARMUP_REQUESTS must be greater than 0"
  [[ "${CREATE_RETRIES}" =~ ^[0-9]+$ ]] || fail "CREATE_RETRIES must be a positive integer"
  (( CREATE_RETRIES > 0 )) || fail "CREATE_RETRIES must be greater than 0"

  log "gateway=${GATEWAY_BASE_URL} programId=${PROGRAM_ID} ticketCategoryId=${TICKET_CATEGORY_ID} warmupRequests=${WARMUP_REQUESTS}"

  for ((request = 1; request <= WARMUP_REQUESTS; request++)); do
    log "warm-up request ${request}/${WARMUP_REQUESTS}"
    for ((attempt = 1; attempt <= CREATE_RETRIES; attempt++)); do
      if try_create_order; then
        wait_order_visible
        break
      fi

      if (( attempt == CREATE_RETRIES )); then
        fail "ledger not ready persisted after ${CREATE_RETRIES} create attempts"
      fi

      log "ledger not ready, retrying create attempt ${attempt}/${CREATE_RETRIES}"
      sleep "${RETRY_INTERVAL_SECONDS}"
    done
  done

  log "order ledgers prewarmed successfully"
}

main "$@"
