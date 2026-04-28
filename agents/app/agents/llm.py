"""Structured outputs and chat model factory for agents."""

from typing import Literal

from langchain_openai import ChatOpenAI
from pydantic import BaseModel

from app.shared.config import Settings, get_settings
from app.shared.runtime_constants import AGENT_ACTIVITY, AGENT_ORDER, INTENT_UNKNOWN, NEXT_AGENT_FINISH


class CoordinatorDecision(BaseModel):
    action: Literal["respond", "delegate"]
    reply: str = ""
    route: Literal[AGENT_ACTIVITY, AGENT_ORDER, INTENT_UNKNOWN] | None = None
    selected_order_id: str | None = None
    selected_program_id: str | None = None
    reason: str = ""


class SupervisorDecision(BaseModel):
    next_agent: Literal[AGENT_ACTIVITY, AGENT_ORDER, NEXT_AGENT_FINISH]
    reason: str = ""


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
