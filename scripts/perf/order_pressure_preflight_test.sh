#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TARGET_SCRIPT="${ROOT_DIR}/scripts/perf/order_pressure_preflight.sh"

fail() {
  printf '[order-pressure-preflight-test] ERROR: %s\n' "$*" >&2
  exit 1
}

assert_eq() {
  local actual="$1"
  local expected="$2"
  local label="$3"

  if [[ "${actual}" != "${expected}" ]]; then
    fail "${label}: expected=${expected} actual=${actual}"
  fi
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local label="$3"

  if [[ "${haystack}" != *"${needle}"* ]]; then
    fail "${label}: expected output to contain '${needle}', got '${haystack}'"
  fi
}

make_token() {
  local user_id="$1"
  local expires_at="$2"

  python3 - <<'PY' "${user_id}" "${expires_at}"
import base64
import hashlib
import hmac
import json
import sys

user_id = int(sys.argv[1])
expires_at = int(sys.argv[2])
header = {"alg": "HS256", "typ": "JWT"}
payload = {"userId": user_id, "exp": expires_at, "iat": expires_at - 3600}
secret = b"local-user-secret-0001"

def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()

signing_input = (
    f"{b64url(json.dumps(header, separators=(',', ':')).encode())}."
    f"{b64url(json.dumps(payload, separators=(',', ':')).encode())}"
)
signature = hmac.new(secret, signing_input.encode(), hashlib.sha256).digest()
print(f"{signing_input}.{b64url(signature)}")
PY
}

write_user_pool() {
  local output_file="$1"
  local user_id_a="$2"
  local ticket_user_id_a="$3"
  local user_id_b="$4"
  local ticket_user_id_b="$5"
  local expires_at

  expires_at="$(( $(date +%s) + 3600 ))"
  cat >"${output_file}" <<EOF
[
  {
    "jwt": "$(make_token "${user_id_a}" "${expires_at}")",
    "ticketUserId": "${ticket_user_id_a}"
  },
  {
    "jwt": "$(make_token "${user_id_b}" "${expires_at}")",
    "ticketUserId": "${ticket_user_id_b}"
  }
]
EOF
}

write_multi_ticket_user_pool() {
  local output_file="$1"
  local expires_at

  expires_at="$(( $(date +%s) + 3600 ))"
  cat >"${output_file}" <<EOF
[
  {
    "jwt": "$(make_token 101 "${expires_at}")",
    "ticketUserIds": ["1001", "1002", "1003"]
  },
  {
    "jwt": "$(make_token 202 "${expires_at}")",
    "ticketUserIds": ["2001", "2002", "2003"]
  }
]
EOF
}

redis_command() {
  python3 - <<'PY' "$@"
import socket
import sys

host = "127.0.0.1"
port = 6379
parts = sys.argv[1:]
sock = socket.create_connection((host, port), timeout=5)
reader = sock.makefile("rb")
chunks = [f"*{len(parts)}\r\n".encode()]
for part in parts:
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
        raise SystemExit(reader.readline()[:-2].decode())
    raise SystemExit(f"unsupported prefix: {prefix!r}")

result = read()
if isinstance(result, list):
    for item in result:
        print(item)
elif result is not None:
    print(result)

reader.close()
sock.close()
PY
}

test_check_user_pool_file_reports_unique_counts() {
  local tmpdir pool_file

  tmpdir="$(mktemp -d)"
  pool_file="${tmpdir}/user_pool.json"
  write_user_pool "${pool_file}" 101 1001 202 2002

  ORDER_PRESSURE_PREFLIGHT_SOURCE_ONLY=1 source "${TARGET_SCRIPT}"

  check_user_pool_file "${pool_file}"

  assert_eq "${USER_POOL_ENTRY_COUNT}" "2" "user pool entry count"
  assert_eq "${USER_POOL_UNIQUE_USER_COUNT}" "2" "user pool unique user count"
  assert_eq "${USER_POOL_UNIQUE_TICKET_USER_COUNT}" "2" "user pool unique ticket user count"
}

