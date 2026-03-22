import asyncio

from app.tools.order import build_order_tools


class FakeOrderClient:
    def __init__(self):
        self.calls: list[tuple[str, dict[str, int]]] = []

    async def list_orders(self, *, user_id: int, page_number: int = 1, page_size: int = 10):
        self.calls.append(
            (
                "list_orders",
                {"user_id": user_id, "page_number": page_number, "page_size": page_size},
            )
        )
        return {
            "list": [
                {"orderNumber": 93001, "orderStatus": 3},
            ]
        }

    async def get_order_service_view(self, *, user_id: int, order_number: int):
        self.calls.append(
            (
                "get_order_service_view",
                {"user_id": user_id, "order_number": order_number},
            )
        )
        return {
            "orderNumber": order_number,
            "orderStatus": 3,
            "payStatus": 2,
            "ticketStatus": 3,
            "canRefund": True,
            "refundBlockedReason": "",
        }


def _tool_by_name(tools, name: str):
    return next(tool for tool in tools if tool.name == name)


def test_order_tools_call_list_orders_and_service_view():
    client = FakeOrderClient()
    tools = build_order_tools(client)

    list_result = asyncio.run(_tool_by_name(tools, "list_orders").ainvoke({"user_id": 3001}))
    detail_result = asyncio.run(
        _tool_by_name(tools, "get_order_service_view").ainvoke(
            {"user_id": 3001, "order_number": 93001}
        )
    )

    assert list_result["list"][0]["orderNumber"] == 93001
    assert detail_result["payStatus"] == 2
    assert client.calls == [
        ("list_orders", {"user_id": 3001, "page_number": 1, "page_size": 10}),
        ("get_order_service_view", {"user_id": 3001, "order_number": 93001}),
    ]
