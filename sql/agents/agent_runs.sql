CREATE TABLE IF NOT EXISTS agent_runs (
  id varchar(64) PRIMARY KEY,
  thread_id varchar(64) NOT NULL,
  user_id bigint NOT NULL,
  trigger_message_id varchar(64) NOT NULL,
  output_message_id varchar(64) NOT NULL,
  status varchar(32) NOT NULL,
  started_at datetime(3) NOT NULL,
  completed_at datetime(3) NULL,
  error_json json NULL,
  metadata_json json NULL,
  KEY idx_agent_runs_thread_started (thread_id, started_at DESC, id DESC),
  KEY idx_agent_runs_user_started (user_id, started_at DESC, id DESC),
  KEY idx_agent_runs_user_status (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
