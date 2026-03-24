#!/usr/bin/env bash
set -euo pipefail

KAFKA_BOOTSTRAP="${KAFKA_BOOTSTRAP:-127.0.0.1:9094}"
TOPIC="${TOPIC:-order.create.command.v1}"
PARTITIONS="${PARTITIONS:-5}"
REPLICATION_FACTOR="${REPLICATION_FACTOR:-1}"
DOCKER_COMPOSE_FILE="${DOCKER_COMPOSE_FILE:-deploy/docker-compose/docker-compose.infrastructure.yml}"
KAFKA_SERVICE="${KAFKA_SERVICE:-kafka}"
KAFKA_TOPICS_BIN="${KAFKA_TOPICS_BIN:-}"

log() {
  printf '[prepare-order-kafka-topic] %s\n' "$*"
}

fail() {
  printf '[prepare-order-kafka-topic] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  local name="$1"

  command -v "${name}" >/dev/null 2>&1 || fail "missing dependency: ${name}"
}

run_kafka_topics() {
  if [[ -n "${KAFKA_TOPICS_BIN}" ]]; then
    "${KAFKA_TOPICS_BIN}" "$@"
    return
  fi

  if command -v kafka-topics.sh >/dev/null 2>&1; then
    kafka-topics.sh "$@"
    return
  fi

  require_cmd docker
  docker compose -f "${DOCKER_COMPOSE_FILE}" exec -T "${KAFKA_SERVICE}" /opt/kafka/bin/kafka-topics.sh "$@"
}

get_current_partitions() {
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

main() {
  local current_partitions final_partitions

  [[ "${PARTITIONS}" =~ ^[0-9]+$ ]] || fail "PARTITIONS must be a positive integer"
  (( PARTITIONS > 0 )) || fail "PARTITIONS must be greater than 0"
  [[ "${REPLICATION_FACTOR}" =~ ^[0-9]+$ ]] || fail "REPLICATION_FACTOR must be a positive integer"
  (( REPLICATION_FACTOR > 0 )) || fail "REPLICATION_FACTOR must be greater than 0"

  current_partitions="$(get_current_partitions)"
  log "topic=${TOPIC} current_partitions=${current_partitions} target_partitions=${PARTITIONS}"

  if (( current_partitions == 0 )); then
    log "creating topic ${TOPIC}"
    run_kafka_topics \
      --bootstrap-server "${KAFKA_BOOTSTRAP}" \
      --create \
      --if-not-exists \
      --topic "${TOPIC}" \
      --partitions "${PARTITIONS}" \
      --replication-factor "${REPLICATION_FACTOR}"
  elif (( current_partitions < PARTITIONS )); then
    log "expanding topic ${TOPIC} partitions ${current_partitions} -> ${PARTITIONS}"
    run_kafka_topics \
      --bootstrap-server "${KAFKA_BOOTSTRAP}" \
      --alter \
      --topic "${TOPIC}" \
      --partitions "${PARTITIONS}"
  else
    log "keeping existing topic ${TOPIC} with ${current_partitions} partitions"
  fi

  final_partitions="$(get_current_partitions)"
  if (( final_partitions < PARTITIONS )); then
    fail "topic ${TOPIC} partitions still below target: current=${final_partitions} target=${PARTITIONS}"
  fi

  log "topic ${TOPIC} ready with ${final_partitions} partitions"
}

main "$@"
