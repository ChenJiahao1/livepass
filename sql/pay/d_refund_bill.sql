DROP TABLE IF EXISTS `d_refund_bill`;
CREATE TABLE `d_refund_bill` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `refund_bill_no` bigint NOT NULL COMMENT 'refund bill number',
  `order_number` bigint NOT NULL COMMENT 'order number',
  `pay_bill_id` bigint NOT NULL COMMENT 'pay bill id',
  `user_id` bigint NOT NULL COMMENT 'user id',
  `refund_amount` decimal(10,0) NOT NULL COMMENT 'refund amount',
  `refund_status` tinyint NOT NULL COMMENT '1 created, 2 refunded',
  `refund_reason` varchar(256) DEFAULT NULL COMMENT 'refund reason',
  `refund_time` datetime DEFAULT NULL COMMENT 'refunded at',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_refund_bill_no` (`refund_bill_no`),
  UNIQUE KEY `uk_order_number` (`order_number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='refund bill table';
