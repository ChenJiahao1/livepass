# 秒杀 Runtime Key 收口与强制预热 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 去掉 `generation` 业务语义，收口秒杀 Redis runtime key 作用域，补齐 `order-rpc` 的 active guard 预热，并把 consumer 的 attempt prepare 热路径收敛到按 `showTime` 直接命中的单次 Lua。

**Architecture:** 本次改动分三层推进。第一层先把 `order-rpc` 的协议、token、event、attempt 模型从 `generation` 解绑，但暂时保持 Redis 物理 key 字面量兼容，避免在途 attempt 因 key rename 丢失。第二层在 `order-rpc` 新增 `PrimeRushRuntime(showTimeId)`，由现有 `jobs/rush-inventory-preheat` worker 统一触发，完成瞬时态清理、active guard 重建和 quota 重建。第三层新增 consumer 专用 `PrepareAttemptForConsume(ctx, showTimeId, orderNumber, now)` Lua，把状态判断、claim 和字段返回合并成一次脚本调用。

**Tech Stack:** Go, go-zero, gRPC, Redis, Lua, MySQL, Asynq

---

### Task 1: 切换 `order-rpc` 预热契约到 `PrimeRushRuntime`

**Files:**
- Modify: `services/order-rpc/order.proto`
- Modify: `services/order-rpc/pb/order.pb.go`
- Modify: `services/order-rpc/pb/order_grpc.pb.go`
- Modify: `services/order-rpc/orderrpc/order_rpc.go`
- Modify: `services/order-rpc/internal/server/order_rpc_server.go`
- Modify: `jobs/rush-inventory-preheat/internal/svc/worker_service_context.go`
- Modify: `jobs/rush-inventory-preheat/internal/worker/rush_inventory_preheat_task_logic.go`
- Modify: `jobs/rush-inventory-preheat/internal/worker/rush_inventory_preheat_task_logic_test.go`
- Modify: `jobs/rush-inventory-preheat/tests/integration/task_serve_mux_test.go`
- Modify: `services/order-rpc/cmd/prime_admission_quota_tmp/main.go`
- Optional Modify: `agents/scripts/generate_proto_stubs.sh`
- Optional Modify: `agents/app/rpc/generated/order_pb2.py`
- Optional Modify: `agents/app/rpc/generated/order_pb2_grpc.py`
- Test: `services/order-rpc/tests/integration/prime_rush_runtime_logic_test.go`

- [ ] **Step 1: 先把 worker 与 RPC 入口的失败测试补上**

在 `services/order-rpc/tests/integration/prime_rush_runtime_logic_test.go` 新建最小失败用例，只校验 `PrimeRushRuntime(showTimeId)` 入口存在并返回 `success=true`。同时把 `jobs/rush-inventory-preheat` 的 fake client 从 `PrimeAdmissionQuota` 改成 `PrimeRushRuntime`，让现有 worker 测试先因为接口缺失而失败。

- [ ] **Step 2: 跑定向测试，确认失败点是协议仍旧使用 `PrimeAdmissionQuota`**

Run:

```bash
go test ./services/order-rpc/tests/integration -run 'TestPrimeRushRuntime.*' -count=1
go test ./jobs/rush-inventory-preheat/... -run 'Test.*RushInventoryPreheat.*' -count=1
```

Expected: 编译失败或测试失败，报未定义 `PrimeRushRuntime` / worker 仍调用旧 RPC。

- [ ] **Step 3: 修改 `order.proto`，把预热 RPC 从 `PrimeAdmissionQuota` 改成 `PrimeRushRuntime`**

协议层只保留一套明确语义：

```proto
message PrimeRushRuntimeReq {
  int64 showTimeId = 1;
}

service OrderRpc {
  rpc PrimeRushRuntime(PrimeRushRuntimeReq) returns (BoolResp);
}
```

`services/order-rpc/cmd/prime_admission_quota_tmp/main.go` 同步改成调试用途的 `PrimeRushRuntime` 入口，避免命令名和实际职责继续错位。

