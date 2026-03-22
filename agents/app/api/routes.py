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
from app.session.store import ConversationStateStore, SessionOwnershipError

router = APIRouter()


@lru_cache(maxsize=1)
def get_redis_client():
    settings = get_settings()
    return redis.Redis.from_url(settings.redis_url, decode_responses=True)


def get_state_store() -> ConversationStateStore:
    settings = get_settings()
    return ConversationStateStore(
        redis_client=get_redis_client(),
        ttl_seconds=settings.session_ttl_seconds,
        key_prefix=settings.session_key_prefix,
    )


@lru_cache(maxsize=1)
def get_graph():
    return build_graph_app()


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
    state_store: ConversationStateStore = Depends(get_state_store),
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
        session = state_store.get_or_create(user_id=user_id, conversation_id=request.conversation_id)
    except SessionOwnershipError as exc:
        raise HTTPException(status_code=status.HTTP_403_FORBIDDEN, detail=str(exc)) from exc

    state_payload = _append_message(session.state, {"role": "user", "content": request.message})
    result = await graph.ainvoke(
        state_payload,
        context={"registry": registry, "llm": llm, "current_user_id": str(user_id)},
    )

    final_reply = result.get("final_reply") or result.get("reply") or ""
    next_state = _normalize_state_for_storage(result)
    next_state["messages"] = _append_message(
        {"messages": next_state.get("messages", [])},
        {"role": "assistant", "content": final_reply},
    )["messages"]
    session.state = next_state
    state_store.save(session)

    return ChatResponse(
        conversationId=session.conversation_id,
        reply=final_reply,
        status="handoff" if result.get("need_handoff") else "completed",
    )


def _append_message(state: dict[str, Any], message: dict[str, str]) -> dict[str, Any]:
    payload = dict(state)
    messages = list(payload.get("messages", []))
    messages.append(message)
    payload["messages"] = messages
    return payload


def _normalize_state_for_storage(state: dict[str, Any]) -> dict[str, Any]:
    payload = dict(state)
    payload["messages"] = [_normalize_message(message) for message in payload.get("messages", [])]
    return payload


def _normalize_message(message: Any) -> dict[str, str]:
    if hasattr(message, "type") and hasattr(message, "content"):
        role = message.type
        if role == "human":
            role = "user"
        elif role == "ai":
            role = "assistant"
        return {"role": role, "content": str(message.content)}
    if isinstance(message, dict):
        return {
            "role": str(message.get("role", "")),
            "content": str(message.get("content", "")),
        }
    return {"role": "assistant", "content": str(message)}
