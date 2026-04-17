from __future__ import annotations

from datetime import datetime, timezone

import pytest
from langgraph.types import Command

from app.agent_runtime.service import AgentRuntimeService
from app.messages.models import MESSAGE_STATUS_ERROR
from app.messages.repository import InMemoryMessageRepository
from app.messages.service import MessageService
from app.runs.event_bus import RunEventBus
from app.runs.event_models import (
    RUN_EVENT_TYPE_MESSAGE_CREATED,
    RUN_EVENT_TYPE_MESSAGE_COMPLETED,
    RUN_EVENT_TYPE_MESSAGE_FAILED,
    RUN_EVENT_TYPE_MESSAGE_CANCELLED,
    RUN_EVENT_TYPE_MESSAGE_DELTA,
    RUN_EVENT_TYPE_RUN_CANCELLED,
    RUN_EVENT_TYPE_RUN_COMPLETED,
    RUN_EVENT_TYPE_RUN_CREATED,
    RUN_EVENT_TYPE_RUN_FAILED,
    RUN_EVENT_TYPE_RUN_PROGRESS,
    RUN_EVENT_TYPE_RUN_UPDATED,
    RUN_EVENT_TYPE_TOOL_CALL_CREATED,
    RUN_EVENT_TYPE_TOOL_CALL_COMPLETED,
    RUN_EVENT_TYPE_TOOL_CALL_FAILED,
    RUN_EVENT_TYPE_TOOL_CALL_PROGRESS,
    RUN_EVENT_TYPE_TOOL_CALL_UPDATED,
    RUN_EVENT_TYPE_TOOL_CALL_WAITING_HUMAN,
)
from app.runs.event_store import InMemoryRunEventStore
from app.runs.executor import RunExecutor
from app.runs.repository import InMemoryRunRepository
from app.runs.service import RunService
from app.runs.tool_call_repository import InMemoryToolCallRepository
from app.threads.repository import InMemoryThreadRepository
from app.threads.service import ThreadService


class FakeRuntime:
    async def ainvoke(self, state_payload, config, context):
        del config
        del context
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
        del state_payload
        del config
        del context
        raise RuntimeError("runtime exploded")


class PauseRuntime:
    async def ainvoke(self, state_payload, config, context):
        del config
        del context
        if isinstance(state_payload, Command):
            return {
                "reply": "恢复后不应继续执行",
                "final_reply": "恢复后不应继续执行",
                "current_agent": "refund",
                "need_handoff": False,
                "route_source": "rule",
            }
        return {
            "reply": "",
            "final_reply": "",
            "current_agent": "refund",
            "need_handoff": False,
            "route_source": "rule",
            "tool_call": {
                "toolName": "human_approval",
                "args": {
                    "action": "refund_order",
                    "orderId": "ORD-1",
                    "values": {"order_id": "ORD-1", "reason": "用户发起退款", "user_id": "3001"},
                },
                "request": {"title": "退款前确认", "description": "订单 ORD-1 预计退款 100"},
            },
        }


class FailingResumeRuntime(PauseRuntime):
    async def astream(self, state_payload, config, context, stream_mode):
        del config
        del context
        del stream_mode
        if isinstance(state_payload, Command):
            raise RuntimeError("resume exploded")
        yield (
            "updates",
            {
                "__interrupt__": (
                    _FakeInterrupt(
                        {
                            "toolName": "human_approval",
                            "args": {
                                "action": "refund_order",
                                "orderId": "ORD-1",
                                "values": {"order_id": "ORD-1", "reason": "用户发起退款", "user_id": "3001"},
                            },
                            "request": {"title": "退款前确认", "description": "订单 ORD-1 预计退款 100"},
                        }
                    ),
                )
            },
        )


class _FakeInterrupt:
    def __init__(self, value: dict) -> None:
        self.value = value


