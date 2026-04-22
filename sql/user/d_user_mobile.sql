DROP TABLE IF EXISTS `d_user_mobile`;

CREATE TABLE `d_user_mobile` (
  `id` bigint NOT NULL COMMENT '主键',
  `user_id` bigint NOT NULL COMMENT '用户编号',
  `mobile` varchar(512) NOT NULL COMMENT '手机号',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `edit_time` datetime DEFAULT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_d_user_mobile_mobile` (`mobile`),
  KEY `idx_d_user_mobile_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户手机号映射表';
