# 规则化退款闭环设计

## 背景

当前 `damai-go` 已经具备以下交易主链路能力：

- `user-api` / `user-rpc`：注册、登录、实名、观演人管理
- `program-api` / `program-rpc`：节目查询、预下单详情、系统分配并冻结座位、支付后确认冻结座位
- `order-api` / `order-rpc`：创建订单、查询订单、取消订单、模拟支付、支付状态查询、超时关单
- `pay-rpc`：模拟支付账单创建与查询
- `gateway-api`：统一 HTTP 入口和订单链路鉴权

当前仓库的售后能力仍停留在“未支付订单取消”和“超时关单”阶段，尚未支持：

- 已支付订单退款
- 节目退款规则判定
- 退款账单持久化
- 已售座位回滚为可售

对照原 Java 项目，可以确认售后链路至少包含以下核心语义：

- 订单状态存在 `4=已退单`
- `pay` 域提供退款能力和退款账单
- `program` 域持有 `permitRefund`、`refundTicketRule`、`refundExplain`
- `order` 域负责串联支付退款和订单状态收敛

因此，下一阶段的目标不是继续扩展随机接口，而是补齐一个最小但完整的“规则化退款闭环”。

## 目标

在保持现有 `gateway -> order-api -> order-rpc -> program-rpc/pay-rpc` 服务边界不变的前提下，新增一条可执行的已支付订单退款链路，满足以下目标：

1. 用户可通过网关发起已支付订单退款
2. 退款资格由节目退款规则决定，而不是写死在订单域
3. 退款成功后生成退款账单
4. 退款成功后订单和购票人订单快照更新为 `已退单`
5. 退款成功后已售座位恢复为可售库存
6. 整条链路支持重复调用时的状态收敛，不因部分成功造成永久脏状态

## 非目标

本轮明确不包含以下能力：

- 部分退票或按观演人拆分退款
- 人工审核流、退款申请单、审核中状态
- 支付晚到后的自动退款补偿
- 独立退款列表页、退款查询页
- `customize` 域通用规则平台
- 多支付渠道差异化退款逻辑

## 方案选择

### 方案 1：最小规则接入

仅根据 `permitRefund` 判断：

- `0` 不可退
- `2` 可全退
- `1` 暂不支持自动退

优点是改动最小，缺点是 `refundTicketRule` 和 `refundExplain` 仍然只是展示字段，不是真正的规则化退款。

### 方案 2：Program 域承载结构化退款规则，Order 域做退款编排

在 `program` 域新增最小可计算退款规则模型，由 `program-rpc` 负责退款资格评估；`order-rpc` 只做交易编排；`pay-rpc` 负责退款执行和退款账单。

优点：

- 服务边界清晰
- 贴近原 Java 语义
- 能在不过度设计的前提下实现规则化退款

缺点：

- 需要同步修改 `program/order/pay` 三个服务的 proto、SQL、模型和测试

### 方案 3：通用规则引擎

把退款规则抽到通用规则平台或 `customize` 域。

优点是未来可复用，缺点是当前阶段明显过重，会把单一业务闭环扩成平台化建设。

### 最终选择

采用方案 2，但严格限制范围：

- 本轮只做“整单退款”
- 本轮只做“最小结构化退款规则”
- 不提前引入通用规则平台

## 设计 1：架构边界与数据流

### 服务职责

- `program` 域负责退款资格评估，以及退款后的已售座位回滚
- `order` 域负责交易编排、订单归属校验、状态收敛
- `pay` 域负责退款执行和退款账单持久化
- 外部调用方仍然只通过 `gateway-api` 访问退款入口

### 对外入口

用户退款入口统一为：

- `gateway-api -> /order/refund`

下游链路保持：

- `gateway-api -> order-api -> order-rpc`
- `order-rpc -> program-rpc`
- `order-rpc -> pay-rpc`

### 退款主链路

退款主链路固定为：

1. 用户调用 `/order/refund`
2. `order-rpc` 锁定订单并校验当前用户归属
3. 校验订单状态必须为 `已支付`
4. 调用 `program-rpc` 评估退款资格、退款比例和应退金额
5. 调用 `pay-rpc` 执行退款
6. 调用 `program-rpc` 回滚已售座位
7. 本地更新 `d_order` 和 `d_order_ticket_user` 为 `已退单`

