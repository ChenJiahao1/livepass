"""MCP tool registry backed by Go MCP servers."""

from __future__ import annotations

import inspect
import json
from collections.abc import Callable
from typing import Any

from langchain_mcp_adapters.client import MultiServerMCPClient

from app.shared.config import Settings, get_settings
from app.shared.ids import new_tool_call_id
from app.integrations.mcp.execution_context import ToolExecutionContext
from app.integrations.mcp.interceptor import MCPToolInterceptor
from app.integrations.mcp.tool_policies import (
    SUPPORTED_TOOLSETS,
    TOOLSET_ACTIVITY,
    TOOLSET_ACCESS_POLICIES,
    TOOLSET_ORDER,
    TOOLSET_TOOL_NAMES,
    ToolAccessPolicy,
    get_tool_access_policy,
)

TOOLSET_PROVIDER = {
    TOOLSET_ORDER: TOOLSET_ORDER,
    TOOLSET_ACTIVITY: TOOLSET_ACTIVITY,
}


class MCPToolRegistry:
    def __init__(
        self,
        *,
        settings: Settings | None = None,
        connections: dict | None = None,
        client: MultiServerMCPClient | None = None,
        interceptor: MCPToolInterceptor | None = None,
    ) -> None:
        self.settings = settings or get_settings()
        self.connections = connections or self._build_connections()
        self._client = client or MultiServerMCPClient(self.connections)
        self._cache: dict[str, list] = {}
        self.interceptor = interceptor or MCPToolInterceptor()

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

    async def invoke(
        self,
        *,
        server_name: str,
        tool_name: str,
        payload: dict[str, Any],
        context: ToolExecutionContext | None = None,
    ) -> Any:
        tools = await self.get_provider_tools(server_name)
        tool_by_name = {tool.name: tool for tool in tools}
        tool = tool_by_name.get(tool_name)
        if context is None:
            if tool is None:
                raise KeyError(f"tool not registered on server {server_name}: {tool_name}")
            result = await tool.ainvoke(payload)
            return self._normalize_result(result)
        return await self.interceptor.invoke(
            server_name=server_name,
            tool_name=tool_name,
            payload=payload,
            context=context,
            tool=tool,
        )

    def bind_context(
        self,
        *,
        user_id: int,
        thread_id: str,
        run_id: str,
        channel_code: str | None = None,
        request_id: str | None = None,
        tool_call_id_factory: Callable[[], str] | None = None,
    ) -> "BoundMCPToolRegistry":
        return BoundMCPToolRegistry(
            registry=self,
            user_id=user_id,
            thread_id=thread_id,
            run_id=run_id,
            channel_code=channel_code,
            request_id=request_id,
            tool_call_id_factory=tool_call_id_factory or new_tool_call_id,
        )

    def _build_connections(self) -> dict[str, dict[str, Any]]:
        return {
            TOOLSET_ACTIVITY: {
                "transport": "streamable_http",
                "url": self.settings.activity_mcp_endpoint,
                "headers": {"X-Internal-Caller": "agents"},
            },
            TOOLSET_ORDER: {
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


class BoundMCPToolRegistry:
    def __init__(
        self,
        *,
        registry: MCPToolRegistry,
        user_id: int,
        thread_id: str,
        run_id: str,
        channel_code: str | None,
        request_id: str | None,
        tool_call_id_factory: Callable[[], str],
    ) -> None:
        self._registry = registry
        self._user_id = user_id
        self._thread_id = thread_id
        self._run_id = run_id
        self._channel_code = channel_code
        self._request_id = request_id
        self._tool_call_id_factory = tool_call_id_factory
        self.connections = registry.connections

    async def get_provider_tools(self, server_name: str) -> list:
        tools = await self._registry.get_provider_tools(server_name)
        return [self._wrap_tool(server_name=server_name, tool=tool) for tool in tools]

    async def get_tools(self, toolset: str) -> list:
        provider_name = TOOLSET_PROVIDER.get(toolset, toolset)
        tools = await self.get_provider_tools(provider_name)
        allowed_tool_names = TOOLSET_TOOL_NAMES.get(toolset)
        if not allowed_tool_names:
            return tools
        return [tool for tool in tools if tool.name in allowed_tool_names]

    async def invoke(self, *, server_name: str, tool_name: str, payload: dict[str, Any]) -> Any:
        return await self._registry.invoke(
            server_name=server_name,
            tool_name=tool_name,
            payload=payload,
            context=self._next_context(),
        )

    def _wrap_tool(self, *, server_name: str, tool: Any) -> Any:
        return _InterceptedTool(
            original_tool=tool,
            server_name=server_name,
            registry=self._registry,
            context_factory=self._next_context,
        )

    def _next_context(self) -> ToolExecutionContext:
        return ToolExecutionContext(
            user_id=self._user_id,
            thread_id=self._thread_id,
            run_id=self._run_id,
            tool_call_id=self._tool_call_id_factory(),
            channel_code=self._channel_code,
            request_id=self._request_id,
        )


class _InterceptedTool:
    def __init__(
        self,
        *,
        original_tool: Any,
        server_name: str,
        registry: MCPToolRegistry,
        context_factory: Callable[[], ToolExecutionContext],
    ) -> None:
        self._original_tool = original_tool
        self._server_name = server_name
        self._registry = registry
        self._context_factory = context_factory
        self.name = getattr(original_tool, "name", "unknown")
        self.description = getattr(original_tool, "description", self.name)
        self.args_schema = getattr(original_tool, "args_schema", None)
        if hasattr(original_tool, "__signature__"):
            self.__signature__ = getattr(original_tool, "__signature__")
        elif hasattr(original_tool, "ainvoke"):
            self.__signature__ = inspect.signature(original_tool.ainvoke)

    async def ainvoke(self, payload: dict[str, Any]) -> Any:
        return await self._registry.interceptor.invoke(
            server_name=self._server_name,
            tool_name=self.name,
            payload=payload,
            context=self._context_factory(),
            tool=self._original_tool,
        )

    async def __call__(self, *args: Any, **kwargs: Any) -> Any:
        if args and isinstance(args[0], dict) and not kwargs:
            payload = dict(args[0])
        elif kwargs:
            payload = dict(kwargs)
        else:
            payload = {}
        return await self.ainvoke(payload)

    def __getattr__(self, item: str) -> Any:
        return getattr(self._original_tool, item)
