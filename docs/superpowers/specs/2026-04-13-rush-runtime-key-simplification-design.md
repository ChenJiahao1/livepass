# 秒杀 Runtime Key 收口与强制预热设计

## 目标

本次设计目标是把秒杀下单链路的 Redis 运行态收口到真实业务维度，并去掉 `generation` 这层业务建模：

- 不再把“几开”作为 Redis key、purchase token、Kafka event、attempt record 的业务输入
- 明确 Redis 只是开售期热读模型，DB 才是业务真相
- 每次开售前必须显式强制预热，不再依赖上一轮 Redis 残留状态
- `quota` 按 `showTime + ticketCategory` 维护
- `program-rpc` 的 seat ledger 按 `showTime + ticketCategory` 维护
- `order-rpc` admission guard 按 `showTime` 维护
- consumer 热路径不再依赖按 `orderNumber` 的 `SCAN` 式寻址

## 本期范围

本期包含：

- 移除 `generation` 的业务语义和业务输入
- 收口 rush runtime key 的作用域
- 增加开售前强制预热要求
- 为 `order-rpc` 补齐 active guard 预热
- 收敛 consumer 的 attempt prepare 热路径

本期不包含：

- 不统一 `FAILED` 后的资源释放与重试语义
- 不调整 `finalize_failure.lua`、`fail_before_processing.lua`、`release_attempt.lua` 的终态释放差异
- 不改变“旧 purchase token 在失败后是否允许原地重试”的行为
- 不重新设计“二开/三开”运营模型

也就是说，本期先解决 key 作用域、预热真相和 consumer 热路径问题；第七点失败重试统一留到后续单独评估。

## 背景

当前实现的核心问题不是单点 bug，而是三类建模混在了一起：

### 一：`generation` 被拿来承载“二开预热”问题

当前 `generation` 的存在，本质上是在绕开“开售前必须强制预热 Redis”这个事实。它不是物理资源维度，也不是 seat ledger 维度，而是一次运营轮次的概念。

但在当前系统里：

- `quota` 的真实维度是 `showTime + ticketCategory`
- seat ledger 的真实维度是 `showTime + ticketCategory`
- 用户是否已持单的 guard 真实维度是 `showTime + user/viewer`

也就是说，`generation` 并不是这些资源的自然主键。

### 二：Redis 里混有真相、投影和瞬时态

当前 rush Redis 同时承载了：

- admission quota
- active projection
- inflight projection
- attempt 状态
- token fingerprint
- order success 后的 seat projection

这些数据的生命周期和真相来源并不一致。如果不明确哪些需要预热、哪些只应该运行时生成，就会出现“开售前到底该准备什么”的反复争论。

### 三：consumer 热路径没有利用已有作用域信息

`CreateOrderConsumer` 消费消息时，event 本身已经带了 `showTimeId`，但当前 `prepareAttemptForConsume` 仍然走：

- `Get(orderNumber)`
- `ClaimProcessing(orderNumber)`
- `Get(orderNumber)`

而 `Get/ClaimProcessing` 内部都要先按 `orderNumber` 去 `SCAN` 找 attempt key。问题不在 Lua 做不到，而在热路径沿用了只按 `orderNumber` 的通用接口。

## 设计原则

### 原则一：作用域跟随真实业务资源，而不是跟随运营轮次

- `quota` 跟随票档库存，所以作用域是 `showTime + ticketCategory`
- seat ledger 跟随座位库存，所以作用域是 `showTime + ticketCategory`
- active / inflight / fingerprint 跟随场次下单资格，所以作用域是 `showTime`

### 原则二：Redis 是热读模型，不是事实源

Redis 的职责是：

- 热路径 admission
- attempt 状态投影
- active / inflight guard 投影
- 幂等索引

DB 的职责是：

- 订单事实
- guard 真相
- 座位事实
- 开售前预热内容的最终来源

### 原则三：开售前必须做覆盖式强制预热

