#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GO_TEST_FLAGS="${GO_TEST_FLAGS:-}"
GO_TEST_PARALLEL="${GO_TEST_PARALLEL:-1}"

log() {
  printf '[order-gene-migration] %s\n' "$*"
}

run_go_test() {
  local pkg="$1"
  local regex="$2"

  log "go test ${pkg} -run ${regex}"
  (
    cd "${ROOT_DIR}"
    go test "${pkg}" -run "${regex}" -count=1 -parallel "${GO_TEST_PARALLEL}" ${GO_TEST_FLAGS}
  )
}

run_go_test ./services/order-rpc/repository 'TestDualWriteOrderRepositoryWritesLegacyAndShardTables|TestDualWriteOrderRepositoryReadsShardWhenRouteStatusPrimaryNew|TestDualWriteOrderRepositoryReadsLegacyWhenRouteStatusRollback'
run_go_test ./jobs/order-close/tests/integration 'TestRunOnceForwardsSlotWindowAndAdvancesCheckpoint'
run_go_test ./jobs/order-migrate/tests/integration '.'

log 'migration acceptance passed'
