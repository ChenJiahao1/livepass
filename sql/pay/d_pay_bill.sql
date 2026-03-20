DROP TABLE IF EXISTS `d_pay_bill`;
CREATE TABLE `d_pay_bill` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `pay_bill_no` bigint NOT NULL COMMENT 'pay bill number',
  `order_number` bigint NOT NULL COMMENT 'order number',
  `user_id` bigint NOT NULL COMMENT 'user id',
  `subject` varchar(256) NOT NULL COMMENT 'pay subject',
  `channel` varchar(32) NOT NULL DEFAULT 'mock' COMMENT 'pay channel',
  `order_amount` decimal(10,0) NOT NULL COMMENT 'order amount',
  `pay_status` tinyint NOT NULL COMMENT '1 created, 2 paid',
  `pay_time` datetime DEFAULT NULL COMMENT 'paid at',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_pay_bill_no` (`pay_bill_no`),
  UNIQUE KEY `uk_order_number` (`order_number`),
  KEY `idx_user_status_time` (`user_id`, `pay_status`, `create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='pay bill table';
