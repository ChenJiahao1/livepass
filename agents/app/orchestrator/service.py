"""Chat service shell that binds sessions to orchestrator calls."""

from __future__ import annotations

from app.orchestrator.state import OrchestratorResult
from app.session.models import SessionMessage
from app.session.store import ConversationSessionStore


class StubOrchestrator:
    async def reply(self, session, *, message: str) -> OrchestratorResult:
        return OrchestratorResult(
            reply=f"已收到消息：{message}",
            status="queued",
            current_agent="stub",
        )


class ChatService:
    def __init__(self, *, session_store: ConversationSessionStore, orchestrator) -> None:
        self.session_store = session_store
        self.orchestrator = orchestrator

    async def handle_chat(
        self,
        *,
        user_id: int,
        message: str,
        conversation_id: str | None,
    ) -> dict[str, str]:
        session = self.session_store.get_or_create(
            user_id=user_id,
            conversation_id=conversation_id,
        )
        session.messages.append(SessionMessage(role="user", content=message))

        result = await self.orchestrator.reply(session, message=message)

        session.messages.append(SessionMessage(role="assistant", content=result.reply))
        session.current_agent = result.current_agent
        if result.need_handoff:
            session.handoff["requested"] = True
        self.session_store.save(session)

        return {
            "conversationId": session.conversation_id,
            "reply": result.reply,
            "status": result.status,
        }
