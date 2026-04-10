# /order/create 快速失败 + Redis TTL 状态机替换式调整设计

## 文档目的

这份 spec 不是对仓库主线历史方案的抽象讨论，而是**基于当前 worktree**
`feat/order-create-accept-async` 的现状，重新收口下一步要调整成什么样。

当前 worktree 已经完成了几件关键事情：

- attempt 状态机已收敛为 `ACCEPTED / PROCESSING / SUCCESS / FAILED`
- `verify_attempt_due` / `reconcile_rush_attempts` / `commitCutoff` / `userDeadline` / `MaxMessageDelay` 已经从主链路移除
- `/order/create` 当前实现为：Redis admission 成功后立即返回，Kafka 发送在 goroutine 中异步进行
- `/order/poll` 当前实现为：先读 Redis，非终态时回查 DB 未支付订单
- `program-rpc` 已经落地 `d_seat` 冻结态生命周期：冻结 `2`、支付成功转已售 `3`、释放转可售 `1`

本次 spec 要解决的，不是旧 verify/reconcile 模型，而是：

- 当前 worktree 的 `create -> goroutine send Kafka` 仍然是“已受理但消息可能根本没送出去”的模型
- 现在决定切到**快速失败模型**
- 即：**Kafka send timeout / send error 是创建接口的失败线**

## 当前 worktree 基线

当前 worktree 的实现口径可以概括为：

1. `/order/create`
   - 先做 Redis admission
   - 写 attempt = `ACCEPTED`
   - 启 goroutine 发 Kafka
   - 不等 Kafka 结果，直接返回 `orderNumber`

2. `consumer`
   - 抢 `PROCESSING`
   - 锁座、落单、写 guard、写 outbox
   - 成功后 `SUCCESS`
   - 明确业务失败后 `FAILED`

3. `/order/poll`
   - Redis 有记录：按 attempt 映射
   - Redis 非终态：再查 DB 未支付订单
   - 当前 Redis 缺失时还没有明确收口到新的快速失败语义

4. 删除项
   - 旧 verify/reconcile 任务链路已经删掉
   - 当前没有额外的“受理后补投递”机制

这意味着当前 worktree 仍然符合“accept + async”，但**不符合本次要落地的快速失败模型**。

## 本次决策

本次采用**方案 A：快速失败模型**，并明确以下业务语义：

- Kafka send timeout / send error 是 `/order/create` 的失败线
- Producer 可以在 Redis 中把 `ACCEPTED` 收口为 `FAILED`
- Consumer 只处理仍然有效的状态，不接管已经失败或已过期的请求
- 不再新增 verify/reconcile/repair job 去补判最终结果
- Redis key TTL 是状态生命周期的一部分，不再额外引入 stale accepted 扫描器
- `/order/poll` 在 Redis 缺失时**必须查 DB**
  - DB 有未支付订单事实 => `SUCCESS`
  - DB 无订单事实 => `FAILED`

也就是说，本方案不是“永远相信 Redis”，而是：

- Redis 负责热路径状态机与并发裁决
- MySQL 负责最终订单事实兜底

## 目标

- 将当前 worktree 从“已受理但 Kafka 可能没送出去”的模型，调整为“Kafka handoff 明确失败则立即失败”
- 保留 Redis admission + Kafka + consumer + DB 幂等落单的主体结构
- 保留现有 `program-rpc` 冻结生命周期实现
- 保留四态 attempt 状态机
- 明确 `/order/poll` 在 Redis 缺失时的 DB 收口语义
- 不重新引入 verify/reconcile 型后台判案任务

## 非目标

- 不改回同步落单
- 不改支付链路主语义
- 不改取消与超时关单主语义
- 不在本次引入新的 DB outbox relay 或额外消息中间件
- 不在本次做跨服务分布式事务

## 状态机

attempt 仍只保留四态：

- `ACCEPTED`
- `PROCESSING`
- `SUCCESS`
- `FAILED`

### 状态语义

#### `ACCEPTED`

- Redis admission 已成功
- quota / inflight / token 指纹索引已写入
- Kafka handoff 尚未被 consumer 接管
- key 带 `accepted_ttl`

