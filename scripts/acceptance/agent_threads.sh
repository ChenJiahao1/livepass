#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/agent_cases.sh"

BASE_URL="${BASE_URL:-http://127.0.0.1:8081}"
JWT="${JWT:-}"
RUN_ID="${RUN_ID:-$(date +%s%N)}"
PASSWORD="${PASSWORD:-123456}"
MOBILE="${MOBILE:-139$(printf '%08d' "$((10#${RUN_ID} % 100000000))")}"

log() {
  printf '[agent-threads] %s\n' "$*"
}

fail() {
  printf '[agent-threads] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

ensure_jwt() {
  if [[ -n "${JWT}" ]]; then
    return
  fi

  require_cmd curl
  require_cmd jq

  local register_payload
  local login_payload
  local login_body

  register_payload="$(jq -nc --arg mobile "${MOBILE}" --arg password "${PASSWORD}" '{mobile:$mobile,password:$password,confirmPassword:$password}')"
  login_payload="$(jq -nc --arg mobile "${MOBILE}" --arg password "${PASSWORD}" '{mobile:$mobile,password:$password}')"

  if public_gateway_curl POST /user/register "${register_payload}" >/dev/null 2>&1; then
    log "registered acceptance user ${MOBILE}"
  else
    log "register skipped for ${MOBILE}, continue with login"
  fi

  login_body="$(public_gateway_curl POST /user/login "${login_payload}")"
  JWT="$(printf '%s' "${login_body}" | jq -r '.token // empty')"
  [[ -n "${JWT}" ]] || fail "login did not return token: ${login_body}"
  log "logged in acceptance user ${MOBILE}"
}

public_gateway_curl() {
  local method="$1"
  local path="$2"
  local payload="${3:-}"

  if [[ -n "${payload}" ]]; then
    curl -fsS -X "${method}" "${BASE_URL}${path}" \
      -H 'Content-Type: application/json' \
      -d "${payload}"
    return
  fi

  curl -fsS -X "${method}" "${BASE_URL}${path}" \
    -H 'Content-Type: application/json'
}

gateway_curl() {
  local method="$1"
  local path="$2"
  local payload="${3:-}"

  if [[ -n "${payload}" ]]; then
    curl -fsS -X "${method}" "${BASE_URL}${path}" \
      -H 'Content-Type: application/json' \
      -H "Authorization: Bearer ${JWT}" \
      -d "${payload}"
    return
  fi

  curl -fsS -X "${method}" "${BASE_URL}${path}" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${JWT}"
}

assert_thread_body() {
  local body="$1"
  printf '%s' "${body}" | jq -e '.thread.id != ""' >/dev/null || fail "invalid thread body: ${body}"
}

assert_messages_body() {
  local body="$1"
  printf '%s' "${body}" | jq -e '.messages | type == "array"' >/dev/null || fail "invalid messages body: ${body}"
}

assert_run_body() {
  local body="$1"
  printf '%s' "${body}" | jq -e '.run.id != "" and .run.status != ""' >/dev/null || fail "invalid run body: ${body}"
}

main() {
  require_cmd curl
  require_cmd jq
  ensure_jwt

  log "BASE_URL=${BASE_URL}"

  local create_body
  local thread_id
  local messages_body
  local run_create_body
  local run_id
  local run_body
  local threads_body

  create_body="$(gateway_curl POST /agent/threads '{"title":"订单咨询"}')"
  assert_thread_body "${create_body}"
  thread_id="$(printf '%s' "${create_body}" | jq -r '.thread.id')"
  log "create_thread=${create_body}"

  messages_body="$(gateway_curl GET "/agent/threads/${thread_id}/messages")"
  assert_messages_body "${messages_body}"
  [[ "$(printf '%s' "${messages_body}" | jq '.messages | length')" == "0" ]] || fail "expected empty initial messages"
  log "initial_messages=${messages_body}"

  run_create_body="$(gateway_curl POST /agent/runs "$(jq -nc --arg thread_id "${thread_id}" --arg message "$(agent_case_order)" '{threadId:$thread_id,input:{parts:[{type:"text",text:$message}]},metadata:{}}')")"
  printf '%s' "${run_create_body}" | jq -e '.thread.id != "" and .run.id != "" and .assistantMessage.id != ""' >/dev/null \
    || fail "invalid create run response: ${run_create_body}"
  run_id="$(printf '%s' "${run_create_body}" | jq -r '.run.id')"
  log "create_run=${run_create_body}"

  run_body="$(gateway_curl GET "/agent/runs/${run_id}")"
  assert_run_body "${run_body}"
  log "get_run=${run_body}"

  threads_body="$(gateway_curl GET /agent/threads)"
  printf '%s' "${threads_body}" | jq -e '.threads | type == "array"' >/dev/null || fail "invalid threads body: ${threads_body}"
  log "list_threads=${threads_body}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
