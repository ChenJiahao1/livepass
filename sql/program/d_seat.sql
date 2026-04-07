DROP TABLE IF EXISTS `d_seat`;
CREATE TABLE `d_seat` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `program_id` bigint NOT NULL COMMENT 'program id',
  `show_time_id` bigint NOT NULL COMMENT 'show time id',
  `ticket_category_id` bigint NOT NULL COMMENT 'ticket category id',
  `row_code` int NOT NULL COMMENT 'seat row code',
  `col_code` int NOT NULL COMMENT 'seat column code',
  `seat_type` tinyint NOT NULL COMMENT 'seat type',
  `price` decimal(10,0) NOT NULL COMMENT 'seat price',
  `seat_status` tinyint NOT NULL COMMENT '1 available, 2 frozen, 3 sold',
  `freeze_token` varchar(64) DEFAULT NULL COMMENT 'freeze token',
  `freeze_expire_time` datetime DEFAULT NULL COMMENT 'freeze expire time',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_show_time_row_col` (`show_time_id`,`row_code`,`col_code`),
  KEY `idx_show_time_ticket_status` (`show_time_id`,`ticket_category_id`,`seat_status`),
  KEY `idx_freeze_token` (`freeze_token`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='program seat inventory';
