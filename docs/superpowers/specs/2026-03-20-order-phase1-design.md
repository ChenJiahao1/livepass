# Order Phase 1 Design

**Date:** 2026-03-20

## Goal

在 `damai-go` 中落地 `order` 域第一阶段最小闭环，优先打通“下单创建、订单查询、主动取消、超时关闭并释放冻结座位”。

本设计参考原 Java 项目的业务语义与主数据结构，但在当前 Go 重建阶段采用更简单的同步化实现：

- 保持主单 `d_order` 与购票人明细 `d_order_ticket_user` 的拆分
- 保持订单状态语义与 Java 对齐
- 保持 `order-api -> order-rpc` 的 go-zero 分层
- 不引入 Kafka、延迟消息、Redis 订单缓存和支付链路

## Scope

本轮覆盖：

- `order-api` 对外 HTTP 接口
- `order-rpc` 订单核心领域逻辑
- `jobs/order-close/` 定时关闭未支付订单
- 订单创建、订单列表、订单详情、订单取消
- 调用 `program-rpc` 完成节目预下单校验、自动分配并冻结座位、释放冻结
- 调用 `user-rpc` 校验购票人与当前登录用户关系
- 主单和购票人明细快照落库

本轮不覆盖：

- 支付、支付状态查询、支付回调
- 退款和售后
- 手动选座
- 网关聚合
- 订单创建接口幂等
- Kafka、延迟消息、Redis 订单缓存

## Architecture

### Service Boundary

新增三个模块：

- `services/order-api/`
- `services/order-rpc/`
- `jobs/order-close/`

职责划分如下：

- `order-api` 只负责 HTTP 入参绑定、登录用户透传、RPC 调用和响应映射
- `order-rpc` 负责订单领域规则、跨域编排、事务落库和关闭补偿
- `jobs/order-close/` 负责扫描已超时的未支付订单，并调用 `order-rpc` 执行关闭

### Java Alignment

本轮只对齐 Java 的业务语义和数据模型，不直接复刻 Java 的基础设施实现。

保留的 Java 语义包括：

- `d_order` 主单表
- `d_order_ticket_user` 购票人订单明细表
- `order_status = 1` 未支付，`order_status = 2` 已取消
- 创建订单时落节目快照和购票人快照
- 取消订单或超时关闭时释放冻结座位

本轮简化的实现包括：

- 用同步 RPC 调用代替异步消息编排
- 用定时任务扫描代替延迟消息关单
- 不引入缓存态订单或支付校验链路

### Dependency Boundary

`order-rpc` 作为领域核心，依赖两个已有服务：

- `program-rpc`
- `user-rpc`

依赖使用原则：

- 节目标题、图片、演出时间、地点、票档价格和限购规则统一以 `program-rpc` 返回数据为准
- 购票人归属、购票人身份信息统一以 `user-rpc` 返回数据为准
- 订单金额、购票张数和座位快照都由 `order-rpc` 在服务端组装，不能信任前端传值

## Data Model

### d_order

表示订单主单，与 Java `d_order` 语义保持一致，但当前阶段采用单库单表。

建议字段：

- `id`
- `order_number`
- `program_id`
- `program_title`
- `program_item_picture`
- `program_place`
- `program_show_time`
- `program_permit_choose_seat`
- `user_id`
- `distribution_mode`
- `take_ticket_mode`
- `ticket_count`
- `order_price`
- `order_status`
- `freeze_token`
- `order_expire_time`
- `create_order_time`
- `cancel_order_time`
- `status`
- `create_time`
- `edit_time`

状态约束：

- `order_status = 1` 表示 `unpaid`
- `order_status = 2` 表示 `cancelled`

索引建议：

- 唯一索引：`uk_order_number(order_number)`
- 普通索引：`idx_user_status_time(user_id, order_status, create_order_time)`
- 普通索引：`idx_program_user_status(program_id, user_id, order_status)`
- 普通索引：`idx_close_scan(order_status, order_expire_time)`

### d_order_ticket_user

表示订单下的购票人明细和座位快照。

建议字段：

- `id`
- `order_number`
- `user_id`
- `ticket_user_id`
- `ticket_user_name`
- `ticket_user_id_number`
- `ticket_category_id`
- `ticket_category_name`
- `ticket_price`
- `seat_id`
- `seat_row`
- `seat_col`
- `seat_price`
- `order_status`
- `create_order_time`
- `status`
- `create_time`
- `edit_time`

索引建议：

- 普通索引：`idx_order_number(order_number)`
- 普通索引：`idx_user_ticket_user(user_id, ticket_user_id)`
- 普通索引：`idx_create_order_time(create_order_time)`

### Simplifications

本轮刻意不新增：

- 订单支付表
- 订单退款表
- 订单状态流转日志表

当前只保留形成闭环所必需的主单与明细快照。后续若要接支付或售后，再围绕现有主单扩展。

## HTTP Contract

### Create Order

建议路由：

- `POST /order/create`

请求字段：

- `programId`
- `ticketCategoryId`
- `ticketUserIds`
- `distributionMode`
- `takeTicketMode`

响应字段：

- `orderNumber`

约束：

- 不接受前端传 `orderPrice`
- 不接受前端传节目快照
- 不接受前端传座位信息

### List Orders

建议路由：

- `POST /order/select/list`

请求字段：

- `pageNumber`
- `pageSize`
- `orderStatus`

响应列表至少包含：

- `orderNumber`
- `programTitle`
- `programItemPicture`
- `programShowTime`
- `orderPrice`
- `orderStatus`
- `ticketCount`
- `createOrderTime`

### Get Order

建议路由：

- `POST /order/get`

请求字段：

- `orderNumber`

响应字段至少包含：