- [ ] **Step 4: 使用 `goctl rpc protoc ... --style go_zero` 重生成 `order-rpc` 代码**

Run:

```bash
cd services/order-rpc && goctl rpc protoc order.proto --go_out=. --go-grpc_out=. --zrpc_out=. --style go_zero
```

Expected: `pb/`、`orderrpc/`、`internal/server/` 的生成代码全部更新为 `PrimeRushRuntime`。

- [ ] **Step 5: 更新 worker 适配层并回归测试**

把 `jobs/rush-inventory-preheat` 的接口、adapter、worker logic、测试 double 全部切到 `PrimeRushRuntime`。如果仓库要求提交 Python gRPC stub，再运行：

```bash
bash agents/scripts/generate_proto_stubs.sh
```

最后跑：

```bash
go test ./jobs/rush-inventory-preheat/... -run 'Test.*RushInventoryPreheat.*' -count=1
```

Expected: worker 包相关测试通过，所有旧 `PrimeAdmissionQuota` 调用点被清干净。

### Task 2: 去掉 `generation` 业务字段，但先保持 Redis 物理 key 兼容

**Files:**
- Modify: `services/order-rpc/internal/rush/purchase_token.go`
- Modify: `services/order-rpc/internal/rush/purchase_token_test.go`
- Modify: `services/order-rpc/internal/rush/attempt_record.go`
- Modify: `services/order-rpc/internal/rush/attempt_keys.go`
- Modify: `services/order-rpc/internal/rush/admit_attempt.lua`
- Modify: `services/order-rpc/internal/rush/attempt_store.go`
- Modify: `services/order-rpc/internal/event/order_create_event.go`
- Modify: `services/order-rpc/internal/logic/create_purchase_token_logic.go`
- Modify: `services/order-rpc/internal/logic/create_order_logic.go`
- Modify: `services/order-rpc/internal/logic/order_create_event_builder.go`
- Modify: `services/order-rpc/internal/logic/create_order_consumer_logic.go`
- Modify: `services/order-rpc/tests/integration/purchase_token_logic_test.go`
- Modify: `services/order-rpc/tests/integration/create_order_rush_logic_test.go`
- Modify: `services/order-rpc/tests/integration/create_order_consumer_rush_logic_test.go`

- [ ] **Step 1: 先写失败测试，锁定“新链路不再输出 generation、旧 payload 仍可解码”**

新增三类断言：

```go
if claims.Generation != "" { t.Fatalf(...) }
if event.Generation != "" { t.Fatalf(...) }
// 带 generation 的旧 token / 旧 event 仍可 Verify / Unmarshal
```

对应测试放在 `purchase_token_test.go`、`purchase_token_logic_test.go`、`create_order_rush_logic_test.go`。

- [ ] **Step 2: 跑定向测试，确认旧断言仍要求 generation 存在**

Run:

```bash
go test ./services/order-rpc/internal/rush -run 'Test(PurchaseToken.*|BuildTokenFingerprint.*)' -count=1
go test ./services/order-rpc/tests/integration -run 'Test(CreateOrderRush.*|PurchaseToken.*)' -count=1
```

Expected: 失败信息仍指向 `Generation` 字段和 `BuildRushGeneration` 相关断言。

- [ ] **Step 3: 清理 purchase token、event、attempt 的 generation 业务语义**

按下面的收口执行：

```go
type PurchaseTokenClaims struct {
    OrderNumber      int64   `json:"orderNumber"`
    UserID           int64   `json:"userId"`
    ShowTimeID       int64   `json:"showTimeId"`
    TicketCategoryID int64   `json:"ticketCategoryId"`
    TicketUserIDs    []int64 `json:"ticketUserIds"`
    TicketCount      int64   `json:"ticketCount"`
    SaleWindowEndAt  int64   `json:"saleWindowEndAt"`
    ShowEndAt        int64   `json:"showEndAt"`
    TokenFingerprint string  `json:"tokenFingerprint"`
}
```

