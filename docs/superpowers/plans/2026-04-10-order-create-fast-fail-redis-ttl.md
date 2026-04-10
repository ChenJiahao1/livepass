# Order Create Fast Fail Redis TTL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `feat/order-create-accept-async` 收口为“`/order/create` 同步 Kafka handoff、attempt 单事实源、consumer 持续续租、`/order/poll` 只读 attempt/DB 事实”的快速失败秒杀链路。

**Architecture:** 保留现有 `order-api -> order-rpc -> Kafka -> create consumer -> program-rpc/user-rpc` 的 go-zero 分层，不新增第二份 Redis 进度 key。所有并发裁决、回补和终态收口统一收敛到 `services/order-rpc/internal/rush` 下的 Lua CAS；`/order/poll` 和兼容性 `GetOrderCache` 只做 attempt 投影与 DB 兜底。

**Tech Stack:** Go, go-zero, gRPC, Redis Lua, Kafka (`segmentio/kafka-go`), MySQL, existing order-rpc integration testkit

---

## Execution Notes

- 使用 `@zero-skills` 保持 `order-api` / `order-rpc` / `program-rpc` 的 go-zero 分层和 `ServiceContext` 注入风格一致。
- 使用 `@test-driven-development` 执行每个任务，先补失败测试，再做最小实现。
- 使用 `@verification-before-completion`，在宣称完成前跑完文末验证命令。
- 本计划保留 `GetOrderCache` 这个兼容面，但它必须停止读写 `order:create:marker:*`，改成返回 attempt 投影视图字符串：`PROCESSING` / `SUCCESS` / `FAILED`。
- 复用现有 `RushOrder.InFlightTTL` 作为 `accepted_ttl` 和 `processing_ttl`，复用 `RushOrder.FinalStateTTL` 作为终态 TTL；`fingerprint_ttl` 在 admission 内按 `max(token-expire-at, saleWindowEndAt, final_ttl)` 计算，不新增第二套配置字段。

## File Map

