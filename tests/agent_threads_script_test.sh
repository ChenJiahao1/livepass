#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="scripts/acceptance/agent_threads.sh"

fail() {
  printf '[agent-threads-script-test] ERROR: %s\n' "$*" >&2
  exit 1
}

[[ -f "${SCRIPT_PATH}" ]] || fail "script not found: ${SCRIPT_PATH}"

bash -n "${SCRIPT_PATH}"

unset JWT
export MOBILE="13900000001"
export PASSWORD="123456"

# shellcheck disable=SC1090
source "${SCRIPT_PATH}"

CALLS_FILE="$(mktemp)"
trap 'rm -f "${CALLS_FILE}"' EXIT

public_gateway_curl() {
  local method="$1"
  local path="$2"
  local payload="${3:-}"

  printf '%s %s\n' "${method}" "${path}" >>"${CALLS_FILE}"
  case "${method} ${path}" in
    "POST /user/register")
      [[ "${payload}" == *'"mobile":"13900000001"'* ]] || fail "register payload missing mobile"
      printf '{"success":true}'
      ;;
    "POST /user/login")
      [[ "${payload}" == *'"mobile":"13900000001"'* ]] || fail "login payload missing mobile"
      printf '{"userId":3001,"token":"jwt-from-login"}'
      ;;
    *)
      fail "unexpected public request: ${method} ${path}"
      ;;
  esac
}

ensure_jwt

[[ "${JWT:-}" == "jwt-from-login" ]] || fail "expected jwt-from-login, got ${JWT:-<empty>}"
[[ "$(cat "${CALLS_FILE}")"$'\n' == $'POST /user/register\nPOST /user/login\n' ]] || fail "unexpected public call sequence: $(cat "${CALLS_FILE}")"

printf '[agent-threads-script-test] ok\n'
