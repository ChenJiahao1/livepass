# /order/create 快速失败秒杀链路设计

## 文档目的

这份 spec 只讨论当前 worktree `feat/order-create-accept-async` 的替换式收口，不再兼容旧的
`verify/reconcile/cutoff/deadline/MaxMessageDelay` 模型。

当前 worktree 已经完成的基础改造有：

- attempt 状态机已经收敛为 `ACCEPTED / PROCESSING / SUCCESS / FAILED`
- `program-rpc` 已经把 `d_seat` 生命周期切成：
  - 冻结成功写 `seat_status = 2`
  - 支付成功转 `seat_status = 3`
  - 释放转 `seat_status = 1`
- 旧 `verify_attempt_due` / `reconcile_rush_attempts` 链路已经删除

但当前 worktree 仍有两个关键偏差：

- `/order/create` 还是 admission 成功后 goroutine 异步发 Kafka，接口本身不等 handoff 结果
- `/order/poll` 还是直接读 attempt，但文档里又想额外加一份 order progress cache，架构上会形成双写冗余

本次 spec 的目标，是把秒杀链路收口成：

- **attempt 是唯一 Redis 进度事实源**
- `order cache` 只保留为逻辑视图概念，不再单独落一个 Redis key

## 当前 worktree 基线

### `/order/create`

- 验证 purchase token
- Redis admission 成功后写 attempt=`ACCEPTED`
- 另起 goroutine 发 Kafka
- 立即返回 `orderNumber`

这仍然是 `accept + async`，不是快速失败。

### `/order/poll`

- 先查 attempt
- attempt 非终态时，DB 有未支付订单则翻译成 `SUCCESS`
- Redis miss 语义还没有按新口径收口

### `order cache`

仓库里现有的 `order cache` 只是一个 1 分钟 marker：

- key: `order:create:marker:{orderNumber}`
- value: `orderNumber`

它不是状态缓存，不能承担最终设计里的职责。

## 本次最终口径

本次采用**快速失败模型**，并明确以下语义：

- `/order/create` 的成功线是：
  - purchase token 校验通过
  - Redis admission 成功
  - Kafka send 返回成功
- Kafka send `timeout / error` 会触发 Producer 的快速失败分支
- Producer 快速失败和 Consumer 抢处理权，都通过同一个 attempt 状态机 Lua 做 CAS
- 只有一个分支能赢：
  - Producer 赢：`ACCEPTED -> FAILED`，回补 quota / inflight
  - Consumer 赢：`ACCEPTED -> PROCESSING`，继续落单
- 所有失败回补都必须以 attempt 状态迁移成功为前提
- 只有第一次把 attempt 推进到 `FAILED` 的分支，才允许执行 quota / inflight / seat 等回补
- 如果 attempt 已经是 `FAILED`，当前分支只能返回幂等 no-op，不再做任何回补，防止超补
- `poll` 面向的是**订单进度视图**
- 这个“订单进度视图”直接由 attempt 映射得到，不再单独存一份 `order progress cache`
- attempt miss 时，再查 DB
- `Redis miss + DB miss => FAILED`
- 为了避免“DB 正在落库时 Redis miss + DB miss”的误判窗口，Consumer 在处理期间必须持续续
  attempt TTL
- 15 秒只是前端等待窗口，不是后端失败线
- purchase token 是一次性尝试凭证，失败后必须重新申请新 token；旧 token 不能开启新的抢票尝试

## 为什么删除独立 order progress cache

上一版 spec 的问题是：

- attempt 表达：
  - `ACCEPTED`
  - `PROCESSING`
  - `SUCCESS`
  - `FAILED`
- order progress cache 表达：
  - `PROCESSING`
  - `SUCCESS`
  - `FAILED`

两者本质上表达的是同一笔请求的同一份进度事实，只差一层简单映射：

- `ACCEPTED -> PROCESSING`
- `PROCESSING -> PROCESSING`
- `SUCCESS -> SUCCESS`
- `FAILED -> FAILED`

如果为了这层映射，额外再存一个 Redis key，会带来：

