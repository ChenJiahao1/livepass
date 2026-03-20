# Program Seat Inventory Design

**Date:** 2026-03-20

## Goal

在 `damai-go` 中为 `program` 域补齐第一版座位级库存内核，优先支持内部调用场景下的系统自动分配座位、冻结座位和释放冻结座位。

本轮采用简单实现：

- 只使用 MySQL 作为库存真相源
- 不引入 Redis 库存热状态
- 不实现前端手动选座
- 不实现成交确认售出
- 不实现后台录入接口

## Scope

本轮覆盖：

- 基于 `showTimeId + ticketCategoryId` 的座位级库存模型
- 系统自动分配座位
- 冻结座位
- 释放冻结座位
- `requestNo` 幂等
- `freezeToken` 幂等
- 懒回收过期冻结
- `program-rpc` 内部 RPC 契约

本轮不覆盖：

- `program-api` 对外座位接口
- 前端手动选座
- 冻结后确认售出
- 定时任务全量过期扫描
- Redis 缓存和 Lua 库存脚本
- 座位模板录入和场次写接口

## Architecture

### Service Boundary

本轮只扩展 `program-rpc`，不新增 `program-api` 的展示或写接口。

继续遵循当前仓库的 go-zero 分层：

- `handler` 不参与本轮
- `logic` 负责库存规则、幂等和事务编排
- `model` 负责座位和冻结记录查询更新
- `svc` 负责注入模型和 MySQL 连接

### Inventory Truth Source

座位库存真相源统一放在 MySQL：

- `d_show_time_seat` 存放场次座位快照和当前座位状态
- `d_show_time_seat_freeze` 存放冻结操作记录

`d_ticket_category.remain_number` 本轮不作为 seat-level 库存真相源，只保留给现有只读接口使用。后续如要对齐 seat-level 展示，再统一设计汇总策略。

## Data Model

### d_show_time_seat

表示某一场次的座位快照。

建议字段：

- `id`
- `show_time_id`
- `program_id`
- `ticket_category_id`
- `row_code`
- `col_code`
- `seat_type`
- `price`
- `seat_status`
- `freeze_token`
- `freeze_expire_time`
- `create_time`
- `edit_time`
- `status`

状态约束：

- `seat_status = 1` 表示 `available`
- `seat_status = 2` 表示 `frozen`
- `seat_status = 3` 表示 `sold`

本轮虽然不实现售出确认，但先保留 `sold` 状态，避免后续改表。

索引建议：

- 唯一索引：`uk_show_time_row_col(show_time_id, row_code, col_code)`
- 普通索引：`idx_show_time_ticket_status(show_time_id, ticket_category_id, seat_status)`
- 普通索引：`idx_freeze_token(freeze_token)`

### d_show_time_seat_freeze

表示一次冻结操作本身，作为幂等和释放依据。

建议字段：

- `id`
- `freeze_token`
- `request_no`
- `show_time_id`
- `program_id`
- `ticket_category_id`
- `seat_count`
- `freeze_status`
- `expire_time`
- `release_reason`
- `release_time`
- `create_time`
- `edit_time`
- `status`

状态约束：

- `freeze_status = 1` 表示 `frozen`
- `freeze_status = 2` 表示 `released`
- `freeze_status = 3` 表示 `expired`

索引建议：

- 唯一索引：`uk_request_no(request_no)`
- 唯一索引：`uk_freeze_token(freeze_token)`
- 普通索引：`idx_show_time_ticket_status(show_time_id, ticket_category_id, freeze_status)`

### Simplifications

本轮刻意不新增：

- 座位模板表
- 冻结明细表

冻结到的具体座位通过 `d_show_time_seat.freeze_token` 反查，先满足最小实现。后续如果需要审计明细或支持更复杂补偿，再补冻结明细表。

## RPC Contract

本轮建议在 `program-rpc` 新增 2 个内部方法。

### AutoAssignAndFreezeSeats

用途：

- 按场次和票档自动分配座位
- 冻结分配结果

请求字段：

- `showTimeId`
- `ticketCategoryId`
- `count`
- `requestNo`
- `freezeSeconds`

响应字段：

- `freezeToken`
- `expireTime`
- `seats`

`seats` 至少包含：

- `seatId`
- `rowCode`
- `colCode`
- `ticketCategoryId`
- `price`

### ReleaseSeatFreeze

用途：

- 释放指定冻结单占用的座位

请求字段：

- `freezeToken`
- `releaseReason`

响应字段：

- `success`

## Data Flow

### AutoAssignAndFreezeSeats

处理流程：

1. 开启事务
2. 校验 `showTimeId` 是否存在
3. 校验 `ticketCategoryId` 是否存在且属于该场次对应节目
4. 在事务内回收当前 `showTimeId + ticketCategoryId` 下已过期冻结
5. 查询当前可售座位并加行锁
6. 执行系统自动分配
7. 若可分配座位不足，返回业务错误
8. 生成 `freezeToken`
9. 写入冻结记录
10. 更新座位状态为 `frozen` 并写入冻结信息
11. 提交事务
12. 返回冻结结果