- Modify: `services/order-rpc/internal/rush/attempt_state.go` - 增加 Kafka handoff 和 TTL 失效相关 `reasonCode`，定义 finalize 结果枚举。
- Modify: `services/order-rpc/internal/rush/attempt_record.go` - 保持 `ACCEPTED/PROCESSING/SUCCESS/FAILED -> PROCESSING/SUCCESS/FAILED` 的单向投影，去掉旧投影语义假设。
- Modify: `services/order-rpc/internal/rush/attempt_store.go` - 删除 `MarkQueued` / `CommitProjection` / `Release` 风格 API，新增 `FailBeforeProcessing` / `RefreshProcessingLease` / `FinalizeSuccess` / `FinalizeFailure` / `FinalizeClosedOrder`。
- Modify: `services/order-rpc/internal/rush/attempt_store_lua.go` - 嵌入新的 Lua 脚本并移除旧投影脚本引用。
- Modify: `services/order-rpc/internal/rush/admit_attempt.lua` - admission 只写 attempt + inflight + fingerprint，accepted 阶段使用短 TTL。
- Modify: `services/order-rpc/internal/rush/claim_processing.lua` - 只允许 `ACCEPTED -> PROCESSING`，返回当前 `processing_epoch`。
- Create: `services/order-rpc/internal/rush/fail_before_processing.lua` - Producer 失败分支 CAS。
- Create: `services/order-rpc/internal/rush/refresh_processing_lease.lua` - Consumer 续租并校验 `processing_epoch`。
- Create: `services/order-rpc/internal/rush/finalize_success.lua` - `PROCESSING(epoch) -> SUCCESS`，写 active/seat 占用并延长终态 TTL。
- Create: `services/order-rpc/internal/rush/finalize_failure.lua` - `PROCESSING(epoch) -> FAILED`，只允许赢家回补 quota/inflight。
- Create: `services/order-rpc/internal/rush/finalize_closed_order.lua` - 已成功下单后，订单关闭时回收 active/seat 占用。
- Delete: `services/order-rpc/internal/rush/mark_attempt_queued.lua`
- Delete: `services/order-rpc/internal/rush/commit_attempt_projection.lua`
- Delete: `services/order-rpc/internal/rush/release_attempt.lua`
- Delete: `services/order-rpc/internal/rush/release_closed_order_projection.lua`
- Modify: `services/order-rpc/internal/mq/producer.go` - `Send` 必须同步返回 Kafka 写结果，不能再只是塞本地缓冲。
- Modify: `services/order-rpc/internal/logic/create_order_logic.go` - `/order/create` 改为同步 handoff，失败时走 `FailBeforeProcessing`。
- Modify: `services/order-rpc/internal/logic/create_order_consumer_logic.go` - Consumer 抢处理权、续租、锁座超时重查、DB 事实重查、成功/失败 finalize。
- Create: `services/order-rpc/internal/logic/create_order_processing_lease.go` - 后台 lease ticker 和 ownership 丢失检测。
- Create: `services/order-rpc/internal/logic/order_progress_projection.go` - 共享 attempt/DB 投影逻辑，给 `PollOrderProgress` 和 `GetOrderCache` 复用。
- Modify: `services/order-rpc/internal/logic/poll_order_progress_logic.go` - attempt first，miss 再查 DB，`miss + DB miss => FAILED`。
- Modify: `services/order-rpc/internal/logic/get_order_cache_logic.go` - 改成调用共享投影 helper。
- Delete: `services/order-rpc/internal/logic/order_cache_marker.go`
- Modify: `services/order-rpc/internal/logic/order_domain_helper.go` - 订单关闭后改调新的关闭收口 helper。
- Create: `services/order-rpc/internal/logic/rush_attempt_release_helper.go` - 仅保留 close/cancel 场景需要的同步收口。
- Delete: `services/order-rpc/internal/logic/rush_attempt_projection_helper.go`
- Delete: `services/order-rpc/internal/logic/order_create_compensation.go`
- Modify: `services/order-rpc/tests/integration/order_test_helpers_test.go` - fake producer/fake RPC 增加阻塞、回调和超时注入能力。
- Modify: `services/order-rpc/tests/integration/rush_attempt_store_test.go`
- Modify: `services/order-rpc/tests/integration/create_order_rush_logic_test.go`
- Modify: `services/order-rpc/tests/integration/create_order_consumer_rush_logic_test.go`
- Modify: `services/order-rpc/tests/integration/poll_order_progress_logic_test.go`
- Modify: `services/order-rpc/tests/integration/order_cache_logic_test.go`
- Modify: `services/order-rpc/tests/integration/close_expired_order_logic_test.go`
- Modify: `services/order-rpc/tests/integration/cancel_order_logic_test.go`
- Modify: `services/order-rpc/tests/integration/order_guard_outbox_integration_test.go`
- Create: `services/order-rpc/internal/logic/create_order_consumer_finalize_test.go`

### Task 1: Redis Attempt State Machine And Lua Primitives

**Files:**
- Modify: `services/order-rpc/internal/rush/attempt_state.go`
- Modify: `services/order-rpc/internal/rush/attempt_record.go`
- Modify: `services/order-rpc/internal/rush/attempt_store.go`
- Modify: `services/order-rpc/internal/rush/attempt_store_lua.go`
- Modify: `services/order-rpc/internal/rush/admit_attempt.lua`
- Modify: `services/order-rpc/internal/rush/claim_processing.lua`
- Create: `services/order-rpc/internal/rush/fail_before_processing.lua`
- Create: `services/order-rpc/internal/rush/refresh_processing_lease.lua`
- Create: `services/order-rpc/internal/rush/finalize_success.lua`
- Create: `services/order-rpc/internal/rush/finalize_failure.lua`
- Create: `services/order-rpc/internal/rush/finalize_closed_order.lua`
- Test: `services/order-rpc/internal/rush/attempt_record_test.go`
- Test: `services/order-rpc/tests/integration/rush_attempt_store_test.go`

- [ ] **Step 1: 先补失败测试，锁定新状态机语义**

```go
func TestMapAttemptRecordToPollMapsAcceptedAndProcessingToProcessing(t *testing.T) {}
func TestAdmitKeepsRejectAndReuseSemantics(t *testing.T) {}
func TestFailBeforeProcessingTransitionsAcceptedToFailedOnce(t *testing.T) {}
func TestRefreshProcessingLeaseRejectsOtherEpoch(t *testing.T) {}
func TestFinalizeFailureDoesNotDoubleCompensate(t *testing.T) {}
func TestFinalizeClosedOrderReleasesActiveProjectionOnce(t *testing.T) {}
```

- [ ] **Step 2: 运行状态机测试，确认当前实现不满足 spec**

