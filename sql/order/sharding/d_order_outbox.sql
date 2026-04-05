DROP TABLE IF EXISTS `d_order_outbox`;
CREATE TABLE `d_order_outbox` (
  `id` bigint NOT NULL COMMENT 'primary key',
  `order_number` bigint NOT NULL COMMENT 'order number',
  `event_type` varchar(64) NOT NULL COMMENT 'event type',
  `payload` longtext NOT NULL COMMENT 'event payload',
  `published_status` tinyint NOT NULL DEFAULT 0 COMMENT '0 pending, 1 published',
  `published_time` datetime DEFAULT NULL COMMENT 'published at',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_event_type` (`order_number`,`event_type`),
  KEY `idx_publish_scan` (`published_status`,`status`,`id`),
  KEY `idx_order_number` (`order_number`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单 outbox';
