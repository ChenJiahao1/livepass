"""HTTP routes for agents service."""

from __future__ import annotations

from functools import lru_cache
from typing import Any

import redis
from fastapi import APIRouter, Depends, Header, HTTPException, status

from app.api.schemas import ChatRequest, ChatResponse
from app.config import get_settings
from app.graph import build_graph_app
from app.llm.client import build_chat_model
from app.mcp_client.registry import MCPToolRegistry
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
    return RedisCheckpointSaver.from_url(
        settings.redis_url,
        ttl_seconds=settings.session_ttl_seconds,
        key_prefix=f"{settings.session_key_prefix}:langgraph",
    )


@lru_cache(maxsize=1)
def get_graph():
    return build_graph_app(checkpointer=get_checkpointer())


@lru_cache(maxsize=1)
def get_tool_registry() -> MCPToolRegistry:
    return MCPToolRegistry()


@lru_cache(maxsize=1)
def get_llm():
    settings = get_settings()
    if not settings.openai_api_key:
        return None
    return build_chat_model(settings)


@router.post("/agent/chat", response_model=ChatResponse)
async def chat(
    request: ChatRequest,
    graph=Depends(get_graph),
    session_store: ConversationStateStore = Depends(get_session_store),
    registry: MCPToolRegistry = Depends(get_tool_registry),
    llm=Depends(get_llm),
    user_header: str | None = Header(default=None, alias="X-User-Id"),
) -> ChatResponse:
    if not user_header:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="missing X-User-Id")

    try:
        user_id = int(user_header)
    except ValueError as exc:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="invalid X-User-Id") from exc

    try:
        session = session_store.get_or_create(user_id=user_id, conversation_id=request.conversation_id)
    except SessionOwnershipError as exc:
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail=str(exc)) from exc

    result = await graph.ainvoke(
        {"messages": [{"role": "user", "content": request.message}]},
        config={"configurable": {"thread_id": session.conversation_id}},
        context={"registry": registry, "llm": llm, "current_user_id": str(user_id)},
    )

    final_reply = result.get("final_reply") or result.get("reply") or ""
    session_store.save(session)

    return ChatResponse(
        conversationId=session.conversation_id,
        reply=final_reply,
        status="handoff" if result.get("need_handoff") else "completed",
    )
