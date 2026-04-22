DROP TABLE IF EXISTS `d_user_email`;

CREATE TABLE `d_user_email` (
  `id` bigint NOT NULL COMMENT '主键',
  `user_id` bigint NOT NULL COMMENT '用户编号',
  `email` varchar(256) NOT NULL COMMENT '邮箱',
  `email_status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '邮箱认证状态：1已验证 0未验证',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `edit_time` datetime DEFAULT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_d_user_email_email` (`email`),
  KEY `idx_d_user_email_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户邮箱映射表';
