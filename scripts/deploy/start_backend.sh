#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
LOG_DIR="${LOG_DIR:-${REPO_ROOT}/.codex/runlogs}"
PID_DIR="${PID_DIR:-${REPO_ROOT}/.codex/pids}"
IMPORT_SQL="${IMPORT_SQL:-0}"
START_AGENTS="${START_AGENTS:-1}"
CHECK_INFRA="${CHECK_INFRA:-1}"

INFRA_COMPOSE_FILE="${REPO_ROOT}/deploy/docker-compose/docker-compose.infrastructure.yml"

log() {
  printf '[start-backend] %s\n' "$*"
}

fail() {
  printf '[start-backend] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  local name="$1"

  command -v "${name}" >/dev/null 2>&1 || fail "missing dependency: ${name}"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --import-sql)
        IMPORT_SQL=1
        ;;
      --skip-agents)
        START_AGENTS=0
        ;;
      --skip-infra-check)
        CHECK_INFRA=0
        ;;
      -h|--help)
        cat <<'EOF'
Usage: bash scripts/deploy/start_backend.sh [options]

Options:
  --import-sql        import mysql schema and seed data before startup
  --skip-agents       do not start order MCP server and agents service
  --skip-infra-check  skip docker infrastructure running checks
  -h, --help          show this help message

Environment:
  IMPORT_SQL=0|1
  START_AGENTS=0|1
  CHECK_INFRA=0|1
  LOG_DIR=/abs/path
  PID_DIR=/abs/path
EOF
        exit 0
        ;;
      *)
        fail "unknown option: $1"
        ;;
    esac
    shift
  done
}

ensure_dirs() {
  mkdir -p "${LOG_DIR}" "${PID_DIR}"
}

is_port_listening() {
  local port="$1"

  ss -ltn "( sport = :${port} )" | grep -q ":${port}"
}

wait_for_port() {
  local name="$1"
  local port="$2"
  local attempts="${3:-30}"
  local sleep_seconds="${4:-1}"
  local i

  for ((i = 0; i < attempts; i++)); do
    if is_port_listening "${port}"; then
      log "${name} is ready on :${port}"
      return 0
    fi
    sleep "${sleep_seconds}"
  done

  return 1
}

is_process_running() {
  local pattern="$1"

  [[ -n "${pattern}" ]] || return 1
  pgrep -af "${pattern}" >/dev/null
}

wait_for_process() {
  local name="$1"
  local pattern="$2"
  local attempts="${3:-15}"
  local sleep_seconds="${4:-1}"
  local i

  for ((i = 0; i < attempts; i++)); do
    if is_process_running "${pattern}"; then
      log "${name} process is running"
      return 0
    fi
    sleep "${sleep_seconds}"
  done

  return 1
}

check_container_running() {
  local container_name="$1"

  docker ps --format '{{.Names}}' | grep -Fx "${container_name}" >/dev/null
}

ensure_infra() {
  local containers=(
    docker-compose-mysql-1
    docker-compose-redis-1
    docker-compose-etcd-1
    docker-compose-kafka-1
  )
  local container_name

  [[ -f "${INFRA_COMPOSE_FILE}" ]] || fail "compose file not found: ${INFRA_COMPOSE_FILE}"

  if [[ "${CHECK_INFRA}" != "1" ]]; then
    log "skip infrastructure checks"
    return
  fi

  require_cmd docker

  for container_name in "${containers[@]}"; do
    check_container_running "${container_name}" || fail "infrastructure container not running: ${container_name}"
  done

  log "infrastructure containers are running"
}

maybe_import_sql() {
  if [[ "${IMPORT_SQL}" != "1" ]]; then
    return
  fi

  log "importing sql data"
  bash "${REPO_ROOT}/scripts/import_sql.sh"
}

start_service() {
  local name="$1"
  local port="$2"
  local process_pattern="$3"
  local cmd="$4"
  local log_file="${LOG_DIR}/${name}.log"
  local pid_file="${PID_DIR}/${name}.pid"
  local pid

  if [[ "${port}" != "0" ]] && is_port_listening "${port}"; then
    log "skip ${name}, port :${port} already listening"
    return
  fi

  if [[ "${port}" == "0" ]] && is_process_running "${process_pattern}"; then
    log "skip ${name}, matching process already running"
    return
  fi

  log "starting ${name}"
  nohup bash -lc "cd '${REPO_ROOT}' && ${cmd}" >"${log_file}" 2>&1 < /dev/null &
  pid=$!
  printf '%s\n' "${pid}" > "${pid_file}"

  if [[ "${port}" != "0" ]]; then
    if ! wait_for_port "${name}" "${port}"; then
      tail -n 40 "${log_file}" >&2 || true
      fail "${name} failed to listen on :${port}"
    fi
    return
  fi

  if ! wait_for_process "${name}" "${process_pattern}"; then
    tail -n 40 "${log_file}" >&2 || true
    fail "${name} failed to stay alive"
  fi
}

