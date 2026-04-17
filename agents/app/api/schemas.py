"""Request and response schemas for agents API."""

from __future__ import annotations

from datetime import datetime
from typing import Any, Literal, TypeAlias

from pydantic import BaseModel, ConfigDict, Field


class ApiSchemaModel(BaseModel):
    model_config = ConfigDict(populate_by_name=True, extra="ignore")


class TextContentDTO(ApiSchemaModel):
    type: Literal["text"]
    text: str = Field(min_length=1)


MessageContentDTO: TypeAlias = TextContentDTO


class RunInputDTO(ApiSchemaModel):
    content: list[MessageContentDTO] = Field(min_length=1)


class ThreadDTO(ApiSchemaModel):
    id: str
    title: str
    status: str
    created_at: datetime = Field(alias="createdAt")
    updated_at: datetime = Field(alias="updatedAt")
    last_message_at: datetime | None = Field(default=None, alias="lastMessageAt")
    active_run_id: str | None = Field(default=None, alias="activeRunId")
    metadata: dict[str, Any] = Field(default_factory=dict)


class MessageDTO(ApiSchemaModel):
    id: str
    thread_id: str = Field(alias="threadId")
    role: Literal["user", "assistant"]
    content: list[MessageContentDTO] = Field(default_factory=list)
    status: str
    created_at: datetime = Field(alias="createdAt")
    updated_at: datetime = Field(alias="updatedAt")
    run_id: str | None = Field(default=None, alias="runId")
    metadata: dict[str, Any] = Field(default_factory=dict)


class RunErrorDTO(ApiSchemaModel):
    code: str
    message: str
    details: dict[str, Any] = Field(default_factory=dict)


class RunDTO(ApiSchemaModel):
    id: str
    thread_id: str = Field(alias="threadId")
    status: str
    trigger_message_id: str = Field(alias="triggerMessageId")
    output_message_id: str = Field(alias="outputMessageId")
    started_at: datetime = Field(alias="startedAt")
    completed_at: datetime | None = Field(default=None, alias="completedAt")
    error: RunErrorDTO | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)


class ErrorDTO(ApiSchemaModel):
    code: str
    message: str
    details: dict[str, Any] = Field(default_factory=dict)


class CreateThreadRequest(ApiSchemaModel):
    title: str | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)


class CreateThreadResponse(ApiSchemaModel):
    thread: ThreadDTO


class ListThreadsResponse(ApiSchemaModel):
    threads: list[ThreadDTO] = Field(default_factory=list)
    next_cursor: str | None = Field(default=None, alias="nextCursor")


class GetThreadResponse(ApiSchemaModel):
    thread: ThreadDTO


class ListThreadMessagesResponse(ApiSchemaModel):
    messages: list[MessageDTO] = Field(default_factory=list)
    next_cursor: str | None = Field(default=None, alias="nextCursor")


class CreateRunRequest(ApiSchemaModel):
    thread_id: str = Field(alias="threadId")
    input: RunInputDTO
    metadata: dict[str, Any] = Field(default_factory=dict)


class CreateRunResponse(ApiSchemaModel):
    thread: ThreadDTO
    run: RunDTO
    input_message: MessageDTO = Field(alias="inputMessage")
    output_message: MessageDTO = Field(alias="outputMessage")


ResumeToolCallAction: TypeAlias = Literal["approve", "reject", "edit"]


class ResumeToolCallRequest(ApiSchemaModel):
    action: ResumeToolCallAction
    reason: str | None = None
    values: dict[str, Any] = Field(default_factory=dict)


class HumanRequestDTO(ApiSchemaModel):
    kind: Literal["approval", "input"]
    title: str
    description: str | None = None
    allowed_actions: list[ResumeToolCallAction] = Field(default_factory=list, alias="allowedActions")


class ToolCallDTO(ApiSchemaModel):
    id: str
    run_id: str = Field(alias="runId")
    thread_id: str = Field(alias="threadId")
    message_id: str | None = Field(default=None, alias="messageId")
    name: str
    status: str
    input: dict[str, Any] = Field(default_factory=dict)
    output: dict[str, Any] | None = None
    error: dict[str, Any] | None = None
    human_request: HumanRequestDTO | None = Field(default=None, alias="humanRequest")
    metadata: dict[str, Any] = Field(default_factory=dict)
    created_at: datetime | None = Field(default=None, alias="createdAt")
    updated_at: datetime | None = Field(default=None, alias="updatedAt")
    completed_at: datetime | None = Field(default=None, alias="completedAt")


class GetRunResponse(ApiSchemaModel):
    run: RunDTO
    output_message: MessageDTO | None = Field(default=None, alias="outputMessage")
    active_tool_call: ToolCallDTO | None = Field(default=None, alias="activeToolCall")


class UpdateThreadRequest(ApiSchemaModel):
    title: str | None = None
    status: Literal["active", "archived"] | None = None


class UpdateThreadResponse(ApiSchemaModel):
    thread: ThreadDTO


PatchThreadRequest = UpdateThreadRequest
PatchThreadResponse = UpdateThreadResponse
ListMessagesResponse = ListThreadMessagesResponse
