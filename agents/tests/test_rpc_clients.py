import asyncio
from types import SimpleNamespace

from app.rpc.order_client import OrderRpcClient


class FakeOrderStub:
    def __init__(self) -> None:
        self.calls: list[tuple[str, object, float]] = []

    async def ListOrders(self, request, timeout: float):
        self.calls.append(("ListOrders", request, timeout))
        return SimpleNamespace(
            pageNum=1,
            pageSize=10,
            totalSize=1,
            list=[
                SimpleNamespace(
                    orderNumber=93001,
                    orderStatus=3,
                    programTitle="银河剧场",
                )
            ],
        )

    async def GetOrderServiceView(self, request, timeout: float):
        self.calls.append(("GetOrderServiceView", request, timeout))
        return SimpleNamespace(
            orderNumber=93001,
            orderStatus=3,
            payStatus=2,
            ticketStatus=3,
            programTitle="银河剧场",
            programShowTime="2026-04-01 19:30:00",
            ticketCount=2,
            orderPrice=996,
            canRefund=True,
            refundBlockedReason="",
        )


def _build_client() -> tuple[OrderRpcClient, FakeOrderStub]:
    stub = FakeOrderStub()
    client = OrderRpcClient.__new__(OrderRpcClient)
    client.timeout_seconds = 5.0
    client.stub = stub
    return client, stub


def test_order_rpc_client_accepts_string_identifier_for_list_orders():
    client, stub = _build_client()

    result = asyncio.run(client.list_user_orders(identifier="3001"))

    assert result["list"][0]["orderNumber"] == 93001
    method, request, timeout = stub.calls[0]
    assert method == "ListOrders"
    assert request.userId == 3001
    assert timeout == 5.0


def test_order_rpc_client_get_order_detail_for_service_uses_string_order_id():
    client, stub = _build_client()

    result = asyncio.run(client.get_order_detail_for_service(order_id="93001", user_id="3001"))

    assert result["payStatus"] == 2
    method, request, timeout = stub.calls[0]
    assert method == "GetOrderServiceView"
    assert request.userId == 3001
    assert request.orderNumber == 93001
    assert timeout == 5.0
