DROP TABLE IF EXISTS `d_user`;

CREATE TABLE `d_user` (
  `id` bigint NOT NULL COMMENT '主键id',
  `name` varchar(256) DEFAULT NULL COMMENT '用户名字',
  `rel_name` varchar(256) DEFAULT NULL COMMENT '用户真实名字',
  `mobile` varchar(512) NOT NULL COMMENT '手机号',
  `gender` int NOT NULL DEFAULT '1' COMMENT '1:男 2:女',
  `password` varchar(512) DEFAULT NULL COMMENT '密码',
  `email_status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否邮箱认证 1:已验证 0:未验证',
  `email` varchar(256) DEFAULT NULL COMMENT '邮箱地址',
  `rel_authentication_status` tinyint(1) NOT NULL DEFAULT '0' COMMENT '是否实名认证 1:已验证 0:未验证',
  `id_number` varchar(512) DEFAULT NULL COMMENT '身份证号码',
  `address` varchar(256) DEFAULT NULL COMMENT '收货地址',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `edit_time` datetime DEFAULT NULL COMMENT '编辑时间',
  `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT '1:正常 0:删除',
  PRIMARY KEY (`id`),
  KEY `idx_d_user_mobile` (`mobile`),
  KEY `idx_d_user_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';
