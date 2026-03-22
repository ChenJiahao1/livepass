import pytest

from app.session.models import SessionMessage
from app.session.store import ConversationSessionStore, SessionOwnershipError


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


def test_session_store_generates_conversation_id_when_missing():
    store = ConversationSessionStore(redis_client=FakeRedis(), ttl_seconds=600)

    session = store.get_or_create(user_id=3001, conversation_id=None)

    assert session.user_id == 3001
    assert session.conversation_id
    assert session.messages == []


def test_session_store_rejects_user_mismatch():
    store = ConversationSessionStore(redis_client=FakeRedis(), ttl_seconds=600)
    session = store.get_or_create(user_id=3001, conversation_id=None)
    store.save(session)

    with pytest.raises(SessionOwnershipError):
        store.get_or_create(user_id=3002, conversation_id=session.conversation_id)


def test_session_store_appends_messages_and_refreshes_ttl():
    redis_client = FakeRedis()
    store = ConversationSessionStore(redis_client=redis_client, ttl_seconds=600)

    session = store.get_or_create(user_id=3001, conversation_id=None)
    session.messages.append(SessionMessage(role="user", content="你好"))
    store.save(session)

    loaded = store.get_or_create(user_id=3001, conversation_id=session.conversation_id)
    loaded.messages.append(SessionMessage(role="assistant", content="您好"))
    store.save(loaded)

    reloaded = store.get_or_create(user_id=3001, conversation_id=session.conversation_id)
    assert [message.content for message in reloaded.messages] == ["你好", "您好"]
    assert redis_client.expire_calls == [
        (f"agents:conversation:{session.conversation_id}", 600),
        (f"agents:conversation:{session.conversation_id}", 600),
    ]
