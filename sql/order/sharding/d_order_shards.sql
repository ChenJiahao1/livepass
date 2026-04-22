DROP TABLE IF EXISTS `d_order_00`;
CREATE TABLE `d_order_00` (
  `id` bigint NOT NULL COMMENT '主键',
  `order_number` bigint NOT NULL COMMENT '订单号',
  `program_id` bigint NOT NULL COMMENT '节目编号',
  `show_time_id` bigint NOT NULL COMMENT '场次编号',
  `program_title` varchar(256) NOT NULL COMMENT '节目标题快照',
  `program_item_picture` varchar(512) NOT NULL DEFAULT '' COMMENT '节目封面快照',
  `program_place` varchar(256) NOT NULL COMMENT '演出地点快照',
  `program_show_time` datetime NOT NULL COMMENT '演出时间快照',
  `program_permit_choose_seat` tinyint NOT NULL COMMENT '是否支持选座快照',
  `user_id` bigint NOT NULL COMMENT '用户编号',
  `distribution_mode` varchar(64) NOT NULL DEFAULT '' COMMENT '配送方式',
  `take_ticket_mode` varchar(64) NOT NULL DEFAULT '' COMMENT '取票方式',
  `ticket_count` int NOT NULL COMMENT '票数',
  `order_price` decimal(10,0) NOT NULL COMMENT '订单总金额',
  `order_status` tinyint NOT NULL COMMENT '订单状态：1待支付 2已取消 3已支付 4已退款',
  `freeze_token` varchar(64) NOT NULL COMMENT '座位冻结令牌',
  `order_expire_time` datetime NOT NULL COMMENT '订单过期时间',
  `create_order_time` datetime NOT NULL COMMENT '下单时间',
  `cancel_order_time` datetime DEFAULT NULL COMMENT '取消时间',
  `pay_order_time` datetime DEFAULT NULL COMMENT '支付时间',
  `create_time` datetime NOT NULL COMMENT '创建时间',
  `edit_time` datetime NOT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_number` (`order_number`),
  KEY `idx_user_status_time` (`user_id`,`order_status`,`create_order_time`,`id`),
  KEY `idx_user_time` (`user_id`,`create_order_time`,`id`),
  KEY `idx_show_time_user_status` (`show_time_id`,`user_id`,`order_status`),
  KEY `idx_close_scan` (`order_status`,`order_expire_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单主表分片 00';

DROP TABLE IF EXISTS `d_order_01`;
-- 基于 `d_order_00` 的结构复制创建订单主表分片 01，沿用全部字段注释与表注释。
CREATE TABLE `d_order_01` LIKE `d_order_00`;
