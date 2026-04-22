CREATE TABLE IF NOT EXISTS agent_messages (
  id varchar(64) PRIMARY KEY COMMENT '消息编号',
  thread_id varchar(64) NOT NULL COMMENT '会话编号',
  user_id bigint NOT NULL COMMENT '用户编号',
  role varchar(32) NOT NULL COMMENT '消息角色',
  content_json json NOT NULL COMMENT '消息内容数据',
  status varchar(32) NOT NULL COMMENT '消息状态',
  run_id varchar(64) NULL COMMENT '关联运行编号',
  created_at datetime(3) NOT NULL COMMENT '创建时间',
  updated_at datetime(3) NOT NULL COMMENT '更新时间',
  metadata_json json NULL COMMENT '扩展元数据',
  KEY idx_agent_messages_thread_created (thread_id, created_at, id),
  KEY idx_agent_messages_user_thread (user_id, thread_id),
  KEY idx_agent_messages_run_id (run_id, created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='智能体消息表';
