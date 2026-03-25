DROP TABLE IF EXISTS `d_user_order_index_00`;
CREATE TABLE `d_user_order_index_00` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `order_number` bigint NOT NULL COMMENT 'order number',
  `user_id` bigint NOT NULL COMMENT 'user id',
  `program_id` bigint NOT NULL COMMENT 'program id',
  `order_status` tinyint NOT NULL COMMENT '1 unpaid, 2 cancelled, 3 paid, 4 refunded',
  `ticket_count` int NOT NULL COMMENT 'ticket count',
  `order_price` decimal(10,0) NOT NULL COMMENT 'order total price',
  `create_order_time` datetime NOT NULL COMMENT 'order created at',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_number` (`order_number`),
  KEY `idx_user_status_time` (`user_id`,`order_status`,`create_order_time`,`id`),
  KEY `idx_user_time` (`user_id`,`create_order_time`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='user order index shard table 00';

DROP TABLE IF EXISTS `d_user_order_index_01`;
CREATE TABLE `d_user_order_index_01` LIKE `d_user_order_index_00`;
