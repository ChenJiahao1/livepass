"""HTTP routes for agents service."""

from functools import lru_cache

import redis
from fastapi import APIRouter, Depends, Header, HTTPException, status

from app.api.schemas import ChatRequest, ChatResponse
from app.config import get_settings
from app.orchestrator.service import ChatService, StubOrchestrator
from app.session.store import ConversationSessionStore

router = APIRouter()


@lru_cache(maxsize=1)
def get_redis_client():
    settings = get_settings()
    return redis.Redis.from_url(settings.redis_url, decode_responses=True)


def get_chat_service() -> ChatService:
    settings = get_settings()
    session_store = ConversationSessionStore(
        redis_client=get_redis_client(),
        ttl_seconds=settings.session_ttl_seconds,
        key_prefix=settings.session_key_prefix,
    )
    return ChatService(
        session_store=session_store,
        orchestrator=StubOrchestrator(),
    )


@router.post("/agent/chat", response_model=ChatResponse)
async def chat(
    request: ChatRequest,
    chat_service: ChatService = Depends(get_chat_service),
    user_header: str | None = Header(default=None, alias="X-User-Id"),
) -> ChatResponse:
    if not user_header:
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="missing X-User-Id")

    try:
        user_id = int(user_header)
    except ValueError as exc:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="invalid X-User-Id") from exc

    payload = await chat_service.handle_chat(
        user_id=user_id,
        message=request.message,
        conversation_id=request.conversation_id,
    )
    return ChatResponse.model_validate(payload)
