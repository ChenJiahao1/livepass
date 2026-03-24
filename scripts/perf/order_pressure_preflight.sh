#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODE="${1:-check}"

MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-compose-mysql-1}"
MYSQL_HOST="${MYSQL_HOST:-127.0.0.1}"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-123456}"

REDIS_HOST="${REDIS_HOST:-127.0.0.1}"
REDIS_PORT="${REDIS_PORT:-6379}"
ETCD_HEALTH_URL="${ETCD_HEALTH_URL:-http://127.0.0.1:2379/health}"

DOCKER_COMPOSE_FILE="${DOCKER_COMPOSE_FILE:-deploy/docker-compose/docker-compose.infrastructure.yml}"
KAFKA_SERVICE="${KAFKA_SERVICE:-kafka}"
KAFKA_BOOTSTRAP="${KAFKA_BOOTSTRAP:-127.0.0.1:9094}"
TOPIC="${TOPIC:-order.create.command.v1}"
GROUP_ID="${GROUP_ID:-damai-go-order-create}"

TARGET_RPC_INSTANCES="${TARGET_RPC_INSTANCES:-5}"
PROGRAM_ID="${PROGRAM_ID:-10001}"
TICKET_CATEGORY_ID="${TICKET_CATEGORY_ID:-40001}"
REQUIRED_SEAT_COUNT="${REQUIRED_SEAT_COUNT:-1000}"
REQUIRE_CLEAN_STATE="${REQUIRE_CLEAN_STATE:-1}"
REQUIRE_ZERO_LAG="${REQUIRE_ZERO_LAG:-1}"

USER_POOL_FILE="${USER_POOL_FILE:-}"
JWT="${JWT:-}"
CHANNEL_SECRET="${CHANNEL_SECRET:-local-user-secret-0001}"

GATEWAY_BASE_URL="${GATEWAY_BASE_URL:-http://127.0.0.1:8081}"
CHANNEL_CODE="${CHANNEL_CODE:-0001}"
TICKET_USER_IDS="${TICKET_USER_IDS:-}"
WARMUP_REQUESTS="${WARMUP_REQUESTS:-1}"
PREWARM_ON_PREPARE="${PREWARM_ON_PREPARE:-0}"
PREWARM_ORDER_LIMIT_LEDGER_ON_PREPARE="${PREWARM_ORDER_LIMIT_LEDGER_ON_PREPARE:-1}"

PORTS_TO_CHECK="${PORTS_TO_CHECK:-}"

USER_POOL_ENTRY_COUNT=0
USER_POOL_UNIQUE_USER_COUNT=0
USER_POOL_UNIQUE_TICKET_USER_COUNT=0
JWT_USER_ID=""
JWT_EXPIRES_AT=""

PASS_COUNT=0
WARN_COUNT=0
FAIL_COUNT=0

log() {
  printf '[order-pressure-preflight] %s\n' "$*"
}

pass() {
  PASS_COUNT=$((PASS_COUNT + 1))
  printf 'PASS %s\n' "$*"
}

warn() {
  WARN_COUNT=$((WARN_COUNT + 1))
  printf 'WARN %s\n' "$*"
}

fail_check() {
  FAIL_COUNT=$((FAIL_COUNT + 1))
  printf 'FAIL %s\n' "$*"
}

fatal() {
  printf '[order-pressure-preflight] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  local name="$1"

  command -v "${name}" >/dev/null 2>&1 || fatal "missing dependency: ${name}"
}

clean_mysql_output() {
  sed '/^mysql: \[Warning\]/d' | sed '/^[[:space:]]*$/d'
}

run_mysql_query() {
  local query="$1"

  docker exec "${MYSQL_CONTAINER}" mysql -u"${MYSQL_USER}" -p"${MYSQL_PASSWORD}" -N -B -e "${query}" 2>&1 | clean_mysql_output
}

mysql_scalar() {
  local query="$1"
  local output

  output="$(run_mysql_query "${query}")" || return 1
  printf '%s\n' "${output}" | tail -n 1
}

