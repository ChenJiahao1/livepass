import pytest

from app.shared.config import Settings
from app.integrations.mcp.execution_context import ToolExecutionContext
from app.integrations.mcp.registry import MCPToolRegistry


def test_registry_points_order_toolset_to_go_provider():
    registry = MCPToolRegistry(
        settings=Settings(
            activity_mcp_endpoint="http://127.0.0.1:9083/message",
            order_mcp_endpoint="http://127.0.0.1:9082/message",
        )
    )

    assert registry.connections["activity"]["transport"] == "streamable_http"
    assert registry.connections["activity"]["url"] == "http://127.0.0.1:9083/message"
    assert registry.connections["activity"]["headers"]["X-Internal-Caller"] == "agents"
    assert registry.connections["order"]["transport"] == "streamable_http"
    assert registry.connections["order"]["url"] == "http://127.0.0.1:9082/message"
    assert registry.connections["order"]["headers"]["X-Internal-Caller"] == "agents"
    assert "handoff" not in registry.connections


class _FakeTool:
    def __init__(self, name: str):
        self.name = name
        self.description = name
        self.calls: list[dict] = []

    async def ainvoke(self, payload: dict):
        self.calls.append(dict(payload))
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
        settings=Settings(
            activity_mcp_endpoint="http://127.0.0.1:9083/message",
            order_mcp_endpoint="http://127.0.0.1:9082/message",
        ),
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


@pytest.mark.anyio
async def test_registry_invoke_routes_through_bound_execution_context():
    client = _FakeClient()
    registry = MCPToolRegistry(
        settings=Settings(
            activity_mcp_endpoint="http://127.0.0.1:9083/message",
            order_mcp_endpoint="http://127.0.0.1:9082/message",
        ),
        client=client,
    )

    result = await registry.invoke(
        server_name="order",
        tool_name="preview_refund_order",
        payload={"order_id": "ORD-10001"},
        context=ToolExecutionContext(
            user_id=3001,
            thread_id="thr_001",
            run_id="run_001",
            tool_call_id="tool_001",
        ),
    )

    assert result == {
        "tool_name": "preview_refund_order",
        "payload": {
            "order_id": "ORD-10001",
        },
    }
    preview_tool = next(tool for tool in client.tools if tool.name == "preview_refund_order")
    assert preview_tool.calls == [{"order_id": "ORD-10001"}]


@pytest.mark.anyio
async def test_bound_registry_wraps_tools_with_runtime_context():
    client = _FakeClient()
    registry = MCPToolRegistry(
        settings=Settings(
            activity_mcp_endpoint="http://127.0.0.1:9083/message",
            order_mcp_endpoint="http://127.0.0.1:9082/message",
        ),
        client=client,
    )
    bound_registry = registry.bind_context(
        user_id=3001,
        thread_id="thr_001",
        run_id="run_001",
        tool_call_id_factory=lambda: "tool_generated_001",
    )

    tools = await bound_registry.get_tools("refund")
    tool = next(tool for tool in tools if tool.name == "preview_refund_order")

    result = await tool.ainvoke({"order_id": "ORD-10001"})

    assert result == {
        "tool_name": "preview_refund_order",
        "payload": {
            "order_id": "ORD-10001",
        },
    }
