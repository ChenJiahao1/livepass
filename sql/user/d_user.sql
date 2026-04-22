DROP TABLE IF EXISTS `d_user`;

CREATE TABLE `d_user` (
  `id` bigint NOT NULL COMMENT '主键',
  `name` varchar(256) DEFAULT NULL COMMENT '用户名称',
  `rel_name` varchar(256) DEFAULT NULL COMMENT '用户真实姓名',
  `mobile` varchar(512) NOT NULL COMMENT '手机号',
  `gender` int NOT NULL DEFAULT '1' COMMENT '性别：1男 2女',
  `password` varchar(512) DEFAULT NULL COMMENT '密码',
  `email_status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '邮箱认证状态：1已验证 0未验证',
  `email` varchar(256) DEFAULT NULL COMMENT '邮箱地址',
  `rel_authentication_status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '实名认证状态：1已验证 0未验证',
  `id_number` varchar(512) DEFAULT NULL COMMENT '身份证号码',
  `address` varchar(256) DEFAULT NULL COMMENT '收货地址',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `edit_time` datetime DEFAULT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`),
  KEY `idx_d_user_mobile` (`mobile`),
  KEY `idx_d_user_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';
