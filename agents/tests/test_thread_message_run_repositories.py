from datetime import datetime, timezone
from pathlib import Path

from app.messages.models import MESSAGE_STATUS_COMPLETED, MessageRecord
from app.messages.repository import InMemoryMessageRepository
from app.threads.repository import InMemoryThreadRepository

NOW = datetime(2026, 4, 16, 10, 0, tzinfo=timezone.utc)
NOW1 = datetime(2026, 4, 16, 10, 1, tzinfo=timezone.utc)
NOW2 = datetime(2026, 4, 16, 10, 2, tzinfo=timezone.utc)
REPO_ROOT = Path(__file__).resolve().parents[2]


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
            content=[{"type": "text", "text": "二"}],
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
            content=[{"type": "text", "text": "一"}],
            status=MESSAGE_STATUS_COMPLETED,
            run_id="run_01",
            created_at=NOW1,
            metadata={},
        )
    )

    messages, next_cursor = repo.list_by_thread(thread_id="thr_01", user_id=3001, limit=50, before=None)

    assert [message.id for message in messages] == ["msg_1", "msg_2"]
    assert messages[0].content == [{"type": "text", "text": "一"}]
    assert next_cursor is None


def test_mysql_message_repository_accepts_tuple_rows_from_cursor():
    from app.messages.repository import MySQLMessageRepository

    rows = (
        {
            "id": "msg_2",
            "thread_id": "thr_01",
            "user_id": 3001,
            "role": "assistant",
            "content_json": '[{"type":"text","text":"二"}]',
            "status": MESSAGE_STATUS_COMPLETED,
            "run_id": "run_01",
            "created_at": NOW2,
            "updated_at": NOW2,
            "metadata_json": "{}",
        },
        {
            "id": "msg_1",
            "thread_id": "thr_01",
            "user_id": 3001,
            "role": "user",
            "content_json": '[{"type":"text","text":"一"}]',
            "status": MESSAGE_STATUS_COMPLETED,
            "run_id": "run_01",
            "created_at": NOW1,
            "updated_at": NOW1,
            "metadata_json": "{}",
        },
    )

    class FakeCursor:
        def __enter__(self):
            return self

        def __exit__(self, exc_type, exc, tb):
            return False

        def execute(self, sql, args):
            return None

        def fetchall(self):
            return rows

    class FakeConnection:
        def cursor(self):
            return FakeCursor()

        def close(self):
            return None

    class FakeConnectionFactory:
        def connect(self):
            return FakeConnection()

    repo = MySQLMessageRepository(connection_factory=FakeConnectionFactory())

    messages, next_cursor = repo.list_by_thread(thread_id="thr_01", user_id=3001, limit=50, before=None)

    assert [message.id for message in messages] == ["msg_1", "msg_2"]
    assert messages[0].content == [{"type": "text", "text": "一"}]
    assert next_cursor is None


def test_resource_models_expose_explicit_persistent_fields():
    from app.messages.models import MessageRecord
    from app.runs.event_models import RunEventRecord
    from app.runs.models import RunRecord
    from app.runs.tool_call_models import ToolCallRecord

    assert "updated_at" in MessageRecord.__dataclass_fields__
    assert "content" in MessageRecord.__dataclass_fields__
    assert "parts" not in MessageRecord.__dataclass_fields__
    assert "output_message_id" in RunRecord.__dataclass_fields__
    assert "assistant_message_id" not in RunRecord.__dataclass_fields__
    assert "message_id" in ToolCallRecord.__dataclass_fields__
    assert "name" in ToolCallRecord.__dataclass_fields__
    assert "input" in ToolCallRecord.__dataclass_fields__
    assert "output" in ToolCallRecord.__dataclass_fields__
    assert "message_id" in RunEventRecord.__dataclass_fields__
    assert "tool_call_id" in RunEventRecord.__dataclass_fields__


