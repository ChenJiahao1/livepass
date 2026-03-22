import asyncio

from fastapi.testclient import TestClient

from app.api.routes import get_chat_service
from app.main import create_app
from app.orchestrator.graph import GraphOrchestrator
from app.orchestrator.service import ChatService
from app.session.store import ConversationSessionStore


class FakeRedis:
    def __init__(self):
        self.values: dict[str, str] = {}

    def get(self, key: str):
        return self.values.get(key)

    def set(self, key: str, value: str):
        self.values[key] = value
        return True

    def expire(self, key: str, ttl_seconds: int):
        return True


class FakeProgramClient:
    async def page_programs(self, *, page_number: int = 1, page_size: int = 10):
        return {
            "list": [
                {"id": 2001, "title": "银河剧场", "showTime": "2026-04-01 19:30:00"},
            ]
        }

    async def get_program_detail(self, *, program_id: int):
        return {
            "id": program_id,
            "title": "银河剧场",
            "showTime": "2026-04-01 19:30:00",
        }


class FakeOrderClient:
    async def list_orders(self, *, user_id: int, page_number: int = 1, page_size: int = 10):
        return {
            "list": [
                {"orderNumber": 93001, "orderStatus": 3, "payStatus": 2, "ticketStatus": 3},
            ]
        }

    async def get_order_service_view(self, *, user_id: int, order_number: int):
        return {
            "orderNumber": order_number,
            "orderStatus": 3,
            "payStatus": 2,
            "ticketStatus": 3,
            "canRefund": True,
            "refundBlockedReason": "",
        }

    async def preview_refund_order(self, *, user_id: int, order_number: int):
        return {
            "orderNumber": order_number,
            "allowRefund": True,
            "refundAmount": 478,
            "refundPercent": 80,
            "rejectReason": "",
        }

    async def refund_order(self, *, user_id: int, order_number: int, reason: str):
        return {
            "orderNumber": order_number,
            "orderStatus": 4,
            "refundAmount": 478,
            "refundPercent": 80,
        }


class FakeUserClient:
    async def get_user_by_id(self, *, user_id: int):
        return {"id": user_id, "name": "测试用户"}


def build_test_client() -> TestClient:
    store = ConversationSessionStore(redis_client=FakeRedis(), ttl_seconds=600)
    orchestrator = GraphOrchestrator(
        program_client=FakeProgramClient(),
        order_client=FakeOrderClient(),
        user_client=FakeUserClient(),
    )
    service = ChatService(session_store=store, orchestrator=orchestrator)
    app = create_app()
    app.dependency_overrides[get_chat_service] = lambda: service
    return TestClient(app)


def test_agent_chat_contract_covers_activity_order_refund_and_handoff():
    client = build_test_client()

    activity = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={"message": "最近有什么演出"},
    )
    order = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={"message": "帮我查一下订单"},
    )
    preview = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={"message": "订单 93001 可以退款吗"},
    )
    refund = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={"message": "帮我退款订单 93001"},
    )
    handoff = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={"message": "我要人工客服"},
    )

    for response in [activity, order, preview, refund, handoff]:
        assert response.status_code == 200
        body = response.json()
        assert set(body.keys()) == {"conversationId", "reply", "status"}
        assert body["conversationId"]
        assert body["reply"]

    assert activity.json()["status"] == "completed"
    assert order.json()["status"] == "completed"
    assert preview.json()["status"] == "completed"
    assert refund.json()["status"] == "completed"
    assert handoff.json()["status"] == "handoff"


def test_agent_chat_contract_supports_multi_turn_conversation():
    client = build_test_client()

    first = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={"message": "帮我查一下订单"},
    )
    first_body = first.json()

    second = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={
            "message": "订单 93001 可以退款吗",
            "conversationId": first_body["conversationId"],
        },
    )
    second_body = second.json()

    assert first.status_code == 200
    assert second.status_code == 200
    assert second_body["conversationId"] == first_body["conversationId"]
    assert second_body["status"] == "completed"
