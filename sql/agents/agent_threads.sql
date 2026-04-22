CREATE TABLE IF NOT EXISTS agent_threads (
  id varchar(64) PRIMARY KEY COMMENT '会话编号',
  user_id bigint NOT NULL COMMENT '用户编号',
  title varchar(128) NOT NULL COMMENT '会话标题',
  status varchar(32) NOT NULL COMMENT '会话状态',
  created_at datetime(3) NOT NULL COMMENT '创建时间',
  updated_at datetime(3) NOT NULL COMMENT '更新时间',
  last_message_at datetime(3) NULL COMMENT '最后消息时间',
  metadata_json json NULL COMMENT '扩展元数据',
  KEY idx_agent_threads_user_status_last_message (user_id, status, last_message_at, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='智能体会话表';
