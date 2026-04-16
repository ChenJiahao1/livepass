CREATE TABLE IF NOT EXISTS agent_run_events (
  id varchar(64) PRIMARY KEY,
  run_id varchar(64) NOT NULL,
  thread_id varchar(64) NOT NULL,
  user_id bigint NOT NULL,
  sequence_no bigint NOT NULL,
  event_type varchar(64) NOT NULL,
  message_id varchar(64) NULL,
  tool_call_id varchar(64) NULL,
  payload_json json NOT NULL,
  created_at datetime(3) NOT NULL,
  UNIQUE KEY uk_agent_run_events_run_seq (run_id, sequence_no),
  KEY idx_agent_run_events_thread_created (thread_id, created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
