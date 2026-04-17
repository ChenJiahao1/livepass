#!/usr/bin/env bash
set -euo pipefail

# 用途：
#   一键启动 livepass 后端运行环境。
#
# 默认行为：
#   - 拉起 MySQL / Redis / etcd / Kafka
#   - 检测空库并自动导入 SQL
#   - 启动全部 Go RPC / API / Job
#   - 启动 order-mcp / program-mcp / agents
#   - 最后启动 gateway-api
#
# 常用参数：
#   --import-sql      启动前强制重新导入 SQL
#   --skip-agents     跳过 MCP 与 agents
#   --only-agents     只启动 agents 相关链路
#   --force-restart   先停止脚本管理的服务，再重新启动
#   --detach          启动完成后立即退出，不保活父脚本
#   --skip-infra-check 跳过基础设施拉起与检查
#   -h, --help        查看完整帮助
#
# 常用示例：
#   bash scripts/deploy/start_backend.sh
#   bash scripts/deploy/start_backend.sh --force-restart
#   bash scripts/deploy/start_backend.sh --only-agents --force-restart
#   bash scripts/deploy/start_backend.sh --detach
#
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
LOG_DIR="${LOG_DIR:-${REPO_ROOT}/.codex/runlogs}"
PID_DIR="${PID_DIR:-${REPO_ROOT}/.codex/pids}"
IMPORT_SQL="${IMPORT_SQL:-0}"
START_AGENTS="${START_AGENTS:-1}"
CHECK_INFRA="${CHECK_INFRA:-1}"
ONLY_AGENTS="${ONLY_AGENTS:-0}"
FORCE_RESTART="${FORCE_RESTART:-0}"
DETACH="${DETACH:-0}"

STARTED_SERVICE_NAMES=()
STARTED_SERVICE_PORTS=()
STARTED_SERVICE_PATTERNS=()
STARTED_SERVICE_LOG_FILES=()
STARTUP_FINISHED=0
CLEANUP_DONE=0

INFRA_COMPOSE_FILE="${REPO_ROOT}/deploy/docker-compose/docker-compose.infrastructure.yml"

MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-compose-mysql-1}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-${MYSQL_ROOT_PASSWORD:-123456}}"
MYSQL_DB_USER="${MYSQL_DB_USER:-livepass_user}"
MYSQL_DB_PROGRAM="${MYSQL_DB_PROGRAM:-livepass_program}"
MYSQL_DB_ORDER="${MYSQL_DB_ORDER:-livepass_order}"
MYSQL_DB_PAY="${MYSQL_DB_PAY:-livepass_pay}"
MYSQL_DB_AGENTS="${MYSQL_DB_AGENTS:-livepass_agents}"

REDIS_CONTAINER="${REDIS_CONTAINER:-docker-compose-redis-1}"
ETCD_CONTAINER="${ETCD_CONTAINER:-docker-compose-etcd-1}"
KAFKA_CONTAINER="${KAFKA_CONTAINER:-docker-compose-kafka-1}"

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
      --only-agents)
        ONLY_AGENTS=1
        ;;
      --force-restart)
        FORCE_RESTART=1
        ;;
      --detach)
        DETACH=1
        ;;
      -h|--help)
        cat <<'EOF'
Usage: bash scripts/deploy/start_backend.sh [options]

Options:
  --import-sql        import mysql schema and seed data before startup
  --skip-agents       do not start order MCP server and agents service
  --skip-infra-check  skip docker infrastructure bootstrap and checks
  --only-agents       only start agents-related services and dependencies
  --force-restart     stop managed services first, then start again
  --detach            exit after startup, do not keep supervisor alive
  -h, --help          show this help message

Environment:
  IMPORT_SQL=0|1
  START_AGENTS=0|1
  CHECK_INFRA=0|1
  ONLY_AGENTS=0|1
  FORCE_RESTART=0|1
  DETACH=0|1
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