不再接受“因为 Redis 里还有上一轮数据，所以先沿用”的思路。每次开售前必须显式执行预热，把本轮开售需要的热数据从 DB 真相重建出来。

### 原则四：seat 冲突的权威保持单点

`program-rpc` 的 seat ledger 是座位库存热模型，`d_seat` 是最终座位真相。`order-rpc` 不再额外维护一份 admission 读路径要使用的 seat guard 投影，避免出现两套座位真相。

### 原则五：consumer 热路径必须可直接命中 key

任何开售高频链路都不应依赖 `SCAN` 解析作用域。既然消息里有 `showTimeId`，attempt key 就应当可以直接命中。

## 方案对比

### 方案 A：保留 `generation`，继续按开售轮次分隔 Redis 作用域

优点：

- 理论上可以通过切换 generation 隔离新旧轮次数据

缺点：

- 把“预热责任”偷换成“切命名空间”
- 资源维度变复杂
- consumer、quota、seat ledger 都要背上不自然的轮次建模

不推荐。

### 方案 B：移除 `generation` 业务语义，每轮开售前强制预热

优点：

- 作用域直接对应真实业务资源
- 与 `quota`、seat ledger、DB guard 的真相维度一致
- consumer 可以按 `showTime` 直接命中 attempt
- 更利于后续按 `showTime + ticketCategory` 做分片

缺点：

- 需要正式把“强制预热”变成流程要求
- 需要补齐 active guard 预热能力

这是推荐方案。

## Redis 运行态模型

### `order-rpc` Redis 模型

本期建议将 `order-rpc` rush Redis 模型收口为下面几类：

| 类型 | 建议作用域 | 来源 | 是否预热 | 说明 |
| --- | --- | --- | --- | --- |
| `attempt` | `showTime + orderNumber` | runtime | 否 | attempt 状态记录 |
| `quota` | `showTime + ticketCategory` | DB / preorder | 是 | admission 可接单额度 |
| `user_active` | `showTime + userId` | DB guard | 是 | 用户在该场次已有有效订单 |
| `viewer_active` | `showTime + viewerId` | DB guard | 是 | 观演人在该场次已有有效订单 |
| `user_inflight` | `showTime + userId` | runtime | 否 | 用户当前有处理中建单 |
| `viewer_inflight` | `showTime + viewerId` | runtime | 否 | 观演人当前有处理中建单 |
| `fingerprint` | `showTime + userId` | runtime | 否 | 同一次 purchase token 幂等复用 |
| `seat_occupied` | `showTime + orderNumber` | runtime | 否 | 订单成功后的 seat 投影，仅用于订单侧补偿 |

### `program-rpc` Seat Ledger 模型

`program-rpc` 继续按 `showTime + ticketCategory` 维护：

- `stock`
- `available`
- `sold`
- `frozen`

这是正确的，因为 seat ledger 天然是场次票档维度的座位库存模型，而不是“几开”维度的模型。

## Key 作用域与命名规则

本期建议从业务模型中移除 `generation` 参数，但 Redis Cluster 的物理 slot tag 需要单独考虑迁移风险。

推荐区分两层：

### 业务作用域

业务上统一视为：

- `attempt(showTimeId, orderNumber)`
- `quota(showTimeId, ticketCategoryId)`
- `user_active(showTimeId, userId)`
- `viewer_active(showTimeId, viewerId)`

也就是说，不再有任何业务接口接收 `generation`。

### 物理 key 迁移策略

为了避免线上混部期间旧 key 与新 key 并存导致 in-flight attempt 丢失，本期建议：

- 先移除 `generation` 的业务输入和业务依赖
- Redis 物理 key 是否立即改名为纯 `{st:<showTimeId>}`，作为单独迁移决策

推荐优先级：

1. 本期先完成“逻辑去 generation”
2. 仅在确认不存在混部与存量未完成 attempt 风险时，再做 key literal 清理