emit_id_chunks() {
  local ids_file="$1"
  local chunk_size="${2:-500}"

  python3 - <<'PY' "${ids_file}" "${chunk_size}"
import sys
from pathlib import Path

path = Path(sys.argv[1])
chunk_size = max(1, int(sys.argv[2]))
ids = [line.strip() for line in path.read_text().splitlines() if line.strip()]

for index in range(0, len(ids), chunk_size):
    print(",".join(ids[index:index + chunk_size]))
PY
}

mysql_count_ids_from_file() {
  local table="$1"
  local ids_file="$2"
  local chunk_size="${3:-500}"
  local chunk_ids chunk_count total

  total=0
  while IFS= read -r chunk_ids; do
    if [[ -z "${chunk_ids}" ]]; then
      continue
    fi

    chunk_count="$(mysql_scalar "SELECT COUNT(*) FROM ${table} WHERE id IN (${chunk_ids});")" || return 1
    total=$((total + chunk_count))
  done < <(emit_id_chunks "${ids_file}" "${chunk_size}")

  printf '%s\n' "${total}"
}

run_kafka_topics() {
  docker compose -f "${DOCKER_COMPOSE_FILE}" exec -T "${KAFKA_SERVICE}" /opt/kafka/bin/kafka-topics.sh "$@"
}

run_kafka_consumer_groups() {
  docker compose -f "${DOCKER_COMPOSE_FILE}" exec -T "${KAFKA_SERVICE}" /opt/kafka/bin/kafka-consumer-groups.sh "$@"
}

build_default_port_list() {
  local ports extra index

  ports=(8080 8081 8082 8083 8084 8888 8889 8890)
  extra=$((TARGET_RPC_INSTANCES - 1))
  if (( extra <= 0 )); then
    printf '%s\n' "$(IFS=,; printf '%s' "${ports[*]}")"
    return
  fi

  for ((index = 0; index < extra; index++)); do
    ports+=("$((28180 + index))")
    ports+=("$((28283 + index))")
    ports+=("$((18082 + index))")
  done

  printf '%s\n' "$(IFS=,; printf '%s' "${ports[*]}")"
}

decode_user_pool_tsv() {
  local user_pool_file="$1"

  python3 - <<'PY' "${user_pool_file}"
import base64
import json
import sys
from pathlib import Path

path = Path(sys.argv[1])
if not path.exists():
    raise SystemExit("user pool file not found")

pool = json.loads(path.read_text())
if not isinstance(pool, list) or not pool:
    raise SystemExit("user pool must be a non-empty JSON array")

for index, entry in enumerate(pool):
    if not isinstance(entry, dict):
        raise SystemExit(f"user pool entry at index {index} must be an object")
    jwt = str(entry.get("jwt", "")).strip()
    if not jwt:
        raise SystemExit(f"missing jwt at index {index}")
    ticket_user_ids_raw = entry.get("ticketUserIds")
    if ticket_user_ids_raw is None:
        ticket_user_ids_raw = [entry.get("ticketUserId", "")]
    if not isinstance(ticket_user_ids_raw, list) or not ticket_user_ids_raw:
        raise SystemExit(f"missing ticketUserIds at index {index}")
    ticket_user_ids = []
    for ticket_user_id in ticket_user_ids_raw:
        ticket_user_id = str(ticket_user_id).strip()
        if not ticket_user_id.isdigit():
            raise SystemExit(f"invalid ticketUserId at index {index}: {ticket_user_id!r}")
        ticket_user_ids.append(ticket_user_id)
    parts = jwt.split(".")
    if len(parts) != 3:
        raise SystemExit(f"invalid jwt format at index {index}")
    payload = parts[1] + "=" * (-len(parts[1]) % 4)
    try:
        claims = json.loads(base64.urlsafe_b64decode(payload))
    except Exception as exc:
        raise SystemExit(f"invalid jwt payload at index {index}: {exc}") from exc
    user_id = claims.get("userId")
    exp = claims.get("exp")
    if not isinstance(user_id, int) or user_id <= 0:
        raise SystemExit(f"invalid userId in jwt at index {index}")
    if not isinstance(exp, int) or exp <= 0:
        raise SystemExit(f"invalid exp in jwt at index {index}")
    print(f"{index}\t{user_id}\t{exp}\t{','.join(ticket_user_ids)}")
PY
}

