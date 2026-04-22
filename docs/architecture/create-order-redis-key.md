- livepass:order:rush:attempt:{st:<showTimeID>}:<orderNumber>
    - 类型：HASH
    - 用途：一次抢票 attempt 状态机。
    - 字段：order_number、user_id、program_id、show_time_id、ticket_category_id、viewer_ids、ticket_count、sale_window_end_at、show_end_at、state、reason_code、accepted_at、finished_at、processing_started_at、created_at、
    last_transition_at
- livepass:order:rush:user_inflight:{st:<showTimeID>}
    - 类型：HASH
    - 字段：<userID>
    - 值：<orderNumber>
    - 用途：同用户同场次处理中防重复提交。
- livepass:order:rush:viewer_inflight:{st:<showTimeID>}
    - 类型：HASH
    - 字段：<viewerID>
    - 值：<orderNumber>
    - 用途：同观演人同场次处理中防重复提交。
- livepass:order:rush:user_active:{st:<showTimeID>}
    - 类型：HASH
    - 字段：<userID>
    - 值：<orderNumber>
    - 用途：同用户同场次已成功订单投影。
- livepass:order:rush:viewer_active:{st:<showTimeID>}
    - 类型：HASH
    - 字段：<viewerID>
    - 值：<orderNumber>
    - 用途：同观演人同场次已成功订单投影。
- livepass:order:rush:quota:{st:<showTimeID>}
    - 类型：HASH
    - 字段：<ticketCategoryID>
    - 值：<availableAdmissionQuota>
    - 用途：抢票准入名额，Admit 时原子扣减，失败/关单时回补。

Program 座位库存 Key

- livepass:program:seat-ledger:stock:{st:<showTimeID>}:<ticketCategoryID>
    - 类型：STRING
    - 值：可用座位数量。
- livepass:program:seat-ledger:available:{st:<showTimeID>}:<ticketCategoryID>
    - 类型：ZSET
    - member：<seatID>|<ticketCategoryID>|<rowCode>|<colCode>|<price>
    - score：rowCode*1000000 + colCode
    - 用途：可分配座位池。
- livepass:program:seat-ledger:frozen:{st:<showTimeID>}:<ticketCategoryID>:<freezeToken>
    - 类型：ZSET
    - member 同上。
    - 用途：某个订单冻结中的座位集合。
- livepass:program:seat-ledger:sold:{st:<showTimeID>}:<ticketCategoryID>
    - 类型：ZSET
    - member 同上。
    - 用途：已售座位集合。
- livepass:program:seat-ledger:loading:{st:<showTimeID>}:<ticketCategoryID>
    - 类型：STRING
    - 值：1
    - 用途：座位库存从 DB 加载到 Redis 的短锁/冷却标记。

预热任务 Key

- 业务 task key：program.rush_inventory_preheat:<programID>:<yyyyMMddHHmmss>
    - 用途：作为 Asynq TaskID，避免重复入队。
    - 示例：program.rush_inventory_preheat:20001:20261231180000