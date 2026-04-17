#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="scripts/deploy/rebuild_databases.sh"

fail() {
  printf '[rebuild-databases-test] ERROR: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  local pattern="$1"

  grep -F "${pattern}" "${SCRIPT_PATH}" >/dev/null || fail "missing pattern: ${pattern}"
}

[[ -f "${SCRIPT_PATH}" ]] || fail "script not found: ${SCRIPT_PATH}"

bash -n "${SCRIPT_PATH}"

assert_contains 'MYSQL_DB_USER="${MYSQL_DB_USER:-livepass_user}"'
assert_contains 'MYSQL_DB_PROGRAM="${MYSQL_DB_PROGRAM:-livepass_program}"'
assert_contains 'MYSQL_DB_ORDER="${MYSQL_DB_ORDER:-livepass_order}"'
assert_contains 'MYSQL_DB_PAY="${MYSQL_DB_PAY:-livepass_pay}"'
assert_contains 'MYSQL_DB_AGENTS="${MYSQL_DB_AGENTS:-livepass_agents}"'
assert_contains 'docker-compose-mysql-1'
assert_contains 'docker-compose-redis-1'
assert_contains 'docker-compose-kafka-1'
assert_contains 'DROP DATABASE IF EXISTS'
assert_contains 'scripts/import_sql.sh'
assert_contains 'redis-cli'
assert_contains 'FLUSHDB'
assert_contains 'ticketing.attempt.command.v1'
assert_contains 'kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --delete'
assert_contains 'kafka-topics.sh --bootstrap-server 127.0.0.1:9092 --create'

printf '[rebuild-databases-test] ok\n'
