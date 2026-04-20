#!/usr/bin/env bash
set -euo pipefail

# 用途：
#   重建运行数据，不负责启动服务。
#
# 默认行为：
#   - 删除并重建 user / program / order / pay / agents MySQL 业务库
#   - 重新导入 schema 与 seed
#   - 清空 Redis 指定 DB
#   - 删除并重建 Kafka 业务 Topic
#
# 常用环境变量：
#   MYSQL_CONTAINER              MySQL 容器名，默认 docker-compose-mysql-1
#   MYSQL_USER / MYSQL_PASSWORD  MySQL 凭据
#   MYSQL_DB_USER...             各业务库名覆盖
#   REDIS_CONTAINER              Redis 容器名，默认 docker-compose-redis-1
#   REDIS_DB                     要清空的 Redis DB，默认 0
#   KAFKA_CONTAINER              Kafka 容器名，默认 docker-compose-kafka-1
#   KAFKA_TOPICS                 需要重建的 Topic，逗号分隔
#   KAFKA_TOPIC_PARTITIONS       Topic 分区数，默认 5
#   KAFKA_TOPIC_DELETE_TIMEOUT_SECONDS  等待 Topic 删除完成的秒数，默认 30
#
# 常用示例：
#   bash scripts/deploy/rebuild_databases.sh
#   REDIS_DB=1 bash scripts/deploy/rebuild_databases.sh
#   KAFKA_TOPICS=ticketing.attempt.command.v1 bash scripts/deploy/rebuild_databases.sh
#
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

MYSQL_CONTAINER="${MYSQL_CONTAINER:-docker-compose-mysql-1}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-${MYSQL_ROOT_PASSWORD:-123456}}"
MYSQL_DB_USER="${MYSQL_DB_USER:-livepass_user}"
MYSQL_DB_PROGRAM="${MYSQL_DB_PROGRAM:-livepass_program}"
MYSQL_DB_ORDER="${MYSQL_DB_ORDER:-livepass_order}"
MYSQL_DB_PAY="${MYSQL_DB_PAY:-livepass_pay}"
MYSQL_DB_AGENTS="${MYSQL_DB_AGENTS:-livepass_agents}"

REDIS_CONTAINER="${REDIS_CONTAINER:-docker-compose-redis-1}"
REDIS_DB="${REDIS_DB:-0}"

KAFKA_CONTAINER="${KAFKA_CONTAINER:-docker-compose-kafka-1}"
KAFKA_TOPICS="${KAFKA_TOPICS:-ticketing.attempt.command.v1}"
KAFKA_TOPIC_PARTITIONS="${KAFKA_TOPIC_PARTITIONS:-5}"
KAFKA_REPLICATION_FACTOR="${KAFKA_REPLICATION_FACTOR:-1}"
KAFKA_TOPIC_DELETE_TIMEOUT_SECONDS="${KAFKA_TOPIC_DELETE_TIMEOUT_SECONDS:-30}"

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

kafka_topic_exists() {
  local topic="$1"

  docker exec "${KAFKA_CONTAINER}" /opt/kafka/bin/kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --list \
    | grep -Fx "${topic}" >/dev/null
}

wait_for_kafka_topic_deleted() {
  local topic="$1"
  local i

  for ((i = 0; i < KAFKA_TOPIC_DELETE_TIMEOUT_SECONDS; i++)); do
    if ! kafka_topic_exists "${topic}"; then
      return 0
    fi
    sleep 1
  done

  fail "kafka topic delete timeout: ${topic}"
}

recreate_kafka_topics() {
  local raw_topic topic

  IFS=',' read -r -a topics <<< "${KAFKA_TOPICS}"
  for raw_topic in "${topics[@]}"; do
    topic="${raw_topic//[[:space:]]/}"
    [[ -n "${topic}" ]] || continue

    log "recreate kafka topic: ${topic}"
    docker exec "${KAFKA_CONTAINER}" /opt/kafka/bin/kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --delete --topic "${topic}" >/dev/null 2>&1 || true
    wait_for_kafka_topic_deleted "${topic}"
    docker exec "${KAFKA_CONTAINER}" /opt/kafka/bin/kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --create --if-not-exists --topic "${topic}" --partitions "${KAFKA_TOPIC_PARTITIONS}" --replication-factor "${KAFKA_REPLICATION_FACTOR}" >/dev/null
    docker exec "${KAFKA_CONTAINER}" /opt/kafka/bin/kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --describe --topic "${topic}" >/dev/null
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
