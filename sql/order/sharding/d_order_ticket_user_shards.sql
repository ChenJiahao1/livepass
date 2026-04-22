DROP TABLE IF EXISTS `d_order_ticket_user_00`;
CREATE TABLE `d_order_ticket_user_00` (
  `id` bigint NOT NULL COMMENT '主键',
  `order_number` bigint NOT NULL COMMENT '订单号',
  `show_time_id` bigint NOT NULL COMMENT '场次编号',
  `user_id` bigint NOT NULL COMMENT '下单用户编号',
  `ticket_user_id` bigint NOT NULL COMMENT '观演人编号',
  `ticket_user_name` varchar(128) NOT NULL COMMENT '观演人姓名快照',
  `ticket_user_id_number` varchar(64) NOT NULL COMMENT '观演人证件号快照',
  `ticket_category_id` bigint NOT NULL COMMENT '票档编号',
  `ticket_category_name` varchar(128) NOT NULL COMMENT '票档名称快照',
  `ticket_price` decimal(10,0) NOT NULL COMMENT '票档价格快照',
  `seat_id` bigint NOT NULL COMMENT '座位编号',
  `seat_row` int NOT NULL COMMENT '座位行号',
  `seat_col` int NOT NULL COMMENT '座位列号',
  `seat_price` decimal(10,0) NOT NULL COMMENT '座位价格快照',
  `order_status` tinyint NOT NULL COMMENT '订单状态：1待支付 2已取消 3已支付 4已退款',
  `create_order_time` datetime NOT NULL COMMENT '下单时间',
  `create_time` datetime NOT NULL COMMENT '创建时间',
  `edit_time` datetime NOT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`),
  KEY `idx_order_number` (`order_number`),
  KEY `idx_show_time_ticket_user` (`show_time_id`,`ticket_user_id`),
  KEY `idx_user_ticket_user` (`user_id`,`ticket_user_id`),
  KEY `idx_create_order_time` (`create_order_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单观演人明细表分片 00';

DROP TABLE IF EXISTS `d_order_ticket_user_01`;
-- 基于 `d_order_ticket_user_00` 的结构复制创建订单观演人明细分片 01，沿用全部字段注释与表注释。
CREATE TABLE `d_order_ticket_user_01` LIKE `d_order_ticket_user_00`;
