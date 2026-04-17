from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any

TOOL_CALL_STATUS_IN_PROGRESS = "in_progress"
TOOL_CALL_STATUS_WAITING_HUMAN = "waiting_human"
TOOL_CALL_STATUS_COMPLETED = "completed"
TOOL_CALL_STATUS_FAILED = "failed"
TOOL_CALL_STATUS_CANCELLED = "cancelled"


@dataclass(slots=True)
class ToolCallRecord:
    id: str
    run_id: str
    message_id: str | None
    thread_id: str
    user_id: int
    name: str
    status: str
    input: dict[str, Any] = field(default_factory=dict)
    human_request: dict[str, Any] = field(default_factory=dict)
    output: dict[str, Any] | None = None
    error: dict[str, Any] | None = None
    created_at: datetime | None = None
    updated_at: datetime | None = None
    completed_at: datetime | None = None
    metadata: dict[str, Any] = field(default_factory=dict)
