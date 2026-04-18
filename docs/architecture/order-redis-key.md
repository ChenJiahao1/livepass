Purchase Token

- 默认：无 Redis key
- token 形态：v1.<payload>.<signature>
- token claims 包含：orderNumber、userId、programId、showTimeId、ticketCategoryId、ticketUserIds、tokenFingerprint
- 下发阶段实际涉及：
    - 不写 Redis
    - 只查节目预下单、观演人、账号已持有有效票数
- 代码入口：services/order-rpc/internal/logic/create_purchase_token_logic.go:33

Order Rush Key

- 默认前缀：livepass:order:rush
- 当前 hash tag：{st:<showTimeId>}
- livepass:order:rush:{st:<showTimeId>}:attempt:<orderNumber> hash
- livepass:order:rush:{st:<showTimeId>}:user_active:<userId> string
- livepass:order:rush:{st:<showTimeId>}:user_inflight:<userId> string
- livepass:order:rush:{st:<showTimeId>}:viewer_active:<viewerId> string
- livepass:order:rush:{st:<showTimeId>}:viewer_inflight:<viewerId> string
- livepass:order:rush:{st:<showTimeId>}:quota:<ticketCategoryId> string
- livepass:order:rush:{st:<showTimeId>}:seat_occupied:<orderNumber> set
- livepass:order:rush:{st:<showTimeId>}:fingerprint:<userId> hash
- 其中 fingerprint:<userId> 的 field 是 tokenFingerprint，value 是 orderNumber
- key 定义：services/order-rpc/internal/rush/attempt_keys.go:5

Order Rush 实际涉及

- 下单前运行时预热会涉及：
    - 清理：user_inflight:*、viewer_inflight:*、fingerprint:*、quota:*
    - 重建：user_active:*、viewer_active:*、quota:*
- /order/create admission 只查/改：
    - attempt:<orderNumber>
    - user_active:<userId>
    - user_inflight:<userId>
    - viewer_active:<viewerId>
    - viewer_inflight:<viewerId>
    - quota:<ticketCategoryId>
    - fingerprint:<userId>
- Kafka 投递失败、还没进 consumer 时会回滚：
    - attempt:<orderNumber>
    - user_inflight:<userId>
    - viewer_inflight:<viewerId>
    - quota:<ticketCategoryId>
- consumer 消费开始只查/改：
    - attempt:<orderNumber>
    - 处理权由 `PrepareAttemptForConsume` 执行的 `ACCEPTED -> PROCESSING` 原子状态变更保证
    - 不再使用 `ClaimProcessing` 或 `processing_epoch`
- consumer 落单成功会涉及：
    - attempt:<orderNumber>
    - user_active:<userId>
    - user_inflight:<userId>
    - viewer_active:<viewerId>
    - viewer_inflight:<viewerId>
    - seat_occupied:<orderNumber>
- consumer 落单失败/释放会额外涉及：
    - attempt:<orderNumber>
    - quota:<ticketCategoryId>
    - fingerprint:<userId>
    - 同时清掉 user_active / user_inflight / viewer_active / viewer_inflight / seat_occupied
- /order/poll 实际是先扫：
    - livepass:order:rush:*:attempt:<orderNumber>
    - 命中后再读取具体的 attempt:<orderNumber>
- poll 的 scan 逻辑在 services/order-rpc/internal/rush/attempt_store.go:1182
- /order/create 入口在 services/order-rpc/internal/logic/create_order_logic.go:37

Program Seat Ledger Key

- 默认前缀：livepass:program:seat-ledger
- 当前 hash tag：{st:<showTimeId>}
- livepass:program:seat-ledger:stock:{st:<showTimeId>}:<ticketCategoryId> string
- livepass:program:seat-ledger:available:{st:<showTimeId>}:<ticketCategoryId> zset
- livepass:program:seat-ledger:sold:{st:<showTimeId>}:<ticketCategoryId> zset
- livepass:program:seat-ledger:frozen:{st:<showTimeId>}:<ticketCategoryId>:<freezeToken> zset
- livepass:program:seat-ledger:frozen:{st:<showTimeId>}:<ticketCategoryId>:*
- livepass:program:seat-ledger:loading:{st:<showTimeId>}:<ticketCategoryId> string
- livepass:program:seat-ledger:freeze:index:{st:<showTimeId>}:<ticketCategoryId> zset
- key 定义：services/program-rpc/internal/seatcache/seat_stock_keys.go:5

Program Seat Ledger 实际涉及

- seat ledger 预热会涉及：
    - stock
    - available
    - sold
    - frozen:*
    - loading
- consumer 锁座时 AutoAssignAndFreezeSeats 只查/改：
    - stock:{st:<showTimeId>}:<ticketCategoryId>
    - available:{st:<showTimeId>}:<ticketCategoryId>
    - frozen:{st:<showTimeId>}:<ticketCategoryId>:<freezeToken>
    - 请求入参是确定性 `freezeToken` + 显式 `freezeExpireTime`
- 如果 ledger 未就绪，可能额外写：
    - loading:{st:<showTimeId>}:<ticketCategoryId>
- consumer 落单成功到此为止：
    - frozen:<freezeToken> 保留
    - sold 不写
- consumer 落单失败释放锁座会涉及：
    - stock
    - available
    - frozen:<freezeToken>
- 锁座主流程入口：services/program-rpc/internal/logic/auto_assign_and_freeze_seats_logic.go:44
