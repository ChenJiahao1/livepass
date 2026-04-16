"""Redis-backed thread ownership store."""

from __future__ import annotations

import json


class ThreadOwnershipError(ValueError):
    """Raised when a thread is accessed by another user."""


class ThreadOwnershipStore:
    def __init__(
        self,
        *,
        redis_client,
        ttl_seconds: int,
        key_prefix: str = "agents:thread",
    ) -> None:
        self.redis_client = redis_client
        self.ttl_seconds = ttl_seconds
        self.key_prefix = key_prefix

    def key_for(self, thread_id: str) -> str:
        return f"{self.key_prefix}:{thread_id}"

    def save(self, *, thread_id: str, user_id: int) -> None:
        key = self.key_for(thread_id)
        payload = json.dumps({"threadId": thread_id, "userId": user_id})
        self.redis_client.set(key, payload)
        self.redis_client.expire(key, self.ttl_seconds)

    def assert_owner(self, *, thread_id: str, user_id: int) -> None:
        payload = self.redis_client.get(self.key_for(thread_id))
        if not payload:
            return

        stored = json.loads(payload)
        if int(stored["userId"]) != user_id:
            raise ThreadOwnershipError(f"thread {thread_id} does not belong to user {user_id}")
