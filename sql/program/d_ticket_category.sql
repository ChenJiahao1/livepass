DROP TABLE IF EXISTS `d_ticket_category`;
CREATE TABLE `d_ticket_category` (
  `id` bigint NOT NULL COMMENT '主键',
  `program_id` bigint NOT NULL COMMENT '节目编号',
  `show_time_id` bigint NOT NULL COMMENT '场次编号',
  `introduce` varchar(256) NOT NULL COMMENT '票档说明',
  `price` decimal(10,0) NOT NULL COMMENT '票档价格',
  `total_number` bigint NOT NULL COMMENT '票档总票数',
  `remain_number` bigint NOT NULL COMMENT '票档余票数',
  `create_time` datetime NOT NULL COMMENT '创建时间',
  `edit_time` datetime NOT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`),
  KEY `idx_program_id` (`program_id`),
  KEY `idx_show_time_id` (`show_time_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='票档表';
