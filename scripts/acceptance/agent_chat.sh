#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/agent_chat_cases.sh"

BASE_URL="${BASE_URL:-http://127.0.0.1:8081}"
JWT="${JWT:-}"

log() {
  printf '[agent-chat] %s\n' "$*"
}

fail() {
  printf '[agent-chat] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

assert_contract() {
  local body="$1"

  printf '%s' "${body}" | jq -e '.conversationId != "" and .reply != "" and .status != ""' >/dev/null \
    || fail "invalid response contract: ${body}"
}

chat_once() {
  local message="$1"
  local conversation_id="${2:-}"
  local payload

  if [[ -n "${conversation_id}" ]]; then
    payload="$(jq -nc --arg message "${message}" --arg conversationId "${conversation_id}" '{message:$message, conversationId:$conversationId}')"
  else
    payload="$(jq -nc --arg message "${message}" '{message:$message}')"
  fi

  curl -fsS "${BASE_URL}/agent/chat" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer ${JWT}" \
    -d "${payload}"
}

main() {
  require_cmd curl
  require_cmd jq

  [[ -n "${JWT}" ]] || fail "JWT is required"

  log "BASE_URL=${BASE_URL}"

  local activity_body
  local order_body
  local preview_body
  local refund_body
  local handoff_body
  local conversation_id

  activity_body="$(chat_once "$(agent_case_activity)")"
  assert_contract "${activity_body}"
  log "activity=${activity_body}"

  order_body="$(chat_once "$(agent_case_order)")"
  assert_contract "${order_body}"
  conversation_id="$(printf '%s' "${order_body}" | jq -r '.conversationId')"
  log "order=${order_body}"

  preview_body="$(chat_once "$(agent_case_refund_preview)" "${conversation_id}")"
  assert_contract "${preview_body}"
  [[ "$(printf '%s' "${preview_body}" | jq -r '.conversationId')" == "${conversation_id}" ]] \
    || fail "conversationId changed unexpectedly in multi-turn preview"
  log "refund_preview=${preview_body}"

  refund_body="$(chat_once "$(agent_case_refund_submit)" "${conversation_id}")"
  assert_contract "${refund_body}"
  log "refund_submit=${refund_body}"

  handoff_body="$(chat_once "$(agent_case_handoff)")"
  assert_contract "${handoff_body}"
  log "handoff=${handoff_body}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
