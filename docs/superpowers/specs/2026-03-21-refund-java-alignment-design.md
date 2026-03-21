# 退款能力 Java 语义对齐设计

## 背景

当前 `damai-go` 已经具备一条完整的用户主动整单退款主链：

- `gateway-api -> order-api -> order-rpc -> program-rpc/pay-rpc`
- `program-rpc` 负责退款资格判断和已售座位回滚
- `pay-rpc` 负责退款执行和退款账单持久化
- `order-rpc` 负责退款编排和订单状态收敛

对照原 Java 项目后，可以确认 Java 已明确具备以下退款相关语义：

- `program` 域持有退款展示和退款权限字段：
  - `permitRefund`
  - `refundTicketRule`
  - `refundExplain`
- `pay` 域持有退款账单和退款执行能力
- `order` 域在“订单已取消但支付晚到”场景下，会触发补偿退款

同时也确认了两个事实：

1. Java 的 `customize` 域并不是退款规则中心，而是接口限流和调用记录中心。
2. Java 并没有落地一条和当前 Go 等价的“用户主动整单退款闭环”。

因此，本次工作不是把 Go 回退到 Java，而是：

- 以 Java 的退款业务语义为基准补全
- 保留 Go 已有的主动整单退款主链
- 把 Java 中已存在但 Go 仍不完整的字段、状态、文案和兼容逻辑补齐

## 目标

本轮退款补全的目标是：

1. 保留现有 `POST /order/refund` 用户主动整单退款入口
2. 对齐 Java `program` 域退款字段语义
3. 对齐 Java `pay` 域退款账单和退款状态语义
4. 对齐 Java `order` 域“支付晚到补偿退款”语义
5. 统一退款后的订单最终状态为 `4=已退单`
6. 对重复退款请求和补偿重入提供稳定收敛结果

## 非目标

本轮明确不做：

- 部分退款
- 按观演人拆分退款
- 退款申请单
- 人工审核流
- 退款中/退款失败状态扩展
- 新增 `customize` 域承接退款规则
- 独立退款列表、退款详情页面接口

## 方案选择

### 方案 1：严格收缩到 Java 现状

只保留 Java 已明确存在的退款字段和支付退款能力，弱化 Go 当前的条件退款和主动退款主链。

优点：

- 表面上最接近 Java

缺点：

- 会让 Go 已有能力倒退
- 不符合当前仓库已经形成的服务边界

### 方案 2：Java 语义对齐 + Go 主链保留

保留 Go 已有的主动整单退款闭环，同时补齐 Java 的字段语义、状态语义、退款账单语义和支付晚到补偿退款逻辑。

优点：

- 对齐 Java 业务语义
- 不丢失 Go 已有能力
- 变更集中在 `program`、`order`、`pay` 三个服务，边界清晰

缺点：

- 需要补齐多处状态和文案兼容逻辑

### 方案 3：退款规则平台化

把退款规则从 `program` 域再抽成独立规则域。

优点：

- 长期边界更干净

缺点：

- 明显超出当前“以 Java 为基准补全”的范围

### 最终选择

采用方案 2：

- 以 Java 语义为基准补全
- 保留 Go 已有主动整单退款闭环
- 不新增新域，不做平台化扩张

## 设计 1：服务职责与数据流

退款相关职责保持三段式，不引入新服务：

- `program-rpc`
  - 持有节目侧退款业务语义
  - 返回 `permitRefund`、`refundTicketRule`、`refundExplain`
  - 继续负责条件退款判定
  - 继续负责退款后的已售座位回滚
- `pay-rpc`
  - 负责退款执行
  - 负责退款账单持久化
  - 支持按订单号幂等收敛
  - 支持补偿退款重入
- `order-rpc`
  - 负责用户主动退款编排
  - 负责支付晚到补偿退款编排
  - 负责订单和订单票状态收敛

### 主路径 1：用户主动整单退款

