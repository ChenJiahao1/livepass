DROP TABLE IF EXISTS `d_order_user_guard_00`;
CREATE TABLE `d_order_user_guard_00` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `order_number` bigint NOT NULL COMMENT 'order number',
  `program_id` bigint NOT NULL COMMENT 'program id',
  `user_id` bigint NOT NULL COMMENT '下单用户ID',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_program_user` (`program_id`,`user_id`),
  KEY `idx_order_number` (`order_number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单用户有效持有 guard 分片 00';

DROP TABLE IF EXISTS `d_order_user_guard_01`;
CREATE TABLE `d_order_user_guard_01` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `order_number` bigint NOT NULL COMMENT 'order number',
  `program_id` bigint NOT NULL COMMENT 'program id',
  `user_id` bigint NOT NULL COMMENT '下单用户ID',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_program_user` (`program_id`,`user_id`),
  KEY `idx_order_number` (`order_number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单用户有效持有 guard 分片 01';