Run: `go test ./services/order-rpc/internal/rush ./services/order-rpc/tests/integration -run 'TestMapAttemptRecordToPollMapsAcceptedAndProcessingToProcessing|TestAdmitKeepsRejectAndReuseSemantics|TestFailBeforeProcessingTransitionsAcceptedToFailedOnce|TestRefreshProcessingLeaseRejectsOtherEpoch|TestFinalizeFailureDoesNotDoubleCompensate|TestFinalizeClosedOrderReleasesActiveProjectionOnce' -count=1`

Expected: FAIL，原因应该是缺少新方法、旧脚本返回值不对，或者出现旧 `MarkQueued/CommitProjection/Release` 语义。

- [ ] **Step 3: 在 Go 层替换旧 API，显式建模所有 CAS 结果**

```go
type AttemptTransitionOutcome string

const (
    AttemptTransitioned     AttemptTransitionOutcome = "transitioned"
    AttemptAlreadyFailed    AttemptTransitionOutcome = "already_failed"
    AttemptAlreadySucceeded AttemptTransitionOutcome = "already_succeeded"
    AttemptLostOwnership    AttemptTransitionOutcome = "lost_ownership"
    AttemptStateMissing     AttemptTransitionOutcome = "state_missing"
)

func (s *AttemptStore) FailBeforeProcessing(...) (AttemptTransitionOutcome, error)
func (s *AttemptStore) RefreshProcessingLease(...) (bool, error)
func (s *AttemptStore) FinalizeSuccess(...) error
func (s *AttemptStore) FinalizeFailure(...) (AttemptTransitionOutcome, error)
func (s *AttemptStore) FinalizeClosedOrder(...) (AttemptTransitionOutcome, error)
```

- [ ] **Step 4: 用 Lua 一次做完状态迁移和资源副作用**

```lua
-- fail_before_processing.lua
if state == "ACCEPTED" then
  -- attempt -> FAILED
  -- quota += ticket_count
  -- del user_inflight/viewer_inflight
  -- keep fingerprint ttl
  return "transitioned"
end
if state == "FAILED" then return "already_failed" end
if state == "PROCESSING" or state == "SUCCESS" then return "lost_ownership" end
return "state_missing"
```

实现要求：
- `admit_attempt.lua` 写入 `ACCEPTED` 后，attempt TTL 只用 `InFlightTTL`，不能再把处理中 key 一次性拉到长 TTL。
- `claim_processing.lua` 只允许 `ACCEPTED -> PROCESSING`，并写 `processing_started_at`、`processing_epoch += 1`、processing TTL。
- `finalize_success.lua` 和 `finalize_failure.lua` 都必须在同一个脚本里完成状态迁移和副作用，防止 Go 层分两次写造成超补。
- `finalize_closed_order.lua` 只处理“订单已成功建立、后续被关单”的释放，不回到旧 verify/reconcile 语义。

- [ ] **Step 5: 跑通 rush 层测试，确认 TTL 和回补都符合新口径**

Run: `go test ./services/order-rpc/internal/rush ./services/order-rpc/tests/integration -run 'TestMapAttemptRecordToPollMapsAcceptedAndProcessingToProcessing|TestAdmitKeepsRejectAndReuseSemantics|TestFailBeforeProcessingTransitionsAcceptedToFailedOnce|TestRefreshProcessingLeaseRejectsOtherEpoch|TestFinalizeFailureDoesNotDoubleCompensate|TestFinalizeClosedOrderReleasesActiveProjectionOnce' -count=1`

Expected: PASS

- [ ] **Step 6: 提交状态机基线**

```bash
git add services/order-rpc/internal/rush services/order-rpc/tests/integration/rush_attempt_store_test.go
git commit -m "refactor: replace rush projection state machine with fast-fail cas primitives"
```

### Task 2: Make `/order/create` Synchronous And Fast-Fail

**Files:**
- Modify: `services/order-rpc/internal/mq/producer.go`
- Modify: `services/order-rpc/internal/logic/create_order_logic.go`
- Modify: `services/order-rpc/tests/integration/order_test_helpers_test.go`
- Test: `services/order-rpc/tests/integration/create_order_rush_logic_test.go`

- [ ] **Step 1: 先写失败测试，固定 Producer 赢/输两条分支**

