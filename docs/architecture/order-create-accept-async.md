# /order/create Accept + Async 方案

## 背景

现有旧方案把一部分创建结果收口建立在内部超时和异步校正上，例如 `verify_attempt_due`、`VERIFYING`、`commitCutoff`、`userDeadline`、`reconciler`。这类机制在工程上只能做到“尽量触发”，不能作为建单正确性的基础。

本方案的目标是把 `/order/create` 收敛成一个更真实、可实现、可监控的异步受理模型：

- `/order/create` 成功仅表示“请求已受理”
- 最终结果只由事实决定，不由超时决定
- 不再引入结果修正型后台任务

## 核心原则

- Redis 只负责热路径准入、幂等和 attempt 投影
- Kafka 只负责异步建单命令传递
- MySQL 未支付订单事实是 `/order/poll SUCCESS` 的直接依据
- MySQL `d_seat` 冻结态是库存事实
- 明确业务终态失败才返回 `FAILED`
- 临时故障只会让状态停留在 `PROCESSING`，由 consumer retry 继续推进
- 异常通过监控和告警暴露，不通过 `reconciler` 或 `repair job` 判结果
- 本次改造是一次性收口旧实现，不保留新旧状态机并行，不做兼容模式
- DB 是唯一事实源；Redis / Kafka 脏状态可以接受

## 改造约束

本方案落地时，必须遵守以下约束：

- 不是“新增一套新语义，再兼容旧语义”
- 而是“直接替换旧语义，删除旧代码和旧配置”

具体要求：

- 不保留 `VERIFYING / COMMITTED / RELEASED` 的兼容分支
- 不保留 `verify_attempt_due` 的空壳任务或只打日志的残留调用
- 不保留 `commitCutoff / userDeadline / MaxMessageDelay` 这类“配置还在、实现已不用”的半删除状态
- 不保留“新 poll 语义 + 旧 attempt 状态字段”混搭实现
- 不保留“新 consumer 流程 + 旧 reconcile/repair”并行运行

也就是说，代码改造完成后的目标应该是：

- attempt 状态机只有 `ACCEPTED / PROCESSING / SUCCESS / FAILED`
- 创建链路里不存在任何“按时间推断结果”的实现
- 配置、任务、测试、文档全部与新方案一致

## 对外语义

### `/order/create`

成功时只表示：

- purchase token 验签通过
- Redis admission 成功
- attempt 已写入 Redis，并进入受理态

返回值仍然是 `orderNumber`，但这个 `orderNumber` 在此时表示“异步受理单号”，不是“已创建成功的订单”。

### `/order/poll(orderNumber)`

只返回 3 个结果：

- `PROCESSING`：仍在异步处理中
- `SUCCESS`：MySQL 已有未支付订单，可进入支付
- `FAILED`：明确业务终态失败

这里的 `SUCCESS` 仍然不是“已支付成功”，只是“未支付订单已创建完成”。

## Attempt 状态机

attempt 只保留最小状态机：

- `ACCEPTED`
  - `/order/create` 已受理
  - attempt 已写入 Redis
- `PROCESSING`
  - consumer 已开始执行
- `SUCCESS`
  - Redis 冻结成功
  - MySQL `d_seat` 已写冻结态
  - MySQL 未支付订单已落库
- `FAILED`
  - 明确业务终态失败

对前端状态码映射：

- `ACCEPTED` / `PROCESSING` -> `PROCESSING`
- `SUCCESS` -> `SUCCESS`
- `FAILED` -> `FAILED`

不再保留：

- `VERIFYING`
- `COMMITTED`
- `RELEASED`
- 基于 deadline 的中间态

## 主链路

### 1. 预下单页

用户先调用 `/program/preorder/detail(showTimeId)`，program 侧从 MySQL 读取场次、票档和可售座位信息，仅用于展示和基础校验。

### 2. 创建 purchase token

用户调用 `/order/purchase/token`：

- 再次读取 preorder 快照
- 校验当前用户与观演人归属
- 校验单笔限购和账号限购
- 生成带 `orderNumber` 的 `purchaseToken`

这一步不写订单库，不发 Kafka。

### 3. 调用 `/order/create`

订单服务执行：

- 验签 `purchaseToken`
- 进入 Redis admission
- 检查 `user_inflight` / `viewer_inflight`
- 检查 `quota`
- 记录 attempt，状态写为 `ACCEPTED`
- 尝试发送 Kafka `ticketing.attempt.command.v1`

admission 成功后即可返回 `orderNumber`。

如果 Kafka 发送失败：

- 不回滚 admission
- 不判失败
- 对用户仍然显示“已受理 / 处理中”
- 记录日志和监控

### 4. Kafka consumer 异步建单

consumer 收到消息后：

- 将 attempt 从 `ACCEPTED` 推进到 `PROCESSING`
- 读取 preorder、观演人快照等必要事实
- 调用 `program-rpc.AutoAssignAndFreezeSeats` 自动排座并冻结
- 将命中的 `d_seat` 写为冻结态（`seat_status = 2`）
- 组装订单写模型
- 开启 MySQL 事务写入：
  - `d_order_xx`
  - `d_order_ticket_user_xx`
  - `d_order_user_guard`
  - `d_order_viewer_guard`
  - `d_order_seat_guard`
  - `d_order_outbox(order.created)`

事务成功后：

- attempt 更新为 `SUCCESS`
- 删除 inflight 占位
- 登记 `close_timeout`

### 5. `/order/poll`

`/order/poll(orderNumber)` 的推荐实现：

- 先读 Redis attempt
- 如果 attempt 已是 `SUCCESS` 或 `FAILED`，直接返回
- 如果 attempt 仍是 `ACCEPTED` 或 `PROCESSING`，再按 `orderNumber` 查询 MySQL
- 若 MySQL 已存在未支付订单，则直接返回 `SUCCESS`

