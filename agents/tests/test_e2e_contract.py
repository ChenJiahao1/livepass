from fastapi.testclient import TestClient

from app.api.routes import get_graph, get_llm, get_session_store, get_tool_registry
from app.main import create_app
from app.session.store import ConversationStateStore


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


class FakeGraph:
    async def ainvoke(self, state_payload, config, context):
        message = state_payload["messages"][-1]["content"]
        if "人工" in message:
            return {
                **state_payload,
                "final_reply": "已为你转接人工客服，请稍候。",
                "current_agent": "handoff",
                "need_handoff": True,
            }
        return {
            **state_payload,
            "final_reply": f"已处理：{message}",
            "current_agent": "order",
            "need_handoff": False,
        }


def build_test_client() -> TestClient:
    app = create_app()
    app.dependency_overrides[get_graph] = lambda: FakeGraph()
    app.dependency_overrides[get_session_store] = lambda: ConversationStateStore(
        redis_client=FakeRedis(),
        ttl_seconds=600,
    )
    app.dependency_overrides[get_tool_registry] = lambda: object()
    app.dependency_overrides[get_llm] = lambda: object()
    return TestClient(app)


def test_agent_chat_contract_returns_completed_and_handoff_status():
    client = build_test_client()

    completed = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={"message": "帮我查一下订单"},
    )
    handoff = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={"message": "我要人工客服"},
    )

    for response in [completed, handoff]:
        assert response.status_code == 200
        body = response.json()
        assert set(body.keys()) == {"conversationId", "reply", "status"}
        assert body["conversationId"]
        assert body["reply"]

    assert completed.json()["status"] == "completed"
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
