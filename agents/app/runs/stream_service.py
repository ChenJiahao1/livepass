from __future__ import annotations

import asyncio

from app.runs.event_bus import RunEventBus
from app.runs.event_models import (
    RUN_EVENT_TYPE_RUN_CANCELLED,
    RUN_EVENT_TYPE_RUN_COMPLETED,
    RUN_EVENT_TYPE_RUN_FAILED,
    RUN_EVENT_TYPE_RUN_PAUSED,
    RunEventRecord,
)
from app.runs.event_store import RunEventStore

TERMINAL_EVENT_TYPES = {
    RUN_EVENT_TYPE_RUN_PAUSED,
    RUN_EVENT_TYPE_RUN_COMPLETED,
    RUN_EVENT_TYPE_RUN_FAILED,
    RUN_EVENT_TYPE_RUN_CANCELLED,
}


class RunStreamService:
    def __init__(
        self,
        *,
        event_store: RunEventStore,
        event_bus: RunEventBus,
        poll_interval_seconds: float = 0.05,
    ) -> None:
        self.event_store = event_store
        self.event_bus = event_bus
        self.poll_interval_seconds = poll_interval_seconds

    async def stream_events(self, *, run_id: str, after_sequence_no: int):
        current = after_sequence_no
        queue = self.event_bus.subscribe(run_id=run_id)
        try:
            while True:
                events = self.event_store.list_after(run_id=run_id, after_sequence_no=current)
                if events:
                    for event in events:
                        current = event.sequence_no
                        yield event
                        if event.event_type in TERMINAL_EVENT_TYPES:
                            return
                    continue
                try:
                    await asyncio.wait_for(queue.get(), timeout=self.poll_interval_seconds)
                except asyncio.TimeoutError:
                    await asyncio.sleep(self.poll_interval_seconds)
        finally:
            self.event_bus.unsubscribe(run_id=run_id, queue=queue)
