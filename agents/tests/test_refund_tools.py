import asyncio

from app.tools.refund import build_refund_tools


class FakeOrderClient:
    def __init__(self):
        self.calls: list[tuple[str, dict[str, object]]] = []

    async def preview_refund_order(self, *, user_id: int, order_number: int):
        self.calls.append(
            (
                "preview_refund_order",
                {"user_id": user_id, "order_number": order_number},
            )
        )
        return {
            "orderNumber": order_number,
            "allowRefund": True,
            "refundAmount": 478,
            "refundPercent": 80,
            "rejectReason": "",
        }

    async def refund_order(self, *, user_id: int, order_number: int, reason: str):
        self.calls.append(
            (
                "refund_order",
                {"user_id": user_id, "order_number": order_number, "reason": reason},
            )
        )
        return {
            "orderNumber": order_number,
            "orderStatus": 4,
            "refundAmount": 478,
            "refundPercent": 80,
        }


def _tool_by_name(tools, name: str):
    return next(tool for tool in tools if tool.name == name)


def test_refund_tools_call_preview_and_refund():
    client = FakeOrderClient()
    tools = build_refund_tools(client)

    preview_result = asyncio.run(
        _tool_by_name(tools, "preview_refund_order").ainvoke(
            {"user_id": 3001, "order_number": 93001}
        )
    )
    refund_result = asyncio.run(
        _tool_by_name(tools, "refund_order").ainvoke(
            {"user_id": 3001, "order_number": 93001, "reason": "行程冲突"}
        )
    )

    assert preview_result["allowRefund"] is True
    assert refund_result["orderStatus"] == 4
    assert client.calls == [
        ("preview_refund_order", {"user_id": 3001, "order_number": 93001}),
        ("refund_order", {"user_id": 3001, "order_number": 93001, "reason": "行程冲突"}),
    ]
