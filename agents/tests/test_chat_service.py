import asyncio

from app.orchestrator.service import ChatService
from app.orchestrator.state import OrchestratorResult
from app.session.store import ConversationSessionStore


class FakeRedis:
    def __init__(self):
        self.values: dict[str, str] = {}
        self.expire_calls: list[tuple[str, int]] = []

    def get(self, key: str):
        return self.values.get(key)

    def set(self, key: str, value: str):
        self.values[key] = value
        return True

    def expire(self, key: str, ttl_seconds: int):
        self.expire_calls.append((key, ttl_seconds))
        return True


class FakeOrchestrator:
    def __init__(self):
        self.calls: list[dict[str, object]] = []

    async def reply(self, session, *, message: str) -> OrchestratorResult:
        self.calls.append(
            {
                "conversation_id": session.conversation_id,
                "user_id": session.user_id,
                "message": message,
            }
        )
        return OrchestratorResult(
            reply=f"收到：{message}",
            status="completed",
            current_agent="stub",
        )


def test_chat_service_creates_conversation_and_persists_turns():
    store = ConversationSessionStore(redis_client=FakeRedis(), ttl_seconds=600)
    orchestrator = FakeOrchestrator()
    service = ChatService(session_store=store, orchestrator=orchestrator)

    response = asyncio.run(
        service.handle_chat(
            user_id=3001,
            message="帮我查一下订单",
            conversation_id=None,
        )
    )

    assert response["conversationId"]
    assert response["reply"] == "收到：帮我查一下订单"
    assert response["status"] == "completed"

    session = store.get_or_create(user_id=3001, conversation_id=response["conversationId"])
    assert [(message.role, message.content) for message in session.messages] == [
        ("user", "帮我查一下订单"),
        ("assistant", "收到：帮我查一下订单"),
    ]
    assert orchestrator.calls == [
        {
            "conversation_id": response["conversationId"],
            "user_id": 3001,
            "message": "帮我查一下订单",
        }
    ]


def test_chat_service_reuses_existing_conversation_and_refreshes_ttl():
    redis_client = FakeRedis()
    store = ConversationSessionStore(redis_client=redis_client, ttl_seconds=600)
    orchestrator = FakeOrchestrator()
    service = ChatService(session_store=store, orchestrator=orchestrator)

    first = asyncio.run(
        service.handle_chat(
            user_id=3001,
            message="第一轮",
            conversation_id=None,
        )
    )
    second = asyncio.run(
        service.handle_chat(
            user_id=3001,
            message="第二轮",
            conversation_id=first["conversationId"],
        )
    )

    assert second["conversationId"] == first["conversationId"]

    session = store.get_or_create(user_id=3001, conversation_id=first["conversationId"])
    assert [message.content for message in session.messages] == [
        "第一轮",
        "收到：第一轮",
        "第二轮",
        "收到：第二轮",
    ]
    assert redis_client.expire_calls[-1] == (f"agents:conversation:{first['conversationId']}", 600)