extract_ticket_user_ids_from_lines() {
  local lines_file="$1"

  awk -F'\t' '
    {
      count = split($4, ids, ",")
      for (i = 1; i <= count; i++) {
        if (ids[i] != "") {
          print ids[i]
        }
      }
    }
  ' "${lines_file}"
}

extract_user_ids_from_lines() {
  local lines_file="$1"

  cut -f2 "${lines_file}"
}

decode_single_jwt() {
  local token="$1"

  python3 - <<'PY' "${token}"
import base64
import json
import sys

token = sys.argv[1].strip()
parts = token.split(".")
if len(parts) != 3:
    raise SystemExit("invalid JWT format")
payload = parts[1] + "=" * (-len(parts[1]) % 4)
claims = json.loads(base64.urlsafe_b64decode(payload))
user_id = claims.get("userId")
exp = claims.get("exp")
if not isinstance(user_id, int) or user_id <= 0:
    raise SystemExit("invalid JWT userId")
if not isinstance(exp, int) or exp <= 0:
    raise SystemExit("invalid JWT exp")
print(f"{user_id}\t{exp}")
PY
}

check_mode() {
  case "${MODE}" in
    check|prepare|report) ;;
    *)
      fatal "unsupported mode: ${MODE}, expected one of: check, prepare, report"
      ;;
  esac
}

check_container_running() {
  local container_name="$1"

  if docker ps --format '{{.Names}}' | grep -Fxq "${container_name}"; then
    pass "container running: ${container_name}"
    return 0
  fi

  fail_check "container not running: ${container_name}"
  return 1
}

check_mysql_connectivity() {
  local result

  if result="$(mysql_scalar 'SELECT 1;')" && [[ "${result}" == "1" ]]; then
    pass "mysql reachable via ${MYSQL_CONTAINER}"
    return 0
  fi

  fail_check "mysql unreachable via ${MYSQL_CONTAINER}"
  return 1
}

check_redis_connectivity() {
  if python3 - <<'PY' "${REDIS_HOST}" "${REDIS_PORT}" >/dev/null
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
s = socket.create_connection((host, port), timeout=3)
reader = s.makefile("rb")
s.sendall(b"*1\r\n$4\r\nPING\r\n")
prefix = reader.read(1)
line = reader.readline()[:-2]
reader.close()
s.close()
if prefix != b"+" or line.decode() != "PONG":
    raise SystemExit(1)
PY
  then
    pass "redis reachable at ${REDIS_HOST}:${REDIS_PORT}"
    return 0
  fi

  fail_check "redis unreachable at ${REDIS_HOST}:${REDIS_PORT}"
  return 1
}

check_etcd_health() {
  local body

  if body="$(curl -fsS "${ETCD_HEALTH_URL}")" && [[ "${body}" == *"true"* ]]; then
    pass "etcd healthy at ${ETCD_HEALTH_URL}"
    return 0
  fi

  fail_check "etcd unhealthy at ${ETCD_HEALTH_URL}"
  return 1
}

current_topic_partitions() {
  local describe_output partition_count

  if ! describe_output="$(run_kafka_topics --bootstrap-server "${KAFKA_BOOTSTRAP}" --describe --topic "${TOPIC}" 2>/dev/null)"; then
    printf '0\n'
    return
  fi

  partition_count="$(printf '%s\n' "${describe_output}" | awk '/PartitionCount:/ {for (i = 1; i <= NF; i++) if ($i == "PartitionCount:") {print $(i + 1); exit}}')"
  if [[ -z "${partition_count}" ]]; then
    partition_count="$(printf '%s\n' "${describe_output}" | awk '$1 == "Topic:" {count++} END {print count + 0}')"
  fi

  printf '%s\n' "${partition_count:-0}"
}