- create / consumer / finalize 全链路双写
- 双 TTL 维护
- 双写不一致窗口
- create 成功线额外依赖“第二个 key 必须写成功”

这个复杂度不值。

所以本次架构收口为：

- **物理事实源只有 attempt**
- **order cache 只是逻辑视图，不是第二个 Redis 对象**

## 对外语义

### `/order/create`

成功返回 `orderNumber` 的含义是：

- 这笔请求已经成功 handoff 给 Kafka
- Redis 中已经存在 attempt 记录
- 用户可以开始轮询 `orderNumber`

接口不会把内部 `ACCEPTED` 暴露给前端。

### `/order/poll`

前端轮询看到的永远是**订单进度视图**：

- `PROCESSING`
- `SUCCESS`
- `FAILED`

这个视图由 attempt 实时映射：

- `ACCEPTED -> PROCESSING`
- `PROCESSING -> PROCESSING`
- `SUCCESS -> SUCCESS`
- `FAILED -> FAILED`

### 前端交互

- 用户拿一次 purchase token 发起一次 `/order/create`
- 前端最多轮询 15 秒
- 15 秒后退出初始等待页
- 如果最终失败，必须重新申请 token，再发起新的抢票尝试

## 完整秒杀链路

下面按时序讲完整条链路。

### 0. 前置约束

- 用户先调用 purchase token 接口，拿到一次性的 `purchaseToken`
- `purchaseToken` 内部承载：
  - `orderNumber`
  - `userId`
  - `showTimeId`
  - `ticketCategoryId`
  - `ticketUserIds`
  - `ticketCount`
  - `generation`
  - `tokenFingerprint`
- 一个 token 只允许绑定一个 `orderNumber`
- 旧 token 重复提交，不开启新的 attempt，只能命中旧 `orderNumber`
- 如果旧 `orderNumber` 最终失败，用户必须重新申请新 token

### 1. `/order/create` 入口

#### 1.1 token 校验

`/order/create` 收到请求后，先做两件事：

- 校验 `purchaseToken` 是否有效
- 校验 token 内 `userId` 是否与当前请求用户一致

#### 成功

- 进入 Redis admission

#### 失败

- 直接返回参数错误或 token 无效
- 不写 Redis
- 不返回 `orderNumber`

#### 超时

- 这一步没有独立“超时后继续”的语义
- 超时就按接口失败处理
- 不写 Redis

### 2. Redis admission

这是 create 主链路的第一段核心原子操作。这里不是散着写多个 Redis 命令，而是一段 Lua 一次做完。

#### 2.1 这一步会操作哪些 Redis 内容

admission Lua 会读写这些 key：

- attempt hash
  - `damai-go:order:rush:{scope}:attempt:{orderNumber}`
- user active
  - `...:user_active:{userId}`
- user inflight
  - `...:user_inflight:{userId}`
- quota
  - `...:quota:{ticketCategoryId}`
- fingerprint hash
  - `...:fingerprint:{userId}`
- viewer active
  - `...:viewer_active:{viewerId}`
- viewer inflight
  - `...:viewer_inflight:{viewerId}`

#### 2.2 admission 要做什么

Lua 内一次完成这些检查与写入：

1. 查 `fingerprint`
   - 如果旧 token 已经绑定过 `orderNumber`，直接返回复用命中
2. 查 `user_active / viewer_active`
   - 如果用户或观演人已有活跃单，直接拒绝
3. 查 `user_inflight / viewer_inflight`
   - 如果用户或观演人已有进行中请求，直接拒绝
4. 查 `quota`
   - 如果库存不够，直接拒绝
5. 扣减 `quota`
6. 写 attempt=`ACCEPTED`
7. 写 `user_inflight / viewer_inflight`
8. 写 `fingerprint -> orderNumber`
9. 给 attempt / inflight / fingerprint 设置 TTL

#### 2.3 Redis 里写入什么内容

attempt 至少要写这些字段：

