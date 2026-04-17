from app.shared.config import Settings
from app.integrations.mcp.registry import MCPToolRegistry


def test_mcp_registry_prefers_go_order_provider():
    settings = Settings(
        activity_mcp_endpoint="http://127.0.0.1:9083/message",
        order_mcp_endpoint="http://127.0.0.1:9082/message",
    )

    registry = MCPToolRegistry(settings=settings)

    activity_provider = registry.connections["activity"]
    assert activity_provider["transport"] in {"http", "sse", "streamable_http"}
    assert activity_provider["url"] == settings.activity_mcp_endpoint
    assert activity_provider["headers"]["X-Internal-Caller"] == "agents"

    provider = registry.connections["order"]
    assert provider["transport"] in {"http", "sse", "streamable_http"}
    assert provider["url"] == settings.order_mcp_endpoint
    assert provider["headers"]["X-Internal-Caller"] == "agents"