```go
func TestCreateOrderFailsWhenKafkaHandoffFailsAndProducerWins(t *testing.T) {}
func TestCreateOrderReturnsOrderNumberWhenKafkaHandoffFailsButConsumerAlreadyClaimed(t *testing.T) {}
func TestCreateOrderDoesNotDoubleCompensateWhenFailBeforeProcessingRepeats(t *testing.T) {}
```

`fakeOrderCreateProducer` 需要加一个 `sendHook func()` 或 `sendFunc func(context.Context, string, []byte) error`，这样测试才能在 `Send` 返回错误前模拟 “Consumer 已经先 claim 成功”。

- [ ] **Step 2: 运行 create 侧测试，确认当前 goroutine handoff 语义会失败**

Run: `go test ./services/order-rpc/tests/integration -run 'TestCreateOrderFailsWhenKafkaHandoffFailsAndProducerWins|TestCreateOrderReturnsOrderNumberWhenKafkaHandoffFailsButConsumerAlreadyClaimed|TestCreateOrderDoesNotDoubleCompensateWhenFailBeforeProcessingRepeats' -count=1`

Expected: FAIL，原因应该是 `CreateOrder` 仍然直接返回 `orderNumber`，或 `Send` 仍然是异步本地 handoff。

- [ ] **Step 3: 把 Kafka Producer 改成真正同步发送**

```go
func (p *kafkaOrderCreateProducer) Send(ctx context.Context, key string, value []byte) error {
    msg := kafka.Message{Key: []byte(key), Value: append([]byte(nil), value...), Time: time.Now()}
    return p.writer.WriteMessages(ctx, msg)
}
```

实现要求：
- 删除本地 `handoff` channel、后台 goroutine 和 “写本地缓冲成功就返回” 的路径。
- `Send` 只在 Kafka 真正返回成功时才返回 `nil`。
- `Close` 只负责关闭 writer，不再 drain 本地队列。

- [ ] **Step 4: 把 `/order/create` 主链路改成 admit + sync send + fail-before-processing**

```go
admission, err := l.svcCtx.AttemptStore.Admit(...)
if admission.Decision == rush.AdmitDecisionReused {
    return &pb.CreateOrderResp{OrderNumber: admission.OrderNumber}, nil
}
err = l.svcCtx.OrderCreateProducer.Send(sendCtx, event.PartitionKey(), body)
if err == nil {
    return &pb.CreateOrderResp{OrderNumber: admission.OrderNumber}, nil
}
outcome, casErr := l.svcCtx.AttemptStore.FailBeforeProcessing(...)
switch outcome {
case rush.AttemptTransitioned, rush.AttemptAlreadyFailed:
    return nil, mapOrderError(mapKafkaHandoffErr(err))
case rush.AttemptLostOwnership, rush.AttemptAlreadySucceeded:
    return &pb.CreateOrderResp{OrderNumber: admission.OrderNumber}, nil
default:
    return nil, status.Error(codes.Internal, xerr.ErrInternal.Error())
}
```

实现要求：
- 删除 `go l.publishOrderCreateEvent(...)` 和 `MarkQueued`。
- `mapKafkaHandoffErr` 要区分 `KAFKA_HANDOFF_TIMEOUT` 和 `KAFKA_HANDOFF_ERROR`。
- 复用 token 命中仍然直接返回旧 `orderNumber`，不能再次发 Kafka。

- [ ] **Step 5: 删除异步等待假设，收紧测试辅助**

Run: `go test ./services/order-rpc/tests/integration -run 'TestCreateOrderFailsWhenKafkaHandoffFailsAndProducerWins|TestCreateOrderReturnsOrderNumberWhenKafkaHandoffFailsButConsumerAlreadyClaimed|TestCreateOrderDoesNotDoubleCompensateWhenFailBeforeProcessingRepeats|TestCreateOrderRushReturnsPreAllocatedOrderNumberAndDoesNotFreezeSeatsInline|TestCreateOrderRushReturnsExistingOrderNumberForSameTokenFingerprint' -count=1`

Expected: PASS

- [ ] **Step 6: 提交 create 快速失败改造**

```bash
git add services/order-rpc/internal/mq/producer.go services/order-rpc/internal/logic/create_order_logic.go services/order-rpc/tests/integration/order_test_helpers_test.go services/order-rpc/tests/integration/create_order_rush_logic_test.go
git commit -m "feat: make rush create handoff synchronous and fail fast"
```

### Task 3: Rework Consumer Claim, Lease, And Finalize

