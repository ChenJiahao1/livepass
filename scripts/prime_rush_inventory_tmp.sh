#!/usr/bin/env bash

# debug-only helper for manual troubleshooting; production flow should use jobs/rush-inventory-preheat
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <showTimeId> [order_config] [program_config]" >&2
  exit 1
fi

SHOW_TIME_ID="$1"
ORDER_CONFIG="${2:-services/order-rpc/etc/order.yaml}"
PROGRAM_CONFIG="${3:-services/program-rpc/etc/program.yaml}"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT_DIR"

echo "debug-only: prime_rush_inventory_tmp.sh is for manual troubleshooting, not production flow" >&2

go run ./services/order-rpc/cmd/prime_admission_quota_tmp --showTimeId "$SHOW_TIME_ID" --config "$ORDER_CONFIG"
go run ./services/program-rpc/cmd/prime_program_seat_ledger_tmp --showTimeId "$SHOW_TIME_ID" --config "$PROGRAM_CONFIG"
