"""MCP tool registry backed by local stdio servers."""

from __future__ import annotations

from pathlib import Path

from langchain_mcp_adapters.client import MultiServerMCPClient

from app.mcp_client.tracing import trace_tool_calls

PROJECT_ROOT = Path(__file__).resolve().parents[2]
SUPPORTED_TOOLSETS = ("activity", "order", "refund", "handoff")


class MCPToolRegistry:
    def __init__(self, *, connections: dict | None = None) -> None:
        self.connections = connections or {
            toolset: {
                "transport": "stdio",
                "command": "uv",
                "args": ["run", "damai-mcp-server", "--toolset", toolset],
                "cwd": str(PROJECT_ROOT),
            }
            for toolset in SUPPORTED_TOOLSETS
        }
        self._client = MultiServerMCPClient(
            self.connections,
            tool_interceptors=[trace_tool_calls],
        )
        self._cache: dict[str, list] = {}

    async def get_tools(self, toolset: str) -> list:
        if toolset not in self._cache:
            self._cache[toolset] = await self._client.get_tools(server_name=toolset)
        return list(self._cache[toolset])
