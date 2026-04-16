"""HTTP routes for agents service."""

from __future__ import annotations

from functools import lru_cache

import redis
from fastapi import APIRouter, Depends, Header, HTTPException, Query, status

from app.agent_runtime.service import AgentRuntimeService
from app.api.schemas import (
    CreateThreadRequest,
    CreateThreadResponse,
    GetRunResponse,
    GetThreadResponse,
    ListMessagesResponse,
    ListThreadsResponse,
    MessageDTO,
    PatchThreadRequest,
    PatchThreadResponse,
    RunDTO,
    RunErrorDTO,
    SendMessageRequest,
    SendMessageResponse,
    TextPartDTO,
    ThreadDTO,
)
from app.common.errors import ApiError, to_http_exception
from app.config import get_settings
from app.graph import build_graph_app
from app.llm.client import build_chat_model
from app.mcp_client.registry import MCPToolRegistry
from app.messages.repository import MessageRepository, MySQLMessageRepository
from app.messages.service import MessageService
from app.runs.repository import MySQLRunRepository, RunRepository
from app.runs.service import RunService
from app.session.checkpointer import RedisCheckpointSaver
from app.session.store import ThreadOwnershipStore
from app.threads.repository import MySQLConnectionFactory, MySQLThreadRepository, ThreadRepository
from app.threads.service import ThreadService

router = APIRouter()


@lru_cache(maxsize=1)
def get_redis_client():
    settings = get_settings()
    return redis.Redis.from_url(settings.redis_url, decode_responses=True)


@lru_cache(maxsize=1)
def get_checkpointer() -> RedisCheckpointSaver:
    settings = get_settings()
    return RedisCheckpointSaver(
        redis_client=get_redis_client(),
        ttl_seconds=settings.session_ttl_seconds,
        key_prefix=settings.checkpoint_key_prefix,
    )


@lru_cache(maxsize=1)
def get_agent_runtime():
    return build_graph_app(checkpointer=get_checkpointer())


@lru_cache(maxsize=1)
def get_tool_registry() -> MCPToolRegistry:
    return MCPToolRegistry()


@lru_cache(maxsize=1)
def get_llm():
    settings = get_settings()
    if not settings.openai_api_key:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE,
            detail="OPENAI_API_KEY is required for /agent/threads/*/messages",
        )
    return build_chat_model(settings)


def get_current_user_id(user_header: str | None = Header(default=None, alias="X-User-Id")) -> int:
    if not user_header:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="missing X-User-Id")
    try:
        return int(user_header)
    except ValueError as exc:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="invalid X-User-Id") from exc


def get_connection_factory() -> MySQLConnectionFactory:
    return MySQLConnectionFactory(get_settings())


def get_thread_repository() -> ThreadRepository:
    return MySQLThreadRepository(get_connection_factory())


def get_message_repository() -> MessageRepository:
    return MySQLMessageRepository(get_connection_factory())


def get_run_repository() -> RunRepository:
    return MySQLRunRepository(get_connection_factory())


def get_thread_ownership_store() -> ThreadOwnershipStore:
    settings = get_settings()
    return ThreadOwnershipStore(
        redis_client=get_redis_client(),
        ttl_seconds=settings.session_ttl_seconds,
        key_prefix=settings.session_key_prefix,
    )


def get_thread_service(
    thread_repository: ThreadRepository = Depends(get_thread_repository),
    ownership_store: ThreadOwnershipStore = Depends(get_thread_ownership_store),
) -> ThreadService:
    return ThreadService(
        thread_repository=thread_repository,
        ownership_store=ownership_store,
    )


def get_run_service(
    run_repository: RunRepository = Depends(get_run_repository),
    ownership_store: ThreadOwnershipStore = Depends(get_thread_ownership_store),
) -> RunService:
    return RunService(
        run_repository=run_repository,
        ownership_store=ownership_store,
    )


def get_runtime_service(
    agent_runtime=Depends(get_agent_runtime),
    registry: MCPToolRegistry = Depends(get_tool_registry),
    llm=Depends(get_llm),
) -> AgentRuntimeService:
    return AgentRuntimeService(
        agent_runtime=agent_runtime,
        registry=registry,
        llm=llm,
    )


def get_message_service(
    thread_repository: ThreadRepository = Depends(get_thread_repository),
    message_repository: MessageRepository = Depends(get_message_repository),
    run_service: RunService = Depends(get_run_service),
    runtime_service: AgentRuntimeService = Depends(get_runtime_service),
    ownership_store: ThreadOwnershipStore = Depends(get_thread_ownership_store),
) -> MessageService:
    return MessageService(
        thread_repository=thread_repository,
        message_repository=message_repository,
        run_service=run_service,
        runtime_service=runtime_service,
        ownership_store=ownership_store,
    )


def to_thread_dto(thread) -> ThreadDTO:
    return ThreadDTO(
        id=thread.id,
        title=thread.title,
        status=thread.status,
        createdAt=thread.created_at,
        updatedAt=thread.updated_at,
        lastMessageAt=thread.last_message_at,
        metadata=thread.metadata,
    )


