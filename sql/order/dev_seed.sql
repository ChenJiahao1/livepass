-- 初始化测试用户已支付订单，用于智能客服查单、退款预览与退款提交冒烟。
-- 订单号由 user_id=10001 按当前 order sharding 编码生成，逻辑槽 516，路由到 d_order_00。
INSERT INTO `d_order_00` (
  `id`, `order_number`, `program_id`, `show_time_id`, `program_title`, `program_item_picture`,
  `program_place`, `program_show_time`, `program_permit_choose_seat`, `user_id`,
  `distribution_mode`, `take_ticket_mode`, `ticket_count`, `order_price`, `order_status`,
  `freeze_token`, `order_expire_time`, `create_order_time`, `cancel_order_time`, `pay_order_time`,
  `create_time`, `edit_time`, `status`
) VALUES (
  10001, 43509738860707840, 10001, 30001, 'Phase1 示例演出', 'https://example.com/program-10001.jpg',
  '北京示例剧场', '2026-12-31 19:30:00', 0, 10001,
  'electronic', 'qr_code', 2, 598, 3,
  'dev-paid-order-10001', '2026-04-28 14:15:00', '2026-04-28 14:00:00', NULL, '2026-04-28 14:01:00',
  '2026-04-28 14:00:00', '2026-04-28 14:01:00', 1
);

INSERT INTO `d_order_ticket_user_00` (
  `id`, `order_number`, `show_time_id`, `user_id`, `ticket_user_id`, `ticket_user_name`,
  `ticket_user_id_number`, `ticket_category_id`, `ticket_category_name`, `ticket_price`,
  `seat_id`, `seat_row`, `seat_col`, `seat_price`, `order_status`,
  `create_order_time`, `create_time`, `edit_time`, `status`
) VALUES
  (
    10001, 43509738860707840, 30001, 10001, 10001, '测试观演人A',
    '110101199001010019', 40001, '普通票', 299,
    50001, 1, 1, 299, 3,
    '2026-04-28 14:00:00', '2026-04-28 14:00:00', '2026-04-28 14:01:00', 1
  ),
  (
    10002, 43509738860707840, 30001, 10001, 10002, '测试观演人B',
    '110101199001010027', 40001, '普通票', 299,
    50002, 1, 2, 299, 3,
    '2026-04-28 14:00:00', '2026-04-28 14:00:00', '2026-04-28 14:01:00', 1
  );

INSERT INTO `d_order_user_guard` (
  `id`, `order_number`, `program_id`, `show_time_id`, `user_id`, `create_time`, `edit_time`, `status`
) VALUES (
  10001, 43509738860707840, 10001, 30001, 10001, '2026-04-28 14:00:00', '2026-04-28 14:01:00', 1
);

INSERT INTO `d_order_viewer_guard` (
  `id`, `order_number`, `program_id`, `show_time_id`, `viewer_id`, `create_time`, `edit_time`, `status`
) VALUES
  (10001, 43509738860707840, 10001, 30001, 10001, '2026-04-28 14:00:00', '2026-04-28 14:01:00', 1),
  (10002, 43509738860707840, 10001, 30001, 10002, '2026-04-28 14:00:00', '2026-04-28 14:01:00', 1);

INSERT INTO `d_order_seat_guard` (
  `id`, `order_number`, `program_id`, `show_time_id`, `seat_id`, `create_time`, `edit_time`, `status`
) VALUES
  (10001, 43509738860707840, 10001, 30001, 50001, '2026-04-28 14:00:00', '2026-04-28 14:01:00', 1),
  (10002, 43509738860707840, 10001, 30001, 50002, '2026-04-28 14:00:00', '2026-04-28 14:01:00', 1);