固定链路为：

1. `gateway-api` 接收 `/order/refund`
2. `order-api` 鉴权并转发到 `order-rpc`
3. `order-rpc` 校验订单归属和状态
4. `program-rpc` 执行退款资格判断
5. `pay-rpc` 执行退款并生成退款账单
6. `program-rpc` 回滚已售座位
7. `order-rpc` 更新订单和订单票状态为 `已退单`

### 主路径 2：支付晚到补偿退款

固定链路为：

1. 订单已经被取消
2. `order-rpc` 在 `PayCheck` 或支付回调收敛逻辑中发现账单已支付
3. `pay-rpc` 执行补偿退款
4. `order-rpc` 将订单最终状态收敛为 `已退单`

### 关键约束

- 不新增 `customize` 域承接退款规则
- 不新增人工审核环节
- 不支持部分退款
- `permitRefund=2` 视为全额退款
- `permitRefund=0` 视为不可退款
- `permitRefund=1` 继续使用 Go 当前机器规则判定

## 设计 2：数据模型与状态兼容

### `program` 域

以 Java `program` 模型为基准，保留以下退款业务字段：

- `permit_refund`
- `refund_ticket_rule`
- `refund_explain`

字段职责定义如下：

- `permit_refund`
  - `0=不支持退`
  - `1=条件退`
  - `2=全部退`
- `refund_ticket_rule`
  - 只承担展示文案，不承担程序判定
- `refund_explain`
  - 只承担展示文案和拒绝说明，不承担程序判定

Go 已有的 `refund_rule_json` 继续保留，不做删除。

最终职责拆分为：

- Java 风格字段承担业务语义和展示
- `refund_rule_json` 承担 `permitRefund=1` 的机器判定

最终规则为：

- `permit_refund=0`
  - 直接拒绝退款
- `permit_refund=2`
  - 直接允许全额退款
- `permit_refund=1`
  - 继续走 `refund_rule_json` 判定
  - 前端和错误提示优先使用 `refund_ticket_rule`、`refund_explain`

### `order` 域

订单状态保持并统一说明：

- `1=未支付`
- `2=已取消`
- `3=已支付`
- `4=已退单`

适用表：

- `d_order`
- `d_order_ticket_user`

状态收敛规则固定为：

- 用户主动退款成功：`已支付 -> 已退单`
- 已取消后支付晚到并补偿退款：最终也收敛到 `已退单`

不保留“已取消但已退款”的中间业务状态。

### `pay` 域

支付域继续保留：

- `d_pay_bill.pay_status`
  - `1=created`
  - `2=paid`
  - `3=refunded`
- `d_refund_bill`

退款账单字段以 Java 语义为基准，保留：

- `id`
- `out_order_no` 或对等订单号字段
- `pay_bill_id`
- `refund_amount`
- `refund_status`
- `refund_time`
- `reason`
- `status`
- `create_time`
- `edit_time`

其中最关键的兼容点是：

- 退款账单按订单号唯一收敛
- 重复退款请求直接命中已有退款账单结果

### 文案兼容

退款相关用户可见文案优先使用：

- `refundTicketRule`
- `refundExplain`

规则如下：

- `permit_refund=0`
  - 拒绝原因优先取 `refundExplain`
- `permit_refund=1` 但未命中规则
  - 拒绝原因优先拼接 `refundTicketRule` 与 `refundExplain`
- 无业务文案时
  - 才返回默认技术错误

## 设计 3：接口契约与兼容行为

### `program-rpc`

不新增新的退款展示接口，继续复用节目查询链路返回退款字段。

需要保证：

- 节目详情稳定返回 `permitRefund`
- 节目详情稳定返回 `refundTicketRule`
- 节目详情稳定返回 `refundExplain`

保留 `EvaluateRefundRule`，但补齐行为兼容：

