#!/usr/bin/env bash
set -euo pipefail

SCRIPT_PATH="scripts/import_sql.sh"

fail() {
  printf '[import-sql-script-test] ERROR: %s\n' "$*" >&2
  exit 1
}

assert_contains() {
  local pattern="$1"
  grep -F "${pattern}" "${SCRIPT_PATH}" >/dev/null || fail "missing pattern: ${pattern}"
}

[[ -f "${SCRIPT_PATH}" ]] || fail "script not found: ${SCRIPT_PATH}"

bash -n "${SCRIPT_PATH}"
assert_contains 'IMPORT_DOMAINS="${IMPORT_DOMAINS:-user,program,order,pay,agents}"'

printf '[import-sql-script-test] ok\n'
