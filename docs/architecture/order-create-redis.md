# 用户抢单到 Kafka 落单消息阶段的 Redis 设计

本文只整理这段链路里 Redis 做了什么：

- `gateway-api -> order-api -> order-rpc -> program-rpc + user-rpc -> Kafka producer`
- 截止点是 `order-rpc` 成功向 Kafka 发送 `order.create.command.v1`
- 不包含 Kafka consumer 落库、支付、关单、退款等后续阶段

## 1. 结论先看

这一段链路里，Redis 主要承担 3 类职责：

| 类别 | 用途 | 主要数据类型 |
| --- | --- | --- |
| 用户购票限额账本 | 判断用户在节目维度是否还能继续下单 | `Hash`、`String` |
| 座位冻结账本 | 判断是否还有座位可冻，并记录本次冻结的座位集合 | `Hash`、`ZSet`、`String` |
| 订单创建中 marker | Kafka 已发出但 MySQL 可能暂时还查不到时，给前端一个短时可见性标记 | `String` |

额外说明：

- 当前这段链路的“防重复提交锁”走的是 etcd，不是 Redis。
- `gateway-api` / `order-api` 本身不直接操作 Redis。
- `user-rpc` 在这段链路里查询的是 MySQL 用户和观演人数据，不走 Redis。

## 2. 时序总览

1. `order-api` 接到 `/order/create` 请求，透传给 `order-rpc`，自己不读写 Redis。
2. `order-rpc` 先做用户节目维度的购票限额预占。
3. `order-rpc` 调 `program-rpc.AutoAssignAndFreezeSeats`，由 `program-rpc` 在 Redis seat ledger 上冻结座位。
4. `order-rpc` 组装 `order create event`，发往 Kafka topic `order.create.command.v1`。
5. Kafka 发送成功后，`order-rpc` 在 Redis 里写一个短时 marker，表示“订单创建消息已经发出”。

## 3. 用户购票限额账本

### 3.1 用途

这份账本解决的是：

- 这个用户在某个 `programId` 维度，是否还允许继续下单
- 当前未支付订单已经占用了多少张票
- 在失败补偿时，是否需要把本次预占回滚掉

它不是 seat ledger 的替代品。

## 3.2 Redis 键设计

### 3.2.1 ledger 主键

- 类型：`Hash`
- key 模板：`damai-go:order:purchase-limit:ledger:{userId}:{programId}`

字段设计：

| field | 含义 | value 示例 |
| --- | --- | --- |
| `active_count` | 当前用户在该节目下已占用的总票数 | `3` |
| `reservation:{orderNumber}` | 某笔未支付订单预占的票数 | `2` |

例子：

```text
key: damai-go:order:purchase-limit:ledger:3001:10001
type: hash

active_count = 3
reservation:91001 = 2
reservation:91009 = 1
```

说明：

- `active_count` 的底层来源是 MySQL 中该用户该节目下 `order_status in (1, 3)` 的票数总和，也就是“未支付 + 已支付”。
- `reservation:{orderNumber}` 只记录未支付订单预占，用于幂等和失败回滚。

### 3.2.2 loading 标记

- 类型：`String`
- key 模板：`damai-go:order:purchase-limit:loading:{userId}:{programId}`
- value：固定为 `"1"`

用途：

- 当 ledger 不存在时，说明 Redis 账本还没就绪
- 系统会尝试用这个 key 做一个短时“冷加载中”标记，避免并发重复回源 MySQL

### 3.2.3 TTL

- ledger 默认 TTL：`4h`
- loading 默认 TTL：`3s`

## 3.3 运行逻辑

下单时，`order-rpc` 会先执行一段 Lua：

- ledger 不存在：返回 `-1`
  - 业务上表现为 `order limit ledger not ready`
  - 同时异步调度一次从 MySQL 回填 Redis 的加载
- `active_count + 本次 ticketCount > limit`：返回 `0`
  - 业务上表现为超出购票上限
- 允许下单：返回 `1`
  - `HINCRBY active_count`
  - `HSET reservation:{orderNumber} = ticketCount`

回填时，系统会从 MySQL 装配出：

- `active_count`
- 当前所有未支付订单对应的 `reservation:{orderNumber}`

## 3.4 这段链路里会如何回滚

在 Kafka 发送前的失败场景里，会尝试回滚这份账本：

- 座位冻结失败
- event 构建失败
- producer 未初始化
- event 序列化失败

回滚动作：

- 检查 ledger 是否 ready
- 检查本次 `reservation:{orderNumber}` 是否存在
- 存在则执行 Lua：
  - `active_count -= reservedCount`
  - `HDEL reservation:{orderNumber}`

当前实现注意点：

- 如果已经执行 `OrderCreateProducer.Send(...)`
- 且该调用直接返回 error
- 当前代码不会立刻回滚 Redis 里的 purchase limit reservation