换句话说，本期的关键是“逻辑上只按 `showTime` 建模”，而不是必须同步完成全量 Redis key rename。

## 预热设计

### 预热目标

每次开售前，需要把本轮真正参与热路径判断的数据准备好。目标不是把所有 Redis 数据重建，而是只重建“准入所需热态”。

### `order-rpc` 预热职责

建议新增统一入口：

- `PrimeRushRuntime(showTimeId)`

职责顺序如下：

1. 清理该 `showTime` 下的瞬时态
   - 删除 `user_inflight`
   - 删除 `viewer_inflight`
   - 删除 `fingerprint`

2. 覆盖式重建 active guard 投影
   - 清理该 `showTime` 下旧的 `user_active`
   - 清理该 `showTime` 下旧的 `viewer_active`
   - 从 DB `d_order_user_guard`、`d_order_viewer_guard` 重新加载并写入 Redis

3. 覆盖式重建 quota
   - 从 DB 真相对应的票档 admission quota 加载
   - 逐票档覆盖写入 `quota(showTimeId, ticketCategoryId)`

4. 不批量清理 attempt
   - `attempt` 记录保留原有 TTL
   - 原因是它承载轮询与排障价值，且不应作为开售前清理对象

5. 不批量清理 `seat_occupied`
   - 它不是 admission 主判断面
   - 它属于订单成功后的订单侧投影，不应与 seat ledger 重建耦合

### `program-rpc` 预热职责

继续使用或增强现有：

- `PrimeSeatLedger(showTimeId)`

要求是：

- 必须强制从 DB 重建，不依赖已有 Redis 状态
- 必须覆盖该 `showTime` 下所有票档的 seat ledger

也就是说，seat ledger 预热的本质是“全量重装本场次座位热模型”，而不是“仅补缺失 key”。

### `guard` 预热边界

本期明确：

- 要预热 `user_active`
- 要预热 `viewer_active`
- 不把 `d_order_seat_guard` 额外投影到 `order-rpc` admission 热路径

原因是：

- `user_active` / `viewer_active` 是 admission Lua 会直接读取的 guard
- `seat_guard` 的业务真相已经由 `program-rpc` seat ledger 和 `d_seat` 承载
- 再复制一套 seat guard 到 `order-rpc` 只会形成两套座位真相

DB 侧唯一约束仍继续保留：

- `d_order_user_guard(show_time_id, user_id)`
- `d_order_viewer_guard(show_time_id, viewer_id)`
- `d_order_seat_guard(show_time_id, seat_id)`

其中 `seat_guard` 继续作为落库阶段最终防线，而不是开售热路径预热内容。

## Repository 与数据访问补充

当前仓库已有写入 guard 的能力，但缺少按 `showTime` 扫描 active guard 的正式读接口。本期需要补齐：

- 按 `showTimeId` 读取 active user guard 列表
- 按 `showTimeId` 读取 active viewer guard 列表

接口应放在 `order-rpc` repository 层，避免把 SQL 散落到 logic。

如果单场次 guard 数量较大，接口应支持分页或游标批量加载，防止单次预热把整场所有 guard 一次性读入内存。

## Consumer 热路径设计

### 现状问题

当前 consumer 的 `prepareAttemptForConsume` 使用：

- `Get(orderNumber)`
- `ClaimProcessing(orderNumber)`
- `Get(orderNumber)`

而 `Get/ClaimProcessing` 内部通过 `resolveAttemptRecordKey` 按 `orderNumber` 执行 `SCAN`。

这导致：

- 一次 prepare consume 包含多次 Redis 往返
- 热路径建立在通用接口之上，而不是建立在消息已知作用域之上

### 目标接口

建议新增：

- `PrepareAttemptForConsume(ctx, showTimeId, orderNumber, now)`

这是一个面向 consumer 的专用接口，不替代所有通用 `Get`。

### 目标行为

目标接口通过单次 Lua 完成：

