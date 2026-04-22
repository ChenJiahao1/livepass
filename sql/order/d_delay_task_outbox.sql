DROP TABLE IF EXISTS `d_delay_task_outbox`;
CREATE TABLE `d_delay_task_outbox` (
  `id` bigint NOT NULL COMMENT '主键',
  `task_type` varchar(64) NOT NULL COMMENT '延迟任务类型',
  `task_key` varchar(128) NOT NULL COMMENT '延迟任务唯一键',
  `payload` longtext NOT NULL COMMENT '任务负载',
  `execute_at` datetime NOT NULL COMMENT '计划执行时间',
  `task_status` tinyint NOT NULL DEFAULT 0 COMMENT '任务状态：0待发布 1已投递 3已处理 4失败',
  `publish_attempts` int NOT NULL DEFAULT 0 COMMENT '发布尝试次数',
  `consume_attempts` int NOT NULL DEFAULT 0 COMMENT '消费尝试次数',
  `last_publish_error` varchar(512) NOT NULL DEFAULT '' COMMENT '最近一次发布错误',
  `last_consume_error` varchar(512) NOT NULL DEFAULT '' COMMENT '最近一次消费错误',
  `published_time` datetime DEFAULT NULL COMMENT '发布时间',
  `processed_time` datetime DEFAULT NULL COMMENT '处理完成时间',
  `create_time` datetime NOT NULL COMMENT '创建时间',
  `edit_time` datetime NOT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_task_type_task_key` (`task_type`,`task_key`),
  KEY `idx_dispatch_scan` (`task_status`,`task_type`,`status`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='延迟任务发件箱表';
