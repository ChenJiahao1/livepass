# 抢票下单异步 Kafka 状态机重构设计

**日期：** 2026-04-18

## 1. 背景

当前 `order-rpc` 抢票下单链路已经多次调整，现状存在以下问题：

- 历史状态语义残留较多，`ACCEPTED`、同步 Kafka handoff、consumer 处理权、TTL 续期之间边界不清
- 用户在超时场景下不能稳定地在有限时间内感知结果
- Redis、Kafka、DB 都存在“结果未知”窗口，若处理不当容易产生“订单已成功但库存被回补”的超卖风险
- 现有实现仍以“同步发送 Kafka”视角组织 create 热路径，与当前需求的“异步发送 Kafka”不一致

本次重构目标不是局部打补丁，而是围绕新的业务约束进行根源性整理。

## 2. 目标

### 2.1 业务目标

- 用户不能长时间无法感知是否失败，必须在有限时间内得到最终感知
- 允许少卖
- 不允许超卖
- `create` 热路径不再同步等待 Kafka 成功再返回
- 用户侧对外只暴露三种结果：
  - 排队中
  - 成功
  - 失败

### 2.2 技术目标

- 统一 attempt 状态机，清除历史遗留状态语义
- 将 Kafka 发送从 create 热路径中移出，改为进程内异步投递
- 所有状态推进都围绕 Redis attempt 状态做 CAS 竞争
- TTL 成为“用户可感知失败”的统一兜底机制
- 对 Redis / Kafka / DB 超时统一按“结果未知”处理，而不是直接当失败

## 3. 非目标

- 不引入 DB outbox 持久化投递方案
- 不引入 TTL 到期后的库存自动回补
- 不追求 attempt 到期后继续恢复处理权
- 不兼容旧状态名、旧状态推进语义，除非外部协议必须保留
- 不在本次重构中顺手修复无关历史问题

## 4. 核心业务约束

### 4.1 attempt 缺失的处理规则

不能把“attempt 不存在”直接当作业务失败信号。

`attempt` 不存在可能是：

- create 阶段未创建成功
- TTL 已到期
- key 被误删
- key 被淘汰
- Redis 异常

因此统一规则为：

- TTL 期间：优先看 Redis 中的 attempt 状态
- `pull` miss 或 TTL 到期：查 DB by `orderNumber`
  - DB 有单：成功
  - DB 无单：失败

## 4.2 超时的处理规则

所有超时都按“结果未知”处理，不能直接当业务失败。

包括但不限于：

- Redis Lua 超时：命令可能已经执行成功
- Kafka 发送超时：消息可能已经成功进入 broker
- DB 超时：订单可能已经成功落库

但本次设计中，**producer 发送失败分支不查 DB**，而是严格以 Redis attempt 状态竞争结果作为是否还能失败回补的依据。

## 4.3 Redis 与 DB 的原子性边界

“排座 + 落单在一次 Lua 内”不成立。

正确边界应为：

- Redis Lua 只能保证 Redis 内状态切换、库存冻结、TTL 续期等原子操作
- 座位冻结 / 落单 / 唯一性约束 / 延迟任务写入等 DB 行为仍由 consumer 在 Redis 之后执行
- 因此 SUCCESS 的定义必须是“DB 已成功落单”而不是“Redis 内已经处理成功”

## 4.4 TTL 到期后的原则

TTL 到期后：

- producer、consumer 都不得再恢复或补写 attempt
- 不允许 attempt 过期后再继续推进状态
- 不做自动回补库存

这会带来库存/座位泄漏，但这是业务上可接受的少卖成本。

同时必须补充后续审计能力：

- TTL 过期失败数量
- Redis 冻结未释放数量
- 库存/座位泄漏对账
- 异常 attempt 告警

本次重构先完成主状态机，不把审计作业作为本次阻塞项，但需在设计中预留。

## 5. 状态设计

