DROP TABLE IF EXISTS `d_order_seat_guard`;
CREATE TABLE `d_order_seat_guard` (
  `id` bigint NOT NULL COMMENT '主键',
  `order_number` bigint NOT NULL COMMENT '订单号',
  `program_id` bigint NOT NULL COMMENT '节目编号',
  `show_time_id` bigint NOT NULL COMMENT '场次编号',
  `seat_id` bigint NOT NULL COMMENT '座位编号',
  `create_time` datetime NOT NULL COMMENT '创建时间',
  `edit_time` datetime NOT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_show_time_seat` (`show_time_id`,`seat_id`),
  KEY `idx_order_number` (`order_number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单座位占用保护表';
