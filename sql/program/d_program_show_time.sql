DROP TABLE IF EXISTS `d_program_show_time`;
CREATE TABLE `d_program_show_time` (
  `id` bigint NOT NULL COMMENT '主键',
  `program_id` bigint NOT NULL COMMENT '节目编号',
  `show_time` datetime NOT NULL COMMENT '演出时间',
  `show_day_time` datetime DEFAULT NULL COMMENT '演出日期',
  `show_week_time` varchar(64) NOT NULL COMMENT '星期描述',
  `show_end_time` datetime DEFAULT NULL COMMENT '散场时间',
  `create_time` datetime DEFAULT NULL COMMENT '创建时间',
  `edit_time` datetime DEFAULT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`),
  KEY `idx_program_id` (`program_id`),
  KEY `idx_show_time` (`show_time`),
  KEY `idx_show_day_time` (`show_day_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='节目场次表';
