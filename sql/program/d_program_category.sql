DROP TABLE IF EXISTS `d_program_category`;
CREATE TABLE `d_program_category` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `parent_id` bigint NOT NULL DEFAULT 0 COMMENT '父级分类编号',
  `name` varchar(120) NOT NULL COMMENT '分类名称',
  `type` int NOT NULL DEFAULT 2 COMMENT '分类层级：1一级 2二级',
  `create_time` datetime NOT NULL COMMENT '创建时间',
  `edit_time` datetime NOT NULL COMMENT '更新时间',
  `status` tinyint(1) NOT NULL DEFAULT 1 COMMENT '数据状态：1正常 0删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_parent_name_type` (`parent_id`, `name`, `type`),
  KEY `idx_parent_id` (`parent_id`),
  KEY `idx_type` (`type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='节目分类表';
