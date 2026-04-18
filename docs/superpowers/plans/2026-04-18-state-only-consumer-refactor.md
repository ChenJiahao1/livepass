# State-Only Consumer Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将订单消费端重构为以 `attempt.state` 作为唯一处理权来源，删除 `processing_epoch` 与基于 epoch 的 fencing。

**Architecture:** `/order/create` 仍负责 admission 并写入 `ACCEPTED`；consumer 通过 Redis Lua 原子执行 `ACCEPTED -> PROCESSING`，只有返回 `shouldProcess=true` 的 consumer 继续落单。lease 只续 `PROCESSING` attempt TTL；成功/失败 finalize 只允许 `PROCESSING -> SUCCESS/FAILED`。Program 锁座协议改为确定性 `freezeToken = showTimeId + ticketCategoryId + orderNumber` 与显式 `freezeExpireTime`。

**Tech Stack:** Go、go-zero RPC、Redis Lua、go test、Protocol Buffers/goctl 生成代码。

---

### Task 1: 锁定 state-only attempt 行为

**Files:**
- Modify: `services/order-rpc/tests/integration/rush_attempt_store_test.go`
- Modify: `services/order-rpc/internal/rush/prepare_attempt_for_consume.lua`
- Modify: `services/order-rpc/internal/rush/finalize_success.lua`
- Modify: `services/order-rpc/internal/rush/finalize_failure.lua`
- Modify: `services/order-rpc/internal/rush/refresh_processing_lease.lua`
- Modify: `services/order-rpc/internal/rush/attempt_store.go`
- Modify: `services/order-rpc/internal/rush/attempt_record.go`

- [ ] Write failing tests: `PrepareAttemptForConsume` returns record + shouldProcess without epoch; second prepare skips.
- [ ] Write failing tests: `RefreshProcessingLease` succeeds only while state is `PROCESSING`.
- [ ] Write failing tests: finalize success/failure only transition from `PROCESSING` and are idempotent on terminal states.
- [ ] Remove `processing_epoch` from attempt record parsing, admit Lua, prepare Lua, finalize Lua and lease Lua.
- [ ] Remove `ClaimProcessing` API and tests that model re-claiming.
- [ ] Run focused order-rpc rush tests.

### Task 2: 重构 consumer 为 state-only

**Files:**
- Modify: `services/order-rpc/internal/logic/create_order_consumer_logic.go`
- Modify: `services/order-rpc/internal/logic/create_order_processing_lease.go`
- Modify: `services/order-rpc/internal/logic/create_order_consumer_finalize.go`
- Modify: `services/order-rpc/tests/integration/create_order_consumer_rush_logic_test.go`

- [ ] Write failing test: consumer freeze token no longer带 epoch。
- [ ] Write failing test: missing attempt no longer通过 embedded snapshots 继续消费。
- [ ] Change consumer prepare result from `(record, epoch, shouldProcess)` to `(record, shouldProcess)`.
- [ ] Change lease start/refresh to orderNumber-only state lease.
- [ ] Change finalize calls to state-only APIs.
- [ ] Remove epoch-specific retry/fencing helpers.

### Task 3: 重构 freeze token 与 Program 协议

**Files:**
- Modify: `pkg/seatfreeze/freeze_token.go`
- Modify: `pkg/seatfreeze/tests/freeze_token_test.go`
- Modify: `services/program-rpc/program.proto`
- Generated: `services/program-rpc/pb/program.pb.go`
- Generated: `services/program-rpc/pb/program_grpc.pb.go`
- Generated: `services/program-rpc/programrpc/program_rpc.go`
- Generated: `services/program-rpc/internal/server/program_rpc_server.go`
- Modify: `services/program-rpc/internal/logic/auto_assign_and_freeze_seats_logic.go`
- Modify: `services/program-rpc/tests/integration/*_test.go`

- [ ] Write failing test: `seatfreeze.FormatToken` 格式不包含 epoch。
- [ ] Change token parser to `freeze-st<showTimeID>-tc<ticketCategoryID>-o<orderNumber>`.
- [ ] Replace `freezeSeconds` with `freezeExpireTime` in Program RPC request.
- [ ] Program validates explicit expire time and persists it directly.
- [ ] Regenerate go-zero RPC code with `--style go_zero` if tooling is available; otherwise apply minimal generated diff.

### Task 4: 更新调用点和文档

**Files:**
- Modify: order/program integration tests under `services/*/tests/integration/`
- Modify: `docs/architecture/order-create-accept-async.md`
- Modify: `docs/architecture/order-redis-key.md`

- [ ] Replace all test fixtures from `FreezeSeconds` to `FreezeExpireTime`.
- [ ] Replace all expected freeze token strings to epoch-less format.
- [ ] Remove docs references to `processing_epoch` and epoch freeze token.
- [ ] Run focused tests, then broader affected package tests.
