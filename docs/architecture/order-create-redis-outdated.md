# [过时] /order/create Redis 异步建单方案

> 本文档保留为旧版实现说明，不再作为当前推荐方案。
> 当前推荐方案见 `docs/architecture/order-create-accept-async.md`。

/order/create 只做 Redis 准入和 Kafka 异步投递，先返回 orderNumber；Kafka consumer 再去冻座并把“未支付订单”落到 MySQL；/order/poll 返回 SUCCESS 代表“订单已创建，可支付”，不是“已支付”；真正把座位从
冻结态写成已售，是 /order/pay。下面我按实现来讲，不按设计稿脑补；seat-ledger 的 key 顺序我也按代码实际值写。注意 program 侧 scope tag 固定由 `seatLedgerScopeTag(showTimeID)` 生成，实际长这样：
`{st:<show_time_id>:g:g-<show_time_id>}`；前一个 `g` 是 tag 里的字段名，后一个 `g-<show_time_id>` 才是 generation 值，所以看起来像重复，其实不是写错。主要代码入口在
services/order-rpc/internal/logic/create_purchase_token_logic.go:33、
services/order-rpc/internal/logic/create_order_logic.go:31、services/order-rpc/internal/rush/admit_attempt.lua:1、services/order-rpc/internal/logic/create_order_consumer_logic.go:38、services/
program-rpc/internal/logic/auto_assign_and_freeze_seats_logic.go:43、services/order-rpc/internal/logic/rush_attempt_projection_helper.go:98、services/order-rpc/internal/logic/
pay_order_logic.go:31。

主链路

1. 用户先看 /program/preorder/detail(showTimeId)。这里走 program-rpc.GetProgramPreorder，直接查 MySQL 的 d_program_show_time、d_program、d_ticket_category、d_seat，把场次、票档和当前可售座位数算出
    来。这个步骤不写 Kafka，也不写订单侧 Redis。services/program-rpc/internal/logic/get_program_preorder_logic.go:24
2. 用户调 /order/purchase/token。订单服务会再读一次 preorder，读 user-rpc 做观演人归属校验，再用 MySQL CountActiveTicketsByUserShowTime(userId, showTimeId) 校验账号限购。这里唯一会写 Redis 的地方，
    是懒初始化 admission 配额：如果 quota key 还不存在，就 SETNX 成当前票档 remainNumber。然后生成一个绑定 orderNumber/userId/showTimeId/ticketCategoryId/ticketUserIds/generation/tokenFingerprint 的
    purchaseToken。此时还没有订单落库，也没有 Kafka 消息。
3. 用户调 /order/create。这一步是纯热路径：先验签 purchaseToken，再进 Admit Lua 做原子准入。Lua 会先查 user_active、viewer_active、user_inflight、viewer_inflight，再查 quota 是否足够；足够则先
    DECRBY quota ticketCount，再写 attempt_record，状态置成 PENDING_PUBLISH，同时写 inflight 占位和 fingerprint 索引。随后服务会登记一个异步 verify_attempt_due 延迟任务，再往 Kafka topic
    ticketing.attempt.command.v1 发一条“创建抢票 attempt”消息，最后把 attempt 标成 QUEUED 并把 orderNumber 直接返回给前端。也就是说，前端此时只拿到了一个“排队中的订单号”。
4. Kafka consumer 消费到这条消息后，先把 attempt 从 PENDING_PUBLISH/QUEUED 抢成 PROCESSING，并给 processing_epoch 加一。然后它会再读 preorder、再读 ticketUser 列表，并调用 program-
    rpc.AutoAssignAndFreezeSeats 真正做系统排座。这里的冻座发生在节目域 Redis seat-ledger，不在 MySQL：Lua 会从 available zset 里优先挑连坐，不够再顺序补齐，把座位从 available 挪到
    frozen:<freezeToken>，同时把 stock 字符串库存值减掉。随后再写一份 freeze metadata，记录 freezeToken/requestNo/showTimeId/ticketCategoryId/ownerOrderNumber/ownerEpoch/expireAt。这一阶段 MySQL
    的 d_seat 还不变，冻结态只存在于 Redis。
