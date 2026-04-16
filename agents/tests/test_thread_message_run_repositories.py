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


def test_run_event_store_lists_events_after_sequence():
    from app.runs.event_models import RUN_EVENT_TYPE_MESSAGE_DELTA, RUN_EVENT_TYPE_RUN_COMPLETED
    from app.runs.event_store import InMemoryRunEventStore

    store = InMemoryRunEventStore()
    event1 = store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_MESSAGE_DELTA,
        payload={"messageId": "msg_01", "delta": "你好"},
        now=NOW1,
    )
    event2 = store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_RUN_COMPLETED,
        payload={"status": "completed"},
        now=NOW2,
    )

    events = store.list_after(run_id="run_01", after_sequence_no=1)

    assert event1.sequence_no == 1
    assert event2.sequence_no == 2
    assert [event.sequence_no for event in events] == [2]
    assert [event.event_type for event in events] == [RUN_EVENT_TYPE_RUN_COMPLETED]


def test_tool_call_repository_updates_waiting_human_to_completed():
    from app.runs.tool_call_models import (
        TOOL_CALL_STATUS_COMPLETED,
        TOOL_CALL_STATUS_WAITING_HUMAN,
        ToolCallRecord,
    )
    from app.runs.tool_call_repository import InMemoryToolCallRepository

    repo = InMemoryToolCallRepository()
    repo.create(
        ToolCallRecord(
            id="tool_01",
            run_id="run_01",
            thread_id="thr_01",
            user_id=3001,
            tool_name="human_approval",
            status=TOOL_CALL_STATUS_WAITING_HUMAN,
            arguments={"title": "退款前确认"},
            output=None,
            error=None,
            created_at=NOW1,
            updated_at=NOW1,
            completed_at=None,
            metadata={},
        )
    )

    updated = repo.update_status(
        tool_call_id="tool_01",
        status=TOOL_CALL_STATUS_COMPLETED,
        output={"action": "approve"},
        error=None,
        now=NOW2,
    )

    assert updated is not None
    assert updated.status == TOOL_CALL_STATUS_COMPLETED
    assert updated.output == {"action": "approve"}
    assert updated.error is None
    assert updated.completed_at == NOW2
    assert updated.updated_at == NOW2


def test_tool_call_repository_finds_waiting_human_by_run_and_marks_cancelled():
    from app.runs.tool_call_models import (
        TOOL_CALL_STATUS_CANCELLED,
        TOOL_CALL_STATUS_COMPLETED,
        TOOL_CALL_STATUS_WAITING_HUMAN,
        ToolCallRecord,
    )
    from app.runs.tool_call_repository import InMemoryToolCallRepository

    repo = InMemoryToolCallRepository()
    repo.create(
        ToolCallRecord(
            id="tool_done",
            run_id="run_01",
            thread_id="thr_01",
            user_id=3001,
            tool_name="human_approval",
            status=TOOL_CALL_STATUS_COMPLETED,
            arguments={},
            request={},
            completed_at=NOW1,
            created_at=NOW1,
            updated_at=NOW1,
        )
    )
    repo.create(
        ToolCallRecord(
            id="tool_waiting",
            run_id="run_01",
            thread_id="thr_01",
            user_id=3001,
            tool_name="human_approval",
            status=TOOL_CALL_STATUS_WAITING_HUMAN,
            arguments={"action": "refund_order"},
            request={"title": "退款前确认"},
            created_at=NOW1,
            updated_at=NOW1,
        )
    )

    waiting = repo.find_waiting_by_run(run_id="run_01")
    cancelled = repo.mark_cancelled(tool_call_id="tool_waiting", now=NOW2)

    assert waiting is not None
    assert waiting.id == "tool_waiting"
    assert cancelled is not None
    assert cancelled.status == TOOL_CALL_STATUS_CANCELLED
    assert cancelled.completed_at == NOW2