内部统一定义 4 个状态：

- `PENDING`
- `PROCESSING`
- `SUCCESS`
- `FAILED`

对外统一映射 3 个结果：

- `PENDING` / `PROCESSING` => 排队中
- `SUCCESS` => 成功
- `FAILED` => 失败

### 5.1 状态定义

#### `PENDING`

含义：

- 已拿到 `orderNumber`
- Redis 已冻结库存
- 异步链路尚未被 consumer 抢占处理

覆盖情况：

- create 刚刚完成 Redis Lua，尚未发 Kafka
- producer 正在发 Kafka
- producer 没拿到 Kafka ack，但 consumer 尚未抢到处理权

#### `PROCESSING`

含义：

- consumer 已抢到处理权
- 异步消费链路正在执行

覆盖情况：

- producer 已收到 Kafka ack，consumer 开始处理
- producer 未收到 ack，但 consumer 实际已经消费并抢到处理权

#### `SUCCESS`

含义：

- DB 已成功落单

#### `FAILED`

含义：

- 明确失败
- 且在允许回补的分支中，已由抢到终态的一方完成回补

## 6. 总体方案对比

### 方案 A：继续同步等待 Kafka

优点：

- create 返回前就知道 Kafka 是否成功发送

缺点：

- 热路径更慢
- Kafka 超时的“结果未知”更容易误伤用户感知
- 与“有限时间内给用户结果，但不阻塞热路径”的目标冲突

### 方案 B：进程内异步投递 Kafka + TTL 状态机（推荐）

优点：

- create 热路径只关心 Redis 准入
- Kafka 发送移出热路径，用户体验更稳定
- 所有分支围绕 attempt 状态 CAS 竞争，超卖风险更可控
- 适配“可少卖，不可超卖”的业务目标

缺点：

- 进程宕机会带来 attempt 泄漏
- 需要更严格的 TTL、CAS、pull 兜底设计

### 方案 C：DB outbox 持久化投递

优点：

- 消息可靠性更高

缺点：

- 架构更重
- 与本次“先做明确状态机重构”的目标不匹配
- 复杂度显著上升

### 结论

采用 **方案 B：进程内异步投递 Kafka + TTL 状态机**。

## 7. 完整状态流转

## 7.1 create 阶段：Redis 准入

`attempt` 创建与库存冻结必须在同一 Lua 内完成。

### 分支 A：Redis Lua 明确失败

状态：

- `无记录 -> 无记录`

触发条件：

- 库存不足
- 用户 / 观演人 inflight 冲突
- 其他明确拒绝分支

动作：

- 不创建 attempt
- 不发 Kafka
- 直接向用户返回失败

### 分支 B：Redis Lua 成功

状态：

- `无记录 -> PENDING`

动作：

- 创建 attempt
- 写入 `status=PENDING`
- 设置 TTL，默认 `30s`
- 冻结库存
- 写入用户/观演人 inflight
- 返回 `orderNumber`
- 对外返回“排队中”
- 之后异步发 Kafka

### 分支 C：Redis Lua 超时

状态：

- 结果未知

动作：

- `orderNumber` 必须在 Redis Lua 前生成，保证即使 Redis 调用超时，用户后续也能按该 `orderNumber` 进行 `pull`
- 仍然向用户返回 `orderNumber` + “排队中”
- 不把 Redis 超时直接映射为失败
- 后续由 `pull` 通过 attempt / DB 兜底收敛

说明：

- 因为 Lua 可能已经成功执行，不能把超时直接当库存冻结失败

## 7.2 Kafka 发送阶段

前置：

- 当前只有 create 链路会发送该消息
- attempt 若存在，状态应为 `PENDING`
- Kafka producer 配置：`acks=-1`，`MaxAttempts=3`

### 分支 A：发送成功

状态：

- `PENDING -> PENDING`

动作：