### 一致性策略

本轮不引入分布式事务，而采用“远端幂等 + 本地状态收敛”的一致性策略：

- `pay-rpc` 的退款按 `orderNumber` 幂等
- `program-rpc` 的已售座位回滚按 `requestNo=refund-<orderNumber>` 幂等
- `order-rpc` 允许对部分成功的退款请求重复触发，并将状态补齐

## 设计 2：数据模型与状态机

### 订单状态

`d_order.order_status` 与 `d_order_ticket_user.order_status` 扩展为：

- `1=未支付`
- `2=已取消`
- `3=已支付`
- `4=已退单`

本轮不在订单表新增 `refund_time` 或 `refund_amount` 字段，退款事实以退款账单为准。

### 支付状态与退款账单

`d_pay_bill.pay_status` 扩展为：

- `1=created`
- `2=paid`
- `3=refunded`

新增 `d_refund_bill` 用于记录退款账单，建议字段如下：

- `id`
- `refund_bill_no`
- `order_number`
- `pay_bill_id`
- `user_id`
- `refund_amount`
- `refund_status`
- `refund_reason`
- `refund_time`
- `create_time`
- `edit_time`
- `status`

### 节目退款规则

`program` 域已有：

- `permit_refund`
- `refund_ticket_rule`
- `refund_explain`

为支持机器可计算的退款规则，本轮建议在 `d_program` 新增：

- `refund_rule_json`

其中：

- `refund_ticket_rule` 和 `refund_explain` 继续承担文案展示职责
- `refund_rule_json` 仅承担程序判定职责

### 规则表达

`refund_rule_json` 仅支持最小阶梯退款模型：

```json
{
  "version": 1,
  "stages": [
    { "beforeMinutes": 10080, "refundPercent": 100 },
    { "beforeMinutes": 1440, "refundPercent": 80 },
    { "beforeMinutes": 120, "refundPercent": 50 }
  ]
}
```

语义固定为：

- `permitRefund=0`：不可退
- `permitRefund=2`：全额可退，不依赖 `refund_rule_json`
- `permitRefund=1`：按 `refund_rule_json` 计算；若当前不命中任何一档，则不可退

本轮只支持整单退款，因此退款金额计算方式固定为：

- `refundAmount = order.order_price * refundPercent / 100`

### 座位与冻结记录

当前支付成功后，`order-rpc` 会调用 `program-rpc.ConfirmSeatFreeze`，把座位从 `frozen` 变成 `sold`。因此退款时不能复用当前的 `ReleaseSeatFreeze`。

退款回滚座位的依据应为：

- `d_order_ticket_user.seat_id`

退款时将这些座位从：

- `seat_status=3 sold`

回滚为：

- `seat_status=1 available`

`d_seat_freeze` 的历史记录保持不变，退款不再修改其状态。

### 状态机

状态机固定为：

- 订单：`unpaid -> cancelled`、`unpaid -> paid`、`paid -> refunded`
- 支付账单：`created -> paid -> refunded`
- 座位：`available -> frozen -> sold -> available`
- 座位冻结记录：`frozen -> released/expired/confirmed`

## 设计 3：接口边界、幂等与错误处理

### 对外接口

新增用户入口：

- `POST /order/refund`

请求体最小保持为：

- `orderNumber` 必填
- `reason` 选填

请求头保持现有订单链路约束：

- `Authorization: Bearer <token>`
- `X-Channel-Code: 0001`

### Order API / RPC

`order-api` 仅做鉴权和参数映射。

`order-rpc` 新增：

- `RefundOrder(userId, orderNumber, reason)`

返回结果建议包含：

- `orderNumber`
- `orderStatus`
- `refundAmount`
- `refundPercent`
- `refundBillNo`
- `refundTime`

### Program RPC

`program-rpc` 新增两个能力：

1. `EvaluateRefundRule`
   - 输入：`programId`、`orderShowTime`、`orderAmount`
   - 输出：是否可退、退款比例、应退金额、失败原因

2. `ReleaseSoldSeats`
   - 输入：`programId`、`seatIds`、`requestNo`
   - 输出：是否成功

### Pay RPC