#### `PROCESSING`

- Consumer 已成功抢到处理权
- 当前消息正在锁座、写单、写 guard、写 outbox
- key 带 `processing_ttl`
- 处理中的 Consumer 需要续 TTL
- 处理权通过 `processing_epoch` 标识

#### `SUCCESS`

- DB 订单事实已成立
- Redis 投影已收口成功
- key 带终态 TTL（可短一些，但必须覆盖前端查询窗口）

#### `FAILED`

- Producer handoff 失败已回滚
- 或 Consumer 明确业务失败已回滚
- key 带终态 TTL（可短一些，但必须覆盖前端查询窗口）

## TTL 语义

本方案接受“状态过期即退出系统视野”的设计，但必须明确如下规则。

### `accepted_ttl`

用途：

- 覆盖 Producer send 阶段与 Kafka 正常抖动
- 避免 Producer 崩溃后 attempt 永远悬挂

语义：

- `ACCEPTED` key 过期后，不再保留“处理中”语义
- 后续消息如果才到达，Consumer 发现状态不存在，直接丢弃
- `/order/poll` 如果发现 Redis 不存在，则转查 DB 决定最终结果

### `processing_ttl`

用途：

- 表达当前 Consumer 的处理租期
- 不是单纯“展示超时”，而是处理权 lease

语义：

- Consumer 抢到 `PROCESSING` 后必须周期性续 TTL
- 续 TTL 时必须校验：
  - `state == PROCESSING`
  - `processing_epoch == myEpoch`
- 如果续 TTL 失败，说明自己已经不再是合法持有者，必须停止后续 finalize

### `success_ttl / failed_ttl`

用途：

- 支撑前端短时轮询和可观测性
- 不是 correctness 依赖

语义：

- 终态 TTL 过期后，Redis 可以自然清理
- `/order/poll` 如果 Redis 缺失，则回查 DB
  - DB 有订单 => `SUCCESS`
  - DB 无订单 => `FAILED`

## `/order/create` 新语义

`/order/create` 的成功含义调整为：

- purchase token 验签通过
- Redis admission 成功
- Kafka send 返回成功

也就是说：

- **不再是 admission 成功就立即返回**
- 而是 admission 成功后，还要同步完成 Kafka handoff 判定

### 返回规则

#### 1. admission rejected

直接返回业务错误：

- 用户/观演人 inflight 冲突
- quota 不足

#### 2. admission accepted，Kafka send 成功

返回：

- `orderNumber`

此时 Redis 状态仍然是 `ACCEPTED`，等待 consumer 抢占为 `PROCESSING`。

#### 3. admission accepted，Kafka send timeout / send error

Producer 立即执行：

- `FailBeforeProcessing`

只允许：

- `ACCEPTED -> FAILED`

并原子回滚：

- quota
- `user_inflight`
- `viewer_inflight`
- token 幂等索引

如果回滚成功：

- `/order/create` 直接返回失败
- 不再把这次请求当成“已受理”

如果回滚失败：

- 说明状态已经不再是 `ACCEPTED`
- 一般意味着 consumer 已经先一步抢到了 `PROCESSING`
- 此时接口不能再对外宣告失败
- 应返回 `orderNumber`，视为“已被异步链路接管”

> 这里采用的是“快速失败优先，但尊重状态赢家”的口径，而不是“Producer 超时一律对外报错”。

## `/order/poll` 新语义

`/order/poll(orderNumber)` 必须按以下顺序执行：

### 1. 先查 Redis attempt

#### Redis 命中且为 `SUCCESS`

返回：

- `SUCCESS`

#### Redis 命中且为 `FAILED`

返回：

- `FAILED`
- 透传 `reasonCode`

#### Redis 命中且为 `ACCEPTED / PROCESSING`

继续查 DB：

- DB 有未支付订单事实 => `SUCCESS`
- DB 无未支付订单事实 => `PROCESSING`

### 2. Redis 未命中

这里不能直接返回 `not found`，必须查 DB：

- DB 有未支付订单事实 => `SUCCESS`
- DB 无订单事实 => `FAILED`