def to_message_dto(message) -> MessageDTO:
    return MessageDTO(
        id=message.id,
        threadId=message.thread_id,
        role=message.role,
        parts=[TextPartDTO(**part) for part in message.parts],
        status=message.status,
        createdAt=message.created_at,
        runId=message.run_id,
        metadata=message.metadata,
    )


def to_run_dto(run) -> RunDTO:
    error = run.error
    return RunDTO(
        id=run.id,
        threadId=run.thread_id,
        status=run.status,
        triggerMessageId=run.trigger_message_id,
        outputMessageIds=list(run.metadata.get("outputMessageIds", [])),
        startedAt=run.started_at,
        completedAt=run.completed_at,
        error=RunErrorDTO(**error) if error else None,
        metadata=run.metadata,
    )


@router.post("/agent/threads", response_model=CreateThreadResponse)
async def create_thread(
    request: CreateThreadRequest,
    user_id: int = Depends(get_current_user_id),
    thread_service: ThreadService = Depends(get_thread_service),
) -> CreateThreadResponse:
    try:
        thread = thread_service.create_thread(user_id=user_id, title=request.title)
        return CreateThreadResponse(thread=to_thread_dto(thread))
    except ApiError as exc:
        raise to_http_exception(exc) from exc


@router.get("/agent/threads", response_model=ListThreadsResponse)
async def list_threads(
    user_id: int = Depends(get_current_user_id),
    thread_service: ThreadService = Depends(get_thread_service),
    limit: int = Query(default=20, ge=1, le=100),
    cursor: str | None = Query(default=None),
    status_value: str = Query(default="active", alias="status"),
) -> ListThreadsResponse:
    try:
        result = thread_service.list_threads(
            user_id=user_id,
            status=status_value,
            limit=limit,
            cursor=cursor,
            include_empty=False,
        )
        return ListThreadsResponse(
            threads=[to_thread_dto(thread) for thread in result.threads],
            nextCursor=result.next_cursor,
        )
    except ApiError as exc:
        raise to_http_exception(exc) from exc


@router.get("/agent/threads/{thread_id}", response_model=GetThreadResponse)
async def get_thread(
    thread_id: str,
    user_id: int = Depends(get_current_user_id),
    thread_service: ThreadService = Depends(get_thread_service),
) -> GetThreadResponse:
    try:
        thread = thread_service.get_thread(user_id=user_id, thread_id=thread_id)
        return GetThreadResponse(thread=to_thread_dto(thread))
    except ApiError as exc:
        raise to_http_exception(exc) from exc


@router.patch("/agent/threads/{thread_id}", response_model=PatchThreadResponse)
async def patch_thread(
    thread_id: str,
    request: PatchThreadRequest,
    user_id: int = Depends(get_current_user_id),
    thread_service: ThreadService = Depends(get_thread_service),
) -> PatchThreadResponse:
    try:
        thread = thread_service.patch_thread(
            user_id=user_id,
            thread_id=thread_id,
            title=request.title,
            status=request.status,
        )
        return PatchThreadResponse(thread=to_thread_dto(thread))
    except ApiError as exc:
        raise to_http_exception(exc) from exc


@router.get("/agent/threads/{thread_id}/messages", response_model=ListMessagesResponse)
async def list_messages(
    thread_id: str,
    user_id: int = Depends(get_current_user_id),
    message_service: MessageService = Depends(get_message_service),
    limit: int = Query(default=20, ge=1, le=100),
    before: str | None = Query(default=None),
) -> ListMessagesResponse:
    try:
        messages, next_cursor = message_service.list_messages(
            user_id=user_id,
            thread_id=thread_id,
            limit=limit,
            before=before,
        )
        return ListMessagesResponse(
            messages=[to_message_dto(message) for message in messages],
            nextCursor=next_cursor,
        )
    except ApiError as exc:
        raise to_http_exception(exc) from exc


@router.post("/agent/threads/{thread_id}/messages", response_model=SendMessageResponse)
async def send_message(
    thread_id: str,
    request: SendMessageRequest,
    user_id: int = Depends(get_current_user_id),
    message_service: MessageService = Depends(get_message_service),
) -> SendMessageResponse:
    try:
        result = await message_service.send_user_message(
            user_id=user_id,
            thread_id=thread_id,
            parts=[part.model_dump() for part in request.message.parts],
        )
        return SendMessageResponse(
            run=to_run_dto(result.run),
            messages=[to_message_dto(message) for message in result.messages],
            thread=to_thread_dto(result.thread),
        )
    except ApiError as exc:
        raise to_http_exception(exc) from exc


@router.get("/agent/threads/{thread_id}/runs/{run_id}", response_model=GetRunResponse)
async def get_run(
    thread_id: str,
    run_id: str,
    user_id: int = Depends(get_current_user_id),
    run_service: RunService = Depends(get_run_service),
) -> GetRunResponse:
    try:
        run = run_service.get_run(user_id=user_id, thread_id=thread_id, run_id=run_id)
        return GetRunResponse(run=to_run_dto(run))
    except ApiError as exc:
        raise to_http_exception(exc) from exc