- `order_number`
- `user_id`
- `program_id`
- `show_time_id`
- `ticket_category_id`
- `viewer_ids`
- `ticket_count`
- `generation`
- `token_fingerprint`
- `state = ACCEPTED`
- `reason_code = ""`
- `accepted_at`
- `finished_at = 0`
- `processing_epoch = 0`
- `last_transition_at`

#### 2.4 TTL 约束

这里有三种不同语义的 TTL，不能简单视为一个值：

- `inflight_ttl`
  - 保护 admission 并发窗口
- `attempt_ttl`
  - 保护 attempt 生命周期
- `fingerprint_ttl`
  - 保护 token 一次性语义

其中 `fingerprint_ttl` 不能短于 token 自身有效期，至少要覆盖：

- `purchaseToken` 的自然过期时间
- 或 `saleWindowEndAt`

否则 fingerprint 先过期、token 还没过期时，旧 token 仍可能再次开启新 attempt。

#### 成功

Redis admission 成功后，状态是：

- attempt=`ACCEPTED`
- quota 已扣减
- user/viewer inflight 已写入
- fingerprint 已绑定当前 `orderNumber`

然后继续进入 Kafka handoff。

#### 失败

Redis admission 失败时，直接在 create 入口返回，不进入 Kafka。

失败分支包括：

- `quota` 不足
- 用户已有 active order
- 观演人已有 active order
- 用户已有 inflight order
- 观演人已有 inflight order

这里对外返回业务错误，不返回新的 `orderNumber`。

#### 复用命中

如果命中旧 token 的 `fingerprint -> orderNumber`：

- 不开启新的 attempt
- 不再扣新的 quota
- 直接返回旧 `orderNumber`

这个分支不是失败，也不是新受理，而是“命中旧请求”。

#### 超时

如果 Redis admission 本身超时或执行失败：

- create 直接返回失败
- 不对外承诺这次已经受理
- 不进入 Kafka

### 3. Kafka handoff

Redis admission 成功后，create 线程继续同步发 Kafka，不再允许 goroutine fire-and-forget。

#### 3.1 这一步要发什么

Kafka 消息体至少带上：

- `orderNumber`
- `userId`
- `programId`
- `showTimeId`
- `ticketCategoryId`
- `ticketUserIds`
- `ticketCount`
- `generation`
- `occurredAt`

如果后续继续沿用内嵌快照模式，也带：

- `requestNo`
- 用户/节目/票档/观演人快照

#### 成功

Kafka send 返回成功后，`/order/create` 就可以返回成功。

这里不再额外写一份 progress cache，因为：

- attempt 已经存在
- 前端第一次 poll 时会把 `ACCEPTED` 映射成 `PROCESSING`

#### 失败

Kafka 明确返回 error 时：

1. 立即执行 `FailBeforeProcessing` Lua
2. 这个 Lua 只允许：
   - `ACCEPTED -> FAILED`
3. 如果 attempt 已经不是 `ACCEPTED`：
   - 已经是 `FAILED` => 返回幂等 no-op
   - 已经被 Consumer 抢成 `PROCESSING / SUCCESS` => 返回失去竞争
4. 只有 CAS 成功时，才在同一个 Lua 里完成：
   - attempt 改 `FAILED`
   - quota 回补
   - 删除 `user_inflight / viewer_inflight`
   - fingerprint 保留到 TTL 结束

然后分两种情况：

- Producer 赢
  - 说明 attempt 还停留在 `ACCEPTED`
  - create 直接返回失败
- Producer 输
  - 说明 Consumer 已经先一步把 attempt 从 `ACCEPTED` 抢走了
  - create 不能再对外宣告失败
  - 直接返回 `orderNumber`

如果 `FailBeforeProcessing` 返回“已是 `FAILED`”：

- 视为失败幂等命中
- create 仍返回失败
- 但不能再做第二次回补

#### 超时

Kafka send timeout 和 Kafka send error 走同一条处理逻辑：

- 先尝试 `FailBeforeProcessing`
- Producer 赢就失败返回
- Producer 输就返回 `orderNumber`

这里的 timeout 是 Producer 的失败触发器，不是“无条件报错”。

