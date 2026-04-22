CREATE TABLE IF NOT EXISTS agent_tool_calls (
  id varchar(64) PRIMARY KEY COMMENT '工具调用编号',
  run_id varchar(64) NOT NULL COMMENT '运行编号',
  message_id varchar(64) NOT NULL COMMENT '消息编号',
  thread_id varchar(64) NOT NULL COMMENT '会话编号',
  user_id bigint NOT NULL COMMENT '用户编号',
  name varchar(128) NOT NULL COMMENT '工具名称',
  status varchar(32) NOT NULL COMMENT '调用状态',
  input_json json NOT NULL COMMENT '输入参数数据',
  human_request_json json NOT NULL COMMENT '人工介入请求数据',
  output_json json NULL COMMENT '输出结果数据',
  error_json json NULL COMMENT '错误信息数据',
  created_at datetime(3) NOT NULL COMMENT '创建时间',
  updated_at datetime(3) NOT NULL COMMENT '更新时间',
  completed_at datetime(3) NULL COMMENT '完成时间',
  metadata_json json NULL COMMENT '扩展元数据',
  KEY idx_agent_tool_calls_run_created (run_id, created_at, id),
  KEY idx_agent_tool_calls_status (run_id, status, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='工具调用记录表';
