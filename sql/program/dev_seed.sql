INSERT INTO `d_program_category` (`id`, `parent_id`, `name`, `type`, `create_time`, `edit_time`, `status`) VALUES
  (1, 0, '演唱会', 1, '2026-01-01 00:00:00', '2026-01-01 00:00:00', 1),
  (2, 0, '话剧歌剧', 1, '2026-01-01 00:00:00', '2026-01-01 00:00:00', 1),
  (11, 1, 'livehouse', 2, '2026-01-01 00:00:00', '2026-01-01 00:00:00', 1),
  (12, 2, '话剧', 2, '2026-01-01 00:00:00', '2026-01-01 00:00:00', 1);

INSERT INTO `d_program_group` (`id`, `program_json`, `recent_show_time`, `create_time`, `edit_time`, `status`) VALUES
  (20001, '[{"programId":10001,"areaId":2,"areaIdName":"北京"}]', '2026-12-31 19:30:00', '2026-01-01 00:00:00', '2026-01-01 00:00:00', 1);

INSERT INTO `d_program` (
  `id`, `program_group_id`, `prime`, `area_id`, `program_category_id`, `parent_program_category_id`,
  `title`, `actor`, `place`, `item_picture`, `pre_sell`, `pre_sell_instruction`, `important_notice`, `detail`,
  `per_order_limit_purchase_count`, `per_account_limit_purchase_count`, `refund_ticket_rule`, `delivery_instruction`,
  `entry_rule`, `child_purchase`, `invoice_specification`, `real_ticket_purchase_rule`, `abnormal_order_description`,
  `kind_reminder`, `performance_duration`, `entry_time`, `min_performance_count`, `main_actor`,
  `min_performance_duration`, `prohibited_item`, `deposit_specification`, `total_count`, `permit_refund`,
  `refund_explain`, `rel_name_ticket_entrance`, `rel_name_ticket_entrance_explain`, `permit_choose_seat`,
  `choose_seat_explain`, `electronic_delivery_ticket`, `electronic_delivery_ticket_explain`, `electronic_invoice`,
  `electronic_invoice_explain`, `high_heat`, `program_status`, `issue_time`, `create_time`, `edit_time`, `status`
) VALUES (
  10001, 20001, 1, 2, 11, 1,
  'Phase1 示例演出', '示例艺人', '北京示例剧场', 'https://example.com/program-10001.jpg', 1,
  '本商品为预售商品，正式开票后将第一时间为您配票。', '请以现场公告为准。', '<p>Phase 1 detail</p>',
  6, 6, '票品为有价票券，不支持退换。', '不支持修改配送电话和地址。',
  '请按场馆规则入场。', '儿童一律凭票入场。', '演出开始前提交发票申请。', '一个订单对应一个证件。',
  '异常订购行为可能被取消订单。', '请提前规划行程。', '约120分钟', '提前30分钟入场', 20, '示例主演',
  '约120分钟', '请勿携带违禁品。', '以现场为准', 1000, 0,
  '不支持退换。', 0, '本场次无需实名入场。', 0,
  '本项目不支持自主选座，同一个订单优先连座。', 1, '电子票扫码入场', 1,
  '电子发票将发送至邮箱。', 1, 1, '2026-06-01 09:00:00', '2026-01-01 00:00:00', '2026-01-01 00:00:00', 1
);

INSERT INTO `d_program_show_time` (`id`, `program_id`, `show_time`, `show_day_time`, `show_week_time`, `create_time`, `edit_time`, `status`) VALUES
  (30001, 10001, '2026-12-31 19:30:00', '2026-12-31 00:00:00', '周四', '2026-01-01 00:00:00', '2026-01-01 00:00:00', 1);

INSERT INTO `d_ticket_category` (`id`, `program_id`, `introduce`, `price`, `total_number`, `remain_number`, `create_time`, `edit_time`, `status`) VALUES
  (40001, 10001, '普通票', 299, 100, 100, '2026-01-01 00:00:00', '2026-01-01 00:00:00', 1),
  (40002, 10001, 'VIP票', 599, 80, 80, '2026-01-01 00:00:00', '2026-01-01 00:00:00', 1);