### 4. `/order/create` 成功返回

现在 create 的成功线回到三段式：

- token 校验成功
- Redis admission 成功
- Kafka send 成功

此时接口返回：

- `orderNumber`

create 返回时，Redis 中至少已经有：

- attempt=`ACCEPTED`
- quota/inflight/fingerprint 副作用

这已经足够支撑前端第一次 poll。

### 5. Consumer 收到 Kafka 消息

到这里为止，create 线程已经结束。接下来进入异步消费链路。

Consumer 收到消息后，第一件事不是写 DB，而是先用 Redis 抢处理权。

#### 5.1 这一步会操作哪些 Redis 内容

核心操作对象是：

- attempt hash

#### 5.2 这一步要做什么

Consumer 执行 `ClaimProcessing` Lua，只允许：

- `ACCEPTED -> PROCESSING`

并在一个 Lua 内完成：

- attempt 改 `PROCESSING`
- `processing_epoch += 1`
- 写 `processing_started_at`
- 刷新 attempt `processing_ttl`

#### 成功

Consumer 抢到处理权后：

- 当前消息成为唯一合法处理者
- 获得本次 `processing_epoch`
- 继续执行业务链路

#### 失败

以下情况，Consumer 直接 ack 丢弃消息：

- attempt 已经是 `FAILED`
- attempt 已经是 `SUCCESS`
- attempt key 已不存在

这表示：

- 这条消息已经过时
- 或者请求已经被快速失败回滚
- 或者 TTL 已经过期失效

#### 超时

如果 `ClaimProcessing` 这步 Redis Lua 超时或报错：

- Consumer 不 ack
- 让消息重试
- 不做本地猜测式 finalize

### 6. Consumer 持续续 Redis lease

Consumer 抢到 `PROCESSING` 后，整个后续执行期间都必须续租，而不是抢到一次就不管。

#### 6.1 要续哪些 Redis 内容

每次续租更新：

- attempt 的 `processing_ttl`

并且必须校验：

- attempt 仍然是 `PROCESSING`
- `processing_epoch == myEpoch`

#### 成功

- 继续执行业务逻辑

#### 失败

如果续租失败，表示当前 Consumer 已经不再是合法持有者：

- 停止后续 finalize
- 不再写 `SUCCESS / FAILED`

#### 超时

- 视为 lease 丢失
- 当前 Consumer 停止处理
- 由后续重试、DB 事实和 poll 兜底

### 7. 锁座

Consumer 拿到处理权后，开始执行业务阶段。第一步是锁座。

#### 7.1 这一步会操作什么

这里主要不是 Redis，而是 `program-rpc.AutoAssignAndFreezeSeats`。

座位服务要完成：

- 自动选座
- 写 `d_seat.seat_status = 2`
- 记录：
  - `owner_order_number`
  - `owner_epoch`

请求幂等键使用：

- `requestNo = orderNumber-epoch`

#### 成功

- 拿到冻结座位结果
- 继续进入 DB 落单

#### 失败

如果是明确业务失败，例如：

- `SEAT_EXHAUSTED`
- 票档不可售
- 场次不可售

则走失败收口：

- 执行 `FinalizeFailure`
- 只有当前 Consumer 成功把 attempt 从 `PROCESSING(myEpoch)` 推进到 `FAILED` 时，才执行 quota 回补、删除 inflight、释放冻结座位
- 如果 attempt 已经是 `FAILED`，视为幂等命中，当前 Consumer 不再重复回补或释放

#### 超时

如果锁座 RPC 超时：

- 不能直接盲判失败
- 必须先按 `requestNo` 查事实

处理规则：

- 能确认已经冻结成功 => 继续后续链路
- 能确认冻结失败 => 走失败收口
- 结果仍然不确定 => 返回错误，让消息重试

### 8. DB 落单

锁座成功后，Consumer 才进入 DB 事务。

#### 8.1 这一步会写哪些 DB 内容

事务内至少写：