current_kafka_lag_total() {
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

check_kafka_topic() {
  local partitions

  partitions="$(current_topic_partitions)"
  if [[ "${partitions}" =~ ^[0-9]+$ ]] && (( partitions >= TARGET_RPC_INSTANCES )); then
    pass "kafka topic ${TOPIC} partitions=${partitions} target=${TARGET_RPC_INSTANCES}"
    return 0
  fi

  if [[ "${MODE}" == "prepare" ]]; then
    log "preparing kafka topic ${TOPIC} to partitions=${TARGET_RPC_INSTANCES}"
    PARTITIONS="${TARGET_RPC_INSTANCES}" bash "${ROOT_DIR}/scripts/perf/prepare_order_kafka_topic.sh" >/dev/null
    partitions="$(current_topic_partitions)"
    if [[ "${partitions}" =~ ^[0-9]+$ ]] && (( partitions >= TARGET_RPC_INSTANCES )); then
      pass "kafka topic ${TOPIC} prepared to partitions=${partitions}"
      return 0
    fi
  fi

  fail_check "kafka topic ${TOPIC} partitions=${partitions} below target=${TARGET_RPC_INSTANCES}"
  return 1
}

check_kafka_lag() {
  local lag_total

  lag_total="$(current_kafka_lag_total)"
  if [[ "${lag_total}" == "unavailable" ]]; then
    warn "kafka consumer lag unavailable for group ${GROUP_ID}; this is expected before order-rpc consumer starts"
    return 0
  fi

  if (( REQUIRE_ZERO_LAG == 0 )); then
    warn "kafka lag check skipped: lag_total=${lag_total}"
    return 0
  fi

  if [[ "${lag_total}" == "0" ]]; then
    pass "kafka lag clean for group ${GROUP_ID}"
    return 0
  fi

  fail_check "kafka lag not clean for group ${GROUP_ID}: lag_total=${lag_total}"
  return 1
}

check_ports_available() {
  local port_list raw_ports port active

  if [[ -n "${PORTS_TO_CHECK}" ]]; then
    port_list="${PORTS_TO_CHECK}"
  else
    port_list="$(build_default_port_list)"
  fi

  IFS=',' read -r -a raw_ports <<<"${port_list}"
  for port in "${raw_ports[@]}"; do
    port="${port// /}"
    [[ -n "${port}" ]] || continue
    active="$(ss -ltnp | awk -v target=":${port}" '$4 ~ target"$" {print $0}')"
    if [[ -n "${active}" ]]; then
      fail_check "port in use: ${port}"
      printf '%s\n' "${active}"
    else
      pass "port available: ${port}"
    fi
  done
}

check_user_pool_file() {
  local user_pool_file="$1"
  local decoded tmpdir lines_file duplicate_users duplicate_ticket_users now expired_count

  [[ -f "${user_pool_file}" ]] || {
    printf 'user pool file not found: %s\n' "${user_pool_file}" >&2
    return 1
  }

  if ! decoded="$(decode_user_pool_tsv "${user_pool_file}" 2>&1)"; then
    printf '%s\n' "${decoded}" >&2
    return 1
  fi

  tmpdir="$(mktemp -d)"
  lines_file="${tmpdir}/lines.tsv"
  printf '%s\n' "${decoded}" >"${lines_file}"

  USER_POOL_ENTRY_COUNT="$(wc -l <"${lines_file}" | tr -d ' ')"
  USER_POOL_UNIQUE_USER_COUNT="$(cut -f2 "${lines_file}" | sort -u | wc -l | tr -d ' ')"
  USER_POOL_UNIQUE_TICKET_USER_COUNT="$(extract_ticket_user_ids_from_lines "${lines_file}" | sort -u | wc -l | tr -d ' ')"

  duplicate_users="$(cut -f2 "${lines_file}" | sort | uniq -d | paste -sd, -)"
  duplicate_ticket_users="$(extract_ticket_user_ids_from_lines "${lines_file}" | sort | uniq -d | paste -sd, -)"
  now="$(date +%s)"
  expired_count="$(awk -F'\t' -v now="${now}" '$3 <= now {count++} END {print count + 0}' "${lines_file}")"

  if [[ -n "${duplicate_users}" ]]; then
    printf 'duplicate userId found in user pool: %s\n' "${duplicate_users}" >&2
    return 1
  fi
  if [[ -n "${duplicate_ticket_users}" ]]; then
    printf 'duplicate ticketUserId found in user pool: %s\n' "${duplicate_ticket_users}" >&2
    return 1
  fi
  if (( expired_count > 0 )); then
    printf 'expired jwt entries found in user pool: %s\n' "${expired_count}" >&2
    return 1
  fi

  return 0
}

check_user_pool_database_mapping() {
  local user_pool_file="$1"
  local decoded tmpdir lines_file user_ids_file ticket_user_ids_file user_count ticket_user_count

  decoded="$(decode_user_pool_tsv "${user_pool_file}")"
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "'"${tmpdir}"'"' RETURN
  lines_file="${tmpdir}/lines.tsv"
  user_ids_file="${tmpdir}/user_ids.txt"
  ticket_user_ids_file="${tmpdir}/ticket_user_ids.txt"
  printf '%s\n' "${decoded}" >"${lines_file}"

  cut -f2 "${lines_file}" | sort -u >"${user_ids_file}"
  extract_ticket_user_ids_from_lines "${lines_file}" | sort -u >"${ticket_user_ids_file}"

  user_count="$(mysql_count_ids_from_file "damai_user.d_user" "${user_ids_file}")"
  ticket_user_count="$(mysql_count_ids_from_file "damai_user.d_ticket_user" "${ticket_user_ids_file}")"

  if [[ "${user_count}" != "${USER_POOL_UNIQUE_USER_COUNT}" ]]; then
    fail_check "user pool contains missing users in MySQL: expected=${USER_POOL_UNIQUE_USER_COUNT} actual=${user_count}"
    return 1
  fi
  if [[ "${ticket_user_count}" != "${USER_POOL_UNIQUE_TICKET_USER_COUNT}" ]]; then
    fail_check "user pool contains missing ticket users in MySQL: expected=${USER_POOL_UNIQUE_TICKET_USER_COUNT} actual=${ticket_user_count}"
    return 1
  fi

  pass "user pool mapped in MySQL: users=${user_count} ticketUsers=${ticket_user_count}"
}

check_single_jwt() {
  local decoded now user_id expires_at

  [[ -n "${JWT}" ]] || {
    warn "single JWT check skipped: JWT not set"
    return 0
  }

  if ! decoded="$(decode_single_jwt "${JWT}" 2>&1)"; then
    fail_check "single JWT invalid: ${decoded}"
    return 1
  fi

  JWT_USER_ID="$(printf '%s\n' "${decoded}" | cut -f1)"
  JWT_EXPIRES_AT="$(printf '%s\n' "${decoded}" | cut -f2)"
  now="$(date +%s)"
  if (( JWT_EXPIRES_AT <= now )); then
    fail_check "single JWT expired for userId=${JWT_USER_ID}"
    return 1
  fi

  pass "single JWT valid for userId=${JWT_USER_ID}"
}

check_order_limit_ledgers() {
  local user_pool_file="$1"
  local decoded tmpdir lines_file result ready_count loading_count missing_count missing_users

  decoded="$(decode_user_pool_tsv "${user_pool_file}")"
  tmpdir="$(mktemp -d)"
  lines_file="${tmpdir}/lines.tsv"
  printf '%s\n' "${decoded}" >"${lines_file}"

  result="$(
    python3 - <<'PY' "${REDIS_HOST}" "${REDIS_PORT}" "${PROGRAM_ID}" "${lines_file}"
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
program_id = sys.argv[3]
lines_file = sys.argv[4]
prefix = "damai-go:order:purchase-limit"

user_ids = []
with open(lines_file, "r", encoding="utf-8") as fh:
    for line in fh:
        line = line.rstrip("\n")
        if not line:
            continue
        parts = line.split("\t")
        user_ids.append(parts[1])

sock = socket.create_connection((host, port), timeout=5)
reader = sock.makefile("rb")

def send(*parts):
    payload = [str(part) for part in parts]
    chunks = [f"*{len(payload)}\r\n".encode()]
    for part in payload:
        encoded = part.encode()
        chunks.append(f"${len(encoded)}\r\n".encode())
        chunks.append(encoded)
        chunks.append(b"\r\n")
    sock.sendall(b"".join(chunks))

    def read():
        prefix = reader.read(1)
        if prefix == b"+":
            return reader.readline()[:-2].decode()
        if prefix == b":":
            return int(reader.readline()[:-2])
        if prefix == b"$":
            length = int(reader.readline()[:-2])
            if length == -1:
                return None
            value = reader.read(length)
            reader.read(2)
            return value.decode()
        if prefix == b"*":
            length = int(reader.readline()[:-2])
            return [read() for _ in range(length)]
        if prefix == b"-":
            raise RuntimeError(reader.readline()[:-2].decode())
        raise RuntimeError(f"unsupported redis prefix: {prefix!r}")

    return read()

ready_count = 0
loading_count = 0
missing_users = []
for user_id in user_ids:
    ledger_key = f"{prefix}:ledger:{user_id}:{program_id}"
    loading_key = f"{prefix}:loading:{user_id}:{program_id}"
    ready = int(send("EXISTS", ledger_key))
    loading = int(send("EXISTS", loading_key))
    ready_count += ready
    loading_count += loading
    if ready == 0:
        missing_users.append(user_id)

reader.close()
sock.close()

sample = ",".join(missing_users[:10])
print(f"{ready_count}\t{loading_count}\t{len(missing_users)}\t{sample}")
PY
  )"

  ready_count="$(printf '%s\n' "${result}" | cut -f1)"
  loading_count="$(printf '%s\n' "${result}" | cut -f2)"
  missing_count="$(printf '%s\n' "${result}" | cut -f3)"
  missing_users="$(printf '%s\n' "${result}" | cut -f4)"

  if [[ "${missing_count}" == "0" ]]; then
    pass "order limit ledger ready: users=${ready_count} loading=${loading_count}"
    return 0
  fi

  if [[ "${MODE}" == "prepare" && "${PREWARM_ORDER_LIMIT_LEDGER_ON_PREPARE}" == "1" ]]; then
    require_cmd go
    log "prewarming order limit ledgers for user pool because missing=${missing_count}"
    USER_POOL_FILE="${user_pool_file}" \
    PROGRAM_ID="${PROGRAM_ID}" \
    REDIS_HOST="${REDIS_HOST}" \
    REDIS_PORT="${REDIS_PORT}" \
    go run ./scripts/perf/prewarm_order_limit_ledgers.go >/dev/null
    check_order_limit_ledgers "${user_pool_file}"
    return
  fi

  fail_check "order limit ledger missing: missing=${missing_count} sampleUserIds=${missing_users}"
  return 1
}

