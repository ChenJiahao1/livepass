import pytest

from app.session.store import ConversationStateStore, SessionOwnershipError


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


def test_state_store_generates_conversation_id_when_missing():
    store = ConversationStateStore(redis_client=FakeRedis(), ttl_seconds=600)

    session = store.get_or_create(user_id=3001, conversation_id=None)

    assert session.user_id == 3001
    assert session.conversation_id


def test_state_store_rejects_user_mismatch():
    store = ConversationStateStore(redis_client=FakeRedis(), ttl_seconds=600)
    session = store.get_or_create(user_id=3001, conversation_id=None)
    store.save(session)

    with pytest.raises(SessionOwnershipError):
        store.get_or_create(user_id=3002, conversation_id=session.conversation_id)


def test_state_store_saves_state_and_refreshes_ttl():
    redis_client = FakeRedis()
    store = ConversationStateStore(redis_client=redis_client, ttl_seconds=600)

    session = store.get_or_create(user_id=3001, conversation_id=None)
    store.save(session)

    loaded = store.get_or_create(user_id=3001, conversation_id=session.conversation_id)
    store.save(loaded)

    reloaded = store.get_or_create(user_id=3001, conversation_id=session.conversation_id)
    assert reloaded.user_id == 3001
    assert reloaded.conversation_id == session.conversation_id
    assert redis_client.expire_calls == [
        (f"agents:conversation:{session.conversation_id}", 600),
        (f"agents:conversation:{session.conversation_id}", 600),
    ]


def test_state_store_persists_runtime_session_fields():
    redis_client = FakeRedis()
    store = ConversationStateStore(redis_client=redis_client, ttl_seconds=600)

    session = store.get_or_create(user_id=3001, conversation_id=None)
    session.selected_order_id = "ORD-10001"
    session.recent_order_candidates = [{"order_id": "ORD-10001"}]
    session.last_task_summary = "退款资格已确认"
    session.last_handoff_ticket_id = "HOF-1001"
    store.save(session)

    loaded = store.get_or_create(user_id=3001, conversation_id=session.conversation_id)

    assert loaded.selected_order_id == "ORD-10001"
    assert loaded.recent_order_candidates == [{"order_id": "ORD-10001"}]
    assert loaded.last_task_summary == "退款资格已确认"
    assert loaded.last_handoff_ticket_id == "HOF-1001"