- `d_order`
- `d_order_ticket_user`
- `d_order_user_guard`
- `d_order_viewer_guard`
- `d_order_seat_guard`
- `d_order_outbox(order.created)`

#### 成功

事务提交成功后：

- 订单事实已经成立
- 继续做 Redis success finalize

#### 失败

如果是明确业务失败，例如 guard 冲突：

- 对外统一映射成 `ALREADY_HAS_ACTIVE_ORDER`
- 执行失败 finalize
- 先看 `FinalizeFailure` 返回的当前状态裁决结果，再决定是否继续本地重试或结束
- 只有拿到 attempt 状态机的失败 CAS 成功结果后，才继续释放冻结座位
- 如果当前状态已经是 `FAILED`，直接按幂等结束
- 如果当前状态已经被别的分支推进到 `SUCCESS` 或其他 `PROCESSING(epoch)`，当前 Consumer 跟随赢家，不再重复回补

#### 超时

DB commit timeout、连接中断、事务结果未知时：

- 不能直接把 timeout 判成 `FAILED`
- 必须先按 `orderNumber` 查 DB 事实

处理规则：

- DB 查到订单
  - 说明事务其实已经成功
  - 后续按 success finalize 收口
- DB 明确没订单，且错误可确定是失败
  - 按失败 finalize 收口
- DB 仍无法确认
  - 返回错误，让消息重试

这里依赖三件事：

- `orderNumber` 唯一约束
- DB 幂等写入
- 重试时按事实查询

### 9. Redis success finalize

DB 成功后，还要把 Redis 热状态收口成成功。

#### 9.1 这一步会操作哪些 Redis 内容

success finalize 至少要改这些内容：

- attempt hash
- user inflight
- viewer inflight
- user active
- viewer active
- seat occupied
- fingerprint

#### 9.2 这一步要做什么

通过单个 Lua 完成：

- attempt=`SUCCESS`
- `reason_code=ORDER_COMMITTED`
- 删除 `user_inflight / viewer_inflight`
- 写 `user_active / viewer_active`
- 写 `seat_occupied`
- 把 attempt / active 投影续到终态 TTL
- fingerprint 保留到 `fingerprint_ttl` 结束

#### 成功

- Redis 热状态与 DB 事实对齐
- poll 直接可见 `SUCCESS`

#### 失败

如果 DB 已成功，但 success finalize 失败：

- 不回滚 DB
- 不把订单改回失败
- 后续 poll 在 Redis miss 时回查 DB，仍然返回 `SUCCESS`

#### 超时

- 与失败同处理
- 仍以 DB 事实为准

### 10. Redis failed finalize

如果链路在锁座、guard、业务校验等阶段明确失败，需要把 Redis 热状态收口成失败。

#### 10.1 这一步会操作哪些 Redis 内容

failed finalize 至少要改这些内容：

- attempt hash
- quota
- user inflight
- viewer inflight
- user active
- viewer active
- seat occupied
- fingerprint

#### 10.2 这一步要做什么

`FinalizeFailure` 不是简单的“执行成功 / 执行失败”二值语义，而是返回当前状态机裁决结果。

单次 Lua 调用内按当前 attempt 状态处理：

- 如果当前是 `PROCESSING(myEpoch)`：
  - 尝试 CAS 为 `FAILED`
  - 只有 CAS 成功时，才在同一个 Lua 内：
    - 写失败 `reasonCode`
    - quota 回补
    - 删除 `user_inflight / viewer_inflight`
    - 删除 `user_active / viewer_active`
    - 删除 `seat_occupied`
    - fingerprint 保留到 `fingerprint_ttl` 结束
  - 返回 `transitioned`
- 如果当前已经是 `FAILED`：
  - 返回 `already_failed`
- 如果当前已经是 `SUCCESS`：
  - 返回 `already_succeeded`
- 如果当前是别的 `PROCESSING(epoch)`：
  - 返回 `lost_ownership`
- 如果 attempt key 不存在：
  - 返回 `state_missing`

调用方根据返回值处理，而不是一律把它当成“Redis failed finalize 失败”：