- 直接按 `showTimeId + orderNumber` 命中 attempt key
- 判断当前 state
- `SUCCESS / FAILED / PROCESSING` 直接返回 `shouldProcess=false`
- `ACCEPTED` 原子切到 `PROCESSING`
- 自增并返回 `processingEpoch`
- 返回 consumer 后续需要的完整 attempt record 必要字段

建议返回字段至少包括：

- `order_number`
- `user_id`
- `program_id`
- `show_time_id`
- `ticket_category_id`
- `viewer_ids`
- `ticket_count`
- `token_fingerprint`
- `sale_window_end_at`
- `show_end_at`
- `processing_epoch`
- `state`

### consumer 消息内容

consumer 已经拥有 `orderNumber` 和 `showTimeId`。因此热路径不应再回退到按 `orderNumber` 全局解析 key 的旧模式。

如果短期需要分步落地，可先做两步：

1. 增加 `GetByScope(showTimeId, orderNumber)` / `ClaimProcessingByScope(showTimeId, orderNumber)`
2. 再收敛到 `PrepareAttemptForConsume` 单 Lua

但设计终态应以单 Lua 为准。

## Purchase Token 与 Event 建模

本期建议：

- `PurchaseTokenClaims` 不再把 `generation` 作为业务字段输出
- `OrderCreateEvent` 不再把 `generation` 作为业务字段传递
- `AttemptRecord` 不再持有 `generation` 业务字段

需要兼顾滚动发布时的兼容性：

- 对旧 token / 旧 event 中残留的 `generation` 字段，解码时允许忽略
- 新代码不再依赖该字段参与 key 计算或业务判断

这样可以做到：

- 新旧 payload 在 JSON 层面可兼容
- 逻辑层彻底脱离 `generation`

## 调度与流程衔接

本期不新建新的预热调度体系，直接复用现有：

- `program-rpc` 写入 `delay_task_outbox`
- `jobs/rush-inventory-preheat` dispatcher 负责把待执行任务发布到 Asynq
- `jobs/rush-inventory-preheat` worker 作为正式预热消费者执行预热

当前现状已经是：

- worker 会校验 `expectedRushSaleOpenTime`
- 然后调用 `OrderRpc.PrimeAdmissionQuota(showTimeId)`
- 再调用 `ProgramRpc.PrimeSeatLedger(showTimeId)`
- 最后将 `inventory_preheat_status` 标记为已完成

因此本设计的调度结论不是“后续再决定谁来触发”，而是：

- 现有 `jobs/rush-inventory-preheat` 就是正式预热入口
- 本期只需要扩充它调用的 `order-rpc` 预热能力，不需要再新增新的 queue、dispatcher 或 worker

### 对现有 worker 的影响

现有 worker 里 `order-rpc` 侧调用的是 `PrimeAdmissionQuota`。而本设计要求 `order-rpc` 侧预热不仅包含 quota，还要包含 active guard 和瞬时态清理。

因此 `order-rpc` 侧需要演进为下面两种方式之一：

#### 方案一：保留 RPC 名称 `PrimeAdmissionQuota`，扩展其语义

扩展后它实际上执行：

- 清理 `user_inflight`
- 清理 `viewer_inflight`
- 清理 `fingerprint`
- 重建 `user_active`
- 重建 `viewer_active`
- 重建 `quota`

优点：

- `jobs/rush-inventory-preheat` worker 几乎不需要改协议

缺点：

- 接口名与实际职责不一致
- 后续维护时容易误导

#### 方案二：新增明确语义的 `PrimeRushRuntime`

worker 改为调用：

1. `order-rpc.PrimeRushRuntime(showTimeId)`
2. `program-rpc.PrimeSeatLedger(showTimeId)`

优点：

- 名称和职责一致
- 语义清晰，便于后续扩展

缺点：

- 需要同步调整 `jobs/rush-inventory-preheat` 的 worker RPC 适配层与测试

推荐方案二。

### 本期统一收口