5. 冻座成功后，consumer 在订单域开启 MySQL 事务落库，写六类事实：d_order_xx 主单、d_order_ticket_user_xx 明细、d_order_user_guard、d_order_viewer_guard、d_order_seat_guard、
    d_order_outbox(order.created)。事务成功后，再把 Redis attempt 投影改成 COMMITTED，删除 inflight，占上 user_active/viewer_active，并登记一个 close_timeout 延迟任务，等订单过期时自动关单。到这里 /
    order/poll 才会看到 SUCCESS。注意，这个 SUCCESS 只是“抢到了一笔未支付订单”。
6. /order/poll(orderNumber) 完全只读 Redis attempt，不查 MySQL、不推进状态。映射关系是：PENDING_PUBLISH/QUEUED/PROCESSING 在 deadline 前返回 PROCESSING(1)；超过用户 deadline 但还没终态，返回
    VERIFYING(2)；COMMITTED 返回 SUCCESS(3)；RELEASED 返回 FAILED(4)。
7. 如果用户 deadline 到了还没终态，异步 verify_attempt_due 会以 orderNumber 为键查 MySQL 主事实。如果 DB 已经有订单，就把 Redis 补成 COMMITTED；如果订单已经取消，就把 Redis 收口成 RELEASED/
    CLOSED_ORDER_RELEASED；如果 DB 还没有订单、且已经过了 commit cutoff，就把 attempt 释放掉并把 quota 加回去；如果还不到 cutoff，就先标 VERIFYING，然后重新投递下一次 verify_attempt_due 延迟任务，
    等后续重试。reconcile 做的是同一套逻辑的批量兜底，不负责新的事实判断。
8. 用户支付时调 /order/pay。订单服务先走 pay-rpc.MockPay 写一条 d_pay_bill，然后调 program-rpc.ConfirmSeatFreeze。这一步才会把节目域 Redis 里的 frozen 挪到 sold，并把 MySQL d_seat 从 seat_status=1
    直接更新成 seat_status=3。最后订单域再把 d_order_xx 和 d_order_ticket_user_xx 的状态更新成已支付。也就是说，当前实现里 MySQL 的座位“已售事实”是在支付时才落地，不是在下单时。
9. 如果用户取消，或者 close_timeout 到期自动关单，订单服务会先调用 ProgramRpc.ReleaseSeatFreeze 把 Redis 里的冻结座位退回 available，再在 MySQL 里把 d_order_xx、d_order_ticket_user_xx 改成取消态，
    删除三张 guard 表的记录，并写 d_order_outbox(order.closed)。最后再把 Redis attempt 收口成 RELEASED，同时把 quota 补回去。

Redis / Kafka / MySQL 对照
order-rush Redis key：

damai-go:order:rush:{st:<show_time_id>:g:<generation>}:attempt:<order_number>
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:user_inflight:<user_id>
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:viewer_inflight:<viewer_id>
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:user_active:<user_id>
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:viewer_active:<viewer_id>
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:quota:<ticket_category_id>
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:fingerprint:<user_id>
damai-go:order:rush:{st:<show_time_id>:g:<generation>}:seat_occupied:<order_number>

- attempt:<orderNo> 是 Hash，value 不是一个整体 JSON，而是一组 field -> string 值。
  示例：
  order_number="202604070001"
  user_id="10001"
  program_id="20001"
  show_time_id="30001"
  ticket_category_id="40001"
  viewer_ids="50001,50002"
  ticket_count="2"
  generation="g-30001"
  sale_window_end_at="1775551200000"
  show_end_at="1775558400000"
  token_fingerprint="4d0a...abc"
  state="PENDING_PUBLISH|QUEUED|PROCESSING|VERIFYING|COMMITTED|RELEASED"
  reason_code=""
  commit_cutoff_at="1775551110000"
  user_deadline_at="1775551115000"
  processing_epoch="0|1|2..."
  processing_started_at="1775551105231"
  verify_started_at="1775551115320"
  created_at="1775551105000"
  last_transition_at="1775551105400"
  说明：
  orderNumber、userId、showTimeId、ticketCategoryId、ticketCount、processingEpoch 都按十进制字符串存。
  时间字段全部按 Unix 毫秒时间戳字符串存，不是 datetime 字符串。
  viewer_ids 是逗号分隔字符串。
  state/reason_code 是状态机枚举值。
