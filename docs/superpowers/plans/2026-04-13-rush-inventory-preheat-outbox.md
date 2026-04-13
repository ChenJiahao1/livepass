# Rush Inventory Preheat Outbox Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 rush inventory preheat 任务改造成基于 delay task outbox 的 `dispatcher + worker` 结构，并迁移到 `jobs/rush-inventory-preheat`

**Architecture:** `services/program-rpc` 仅负责根据场次状态写入延迟任务 outbox；新 job 目录负责消息契约、outbox 派发和 asynq 消费。目录布局、配置模型和测试策略对齐 `jobs/order-close`，避免继续维护两套延迟任务形态。

**Tech Stack:** Go, go-zero, asynq, MySQL, zrpc, Go testing

---

### Task 1: 建立重构目标的测试基线

**Files:**
- Create: `jobs/rush-inventory-preheat/taskdef/rush_inventory_preheat_task_test.go`
- Create: `jobs/rush-inventory-preheat/tests/integration/task_serve_mux_test.go`
- Create: `jobs/rush-inventory-preheat/tests/integration/dispatch_run_once_logic_test.go`
- Modify: `services/program-rpc/tests/integration/program_write_logic_test.go`

- [ ] **Step 1: 为 taskdef 写失败测试**
- [ ] **Step 2: 运行 taskdef 测试，确认因缺少实现而失败**
- [ ] **Step 3: 为 worker 路由和 dispatcher 发布行为写失败测试**
- [ ] **Step 4: 运行新增 job 测试，确认失败原因与目标行为一致**
- [ ] **Step 5: 为 program-rpc 写入 outbox 的行为补失败测试**

### Task 2: 落地新 job 目录与消息契约

**Files:**
- Create: `jobs/rush-inventory-preheat/cmd/dispatcher/main.go`
- Create: `jobs/rush-inventory-preheat/cmd/worker/main.go`
- Create: `jobs/rush-inventory-preheat/etc/rush-inventory-preheat-dispatcher.yaml`
- Create: `jobs/rush-inventory-preheat/etc/rush-inventory-preheat-worker.yaml`
- Create: `jobs/rush-inventory-preheat/internal/config/config.go`
- Create: `jobs/rush-inventory-preheat/internal/svc/dispatcher_service_context.go`
- Create: `jobs/rush-inventory-preheat/internal/svc/worker_service_context.go`
- Create: `jobs/rush-inventory-preheat/internal/worker/task_serve_mux.go`
- Create: `jobs/rush-inventory-preheat/internal/worker/rush_inventory_preheat_task_logic.go`
- Create: `jobs/rush-inventory-preheat/internal/dispatch/store.go`
- Create: `jobs/rush-inventory-preheat/internal/dispatch/mysql_store.go`
- Create: `jobs/rush-inventory-preheat/internal/dispatch/run_once_logic.go`
- Create: `jobs/rush-inventory-preheat/taskdef/rush_inventory_preheat_task.go`

- [ ] **Step 1: 先实现 taskdef 最小代码让 taskdef 测试转绿**
- [ ] **Step 2: 实现 worker 所需配置、service context、serve mux 与处理逻辑**
- [ ] **Step 3: 实现 dispatcher 的 store、run once logic 与启动入口**
- [ ] **Step 4: 运行 job 相关测试，确认新增目录的红转绿**

### Task 3: 改造 program-rpc 从直接入队切换到 outbox

**Files:**
- Modify: `services/program-rpc/internal/config/config.go`
- Modify: `services/program-rpc/internal/svc/service_context.go`
- Modify: `services/program-rpc/internal/svc/rush_inventory_preheat_client.go`
- Modify: `services/program-rpc/internal/logic/rush_inventory_preheat_helper.go`
- Modify: `services/program-rpc/etc/program-rpc.yaml`

- [ ] **Step 1: 先让 program-rpc 的新测试失败，确认还在直接 enqueue**
- [ ] **Step 2: 将 client 适配为写 delay task outbox**
- [ ] **Step 3: 调整配置和 service context 的依赖注入**
- [ ] **Step 4: 运行 program-rpc 相关测试，确认写 outbox 行为生效**

### Task 4: 迁移引用并移除旧 worker 目录

**Files:**
- Modify: `scripts/deploy/start_backend.sh`
- Delete: `jobs/rush-inventory-preheat-worker/rush_inventory_preheat_worker.go`
- Delete: `jobs/rush-inventory-preheat-worker/internal/config/config.go`
- Delete: `jobs/rush-inventory-preheat-worker/internal/svc/service_context.go`
- Delete: `jobs/rush-inventory-preheat-worker/internal/worker/task_serve_mux.go`
- Delete: `jobs/rush-inventory-preheat-worker/internal/logic/rush_inventory_preheat_task_logic.go`
- Delete: `jobs/rush-inventory-preheat-worker/internal/logic/rush_inventory_preheat_task_logic_test.go`
- Delete: `jobs/rush-inventory-preheat-worker/tests/integration/task_serve_mux_test.go`
- Delete: `jobs/rush-inventory-preheat-worker/etc/rush-inventory-preheat-worker.yaml`
- Delete: `services/program-rpc/preheatqueue/task.go`
- Delete: `services/program-rpc/preheatqueue/task_test.go`

- [ ] **Step 1: 更新脚本与导入路径到新 job 目录**
- [ ] **Step 2: 删除旧目录和旧消息定义**
- [ ] **Step 3: 运行 `rg` 检查旧路径残留**

### Task 5: 完成验证

**Files:**
- Verify only

- [ ] **Step 1: 运行新增 job 的单测和集成测试**
- [ ] **Step 2: 运行 program-rpc 相关测试**
- [ ] **Step 3: 运行针对改动范围的 `go test` 或包级验证**
- [ ] **Step 4: 复查脚本、配置和引用是否一致**