- `transitioned`
  - 当前调用赢得失败收口
  - 允许继续执行外部冻结座位释放
- `already_failed`
  - 失败收口已经完成
  - 当前重试按幂等结束，不再重复回补
- `already_succeeded` / `lost_ownership`
  - 说明其他分支已经赢了状态机
  - 当前 Consumer 跟随当前状态，不再做任何回补或释放
- `state_missing`
  - 不猜测结果
  - 交给调用方结合 DB 事实与后续重试决定

#### 成功

- Lua 返回 `transitioned` 时，这次请求正式收口为失败
- Lua 返回 `already_failed` 时，说明失败收口已经完成，当前重试只做幂等结束
- 上面两种情况下，poll 会按 attempt 当前状态看到 `FAILED`
- Lua 返回 `already_succeeded` / `lost_ownership` 时，说明当前调用输了竞争，按赢家状态继续
- 这种情况下，poll 跟随 attempt 当前状态，可能是 `SUCCESS`，也可能仍是别的 `PROCESSING`

#### 失败

- 只有“Redis 脚本本身报错 / 当前状态仍无法确认”才算这里的失败
- 此时不要直接做本地猜测
- 先重新读取 attempt 当前状态，再决定是否本地重试或让消息重试：
  - 读到 `PROCESSING(myEpoch)` => 当前 Consumer 仍持有处理权，可以重试 `FinalizeFailure`
  - 读到 `FAILED` => 说明别的重试或上一次调用已经完成失败收口，直接结束
  - 读到 `SUCCESS` / `PROCESSING(otherEpoch)` => 跟随当前赢家，直接结束
  - 仍读不到稳定状态 => Consumer 不 ack，让消息重试

#### 超时

- 与失败同处理
- timeout 后先读当前 attempt 状态，不直接假设“已经回补成功”或“还没回补”

### 11. `/order/poll`

前端拿到 `orderNumber` 后，最多轮询 15 秒。

#### 11.1 先查什么

`poll` 先查 attempt。

这里的“order cache”不再是单独 Redis key，而是**attempt 的逻辑投影视图**。

#### Redis 命中 `ACCEPTED`

返回对外视图：

- `orderStatus = PROCESSING`
- `done = false`

#### Redis 命中 `PROCESSING`

返回对外视图：

- `orderStatus = PROCESSING`
- `done = false`

#### Redis 命中 `SUCCESS`

返回：

- `orderStatus = SUCCESS`
- `done = true`

#### Redis 命中 `FAILED`

返回：

- `orderStatus = FAILED`
- `done = true`
- `reasonCode`

#### Redis miss

如果 attempt miss，再查 DB 订单事实。

- DB 有订单
  - 返回 `SUCCESS`
- DB 无订单
  - 返回 `FAILED`
  - 可选 reason：
    - `PROCESSING_TIMEOUT`
    - `STATE_EXPIRED`

另外，poll 在读 attempt 和读 DB 时，都必须校验 `userId` 归属；归属不匹配仍按
`order not found` 处理。

## 为什么 `Redis miss + DB miss` 可以直接按失败收口

用户关心的核心风险是：

1. create 已经返回了 `orderNumber`
2. 但 poll 时 Redis miss
3. DB 也还没看到订单
4. 会不会把“还在处理中”的请求错判成失败

本方案的回答是：正常处理中的请求，不应该出现这个窗口。

因为链路里有两层保护：

### 1. create 成功返回前，attempt 已经存在

也就是说，前端第一次 poll 开始时，Redis 已经有可见状态：

- attempt=`ACCEPTED`

对外会映射成：

- `PROCESSING`

### 2. Consumer 抢到处理权后，持续续 attempt TTL

续租覆盖整个处理阶段：

- 拉快照
- 锁座
- 写 DB
- finalize

所以正常慢处理不会出现：

- Redis miss
- DB miss

只有这些 fail-stop 场景，才会出现 Redis miss：

- Consumer 崩溃
- Redis 长时间不可用
- lease 丢失后无人接管
- 处理时间超过 TTL 且没有续上

