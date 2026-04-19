#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="scripts/deploy/start_backend.sh"

fail() {
  printf '[start-backend-test] ERROR: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  local pattern="$1"

  grep -F -- "${pattern}" "${SCRIPT_PATH}" >/dev/null || fail "missing pattern: ${pattern}"
}

assert_not_contains() {
  local pattern="$1"

  if grep -F -- "${pattern}" "${SCRIPT_PATH}" >/dev/null; then
    fail "unexpected pattern: ${pattern}"
  fi
}

[[ -f "${SCRIPT_PATH}" ]] || fail "script not found: ${SCRIPT_PATH}"

bash -n "${SCRIPT_PATH}"

assert_contains 'docker-compose.infrastructure.yml'
assert_contains 'FORCE_RESTART="${FORCE_RESTART:-0}"'
assert_contains 'ONLY_AGENTS="${ONLY_AGENTS:-0}"'
assert_contains 'DETACH="${DETACH:-0}"'
assert_contains 'START_PROFILE="${START_PROFILE:-default}"'
assert_contains '--force-restart'
assert_contains '--only-agents'
assert_contains '--detach'
assert_contains '--perf'
assert_contains 'START_PROFILE=perf'
assert_contains 'config_for()'
assert_contains 'if [[ "${START_PROFILE}" != "default" && "${START_PROFILE}" != "perf" ]]; then'
assert_contains 'docker compose -f "${INFRA_COMPOSE_FILE}" up -d'
assert_contains 'scripts/import_sql.sh'
assert_contains 'should_import_sql'
assert_contains 'database_has_tables'
assert_contains 'stop_service'
assert_contains 'stop_port_listeners'
assert_contains 'force_restart_requested_services'
assert_contains 'monitor_services'
assert_contains 'cleanup_started_services'
assert_contains 'trap '\''cleanup_started_services $?'\'' EXIT'
assert_contains 'start_agents_related_services'
assert_contains 'if [[ "${ONLY_AGENTS}" == "1" ]]; then'
assert_contains 'if [[ "${DETACH}" == "1" ]]; then'
assert_contains 'services/user-rpc/user.go'
assert_contains 'services/program-rpc/program.go'
assert_contains 'services/pay-rpc/pay.go'
assert_contains 'services/order-rpc/order.go'
assert_contains 'jobs/order-close/cmd/worker/main.go'
assert_contains 'jobs/order-close/cmd/dispatcher/main.go'
assert_contains 'start_service "order-close-dispatcher" 0 "jobs/order-close/cmd/dispatcher/main.go|order-close-dispatcher.yaml"'
assert_contains 'services/user-api/user.go'
assert_contains 'services/program-api/program.go'
assert_contains 'services/order-api/order.go'
assert_contains 'services/pay-api/pay.go'
assert_contains 'services/order-rpc/cmd/order_mcp_server'
assert_contains 'services/program-rpc/cmd/program_mcp_server'
assert_not_contains 'generate_proto_stubs'
assert_not_contains 'agents-generate-proto-stubs'
assert_contains 'services/gateway-api/gateway.go'
assert_contains 'services/user-rpc/etc/user.perf.yaml'
assert_contains 'services/program-rpc/etc/program.perf.yaml'
assert_contains 'services/pay-rpc/etc/pay.perf.yaml'
assert_contains 'services/order-rpc/etc/order.perf.yaml'
assert_contains 'services/user-api/etc/user-api.perf.yaml'
assert_contains 'services/program-api/etc/program-api.perf.yaml'
assert_contains 'services/order-api/etc/order-api.perf.yaml'
assert_contains 'services/pay-api/etc/pay-api.perf.yaml'
assert_contains 'services/gateway-api/etc/gateway-api.perf.yaml'
assert_contains 'uv run uvicorn app.api.app:app --host 0.0.0.0 --port 8891 --reload'
assert_not_contains 'uv run uvicorn app.main:app --host 0.0.0.0 --port 8891 --reload'
assert_contains ":(8080|8081|8082|8083|8084|8888|8889|8890|8891|8892|9082|9083) "

printf '[start-backend-test] ok\n'
