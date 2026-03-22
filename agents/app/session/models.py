"""Session models for agents chat."""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, ConfigDict, Field


class SessionMessage(BaseModel):
    role: str
    content: str


class ConversationSession(BaseModel):
    model_config = ConfigDict(populate_by_name=True)

    conversation_id: str = Field(alias="conversationId")
    user_id: int = Field(alias="userId")
    messages: list[SessionMessage] = Field(default_factory=list)
    summary: str | None = None
    slots: dict[str, Any] = Field(default_factory=dict)
    handoff: dict[str, Any] = Field(default_factory=dict)
    current_agent: str | None = Field(default=None, alias="currentAgent")
