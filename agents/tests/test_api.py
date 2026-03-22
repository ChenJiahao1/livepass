from fastapi.testclient import TestClient

from app.api.routes import get_graph, get_llm, get_state_store, get_tool_registry
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
    def __init__(self):
        self.calls: list[dict] = []

    async def ainvoke(self, state_payload, context):
        self.calls.append({"state": state_payload, "context": context})
        message = state_payload["messages"][-1]["content"]
        return {
            **state_payload,
            "final_reply": f"已处理：{message}",
            "current_agent": "order",
            "need_handoff": False,
        }


def build_test_app():
    graph = FakeGraph()
    store = ConversationStateStore(redis_client=FakeRedis(), ttl_seconds=600)
    app = create_app()
    app.dependency_overrides[get_graph] = lambda: graph
    app.dependency_overrides[get_state_store] = lambda: store
    app.dependency_overrides[get_tool_registry] = lambda: object()
    app.dependency_overrides[get_llm] = lambda: object()
    return app, graph, store


def test_chat_requires_user_header():
    client = TestClient(create_app())

    response = client.post("/agent/chat", json={"message": "你好，帮我查订单"})

    assert response.status_code in {400, 401}


def test_chat_api_persists_conversation_state_and_reuses_conversation_id():
    app, graph, _store = build_test_app()
    client = TestClient(app)

    first = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={"message": "帮我查订单"},
    )
    second = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={"message": "订单 93001 可以退款吗", "conversationId": first.json()["conversationId"]},
    )

    assert first.status_code == 200
    assert second.status_code == 200
    assert second.json()["conversationId"] == first.json()["conversationId"]
    assert graph.calls[1]["state"]["messages"][0]["content"] == "帮我查订单"
    assert graph.calls[1]["state"]["messages"][1]["content"] == "已处理：帮我查订单"
