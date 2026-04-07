DROP TABLE IF EXISTS `d_order_ticket_user_00`;
CREATE TABLE `d_order_ticket_user_00` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `order_number` bigint NOT NULL COMMENT 'order number',
  `show_time_id` bigint NOT NULL COMMENT 'show time id',
  `user_id` bigint NOT NULL COMMENT '下单用户ID',
  `ticket_user_id` bigint NOT NULL COMMENT '观演人ID',
  `ticket_user_name` varchar(128) NOT NULL COMMENT '观演人姓名快照',
  `ticket_user_id_number` varchar(64) NOT NULL COMMENT '观演人证件号快照',
  `ticket_category_id` bigint NOT NULL COMMENT 'ticket category id',
  `ticket_category_name` varchar(128) NOT NULL COMMENT 'ticket category name snapshot',
  `ticket_price` decimal(10,0) NOT NULL COMMENT 'ticket category price snapshot',
  `seat_id` bigint NOT NULL COMMENT 'seat id',
  `seat_row` int NOT NULL COMMENT 'seat row',
  `seat_col` int NOT NULL COMMENT 'seat col',
  `seat_price` decimal(10,0) NOT NULL COMMENT 'seat price snapshot',
  `order_status` tinyint NOT NULL COMMENT '1 unpaid, 2 cancelled, 3 paid, 4 refunded',
  `create_order_time` datetime NOT NULL COMMENT 'order created at',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  KEY `idx_order_number` (`order_number`),
  KEY `idx_show_time_ticket_user` (`show_time_id`,`ticket_user_id`),
  KEY `idx_user_ticket_user` (`user_id`,`ticket_user_id`),
  KEY `idx_create_order_time` (`create_order_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单观演人明细表分片 00';

DROP TABLE IF EXISTS `d_order_ticket_user_01`;
CREATE TABLE `d_order_ticket_user_01` LIKE `d_order_ticket_user_00`;
