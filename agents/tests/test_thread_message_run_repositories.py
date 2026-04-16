from datetime import datetime, timezone

from app.messages.models import MESSAGE_STATUS_COMPLETED, MessageRecord
from app.messages.repository import InMemoryMessageRepository
from app.threads.repository import InMemoryThreadRepository

NOW = datetime(2026, 4, 16, 10, 0, tzinfo=timezone.utc)
NOW1 = datetime(2026, 4, 16, 10, 1, tzinfo=timezone.utc)
NOW2 = datetime(2026, 4, 16, 10, 2, tzinfo=timezone.utc)


def test_in_memory_thread_repository_filters_empty_threads_by_default():
    repo = InMemoryThreadRepository()
    created = repo.create(user_id=3001, title="新会话", now=NOW)

    threads, next_cursor = repo.list_by_user(
        user_id=3001,
        status="active",
        limit=20,
        cursor=None,
        include_empty=False,
    )

    assert created.id.startswith("thr_")
    assert threads == []
    assert next_cursor is None


def test_in_memory_message_repository_returns_recent_messages_ascending():
    repo = InMemoryMessageRepository()
    repo.create(
        MessageRecord(
            id="msg_2",
            thread_id="thr_01",
            user_id=3001,
            role="assistant",
            parts=[{"type": "text", "text": "二"}],
            status=MESSAGE_STATUS_COMPLETED,
            run_id="run_01",
            created_at=NOW2,
            metadata={},
        )
    )
    repo.create(
        MessageRecord(
            id="msg_1",
            thread_id="thr_01",
            user_id=3001,
            role="user",
            parts=[{"type": "text", "text": "一"}],
            status=MESSAGE_STATUS_COMPLETED,
            run_id="run_01",
            created_at=NOW1,
            metadata={},
        )
    )

    messages, next_cursor = repo.list_by_thread(thread_id="thr_01", user_id=3001, limit=50, before=None)

    assert [message.id for message in messages] == ["msg_1", "msg_2"]
    assert next_cursor is None