- 不推进状态
- 等待 consumer 通过 CAS 抢占到 `PROCESSING`

说明：

- producer 收到 ack，不等于业务成功
- `PENDING` 必须保留给 consumer 抢占

### 分支 B：发送明确失败 / 超时耗尽

状态：

- `PENDING -> FAILED`，或竞争失败不变

动作：

- 通过 Lua CAS 尝试 `PENDING -> FAILED`
- 只有抢到终态时才回补库存、清理 inflight
- 若 CAS 失败，说明处理权或终态已被其他分支拿走，当前分支直接退出

说明：

- **这里不查 DB**
- producer 失败分支只与 consumer 抢占 attempt 状态，不做结果推断
- 这样可以避免 producer 失败分支与 consumer/DB 成功分支发生错误补偿交叉

## 7.3 Consumer 阶段

前置原则：

- 只允许处理未过期 attempt
- consumer 抢占与首次续期必须在同一 Lua 内完成，避免 TTL 竞态

### 分支 A：抢占处理权成功

状态：

- `PENDING -> PROCESSING`

动作：

- Lua 内原子完成：
  - 校验 attempt key 仍存在
  - 校验 attempt 当前未过期/仍可处理
  - 校验状态必须为 `PENDING`
  - CAS 切换为 `PROCESSING`
  - 写入 `processing_started_at`
  - 立即续期 TTL
- 抢到后开始后续处理
- 处理期间定时续期

说明：

- 这一步与 producer 的 `PENDING -> FAILED` 失败分支形成竞争
- 谁先抢到，谁拥有后续推进权

### 分支 B：DB 落单成功

状态：

- `PROCESSING -> SUCCESS`

动作：

- DB 成功落单
- Lua CAS `PROCESSING -> SUCCESS`
- 停止续期

### 分支 C：明确业务失败，且确认可以失败

状态：

- `PROCESSING -> FAILED`

动作：

- 典型场景：排座失败、唯一约束冲突确认失败、明确资源不足
- Lua CAS `PROCESSING -> FAILED`
- 抢到失败终态的一方回补库存
- 停止续期

### 分支 D：DB 超时 / DB 结果未知

状态：

- `PROCESSING -> PROCESSING`，随后自然过期

动作：

- 停止续期
- 不回补库存
- 不再继续尝试恢复处理
- 等待 attempt 自然过期
- 用户后续 `pull` 查 DB：
  - 有单：成功
  - 无单：失败

说明：

- 这里不能直接转 `FAILED`
- 因为 DB 结果未知，若错误回补会导致超卖

## 7.4 TTL 阶段

### `PENDING` 到期

动作：

- 不回补库存
- producer / consumer 都不得再恢复该 attempt
- 用户 `pull` 时查 DB：
  - DB 有单：成功
  - DB 无单：失败

### `PROCESSING` 到期

动作：

- 不回补库存
- 不允许继续续期或恢复处理
- 用户 `pull` 时查 DB：
  - DB 有单：成功
  - DB 无单：失败

## 8. Pull 规则

统一收敛规则如下：

- `SUCCESS`：成功
- `FAILED`：失败
- `PENDING`：排队中
- `PROCESSING`：排队中
- attempt 不存在：
  - 查 DB by `orderNumber`
  - DB 有单：成功
  - DB 无单：失败

说明：

- `attempt` miss 本身不是业务失败信号
- `pull` 是状态兜底出口，不是“只信 Redis”的读路径

## 9. Redis Lua 原子性要求

## 9.1 `admit_attempt.lua`

职责调整：

- 将 `state` 从现有 `ACCEPTED` 改为 `PENDING`
- 在一次 Lua 内完成：
  - 重入/指纹复用判断
  - 用户/观演人 inflight 冲突判断
  - quota 校验与扣减
  - attempt hash 创建
  - inflight key 写入
  - fingerprint 写入
  - TTL 初始化

约束：

