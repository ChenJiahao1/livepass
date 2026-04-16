CREATE TABLE IF NOT EXISTS agent_messages (
  id varchar(64) PRIMARY KEY,
  thread_id varchar(64) NOT NULL,
  user_id bigint NOT NULL,
  role varchar(32) NOT NULL,
  parts_json json NOT NULL,
  status varchar(32) NOT NULL,
  run_id varchar(64) NULL,
  created_at datetime(3) NOT NULL,
  metadata_json json NULL,
  KEY idx_agent_messages_thread_created (thread_id, created_at, id),
  KEY idx_agent_messages_user_thread (user_id, thread_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
