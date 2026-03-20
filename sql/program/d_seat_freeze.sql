DROP TABLE IF EXISTS `d_seat_freeze`;
CREATE TABLE `d_seat_freeze` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `freeze_token` varchar(64) NOT NULL COMMENT 'freeze token',
  `request_no` varchar(64) NOT NULL COMMENT 'idempotent request no',
  `program_id` bigint NOT NULL COMMENT 'program id',
  `ticket_category_id` bigint NOT NULL COMMENT 'ticket category id',
  `seat_count` int NOT NULL COMMENT 'frozen seat count',
  `freeze_status` tinyint NOT NULL COMMENT '1 frozen, 2 released, 3 expired, 4 confirmed',
  `expire_time` datetime NOT NULL COMMENT 'freeze expire time',
  `release_reason` varchar(128) DEFAULT NULL COMMENT 'release reason',
  `release_time` datetime DEFAULT NULL COMMENT 'released at',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_request_no` (`request_no`),
  UNIQUE KEY `uk_freeze_token` (`freeze_token`),
  KEY `idx_program_ticket_status` (`program_id`,`ticket_category_id`,`freeze_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='program seat freeze records';
