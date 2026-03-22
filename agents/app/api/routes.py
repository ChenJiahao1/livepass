"""HTTP routes for agents service."""

from fastapi import APIRouter, Depends, Header, HTTPException, status

from app.api.schemas import ChatRequest, ChatResponse

router = APIRouter()


class FakeChatService:
    async def handle_chat(
        self,
        *,
        user_id: int,
        message: str,
        conversation_id: str | None,
    ) -> dict[str, str]:
        return {
            "conversationId": conversation_id or f"conv-{user_id}",
            "reply": f"已收到消息：{message}",
            "status": "queued",
        }


def get_chat_service() -> FakeChatService:
    return FakeChatService()


@router.post("/agent/chat", response_model=ChatResponse)
async def chat(
    request: ChatRequest,
    chat_service: FakeChatService = Depends(get_chat_service),
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
