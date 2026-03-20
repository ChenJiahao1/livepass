DROP TABLE IF EXISTS `d_program_group`;
CREATE TABLE `d_program_group` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `program_json` text NOT NULL COMMENT 'json list of simple program infos',
  `recent_show_time` datetime NOT NULL COMMENT 'nearest show time',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='program groups';