- 主单快照
- 购票人明细
- 座位明细

### Cancel Order

建议路由：

- `POST /order/cancel`

请求字段：

- `orderNumber`

响应字段：

- `success`

约束：

- 只允许取消当前登录用户自己的订单
- 只允许取消未支付订单

## RPC Contract

本轮建议在 `order-rpc` 提供以下内部方法：

- `CreateOrder`
- `ListOrders`
- `GetOrder`
- `CancelOrder`
- `CloseExpiredOrders`
- `CountActiveTicketsByUserProgram`

说明：

- `CreateOrder` 负责下单完整链路
- `ListOrders` 和 `GetOrder` 负责订单读取
- `CancelOrder` 负责主动取消与释放冻结
- `CloseExpiredOrders` 供 `jobs/order-close/` 定时调用
- `CountActiveTicketsByUserProgram` 供创建订单时做账号限购校验复用

## Data Flow

### CreateOrder

处理流程：

1. `order-api` 读取登录态用户 `userId`，校验请求基本格式
2. `order-rpc` 调 `program-rpc.GetProgramPreorder` 校验节目和票档是否可下单
3. `order-rpc` 调 `user-rpc` 校验 `ticketUserIds` 是否全部归属当前用户
4. `order-rpc` 校验单笔购票数量不超过 `perOrderLimitPurchaseCount`
5. `order-rpc` 统计当前账号该节目未支付订单张数，并校验不超过 `perAccountLimitPurchaseCount`
6. `order-rpc` 调 `program-rpc.AutoAssignAndFreezeSeats`
7. `order-rpc` 按节目票档价格和购票人数计算订单金额
8. `order-rpc` 基于系统配置计算 `order_expire_time`
9. `order-rpc` 开启事务，写入 `d_order` 和 `d_order_ticket_user`
10. 事务成功后返回 `orderNumber`
11. 若落库失败，补偿调用 `program-rpc.ReleaseSeatFreeze`

### CancelOrder

处理流程：

1. 查询订单是否存在且属于当前用户
2. 校验订单当前状态为未支付
3. 调 `program-rpc.ReleaseSeatFreeze`
4. 本地事务更新主单和明细状态为已取消
5. 写入 `cancel_order_time`
6. 返回成功

### CloseExpiredOrders

处理流程：

1. 定时任务按 `order_status = 1` 且 `order_expire_time <= now` 扫描待关闭订单
2. 按批调用 `order-rpc.CloseExpiredOrders`
3. 对每一笔待关闭订单执行释放冻结
4. 更新订单状态为已取消
5. 记录取消时间

本轮采用定时扫描，不引入延迟消息。

## Business Rules

### Ticket User Validation

- 所有 `ticketUserIds` 必须存在
- 所有 `ticketUserIds` 必须归属当前用户
- 购票人数量必须大于 0

### Purchase Limit Validation

- 本次下单张数不能超过节目单笔限购
- 当前账号该节目未支付订单张数与本次张数之和，不能超过节目账号限购

当前阶段只统计未支付订单，不统计已取消订单。

### Price and Snapshot Rules

- 订单金额必须由服务端根据票档价格和购票人数计算
- 订单关闭时间必须由服务端配置计算并落库
- 节目标题、图片、地点、演出时间必须从 `program-rpc` 获取并落快照
- 购票人姓名、证件号必须从 `user-rpc` 获取并落快照
- 座位信息必须以 `AutoAssignAndFreezeSeats` 的返回值为准

### Compensation Rules

- 如果冻结成功但订单落库失败，必须补偿释放冻结
- 如果订单已取消，再次取消可以返回成功，或返回明确的状态错误；实现阶段需统一一种语义并覆盖测试

## Testing

### order-api

测试重点：

- 请求字段映射
- 登录用户 `userId` 透传
- 响应字段映射
- RPC 错误翻译

建议文件：

- `services/order-api/internal/logic/*_test.go`
- `services/order-api/internal/logic/order_rpc_fake_test.go`

### order-rpc

测试重点：

- 创建订单成功
- 购票人不属于当前用户失败
- 超过单笔限购失败
- 超过账号限购失败
- 座位冻结失败时不落库
- 落库失败时补偿释放冻结
- 取消订单成功并释放冻结
- 已取消订单重复取消的既定语义
- 超时关闭只关闭已到期未支付订单

建议文件：

- `services/order-rpc/internal/logic/create_order_logic_test.go`
- `services/order-rpc/internal/logic/query_order_logic_test.go`
- `services/order-rpc/internal/logic/cancel_order_logic_test.go`
- `services/order-rpc/internal/logic/close_expired_orders_logic_test.go`
- `services/order-rpc/internal/logic/order_test_helpers_test.go`

### Job Layer

测试重点：

- 扫描条件正确
- 批量关闭调用参数正确
- 空批次时安全退出

建议文件：

- `jobs/order-close/internal/logic/*_test.go`

## Manual Verification

建议在 `README.md` 中补充以下手工验证流程：

1. 调用 `/program/preorder/detail` 查看节目和票档
2. 调用 `/order/create` 创建订单
3. 调用 `/order/get` 查看订单快照和座位明细
4. 调用 `/order/select/list` 查看当前用户订单列表
5. 调用 `/order/cancel` 取消未支付订单
6. 再次创建同票档订单，验证已释放的座位可以重新分配

## Open Decisions Locked For Phase 1

以下决策在本轮已固定：

- `order` 采用 `order-api + order-rpc` 两层
- 订单创建由 `order-rpc` 同步编排 `program-rpc` 与 `user-rpc`
- 超时关闭采用定时任务扫描
- 订单创建接口本轮不做幂等
- 只对齐 Java 语义，不复刻 Java 的异步基础设施
