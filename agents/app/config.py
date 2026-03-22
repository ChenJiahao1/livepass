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
    redis_url: str = "redis://127.0.0.1:6379/0"
    session_ttl_seconds: int = 1800
    session_key_prefix: str = "agents:conversation"
    order_rpc_target: str = "127.0.0.1:8083"
    program_rpc_target: str = "127.0.0.1:8082"
    user_rpc_target: str = "127.0.0.1:8081"


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    return Settings()