`pay-rpc` 新增：

- `Refund(orderNumber, userId, amount, channel, reason)`

返回结果至少包含：

- `refundBillNo`
- `payStatus`
- `refundTime`

### 幂等策略

幂等约束固定为：

- `pay-rpc.Refund` 以 `orderNumber` 幂等，若已退款则直接返回已有退款账单
- `program-rpc.ReleaseSoldSeats` 以 `requestNo=refund-<orderNumber>` 幂等，重复调用直接成功
- `order-rpc.RefundOrder` 若订单已为 `已退单`，直接返回当前退款结果，不报错

### 部分成功场景

必须显式支持以下状态收敛场景：

- `pay-rpc` 已退款成功，但 `program-rpc.ReleaseSoldSeats` 失败
- `pay-rpc` 已退款成功，座位已回滚，但本地订单状态仍停留在 `已支付`

因此 `order-rpc.RefundOrder` 在发现：

- 支付账单已退款
- 订单仍为 `已支付`

时，不应再次发起退款，而应继续补做：

- 已售座位回滚
- 本地订单状态更新

### 错误分类

错误分类建议如下：

- `NotFound`
  - 订单不存在
  - 订单不属于当前用户

- `FailedPrecondition`
  - 订单不是已支付状态
  - 节目不允许退款
  - 退款时间窗口已过
  - 退款规则不匹配

- `Internal`
  - `refund_rule_json` 配置非法
  - 下游 RPC 异常
  - 数据库事务失败

## 设计 4：测试策略与本轮切分

### 测试目标

本轮测试优先验证以下能力：

- 退款规则判断正确
- 退款账单落库正确
- 订单状态能够稳定收敛到 `已退单`
- 已售座位可正确回滚为可售

### 测试目录

遵守当前仓库测试布局约束：

- `services/program-rpc/tests/integration/`
- `services/pay-rpc/tests/integration/`
- `services/order-rpc/tests/integration/`
- 根级 `scripts/acceptance/` 与 `docs/api/`

### Program RPC 测试

至少覆盖：

1. `permitRefund=0` 时拒绝退款
2. `permitRefund=2` 时允许全额退款
3. `permitRefund=1` 时按阶梯规则返回正确比例和金额
4. `ReleaseSoldSeats` 能回滚已售座位，重复执行仍成功

### Pay RPC 测试

至少覆盖：

1. 已支付账单退款后 `pay_status=refunded`
2. 新增 `d_refund_bill` 记录正确
3. 同一 `orderNumber` 重复退款幂等返回，不重复写退款账单

### Order RPC 测试

至少覆盖：

1. 已支付订单退款成功后，订单与购票人订单快照更新为 `已退单`
2. 未支付、已取消、已退单订单调用退款时被拒绝
3. 节目规则判定不可退时返回明确错误
4. `pay-rpc` 已退款但本地状态未更新时，重复调用可完成状态收敛
5. 退款成功后节目余票恢复

### 验收脚本

新增一条退款验收主路径：

1. 复用当前下单主路径创建并支付订单
2. 调用 `/order/refund`
3. 查询 `/order/get`
4. 查询 `/order/pay/check`
5. 查询节目余票

验收成功标准：

- 订单状态为 `已退单`
- 支付账单状态为 `refunded`
- 退款账单已生成
- 节目余票已恢复

### 实施切分

本轮建议切分为 5 个阶段：

1. 扩表结构和状态枚举
2. `program-rpc`：退款规则评估 + 已售座位回滚
3. `pay-rpc`：退款接口 + 退款账单 + 幂等
4. `order-rpc/order-api/gateway`：退款编排和对外入口
5. 文档与验收脚本：补退款主路径

## 成功标准

本轮完成后，应满足：

- 用户可以对已支付订单发起整单退款
- 退款资格受节目配置约束
- 退款成功后支付账单与退款账单可追溯
- 订单状态和购票人订单状态收敛到 `已退单`
- 已售座位回滚为可售库存
- 重复触发同一笔退款不会重复扣减或重复落账

## 实施后的下一步

若本轮退款闭环稳定，后续可按优先级继续推进：

1. 支付晚到后的自动退款补偿
2. 部分退票
3. 售后列表与退款查询
4. `customize` 域规则平台化
