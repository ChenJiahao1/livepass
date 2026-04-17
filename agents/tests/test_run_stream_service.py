from __future__ import annotations

import asyncio
from datetime import datetime, timezone

import pytest

from app.runs.event_bus import RunEventBus
from app.runs.event_models import (
    RUN_EVENT_TYPE_MESSAGE_DELTA,
    RUN_EVENT_TYPE_RUN_COMPLETED,
    RUN_EVENT_TYPE_RUN_CREATED,
    RUN_EVENT_TYPE_RUN_UPDATED,
    RUN_EVENT_TYPE_TOOL_CALL_WAITING_HUMAN,
)
from app.runs.event_store import InMemoryRunEventStore
from app.runs.stream_service import RunStreamService


def test_serialize_event_uses_spec_envelope_without_debug_by_default():
    now = datetime(2026, 4, 17, 10, 0, tzinfo=timezone.utc)
    store = InMemoryRunEventStore()
    bus = RunEventBus()
    service = RunStreamService(event_store=store, event_bus=bus)

    event = store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_MESSAGE_DELTA,
        message_id="msg_01",
        payload={"delta": {"type": "text", "text": "正在帮你查询订单"}},
        now=now,
    )

    payload = service.serialize_event(event)

    assert payload == {
        "type": RUN_EVENT_TYPE_MESSAGE_DELTA,
        "sequenceNo": 1,
        "threadId": "thr_01",
        "runId": "run_01",
        "timestamp": "2026-04-17T10:00:00Z",
        "messageId": "msg_01",
        "delta": {"type": "text", "text": "正在帮你查询订单"},
    }
    assert "debug" not in payload
    assert "payload" not in payload
    assert "schemaVersion" not in payload


def test_serialize_event_uses_nested_message_and_tool_call_snapshots():
    now = datetime(2026, 4, 17, 10, 0, tzinfo=timezone.utc)
    store = InMemoryRunEventStore()
    bus = RunEventBus()
    service = RunStreamService(event_store=store, event_bus=bus)

    event = store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_TOOL_CALL_WAITING_HUMAN,
        message_id="msg_01",
        tool_call_id="tc_01",
        payload={
            "toolCall": {
                "id": "tc_01",
                "messageId": "msg_01",
                "name": "refund_apply",
                "status": "waiting_human",
                "input": {"orderId": "ORD-1"},
                "humanRequest": {"kind": "approval", "title": "确认退款", "allowedActions": ["approve", "reject", "edit"]},
            }
        },
        now=now,
    )

    payload = service.serialize_event(event)

    assert payload == {
        "type": RUN_EVENT_TYPE_TOOL_CALL_WAITING_HUMAN,
        "runId": "run_01",
        "threadId": "thr_01",
        "sequenceNo": 1,
        "timestamp": "2026-04-17T10:00:00Z",
        "toolCall": {
            "id": "tc_01",
            "messageId": "msg_01",
            "name": "refund_apply",
            "status": "waiting_human",
            "input": {"orderId": "ORD-1"},
            "humanRequest": {"kind": "approval", "title": "确认退款", "allowedActions": ["approve", "reject", "edit"]},
        },
    }


def test_serialize_event_uses_nested_message_snapshot_for_completed_message():
    now = datetime(2026, 4, 17, 10, 0, tzinfo=timezone.utc)
    store = InMemoryRunEventStore()
    bus = RunEventBus()
    service = RunStreamService(event_store=store, event_bus=bus)

    event = store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type="message.completed",
        message_id="msg_01",
        payload={"message": {"id": "msg_01", "status": "completed", "content": [{"type": "text", "text": "已完成"}]}},
        now=now,
    )

    payload = service.serialize_event(event)

    assert payload == {
        "type": "message.completed",
        "sequenceNo": 1,
        "threadId": "thr_01",
        "runId": "run_01",
        "timestamp": "2026-04-17T10:00:00Z",
        "message": {"id": "msg_01", "status": "completed", "content": [{"type": "text", "text": "已完成"}]},
    }