- user_inflight:<userId>、viewer_inflight:<viewerId> 是 String。
  示例 value：`202604070001`
  含义：这个用户/观演人在该场次当前有一个处理中 attempt。
  TTL：默认 30 秒。
- user_active:<userId>、viewer_active:<viewerId> 是 String。
  示例 value：`202604070001`
  含义：这个用户/观演人在该场次当前已经对应到一笔有效订单投影。
- quota:<ticketCategoryId> 是 String。
  示例 value：`23`
  含义：热路径 admission 余量，不是 seat-ledger 真实剩余座位数。
  变化：
  /order/create admission 成功时 `DECRBY ticketCount`
  RELEASED/CLOSED_ORDER_RELEASED 收口时 `INCRBY ticketCount`
- fingerprint:<userId> 是 Hash。
  示例：
  field=`4d0a...abc`
  value=`202604070001`
  含义：同一 user 在同一场次同一 tokenFingerprint 的幂等复用索引。
- seat_occupied:<orderNo> 设计上是 Set<seatId>。
  示例 members：`88001`、`88002`
  含义：这个订单占到的 seatId 集合。
  现状：
  当前首次正常消费时，这个集合大概率为空，因为 commit 投影时传入的是原始 Kafka 消息里的 seatSnapshot，而原始 create 消息并不带 seatSnapshot。services/
  order-rpc/internal/logic/create_order_consumer_logic.go:143 services/order-rpc/internal/logic/order_create_event_builder.go:178

program seat-ledger Redis key：

其中 `<scope_tag>` 的实际值是 `{st:<show_time_id>:g:g-<show_time_id>}`。

damai-go:program:seat-ledger:stock:<scope_tag>:<ticket_category_id>
damai-go:program:seat-ledger:available:<scope_tag>:<ticket_category_id>
damai-go:program:seat-ledger:sold:<scope_tag>:<ticket_category_id>
damai-go:program:seat-ledger:frozen:<scope_tag>:<ticket_category_id>:<freeze_token>
damai-go:program:seat-ledger:loading:<scope_tag>:<ticket_category_id>
damai-go:program:seat-ledger:freeze:meta:<freeze_token>
damai-go:program:seat-ledger:freeze:req:<request_no>
damai-go:program:seat-ledger:freeze:index:<scope_tag>:<ticket_category_id>

- stock 是 String。
  示例 value：`128`
  含义：该 showTime + ticketCategory 当前真实可冻结座位数。
- available 是 ZSET。
  示例 member：`88001|40001|12|8|380`
  示例 score：`12000008`
  含义：当前可分配座位列表。
  member 编码规则：
  seatId|ticketCategoryId|rowCode|colCode|price
  score 规则：
  rowCode * 1000000 + colCode
- sold 是 ZSET。
  value 结构和 available 一样。
  示例 member：`88001|40001|12|8|380`
  示例 score：`12000008`
  含义：已经确认售出的座位列表。
- frozen:<freezeToken> 是 ZSET。
  value 结构和 available 一样。
  示例 key：
  damai-go:program:seat-ledger:frozen:{st:30001:g:g-30001}:40001:freeze-90001
  示例 members：
  `88001|40001|12|8|380`
  `88002|40001|12|9|380`
  含义：某一次冻结拿到的座位集合。
- loading 是 String。
  示例 value：`1`
  含义：seat ledger 正在从 MySQL 回灌 Redis，防止短时间内重复触发加载。
  TTL：默认 3 秒冷却。