`json.Unmarshal` 对旧 payload 中残留的 `generation` 会自动忽略，所以兼容性由结构体天然承接，不需要显式兜底字段。

- [ ] **Step 4: 调整 fingerprint 与 scope helper，只按 `showTime` 建模**

`BuildTokenFingerprint` 去掉可变参 `generation`，只保留：

```go
func BuildTokenFingerprint(orderNumber, userID, showTimeID, ticketCategoryID int64, ticketUserIDs []int64, distributionMode, takeTicketMode string) string
```

`attempt_keys.go` 去掉所有 `generation string` 入参，但本批次不要改 Redis 物理 key literal；统一封装成 `rushScopeTag(showTimeID int64)`，内部继续生成当前兼容的 slot tag，避免滚动发布期间新旧进程读写两套 key。

- [ ] **Step 5: 更新 Lua 与调用链，然后回归测试**

`admit_attempt.lua` 不再写入 `generation` hash field，`CreatePurchaseToken` / `CreateOrder` / `buildAttemptCreateEvent` / `buildConsumerOrderEvent` 全部不再传递 `generation`。最后跑：

```bash
go test ./services/order-rpc/internal/rush -run 'Test(PurchaseToken.*|BuildTokenFingerprint.*)' -count=1
go test ./services/order-rpc/tests/integration -run 'Test(CreateOrderRush.*|PurchaseToken.*|CreateOrderConsumer.*)' -count=1
```

Expected: 新 token、新 event、新 attempt 不再带 `generation`，旧 payload 仍可通过解码。

### Task 3: 为 `order-rpc` 补齐按 `showTime` 加载 active guard 的 repository 能力

**Files:**
- Modify: `services/order-rpc/internal/model/d_order_user_guard_model.go`
- Modify: `services/order-rpc/internal/model/d_order_viewer_guard_model.go`
- Modify: `services/order-rpc/repository/order_repository.go`
- Modify: `services/order-rpc/repository/order_repository_sharded.go`
- Modify: `services/order-rpc/repository/order_repository_test.go`

- [ ] **Step 1: 先写 repository 失败测试，覆盖跨 shard 按 showTime 扫 active guard**

在 `order_repository_test.go` 新增用例，分别往两个 shard 的 `d_order_user_guard` / `d_order_viewer_guard` 插入同一 `show_time_id` 的数据，断言 repository 能把两个 shard 的 active guard 都扫描出来。

- [ ] **Step 2: 跑定向测试，确认当前 repository 缺少 showTime 读接口**

Run:

```bash
go test ./services/order-rpc/repository -run 'TestWalkActive.*Guard.*ShowTime' -count=1
```

Expected: 编译失败或测试失败，提示 `OrderRepository` 没有对应方法。

- [ ] **Step 3: 在 model 层加分页查询方法，避免预热时一次性读爆内存**

建议直接在两个 model 接口中增加：

```go
FindActiveByShowTimeAfterID(ctx context.Context, showTimeID, afterID, limit int64) ([]*DOrderUserGuard, error)
FindActiveByShowTimeAfterID(ctx context.Context, showTimeID, afterID, limit int64) ([]*DOrderViewerGuard, error)
```

SQL 统一按 `show_time_id = ? and status = 1 and id > ? order by id asc limit ?`。

- [ ] **Step 4: 在 repository 层实现 `WalkActive...ByShowTime`**

接口建议定义成 callback 风格，避免把全量 guard 堆进内存：

```go
WalkActiveUserGuardsByShowTime(ctx context.Context, showTimeID, batchSize int64, fn func([]*model.DOrderUserGuard) error) error
WalkActiveViewerGuardsByShowTime(ctx context.Context, showTimeID, batchSize int64, fn func([]*model.DOrderViewerGuard) error) error
```

实现时对 `deps.ShardConns` 做逐 shard fan-out，单 shard 内按 `id` 游标分页。

