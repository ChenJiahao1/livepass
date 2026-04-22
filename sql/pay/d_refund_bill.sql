DROP TABLE IF EXISTS `d_refund_bill`;
CREATE TABLE `d_refund_bill` (
  `id` bigint NOT NULL COMMENT '主键',
  `refund_bill_no` bigint NOT NULL COMMENT '退款单号',
  `order_number` bigint NOT NULL COMMENT '业务订单号',
  `pay_bill_id` bigint NOT NULL COMMENT '支付单编号',
  `user_id` bigint NOT NULL COMMENT '用户编号',
  `refund_amount` decimal(10,0) NOT NULL COMMENT '退款金额',
  `refund_status` tinyint NOT NULL COMMENT '退款状态：1已创建 2已退款',
  `refund_reason` varchar(256) DEFAULT NULL COMMENT '退款原因快照',
  `refund_time` datetime DEFAULT NULL COMMENT '退款成功时间',
  `create_time` datetime NOT NULL COMMENT '创建时间',
  `edit_time` datetime NOT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_refund_bill_no` (`refund_bill_no`),
  UNIQUE KEY `uk_order_number` (`order_number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='退款单表';
