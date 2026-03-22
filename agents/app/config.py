"""Application configuration."""

from functools import lru_cache

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        env_ignore_empty=True,
        extra="ignore",
    )

    app_name: str = "damai-agents"
    openai_api_key: str | None = None
    openai_base_url: str | None = None
    openai_model: str = "gpt-4.1-mini"
    llm_timeout_seconds: float = 30.0
    lightrag_base_url: str = "http://127.0.0.1:9621"
    lightrag_api_key: str | None = None
    lightrag_timeout_seconds: float = 30.0
    max_tool_steps: int = 3
    redis_url: str = "redis://127.0.0.1:6379/0"
    session_ttl_seconds: int = 1800
    session_key_prefix: str = "agents:conversation"
    order_rpc_target: str = "127.0.0.1:8082"
    program_rpc_target: str = "127.0.0.1:8083"
    user_rpc_target: str = "127.0.0.1:8080"


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    return Settings()
