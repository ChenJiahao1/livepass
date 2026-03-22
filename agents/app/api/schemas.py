"""Request and response schemas for agents API."""

from pydantic import BaseModel, ConfigDict, Field


class ChatRequest(BaseModel):
    model_config = ConfigDict(populate_by_name=True)

    message: str = Field(min_length=1)
    conversation_id: str | None = Field(default=None, alias="conversationId")


class ChatResponse(BaseModel):
    model_config = ConfigDict(populate_by_name=True)

    conversation_id: str = Field(alias="conversationId")
    reply: str
    status: str