也就是说，这个分支下：

- marker 不会写入
- 但 purchase limit 账本可能已经预占成功，后续需要依赖其他补偿或账本 TTL 自然过期

## 4. 座位冻结账本

### 4.1 用途

这份账本解决的是：

- 某个节目、某个票档下，现在还有多少座位可售
- 本次冻结具体拿到了哪些座位
- 同一个 `requestNo` 是否已经冻结过
- 哪些冻结已经过期，需要回收

当前 `/order/create` 阶段的冻结判断口径看 Redis，不看 MySQL。

## 4.2 Redis 键设计

### 4.2.1 stock 主键

- 类型：`Hash`
- key 模板：`damai-go:program:seat-ledger:stock:{programId}:{ticketCategoryId}`

字段设计：

| field | 含义 | value 示例 |
| --- | --- | --- |
| `available_count` | 当前剩余可售座位数 | `998` |

### 4.2.2 available seats

- 类型：`ZSet`
- key 模板：`damai-go:program:seat-ledger:available:{programId}:{ticketCategoryId}`

member 编码格式：

```text
seatId|ticketCategoryId|rowCode|colCode|price
```

例如：

```text
71001|40001|1|1|380
71002|40001|1|2|380
```

score 规则：

```text
rowCode * 1000000 + colCode
```

用途：

- 按行列顺序读取可售座位
- 优先尝试同排连续座位
- 如果找不到连续座位，再退化为按排序取前 `N` 个

### 4.2.3 frozen seats

- 类型：`ZSet`
- key 模板：`damai-go:program:seat-ledger:frozen:{programId}:{ticketCategoryId}:{freezeToken}`

member 和 score 与 `available` 保持一致。

用途：

- 记录某一次冻结具体冻结了哪些座位
- 释放冻结时，把这些 member 从 frozen 放回 available

### 4.2.4 sold seats

- 类型：`ZSet`
- key 模板：`damai-go:program:seat-ledger:sold:{programId}:{ticketCategoryId}`

说明：

- 这个 key 属于 seat ledger 设计的一部分
- 但“用户抢单直到 Kafka 发落单消息”这一段不会写它
- 它主要用于后续确认冻结、把座位转成已售时使用

### 4.2.5 freeze metadata

- 类型：`String`
- key 模板：`damai-go:program:seat-ledger:freeze:meta:{freezeToken}`
- value：JSON

JSON 结构核心字段：

| 字段 | 含义 |
| --- | --- |
| `freezeToken` | 冻结 token |
| `requestNo` | 本次冻结请求号 |
| `programId` | 节目 ID |
| `ticketCategoryId` | 票档 ID |
| `seatCount` | 冻结座位数 |
| `freezeStatus` | 冻结状态 |
| `expireAt` | 过期时间，Unix 秒 |
| `releaseReason` | 释放原因，可选 |
| `releaseAt` | 释放时间，可选 |
| `updatedAt` | 最后更新时间 |

状态值：

| 值 | 含义 |
| --- | --- |
| `1` | frozen |
| `2` | released |
| `3` | expired |
| `4` | confirmed |

示例：

```json
{
  "freezeToken": "freeze-123456789",
  "requestNo": "order-123456789",
  "programId": 10001,
  "ticketCategoryId": 40001,
  "seatCount": 2,
  "freezeStatus": 1,
  "expireAt": 1770000000,
  "updatedAt": 1770000000
}
```

### 4.2.6 requestNo 幂等索引

- 类型：`String`
- key 模板：`damai-go:program:seat-ledger:freeze:req:{requestNo}`
- value：`freezeToken`

用途：

- 同一个 `requestNo` 重试时，直接找到已有冻结
- 避免重复分配不同座位

### 4.2.7 过期冻结索引

- 类型：`ZSet`
- key 模板：`damai-go:program:seat-ledger:freeze:index:{programId}:{ticketCategoryId}`
- member：`freezeToken`
- score：`expireAt`

用途：

- 按 `expireAt <= now` 查出已经过期的冻结 token
- 在新冻结前先回收过期冻结

### 4.2.8 loading 标记

- 类型：`String`
- key 模板：`damai-go:program:seat-ledger:loading:{programId}:{ticketCategoryId}`
- value：固定为 `"1"`

用途：

- seat ledger 不存在时，说明 Redis 账本未 ready
- 用于防止并发重复回源 MySQL

### 4.2.9 TTL

- stock 默认 TTL：`4h`
- available / frozen / sold / freeze metadata 默认 TTL：`4h`
- loading 默认 TTL：`3s`

## 4.3 运行逻辑

新建冻结时，`program-rpc` 大致按下面流程操作 Redis：

