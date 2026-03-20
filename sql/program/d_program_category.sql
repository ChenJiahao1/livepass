DROP TABLE IF EXISTS `d_program_category`;
CREATE TABLE `d_program_category` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  `parent_id` bigint NOT NULL DEFAULT 0 COMMENT 'parent category id',
  `name` varchar(120) NOT NULL COMMENT 'category name',
  `type` int NOT NULL DEFAULT 2 COMMENT '1 root, 2 child',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_parent_name_type` (`parent_id`, `name`, `type`),
  KEY `idx_parent_id` (`parent_id`),
  KEY `idx_type` (`type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='program categories';
