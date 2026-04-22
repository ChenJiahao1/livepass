CREATE TABLE IF NOT EXISTS agent_runs (
  id varchar(64) PRIMARY KEY COMMENT '运行编号',
  thread_id varchar(64) NOT NULL COMMENT '会话编号',
  user_id bigint NOT NULL COMMENT '用户编号',
  trigger_message_id varchar(64) NOT NULL COMMENT '触发消息编号',
  output_message_id varchar(64) NOT NULL COMMENT '输出消息编号',
  status varchar(32) NOT NULL COMMENT '运行状态',
  started_at datetime(3) NOT NULL COMMENT '开始时间',
  completed_at datetime(3) NULL COMMENT '完成时间',
  error_json json NULL COMMENT '错误信息数据',
  metadata_json json NULL COMMENT '扩展元数据',
  KEY idx_agent_runs_thread_started (thread_id, started_at DESC, id DESC),
  KEY idx_agent_runs_user_started (user_id, started_at DESC, id DESC),
  KEY idx_agent_runs_user_status (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='智能体运行表';