- [ ] **Step 5: 跑 repository 测试确认通过**

Run:

```bash
go test ./services/order-rpc/repository -run 'TestWalkActive.*Guard.*ShowTime' -count=1
```

Expected: 单场次 guard 可以跨 shard 扫描，且分页接口不会漏数或重复。

### Task 4: 给 `AttemptStore` 增加 preheat 需要的 showTime 级操作

**Files:**
- Modify: `services/order-rpc/internal/rush/attempt_keys.go`
- Modify: `services/order-rpc/internal/rush/attempt_store.go`
- Modify: `services/order-rpc/tests/integration/rush_attempt_store_test.go`

- [ ] **Step 1: 先写失败测试，锁定 showTime 级清理与 active projection 重建行为**

新增以下测试场景：

```go
Prime helpers clear only user_inflight/viewer_inflight/fingerprint under one showTime
Prime helpers replace user_active/viewer_active under one showTime
Quota key remains showTime + ticketCategory scoped
```

测试统一放在 `rush_attempt_store_test.go`。

- [ ] **Step 2: 跑定向测试，确认 `AttemptStore` 还没有 showTime 级 helper**

Run:

```bash
go test ./services/order-rpc/tests/integration -run 'Test(PrimeRushRuntime|RushAttemptStorePrime).*' -count=1
```

Expected: 编译失败或断言失败，提示缺少清理/重建 helper。

- [ ] **Step 3: 在 `AttemptStore` 增加 showTime 级 helper**

建议至少提供这几类能力：

```go
ClearUserInflightByShowTime(ctx context.Context, showTimeID int64) error
ClearViewerInflightByShowTime(ctx context.Context, showTimeID int64) error
ClearFingerprintByShowTime(ctx context.Context, showTimeID int64) error
ReplaceUserActiveByShowTime(ctx context.Context, showTimeID int64, rows map[int64]int64, ttlSeconds int) error
ReplaceViewerActiveByShowTime(ctx context.Context, showTimeID int64, rows map[int64]int64, ttlSeconds int) error
```

`rows` 的 key 是 `userId/viewerId`，value 是 `orderNumber`。清理允许使用 showTime 定位后的 `SCAN`，因为这是开售前离线预热路径，不是 consumer 热路径。

- [ ] **Step 4: 明确物理 key 兼容策略**

在 helper 内全部只接收 `showTimeID`，不再接收 `generation`。但 `rushScopeTag(showTimeID)` 先维持兼容字面量，避免本批次把在途 Redis key 一起改名。

- [ ] **Step 5: 跑 attempt store 集成测试**

Run:

```bash
go test ./services/order-rpc/tests/integration -run 'Test(PrimeRushRuntime|RushAttemptStorePrime|AdmitKeepsRejectAndReuseSemantics|FailBeforeProcessingTransitionsAcceptedToFailedOnce)$' -count=1
```

Expected: showTime 级 helper 行为稳定，且 admission 既有语义不回退。

### Task 5: 实现 `PrimeRushRuntime`，复用现有 `jobs/rush-inventory-preheat`

**Files:**
- Create: `services/order-rpc/internal/logic/prime_rush_runtime_logic.go`
- Modify: `services/order-rpc/internal/server/order_rpc_server.go`
- Modify: `services/order-rpc/internal/logic/prime_admission_quota_logic.go`
- Modify: `services/order-rpc/tests/integration/prime_rush_runtime_logic_test.go`
- Modify: `jobs/rush-inventory-preheat/internal/worker/rush_inventory_preheat_task_logic.go`
- Modify: `jobs/rush-inventory-preheat/internal/worker/rush_inventory_preheat_task_logic_test.go`
- Modify: `jobs/rush-inventory-preheat/tests/integration/task_serve_mux_test.go`

- [ ] **Step 1: 先写失败测试，覆盖“清理瞬时态 + 重建 active guard + 重建 quota”**

测试至少覆盖：