@pytest.mark.anyio
async def test_stream_replays_history_then_tails_incremental_events_without_duplicates():
    store = InMemoryRunEventStore()
    bus = RunEventBus()
    service = RunStreamService(event_store=store, event_bus=bus)

    store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_RUN_CREATED,
        payload={"status": "queued"},
        now=datetime.now(timezone.utc),
    )
    store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_MESSAGE_DELTA,
        message_id="msg_01",
        payload={"delta": {"type": "text", "text": "正在帮你查询订单"}},
        now=datetime.now(timezone.utc),
    )

    events: list[int] = []

    async def _collect():
        async for event in service.stream_events(run_id="run_01", after_sequence_no=0):
            events.append(event.sequence_no)
            if event.event_type == RUN_EVENT_TYPE_RUN_COMPLETED and event.payload["status"] == "completed":
                break

    task = asyncio.create_task(_collect())
    await asyncio.sleep(0)

    third = store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_RUN_UPDATED,
        payload={"status": "running"},
        now=datetime.now(timezone.utc),
    )
    bus.publish(run_id="run_01", sequence_no=third.sequence_no)
    fourth = store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_RUN_COMPLETED,
        payload={"status": "completed"},
        now=datetime.now(timezone.utc),
    )
    bus.publish(run_id="run_01", sequence_no=fourth.sequence_no)
    await task

    assert events == [1, 2, 3, 4]


@pytest.mark.anyio
async def test_stream_after_cursor_only_replays_strictly_greater_sequence():
    store = InMemoryRunEventStore()
    bus = RunEventBus()
    service = RunStreamService(event_store=store, event_bus=bus)

    store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_RUN_CREATED,
        payload={"status": "queued"},
        now=datetime.now(timezone.utc),
    )
    store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_MESSAGE_DELTA,
        message_id="msg_01",
        payload={"delta": {"type": "text", "text": "正在帮你查询订单"}},
        now=datetime.now(timezone.utc),
    )
    store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_RUN_COMPLETED,
        payload={"status": "completed"},
        now=datetime.now(timezone.utc),
    )

    events = []
    async for event in service.stream_events(run_id="run_01", after_sequence_no=2):
        events.append(event.sequence_no)
        break

    assert events == [3]


@pytest.mark.anyio
async def test_stream_closes_after_requires_action_pause():
    store = InMemoryRunEventStore()
    bus = RunEventBus()
    service = RunStreamService(event_store=store, event_bus=bus)

    store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_RUN_CREATED,
        payload={"status": "queued"},
        now=datetime.now(timezone.utc),
    )
    store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_TOOL_CALL_WAITING_HUMAN,
        payload={"status": "waiting_human"},
        now=datetime.now(timezone.utc),
    )

    events = []
    async for event in service.stream_events(run_id="run_01", after_sequence_no=0):
        events.append((event.sequence_no, event.event_type, event.payload["status"]))

    assert events == [(1, RUN_EVENT_TYPE_RUN_CREATED, "queued"), (2, RUN_EVENT_TYPE_TOOL_CALL_WAITING_HUMAN, "waiting_human")]


@pytest.mark.anyio
async def test_stream_closes_when_cursor_is_beyond_paused_run_latest_event():
    store = InMemoryRunEventStore()
    bus = RunEventBus()
    service = RunStreamService(event_store=store, event_bus=bus)

    store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_RUN_CREATED,
        payload={"status": "queued"},
        now=datetime.now(timezone.utc),
    )
    store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_TOOL_CALL_WAITING_HUMAN,
        payload={"status": "waiting_human"},
        now=datetime.now(timezone.utc),
    )

    async def _collect():
        events = []
        async for event in service.stream_events(run_id="run_01", after_sequence_no=12):
            events.append(event.sequence_no)
        return events

    assert await asyncio.wait_for(_collect(), timeout=0.2) == []
