DROP TABLE IF EXISTS `d_pay_bill`;
CREATE TABLE `d_pay_bill` (
  `id` bigint NOT NULL COMMENT '主键',
  `pay_bill_no` bigint NOT NULL COMMENT '支付单号',
  `order_number` bigint NOT NULL COMMENT '订单号',
  `user_id` bigint NOT NULL COMMENT '用户编号',
  `subject` varchar(256) NOT NULL COMMENT '支付主题',
  `channel` varchar(32) NOT NULL DEFAULT 'mock' COMMENT '支付渠道',
  `order_amount` decimal(10,0) NOT NULL COMMENT '订单金额',
  `pay_status` tinyint NOT NULL COMMENT '支付状态：1待支付 2已支付 3已退款',
  `pay_time` datetime DEFAULT NULL COMMENT '支付时间',
  `create_time` datetime NOT NULL COMMENT '创建时间',
  `edit_time` datetime NOT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_pay_bill_no` (`pay_bill_no`),
  UNIQUE KEY `uk_order_number` (`order_number`),
  KEY `idx_user_status_time` (`user_id`, `pay_status`, `create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='支付单表';