**Files:**
- Modify: `services/order-rpc/internal/logic/create_order_consumer_logic.go`
- Create: `services/order-rpc/internal/logic/create_order_processing_lease.go`
- Create: `services/order-rpc/internal/logic/create_order_consumer_finalize_test.go`
- Delete: `services/order-rpc/internal/logic/order_create_compensation.go`
- Modify: `services/order-rpc/tests/integration/order_test_helpers_test.go`
- Test: `services/order-rpc/tests/integration/create_order_consumer_rush_logic_test.go`

- [ ] **Step 1: 先补失败测试，锁定 lease 和 finalize 决策矩阵**

```go
func TestCreateOrderConsumerRefreshesLeaseDuringSlowProcessing(t *testing.T) {}
func TestCreateOrderConsumerStopsFinalizeWhenLeaseLost(t *testing.T) {}
func TestCreateOrderConsumerRechecksSeatFreezeByRequestNoAfterTimeout(t *testing.T) {}
func TestCreateOrderConsumerGuardConflictFollowsFinalizeFailureOutcome(t *testing.T) {}
func TestFinalizeFailureRetriesWhenScriptErrorLeavesProcessingOwner(t *testing.T) {}
```

白盒测试 `create_order_consumer_finalize_test.go` 要覆盖：
- `transitioned` 时允许释放冻结座位。
- `already_failed` 时幂等结束，不重复释放。
- `already_succeeded` / `lost_ownership` 时跟随赢家。
- `state_missing` 时返回错误，让消息重试。

- [ ] **Step 2: 运行 consumer 测试，确认当前实现还会处理旧 `PROCESSING` 和旧投影语义**

Run: `go test ./services/order-rpc/internal/logic ./services/order-rpc/tests/integration -run 'TestCreateOrderConsumerRefreshesLeaseDuringSlowProcessing|TestCreateOrderConsumerStopsFinalizeWhenLeaseLost|TestCreateOrderConsumerRechecksSeatFreezeByRequestNoAfterTimeout|TestCreateOrderConsumerGuardConflictFollowsFinalizeFailureOutcome|TestFinalizeFailureRetriesWhenScriptErrorLeavesProcessingOwner' -count=1`

Expected: FAIL，原因应该是没有续租 helper、`prepareAttemptForConsume` 还允许直接消费旧 `PROCESSING`、释放逻辑还是旧 `Release()` 语义。

- [ ] **Step 3: 增加 processing lease helper，明确 ownership 丢失后的停止规则**

```go
type processingLease struct {
    epoch int64
    lost  atomic.Bool
    stop  func()
}

func startProcessingLease(ctx context.Context, store *rush.AttemptStore, orderNumber, epoch int64, interval time.Duration) *processingLease
```

实现要求：
- ticker 间隔取 `InFlightTTL / 3`，最小 100ms，避免 lease 刚好卡边界。
- `RefreshProcessingLease` 返回 false 或报错时，把 `lost=true`，当前 consumer 后续不得再写 `SUCCESS/FAILED`。

- [ ] **Step 4: 重写 consume 主流程，严格按事实收口**

```go
claimed, epoch, err := l.svcCtx.AttemptStore.ClaimProcessing(...)
if !claimed {
    return nil // 已失败、已成功、或被别的 consumer 抢走，直接 ack
}
lease := startProcessingLease(...)
defer lease.stop()

freezeResp, err := l.svcCtx.ProgramRpc.AutoAssignAndFreezeSeats(...)
if timeout(err) {
    freezeResp, err = l.svcCtx.ProgramRpc.AutoAssignAndFreezeSeats(...same requestNo...)
}

if businessFailure {
    outcome, err := l.svcCtx.AttemptStore.FinalizeFailure(...)
    if outcome == rush.AttemptTransitioned {
        releaseOrderCreateFreezeWithOwner(...)
    }
    return errToAckOrRetry(outcome, err)
}

if dbCommitted {
    _ = l.svcCtx.AttemptStore.FinalizeSuccess(...)
    return nil
}
```

实现要求：
- `prepareAttemptForConsume` 只在当前消息成功 `ClaimProcessing` 时继续处理；看到已有 `PROCESSING` 但不是当前消息 claim 出来的 ownership，直接 ack。
- 锁座超时不能盲判失败，必须复用同一个 `requestNo` 再查一次，利用 `program-rpc.AutoAssignAndFreezeSeats` 的 requestNo 幂等语义拿事实。
- DB 提交超时或连接断开时，先 `FindOrderByNumber(orderNumber)`，有单据走 success finalize，明确无单据再走 failure finalize。
- `FinalizeSuccess` 失败时只记录日志，不回滚 DB。

