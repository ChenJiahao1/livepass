#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_DIR="${ROOT_DIR}/agents/app/rpc/generated"

mkdir -p "${OUT_DIR}"

uv run python -m grpc_tools.protoc \
  -I"${ROOT_DIR}/services/order-rpc" \
  --python_out="${OUT_DIR}" \
  --grpc_python_out="${OUT_DIR}" \
  "${ROOT_DIR}/services/order-rpc/order.proto"

uv run python -m grpc_tools.protoc \
  -I"${ROOT_DIR}/services/program-rpc" \
  --python_out="${OUT_DIR}" \
  --grpc_python_out="${OUT_DIR}" \
  "${ROOT_DIR}/services/program-rpc/program.proto"

uv run python -m grpc_tools.protoc \
  -I"${ROOT_DIR}/services/user-rpc" \
  --python_out="${OUT_DIR}" \
  --grpc_python_out="${OUT_DIR}" \
  "${ROOT_DIR}/services/user-rpc/user.proto"

sed -i 's/^import order_pb2 as/from . import order_pb2 as/' "${OUT_DIR}/order_pb2_grpc.py"
sed -i 's/^import program_pb2 as/from . import program_pb2 as/' "${OUT_DIR}/program_pb2_grpc.py"
sed -i 's/^import user_pb2 as/from . import user_pb2 as/' "${OUT_DIR}/user_pb2_grpc.py"
