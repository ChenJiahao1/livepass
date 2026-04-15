import pytest

from app.session.store import ConversationStateStore, SessionOwnershipError
from tests.fakes import FakeRedis


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


def test_session_store_rejects_foreign_user():
    store = ConversationStateStore(redis_client=FakeRedis(), ttl_seconds=60)
    store.get_or_create(user_id=1, conversation_id="conv-1")
    store.save(store.get_or_create(user_id=1, conversation_id="conv-1"))

    with pytest.raises(SessionOwnershipError):
        store.get_or_create(user_id=2, conversation_id="conv-1")
