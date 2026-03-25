DROP TABLE IF EXISTS `d_order_route_legacy`;
CREATE TABLE `d_order_route_legacy` (
  `order_number` bigint NOT NULL COMMENT 'legacy order number',
  `user_id` bigint NOT NULL COMMENT 'user id',
  `logic_slot` int NOT NULL COMMENT 'stable logic slot',
  `route_version` varchar(64) NOT NULL COMMENT 'route map version',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '1 active, 0 deleted',
  `create_time` datetime NOT NULL COMMENT 'created at',
  `edit_time` datetime NOT NULL COMMENT 'updated at',
  PRIMARY KEY (`order_number`),
  UNIQUE KEY `uk_order_number` (`order_number`),
  KEY `idx_user_time` (`user_id`,`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='legacy order routing directory';