推荐 reason code：

- `ATTEMPT_EXPIRED_OR_DROPPED`

这条规则是本次 spec 的关键口径之一：

> Redis 不存在，不等于“查无此单”；  
> Redis 不存在时，必须让 DB 来做最终事实兜底。

## Redis 侧数据模型

本次不引入新的全局请求 ID，继续使用现有：

- `orderNumber` 作为唯一命令号
- `tokenFingerprint` 作为 admission 幂等辅助索引

attempt hash 最少保留以下字段：

- `order_number`
- `user_id`
- `program_id`
- `show_time_id`
- `ticket_category_id`
- `viewer_ids`
- `ticket_count`
- `generation`
- `token_fingerprint`
- `state`
- `reason_code`
- `accepted_at`
- `processing_started_at`
- `finished_at`
- `publish_attempts`
- `processing_epoch`
- `created_at`
- `last_transition_at`

不再保留：

- `commitCutoffAt`
- `userDeadlineAt`
- `VERIFYING`
- `COMMITTED`
- `RELEASED`

## Lua 脚本职责

### 1. `admit_attempt.lua`

职责：

- 校验 quota / inflight / active
- 写 attempt = `ACCEPTED`
- 写 inflight
- 扣 quota
- 写 token index
- 设置 `accepted_ttl`

### 2. `fail_before_processing.lua`

职责：

- 只允许 `ACCEPTED -> FAILED`
- 原子回滚 admission 副作用：
  - quota 回补
  - 删除 inflight
  - 删除 token index
- 设置 `failed_ttl`

返回：

- `1`：成功回滚并进入 `FAILED`
- `0`：当前状态不是 `ACCEPTED`

### 3. `claim_processing.lua`

职责：

- 只允许 `ACCEPTED -> PROCESSING`
- `processing_epoch += 1`
- 写 `processing_started_at`
- 设置 `processing_ttl`

返回：

- `claimed`
- `epoch`

### 4. `renew_processing.lua`

职责：

- 仅当：
  - `state == PROCESSING`
  - `processing_epoch == myEpoch`
- 才允许续 `processing_ttl`

### 5. `commit_success.lua`

职责：

- 仅当：
  - `state == PROCESSING`
  - `processing_epoch == myEpoch`
- 才允许 `PROCESSING -> SUCCESS`
- 删除 inflight
- 建立 active 投影
- 写 seat occupied 投影
- 设置 `success_ttl`

### 6. `fail_after_processing.lua`

职责：

- 仅当：
  - `state == PROCESSING`
  - `processing_epoch == myEpoch`
- 才允许 `PROCESSING -> FAILED`
- 回滚业务副作用：
  - quota 恢复
  - inflight 删除
  - 删除 token index（按当前业务保持可重新参与）
- 设置 `failed_ttl`

## Consumer 处理流程

Consumer 收到 Kafka 消息后，不是直接落库，而是：

### 1. 先 `claim_processing`

如果 claim 失败：

- 状态不存在 => 直接丢弃
- 状态不是 `ACCEPTED` => 直接丢弃

### 2. 抢到处理权后开始续 TTL

Consumer 在处理过程中要启动 heartbeat：

- 周期性调用 `renew_processing`
- 续 TTL 失败则必须停止 finalize

### 3. 执行业务主链路

- 读取 preorder / 观演人等事实
- `program-rpc.AutoAssignAndFreezeSeats`
- `d_seat` 写冻结态
- 组装订单写模型
- DB 事务写：
  - `d_order_xx`
  - `d_order_ticket_user_xx`
  - `d_order_user_guard`
  - `d_order_viewer_guard`
  - `d_order_seat_guard`
  - `d_order_outbox(order.created)`

### 4. DB 成功后 finalize success

- 调 `commit_success`

### 5. 明确业务失败后 finalize failed

例如：

- 锁座失败
- guard 冲突
- quota / seat 明确不足

则：

- 调 `fail_after_processing`

### 6. DB 提交不确定时的处理

对于：

- commit timeout
- 连接中断
- 不确定事务是否已提交

不能直接改 `FAILED`。

应执行：

