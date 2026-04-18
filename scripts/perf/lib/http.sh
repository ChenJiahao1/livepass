#!/usr/bin/env bash
set -euo pipefail

PERF_HTTP_STATUS=""
PERF_HTTP_BODY=""

perf_http_post_as_user() {
  local path="$1"
  local payload="$2"
  local user_id="$3"
  local tmp_file
  local http_code

  tmp_file="$(mktemp)"
  if ! http_code="$(
    curl -sS \
      -o "${tmp_file}" \
      -w "%{http_code}" \
      -X POST "${BASE_URL}${path}" \
      -H "Content-Type: application/json" \
      -H "${PERF_HEADER_NAME}: ${PERF_SECRET}" \
      -H "${PERF_USER_ID_HEADER}: ${user_id}" \
      -d "${payload}"
  )"; then
    rm -f "${tmp_file}"
    perf_fail "request failed: ${path}"
  fi

  PERF_HTTP_STATUS="${http_code}"
  PERF_HTTP_BODY="$(cat "${tmp_file}")"
  rm -f "${tmp_file}"
}
