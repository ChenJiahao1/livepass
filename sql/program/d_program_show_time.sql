DROP TABLE IF EXISTS `d_program_show_time`;
CREATE TABLE `d_program_show_time` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `program_id` bigint NOT NULL COMMENT 'program id',
  `show_time` datetime NOT NULL COMMENT 'show datetime',
  `show_day_time` datetime DEFAULT NULL COMMENT 'show day datetime',
  `show_week_time` varchar(64) NOT NULL COMMENT 'weekday text',
  `create_time` datetime DEFAULT NULL COMMENT 'created at',
  `edit_time` datetime DEFAULT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  KEY `idx_program_id` (`program_id`),
  KEY `idx_show_time` (`show_time`),
  KEY `idx_show_day_time` (`show_day_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='program show times';
