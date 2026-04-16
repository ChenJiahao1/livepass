from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any

RUN_STATUS_QUEUED = "queued"
RUN_STATUS_RUNNING = "running"
RUN_STATUS_REQUIRES_ACTION = "requires_action"
RUN_STATUS_COMPLETED = "completed"
RUN_STATUS_FAILED = "failed"
RUN_STATUS_CANCELLED = "cancelled"


@dataclass(slots=True)
class RunError:
    code: str
    message: str
    details: dict[str, Any] = field(default_factory=dict)


@dataclass(slots=True)
class RunRecord:
    id: str
    thread_id: str
    user_id: int
    trigger_message_id: str
    status: str
    started_at: datetime
    completed_at: datetime | None = None
    error: dict[str, Any] | None = None
    metadata: dict[str, Any] = field(default_factory=dict)