check_program_inventory() {
  local program_exists ticket_category_exists total_seats remain_number available_count order_count

  program_exists="$(mysql_scalar "SELECT COUNT(*) FROM damai_program.d_program WHERE id = ${PROGRAM_ID};")"
  ticket_category_exists="$(mysql_scalar "SELECT COUNT(*) FROM damai_program.d_ticket_category WHERE id = ${TICKET_CATEGORY_ID} AND program_id = ${PROGRAM_ID};")"
  total_seats="$(mysql_scalar "SELECT COUNT(*) FROM damai_program.d_seat WHERE program_id = ${PROGRAM_ID} AND ticket_category_id = ${TICKET_CATEGORY_ID} AND status = 1;")"
  remain_number="$(mysql_scalar "SELECT remain_number FROM damai_program.d_ticket_category WHERE id = ${TICKET_CATEGORY_ID};")"
  available_count="$(mysql_scalar "SELECT COUNT(*) FROM damai_program.d_seat WHERE program_id = ${PROGRAM_ID} AND ticket_category_id = ${TICKET_CATEGORY_ID} AND seat_status = 1 AND status = 1;")"
  order_count="$(mysql_scalar "SELECT COUNT(*) FROM damai_order.d_order;")"

  if [[ "${program_exists}" != "1" ]]; then
    fail_check "program missing: programId=${PROGRAM_ID}"
  else
    pass "program exists: programId=${PROGRAM_ID}"
  fi

  if [[ "${ticket_category_exists}" != "1" ]]; then
    fail_check "ticket category missing: ticketCategoryId=${TICKET_CATEGORY_ID}"
  else
    pass "ticket category exists: ticketCategoryId=${TICKET_CATEGORY_ID}"
  fi

  if [[ "${total_seats}" != "${REQUIRED_SEAT_COUNT}" ]]; then
    fail_check "seat total mismatch: expected=${REQUIRED_SEAT_COUNT} actual=${total_seats}"
  else
    pass "seat total matches target: ${total_seats}"
  fi

  if [[ "${remain_number}" != "${REQUIRED_SEAT_COUNT}" ]]; then
    fail_check "ticket remain_number mismatch: expected=${REQUIRED_SEAT_COUNT} actual=${remain_number}"
  else
    pass "ticket remain_number matches target: ${remain_number}"
  fi

  if [[ "${available_count}" != "${REQUIRED_SEAT_COUNT}" ]]; then
    fail_check "mysql available seat count mismatch: expected=${REQUIRED_SEAT_COUNT} actual=${available_count}"
  else
    pass "mysql available seat count matches target: ${available_count}"
  fi

  if (( REQUIRE_CLEAN_STATE == 1 )); then
    if [[ "${order_count}" != "0" ]]; then
      fail_check "order table not clean: d_order=${order_count}"
    else
      pass "order table clean"
    fi
  else
    warn "clean state check skipped: d_order=${order_count}"
  fi
}