register_started_service() {
  local name="$1"
  local port="$2"
  local process_pattern="$3"
  local log_file="$4"

  STARTED_SERVICE_NAMES+=("${name}")
  STARTED_SERVICE_PORTS+=("${port}")
  STARTED_SERVICE_PATTERNS+=("${process_pattern}")
  STARTED_SERVICE_LOG_FILES+=("${log_file}")
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

wait_for_command() {
  local name="$1"
  local attempts="${2:-30}"
  local sleep_seconds="${3:-1}"
  shift 3
  local i

  for ((i = 0; i < attempts; i++)); do
    if "$@"; then
      log "${name} check passed"
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

wait_for_process_stopped() {
  local pattern="$1"
  local attempts="${2:-20}"
  local sleep_seconds="${3:-1}"
  local i

  for ((i = 0; i < attempts; i++)); do
    if ! is_process_running "${pattern}"; then
      return 0
    fi
    sleep "${sleep_seconds}"
  done

  return 1
}

stop_port_listeners() {
  local port="$1"
  local pids pid

  [[ "${port}" != "0" ]] || return 0

  pids="$(ss -ltnp "( sport = :${port} )" 2>/dev/null | grep -o 'pid=[0-9]\+' | cut -d= -f2 | sort -u || true)"
  [[ -n "${pids}" ]] || return 0

  for pid in ${pids}; do
    if kill -0 "${pid}" >/dev/null 2>&1; then
      log "stopping port :${port} listener pid ${pid}"
      kill "${pid}" >/dev/null 2>&1 || true
    fi
  done

  sleep 1

  pids="$(ss -ltnp "( sport = :${port} )" 2>/dev/null | grep -o 'pid=[0-9]\+' | cut -d= -f2 | sort -u || true)"
  for pid in ${pids}; do
    if kill -0 "${pid}" >/dev/null 2>&1; then
      kill -9 "${pid}" >/dev/null 2>&1 || true
    fi
  done
}

check_container_running() {
  local container_name="$1"

  docker ps --format '{{.Names}}' | grep -Fx "${container_name}" >/dev/null
}

mysql_exec() {
  docker exec "${MYSQL_CONTAINER}" \
    mysql \
    --default-character-set=utf8mb4 \
    -u"${MYSQL_USER}" \
    "-p${MYSQL_PASSWORD}" \
    "$@"
}

mysql_ready() {
  mysql_exec -e 'SELECT 1' >/dev/null 2>&1
}

redis_ready() {
  docker exec "${REDIS_CONTAINER}" redis-cli ping | grep -F 'PONG' >/dev/null
}

kafka_ready() {
  docker exec "${KAFKA_CONTAINER}" \
    /opt/kafka/bin/kafka-topics.sh \
    --bootstrap-server 127.0.0.1:9092 \
    --list >/dev/null 2>&1
}

database_has_tables() {
  local database="$1"
  local table_count

  table_count="$(mysql_exec -N -B -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${database}'" 2>/dev/null || echo 0)"
  [[ "${table_count:-0}" -gt 0 ]]
}

should_import_sql() {
  if [[ "${IMPORT_SQL}" == "1" ]]; then
    return 0
  fi

  database_has_tables "${MYSQL_DB_USER}" || return 0
  database_has_tables "${MYSQL_DB_PROGRAM}" || return 0
  database_has_tables "${MYSQL_DB_ORDER}" || return 0
  database_has_tables "${MYSQL_DB_PAY}" || return 0
  database_has_tables "${MYSQL_DB_AGENTS}" || return 0
  return 1
}

ensure_infra() {
  local containers=(
    "${MYSQL_CONTAINER}"
    "${REDIS_CONTAINER}"
    "${ETCD_CONTAINER}"
    "${KAFKA_CONTAINER}"
  )
  local container_name

  [[ -f "${INFRA_COMPOSE_FILE}" ]] || fail "compose file not found: ${INFRA_COMPOSE_FILE}"

  if [[ "${CHECK_INFRA}" != "1" ]]; then
    log "skip infrastructure bootstrap and checks"
    return
  fi

  log "ensuring infrastructure via docker compose"
  docker compose -f "${INFRA_COMPOSE_FILE}" up -d

  for container_name in "${containers[@]}"; do
    check_container_running "${container_name}" || fail "infrastructure container not running: ${container_name}"
  done

  wait_for_port "mysql" 3306 60 1 || fail "mysql not ready on :3306"
  wait_for_port "redis" 6379 60 1 || fail "redis not ready on :6379"
  wait_for_port "etcd" 2379 60 1 || fail "etcd not ready on :2379"
  wait_for_port "kafka" 9094 90 1 || fail "kafka not ready on :9094"
  wait_for_command "mysql" 60 1 mysql_ready || fail "mysql readiness check failed"
  wait_for_command "redis" 60 1 redis_ready || fail "redis readiness check failed"
  wait_for_command "kafka" 90 1 kafka_ready || fail "kafka readiness check failed"

  log "infrastructure containers are running"
}

maybe_import_sql() {
  if ! should_import_sql; then
    log "skip sql import, databases already initialized"
    return
  fi

  log "importing sql data"
  bash "${REPO_ROOT}/scripts/import_sql.sh"
}

run_logged_command() {
  local name="$1"
  local cmd="$2"
  local log_file="${LOG_DIR}/${name}.log"

  log "running ${name}"
  if ! bash -lc "cd '${REPO_ROOT}' && ${cmd}" >"${log_file}" 2>&1; then
    tail -n 40 "${log_file}" >&2 || true
    fail "${name} failed"
  fi
}

stop_service() {
  local name="$1"
  local port="$2"
  local process_pattern="$3"
  local pid_file="${PID_DIR}/${name}.pid"
  local pid

  if [[ -f "${pid_file}" ]]; then
    pid="$(cat "${pid_file}" 2>/dev/null || true)"
    if [[ -n "${pid}" ]] && kill -0 "${pid}" >/dev/null 2>&1; then
      log "stopping ${name} by pid ${pid}"
      kill "${pid}" >/dev/null 2>&1 || true
    fi
  fi

  if is_process_running "${process_pattern}"; then
    log "stopping ${name} by pattern"
    pkill -f "${process_pattern}" >/dev/null 2>&1 || true
    if ! wait_for_process_stopped "${process_pattern}" 15 1; then
      pkill -9 -f "${process_pattern}" >/dev/null 2>&1 || true
      wait_for_process_stopped "${process_pattern}" 5 1 || fail "failed to stop ${name}"
    fi
  fi

  stop_port_listeners "${port}"
  rm -f "${pid_file}"
}

cleanup_started_services() {
  local exit_code="${1:-0}"
  local idx

  if [[ "${CLEANUP_DONE}" == "1" ]]; then
    return
  fi
  CLEANUP_DONE=1

  if [[ "${DETACH}" == "1" && "${STARTUP_FINISHED}" == "1" && "${exit_code}" == "0" ]]; then
    return
  fi

  if [[ "${#STARTED_SERVICE_NAMES[@]}" -eq 0 ]]; then
    return
  fi

  if [[ "${exit_code}" == "0" ]]; then
    log "stopping managed services"
  else
    log "stopping managed services after exit code ${exit_code}"
  fi

  for ((idx = ${#STARTED_SERVICE_NAMES[@]} - 1; idx >= 0; idx--)); do
    stop_service \
      "${STARTED_SERVICE_NAMES[${idx}]}" \
      "${STARTED_SERVICE_PORTS[${idx}]}" \
      "${STARTED_SERVICE_PATTERNS[${idx}]}"
  done
}

force_restart_requested_services() {
  if [[ "${FORCE_RESTART}" != "1" ]]; then
    return
  fi

  log "force restarting requested services"

  if [[ "${ONLY_AGENTS}" == "1" ]]; then
    stop_service "agents" 8891 "uvicorn app.main:app"
    stop_service "program-mcp" 9083 "program_mcp_server"
    stop_service "order-mcp" 9082 "order_mcp_server"
    stop_service "order-rpc" 8082 "services/order-rpc/order.go"
    stop_service "program-rpc" 8083 "services/program-rpc/program.go"
    stop_service "user-rpc" 8080 "services/user-rpc/user.go"
    return
  fi

  stop_service "gateway-api" 8081 "services/gateway-api/gateway.go"
  stop_service "agents" 8891 "uvicorn app.main:app"
  stop_service "program-mcp" 9083 "program_mcp_server"
  stop_service "order-mcp" 9082 "order_mcp_server"
  stop_service "pay-api" 8892 "services/pay-api/pay.go"
  stop_service "order-api" 8890 "services/order-api/order.go"
  stop_service "program-api" 8889 "services/program-api/program.go"
  stop_service "user-api" 8888 "services/user-api/user.go"
  stop_service "rush-inventory-preheat-dispatcher" 0 "jobs/rush-inventory-preheat/cmd/dispatcher/main.go|rush-inventory-preheat-dispatcher.yaml"
  stop_service "rush-inventory-preheat-worker" 0 "jobs/rush-inventory-preheat/cmd/worker/main.go|rush-inventory-preheat-worker.yaml"
  stop_service "order-close-dispatcher" 0 "jobs/order-close/cmd/dispatcher/main.go|order-close-dispatcher.yaml"
  stop_service "order-close-worker" 0 "jobs/order-close/cmd/worker/main.go|order-close-worker.yaml"
  stop_service "order-rpc" 8082 "services/order-rpc/order.go"
  stop_service "pay-rpc" 8084 "services/pay-rpc/pay.go"
  stop_service "program-rpc" 8083 "services/program-rpc/program.go"
  stop_service "user-rpc" 8080 "services/user-rpc/user.go"
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
    register_started_service "${name}" "${port}" "${process_pattern}" "${log_file}"
    return
  fi

  if ! wait_for_process "${name}" "${process_pattern}"; then
    tail -n 40 "${log_file}" >&2 || true
    fail "${name} failed to stay alive"
  fi

  register_started_service "${name}" "${port}" "${process_pattern}" "${log_file}"
}

service_is_healthy() {
  local port="$1"
  local process_pattern="$2"

  if [[ "${port}" != "0" ]]; then
    is_port_listening "${port}"
    return
  fi

  is_process_running "${process_pattern}"
}

monitor_services() {
  local idx
  local name
  local port
  local process_pattern
  local log_file

  if [[ "${DETACH}" == "1" ]]; then
    log "detach mode enabled, skip supervisor keepalive"
    return
  fi

  log "services are running under foreground supervisor, press Ctrl-C to stop"

  while true; do
    for idx in "${!STARTED_SERVICE_NAMES[@]}"; do
      name="${STARTED_SERVICE_NAMES[${idx}]}"
      port="${STARTED_SERVICE_PORTS[${idx}]}"
      process_pattern="${STARTED_SERVICE_PATTERNS[${idx}]}"
      log_file="${STARTED_SERVICE_LOG_FILES[${idx}]}"

      if service_is_healthy "${port}" "${process_pattern}"; then
        continue
      fi

      tail -n 40 "${log_file}" >&2 || true
      fail "${name} stopped unexpectedly"
    done
    sleep 2
  done
}

start_core_services() {
  start_service "user-rpc" 8080 "services/user-rpc/user.go" "go run services/user-rpc/user.go -f services/user-rpc/etc/user.yaml"
  start_service "program-rpc" 8083 "services/program-rpc/program.go" "go run services/program-rpc/program.go -f services/program-rpc/etc/program.yaml"
  start_service "pay-rpc" 8084 "services/pay-rpc/pay.go" "go run services/pay-rpc/pay.go -f services/pay-rpc/etc/pay.yaml"
  start_service "order-rpc" 8082 "services/order-rpc/order.go" "go run services/order-rpc/order.go -f services/order-rpc/etc/order.yaml"

  start_service "order-close-worker" 0 "jobs/order-close/cmd/worker/main.go|order-close-worker.yaml" "go run jobs/order-close/cmd/worker/main.go -f jobs/order-close/etc/order-close-worker.yaml"
  start_service "order-close-dispatcher" 0 "jobs/order-close/cmd/dispatcher/main.go|order-close-dispatcher.yaml" "go run jobs/order-close/cmd/dispatcher/main.go -f jobs/order-close/etc/order-close-dispatcher.yaml"
  start_service "rush-inventory-preheat-worker" 0 "jobs/rush-inventory-preheat/cmd/worker/main.go|rush-inventory-preheat-worker.yaml" "go run jobs/rush-inventory-preheat/cmd/worker/main.go -f jobs/rush-inventory-preheat/etc/rush-inventory-preheat-worker.yaml"
  start_service "rush-inventory-preheat-dispatcher" 0 "jobs/rush-inventory-preheat/cmd/dispatcher/main.go|rush-inventory-preheat-dispatcher.yaml" "go run jobs/rush-inventory-preheat/cmd/dispatcher/main.go -f jobs/rush-inventory-preheat/etc/rush-inventory-preheat-dispatcher.yaml"

  start_service "user-api" 8888 "services/user-api/user.go" "go run services/user-api/user.go -f services/user-api/etc/user-api.yaml"
  start_service "program-api" 8889 "services/program-api/program.go" "go run services/program-api/program.go -f services/program-api/etc/program-api.yaml"
  start_service "order-api" 8890 "services/order-api/order.go" "go run services/order-api/order.go -f services/order-api/etc/order-api.yaml"
  start_service "pay-api" 8892 "services/pay-api/pay.go" "go run services/pay-api/pay.go -f services/pay-api/etc/pay-api.yaml"
}

start_agents_related_services() {
  start_service "user-rpc" 8080 "services/user-rpc/user.go" "go run services/user-rpc/user.go -f services/user-rpc/etc/user.yaml"
  start_service "program-rpc" 8083 "services/program-rpc/program.go" "go run services/program-rpc/program.go -f services/program-rpc/etc/program.yaml"
  start_service "order-rpc" 8082 "services/order-rpc/order.go" "go run services/order-rpc/order.go -f services/order-rpc/etc/order.yaml"
  start_optional_services
}

start_optional_services() {
  if [[ "${START_AGENTS}" != "1" ]]; then
    log "skip order-mcp, program-mcp and agents services"
    return
  fi

  require_cmd uv

  start_service "order-mcp" 9082 "order_mcp_server" "go run ./services/order-rpc/cmd/order_mcp_server -f services/order-rpc/etc/order-mcp.yaml"
  start_service "program-mcp" 9083 "program_mcp_server" "go run ./services/program-rpc/cmd/program_mcp_server -f services/program-rpc/etc/program-mcp.yaml"
  start_service "agents" 8891 "uvicorn app.main:app" "cd agents && uv run uvicorn app.main:app --host 0.0.0.0 --port 8891 --reload"
}

start_gateway() {
  start_service "gateway-api" 8081 "services/gateway-api/gateway.go" "go run services/gateway-api/gateway.go -f services/gateway-api/etc/gateway-api.yaml"
}

print_summary() {
  local port_pattern=':(8080|8081|8082|8083|8084|8888|8889|8890|8891|8892|9082|9083) '

  if [[ "${ONLY_AGENTS}" == "1" ]]; then
    port_pattern=':(8080|8082|8083|8891|9082|9083) '
  fi

  cat <<EOF
[start-backend] startup finished
[start-backend] logs: ${LOG_DIR}
[start-backend] pids: ${PID_DIR}
[start-backend] mode: $(if [[ "${DETACH}" == "1" ]]; then echo detach; else echo foreground-supervisor; fi)
[start-backend] check ports with:
ss -ltnp | grep -E '${port_pattern}'
EOF
}

main() {
  require_cmd bash
  require_cmd ss
  require_cmd grep
  require_cmd nohup
  require_cmd go
  require_cmd pgrep
  require_cmd pkill
  require_cmd docker

  trap 'cleanup_started_services $?' EXIT
  trap 'exit 0' INT TERM

  parse_args "$@"
  if [[ "${ONLY_AGENTS}" == "1" && "${START_AGENTS}" != "1" ]]; then
    fail "--only-agents cannot be combined with --skip-agents"
  fi
  ensure_dirs
  force_restart_requested_services
  ensure_infra
  maybe_import_sql
  if [[ "${ONLY_AGENTS}" == "1" ]]; then
    start_agents_related_services
    STARTUP_FINISHED=1
    print_summary
    monitor_services
    return
  fi
  start_core_services
  start_optional_services
  start_gateway
  STARTUP_FINISHED=1
  print_summary
  monitor_services
}

main "$@"
