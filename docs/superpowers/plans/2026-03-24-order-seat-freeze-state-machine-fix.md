# Order Seat Freeze State Machine Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复支付确认与取消释放在 `freezeToken` 状态机上的异常语义，避免已确认锁座被误取消，同时补齐必要的 `orderNumber` 分布式串行化。

**Architecture:** 保留数据库作为订单最终状态裁决，继续由 `program-rpc` 负责座位账本状态迁移。本次只修正 `ConfirmSeatFreeze` / `ReleaseSeatFreeze` 的状态机语义，并在 `order-rpc` 的支付与取消入口评估复用现有 etcd guard 做按 `orderNumber` 的分布式串行化。

**Tech Stack:** Go, go-zero, gRPC, etcd concurrency mutex, MySQL, Redis, Go integration tests

---

### Task 1: 固化状态机回归用例

**Files:**
- Modify: `services/program-rpc/tests/integration/confirm_seat_freeze_logic_test.go`
- Modify: `services/program-rpc/tests/integration/release_seat_freeze_logic_test.go`

- [ ] **Step 1: 写失败测试**

为以下语义补测试：
- `ConfirmSeatFreeze` 对 `confirmed` 状态幂等成功
- `ReleaseSeatFreeze` 对 `confirmed` 状态返回失败前置条件，不能静默成功

- [ ] **Step 2: 运行测试确认先失败**

Run: `go test ./services/program-rpc/tests/integration -run 'TestConfirmSeatFreeze|TestReleaseSeatFreeze' -count=1`
Expected: 新增用例失败，且失败原因与当前状态机语义不符一致。

### Task 2: 修正 program-rpc 状态机

**Files:**
- Modify: `services/program-rpc/internal/logic/confirm_seat_freeze_logic.go`
- Modify: `services/program-rpc/internal/logic/release_seat_freeze_logic.go`

- [ ] **Step 1: 最小实现 confirm 幂等**

当 metadata 已是 `confirmed` 时直接返回成功，不再报 `FailedPrecondition`。

- [ ] **Step 2: 最小实现 release 拒绝 confirmed**

当 metadata 已是 `confirmed` 时返回 `seat freeze status invalid`，禁止订单侧把“已售座位”误当成可释放冻结。

- [ ] **Step 3: 运行 program-rpc 测试**

Run: `go test ./services/program-rpc/tests/integration -run 'TestConfirmSeatFreeze|TestReleaseSeatFreeze' -count=1`
Expected: 所有相关测试通过。

### Task 3: 评估并补 order-rpc 串行化

**Files:**
- Modify: `services/order-rpc/internal/repeatguard/guard.go`
- Modify: `services/order-rpc/internal/logic/pay_order_logic.go`
- Modify: `services/order-rpc/internal/logic/order_domain_helper.go`
- Modify: `services/order-rpc/tests/integration/pay_order_logic_test.go`
- Modify: `services/order-rpc/tests/integration/cancel_order_logic_test.go`

- [ ] **Step 1: 写失败测试或探针测试**

补最小测试证明支付、取消入口会尝试获取按 `orderNumber` 的 guard key。

- [ ] **Step 2: 复用现有 guard**

新增按 `orderNumber` 的 key builder，并在支付/取消入口调用 `RepeatGuard.Lock`，只做前置串行化，不替代数据库状态检查。

- [ ] **Step 3: 运行 order-rpc 相关测试**

Run: `go test ./services/order-rpc/tests/integration -run 'TestPayOrder|TestCancelOrder' -count=1`
Expected: 相关测试通过。

### Task 4: 整体验证

**Files:**
- No code changes

- [ ] **Step 1: 运行聚合验证**

Run: `go test ./services/program-rpc/tests/integration ./services/order-rpc/tests/integration -run 'TestConfirmSeatFreeze|TestReleaseSeatFreeze|TestPayOrder|TestCancelOrder' -count=1`
Expected: PASS

- [ ] **Step 2: 记录结果**

在最终说明里明确：
- 修复了哪些异常态
- 保留了哪些与 Java 不同但仍合理的设计
- 是否已经补上 `orderNumber` 分布式锁