- [ ] **Step 5: 删除旧补偿死代码，保留当前链路唯一真实入口**

Run: `go test ./services/order-rpc/internal/logic ./services/order-rpc/tests/integration -run 'TestCreateOrderConsumerPersistsOrderFromRushEvent|TestCreateOrderConsumerRefreshesLeaseDuringSlowProcessing|TestCreateOrderConsumerStopsFinalizeWhenLeaseLost|TestCreateOrderConsumerRechecksSeatFreezeByRequestNoAfterTimeout|TestCreateOrderConsumerGuardConflictFollowsFinalizeFailureOutcome|TestFinalizeFailureRetriesWhenScriptErrorLeavesProcessingOwner|TestCreateOrderConsumerReleasesAttemptWhenSeatFreezeFails' -count=1`

Expected: PASS

- [ ] **Step 6: 提交 consumer 续租和 finalize 改造**

```bash
git add services/order-rpc/internal/logic/create_order_consumer_logic.go services/order-rpc/internal/logic/create_order_processing_lease.go services/order-rpc/internal/logic/create_order_consumer_finalize_test.go services/order-rpc/tests/integration/create_order_consumer_rush_logic_test.go services/order-rpc/tests/integration/order_test_helpers_test.go
git commit -m "feat: add rush consumer lease and finalize ownership handling"
```

### Task 4: Make Poll And GetOrderCache Read The Same Projection

**Files:**
- Create: `services/order-rpc/internal/logic/order_progress_projection.go`
- Modify: `services/order-rpc/internal/logic/poll_order_progress_logic.go`
- Modify: `services/order-rpc/internal/logic/get_order_cache_logic.go`
- Delete: `services/order-rpc/internal/logic/order_cache_marker.go`
- Test: `services/order-rpc/tests/integration/poll_order_progress_logic_test.go`
- Test: `services/order-rpc/tests/integration/order_cache_logic_test.go`

- [ ] **Step 1: 先写失败测试，固定新的 miss 语义和兼容接口语义**

```go
func TestPollMapsAcceptedAndProcessingToProcessing(t *testing.T) {}
func TestPollStaysProcessingWhileConsumerLeaseIsRefreshing(t *testing.T) {}
func TestPollReturnsSuccessWhenAttemptMissesButDBExists(t *testing.T) {}
func TestPollReturnsFailedWhenAttemptMissesAndDBMissing(t *testing.T) {}
func TestGetOrderCacheReturnsProjectedStatusWithoutMarkerKey(t *testing.T) {}
```

- [ ] **Step 2: 运行 poll/cache 测试，确认当前实现仍然依赖 marker 或旧 DB 补翻译**

Run: `go test ./services/order-rpc/tests/integration -run 'TestPollMapsAcceptedAndProcessingToProcessing|TestPollStaysProcessingWhileConsumerLeaseIsRefreshing|TestPollReturnsSuccessWhenAttemptMissesButDBExists|TestPollReturnsFailedWhenAttemptMissesAndDBMissing|TestGetOrderCacheReturnsProjectedStatusWithoutMarkerKey' -count=1`

Expected: FAIL，原因应该是 `poll` 还会在非终态时把 DB 未支付单翻成成功，`GetOrderCache` 还在读 `order:create:marker:*`。

- [ ] **Step 3: 抽一个共享投影 helper，统一 attempt/DB 判定**

```go
type projectedOrderProgress struct {
    OrderNumber int64
    OrderStatus int64
    Done        bool
    ReasonCode  string
    CacheView   string
}

func loadProjectedOrderProgress(ctx context.Context, svcCtx *svc.ServiceContext, userID, orderNumber int64) (*projectedOrderProgress, error)
```

投影规则固定为：
- `ACCEPTED -> PROCESSING`
- `PROCESSING -> PROCESSING`
- `SUCCESS -> SUCCESS`
- `FAILED -> FAILED`
- attempt miss + DB hit -> `SUCCESS`
- attempt miss + DB miss -> `FAILED`，默认 `reasonCode = STATE_EXPIRED`

