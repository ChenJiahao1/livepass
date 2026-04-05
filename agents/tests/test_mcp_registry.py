import asyncio
from types import SimpleNamespace

import pytest

from app.config import Settings
from app.mcp_client.registry import MCPToolRegistry
from app.mcp_client.tracing import trace_tool_calls
from langchain_mcp_adapters.interceptors import MCPToolCallRequest


def test_registry_points_order_toolset_to_go_provider():
    registry = MCPToolRegistry(
        settings=Settings(order_mcp_endpoint="http://127.0.0.1:9082/message")
    )

    assert registry.connections["order"]["transport"] == "streamable_http"
    assert registry.connections["order"]["url"] == "http://127.0.0.1:9082/message"
    assert registry.connections["order"]["headers"]["X-Internal-Caller"] == "agents"


def test_trace_tool_calls_appends_trace_entries():
    trace: list[str] = []
    request = MCPToolCallRequest(
        name="get_order_detail_for_service",
        args={"order_id": "93001"},
        server_name="order",
        headers=None,
        runtime=SimpleNamespace(context={"trace": trace}),
    )

    async def handler(_request):
        return {"ok": True}

    asyncio.run(trace_tool_calls(request, handler))

    assert trace == ["tool:get_order_detail_for_service"]


class _FakeTool:
    def __init__(self, name: str):
        self.name = name
        self.description = name

    async def ainvoke(self, payload: dict):
        return {"tool_name": self.name, "payload": payload}


class _FakeClient:
    def __init__(self):
        self.calls: list[str | None] = []
        self.tools = [_FakeTool("list_user_orders"), _FakeTool("preview_refund_order")]

    async def get_tools(self, server_name: str | None = None):
        self.calls.append(server_name)
        return list(self.tools)


@pytest.mark.anyio
async def test_registry_invokes_refund_tool_from_cached_provider_catalog():
    client = _FakeClient()
    registry = MCPToolRegistry(
        settings=Settings(order_mcp_endpoint="http://127.0.0.1:9082/message"),
        client=client,
    )

    result = await registry.invoke(
        server_name="order",
        tool_name="preview_refund_order",
        payload={"order_id": "ORD-10001"},
    )

    assert result == {
        "tool_name": "preview_refund_order",
        "payload": {"order_id": "ORD-10001"},
    }
    assert client.calls == ["order"]
