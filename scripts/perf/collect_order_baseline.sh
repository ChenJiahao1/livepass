#!/usr/bin/env bash
set -euo pipefail

MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-123456}"
MYSQL_DATABASE="${MYSQL_DATABASE:-damai_order}"
BASELINE_START_ORDER_COUNT="${BASELINE_START_ORDER_COUNT:-}"

KAFKA_BOOTSTRAP="${KAFKA_BOOTSTRAP:-127.0.0.1:9094}"
TOPIC="${TOPIC:-order.create.command.v1}"
GROUP_ID="${GROUP_ID:-damai-go-order-create}"
DOCKER_COMPOSE_FILE="${DOCKER_COMPOSE_FILE:-deploy/docker-compose/docker-compose.infrastructure.yml}"
KAFKA_SERVICE="${KAFKA_SERVICE:-kafka}"
KAFKA_CONSUMER_GROUPS_BIN="${KAFKA_CONSUMER_GROUPS_BIN:-}"

GATEWAY_BASE_URL="${GATEWAY_BASE_URL:-http://127.0.0.1:8081}"
CHANNEL_CODE="${CHANNEL_CODE:-0001}"
JWT="${JWT:-}"
SAMPLE_ORDER_NUMBER="${SAMPLE_ORDER_NUMBER:-}"
ORDER_VISIBLE_WAIT_SECONDS="${ORDER_VISIBLE_WAIT_SECONDS:-10}"

ORDER_RPC_LOG_FILE="${ORDER_RPC_LOG_FILE:-}"

CURL_LAST_STATUS=""
CURL_LAST_BODY=""

log() {
  printf '[collect-order-baseline] %s\n' "$*" >&2
}

fail() {
  printf '[collect-order-baseline] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  local name="$1"

  command -v "${name}" >/dev/null 2>&1 || fail "missing dependency: ${name}"
}

run_mysql_scalar() {
  local query="$1"

  MYSQL_PWD="${MYSQL_PASSWORD}" mysql \
    -N \
    -h "${MYSQL_HOST}" \
    -P "${MYSQL_PORT}" \
    -u "${MYSQL_USER}" \
    "${MYSQL_DATABASE}" \
    -e "${query}"
}

run_kafka_consumer_groups() {
  if [[ -n "${KAFKA_CONSUMER_GROUPS_BIN}" ]]; then
    "${KAFKA_CONSUMER_GROUPS_BIN}" "$@"
    return
  fi

  if command -v kafka-consumer-groups.sh >/dev/null 2>&1; then
    kafka-consumer-groups.sh "$@"
    return
  fi

  require_cmd docker
  docker compose -f "${DOCKER_COMPOSE_FILE}" exec -T "${KAFKA_SERVICE}" /opt/kafka/bin/kafka-consumer-groups.sh "$@"
}

collect_kafka_lag_total() {
  local describe_output lag_total

  if ! describe_output="$(
    run_kafka_consumer_groups \
      --bootstrap-server "${KAFKA_BOOTSTRAP}" \
      --describe \
      --group "${GROUP_ID}" \
      --topic "${TOPIC}" 2>/dev/null
  )"; then
    printf 'unavailable\n'
    return
  fi

  lag_total="$(printf '%s\n' "${describe_output}" | awk 'NR > 1 && $0 !~ /^$/ {if ($(NF) ~ /^[0-9]+$/) sum += $(NF)} END {print sum + 0}')"
  printf '%s\n' "${lag_total:-0}"
}

curl_request_auth() {
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

now_millis() {
  date +%s%3N
}

collect_visibility_latency_millis() {
  local start_ms current_ms attempt payload

  if [[ -z "${SAMPLE_ORDER_NUMBER}" ]]; then
    printf 'n/a\n'
    return
  fi

  [[ -n "${JWT}" ]] || fail "JWT is required when SAMPLE_ORDER_NUMBER is set"
  payload="$(printf '{"orderNumber":%s}' "${SAMPLE_ORDER_NUMBER}")"
  start_ms="$(now_millis)"

  for ((attempt = 1; attempt <= ORDER_VISIBLE_WAIT_SECONDS; attempt++)); do
    curl_request_auth "/order/get" "${payload}"
    if [[ "${CURL_LAST_STATUS}" =~ ^2 ]]; then
      current_ms="$(now_millis)"
      printf '%s\n' "$((current_ms - start_ms))"
      return
    fi
    if [[ "${CURL_LAST_BODY}" == *"order not found"* ]]; then
      sleep 1
      continue
    fi

    printf 'error:http=%s\n' "${CURL_LAST_STATUS}"
    return
  done

  printf 'timeout\n'
}

collect_expired_skip_count() {
  if [[ -z "${ORDER_RPC_LOG_FILE}" ]]; then
    printf 'n/a\n'
    return
  fi
  if [[ ! -f "${ORDER_RPC_LOG_FILE}" ]]; then
    printf 'missing-log-file\n'
    return
  fi

  grep -c "skip expired order create event" "${ORDER_RPC_LOG_FILE}" || true
}

main() {
  local current_order_count order_delta kafka_lag_total visibility_latency expired_skip_count

  require_cmd mysql

  current_order_count="$(run_mysql_scalar 'SELECT (SELECT COUNT(*) FROM d_order_00) + (SELECT COUNT(*) FROM d_order_01);')"
  if [[ -n "${BASELINE_START_ORDER_COUNT}" ]]; then
    order_delta="$((current_order_count - BASELINE_START_ORDER_COUNT))"
  else
    order_delta="n/a"
  fi

  kafka_lag_total="$(collect_kafka_lag_total)"
  visibility_latency="$(collect_visibility_latency_millis)"
  expired_skip_count="$(collect_expired_skip_count)"

  printf '%-32s %s\n' "metric" "value"
  printf '%-32s %s\n' "kafka_lag_total" "${kafka_lag_total}"
  printf '%-32s %s\n' "shard_order_count" "${current_order_count}"
  printf '%-32s %s\n' "shard_order_delta_vs_start" "${order_delta}"
  printf '%-32s %s\n' "sample_visibility_latency_ms" "${visibility_latency}"
  printf '%-32s %s\n' "skip_expired_event_count" "${expired_skip_count}"
}

main "$@"