- 只要 Lua 明确返回失败，就视为确定失败
- Lua 超时则视为结果未知，不做立即补偿

## 9.2 `prepare_attempt_for_consume.lua`

需要根源性改写，不能再只做“存在 + 状态切换”。

必须在一次 Lua 内完成：

- 校验 key 存在
- 校验当前状态为 `PENDING`
- 校验 key 未过期且仍允许处理
- CAS `PENDING -> PROCESSING`
- 写入 `processing_started_at`
- 同步续期 TTL
- 返回最新 attempt 快照

返回语义建议：

- `should_process=1`：抢占成功
- `should_process=0`：未抢到，但 attempt 仍存在
- `state_missing/expired`：attempt 不可继续处理

## 9.3 `refresh_processing_lease.lua`

必须强化为：

- 只允许 `PROCESSING` 状态续期
- key 不存在则返回丢失处理权
- 状态不是 `PROCESSING` 则返回丢失处理权
- 不允许 attempt 已失效后再次补活

## 9.4 `fail_before_processing.lua`

职责收敛为：

- 只处理 `PENDING -> FAILED`
- producer 失败分支专用
- 抢到失败终态时：
  - 回补 quota
  - 删除 inflight
  - 写入最终状态 TTL
- 若状态已不是 `PENDING`，直接返回竞争失败结果

## 9.5 `finalize_success.lua`

职责：

- 只处理 `PROCESSING -> SUCCESS`
- 写成功终态
- 删除 inflight
- 写 active projection
- 写终态 TTL

## 9.6 `finalize_failure.lua`

职责：

- 只处理 `PROCESSING -> FAILED`
- 仅在明确业务失败且允许回补时使用
- 抢到终态后回补 quota，并清理 inflight/active projection

## 10. Go 侧重构设计

## 10.1 `CreateOrderLogic`

现状问题：

- `CreateOrder` 仍同步等待 `OrderCreateProducer.Send`
- `handleKafkaHandoffFailure` 仍按同步 handoff 失败视角工作

重构后：

- `CreateOrder` 成功完成 Redis 准入后立即返回 `orderNumber`
- 不再阻塞等待 Kafka 发送完成
- 将 Kafka 发送改为进程内异步任务
- 若 Redis Lua 明确失败，直接失败
- 若 Redis Lua 超时，仍返回排队中

新增约束：

- create 链路不做 Kafka 失败同步回包
- create 成功返回只表示“已进入 attempt 状态机”，不表示消息一定已被消费

## 10.2 异步 Kafka 发送器

新增/调整职责：

- 接收已成功准入的 `orderNumber + event`
- 在后台执行 Kafka `Send`
- 发送失败/超时耗尽时调用 `AttemptStore.FailBeforeProcessing`
- 不查 DB
- 不对用户直接回包

说明：

- 这是状态机内部补偿分支，不是 create 同步链路的一部分

## 10.3 `CreateOrderConsumerLogic`

重构重点：

- 先通过 Lua 抢占 `PENDING -> PROCESSING`
- 抢占成功后马上进入 TTL 续期机制
- 处理过程中如 lease 丢失，立即停止继续推进
- DB 落单成功后进入 `SUCCESS`
- 明确失败进入 `FAILED`
- DB 结果未知则停止续期，等待 TTL 自然过期

关键约束：

- consumer 不得在 attempt 已过期后恢复处理
- consumer 不得在 lease 丢失后继续写终态

## 10.4 `order_progress_projection.go`

重构重点：

- 统一将 `PENDING / PROCESSING` 映射为“排队中”
- attempt miss 时必须查 DB
- 不允许把 attempt miss 直接映射为失败

## 10.5 `attempt_state.go`

需要重构为新状态常量：

- `AttemptStatePending`
- `AttemptStateProcessing`
- `AttemptStateSuccess`
- `AttemptStateFailed`

并同步更新轮询状态映射与失败原因常量的使用语义。

