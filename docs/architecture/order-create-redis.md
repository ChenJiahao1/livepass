# 抢票 V1 Redis / Kafka / DB 收口说明

本文记录当前 `purchaseToken -> create -> poll -> verify_attempt_due -> reconcile` 抢票链路里的真实口径，覆盖 Redis、Kafka、DB guard 和终裁职责边界。

## 1. 主链路

1. `program-api /program/preorder/detail(showTimeId)` 返回真实场次视图和票档余量。
2. `order-api /order/purchase/token(showTimeId, ticketCategoryId, ticketUserIds)` 生成绑定 `show_time_id`、`generation` 和观演人的购买令牌。
3. `order-api /order/create(purchaseToken)` 只做 admission、登记延迟校验任务、异步 handoff Kafka，不同步落库。
4. `order-rpc` consumer 读取 Kafka 命令后补齐节目/观演人快照、冻座、落库、写 outbox、登记 close timeout。
5. `order-api /order/poll(orderNumber)` 只读 Redis attempt 投影，不查 DB、不推进状态。
6. `verify_attempt_due` 在用户 deadline 后按 DB 事实终裁，`reconcile` 只做窗口后兜底和 closed-order projection 修复。

## 2. Redis 关键键

所有抢票键都按 `show_time_id + generation` 隔离，并共用同一 hash tag：

```text
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:attempt:<order_number>
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:user_inflight:<user_id>
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:viewer_inflight:<viewer_id>
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:user_active:<user_id>
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:viewer_active:<viewer_id>
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:quota:<ticket_category_id>
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:progress_index
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:seat_occupied:<order_number>
damai-go:program:seat-ledger:{st:<show_time_id>:g:<generation>}:stock:<ticket_category_id>
damai-go:program:seat-ledger:{st:<show_time_id>:g:<generation>}:available:<ticket_category_id>
damai-go:program:seat-ledger:{st:<show_time_id>:g:<generation>}:sold:<ticket_category_id>
damai-go:program:seat-ledger:{st:<show_time_id>:g:<generation>}:frozen:<ticket_category_id>:<freeze_token>
```

### 2.1 `attempt_record`

- 类型：`Hash`
- 核心字段：`order_number / user_id / show_time_id / program_id / ticket_category_id / viewer_ids / ticket_count / generation / sale_window_end_at / show_end_at / commit_cutoff_at / user_deadline_at / state / reason_code / processing_epoch / next_db_probe_at / db_probe_attempts`
- 作用：
  - 记录 admission 成功后的最小抢票事实
  - 为 `/order/poll`、`verify_attempt_due` 和 `reconcile` 提供统一状态源

### 2.2 inflight / active guard

- `user_inflight`、`viewer_inflight`：表示当前仍在处理中的同场次占位
- `user_active`、`viewer_active`：表示已经成功提交的同场次有效持有态
- `quota`：按 `show_time_id + ticket_category_id` 的热路径 admission 余量

### 2.3 seat ledger

- `stock` / `available` / `sold` / `frozen`
- 节目域冻座、确认、释放、回售全部以 `show_time_id` 为真实作用域
- `ReleaseSoldSeats` 在秒杀窗口内直接拒绝，避免退款回售污染窗口

## 3. Kafka 口径

- Topic：`ticketing.attempt.command.v1`
- Consumer Group：`damai-go-ticketing-attempt`
- Partition Key：`<show_time_id>#<ticket_category_id>`
- 消息体核心字段：
  - `orderNumber`
  - `userId`
  - `programId`
  - `showTimeId`
  - `ticketCategoryId`
  - `ticketUserIds`
  - `ticketCount`
  - `generation`
  - `saleWindowEndAt`
  - `showEndAt`
  - `commitCutoffAt`
  - `userDeadlineAt`
  - `occurredAt`

当前 `CreateOrder` 请求线程只要求：

- admission 成功
- verify_attempt_due 任务登记成功或至少尽力登记
- Kafka handoff 发起成功或记录失败日志

不会再同步等待 Kafka `Send()` 成功后才返回。

## 4. DB guard 与 outbox

订单域所有唯一性和补偿事实都改到 `show_time_id`：

```text
d_order_xx.show_time_id
d_order_ticket_user_xx.show_time_id
d_order_user_guard.uk_show_time_user(show_time_id, user_id)
d_order_viewer_guard.uk_show_time_viewer(show_time_id, viewer_id)
d_order_seat_guard.uk_show_time_seat(show_time_id, seat_id)
d_order_outbox.show_time_id
```

说明：

- Redis guard 负责热路径 admission
- MySQL guard 负责最终唯一性兜底
- `order.closed`、`order.refunded` 等 outbox 事件都携带 `show_time_id`

## 5. poll / verify / reconcile 职责边界

### 5.1 `/order/poll`

- 只读 Redis
- 绝不查 DB
- 绝不推进状态
- 用户一旦看到 `FAILED`，后续对外语义不可逆

### 5.2 `verify_attempt_due`

- 唯一 DB 终裁 owner
- 输入只需要 `orderNumber`
- 规则：
  - DB 查到有效订单：`COMMITTED`
  - DB 查到已取消订单：修 closed-order projection
  - worker 已明确失败且超过 cutoff：`RELEASED`
  - DB 暂不可判定：`VERIFYING` 并按退避继续探测
  - 用户已见 `FAILED` 后晚到成功：自动关单并告警，不翻回 `SUCCESS`

### 5.3 `reconcile`

- 只承担窗口后批量兜底
- 扫描 `PENDING/QUEUED/PROCESSING/VERIFYING` 的长期未终态 attempt
- 修复 `COMMITTED` 订单被关闭后的 Redis 投影缺口
- 不与 `verify_attempt_due` 抢 DB 终裁职责

## 6. 退款边界

- 退款窗口判定统一读取 `d_program_show_time.rush_sale_open_time / rush_sale_end_time`
- `EvaluateRefundRuleReq` 以 `showTimeId + orderAmount` 为唯一输入口径
- 秒杀窗口内：
  - `PreviewRefundOrder`
  - `RefundOrder`
  - `GetOrderServiceView.canRefund`
  - MCP `preview_refund_order / refund_order`
  - `program-rpc.ReleaseSoldSeats`
  都返回统一拒绝文案：`秒杀活动进行中，暂不支持退票`
- 命中禁退时不会触发：
  - `PayRpc.Refund`
  - `ProgramRpc.ReleaseSoldSeats`
  - 订单退款状态更新
  - 退款 outbox
