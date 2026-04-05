from app.config import Settings
from app.mcp_client.registry import MCPToolRegistry


def test_mcp_registry_prefers_go_order_provider():
    settings = Settings(order_mcp_endpoint="http://127.0.0.1:9082/message")

    registry = MCPToolRegistry(settings=settings)

    provider = registry.connections["order"]
    assert provider["transport"] in {"http", "sse", "streamable_http"}
    assert provider["url"] == settings.order_mcp_endpoint
    assert provider["headers"]["X-Internal-Caller"] == "agents"