```go
PrimeRushRuntime clears inflight and fingerprint
PrimeRushRuntime rebuilds user_active and viewer_active from DB guards
PrimeRushRuntime rewrites quota from preorder admission quota
Worker marks inventory preheated only after PrimeRushRuntime + PrimeSeatLedger both succeed
```

- [ ] **Step 2: 跑定向测试，确认当前逻辑只会预热 quota**

Run:

```bash
go test ./services/order-rpc/tests/integration -run 'TestPrimeRushRuntime.*' -count=1
go test ./jobs/rush-inventory-preheat/... -run 'Test.*RushInventoryPreheat.*' -count=1
```

Expected: 失败信息仍表明只重建了 quota，guard 和瞬时态没有处理。

- [ ] **Step 3: 实现 `PrimeRushRuntime(ctx, svcCtx, showTimeID)` 内部逻辑**

推荐顺序固定为：

```go
1. load preorder and resolve showTimeID
2. clear user_inflight / viewer_inflight / fingerprint
3. compute active TTL from preorder show end time
4. replace user_active from d_order_user_guard
5. replace viewer_active from d_order_viewer_guard
6. rewrite quota(showTimeID, ticketCategoryID)
```

不要批量清理 `attempt` 和 `seat_occupied`，这两类不属于本次预热目标。

- [ ] **Step 4: 保留一个薄兼容入口，避免过渡期内部调用炸掉**

如果仓库里还有 `PrimeAdmissionQuota(...)` 内部 helper 被调试命令或旧测试使用，可以让它临时包装到 `PrimeRushRuntime(...)`，但正式 RPC、worker、测试命名全部统一成 `PrimeRushRuntime`。

- [ ] **Step 5: 跑 order-rpc 与 worker 测试**

Run:

```bash
go test ./services/order-rpc/tests/integration -run 'TestPrimeRushRuntime.*' -count=1
go test ./jobs/rush-inventory-preheat/... -run 'Test.*RushInventoryPreheat.*' -count=1
```

Expected: `jobs/rush-inventory-preheat` 继续是正式消费端，但 order 侧预热内容已经扩成完整 runtime preheat。

### Task 6: 收敛 consumer 热路径到单次 Lua `PrepareAttemptForConsume`

**Files:**
- Create: `services/order-rpc/internal/rush/prepare_attempt_for_consume.lua`
- Modify: `services/order-rpc/internal/rush/attempt_store.go`
- Modify: `services/order-rpc/internal/logic/create_order_consumer_logic.go`
- Modify: `services/order-rpc/tests/integration/rush_attempt_store_test.go`
- Modify: `services/order-rpc/tests/integration/create_order_consumer_rush_logic_test.go`

- [ ] **Step 1: 先写失败测试，锁定 consumer prepare 的最终行为**

新增两组断言：

```go
PrepareAttemptForConsume returns shouldProcess=false for SUCCESS/FAILED/PROCESSING
PrepareAttemptForConsume atomically flips ACCEPTED -> PROCESSING and returns full attempt fields
```

consumer 集成测试再补一条，确保它从 event 的 `showTimeId` 直接进入 prepare，不再走 `Get -> ClaimProcessing -> Get`。

- [ ] **Step 2: 跑定向测试，确认现状仍是三次 Redis 交互**

Run:

```bash
go test ./services/order-rpc/tests/integration -run 'Test(PrepareAttemptForConsume|CreateOrderConsumerRush).*' -count=1
```

Expected: 编译失败或测试失败，提示 `AttemptStore` 没有 `PrepareAttemptForConsume`，consumer 仍依赖旧流程。

- [ ] **Step 3: 在 `AttemptStore` 新增 consumer 专用接口**

接口建议直接定成：

```go
func (s *AttemptStore) PrepareAttemptForConsume(ctx context.Context, showTimeID, orderNumber int64, now time.Time) (*AttemptRecord, int64, bool, error)
```

Lua 返回值至少带：