start_core_services() {
  start_service "user-rpc" 8080 "services/user-rpc/user.go" "go run services/user-rpc/user.go -f services/user-rpc/etc/user-rpc.yaml"
  start_service "program-rpc" 8083 "services/program-rpc/program.go" "go run services/program-rpc/program.go -f services/program-rpc/etc/program-rpc.yaml"
  start_service "pay-rpc" 8084 "services/pay-rpc/pay.go" "go run services/pay-rpc/pay.go -f services/pay-rpc/etc/pay-rpc.yaml"
  start_service "order-rpc" 8082 "services/order-rpc/order.go" "go run services/order-rpc/order.go -f services/order-rpc/etc/order-rpc.yaml"

  start_service "order-close-worker" 0 "jobs/order-close/cmd/worker/main.go|order-close-worker.yaml" "go run jobs/order-close/cmd/worker/main.go -f jobs/order-close/etc/order-close-worker.yaml"
  start_service "order-close-dispatcher" 0 "jobs/order-close/cmd/dispatcher/main.go|order-close-dispatcher.yaml" "go run jobs/order-close/cmd/dispatcher/main.go -f jobs/order-close/etc/order-close-dispatcher.yaml"
  start_service "rush-inventory-preheat-worker" 0 "jobs/rush-inventory-preheat/cmd/worker/main.go|rush-inventory-preheat-worker.yaml" "go run jobs/rush-inventory-preheat/cmd/worker/main.go -f jobs/rush-inventory-preheat/etc/rush-inventory-preheat-worker.yaml"
  start_service "rush-inventory-preheat-dispatcher" 0 "jobs/rush-inventory-preheat/cmd/dispatcher/main.go|rush-inventory-preheat-dispatcher.yaml" "go run jobs/rush-inventory-preheat/cmd/dispatcher/main.go -f jobs/rush-inventory-preheat/etc/rush-inventory-preheat-dispatcher.yaml"

  start_service "user-api" 8888 "services/user-api/user.go" "go run services/user-api/user.go -f services/user-api/etc/user-api.yaml"
  start_service "program-api" 8889 "services/program-api/program.go" "go run services/program-api/program.go -f services/program-api/etc/program-api.yaml"
  start_service "order-api" 8890 "services/order-api/order.go" "go run services/order-api/order.go -f services/order-api/etc/order-api.yaml"
  start_service "pay-api" 8892 "services/pay-api/pay.go" "go run services/pay-api/pay.go -f services/pay-api/etc/pay-api.yaml"
}

start_optional_services() {
  if [[ "${START_AGENTS}" != "1" ]]; then
    log "skip order-mcp and agents services"
    return
  fi

  require_cmd uv

  start_service "order-mcp" 9082 "order_mcp_server" "go run ./services/order-rpc/cmd/order_mcp_server -f services/order-rpc/etc/order-mcp.yaml"
  start_service "agents" 8891 "uvicorn app.main:app" "cd agents && uv run uvicorn app.main:app --host 0.0.0.0 --port 8891 --reload"
}

start_gateway() {
  start_service "gateway-api" 8081 "services/gateway-api/gateway.go" "go run services/gateway-api/gateway.go -f services/gateway-api/etc/gateway-api.yaml"
}

print_summary() {
  cat <<EOF
[start-backend] startup finished
[start-backend] logs: ${LOG_DIR}
[start-backend] pids: ${PID_DIR}
[start-backend] check ports with:
ss -ltnp | grep -E ':(8080|8081|8082|8083|8084|8888|8889|8890|8891|8892|9082) '
EOF
}

main() {
  require_cmd bash
  require_cmd ss
  require_cmd grep
  require_cmd nohup
  require_cmd go
  require_cmd pgrep

  parse_args "$@"
  ensure_dirs
  ensure_infra
  maybe_import_sql
  start_core_services
  start_optional_services
  start_gateway
  print_summary
}

main "$@"
