import asyncio
from types import SimpleNamespace

from app.mcp_client.registry import MCPToolRegistry
from app.mcp_client.tracing import trace_tool_calls
from langchain_mcp_adapters.interceptors import MCPToolCallRequest


def test_registry_loads_order_toolset_via_stdio():
    registry = MCPToolRegistry()

    tools = asyncio.run(registry.get_tools("order"))

    assert any(tool.name == "get_order_detail_for_service" for tool in tools)


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
