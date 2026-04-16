from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any

RUN_EVENT_TYPE_RUN_CREATED = "run.created"
RUN_EVENT_TYPE_RUN_UPDATED = "run.updated"
RUN_EVENT_TYPE_RUN_COMPLETED = "run.completed"
RUN_EVENT_TYPE_RUN_FAILED = "run.failed"
RUN_EVENT_TYPE_RUN_CANCELLED = "run.cancelled"
RUN_EVENT_TYPE_MESSAGE_CREATED = "message.created"
RUN_EVENT_TYPE_MESSAGE_DELTA = "message.delta"
RUN_EVENT_TYPE_MESSAGE_UPDATED = "message.updated"
RUN_EVENT_TYPE_TOOL_CALL_CREATED = "tool_call.created"
RUN_EVENT_TYPE_TOOL_CALL_UPDATED = "tool_call.updated"
RUN_EVENT_TYPE_TOOL_CALL_PROGRESS = "tool_call.progress"
RUN_EVENT_TYPE_TOOL_CALL_COMPLETED = "tool_call.completed"
RUN_EVENT_TYPE_TOOL_CALL_FAILED = "tool_call.failed"
RUN_EVENT_TYPE_RUN_PROGRESS = "run.progress"


@dataclass(slots=True)
class RunEventRecord:
    id: str
    run_id: str
    thread_id: str
    user_id: int
    sequence_no: int
    event_type: str
    message_id: str | None = None
    tool_call_id: str | None = None
    payload: dict[str, Any] = field(default_factory=dict)
    created_at: datetime | None = None