test_check_user_pool_file_rejects_duplicate_user_ids() {
  local tmpdir pool_file output_file

  tmpdir="$(mktemp -d)"
  pool_file="${tmpdir}/user_pool.json"
  output_file="${tmpdir}/stderr.log"
  write_user_pool "${pool_file}" 303 3003 303 3004

  ORDER_PRESSURE_PREFLIGHT_SOURCE_ONLY=1 source "${TARGET_SCRIPT}"

  if check_user_pool_file "${pool_file}" > /dev/null 2>"${output_file}"; then
    fail "expected duplicate user pool validation to fail"
  fi

  assert_contains "$(cat "${output_file}")" "duplicate userId" "duplicate user id failure message"
}

test_check_user_pool_file_accepts_multi_ticket_user_ids() {
  local tmpdir pool_file

  tmpdir="$(mktemp -d)"
  pool_file="${tmpdir}/user_pool_multi.json"
  write_multi_ticket_user_pool "${pool_file}"

  ORDER_PRESSURE_PREFLIGHT_SOURCE_ONLY=1 source "${TARGET_SCRIPT}"

  check_user_pool_file "${pool_file}"

  assert_eq "${USER_POOL_ENTRY_COUNT}" "2" "multi user pool entry count"
  assert_eq "${USER_POOL_UNIQUE_USER_COUNT}" "2" "multi user pool unique user count"
  assert_eq "${USER_POOL_UNIQUE_TICKET_USER_COUNT}" "6" "multi user pool unique ticket user count"
}

test_check_order_limit_ledgers_reports_missing_entries() {
  local tmpdir pool_file output_file

  tmpdir="$(mktemp -d)"
  pool_file="${tmpdir}/user_pool_multi.json"
  output_file="${tmpdir}/output.log"
  write_multi_ticket_user_pool "${pool_file}"

  redis_command DEL \
    "damai-go:order:purchase-limit:ledger:101:10001" \
    "damai-go:order:purchase-limit:loading:101:10001" \
    "damai-go:order:purchase-limit:ledger:202:10001" \
    "damai-go:order:purchase-limit:loading:202:10001" >/dev/null || true

  ORDER_PRESSURE_PREFLIGHT_SOURCE_ONLY=1 source "${TARGET_SCRIPT}"

  if check_order_limit_ledgers "${pool_file}" >"${output_file}" 2>&1; then
    fail "expected missing order limit ledger check to fail"
  fi

  assert_contains "$(cat "${output_file}")" "order limit ledger" "missing order limit ledger failure message"
}

test_check_order_limit_ledgers_accepts_ready_entries() {
  local tmpdir pool_file

  tmpdir="$(mktemp -d)"
  pool_file="${tmpdir}/user_pool_multi.json"
  write_multi_ticket_user_pool "${pool_file}"

  redis_command DEL \
    "damai-go:order:purchase-limit:loading:101:10001" \
    "damai-go:order:purchase-limit:loading:202:10001" >/dev/null || true
  redis_command HSET "damai-go:order:purchase-limit:ledger:101:10001" active_count 0 >/dev/null
  redis_command HSET "damai-go:order:purchase-limit:ledger:202:10001" active_count 0 >/dev/null
  redis_command EXPIRE "damai-go:order:purchase-limit:ledger:101:10001" 3600 >/dev/null
  redis_command EXPIRE "damai-go:order:purchase-limit:ledger:202:10001" 3600 >/dev/null

  ORDER_PRESSURE_PREFLIGHT_SOURCE_ONLY=1 source "${TARGET_SCRIPT}"

  check_order_limit_ledgers "${pool_file}"
}

main() {
  test_check_user_pool_file_reports_unique_counts
  test_check_user_pool_file_rejects_duplicate_user_ids
  test_check_user_pool_file_accepts_multi_ticket_user_ids
  test_check_order_limit_ledgers_reports_missing_entries
  test_check_order_limit_ledgers_accepts_ready_entries
  printf '[order-pressure-preflight-test] PASS\n'
}

main "$@"
