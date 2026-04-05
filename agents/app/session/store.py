"""Redis-backed conversation ownership store."""

from __future__ import annotations

from uuid import uuid4

from pydantic import BaseModel, ConfigDict, Field


class SessionOwnershipError(ValueError):
    """Raised when a conversation is accessed by another user."""

class StoredConversation(BaseModel):
    model_config = ConfigDict(populate_by_name=True)

    conversation_id: str = Field(alias="conversationId")
    user_id: int = Field(alias="userId")
    selected_order_id: str | None = Field(default=None, alias="selectedOrderId")
    recent_order_candidates: list[dict] = Field(default_factory=list, alias="recentOrderCandidates")
    last_task_summary: str | None = Field(default=None, alias="lastTaskSummary")
    last_handoff_ticket_id: str | None = Field(default=None, alias="lastHandoffTicketId")


class ConversationStateStore:
    def __init__(
        self,
        *,
        redis_client,
        ttl_seconds: int,
        key_prefix: str = "agents:conversation",
    ) -> None:
        self.redis_client = redis_client
        self.ttl_seconds = ttl_seconds
        self.key_prefix = key_prefix

    def key_for(self, conversation_id: str) -> str:
        return f"{self.key_prefix}:{conversation_id}"

    def get_or_create(self, *, user_id: int, conversation_id: str | None) -> StoredConversation:
        if not conversation_id:
            return StoredConversation(conversationId=uuid4().hex, userId=user_id)

        payload = self.redis_client.get(self.key_for(conversation_id))
        if not payload:
            return StoredConversation(conversationId=conversation_id, userId=user_id)

        session = StoredConversation.model_validate_json(payload)
        if session.user_id != user_id:
            raise SessionOwnershipError(f"conversation {conversation_id} does not belong to user {user_id}")

        return session

    def save(self, session: StoredConversation) -> None:
        key = self.key_for(session.conversation_id)
        self.redis_client.set(key, session.model_dump_json(by_alias=True))
        self.redis_client.expire(key, self.ttl_seconds)
