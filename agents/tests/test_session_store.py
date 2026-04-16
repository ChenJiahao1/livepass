import pytest

from app.session.store import ThreadOwnershipError, ThreadOwnershipStore
from tests.fakes import FakeRedis


def test_thread_ownership_store_uses_thread_id_key():
    redis = FakeRedis()
    store = ThreadOwnershipStore(redis_client=redis, ttl_seconds=600, key_prefix="agents:thread")

    store.save(thread_id="thr_01", user_id=3001)

    assert redis.values["agents:thread:thr_01"] == '{"threadId": "thr_01", "userId": 3001}'
    assert store.assert_owner(thread_id="thr_01", user_id=3001) is None


def test_thread_ownership_store_rejects_other_user():
    redis = FakeRedis()
    store = ThreadOwnershipStore(redis_client=redis, ttl_seconds=600, key_prefix="agents:thread")
    store.save(thread_id="thr_01", user_id=3001)

    with pytest.raises(ThreadOwnershipError):
        store.assert_owner(thread_id="thr_01", user_id=3002)
