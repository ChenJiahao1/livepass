from __future__ import annotations

from datetime import datetime, timezone

import pytest

from app.agent_runtime.service import AgentRuntimeService
from app.common.errors import ApiError
from app.messages.repository import InMemoryMessageRepository
from app.messages.service import MessageService
from app.runs.event_models import (
    RUN_EVENT_TYPE_MESSAGE_DELTA,
    RUN_EVENT_TYPE_RUN_CANCELLED,
    RUN_EVENT_TYPE_RUN_COMPLETED,
    RUN_EVENT_TYPE_RUN_FAILED,
    RUN_EVENT_TYPE_RUN_STARTED,
)
from app.runs.event_store import InMemoryRunEventStore
from app.runs.executor import RunExecutor
from app.runs.repository import InMemoryRunRepository
from app.runs.service import RunService
from app.runs.tool_call_models import TOOL_CALL_STATUS_WAITING_HUMAN, ToolCallRecord
from app.runs.tool_call_repository import InMemoryToolCallRepository
from app.session.store import ThreadOwnershipStore
from app.threads.repository import InMemoryThreadRepository
from tests.fakes import FakeRedis, StubRegistry


class FakeRuntime:
    async def ainvoke(self, state_payload, config, context):
        user_text = state_payload["messages"][-1]["content"]
        return {
            "reply": f"已处理：{user_text}",
            "final_reply": f"已处理：{user_text}",
            "current_agent": "order",
            "need_handoff": False,
            "route_source": "rule",
        }


class FailingRuntime:
    async def ainvoke(self, state_payload, config, context):
        raise RuntimeError("runtime exploded")


def build_executor(
    *,
    runtime: object | None = None,
) -> tuple[
    RunExecutor,
    RunService,
    MessageService,
    InMemoryRunEventStore,
    InMemoryToolCallRepository,
    str,
    str,
]:
    thread_repo = InMemoryThreadRepository()
    message_repo = InMemoryMessageRepository()
    run_repo = InMemoryRunRepository()
    event_store = InMemoryRunEventStore()
    tool_call_repo = InMemoryToolCallRepository()
    ownership_store = ThreadOwnershipStore(redis_client=FakeRedis(), ttl_seconds=600, key_prefix="agents:thread")

    message_service = MessageService(
        thread_repository=thread_repo,
        message_repository=message_repo,
        ownership_store=ownership_store,
    )
    run_service = RunService(
        run_repository=run_repo,
        message_service=message_service,
        ownership_store=ownership_store,
    )
    runtime_service = AgentRuntimeService(
        agent_runtime=runtime or FakeRuntime(),
        registry=StubRegistry(),
        llm=object(),
    )
    executor = RunExecutor(
        run_repository=run_repo,
        run_service=run_service,
        message_service=message_service,
        event_store=event_store,
        tool_call_repository=tool_call_repo,
        runtime_service=runtime_service,
    )

    thread = thread_repo.create(user_id=3001, title="退款", now=datetime.now(timezone.utc))
    ownership_store.save(thread_id=thread.id, user_id=3001)
    run = run_service.create_run(
        user_id=3001,
        thread_id=thread.id,
        parts=[{"type": "text", "text": "帮我查订单"}],
    )
    return executor, run_service, message_service, event_store, tool_call_repo, thread.id, run.id


@pytest.mark.anyio
async def test_executor_projects_run_started_message_delta_and_run_completed():
    executor, run_service, message_service, event_store, _tool_call_repo, thread_id, run_id = build_executor()
    run = run_service.get_active_run(user_id=3001, thread_id=thread_id)
    assert run is not None

    await executor.start(run_id)

    saved_run = run_service.get_run(user_id=3001, run_id=run_id)
    messages, _ = message_service.list_messages(user_id=3001, thread_id=run.thread_id, limit=20, before=None)
    events = event_store.list_after(run_id=run_id, after_sequence_no=0)

    assert saved_run.status == "completed"
    assert [event.event_type for event in events] == [
        RUN_EVENT_TYPE_RUN_STARTED,
        RUN_EVENT_TYPE_MESSAGE_DELTA,
        RUN_EVENT_TYPE_RUN_COMPLETED,
    ]
    assert messages[1].parts == [{"type": "text", "text": "已处理：帮我查订单"}]


@pytest.mark.anyio
async def test_cancel_running_run_appends_run_cancelled_event():
    executor, run_service, _message_service, event_store, _tool_call_repo, thread_id, run_id = build_executor()
    run = run_service.get_active_run(user_id=3001, thread_id=thread_id)
    assert run is not None

    await executor.cancel(run_id)

    saved_run = run_service.get_run(user_id=3001, run_id=run_id)
    events = event_store.list_after(run_id=run_id, after_sequence_no=0)

    assert saved_run.status == "cancelled"
    assert events[-1].event_type == RUN_EVENT_TYPE_RUN_CANCELLED


@pytest.mark.anyio
async def test_resume_rejects_tool_call_from_other_run():
    executor, run_service, _message_service, _event_store, tool_call_repo, thread_id, run_id = build_executor()
    run = run_service.get_active_run(user_id=3001, thread_id=thread_id)
    assert run is not None
    run_service.mark_requires_action(run_id=run.id)
    tool_call_repo.create(
        ToolCallRecord(
            id="tool_other",
            run_id="run_other",
            thread_id=thread_id,
            user_id=3001,
            tool_name="human_approval",
            status=TOOL_CALL_STATUS_WAITING_HUMAN,
            arguments={"action": "refund_order", "values": {"order_id": "ORD-1"}},
            created_at=datetime.now(timezone.utc),
            updated_at=datetime.now(timezone.utc),
        )
    )

    with pytest.raises(ApiError):
        await executor.resume(run.id, "tool_other", {"action": "approve", "values": {}})


@pytest.mark.anyio
async def test_executor_runtime_failure_marks_run_failed_and_appends_terminal_event():
    executor, run_service, message_service, event_store, _tool_call_repo, thread_id, run_id = build_executor(
        runtime=FailingRuntime()
    )
    run = run_service.get_active_run(user_id=3001, thread_id=thread_id)
    assert run is not None

    await executor.start(run_id)

    saved_run = run_service.get_run(user_id=3001, run_id=run_id)
    messages, _ = message_service.list_messages(user_id=3001, thread_id=thread_id, limit=20, before=None)
    events = event_store.list_after(run_id=run_id, after_sequence_no=0)

    assert saved_run.status == "failed"
    assert messages[1].status == "completed"
    assert events[-1].event_type == RUN_EVENT_TYPE_RUN_FAILED
