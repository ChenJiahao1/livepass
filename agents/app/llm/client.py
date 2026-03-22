"""LLM client factory."""

from langchain_openai import ChatOpenAI

from app.config import Settings, get_settings


def build_chat_model(settings: Settings | None = None) -> ChatOpenAI:
    settings = settings or get_settings()

    kwargs: dict[str, object] = {
        "model": settings.openai_model,
        "timeout": settings.llm_timeout_seconds,
    }
    if settings.openai_api_key:
        kwargs["api_key"] = settings.openai_api_key
    if settings.openai_base_url:
        kwargs["base_url"] = settings.openai_base_url

    return ChatOpenAI(**kwargs)