- `permitRefund=0`
  - 返回拒绝退款
  - `rejectReason` 优先使用 `refundExplain`
- `permitRefund=1`
  - 按 `refund_rule_json` 评估
  - 未命中时 `rejectReason` 优先拼装 `refundTicketRule`、`refundExplain`
- `permitRefund=2`
  - 直接允许全额退款

### `pay-rpc`

继续保留现有退款 RPC，不新增并行接口。

需要补齐：

- 按订单号幂等
- 已退款重复调用返回已有退款结果
- 返回值稳定包含：
  - `refundBillNo`
  - `orderNumber`
  - `refundAmount`
  - `payStatus`
  - `refundTime`
- 支持补偿退款重入

### `order-rpc`

保留现有 `RefundOrder`。

需补齐行为：

- 用户主动退款只允许从 `已支付` 进入
- 订单已经 `已退单` 时，重复请求返回已收敛结果
- 订单已取消但支付账单显示已支付时，触发补偿退款
- 补偿退款完成后，订单最终状态统一更新为 `已退单`

关键兼容入口为：

- `/order/refund`
- `/order/pay/check`
- 支付回调的状态收敛逻辑

### `order-api` 和 `gateway-api`

不新增新路由，继续使用：

- `POST /order/refund`

返回语义需要稳定包含：

- 退款金额
- 退款单号
- 退款时间

并且满足：

- 已退款重复请求返回当前已退款结果
- 退款被拒绝时优先返回业务文案

## 设计 4：测试与实施顺序

### 阶段 1：修测试基线

先补齐当前接口扩展造成的测试桩失配，恢复 `go test ./...` 可演进状态。

已确认需要修复的测试包括：

- `services/program-api/tests/integration/program_logic_test.go`
- `jobs/order-close/tests/integration/closeexpiredorderslogic_test.go`

### 阶段 2：补 `program` 退款语义

重点验证：

- `permit_refund=0`
- `permit_refund=1` 命中条件退款
- `permit_refund=1` 未命中条件退款
- `permit_refund=2`
- Java 风格文案优先级

### 阶段 3：补 `pay` 退款账单兼容

重点验证：

- 首次退款成功
- 重复退款命中已有退款账单
- 支付账单不存在
- 支付状态不允许退款

### 阶段 4：补 `order` 状态收敛

重点验证：

- 主动整单退款成功
- 已退款重复调用
- 已取消后支付晚到触发补偿退款
- 拒绝退款时返回业务文案

### 阶段 5：补文档与验收

需要更新：

- `README.md`
- `docs/api/order-refund-acceptance.md`

并补充：

- 支付晚到补偿退款验收步骤
- 全量 `go test ./...` 验证

## 风险与控制

### 风险 1：文案字段和机器规则冲突

表现：

- `refund_ticket_rule` 文案与 `refund_rule_json` 规则不一致

控制：

- 程序判定以 `refund_rule_json` 为准
- 文案仅承担展示职责
- 在测试和种子数据中保持两者一致

### 风险 2：支付晚到补偿和主动退款竞争

表现：

- 用户主动退款与支付晚到补偿并发进入

控制：

- `pay-rpc` 对订单号退款幂等
- `order-rpc` 对最终订单状态收敛到 `已退单`

### 风险 3：座位释放重复执行

表现：

- 重复退款或补偿重入导致重复回滚已售座位

控制：

- `program-rpc` 已售座位释放保持幂等请求语义
- `order-rpc` 在状态已收敛场景下直接返回最终结果

## 结论

本轮退款补全不追求做成平台，而是围绕当前主链把 Java 已存在的退款语义补齐：

- `program` 补业务字段语义和文案兼容
- `pay` 补退款账单和退款幂等兼容
- `order` 补支付晚到补偿退款和最终状态收敛

最终形态是：

- 业务语义与 Java 对齐
- 能力闭环保留 Go 当前整单退款主链
- 不引入额外服务和额外退款状态