class StreamOnlyRuntime:
    def __init__(self) -> None:
        self.stream_modes: list[list[str]] = []

    async def ainvoke(self, state_payload, config, context):
        del state_payload
        del config
        del context
        raise AssertionError("run executor should project chunks from astream instead of ainvoke result")

    async def astream(self, state_payload, config, context, stream_mode):
        del state_payload
        del config
        del context
        self.stream_modes.append(list(stream_mode))
        yield ("updates", {"run": {"status": "running"}})
        yield ("messages", {"delta": "正在查询订单"})
        yield ("custom", {"tool": {"name": "preview_refund_order", "status": "completed"}})
        yield ("custom", {"progress": {"message": "正在整理结果"}})


def build_executor(
    *,
    runtime: object | None = None,
) -> tuple[
    RunExecutor,
    RunService,
    ThreadService,
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
    event_bus = RunEventBus()
    tool_call_repo = InMemoryToolCallRepository()

    message_service = MessageService(
        thread_repository=thread_repo,
        message_repository=message_repo,
    )
    run_service = RunService(
        run_repository=run_repo,
        message_service=message_service,
    )
    thread_service = ThreadService(
        thread_repository=thread_repo,
        run_repository=run_repo,
    )
    runtime_service = AgentRuntimeService(
        agent_runtime=runtime or FakeRuntime(),
        registry=object(),
        llm=object(),
    )
    executor = RunExecutor(
        run_repository=run_repo,
        run_service=run_service,
        message_service=message_service,
        event_store=event_store,
        event_bus=event_bus,
        tool_call_repository=tool_call_repo,
        runtime_service=runtime_service,
    )

    thread = thread_service.create_thread(user_id=3001, title="退款")
    run, _, _ = run_service.create_run(
        user_id=3001,
        thread_id=thread.id,
        content=[{"type": "text", "text": "帮我查订单"}],
    )
    return executor, run_service, thread_service, message_service, event_store, tool_call_repo, thread.id, run.id


@pytest.mark.anyio
async def test_executor_start_persists_run_created_message_created_and_terminal_snapshots():
    executor, run_service, _thread_service, message_service, event_store, _tool_call_repo, thread_id, run_id = build_executor()

    await executor.start(run_id)

    saved_run = run_service.get_run(user_id=3001, run_id=run_id)
    messages, _ = message_service.list_thread_messages(user_id=3001, thread_id=thread_id, limit=20, before=None)
    events = event_store.list_after(run_id=run_id, after_sequence_no=0)

    assert saved_run.status == "completed"
    assert [event.event_type for event in events] == [
        RUN_EVENT_TYPE_RUN_CREATED,
        RUN_EVENT_TYPE_MESSAGE_CREATED,
        RUN_EVENT_TYPE_RUN_UPDATED,
        RUN_EVENT_TYPE_MESSAGE_DELTA,
        RUN_EVENT_TYPE_MESSAGE_COMPLETED,
        RUN_EVENT_TYPE_RUN_COMPLETED,
    ]
    assert [event.sequence_no for event in events] == [1, 2, 3, 4, 5, 6]
    assert all(event.run_id == run_id for event in events)
    assert all(event.thread_id == thread_id for event in events)
    assert events[1].message_id == saved_run.output_message_id
    assert events[3].message_id == saved_run.output_message_id
    assert events[3].payload["delta"] == {"type": "text", "text": "已处理：帮我查订单"}
    assert events[4].payload["message"]["status"] == "completed"
    assert events[4].payload["message"]["content"] == [{"type": "text", "text": "已处理：帮我查订单"}]
    assert events[5].payload["run"]["status"] == "completed"
    assert messages[1].content == [{"type": "text", "text": "已处理：帮我查订单"}]


@pytest.mark.anyio
async def test_executor_runtime_failure_persists_failed_message_snapshot():
    executor, run_service, _thread_service, message_service, event_store, _tool_call_repo, thread_id, run_id = build_executor(
        runtime=FailingRuntime()
    )

    await executor.start(run_id)

    saved_run = run_service.get_run(user_id=3001, run_id=run_id)
    messages, _ = message_service.list_thread_messages(user_id=3001, thread_id=thread_id, limit=20, before=None)
    events = event_store.list_after(run_id=run_id, after_sequence_no=0)

    assert saved_run.status == "failed"
    assert messages[1].status == MESSAGE_STATUS_ERROR
    assert events[-2].event_type == RUN_EVENT_TYPE_MESSAGE_FAILED
    assert events[-2].payload["message"]["status"] == "failed"
    assert events[-1].event_type == RUN_EVENT_TYPE_RUN_FAILED
    assert events[-1].payload["run"]["status"] == "failed"


@pytest.mark.anyio
async def test_cancel_waiting_human_run_closes_tool_and_message_snapshots():
    executor, run_service, thread_service, message_service, event_store, tool_call_repo, thread_id, run_id = build_executor(
        runtime=PauseRuntime()
    )

    await executor.start(run_id)
    tool_call_id = next(iter(tool_call_repo._tool_calls.keys()))
    paused_events = event_store.list_after(run_id=run_id, after_sequence_no=0)

    await executor.cancel(run_id)

    saved_run = run_service.get_run(user_id=3001, run_id=run_id)
    thread = thread_service.get_thread(user_id=3001, thread_id=thread_id)
    messages, _ = message_service.list_thread_messages(user_id=3001, thread_id=thread_id, limit=20, before=None)
    events = event_store.list_after(run_id=run_id, after_sequence_no=0)

    assert [event.event_type for event in paused_events] == [
        RUN_EVENT_TYPE_RUN_CREATED,
        RUN_EVENT_TYPE_MESSAGE_CREATED,
        RUN_EVENT_TYPE_RUN_UPDATED,
        RUN_EVENT_TYPE_TOOL_CALL_CREATED,
        RUN_EVENT_TYPE_RUN_UPDATED,
        RUN_EVENT_TYPE_TOOL_CALL_WAITING_HUMAN,
    ]
    assert paused_events[1].message_id == saved_run.output_message_id
    assert paused_events[3].message_id == saved_run.output_message_id
    assert paused_events[3].tool_call_id == tool_call_id
    assert paused_events[3].payload["toolCall"]["messageId"] == saved_run.output_message_id
    assert paused_events[3].payload["toolCall"]["humanRequest"] == paused_events[5].payload["toolCall"]["humanRequest"]
    assert paused_events[5].message_id == saved_run.output_message_id
    assert paused_events[5].tool_call_id == tool_call_id

    assert saved_run.status == "cancelled"
    assert thread.active_run_id is None
    assert messages[1].status == "cancelled"
    assert [event.event_type for event in events[-3:]] == [
        RUN_EVENT_TYPE_TOOL_CALL_UPDATED,
        RUN_EVENT_TYPE_MESSAGE_CANCELLED,
        RUN_EVENT_TYPE_RUN_CANCELLED,
    ]
    assert events[-3].payload["toolCall"]["status"] == "cancelled"
    assert events[-3].payload["toolCall"]["messageId"] == saved_run.output_message_id
    assert events[-3].payload["toolCall"]["humanRequest"] == paused_events[5].payload["toolCall"]["humanRequest"]
    assert events[-2].payload["message"]["status"] == "cancelled"
    assert events[-1].payload["run"]["status"] == "cancelled"


@pytest.mark.anyio
async def test_resume_projects_completed_tool_call_with_same_snapshot_shape():
    executor, run_service, _thread_service, _message_service, event_store, tool_call_repo, _thread_id, run_id = build_executor(
        runtime=PauseRuntime()
    )

    await executor.start(run_id)
    tool_call_id = next(iter(tool_call_repo._tool_calls.keys()))
    waiting_event = event_store.list_after(run_id=run_id, after_sequence_no=0)[-1]

    await executor.resume(run_id, tool_call_id, {"action": "approve", "values": {}})

    events = event_store.list_after(run_id=run_id, after_sequence_no=0)
    completed_event = next(event for event in events if event.event_type == RUN_EVENT_TYPE_TOOL_CALL_COMPLETED)
    saved_run = run_service.get_run(user_id=3001, run_id=run_id)

    assert completed_event.payload["toolCall"]["id"] == tool_call_id
    assert completed_event.payload["toolCall"]["messageId"] == saved_run.output_message_id
    assert completed_event.payload["toolCall"]["name"] == "human_approval"
    assert completed_event.payload["toolCall"]["status"] == "completed"
    assert completed_event.payload["toolCall"]["input"] == waiting_event.payload["toolCall"]["input"]
    assert completed_event.payload["toolCall"]["humanRequest"] == waiting_event.payload["toolCall"]["humanRequest"]
    assert completed_event.payload["toolCall"]["output"] == {"action": "approve"}


@pytest.mark.anyio
async def test_resume_projects_failed_tool_call_with_same_snapshot_shape():
    executor, run_service, _thread_service, _message_service, event_store, tool_call_repo, _thread_id, run_id = build_executor(
        runtime=FailingResumeRuntime()
    )

    await executor.start(run_id)
    tool_call_id = next(iter(tool_call_repo._tool_calls.keys()))
    waiting_event = event_store.list_after(run_id=run_id, after_sequence_no=0)[-1]

    await executor.resume(run_id, tool_call_id, {"action": "approve", "values": {}})

    events = event_store.list_after(run_id=run_id, after_sequence_no=0)
    failed_event = next(event for event in events if event.event_type == RUN_EVENT_TYPE_TOOL_CALL_FAILED)
    saved_run = run_service.get_run(user_id=3001, run_id=run_id)

    assert failed_event.payload["toolCall"]["id"] == tool_call_id
    assert failed_event.payload["toolCall"]["messageId"] == saved_run.output_message_id
    assert failed_event.payload["toolCall"]["name"] == "human_approval"
    assert failed_event.payload["toolCall"]["status"] == "failed"
    assert failed_event.payload["toolCall"]["input"] == waiting_event.payload["toolCall"]["input"]
    assert failed_event.payload["toolCall"]["humanRequest"] == waiting_event.payload["toolCall"]["humanRequest"]
    assert failed_event.payload["toolCall"]["error"] == {"message": "resume exploded"}


@pytest.mark.anyio
async def test_executor_projects_langgraph_stream_chunks_without_result_dicts():
    runtime = StreamOnlyRuntime()
    executor, run_service, _thread_service, message_service, event_store, _tool_call_repo, thread_id, run_id = build_executor(
        runtime=runtime
    )

    await executor.start(run_id)

    saved_run = run_service.get_run(user_id=3001, run_id=run_id)
    messages, _ = message_service.list_thread_messages(user_id=3001, thread_id=thread_id, limit=20, before=None)
    events = event_store.list_after(run_id=run_id, after_sequence_no=0)

    assert saved_run.status == "completed"
    assert runtime.stream_modes == [["messages", "updates", "custom"]]
    assert [event.event_type for event in events] == [
        RUN_EVENT_TYPE_RUN_CREATED,
        RUN_EVENT_TYPE_MESSAGE_CREATED,
        RUN_EVENT_TYPE_RUN_UPDATED,
        RUN_EVENT_TYPE_MESSAGE_DELTA,
        RUN_EVENT_TYPE_TOOL_CALL_PROGRESS,
        RUN_EVENT_TYPE_RUN_PROGRESS,
        RUN_EVENT_TYPE_MESSAGE_COMPLETED,
        RUN_EVENT_TYPE_RUN_COMPLETED,
    ]
    tool_progress_event = next(event for event in events if event.event_type == RUN_EVENT_TYPE_TOOL_CALL_PROGRESS)
    assert tool_progress_event.payload["toolCall"]["messageId"] == saved_run.output_message_id
    assert tool_progress_event.payload["toolCall"]["name"] == "preview_refund_order"
    assert tool_progress_event.payload["toolCall"]["status"] == "completed"
    assert tool_progress_event.payload["toolCall"]["input"] == {}
    assert messages[1].content == [{"type": "text", "text": "正在查询订单"}]
