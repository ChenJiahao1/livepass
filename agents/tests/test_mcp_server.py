import asyncio

from app.mcp_server.server import build_server
from app.mcp_server.tools.order import build_get_order_detail_for_service_tool


class FakeOrderRpcClient:
    def __init__(self, *, order_detail: dict):
        self.order_detail = order_detail

    async def get_order_detail_for_service(self, *, order_id: str, user_id: str | int | None = None):
        return dict(self.order_detail)


def test_order_tool_normalizes_rpc_response():
    client = FakeOrderRpcClient(
        order_detail={
            "orderNumber": 93001,
            "orderStatus": 3,
            "payStatus": 2,
            "ticketStatus": 3,
        }
    )
    tool = build_get_order_detail_for_service_tool(client)

    result = asyncio.run(tool.ainvoke({"order_id": "93001", "user_id": 3001}))

    assert result["order_id"] == "93001"
    assert result["payment_status"] == "paid"


def test_build_server_registers_order_tools():
    server = build_server(
        "order",
        clients={
            "order": FakeOrderRpcClient(
                order_detail={
                    "orderNumber": 93001,
                    "orderStatus": 3,
                    "payStatus": 2,
                    "ticketStatus": 3,
                }
            )
        },
    )

    tools = asyncio.run(server.list_tools())

    assert any(tool.name == "get_order_detail_for_service" for tool in tools)
