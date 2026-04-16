"""MCP tool registry backed by Go MCP servers."""

from __future__ import annotations

import json
from typing import Any

from langchain_mcp_adapters.client import MultiServerMCPClient

from app.config import Settings, get_settings

SUPPORTED_TOOLSETS = ("activity", "order", "refund")
TOOLSET_TOOL_NAMES = {
    "order": {"list_user_orders", "get_order_detail_for_service"},
    "refund": {"preview_refund_order", "refund_order"},
}
TOOLSET_PROVIDER = {
    "order": "order",
    "refund": "order",
    "activity": "activity",
}


class MCPToolRegistry:
    def __init__(
        self,
        *,
        settings: Settings | None = None,
        connections: dict | None = None,
        client: MultiServerMCPClient | None = None,
    ) -> None:
        self.settings = settings or get_settings()
        self.connections = connections or self._build_connections()
        self._client = client or MultiServerMCPClient(self.connections)
        self._cache: dict[str, list] = {}

    async def get_provider_tools(self, server_name: str) -> list:
        if server_name not in self._cache:
            self._cache[server_name] = await self._client.get_tools(server_name=server_name)
        return list(self._cache[server_name])

    async def get_tools(self, toolset: str) -> list:
        provider_name = TOOLSET_PROVIDER.get(toolset, toolset)
        tools = await self.get_provider_tools(provider_name)
        allowed_tool_names = TOOLSET_TOOL_NAMES.get(toolset)
        if not allowed_tool_names:
            return tools
        return [tool for tool in tools if tool.name in allowed_tool_names]

    async def invoke(self, *, server_name: str, tool_name: str, payload: dict[str, Any]) -> Any:
        tools = await self.get_provider_tools(server_name)
        tool_by_name = {tool.name: tool for tool in tools}
        tool = tool_by_name.get(tool_name)
        if tool is None:
            raise KeyError(f"tool not registered on server {server_name}: {tool_name}")
        result = await tool.ainvoke(payload)
        return self._normalize_result(result)

    def _build_connections(self) -> dict[str, dict[str, Any]]:
        return {
            "activity": {
                "transport": "streamable_http",
                "url": self.settings.activity_mcp_endpoint,
                "headers": {"X-Internal-Caller": "agents"},
            },
            "order": {
                "transport": "streamable_http",
                "url": self.settings.order_mcp_endpoint,
                "headers": {"X-Internal-Caller": "agents"},
            },
        }

    def _normalize_result(self, result: Any) -> Any:
        if isinstance(result, dict):
            return result
        if isinstance(result, str):
            return self._maybe_parse_json(result)
        if isinstance(result, list):
            text_parts: list[str] = []
            for item in result:
                if isinstance(item, dict) and item.get("type") == "text":
                    text_parts.append(str(item.get("text", "")))
            if text_parts:
                return self._maybe_parse_json("\n".join(text_parts))
        return result

    def _maybe_parse_json(self, text: str) -> Any:
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            return text