- freeze:meta:<freezeToken> 是 String(JSON)。
  示例 value：
  {"freezeToken":"freeze-90001","requestNo":"202604070001-1","programId":20001,"showTimeId":30001,"ticketCategoryId":40001,"ownerOrderNumber":202604070001,"ownerEpoch":1,"seatCount":2,"freezeStatus":1,"expireAt":1775552000,"updatedAt":1775551105}
  字段说明：
  freezeStatus:
  1=Frozen
  2=Released
  3=Expired
  4=Confirmed
  ownerOrderNumber + ownerEpoch 用来做 fencing，避免旧 consumer/旧请求误释放或误确认。
- freeze:req:<requestNo> 是 String。
  示例 value：`freeze-90001`
  含义：按 requestNo 查回本次冻结 token，做冻座幂等。
- freeze:index 是 ZSET。
  示例：
  member=`freeze-90001`
  score=`1775552000`
  含义：按过期时间索引当前仍处于 frozen 状态的 freezeToken，供过期回收扫描。
- 这里的 stock 字符串值是“真实排座可冻结数”；和订单侧 quota 是两套口径，允许短时不一致。

Kafka：

- 当前抢票主链路只看到一个 topic：ticketing.attempt.command.v1，consumer group 是 damai-go-ticketing-attempt。services/order-rpc/internal/mq/topics.go:5
- 分区 key 是 <showTimeId>#<ticketCategoryId>。services/order-rpc/internal/event/order_create_event.go:68
- 消息体字段定义在 OrderCreateEvent，但 /order/create 真正发出去的是“最小消息”，只带 orderNumber/userId/programId/showTimeId/ticketCategoryId/ticketUserIds/ticketCount/generation/distributionMode/
  takeTicketMode/saleWindowEndAt/showEndAt/commitCutoffAt/userDeadlineAt/occurredAt，初始不带 freezeToken/programSnapshot/ticketUserSnapshot/seatSnapshot。services/order-rpc/internal/event/
  order_create_event.go:10 services/order-rpc/internal/logic/order_create_event_builder.go:153
- order.created、order.closed 这些并没有在当前代码里继续发 Kafka；它们只是写进 MySQL 的 d_order_outbox，等待后续独立 outbox publisher 去发，目前这段代码里我没看到 publisher。

MySQL：

- 节目域读表：d_program_show_time、d_program、d_ticket_category、d_seat。用途是 preorder 展示、票档校验、场次校验、排座和支付确认。
- 订单创建事务写表：d_order_xx、d_order_ticket_user_xx、d_order_user_guard、d_order_viewer_guard、d_order_seat_guard、d_order_outbox(eventType=order.created)。sql/order/sharding/d_order_shards.sql
  sql/order/sharding/d_order_ticket_user_shards.sql sql/order/sharding/d_order_user_guard.sql sql/order/sharding/d_order_viewer_guard.sql sql/order/sharding/d_order_seat_guard.sql sql/order/
  sharding/d_order_outbox.sql
- 订单取消/过期事务会更新 d_order_xx、d_order_ticket_user_xx，删除三张 guard 表对应记录，并再写一条 d_order_outbox(eventType=order.closed)。services/order-rpc/internal/logic/
  order_domain_helper.go:246
- 支付时 pay-rpc.MockPay 会写 d_pay_bill；随后 PayOrder 更新 d_order_xx/d_order_ticket_user_xx 为已支付。services/pay-rpc/internal/logic/mock_pay_logic.go sql/pay/d_pay_bill.sql
- 当前实现里 d_seat 在下单阶段不写冻结态；支付确认时才直接把命中的座位从 seat_status=1 改成 seat_status=3。services/program-rpc/internal/model/d_seat_model.go:293

几个当前实现里最关键的点

- poll SUCCESS 的含义是“Redis attempt 已 COMMITTED，MySQL 已有未支付订单”，不是“用户已支付成功”。
- 当前代码里，座位冻结是 Redis-only；MySQL d_seat 直到支付确认才写成已售。所以未支付取消时不会去改 d_seat，只会释放 Redis 冻结。
- quota 和 seat-ledger.stock 是双轨模型。前者只负责热路径 admission，后者才负责真实排座冻结；两者会短时不一致，这是当前实现刻意接受的。
