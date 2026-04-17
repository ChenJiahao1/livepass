#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-compose-mysql-1}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-${MYSQL_ROOT_PASSWORD:-123456}}"
MYSQL_DB_USER="${MYSQL_DB_USER:-damai_user}"
MYSQL_DB_PROGRAM="${MYSQL_DB_PROGRAM:-damai_program}"
MYSQL_DB_ORDER="${MYSQL_DB_ORDER:-damai_order}"
MYSQL_DB_PAY="${MYSQL_DB_PAY:-damai_pay}"
MYSQL_DB_AGENTS="${MYSQL_DB_AGENTS:-damai_agents}"

REDIS_CONTAINER="${REDIS_CONTAINER:-docker-compose-redis-1}"
REDIS_DB="${REDIS_DB:-0}"

KAFKA_CONTAINER="${KAFKA_CONTAINER:-docker-compose-kafka-1}"
KAFKA_TOPICS="${KAFKA_TOPICS:-ticketing.attempt.command.v1}"
KAFKA_TOPIC_PARTITIONS="${KAFKA_TOPIC_PARTITIONS:-5}"
KAFKA_REPLICATION_FACTOR="${KAFKA_REPLICATION_FACTOR:-1}"

log() {
  printf '[rebuild-databases] %s\n' "$*"
}

fail() {
  printf '[rebuild-databases] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  local name="$1"

  command -v "${name}" >/dev/null 2>&1 || fail "missing dependency: ${name}"
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

ensure_runtime_dependencies() {
  require_cmd docker

  check_container_running "${MYSQL_CONTAINER}" || fail "mysql container not running: ${MYSQL_CONTAINER}"
  check_container_running "${REDIS_CONTAINER}" || fail "redis container not running: ${REDIS_CONTAINER}"
  check_container_running "${KAFKA_CONTAINER}" || fail "kafka container not running: ${KAFKA_CONTAINER}"
}

rebuild_mysql_databases() {
  local databases=(
    "${MYSQL_DB_USER}"
    "${MYSQL_DB_PROGRAM}"
    "${MYSQL_DB_ORDER}"
    "${MYSQL_DB_PAY}"
    "${MYSQL_DB_AGENTS}"
  )
  local database

  for database in "${databases[@]}"; do
    log "rebuild mysql database: ${database}"
    mysql_exec -e "DROP DATABASE IF EXISTS \`${database}\`"
    mysql_exec -e "CREATE DATABASE \`${database}\` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"
  done

  log "re-import mysql schema and seed"
  bash "${REPO_ROOT}/scripts/import_sql.sh"
}

flush_redis() {
  log "flush redis db ${REDIS_DB}"
  docker exec "${REDIS_CONTAINER}" redis-cli -n "${REDIS_DB}" FLUSHDB >/dev/null
}

recreate_kafka_topics() {
  local raw_topic topic

  IFS=',' read -r -a topics <<< "${KAFKA_TOPICS}"
  for raw_topic in "${topics[@]}"; do
    topic="${raw_topic//[[:space:]]/}"
    [[ -n "${topic}" ]] || continue

    log "recreate kafka topic: ${topic}"
    docker exec "${KAFKA_CONTAINER}" /opt/kafka/bin/kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --delete --topic "${topic}" >/dev/null 2>&1 || true
    sleep 1
    docker exec "${KAFKA_CONTAINER}" /opt/kafka/bin/kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --create --if-not-exists --topic "${topic}" --partitions "${KAFKA_TOPIC_PARTITIONS}" --replication-factor "${KAFKA_REPLICATION_FACTOR}" >/dev/null
  done
}

main() {
  ensure_runtime_dependencies
  rebuild_mysql_databases
  flush_redis
  recreate_kafka_topics
  log "runtime data rebuild finished"
}

main "$@"
