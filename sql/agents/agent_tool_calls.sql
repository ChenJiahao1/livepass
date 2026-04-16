CREATE TABLE IF NOT EXISTS agent_tool_calls (
  id varchar(64) PRIMARY KEY,
  run_id varchar(64) NOT NULL,
  thread_id varchar(64) NOT NULL,
  user_id bigint NOT NULL,
  message_id varchar(64) NULL,
  tool_name varchar(128) NOT NULL,
  status varchar(32) NOT NULL,
  arguments_json json NOT NULL,
  request_json json NOT NULL,
  output_json json NULL,
  error_json json NULL,
  created_at datetime(3) NOT NULL,
  updated_at datetime(3) NOT NULL,
  completed_at datetime(3) NULL,
  metadata_json json NULL,
  KEY idx_agent_tool_calls_run_created (run_id, created_at, id),
  KEY idx_agent_tool_calls_status (run_id, status, updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
