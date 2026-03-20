DROP TABLE IF EXISTS `d_ticket_category`;
CREATE TABLE `d_ticket_category` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `program_id` bigint NOT NULL COMMENT 'program id',
  `introduce` varchar(256) NOT NULL COMMENT 'ticket category introduction',
  `price` decimal(10,0) NOT NULL COMMENT 'price',
  `total_number` bigint NOT NULL COMMENT 'total ticket number',
  `remain_number` bigint NOT NULL COMMENT 'remaining ticket number',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  KEY `idx_program_id` (`program_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ticket categories';
