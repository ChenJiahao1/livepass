#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="scripts/deploy/start_backend.sh"

fail() {
  printf '[start-backend-test] ERROR: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  local pattern="$1"

  grep -F "${pattern}" "${SCRIPT_PATH}" >/dev/null || fail "missing pattern: ${pattern}"
}

[[ -f "${SCRIPT_PATH}" ]] || fail "script not found: ${SCRIPT_PATH}"

bash -n "${SCRIPT_PATH}"

assert_contains 'docker-compose.infrastructure.yml'
assert_contains 'scripts/import_sql.sh'
assert_contains 'services/user-rpc/user.go'
assert_contains 'services/program-rpc/program.go'
assert_contains 'services/pay-rpc/pay.go'
assert_contains 'services/order-rpc/order.go'
assert_contains 'jobs/order-close-worker/order_close_worker.go'
assert_contains 'jobs/order-close/order_close.go'
assert_contains 'start_service "order-close" 0 "jobs/order-close/order_close.go"'
assert_contains 'services/user-api/user.go'
assert_contains 'services/program-api/program.go'
assert_contains 'services/order-api/order.go'
assert_contains 'services/pay-api/pay.go'
assert_contains 'services/order-rpc/cmd/order_mcp_server'
assert_contains 'services/gateway-api/gateway.go'
assert_contains 'uv run uvicorn app.main:app --host 0.0.0.0 --port 8891 --reload'

printf '[start-backend-test] ok\n'
