DROP TABLE IF EXISTS `d_seat`;
CREATE TABLE `d_seat` (
  `id` bigint NOT NULL COMMENT '主键',
  `program_id` bigint NOT NULL COMMENT '节目编号',
  `show_time_id` bigint NOT NULL COMMENT '场次编号',
  `ticket_category_id` bigint NOT NULL COMMENT '票档编号',
  `row_code` int NOT NULL COMMENT '座位行号',
  `col_code` int NOT NULL COMMENT '座位列号',
  `seat_type` tinyint NOT NULL COMMENT '座位类型',
  `price` decimal(10,0) NOT NULL COMMENT '座位价格',
  `seat_status` tinyint NOT NULL COMMENT '座位状态：1可售 2冻结 3已售',
  `freeze_token` varchar(64) DEFAULT NULL COMMENT '冻结令牌',
  `freeze_expire_time` datetime DEFAULT NULL COMMENT '冻结过期时间',
  `create_time` datetime NOT NULL COMMENT '创建时间',
  `edit_time` datetime NOT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_show_time_row_col` (`show_time_id`,`row_code`,`col_code`),
  KEY `idx_show_time_ticket_status` (`show_time_id`,`ticket_category_id`,`seat_status`),
  KEY `idx_freeze_token` (`freeze_token`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='座位库存表';
