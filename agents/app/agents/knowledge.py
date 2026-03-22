"""Knowledge specialist agent."""

from __future__ import annotations

from typing import Any

import httpx

from app.config import Settings, get_settings
from app.state import ConversationState

REALTIME_KEYWORDS = ("最近", "实时", "新闻", "最新", "今天")


class KnowledgeAgent:
    def __init__(self, *, http_client: httpx.AsyncClient | None = None, settings: Settings | None = None) -> None:
        self.settings = settings or get_settings()
        self.http_client = http_client or httpx.AsyncClient(
            base_url=self.settings.lightrag_base_url,
            timeout=self.settings.lightrag_timeout_seconds,
        )

    async def handle(self, state: ConversationState) -> dict[str, Any]:
        question = self._latest_user_message(state)
        if any(keyword in question for keyword in REALTIME_KEYWORDS):
            reply = "当前知识库不支持实时新闻或最新动态查询。"
            return self._result(reply=reply, specialist_result={"question": question})

        payload = {"query": question}
        headers = {}
        if self.settings.lightrag_api_key:
            headers["Authorization"] = f"Bearer {self.settings.lightrag_api_key}"
        response = await self.http_client.post("/query", json=payload, headers=headers)
        response.raise_for_status()
        body = response.json()
        answer = body.get("answer") or body.get("response") or "暂时没有查询到相关知识。"
        return self._result(reply=answer, specialist_result=body)

    def _latest_user_message(self, state: ConversationState) -> str:
        messages = state.get("messages", [])
        for message in reversed(messages):
            role = getattr(message, "type", None)
            if role is None and hasattr(message, "get"):
                role = message.get("role")
            if role in {"human", "user"}:
                if hasattr(message, "content"):
                    return str(message.content)
                return str(message.get("content", ""))
        return ""

    def _result(self, *, reply: str, specialist_result: dict[str, Any] | None = None) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "reply": reply,
            "final_reply": reply,
            "current_agent": "knowledge",
            "status": "completed",
            "need_handoff": False,
        }
        if specialist_result is not None:
            payload["specialist_result"] = specialist_result
        return payload