1. 按 `orderNumber` 查 DB
2. 若订单已存在 => `commit_success`
3. 若订单不存在 => 允许重试；若最终 lease 失效，则由 TTL 驱动退出

因此本方案明确接受：

- Producer -> Kafka 边界走快速失败
- Consumer -> DB 边界仍然靠“唯一约束 + 幂等 + 查事实”兜底

## DB 幂等要求

Consumer 侧必须继续依赖 DB 唯一约束保证幂等：

- `d_order.order_number`
- `d_order_outbox(order_number, event_type)`
- `d_order_user_guard`
- `d_order_viewer_guard`
- `d_order_seat_guard`

重试时如果发现：

- 订单已存在
- outbox 已存在
- guard 已存在

应优先按“成功事实已存在”补收口，而不是简单报错退出。

## reasonCode 规范

建议在现有 reasonCode 基础上补充以下值：

- `PRODUCER_SEND_TIMEOUT`
- `PRODUCER_SEND_ERROR`
- `ATTEMPT_EXPIRED_OR_DROPPED`
- `ALREADY_HAS_ACTIVE_ORDER`
- `SEAT_EXHAUSTED`
- `QUOTA_EXHAUSTED`
- `USER_HOLD_CONFLICT`
- `VIEWER_HOLD_CONFLICT`

对外不新增状态，只新增失败原因。

## 对当前 worktree 的改造点

### 1. `services/order-rpc/internal/logic/create_order_logic.go`

当前：

- admission 后起 goroutine 异步 send

目标：

- admission 后同步 send
- send 失败时调用 `fail_before_processing`
- 根据 CAS 结果决定：
  - 直接失败
  - 或返回 `orderNumber`

### 2. `services/order-rpc/internal/rush/attempt_store.go`

新增方法：

- `FailBeforeProcessing`
- `RenewProcessing`
- `CommitSuccess`（可复用现有 `CommitProjection`，但要补 epoch 校验）
- `FailAfterProcessing`（可复用现有 `Release`，但要补 epoch 校验）

### 3. `services/order-rpc/internal/rush/*.lua`

需要新增或重写：

- `fail_before_processing.lua`
- `renew_processing.lua`
- `claim_processing.lua`（补 TTL/epoch 语义）
- `commit_success.lua`（或升级现有 commit 脚本）
- `fail_after_processing.lua`（或升级现有 release 脚本）

### 4. `services/order-rpc/internal/logic/create_order_consumer_logic.go`

需要补：

- claim 后 heartbeat/renew
- finalize 前 epoch 校验
- Redis 缺失/非合法状态时直接丢弃

### 5. `services/order-rpc/internal/logic/poll_order_progress_logic.go`

需要调整为：

- Redis 缺失时查 DB
- DB 无订单则返回 `FAILED`
- 不返回裸 `order not found`

### 6. 文档与测试

需要同步更新：

- README
- `docs/architecture/order-create-accept-async.md`（若继续保留该文档，应明确它已不再代表目标方案）
- RPC/API 契约测试
- integration tests

## 与当前 accept + async 文档的关系

当前 worktree 内已有的
`docs/architecture/order-create-accept-async.md`
描述的是“admission 成功即返回，不以 Kafka send 结果为失败线”的模型。

本 spec 明确替换该口径：

- 当前架构文档可视为**上一版工作树语义的记录**
- 后续如果按本 spec 实施，应同步重写该架构文档

## 验收标准

本次按本 spec 改造完成后，应满足：

- `/order/create` 不再使用 goroutine fire-and-forget Kafka send
- Kafka send timeout / send error 能在 Redis 中把 `ACCEPTED` 收口为 `FAILED`
- Producer 与 Consumer 通过 Redis CAS 裁决唯一赢家
- `PROCESSING` 使用 TTL + renew + `processing_epoch`
- Consumer finalize 必须带 epoch 校验
- `/order/poll` 在 Redis 缺失时必须查 DB
- Redis 缺失 + DB 无订单时，对外返回 `FAILED`，而不是裸 `not found`
- 不重新引入 verify/reconcile/cutoff/deadline/maxDelay

