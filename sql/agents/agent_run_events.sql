CREATE TABLE IF NOT EXISTS agent_run_events (
  id varchar(64) PRIMARY KEY COMMENT '事件编号',
  run_id varchar(64) NOT NULL COMMENT '运行编号',
  thread_id varchar(64) NOT NULL COMMENT '会话编号',
  user_id bigint NOT NULL COMMENT '用户编号',
  sequence_no bigint NOT NULL COMMENT '事件序号',
  event_type varchar(64) NOT NULL COMMENT '事件类型',
  message_id varchar(64) NULL COMMENT '关联消息编号',
  tool_call_id varchar(64) NULL COMMENT '关联工具调用编号',
  payload_json json NOT NULL COMMENT '事件负载数据',
  created_at datetime(3) NOT NULL COMMENT '创建时间',
  UNIQUE KEY uk_agent_run_events_run_seq (run_id, sequence_no),
  KEY idx_agent_run_events_thread_created (thread_id, created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='智能体运行事件表';