这些场景在本方案里统一按“这次尝试已经失效”处理，因此：

- Redis miss + DB miss => `FAILED`

## Redis 内容汇总

### 1. attempt

key：

- `damai-go:order:rush:{scope}:attempt:{orderNumber}`

type：

- `hash`

状态：

- `ACCEPTED`
- `PROCESSING`
- `SUCCESS`
- `FAILED`

用途：

- 内部状态机
- Producer / Consumer 并发裁决
- 对外进度视图的唯一 Redis 事实源

TTL：

- `accepted_ttl`
- `processing_ttl`
- `final_ttl`

### 2. 资源占用 key

包括：

- `quota:{ticketCategoryId}`
- `user_inflight:{userId}`
- `viewer_inflight:{viewerId}`
- `user_active:{userId}`
- `viewer_active:{viewerId}`
- `seat_occupied:{orderNumber}`
- `fingerprint:{userId}`

职责：

- `quota / inflight` 表达受理期占用
- `active / seat_occupied` 表达成功后的活跃占用
- `fingerprint` 表达 token 幂等与“旧 token 不能开启新尝试”

TTL 约束：

- `fingerprint_ttl >= purchaseToken` 有效期
- 不能让 fingerprint 比 token 更早过期

## reasonCode 口径

对外失败 reason 至少覆盖：

- `KAFKA_HANDOFF_TIMEOUT`
- `KAFKA_HANDOFF_ERROR`
- `SEAT_EXHAUSTED`
- `ALREADY_HAS_ACTIVE_ORDER`
- `PROCESSING_TIMEOUT`
- `STATE_EXPIRED`

`ALREADY_HAS_ACTIVE_ORDER` 只在对外映射层暴露，不要求前端理解内部 guard 细节。

## 对当前 worktree 的替换式改动点

### 1. `/order/create`

- 去掉 goroutine 异步发 Kafka 后立即返回
- 改成同步 send
- send timeout/error 时执行 `FailBeforeProcessing`
- 不再新增独立 progress cache 写入步骤

### 2. `order cache`

- 删除 `order:create:marker:{orderNumber}`
- 不新增新的 `order:create:progress:{orderNumber}`
- 如果继续保留 `GetOrderCache`，它应返回“attempt 投影视图”，而不是读第二份 Redis 状态

### 3. `/order/poll`

- 逻辑上仍然是“查订单进度视图”
- 物理上改成“先查 attempt，做状态映射，miss 再查 DB”

### 4. attempt store / Lua

需要新增或替换这几类原子操作：

- `FailBeforeProcessing`
- `ClaimProcessing`
- `RefreshProcessingLease`
- `FinalizeSuccess`
- `FinalizeFailure`

### 5. Consumer

- 抢到处理权后启动心跳续租
- 成功/失败 finalize 时只维护 attempt 及资源占用 key
- `PROCESSING` 失租后停止 finalize

### 6. 测试

至少补齐：

- Redis admission 成功 / 拒绝 / 复用命中
- Kafka timeout Producer 赢 / 输两条分支
- `FailBeforeProcessing` 重试命中 `FAILED` 时，不会重复 quota 回补
- poll 对 `ACCEPTED / PROCESSING / SUCCESS / FAILED` 的状态映射
- attempt miss + DB hit => `SUCCESS`
- attempt miss + DB miss => `FAILED`
- Consumer 处理期间持续续 TTL，poll 不误判失败
- `FinalizeFailure` 重试或重复消息命中 `FAILED` 时，不会重复回补或释放
- `FinalizeFailure` 脚本报错或超时后，若 attempt 仍是 `PROCESSING(myEpoch)`，会按当前状态本地重试
- `FinalizeFailure` 后续读到 `SUCCESS` / `PROCESSING(otherEpoch)` 时，会跟随赢家，不重复回补
- 旧 token 重复提交不会开启新 attempt

## 非目标

- 不恢复 verify/reconcile/stale scanner
- 不引入 processing 接管 worker
- 不把 15 秒前端轮询窗口变成后端失败线
- 不再引入第二份 Redis 进度状态对象