def test_agents_sql_files_define_explicit_resource_columns_and_import_list():
    messages_sql = (REPO_ROOT / "sql/agents/agent_messages.sql").read_text(encoding="utf-8")
    runs_sql = (REPO_ROOT / "sql/agents/agent_runs.sql").read_text(encoding="utf-8")
    tool_calls_sql = (REPO_ROOT / "sql/agents/agent_tool_calls.sql").read_text(encoding="utf-8")
    run_events_sql = (REPO_ROOT / "sql/agents/agent_run_events.sql").read_text(encoding="utf-8")
    import_sql = (REPO_ROOT / "scripts/import_sql.sh").read_text(encoding="utf-8")

    assert "updated_at datetime(3) NOT NULL" in messages_sql
    assert "content_json json NOT NULL" in messages_sql
    assert "parts_json" not in messages_sql
    assert "output_message_id varchar(64) NOT NULL" in runs_sql
    assert "message_id varchar(64) NOT NULL" in tool_calls_sql
    assert "input_json json NOT NULL" in tool_calls_sql
    assert "human_request_json json NOT NULL" in tool_calls_sql
    assert "output_json json NULL" in tool_calls_sql
    assert "message_id varchar(64) NULL" in run_events_sql
    assert "tool_call_id varchar(64) NULL" in run_events_sql
    assert '"sql/agents/agent_run_events.sql"' in import_sql
    assert '"sql/agents/agent_tool_calls.sql"' in import_sql


def test_run_event_store_lists_events_after_sequence():
    from app.runs.event_models import RUN_EVENT_TYPE_MESSAGE_DELTA, RUN_EVENT_TYPE_RUN_COMPLETED
    from app.runs.event_store import InMemoryRunEventStore

    store = InMemoryRunEventStore()
    event1 = store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_MESSAGE_DELTA,
        payload={"messageId": "msg_01", "delta": {"type": "text", "text": "你好"}},
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
    assert getattr(event1, "message_id", None) == "msg_01"
    assert event1.payload == {"delta": {"type": "text", "text": "你好"}}
    assert getattr(event2, "tool_call_id", None) is None


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
            message_id="msg_asst_01",
            name="human_approval",
            status=TOOL_CALL_STATUS_WAITING_HUMAN,
            input={"title": "退款前确认"},
            human_request={"title": "退款前确认", "allowedActions": ["approve", "reject"]},
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
        error=None,
        now=NOW2,
        output={"action": "approve"},
    )

    assert updated is not None
    assert updated.status == TOOL_CALL_STATUS_COMPLETED
    assert updated.message_id == "msg_asst_01"
    assert updated.name == "human_approval"
    assert updated.input == {"title": "退款前确认"}
    assert updated.human_request == {"title": "退款前确认", "allowedActions": ["approve", "reject"]}
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
            message_id="msg_asst_01",
            name="human_approval",
            status=TOOL_CALL_STATUS_COMPLETED,
            input={},
            human_request={},
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
            message_id="msg_asst_01",
            name="human_approval",
            status=TOOL_CALL_STATUS_WAITING_HUMAN,
            input={"action": "refund_order"},
            human_request={"title": "退款前确认", "allowedActions": ["approve", "reject"]},
            created_at=NOW1,
            updated_at=NOW1,
        )
    )

    waiting = repo.find_waiting_by_run(run_id="run_01")
    cancelled = repo.mark_cancelled(tool_call_id="tool_waiting", now=NOW2)

    assert waiting is not None
    assert waiting.id == "tool_waiting"
    assert waiting.message_id == "msg_asst_01"
    assert waiting.name == "human_approval"
    assert waiting.input["action"] == "refund_order"
    assert waiting.human_request == {"title": "退款前确认", "allowedActions": ["approve", "reject"]}
    assert cancelled is not None
    assert cancelled.status == TOOL_CALL_STATUS_CANCELLED
    assert cancelled.completed_at == NOW2
