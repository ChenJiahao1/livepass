"""Dependency providers for the agents HTTP API."""

from __future__ import annotations

from functools import lru_cache

import redis
from fastapi import Depends, Header, HTTPException, status

from app.agents.llm import build_chat_model
from app.conversations.messages.repository import MessageRepository, MySQLMessageRepository
from app.conversations.messages.service import MessageService
from app.conversations.threads.repository import MySQLThreadRepository, ThreadRepository
from app.conversations.threads.service import ThreadService
from app.graph import build_graph_app
from app.integrations.mcp.registry import MCPToolRegistry
from app.integrations.storage.mysql import MySQLConnectionFactory
from app.integrations.storage.redis import RedisCheckpointSaver
from app.runs.event_store import MySQLRunEventStore, RunEventStore
from app.runs.execution.event_bus import RunEventBus
from app.runs.execution.executor import RunExecutor
from app.runs.execution.runtime import AgentRuntimeService, RunService
from app.runs.execution.stream import RunStreamService
from app.runs.repository import MySQLRunRepository, RunRepository
from app.runs.tool_call_repository import MySQLToolCallRepository, ToolCallRepository
from app.shared.config import get_settings


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
            detail="OPENAI_API_KEY is required for /agent/runs",
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


@lru_cache(maxsize=1)
def get_event_bus() -> RunEventBus:
    return RunEventBus()


def get_event_store() -> RunEventStore:
    return MySQLRunEventStore(get_connection_factory())


def get_tool_call_repository() -> ToolCallRepository:
    return MySQLToolCallRepository(get_connection_factory())


def get_run_repository() -> RunRepository:
    return MySQLRunRepository(get_connection_factory())


def get_thread_service(
    thread_repository: ThreadRepository = Depends(get_thread_repository),
    run_repository: RunRepository = Depends(get_run_repository),
) -> ThreadService:
    return ThreadService(thread_repository=thread_repository, run_repository=run_repository)


def get_message_service(
    thread_repository: ThreadRepository = Depends(get_thread_repository),
    message_repository: MessageRepository = Depends(get_message_repository),
) -> MessageService:
    return MessageService(thread_repository=thread_repository, message_repository=message_repository)


def get_run_service(
    run_repository: RunRepository = Depends(get_run_repository),
    message_service: MessageService = Depends(get_message_service),
) -> RunService:
    return RunService(run_repository=run_repository, message_service=message_service)


def get_runtime_service(
    agent_runtime=Depends(get_agent_runtime),
    registry: MCPToolRegistry = Depends(get_tool_registry),
    llm=Depends(get_llm),
) -> AgentRuntimeService:
    return AgentRuntimeService(agent_runtime=agent_runtime, registry=registry, llm=llm)


def get_run_executor(
    run_repository: RunRepository = Depends(get_run_repository),
    run_service: RunService = Depends(get_run_service),
    message_service: MessageService = Depends(get_message_service),
    event_store: RunEventStore = Depends(get_event_store),
    event_bus: RunEventBus = Depends(get_event_bus),
    tool_call_repository: ToolCallRepository = Depends(get_tool_call_repository),
    runtime_service: AgentRuntimeService = Depends(get_runtime_service),
) -> RunExecutor:
    return RunExecutor(
        run_repository=run_repository,
        run_service=run_service,
        message_service=message_service,
        event_store=event_store,
        event_bus=event_bus,
        tool_call_repository=tool_call_repository,
        runtime_service=runtime_service,
    )


def get_run_stream_service(
    event_store: RunEventStore = Depends(get_event_store),
    event_bus: RunEventBus = Depends(get_event_bus),
) -> RunStreamService:
    return RunStreamService(event_store=event_store, event_bus=event_bus)
