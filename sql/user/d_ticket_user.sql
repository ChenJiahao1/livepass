DROP TABLE IF EXISTS `d_ticket_user`;

CREATE TABLE `d_ticket_user` (
  `id` bigint NOT NULL COMMENT '主键id',
  `user_id` bigint NOT NULL COMMENT '用户id',
  `rel_name` varchar(256) NOT NULL COMMENT '真实姓名',
  `id_type` int NOT NULL DEFAULT '1' COMMENT '证件类型',
  `id_number` varchar(512) NOT NULL COMMENT '证件号',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `edit_time` datetime DEFAULT NULL COMMENT '编辑时间',
  `status` tinyint(1) NOT NULL DEFAULT '1' COMMENT '1:正常 0:删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_d_ticket_user_identity` (`user_id`,`id_type`,`id_number`),
  KEY `idx_d_ticket_user_user_id` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='观演人表';