- [ ] **Step 4: 用共享 helper 重写 `PollOrderProgress` 和 `GetOrderCache`**

```go
progress, err := loadProjectedOrderProgress(...)
return &pb.PollOrderProgressResp{...}

progress, err := loadProjectedOrderProgress(...)
return &pb.GetOrderCacheResp{Cache: progress.CacheView}
```

实现要求：
- `PollOrderProgress` 必须先校验 attempt 或 DB 里的 `userId` 归属，不匹配继续返回 `order not found`。
- `GetOrderCache` 不再创建、读取或续命任何 marker key。
- 删掉 `order_cache_marker.go`，避免后续有人误用旧路径。

- [ ] **Step 5: 重新运行投影测试，确认只有 attempt/DB 两个事实源**

Run: `go test ./services/order-rpc/tests/integration -run 'TestPollMapsAcceptedAndProcessingToProcessing|TestPollStaysProcessingWhileConsumerLeaseIsRefreshing|TestPollReturnsSuccessWhenAttemptMissesButDBExists|TestPollReturnsFailedWhenAttemptMissesAndDBMissing|TestGetOrderCacheReturnsProjectedStatusWithoutMarkerKey|TestPollOrderProgressReturnsReasonCodeWhenAttemptFailed' -count=1`

Expected: PASS

- [ ] **Step 6: 提交 poll/cache 收口**

```bash
git add services/order-rpc/internal/logic/order_progress_projection.go services/order-rpc/internal/logic/poll_order_progress_logic.go services/order-rpc/internal/logic/get_order_cache_logic.go services/order-rpc/tests/integration/poll_order_progress_logic_test.go services/order-rpc/tests/integration/order_cache_logic_test.go
git commit -m "refactor: derive poll and cache views directly from rush attempt"
```

### Task 5: Align Close/Cancel Release Path With The New Attempt APIs

**Files:**
- Modify: `services/order-rpc/internal/logic/order_domain_helper.go`
- Create: `services/order-rpc/internal/logic/rush_attempt_release_helper.go`
- Delete: `services/order-rpc/internal/logic/rush_attempt_projection_helper.go`
- Test: `services/order-rpc/tests/integration/close_expired_order_logic_test.go`
- Test: `services/order-rpc/tests/integration/cancel_order_logic_test.go`
- Test: `services/order-rpc/tests/integration/order_guard_outbox_integration_test.go`

- [ ] **Step 1: 先补失败测试，锁定关单后只能释放一次的要求**

```go
func TestCloseExpiredOrderFinalizesCommittedAttemptAsClosedReleased(t *testing.T) {}
func TestCancelOrderDoesNotDoubleReleaseClosedAttempt(t *testing.T) {}
func TestGuardConflictFinalizeFailureDoesNotReleaseTwice(t *testing.T) {}
```

- [ ] **Step 2: 运行 close/cancel 测试，确认当前 helper 还是旧 projection 名义**

Run: `go test ./services/order-rpc/tests/integration -run 'TestCloseExpiredOrderFinalizesCommittedAttemptAsClosedReleased|TestCancelOrderDoesNotDoubleReleaseClosedAttempt|TestGuardConflictFinalizeFailureDoesNotReleaseTwice' -count=1`

Expected: FAIL，原因应该是还在走 `ReleaseClosedOrderProjection` 或旧 `Release()` 语义。

- [ ] **Step 3: 只保留“订单关闭后的同步收口” helper，删掉旧 reconcile 命名和死分支**

```go
func syncClosedRushAttempt(ctx context.Context, svcCtx *svc.ServiceContext, orderNumber int64, now time.Time) error {
    record, err := svcCtx.AttemptStore.Get(ctx, orderNumber)
    if errors.Is(err, xerr.ErrOrderNotFound) { return nil }
    outcome, err := svcCtx.AttemptStore.FinalizeClosedOrder(ctx, record, now)
    return mapClosedOutcome(outcome, err)
}
```

实现要求：
- `order_domain_helper.go` 的 cancel/close 成功路径改调新 helper。
- 删除 `shouldReconcileRushAttempt` / `reconcileRushAttemptProjection` / `nextRushAttemptProbeAt` 这类旧 verify/reconcile 残留。
- `FinalizeClosedOrder` 只在 attempt 已成功建单且订单实际被取消时释放 active/seat 占用，不回到旧 `ACCEPTED/PROCESSING` 补偿模型。

