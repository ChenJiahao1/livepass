from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any

MESSAGE_ROLE_USER = "user"
MESSAGE_ROLE_ASSISTANT = "assistant"
MESSAGE_STATUS_IN_PROGRESS = "in_progress"
MESSAGE_STATUS_COMPLETED = "completed"
MESSAGE_STATUS_ERROR = "error"


@dataclass(slots=True)
class MessagePart:
    type: str
    text: str


@dataclass(slots=True)
class MessageRecord:
    id: str
    thread_id: str
    user_id: int
    role: str
    parts: list[dict[str, Any]] = field(default_factory=list)
    status: str = MESSAGE_STATUS_COMPLETED
    run_id: str | None = None
    created_at: datetime | None = None
    metadata: dict[str, Any] = field(default_factory=dict)
