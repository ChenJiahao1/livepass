"""Knowledge specialist agent."""

from __future__ import annotations

from typing import Any

from app.knowledge.service import KnowledgeService
from app.state import ConversationState


class KnowledgeAgent:
    def __init__(self, *, service: KnowledgeService | None = None) -> None:
        self.service = service or KnowledgeService()

    async def handle(self, state: ConversationState) -> dict[str, Any]:
        result = await self.service.handle(dict(state))
        return {
            **result,
            "current_agent": "knowledge",
            "final_reply": result["reply"],
            "status": "completed",
        }
