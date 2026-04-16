"""Request and response schemas for agents API."""

from __future__ import annotations

from datetime import datetime
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field


class ApiSchemaModel(BaseModel):
    model_config = ConfigDict(populate_by_name=True, extra="ignore")


class TextPartDTO(ApiSchemaModel):
    type: Literal["text"]
    text: str = Field(min_length=1)


class MessageInputDTO(ApiSchemaModel):
    role: Literal["user"]
    parts: list[TextPartDTO] = Field(min_length=1)


class ThreadDTO(ApiSchemaModel):
    id: str
    title: str
    status: str
    created_at: datetime = Field(alias="createdAt")
    updated_at: datetime = Field(alias="updatedAt")
    last_message_at: datetime | None = Field(default=None, alias="lastMessageAt")
    active_run_id: str | None = Field(default=None, alias="activeRunId")
    metadata: dict = Field(default_factory=dict)


class MessageDTO(ApiSchemaModel):
    id: str
    thread_id: str = Field(alias="threadId")
    role: Literal["user", "assistant"]
    parts: list[TextPartDTO] = Field(min_length=1)
    status: str
    created_at: datetime = Field(alias="createdAt")
    run_id: str | None = Field(default=None, alias="runId")
    metadata: dict = Field(default_factory=dict)


class RunErrorDTO(ApiSchemaModel):
    code: str
    message: str
    details: dict = Field(default_factory=dict)


class RunDTO(ApiSchemaModel):
    id: str
    thread_id: str = Field(alias="threadId")
    status: str
    trigger_message_id: str = Field(alias="triggerMessageId")
    output_message_ids: list[str] = Field(default_factory=list, alias="outputMessageIds")
    started_at: datetime = Field(alias="startedAt")
    completed_at: datetime | None = Field(default=None, alias="completedAt")
    error: RunErrorDTO | None = None
    metadata: dict = Field(default_factory=dict)


class ErrorDTO(ApiSchemaModel):
    code: str
    message: str
    details: dict = Field(default_factory=dict)


class CreateThreadRequest(ApiSchemaModel):
    title: str | None = None


class CreateThreadResponse(ApiSchemaModel):
    thread: ThreadDTO


class ListThreadsResponse(ApiSchemaModel):
    threads: list[ThreadDTO] = Field(default_factory=list)
    next_cursor: str | None = Field(default=None, alias="nextCursor")


class GetThreadResponse(ApiSchemaModel):
    thread: ThreadDTO


class RunInputMessageDTO(ApiSchemaModel):
    role: Literal["user"]
    parts: list[TextPartDTO] = Field(min_length=1)


class ListMessagesResponse(ApiSchemaModel):
    messages: list[MessageDTO] = Field(default_factory=list)
    next_cursor: str | None = Field(default=None, alias="nextCursor")


class CreateRunRequest(ApiSchemaModel):
    thread_id: str | None = Field(default=None, alias="threadId")
    messages: list[RunInputMessageDTO] = Field(min_length=1)


class CreateRunResponse(ApiSchemaModel):
    run_id: str = Field(alias="runId")
    thread_id: str = Field(alias="threadId")


class ResumeToolCallRequest(ApiSchemaModel):
    action: Literal["approve", "reject", "respond"]
    reason: str | None = None
    values: dict[str, Any] = Field(default_factory=dict)


class GetRunResponse(ApiSchemaModel):
    run: RunDTO


class PatchThreadRequest(ApiSchemaModel):
    title: str | None = None
    status: Literal["active", "archived"] | None = None


class PatchThreadResponse(ApiSchemaModel):
    thread: ThreadDTO
