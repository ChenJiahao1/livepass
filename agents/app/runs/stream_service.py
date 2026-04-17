from __future__ import annotations

import asyncio

from app.runs.event_bus import RunEventBus
from app.runs.event_models import (
    RUN_EVENT_TYPE_RUN_CANCELLED,
    RUN_EVENT_TYPE_RUN_COMPLETED,
    RUN_EVENT_TYPE_RUN_FAILED,
    RUN_EVENT_TYPE_TOOL_CALL_WAITING_HUMAN,
    RunEventRecord,
)
from app.runs.event_store import RunEventStore

TERMINAL_EVENT_TYPES = {
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

    def serialize_event(self, event: RunEventRecord, *, debug: dict | None = None) -> dict:
        payload = {
            "type": event.event_type,
            "sequenceNo": event.sequence_no,
            "threadId": event.thread_id,
            "runId": event.run_id,
            "timestamp": event.created_at.isoformat().replace("+00:00", "Z") if event.created_at else None,
        }
        payload.update(dict(event.payload))
        if event.event_type == "message.delta" and event.message_id is not None:
            payload["messageId"] = event.message_id
        if debug:
            payload["debug"] = dict(debug)
        return payload

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
                        if self._should_close_stream(event):
                            return
                    continue
                latest_event = self.event_store.latest(run_id=run_id)
                if latest_event is not None and latest_event.sequence_no <= current and self._should_close_stream(latest_event):
                    return
                try:
                    await asyncio.wait_for(queue.get(), timeout=self.poll_interval_seconds)
                except asyncio.TimeoutError:
                    await asyncio.sleep(self.poll_interval_seconds)
        finally:
            self.event_bus.unsubscribe(run_id=run_id, queue=queue)

    def _should_close_stream(self, event: RunEventRecord) -> bool:
        if event.event_type in TERMINAL_EVENT_TYPES:
            return True
        return event.event_type == RUN_EVENT_TYPE_TOOL_CALL_WAITING_HUMAN
