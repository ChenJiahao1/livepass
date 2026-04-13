DROP TABLE IF EXISTS `d_delay_task_outbox`;
CREATE TABLE `d_delay_task_outbox` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `task_type` varchar(64) NOT NULL COMMENT 'delay task type',
  `task_key` varchar(128) NOT NULL COMMENT 'delay task unique key',
  `payload` longtext NOT NULL COMMENT 'task payload',
  `execute_at` datetime NOT NULL COMMENT 'scheduled execute time',
  `published_status` tinyint NOT NULL DEFAULT 0 COMMENT '0 pending, 1 published',
  `publish_attempts` int NOT NULL DEFAULT 0 COMMENT 'publish attempts',
  `last_publish_error` varchar(512) NOT NULL DEFAULT '' COMMENT 'last publish error',
  `published_time` datetime DEFAULT NULL COMMENT 'published at',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_task_type_task_key` (`task_type`,`task_key`),
  KEY `idx_dispatch_scan` (`published_status`,`task_type`,`status`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='延迟任务 outbox';