1. 用 `freeze:req:{requestNo}` 查是否已经存在冻结
2. 用 `freeze:index:{programId}:{ticketCategoryId}` 找出过期冻结 token
3. 逐个释放过期冻结：
   - 把 frozen zset 中的座位放回 available zset
   - `available_count` 加回去
   - metadata 状态改成 `expired`
4. 新生成 `freezeToken`
5. 执行 Lua：
   - 检查 `stock` 是否存在
   - 检查 `available_count` 是否足够
   - 从 `available` 里选座
   - 选中的 member 从 `available` 移到 `frozen:{freezeToken}`
   - `available_count` 扣减
6. 写入 `freeze:meta:{freezeToken}`
7. 写入 `freeze:req:{requestNo}`
8. 把 `freezeToken` 写入 `freeze:index:{programId}:{ticketCategoryId}`

选座策略：

- 优先选同排连续座位
- 如果没有满足数量的连续座位，退化为按行列排序后的前 `N` 个

## 4.4 这段链路里会如何释放冻结

在“抢单到 Kafka 发消息”这段里，冻结释放主要发生在创建失败补偿阶段：

- 座位冻结后、Kafka 发送前失败
- 订单创建超时/过期

释放动作：

- 读取 `freeze:meta:{freezeToken}`
- 校验冻结状态
- 把 `frozen:{freezeToken}` 的座位放回 `available`
- `available_count` 加回去
- metadata 状态标成 `released` 或 `expired`
- 从 `freeze:index:{programId}:{ticketCategoryId}` 中移除该 `freezeToken`

## 5. Kafka 成功后的短时 marker

### 5.1 键设计

- 类型：`String`
- key 模板：`order:create:marker:{orderNumber}`
- value：`orderNumber` 的字符串

例如：

```text
key: order:create:marker:91001
type: string
value: 91001
ttl: 60s
```

### 5.2 用途

只有在 Kafka 发送成功后，系统才会写这个 marker。

它解决的是一个短时可见性问题：

- Kafka 消息已经发出
- 但 consumer 还没把订单落到 MySQL
- 这时如果前端立刻查订单，可能会短暂查不到

所以这个 marker 表示的是：

- “这笔订单创建流程已经把消息发出去了”
- “只是最终订单数据可能暂时还没落库”

## 6. 不属于 Redis 的内容

下面这些能力在这段链路里不是 Redis：

- `order-api` 只是转 RPC，不操作 Redis
- `user-rpc.GetUserAndTicketUserList` 查的是 MySQL
- 下单防重锁走的是 etcd

etcd 防重锁的 key 语义是：

```text
create_order:{userId}:{programId}
```

默认前缀是：

```text
/damai-go/repeat-guard/order-create/
```

它和 Redis purchase limit ledger 是两层不同能力：

- etcd 锁解决“同一用户短时间重复提交”
- Redis purchase limit ledger 解决“这个用户在该节目维度还能不能再买”

## 7. 当前实现的关键注意点

### 7.1 order limit ledger 和 seat ledger 缺一不可

- `seat ledger` 解决“有没有座位可冻”
- `order limit ledger` 解决“用户还能不能继续下单”

只保证其中一个 ready 还不够。

### 7.2 创建订单阶段的冻结判断口径看 Redis

当前 `/order/create` 阶段：

- 不以 MySQL seat 状态作为冻结前置判断
- 主要看 Redis seat ledger 状态

### 7.3 Kafka send error 分支当前没有即时 Redis 回滚

当前代码里：

- Kafka 发送成功后才写 `order:create:marker:{orderNumber}`
- 但如果 `Send(...)` 直接报错
- 当前没有在该分支同步回滚 purchase limit reservation 和 seat freeze

这是阅读现有实现后能确认的当前行为，不是这里额外推断出来的新设计。

## 8. 相关实现位置

- `services/order-rpc/internal/logic/create_order_logic.go`
- `services/order-rpc/internal/limitcache/purchase_limit_store.go`
- `services/order-rpc/internal/limitcache/purchase_limit_loader.go`
- `services/order-rpc/internal/limitcache/reserve_purchase_limit.lua`
- `services/order-rpc/internal/limitcache/release_purchase_limit.lua`
- `services/order-rpc/internal/logic/order_cache_marker.go`
- `services/order-rpc/internal/logic/order_create_compensation.go`
- `services/program-rpc/internal/logic/auto_assign_and_freeze_seats_logic.go`
- `services/program-rpc/internal/seatcache/seat_stock_store.go`
- `services/program-rpc/internal/seatcache/seat_stock_loader.go`
- `services/program-rpc/internal/seatcache/freeze_auto_assigned_seats.lua`
- `services/program-rpc/internal/seatcache/release_frozen_seats.lua`
- `services/program-rpc/internal/seatcache/seat_freeze_metadata.go`
