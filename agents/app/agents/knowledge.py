"""Knowledge specialist agent."""

from __future__ import annotations

from typing import Any

import httpx
from langchain_core.messages import AnyMessage

from app.config import Settings, get_settings
from app.state import ConversationState

REALTIME_KEYWORDS = ("最近", "最新", "新闻", "八卦", "热搜", "绯闻", "近况", "动态", "今天")


class KnowledgeAgent:
    def __init__(self, *, http_client: httpx.AsyncClient | None = None, settings: Settings | None = None) -> None:
        self.settings = settings or get_settings()
        self.base_url = self.settings.lightrag_base_url.rstrip("/")
        self.api_key = self.settings.lightrag_api_key
        self.timeout = self.settings.lightrag_timeout_seconds
        self.http_client = http_client or httpx.AsyncClient(timeout=self.timeout)

    async def handle(self, state: ConversationState) -> dict[str, Any]:
        query = self._latest_user_message(state)

        if self._is_out_of_scope_query(query):
            return self._reply(
                reply="当前仅支持明星基础百科问题，不支持实时新闻、八卦或最新动态查询。",
                trace="knowledge:out_of_scope",
                result_summary="知识问题超出基础百科范围",
            )
        if not self.api_key:
            return self._reply(
                reply="知识查询暂不可用，LightRAG API Key 尚未配置。",
                trace="knowledge:config_error",
                result_summary="LightRAG API Key 未配置",
            )

        try:
            response = await self.http_client.post(
                f"{self.base_url}/query",
                headers={
                    "X-API-Key": self.api_key,
                    "Accept": "application/json",
                },
                json={
                    "query": query,
                    "mode": "mix",
                    "response_type": "Bullet Points",
                    "include_references": False,
                },
                timeout=self.timeout,
            )
            response.raise_for_status()
        except httpx.TimeoutException:
            return self._reply(
                reply="当前无法获取明星基础百科结果，请稍后重试。",
                trace="knowledge:lightrag_timeout",
                result_summary="LightRAG 请求超时",
            )
        except httpx.HTTPError:
            return self._reply(
                reply="当前无法获取明星基础百科结果，请稍后重试。",
                trace="knowledge:lightrag_error",
                result_summary="LightRAG 请求失败",
            )

        try:
            body = response.json()
        except ValueError:
            return self._reply(
                reply="当前无法获取明星基础百科结果，请稍后重试。",
                trace="knowledge:lightrag_bad_json",
                result_summary="LightRAG 返回非 JSON",
            )

        if not isinstance(body, dict):
            return self._reply(
                reply="当前无法获取明星基础百科结果，请稍后重试。",
                trace="knowledge:lightrag_bad_json",
                result_summary="LightRAG 返回结构异常",
            )

        answer = str(body.get("response", "")).strip()
        if not answer:
            return self._reply(
                reply="当前无法获取明星基础百科结果，请稍后重试。",
                trace="knowledge:lightrag_empty",
                result_summary="LightRAG 返回空结果",
            )

        return self._reply(
            reply=answer,
            trace="knowledge:lightrag",
            result_summary="明星基础百科已返回",
        )

    def _is_out_of_scope_query(self, query: str) -> bool:
        return any(keyword in query for keyword in REALTIME_KEYWORDS)

    def _latest_user_message(self, state: ConversationState) -> str:
        messages = state.get("messages", [])
        for message in reversed(messages):
            if self._message_role(message) == "user":
                return self._message_content(message).strip()
        return ""

    def _message_role(self, message: AnyMessage | dict[str, Any]) -> str | None:
        if hasattr(message, "type"):
            if message.type == "human":
                return "user"
            if message.type == "ai":
                return "assistant"
        if isinstance(message, dict):
            return message.get("role") or message.get("type")
        return None

    def _message_content(self, message: AnyMessage | dict[str, Any]) -> str:
        if hasattr(message, "content"):
            return str(message.content)
        if isinstance(message, dict):
            return str(message.get("content", ""))
        return ""

    def _reply(self, *, reply: str, trace: str, result_summary: str) -> dict[str, Any]:
        return {
            "agent": "knowledge",
            "reply": reply,
            "trace": [trace],
            "need_handoff": False,
            "completed": True,
            "result_summary": result_summary,
        }