这里的 DB fallback 是读路径兜底，不是后台 repair job。它的作用只是避免“订单事实已经存在，但 Redis 投影偶发滞后”时用户一直看到 `PROCESSING`。

若 15 秒轮询窗口内仍未出结果，前端也只能展示“已受理，结果仍在处理中”，不能把 15 秒当成失败线。

### 6. `/order/pay`

支付阶段保持现有语义：

- `pay-rpc.MockPay` 写支付单
- `program-rpc.ConfirmSeatFreeze` 将 Redis `frozen` 转为 `sold`
- MySQL `d_seat` 从冻结态改为已售态
- 订单状态推进为已支付

### 7. 取消与超时关单

取消和 `close_timeout` 保持现有语义：

- 释放 Redis freeze
- MySQL `d_seat` 从冻结态恢复为可售态
- 更新订单和订单明细为取消态
- 删除 guard
- 写 `d_order_outbox(order.closed)`

这属于订单生命周期管理，不属于建单结果判定。

## Redis / Kafka / MySQL 职责划分

### Redis

只承担以下职责：

- admission 准入
- inflight 并发互斥
- token 幂等指纹
- attempt 投影

Redis 不再承担：

- 基于超时推断建单失败
- 基于 verify/reconcile 补判最终结果

### Kafka

Kafka 只承担异步命令传递：

- topic 仍为 `ticketing.attempt.command.v1`
- 分区 key 仍建议使用 `<showTimeId>#<ticketCategoryId>`
- 消息晚到不直接判失败

也就是说，不再保留“消息 age 超过阈值就直接释放 attempt”的语义。

### MySQL

MySQL 里的未支付订单事实，是 `/order/poll SUCCESS` 的直接依据：

- 只要订单事务已经提交成功，就应视为创建成功
- 不能因为某个超时点已到而把它重新判成失败

与此同时：

- MySQL `d_seat` 的冻结态承担库存事实
- 支付时再从冻结态推进到已售态
- 取消或超时关单时再从冻结态恢复为可售态

## Attempt 字段建议

attempt hash 建议仅保留与受理和结果有关的字段，例如：

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
- `processing_epoch`
- `freeze_token`
- `created_at`
- `accepted_at`
- `processing_started_at`
- `finished_at`
- `last_error_code`
- `last_error_at`

建议删除只服务于旧超时状态机的字段，例如：

- `commit_cutoff_at`
- `user_deadline_at`
- `verify_started_at`
- 以及所有仅为 verify/reconcile 服务的调度字段

## 短锁与重新参与

`user_inflight / viewer_inflight` 只是短锁，不是最终资格锁。

因此：

- inflight TTL 到期后，允许用户再次参与
- 最终唯一性由 DB guard 保证
- 如果重复请求中的后到单因为 guard 冲突失败，用户提示应为：
  - 你已有一笔订单创建成功，请到订单列表查看

## 失败语义

### 明确业务终态失败

以下情况可直接把 attempt 判为 `FAILED`：

- 真实余票不足
- 观演人冲突
- 场次关闭
- 票档不可售
- 明确不可恢复的冻座失败

### 临时失败

以下情况不应直接判 `FAILED`：

- Kafka 消费延迟
- program-rpc / user-rpc 临时超时
- MySQL 短暂故障
- Redis 短暂故障

这类情况只会让 attempt 保持 `PROCESSING`，依赖消息重试或 consumer retry 继续推进。

## 删除项

本方案明确删除以下机制：

- `verify_attempt_due`
- `VERIFYING`
- `commitCutoff`
- `userDeadline`
- `reconciler`
- `repair job`
- `MaxMessageDelay` 驱动的结果判定

这些删除项的要求是“代码和配置一起删掉”，而不是仅停止调用。

## 保留项

本方案仍保留以下必要异步机制：

- `close_timeout`
- freeze expiry / release
- outbox publisher

这三类任务分别服务于订单生命周期、资源回收和事件投递，不参与建单结果判定。

## 异常暴露方式

不再引入额外 repair job 后，异常主要通过现有可观测能力暴露：

- Kafka lag
- consumer 错误日志
- RPC / MySQL / Redis 错误率与延迟
- 当前 `PROCESSING` 数量和积压趋势
- `SUCCESS / FAILED` 转化率异常

也就是说，异常靠监控发现，必要时人工介入；而不是靠后台扫 attempt 来“重新判案”。

## 迁移建议

迁移时建议按以下顺序推进：

1. 先更新文档和接口语义，明确 `/order/create = accepted`
2. 收敛 attempt 状态机为 `ACCEPTED / PROCESSING / SUCCESS / FAILED`
3. 改造 `/order/create`，要求 Kafka 命令 durable 成功后才返回成功
4. 删除 `verify_attempt_due` 任务、RPC、调用点和相关测试
5. 删除 `commitCutoff`、`userDeadline`、`VERIFYING` 相关字段、配置和实现
6. 删除 `reconciler` / `repair job` 相关代码与说明
7. 将 `/order/poll` 收敛为 Redis + MySQL fallback 的只读查询
8. 删除旧状态机语义下的测试，改为新语义测试，不做双语义兼容

迁移完成后的验收标准不是“新方案能跑”，而是：

- 旧方案入口和旧状态语义已经不存在
- 代码库中不再残留旧机制的有效调用链
- 配置文件、测试断言、架构文档全部只表达新方案

## 总结

本方案的核心变化只有一句话：

- 旧方案是“时间到了，系统推断结果”
- 新方案是“事实发生了，系统展示结果”

这样可以让 `/order/create` 的异步模型和分布式系统的真实能力保持一致，也能显著降低文档、状态机和实现复杂度。
