CREATE TABLE IF NOT EXISTS agent_threads (
  id varchar(64) PRIMARY KEY,
  user_id bigint NOT NULL,
  title varchar(128) NOT NULL,
  status varchar(32) NOT NULL,
  created_at datetime(3) NOT NULL,
  updated_at datetime(3) NOT NULL,
  last_message_at datetime(3) NULL,
  metadata_json json NULL,
  KEY idx_agent_threads_user_status_last_message (user_id, status, last_message_at, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