## 11. 文件级改造范围

本次预期主要改动：

- `services/order-rpc/internal/rush/attempt_state.go`
- `services/order-rpc/internal/rush/attempt_record.go`
- `services/order-rpc/internal/rush/attempt_store.go`
- `services/order-rpc/internal/rush/admit_attempt.lua`
- `services/order-rpc/internal/rush/prepare_attempt_for_consume.lua`
- `services/order-rpc/internal/rush/refresh_processing_lease.lua`
- `services/order-rpc/internal/rush/fail_before_processing.lua`
- `services/order-rpc/internal/rush/finalize_success.lua`
- `services/order-rpc/internal/rush/finalize_failure.lua`
- `services/order-rpc/internal/logic/create_order_logic.go`
- `services/order-rpc/internal/logic/create_order_consumer_logic.go`
- `services/order-rpc/internal/logic/create_order_processing_lease.go`
- `services/order-rpc/internal/logic/order_progress_projection.go`
- 相关 integration tests / unit tests

视实现情况可能新增：

- create 链路的异步投递 helper
- producer 失败补偿 helper
- 更明确的 attempt 抢占返回模型

## 12. 测试策略

### 12.1 AttemptStore / Lua 集成测试

至少覆盖：

- `Admit` 成功后状态为 `PENDING`
- `PrepareAttemptForConsume` 只能从 `PENDING` 抢到 `PROCESSING`
- `PrepareAttemptForConsume` 抢占时同步续期
- `FailBeforeProcessing` 只能从 `PENDING` 失败一次
- `FinalizeSuccess` 只能从 `PROCESSING` 成功一次
- `FinalizeFailure` 只能从 `PROCESSING` 失败一次
- attempt 到期后不可再抢占、不可再续期、不可再写终态

### 12.2 CreateOrder 逻辑测试

至少覆盖：

- Redis 明确失败时，create 直接失败
- Redis 成功时，create 立即返回 `orderNumber`
- Kafka 后台发送失败时，不影响 create 已返回结果
- Kafka 后台发送失败后，若 CAS 抢到 `FAILED`，库存被回补
- Kafka 后台发送失败后，若 consumer 已抢到 `PROCESSING`，producer 分支不回补

### 12.3 Consumer 测试

至少覆盖：

- consumer 抢到 `PROCESSING` 后开始续期
- lease 丢失时 consumer 停止推进
- DB 成功后进入 `SUCCESS`
- 明确业务失败后进入 `FAILED`
- DB 超时/未知时不回补，最终由 TTL + pull 查 DB 收敛

### 12.4 Pull 测试

至少覆盖：

- `PENDING` 返回排队中
- `PROCESSING` 返回排队中
- `SUCCESS` 返回成功
- `FAILED` 返回失败
- attempt miss + DB 有单 => 成功
- attempt miss + DB 无单 => 失败

## 13. 风险与后续事项

### 13.1 已接受风险

- attempt 过期后库存/座位可能泄漏
- 进程内异步发送在实例崩溃时可能导致消息丢失，但 TTL 会让用户最终感知失败
- 允许少卖换取不超卖

### 13.2 后续建议

本次重构完成后，建议追加以下非阻塞工作：

- TTL 过期未落单 attempt 审计
- Redis quota / DB 订单 / 座位冻结对账任务
- attempt 状态与失败原因监控面板
- producer 失败、consumer lease 丢失、TTL 到期数量告警

## 14. 实施原则

- 严格以本设计中的状态机为准，不继续兼容旧 `ACCEPTED` 语义
- 严格遵循“未知结果不直接失败、但 producer 失败分支不查 DB”的边界
- 严格遵循“consumer 抢占时先校验是否过期，再原子续期”的要求
- 严格遵循“TTL 到期后不恢复、不回补”的原则
- 实现过程中若发现现有代码与本 spec 冲突，应以本 spec 为准清理历史残留