check_redis_seat_ledger() {
  local result ready available_count available_zcard frozen_keys

  result="$(
    python3 - <<'PY' "${REDIS_HOST}" "${REDIS_PORT}" "${PROGRAM_ID}" "${TICKET_CATEGORY_ID}"
import socket
import sys

host = sys.argv[1]
port = int(sys.argv[2])
program_id = sys.argv[3]
ticket_category_id = sys.argv[4]

def command(*parts):
    sock = socket.create_connection((host, port), timeout=5)
    reader = sock.makefile("rb")
    payload = [str(part) for part in parts]
    chunks = [f"*{len(payload)}\r\n".encode()]
    for part in payload:
        encoded = part.encode()
        chunks.append(f"${len(encoded)}\r\n".encode())
        chunks.append(encoded)
        chunks.append(b"\r\n")
    sock.sendall(b"".join(chunks))

    def read():
        prefix = reader.read(1)
        if prefix == b"+":
            return reader.readline()[:-2].decode()
        if prefix == b":":
            return int(reader.readline()[:-2])
        if prefix == b"$":
            length = int(reader.readline()[:-2])
            if length == -1:
                return None
            value = reader.read(length)
            reader.read(2)
            return value.decode()
        if prefix == b"*":
            length = int(reader.readline()[:-2])
            return [read() for _ in range(length)]
        if prefix == b"-":
            raise RuntimeError(reader.readline()[:-2].decode())
        raise RuntimeError(f"unsupported redis reply prefix: {prefix!r}")

    output = read()
    reader.close()
    sock.close()
    return output

prefix = "damai-go"
stock_key = f"{prefix}:program:seat-ledger:stock:{program_id}:{ticket_category_id}"
available_key = f"{prefix}:program:seat-ledger:available:{program_id}:{ticket_category_id}"
frozen_pattern = f"{prefix}:program:seat-ledger:frozen:{program_id}:{ticket_category_id}:*"

ready = int(command("EXISTS", stock_key))
available_count = command("HGET", stock_key, "available_count") if ready else None
available_zcard = command("ZCARD", available_key) if ready else None
frozen_keys = command("KEYS", frozen_pattern) if ready else []
frozen_count = len(frozen_keys or [])

def normalized(value):
    if value is None:
        return ""
    return str(value)

print(f"{ready}\t{normalized(available_count)}\t{normalized(available_zcard)}\t{frozen_count}")
PY
  )"

  ready="$(printf '%s\n' "${result}" | cut -f1)"
  available_count="$(printf '%s\n' "${result}" | cut -f2)"
  available_zcard="$(printf '%s\n' "${result}" | cut -f3)"
  frozen_keys="$(printf '%s\n' "${result}" | cut -f4)"

  if [[ "${ready}" != "1" ]]; then
    if [[ "${MODE}" == "prepare" && "${PREWARM_ON_PREPARE}" == "1" && -n "${JWT}" && -n "${TICKET_USER_IDS}" ]]; then
      log "prewarming order ledgers because redis seat ledger is not ready"
      JWT="${JWT}" \
      GATEWAY_BASE_URL="${GATEWAY_BASE_URL}" \
      CHANNEL_CODE="${CHANNEL_CODE}" \
      PROGRAM_ID="${PROGRAM_ID}" \
      TICKET_CATEGORY_ID="${TICKET_CATEGORY_ID}" \
      TICKET_USER_IDS="${TICKET_USER_IDS}" \
      WARMUP_REQUESTS="${WARMUP_REQUESTS}" \
      bash "${ROOT_DIR}/scripts/perf/prewarm_order_ledgers.sh" >/dev/null
      check_redis_seat_ledger
      return
    fi

    fail_check "redis seat ledger not ready for programId=${PROGRAM_ID} ticketCategoryId=${TICKET_CATEGORY_ID}"
    return 1
  fi

  if [[ "${available_count}" != "${REQUIRED_SEAT_COUNT}" ]]; then
    fail_check "redis available_count mismatch: expected=${REQUIRED_SEAT_COUNT} actual=${available_count}"
  else
    pass "redis available_count matches target: ${available_count}"
  fi

  if [[ "${available_zcard}" != "${REQUIRED_SEAT_COUNT}" ]]; then
    fail_check "redis available zset mismatch: expected=${REQUIRED_SEAT_COUNT} actual=${available_zcard}"
  else
    pass "redis available zset matches target: ${available_zcard}"
  fi

  if [[ "${frozen_keys}" != "0" ]]; then
    fail_check "redis frozen seat keys not clean: frozen_keys=${frozen_keys}"
  else
    pass "redis frozen seat keys clean"
  fi
}

