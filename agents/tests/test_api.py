from fastapi.testclient import TestClient

from app.api.routes import get_chat_service
from app.main import create_app


class FakeChatService:
    async def handle_chat(self, *, user_id: int, message: str, conversation_id: str | None):
        assert user_id == 3001
        assert message == "你好，帮我查订单"
        assert conversation_id is None
        return {
            "conversationId": "conv-test-1",
            "reply": "已收到你的问题",
            "status": "completed",
        }


def test_chat_requires_user_header():
    client = TestClient(create_app())

    response = client.post(
        "/agent/chat",
        json={"message": "你好，帮我查订单"},
    )

    assert response.status_code in {400, 401}


def test_chat_returns_contract_json():
    app = create_app()
    app.dependency_overrides[get_chat_service] = lambda: FakeChatService()
    client = TestClient(app)

    response = client.post(
        "/agent/chat",
        headers={"X-User-Id": "3001"},
        json={"message": "你好，帮我查订单"},
    )

    assert response.status_code == 200
    assert response.json() == {
        "conversationId": "conv-test-1",
        "reply": "已收到你的问题",
        "status": "completed",
    }
