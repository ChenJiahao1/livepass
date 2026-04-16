from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any

RUN_EVENT_TYPE_RUN_CREATED = "run_created"
RUN_EVENT_TYPE_RUN_STARTED = "run_started"
RUN_EVENT_TYPE_RUN_RESUMED = "run_resumed"
RUN_EVENT_TYPE_RUN_PAUSED = "run_paused"
RUN_EVENT_TYPE_RUN_COMPLETED = "run_completed"
RUN_EVENT_TYPE_RUN_FAILED = "run_failed"
RUN_EVENT_TYPE_RUN_CANCELLED = "run_cancelled"
RUN_EVENT_TYPE_MESSAGE_DELTA = "message_delta"
RUN_EVENT_TYPE_TOOL_CALL_STARTED = "tool_call_started"
RUN_EVENT_TYPE_TOOL_CALL_REQUIRES_HUMAN = "tool_call_requires_human"
RUN_EVENT_TYPE_TOOL_CALL_COMPLETED = "tool_call_completed"
RUN_EVENT_TYPE_TOOL_CALL_FAILED = "tool_call_failed"


@dataclass(slots=True)
class RunEventRecord:
    id: str
    run_id: str
    thread_id: str
    user_id: int
    sequence_no: int
    event_type: str
    payload: dict[str, Any] = field(default_factory=dict)
    created_at: datetime | None = None
