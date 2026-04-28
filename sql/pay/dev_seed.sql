-- 初始化测试用户已支付支付单，用于订单退款冒烟。
INSERT INTO `d_pay_bill` (
  `id`, `pay_bill_no`, `order_number`, `user_id`, `subject`, `channel`, `order_amount`,
  `pay_status`, `pay_time`, `create_time`, `edit_time`, `status`
) VALUES (
  10001, 202604281001, 43509738860707840, 10001, 'Phase1 示例演出 普通票 x2', 'mock', 598,
  2, '2026-04-28 14:01:00', '2026-04-28 14:00:00', '2026-04-28 14:01:00', 1
);
