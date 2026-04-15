"""HTTP routes for agents service."""

from __future__ import annotations

import json
from collections.abc import AsyncIterator
from functools import lru_cache

import redis
from fastapi import APIRouter, Depends, Header, HTTPException, status
from fastapi.responses import StreamingResponse

from app.api.schemas import ChatRequest, ChatResponse
from app.config import get_settings
from app.graph import build_graph_app
from app.llm.client import build_chat_model
from app.mcp_client.registry import MCPToolRegistry
from app.observability.audit import build_audit_record
from app.observability.tracing import build_trace_record
from app.session.checkpointer import RedisCheckpointSaver
from app.session.store import ConversationStateStore, SessionOwnershipError

router = APIRouter()


@lru_cache(maxsize=1)
def get_redis_client():
    settings = get_settings()
    return redis.Redis.from_url(settings.redis_url, decode_responses=True)


def get_session_store() -> ConversationStateStore:
    settings = get_settings()
    return ConversationStateStore(
        redis_client=get_redis_client(),
        ttl_seconds=settings.session_ttl_seconds,
        key_prefix=settings.session_key_prefix,
    )


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
            detail="OPENAI_API_KEY is required for /agent/chat",
        )
    return build_chat_model(settings)


def get_current_user_id(user_header: str | None = Header(default=None, alias="X-User-Id")) -> int:
    if not user_header:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="missing X-User-Id")

    try:
        return int(user_header)
    except ValueError as exc:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="invalid X-User-Id") from exc


def get_request_llm(
    _user_id: int = Depends(get_current_user_id),
    llm=Depends(get_llm),
):
    return llm


async def run_chat_turn(
    *,
    request: ChatRequest,
    agent_runtime,
    session_store: ConversationStateStore,
    registry: MCPToolRegistry,
    user_id: int,
    llm,
) -> tuple[str, str, str]:
    try:
        session = session_store.get_or_create(user_id=user_id, conversation_id=request.conversation_id)
    except SessionOwnershipError as exc:
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail=str(exc)) from exc

    result = await agent_runtime.ainvoke(
        {"messages": [{"role": "user", "content": request.message}]},
        config={"configurable": {"thread_id": session.conversation_id}},
        context={
            "registry": registry,
            "llm": llm,
            "current_user_id": str(user_id),
        },
    )
    final_reply = result.get("final_reply") or result.get("reply") or ""
    session_store.save(session)
    _ = build_trace_record(
        route_source=result.get("route_source", "rule"),
        result=result,
        session=session,
    )
    _ = build_audit_record(result=result)

    response_status = "handoff" if result.get("need_handoff") else str(result.get("status") or "completed")
    return session.conversation_id, final_reply, response_status


@router.post("/agent/chat", response_model=ChatResponse)
async def chat(
    request: ChatRequest,
    agent_runtime=Depends(get_agent_runtime),
    session_store: ConversationStateStore = Depends(get_session_store),
    registry: MCPToolRegistry = Depends(get_tool_registry),
    user_id: int = Depends(get_current_user_id),
    llm=Depends(get_request_llm),
) -> ChatResponse:
    conversation_id, final_reply, response_status = await run_chat_turn(
        request=request,
        agent_runtime=agent_runtime,
        session_store=session_store,
        registry=registry,
        user_id=user_id,
        llm=llm,
    )

    return ChatResponse(
        conversationId=conversation_id,
        reply=final_reply,
        status=response_status,
    )


@router.post("/agent/chat/stream")
async def chat_stream(
    request: ChatRequest,
    agent_runtime=Depends(get_agent_runtime),
    session_store: ConversationStateStore = Depends(get_session_store),
    registry: MCPToolRegistry = Depends(get_tool_registry),
    user_id: int = Depends(get_current_user_id),
    llm=Depends(get_request_llm),
) -> StreamingResponse:
    try:
        session = session_store.get_or_create(user_id=user_id, conversation_id=request.conversation_id)
    except SessionOwnershipError as exc:
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail=str(exc)) from exc

    async def event_stream() -> AsyncIterator[str]:
        yield format_sse("meta", {"conversationId": session.conversation_id})
        try:
            result = await agent_runtime.ainvoke(
                {"messages": [{"role": "user", "content": request.message}]},
                config={"configurable": {"thread_id": session.conversation_id}},
                context={
                    "registry": registry,
                    "llm": llm,
                    "current_user_id": str(user_id),
                },
            )
            final_reply = result.get("final_reply") or result.get("reply") or ""
            session_store.save(session)
            _ = build_trace_record(
                route_source=result.get("route_source", "rule"),
                result=result,
                session=session,
            )
            _ = build_audit_record(result=result)

            for chunk in split_stream_text(final_reply):
                yield format_sse("delta", {"text": chunk})

            response_status = "handoff" if result.get("need_handoff") else str(result.get("status") or "completed")
            yield format_sse("done", {"status": response_status})
        except Exception as exc:
            yield format_sse("error", {"message": str(exc)})

    return StreamingResponse(
        event_stream(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive",
            "X-Accel-Buffering": "no",
        },
    )


def format_sse(event: str, data: dict) -> str:
    payload = json.dumps(data, ensure_ascii=False)
    return f"event: {event}\ndata: {payload}\n\n"


def split_stream_text(text: str, *, chunk_size: int = 12) -> list[str]:
    if not text:
        return []
    return [text[index : index + chunk_size] for index in range(0, len(text), chunk_size)]