- [ ] **Step 4: 跑通 close/cancel 回归，确认不会重复释放 quota/seat**

Run: `go test ./services/order-rpc/tests/integration -run 'TestCloseExpiredOrderFinalizesCommittedAttemptAsClosedReleased|TestCancelOrderDoesNotDoubleReleaseClosedAttempt|TestGuardConflictFinalizeFailureDoesNotReleaseTwice|TestCreateOrderConsumerReleasesAttemptWhenSeatFreezeFails' -count=1`

Expected: PASS

- [ ] **Step 5: 提交关闭链路收口**

```bash
git add services/order-rpc/internal/logic/order_domain_helper.go services/order-rpc/internal/logic/rush_attempt_release_helper.go services/order-rpc/tests/integration/close_expired_order_logic_test.go services/order-rpc/tests/integration/cancel_order_logic_test.go services/order-rpc/tests/integration/order_guard_outbox_integration_test.go
git commit -m "refactor: unify closed-order rush release with fast-fail state machine"
```

### Task 6: Verification Sweep

**Files:**
- Test: `services/order-rpc/internal/rush/attempt_record_test.go`
- Test: `services/order-rpc/internal/logic/create_order_consumer_finalize_test.go`
- Test: `services/order-rpc/tests/integration/create_order_rush_logic_test.go`
- Test: `services/order-rpc/tests/integration/create_order_consumer_rush_logic_test.go`
- Test: `services/order-rpc/tests/integration/poll_order_progress_logic_test.go`
- Test: `services/order-rpc/tests/integration/order_cache_logic_test.go`
- Test: `services/order-rpc/tests/integration/close_expired_order_logic_test.go`
- Test: `services/order-rpc/tests/integration/cancel_order_logic_test.go`
- Test: `services/order-rpc/tests/integration/order_guard_outbox_integration_test.go`

- [ ] **Step 1: 先跑精确回归集，确认这次改动覆盖的主链路稳定**

Run: `go test ./services/order-rpc/internal/rush ./services/order-rpc/internal/logic ./services/order-rpc/tests/integration -run 'TestAdmitKeepsRejectAndReuseSemantics|TestFailBeforeProcessingTransitionsAcceptedToFailedOnce|TestCreateOrderFailsWhenKafkaHandoffFailsAndProducerWins|TestCreateOrderReturnsOrderNumberWhenKafkaHandoffFailsButConsumerAlreadyClaimed|TestCreateOrderConsumerRefreshesLeaseDuringSlowProcessing|TestCreateOrderConsumerStopsFinalizeWhenLeaseLost|TestFinalizeFailureRetriesWhenScriptErrorLeavesProcessingOwner|TestPollStaysProcessingWhileConsumerLeaseIsRefreshing|TestPollReturnsSuccessWhenAttemptMissesButDBExists|TestPollReturnsFailedWhenAttemptMissesAndDBMissing|TestGetOrderCacheReturnsProjectedStatusWithoutMarkerKey|TestCloseExpiredOrderFinalizesCommittedAttemptAsClosedReleased|TestCancelOrderDoesNotDoubleReleaseClosedAttempt' -count=1`

Expected: PASS

- [ ] **Step 2: 再跑 broader order-rpc 集成测试，确认没有把其他订单能力带崩**

Run: `go test ./services/order-rpc/tests/integration -count=1`

Expected: PASS

- [ ] **Step 3: 跑 order-api 集成测试，确认 HTTP 侧 contract 没被破坏**

Run: `go test ./services/order-api/tests/integration -count=1`

Expected: PASS

- [ ] **Step 4: 如果环境齐全，再跑失败路径验收脚本**

Run: `./scripts/acceptance/order_checkout_failures.sh`

Expected: 脚本返回 0，且失败场景表现为“create 快速失败或 poll 最终失败”，不会再出现旧 marker 驱动的悬挂态。

- [ ] **Step 5: 最后提交验证通过的收口结果**

```bash
git status --short
git log --oneline -5
```

Expected: 只剩本计划涉及文件，且最近提交消息对应上面 5 个任务。

## Out Of Scope After This Plan

- 不恢复 verify/reconcile/stale scanner。
- 不引入 processing 接管 worker。
- 不把前端 15 秒等待窗口变成后端失败线。
- 不在本次删除 `GetOrderCache` HTTP/RPC contract；等调用方迁走后，再单独做一次契约删除和生成代码清理。
