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
    def __init__(self):
        self.calls: list[dict] = []

    async def ainvoke(self, state_payload, config, context):
        self.calls.append({"state": state_payload, "config": config, "context": context})
        message = state_payload["messages"][-1]["content"]
        if "人工" in message:
            return {
                **state_payload,
                "reply": "已为你转接人工客服，请稍候。",
                "current_agent": "handoff",
                "need_handoff": True,
            }
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
    app.dependency_overrides[get_session_store] = lambda: store
    app.dependency_overrides[get_tool_registry] = lambda: object()
    app.dependency_overrides[get_llm] = lambda: object()
    return app, graph, store


def test_chat_requires_user_header():
    client = TestClient(create_app())

    response = client.post("/agent/chat", json={"message": "你好，帮我查订单"})

    assert response.status_code in {400, 401}


def test_chat_api_injects_thread_and_user_context():
    app, graph, _store = build_test_app()
    client = TestClient(app)

    response = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={"message": "帮我查订单"},
    )

    assert response.status_code == 200
    body = response.json()
    assert graph.calls[0]["config"]["configurable"]["thread_id"] == body["conversationId"]
    assert graph.calls[0]["context"]["current_user_id"] == "3001"
    assert body["status"] == "completed"
    assert body["reply"] == "已处理：帮我查订单"


def test_chat_api_maps_reply_fallback_and_handoff_status():
    app, _graph, _store = build_test_app()
    client = TestClient(app)

    response = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={"message": "我要人工客服"},
    )

    assert response.status_code == 200
    body = response.json()
    assert body["status"] == "handoff"
    assert body["reply"] == "已为你转接人工客服，请稍候。"


def test_chat_api_reuses_conversation_id_and_starts_new_graph_turn_each_request():
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
    assert graph.calls[0]["config"]["configurable"]["thread_id"] == first.json()["conversationId"]
    assert graph.calls[1]["config"]["configurable"]["thread_id"] == first.json()["conversationId"]
    assert graph.calls[0]["state"]["messages"] == [{"role": "user", "content": "帮我查订单"}]
    assert graph.calls[1]["state"]["messages"] == [{"role": "user", "content": "订单 93001 可以退款吗"}]