print_summary() {
  printf '\nSummary: pass=%d warn=%d fail=%d mode=%s\n' "${PASS_COUNT}" "${WARN_COUNT}" "${FAIL_COUNT}" "${MODE}"
  if [[ -n "${USER_POOL_FILE}" ]]; then
    printf 'UserPool: file=%s entries=%s uniqueUsers=%s uniqueTicketUsers=%s\n' \
      "${USER_POOL_FILE}" \
      "${USER_POOL_ENTRY_COUNT}" \
      "${USER_POOL_UNIQUE_USER_COUNT}" \
      "${USER_POOL_UNIQUE_TICKET_USER_COUNT}"
  fi
  if [[ -n "${JWT_USER_ID}" ]]; then
    printf 'JWT: userId=%s exp=%s\n' "${JWT_USER_ID}" "${JWT_EXPIRES_AT}"
  fi
}

main() {
  check_mode
  require_cmd docker
  require_cmd python3
  require_cmd curl
  require_cmd ss
  require_cmd awk
  require_cmd sort
  require_cmd uniq
  require_cmd paste

  check_container_running "${MYSQL_CONTAINER}" || true
  check_container_running "docker-compose-redis-1" || true
  check_container_running "docker-compose-etcd-1" || true
  check_container_running "docker-compose-kafka-1" || true

  check_mysql_connectivity || true
  check_redis_connectivity || true
  check_etcd_health || true
  check_kafka_topic || true
  check_kafka_lag || true
  check_ports_available || true

  if [[ -n "${USER_POOL_FILE}" ]]; then
    if check_user_pool_file "${USER_POOL_FILE}"; then
      pass "user pool valid: entries=${USER_POOL_ENTRY_COUNT} uniqueUsers=${USER_POOL_UNIQUE_USER_COUNT} uniqueTicketUsers=${USER_POOL_UNIQUE_TICKET_USER_COUNT}"
      check_user_pool_database_mapping "${USER_POOL_FILE}" || true
      check_order_limit_ledgers "${USER_POOL_FILE}" || true
    else
      fail_check "user pool invalid: ${USER_POOL_FILE}"
    fi
  else
    warn "user pool check skipped: USER_POOL_FILE not set"
  fi

  check_single_jwt || true
  check_program_inventory || true
  check_redis_seat_ledger || true

  print_summary

  if (( FAIL_COUNT > 0 )); then
    exit 1
  fi
}

if [[ "${ORDER_PRESSURE_PREFLIGHT_SOURCE_ONLY:-0}" != "1" ]]; then
  main "$@"
fi