无论最终采用哪种 RPC 命名，本期都应保证现有 `jobs/rush-inventory-preheat` worker 最终执行的是同一收口：

1. `order-rpc` 侧完成 rush runtime 预热
2. `program-rpc` 侧完成 seat ledger 预热
3. 两步都成功后，worker 再将 `inventory_preheat_status` 标记为完成

也就是说，预热任务的正式消费端已经存在，本期只调整它消费时触发的预热内容，而不是重做调度链路。

## 测试要求

本期测试至少覆盖下面几类：

### Redis 作用域测试

- `generation` 不再作为 key 作用域输入
- `quota` 仍按 `showTime + ticketCategory` 正确读写
- `user_active/viewer_active` 按 `showTime` 正确读写

### 预热测试

- `PrimeRushRuntime` 会清理 `inflight/fingerprint`
- `PrimeRushRuntime` 会从 DB guard 覆盖重建 `user_active/viewer_active`
- `PrimeRushRuntime` 会覆盖重建 quota
- `PrimeSeatLedger` 仍会从 DB 强制重建所有票档
- `jobs/rush-inventory-preheat` worker 会继续作为正式预热消费者，顺序执行 `order-rpc` 预热与 `program-rpc` 预热
- 仅当两侧预热都成功时，worker 才会把 `inventory_preheat_status` 标记为完成

### Consumer 测试

- `PrepareAttemptForConsume` 对 `SUCCESS / FAILED / PROCESSING` 返回不处理
- `PrepareAttemptForConsume` 对 `ACCEPTED` 原子切换为 `PROCESSING`
- 返回字段足以支撑后续 `FinalizeSuccess / FinalizeFailure`
- consumer 热路径不再依赖 `SCAN`

### 兼容性测试

- 旧 payload 中带 `generation` 字段时，新逻辑仍可解码
- 新 token / 新 event 不带 `generation` 时，整条链路仍可通行

## 风险与迁移

### 风险一：Redis 物理 key rename 导致在途 attempt 丢失

如果在存在 in-flight attempt 时直接切换 key literal，新的 consumer / poll 可能读不到旧 key。

应对方式：

- 本期优先做“逻辑去 generation”
- 对物理 key rename 单独做迁移评估
- 若必须同步改名，应选择无在途 attempt 的维护窗口

### 风险二：未先预热就开售

如果未完成 `PrimeRushRuntime + PrimeSeatLedger` 就直接开售，会出现：

- quota 未准备
- active guard 缺失
- seat ledger 为空或陈旧

因此预热必须成为正式流程前置条件，而不是“尽量做”。

### 风险三：active guard 与 DB guard 暂时不一致

Redis `user_active/viewer_active` 只是投影，极端情况下仍可能短暂失真。

应对方式：

- 预热时从 DB 覆盖重建
- consumer 落库阶段继续依赖 DB guard 唯一约束兜底

## Deferred 项

本期明确延期，不纳入实现：

- 第七点“失败与重试语义统一”
- 所有 `FAILED` 路径的资源释放语义统一
- `fingerprint` 在失败终态中的清理规则统一
- “失败后是否允许旧 purchase token 原地重试”的行为收口

这部分将在后续单独设计。当前阶段只保证：

- 新 token 会生成新 `orderNumber`
- 新逻辑不再依赖 `generation` 隔离轮次
- 开售热路径按 `showTime` 真实维度工作

## 结论

本期方案的核心不是“换一套 key 名字”，而是把系统收口到一套更真实的运行模型：

- `generation` 不再作为业务建模存在
- Redis 明确退回热读模型角色
- 每轮开售前显式强制预热
- `quota`、seat ledger、active guard 分别按真实资源维度维护
- consumer 通过 `showTime` 直接命中 attempt，而不是再走 `SCAN`

这样收口后，二开不再依赖切换 generation，而依赖“重新强制预热本场真实热数据”。这更符合业务语义，也更利于后续扩展与分片。
