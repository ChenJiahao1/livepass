DROP TABLE IF EXISTS `d_program_group`;
CREATE TABLE `d_program_group` (
  `id` bigint NOT NULL COMMENT '主键',
  `program_json` text NOT NULL COMMENT '节目简要信息列表',
  `recent_show_time` datetime NOT NULL COMMENT '最近演出时间',
  `create_time` datetime NOT NULL COMMENT '创建时间',
  `edit_time` datetime NOT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='节目分组表';