### ReleaseSeatFreeze

处理流程：

1. 开启事务
2. 按 `freezeToken` 查询冻结记录
3. 不存在则返回冻结记录不存在
4. 若已 `released`，直接幂等成功
5. 若已 `expired`，直接幂等成功
6. 将该 `freezeToken` 对应座位恢复为 `available`
7. 清空座位上的冻结字段
8. 更新冻结记录为 `released`
9. 提交事务
10. 返回成功

## Seat Assignment Strategy

系统自动分配逻辑抽成独立 helper，保证逻辑层和模型层边界清晰。

第一版策略：

1. 优先同排连续座位
2. 找不到同排连续时，按行优先顺序补足前 `N` 个座位
3. 若总量不足，则返回库存不足错误

这轮不实现：

- 复杂排座偏好
- 隔座策略
- 最优视野策略
- 多票档混合分配

## Concurrency Control

本轮只使用 MySQL 事务和行锁，不引入 Redis 锁、本地锁或分布式锁。

并发规则：

- 粒度为 `show_time_id + ticket_category_id`
- 在事务内使用 `SELECT ... FOR UPDATE` 锁定当前票档当前场次的候选座位
- 同一场次同一票档下的并发冻结请求串行执行

这样虽然吞吐一般，但实现最简单、最稳，适合作为首版 seat-level 库存内核。

## Idempotency

### AutoAssignAndFreezeSeats

使用 `request_no` 保证幂等。

规则：

- `request_no` 全局唯一
- 若同一 `request_no` 已存在且状态仍为 `frozen`，直接返回原冻结结果
- 若同一 `request_no` 已存在且状态为 `released` 或 `expired`，返回业务错误，要求调用方改用新的 `request_no`

### ReleaseSeatFreeze

使用 `freeze_token` 保证幂等。

规则：

- 若冻结记录已是 `released`，直接返回成功
- 若冻结记录已是 `expired`，直接返回成功
- 只有记录不存在时才返回错误

## Expiration and Recovery

本轮采用懒回收，不做定时任务。

规则：

- 每次执行 `AutoAssignAndFreezeSeats` 前，先回收当前 `showTimeId + ticketCategoryId` 下已过期冻结
- 每次执行 `ReleaseSeatFreeze` 时，也校验冻结单是否已经过期

过期回收动作：

- `d_show_time_seat_freeze.freeze_status` 更新为 `expired`
- 对应座位从 `frozen` 恢复为 `available`
- 清空 `freeze_token` 和 `freeze_expire_time`

后续如果需要更强回收及时性，再增加 job 扫描全库过期冻结。

## Error Handling

建议补充 `pkg/xerr` 领域错误：

- `ErrProgramShowTimeNotFound`
- `ErrProgramTicketCategoryNotFound`
- `ErrSeatInventoryInsufficient`
- `ErrSeatFreezeNotFound`
- `ErrSeatFreezeExpired`
- `ErrSeatFreezeRequestConflict`

`program-rpc` 对外返回 gRPC 语义时：

- 参数问题返回 `InvalidArgument`
- 资源不存在返回 `NotFound`
- 库存不足、幂等冲突等业务冲突返回 `FailedPrecondition` 或 `AlreadyExists`
- 未知问题返回 `Internal`

## Testing Strategy

### Helper Unit Tests

覆盖座位分配 helper：

- 同排连续优先
- 同排不足时按行优先补足
- 座位不足报错
- 非法数量报错

### RPC Logic Integration Tests

延续现有 `program-rpc` 测试风格，使用本地 MySQL 测试库。

覆盖场景：

- 自动分配并冻结成功
- 重复 `requestNo` 幂等返回同一结果
- 座位不足失败
- 释放冻结成功
- 重复释放幂等成功
- 冻结过期后再次分配会自动回收并成功分配

### Concurrency Test

对同一个 `showTimeId + ticketCategoryId` 发起并发冻结请求，断言：

- 不会分配到重复座位
- 成功请求返回的冻结座位互不重叠
- 冻结总数不超过可售总数

## Completion Criteria

本轮完成标准：

- 新增场次座位快照表和冻结记录表
- `program-rpc` 提供自动分配冻结和释放冻结两个内部 RPC
- MySQL 事务和行锁可以保证同票档同场次串行分配
- 支持 `requestNo` 和 `freezeToken` 幂等
- 支持懒回收过期冻结
- `go test ./services/program-rpc/...` 通过

## Follow-up Work

后续按顺序推进：

1. 补场次写接口和座位模板能力
2. 增加冻结确认售出
3. 让 `ticket_category` 展示余量与 seat-level 真相统一
4. 评估 Redis 热状态投影和高并发优化
5. 再决定是否开放 `program-api` 座位相关接口
