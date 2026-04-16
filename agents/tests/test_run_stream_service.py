from __future__ import annotations

import asyncio

import pytest

from app.runs.event_bus import RunEventBus
from app.runs.event_models import RUN_EVENT_TYPE_MESSAGE_DELTA, RUN_EVENT_TYPE_RUN_COMPLETED
from app.runs.event_store import InMemoryRunEventStore
from app.runs.stream_service import RunStreamService


@pytest.mark.anyio
async def test_stream_replays_history_then_tails_incremental_events():
    store = InMemoryRunEventStore()
    bus = RunEventBus()
    service = RunStreamService(event_store=store, event_bus=bus)

    first = store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_MESSAGE_DELTA,
        payload={"messageId": "msg_01", "delta": "你"},
        now=__import__("datetime").datetime.now(__import__("datetime").timezone.utc),
    )

    events: list[str] = []

    async def _collect():
        async for event in service.stream_events(run_id="run_01", after_sequence_no=0):
            events.append(event.event_type)
            if event.event_type == RUN_EVENT_TYPE_RUN_COMPLETED:
                break

    task = asyncio.create_task(_collect())
    await asyncio.sleep(0)

    second = store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_RUN_COMPLETED,
        payload={"status": "completed"},
        now=__import__("datetime").datetime.now(__import__("datetime").timezone.utc),
    )
    bus.publish(run_id="run_01", sequence_no=second.sequence_no)
    await task

    assert first.sequence_no == 1
    assert second.sequence_no == 2
    assert events == [RUN_EVENT_TYPE_MESSAGE_DELTA, RUN_EVENT_TYPE_RUN_COMPLETED]


@pytest.mark.anyio
async def test_stream_after_cursor_skips_old_message_delta():
    store = InMemoryRunEventStore()
    bus = RunEventBus()
    service = RunStreamService(event_store=store, event_bus=bus)

    store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_MESSAGE_DELTA,
        payload={"messageId": "msg_01", "delta": "你"},
        now=__import__("datetime").datetime.now(__import__("datetime").timezone.utc),
    )
    store.append(
        run_id="run_01",
        thread_id="thr_01",
        user_id=3001,
        event_type=RUN_EVENT_TYPE_RUN_COMPLETED,
        payload={"status": "completed"},
        now=__import__("datetime").datetime.now(__import__("datetime").timezone.utc),
    )

    events = []
    async for event in service.stream_events(run_id="run_01", after_sequence_no=1):
        events.append(event.event_type)

    assert events == [RUN_EVENT_TYPE_RUN_COMPLETED]