```text
state
processing_epoch
order_number
user_id
program_id
show_time_id
ticket_category_id
viewer_ids
ticket_count
token_fingerprint
sale_window_end_at
show_end_at
```

这样 consumer 后续 `FinalizeSuccess` / `FinalizeFailure` 不需要再补第二次 `Get`。

- [ ] **Step 4: 改 consumer，只按 `showTimeID + orderNumber` 直达**

`CreateOrderConsumerLogic.prepareAttemptForConsume` 改成从 event 里取 `showTimeId`，缺失时回退 `programId`，然后直接调 `AttemptStore.PrepareAttemptForConsume(...)`。如果 event 没有嵌入快照且 attempt 已不存在，保留现有 `ErrOrderNotFound` 行为。

- [ ] **Step 5: 跑 consumer 与 attempt store 回归测试**

Run:

```bash
go test ./services/order-rpc/tests/integration -run 'Test(PrepareAttemptForConsume|CreateOrderConsumerRush|PollOrderProgress|CloseExpiredOrder).*' -count=1
```

Expected: consumer 热路径不再依赖 `resolveAttemptRecordKey` 的 `SCAN`，且终态、租约、补偿链路都不回退。

### Task 7: 做一次统一回归，确认本期范围没有越界到“失败释放统一”

**Files:**
- Test: `services/order-rpc/internal/rush/purchase_token_test.go`
- Test: `services/order-rpc/repository/order_repository_test.go`
- Test: `services/order-rpc/tests/integration/prime_rush_runtime_logic_test.go`
- Test: `services/order-rpc/tests/integration/rush_attempt_store_test.go`
- Test: `services/order-rpc/tests/integration/create_order_rush_logic_test.go`
- Test: `services/order-rpc/tests/integration/create_order_consumer_rush_logic_test.go`
- Test: `jobs/rush-inventory-preheat/internal/worker/rush_inventory_preheat_task_logic_test.go`
- Test: `jobs/rush-inventory-preheat/tests/integration/task_serve_mux_test.go`

- [ ] **Step 1: 跑协议与 token 相关测试**

Run:

```bash
go test ./services/order-rpc/internal/rush -run 'Test(PurchaseToken.*|BuildTokenFingerprint.*)' -count=1
```

Expected: token/fingerprint/generation 相关断言全部切到 showTime 语义。

- [ ] **Step 2: 跑 repository 与 preheat 相关测试**

Run:

```bash
go test ./services/order-rpc/repository -run 'TestWalkActive.*Guard.*ShowTime' -count=1
go test ./services/order-rpc/tests/integration -run 'TestPrimeRushRuntime.*' -count=1
go test ./jobs/rush-inventory-preheat/... -run 'Test.*RushInventoryPreheat.*' -count=1
```

Expected: guard 读取、runtime preheat、worker 串联全部通过。

- [ ] **Step 3: 跑 attempt store 与 consumer 热路径测试**

Run:

```bash
go test ./services/order-rpc/tests/integration -run 'Test(PrepareAttemptForConsume|RushAttemptStorePrime|AdmitKeepsRejectAndReuseSemantics|CreateOrderRush|CreateOrderConsumerRush).*' -count=1
```

Expected: Redis runtime key 作用域收口后，热路径与 admission 仍稳定。

- [ ] **Step 4: 跑更大一圈的 order-rpc 回归**

Run:

```bash
go test ./services/order-rpc/... -count=1
```

Expected: 通过。如果这里暴露 `FAILED` 资源释放差异，记录为后续 work item，不在本期顺手改 `finalize_failure.lua` / `fail_before_processing.lua` / `release_attempt.lua`。

- [ ] **Step 5: 收口变更说明**

提交前确认三件事：

```text
1. 本批次没有做 Redis 物理 key rename
2. 本批次没有统一 FAILED 释放语义
3. jobs/rush-inventory-preheat 仍是唯一正式预热消费者
```

把这三点写进交付说明，避免后续把“逻辑去 generation”和“物理 rename / 失败语义统一”混成同一件事。
