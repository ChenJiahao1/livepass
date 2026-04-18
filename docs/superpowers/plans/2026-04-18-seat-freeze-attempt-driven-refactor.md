# Seat Freeze Attempt-Driven Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将座位冻结链路重构为由 `attempt` 状态机统一控制，并将 `freezeToken` 降级为 `orderNumber + processingEpoch` 派生的资源标识。

**Architecture:** `order-rpc` 继续作为控制面，负责消费抢占、lease、success/failure 以及失败补偿资格；`program-rpc` 仅保留基于 seat ledger 的冻结、释放、确认能力，不再维护独立冻结状态机与过期扫描。重试幂等通过确定性 `freezeToken`、Redis `frozen` 集合及 DB 冻结状态对账保证。

**Tech Stack:** Go, go-zero, gRPC, Redis Lua, MySQL, protobuf, Go integration tests

---

### Task 1: 重塑契约与冻结标识语义

**Files:**
- Modify: `services/program-rpc/program.proto`
- Modify: `services/order-rpc/internal/logic/create_order_consumer_logic.go`
- Modify: `services/program-rpc/internal/logic/auto_assign_and_freeze_seats_logic.go`
- Test: `services/program-rpc/tests/integration/auto_assign_and_freeze_seats_logic_test.go`

- [ ] **Step 1: 写失败测试**
- [ ] **Step 2: 运行测试确认因旧契约/旧语义失败**
- [ ] **Step 3: 调整 proto 与 consumer，改为显式传入确定性 `freezeToken`**
- [ ] **Step 4: 运行相关测试确认通过**

### Task 2: 删除 program 侧冻结状态机

**Files:**
- Modify: `services/program-rpc/internal/seatcache/seat_stock_store.go`
- Delete: `services/program-rpc/internal/seatcache/seat_freeze_metadata.go`
- Modify: `services/program-rpc/internal/logic/confirm_seat_freeze_logic.go`
- Modify: `services/program-rpc/internal/logic/release_seat_freeze_logic.go`
- Modify: `services/program-rpc/internal/logic/auto_assign_and_freeze_seats_logic.go`
- Test: `services/program-rpc/tests/integration/confirm_seat_freeze_logic_test.go`
- Test: `services/program-rpc/tests/integration/release_seat_freeze_logic_test.go`

- [ ] **Step 1: 写失败测试覆盖无 metadata 仍可确认/释放/重试**
- [ ] **Step 2: 运行测试确认旧实现失败**
- [ ] **Step 3: 删除 metadata/index/requestNo 依赖并重写确认/释放校验**
- [ ] **Step 4: 运行相关测试确认通过**

### Task 3: 以 attempt CAS 驱动补偿链路

**Files:**
- Modify: `services/order-rpc/internal/logic/create_order_consumer_logic.go`
- Modify: `services/order-rpc/internal/logic/order_domain_helper.go`
- Modify: `services/order-rpc/internal/logic/rush_attempt_release_helper.go`
- Test: `services/order-rpc/tests/integration/create_order_consumer_rush_logic_test.go`
- Test: `services/order-rpc/tests/integration/order_guard_outbox_integration_test.go`

- [ ] **Step 1: 写失败测试覆盖同 epoch 重试、失败补偿释放、关单释放**
- [ ] **Step 2: 运行测试确认旧实现失败**
- [ ] **Step 3: 改为 attempt 派生 token 与补偿释放**
- [ ] **Step 4: 运行相关测试确认通过**

### Task 4: 清理冗余实现并做回归验证

**Files:**
- Modify: `services/program-rpc/tests/integration/seat_inventory_logic_test.go`
- Modify: `services/program-rpc/tests/integration/program_test_helpers_test.go`
- Modify: `services/program-rpc/internal/logic/program_management_helper.go`
- Test: `services/program-rpc/tests/integration/...`
- Test: `services/order-rpc/tests/integration/...`

- [ ] **Step 1: 删除过期扫描与旧 metadata 测试夹具**
- [ ] **Step 2: 更新辅助函数与清理逻辑**
- [ ] **Step 3: 运行 program/order 相关集成测试回归**
- [ ] **Step 4: 记录残余风险并收尾**
