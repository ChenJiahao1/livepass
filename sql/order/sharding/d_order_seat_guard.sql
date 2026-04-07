DROP TABLE IF EXISTS `d_order_seat_guard`;
CREATE TABLE `d_order_seat_guard` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `order_number` bigint NOT NULL COMMENT 'order number',
  `program_id` bigint NOT NULL COMMENT 'program id',
  `show_time_id` bigint NOT NULL COMMENT 'show time id',
  `seat_id` bigint NOT NULL COMMENT 'seat id',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_show_time_seat` (`show_time_id`,`seat_id`),
  KEY `idx_order_number` (`order_number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单座位有效持有 guard';
