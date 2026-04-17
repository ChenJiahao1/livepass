from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any

MESSAGE_ROLE_USER = "user"
MESSAGE_ROLE_ASSISTANT = "assistant"
MESSAGE_STATUS_STREAMING = "streaming"
MESSAGE_STATUS_IN_PROGRESS = MESSAGE_STATUS_STREAMING
MESSAGE_STATUS_COMPLETED = "completed"
MESSAGE_STATUS_ERROR = "failed"
MESSAGE_STATUS_CANCELLED = "cancelled"


@dataclass(slots=True)
class MessageContent:
    type: str
    text: str


@dataclass(slots=True)
class MessageRecord:
    id: str
    thread_id: str
    user_id: int
    role: str
    content: list[dict[str, Any]] = field(default_factory=list)
    status: str = MESSAGE_STATUS_COMPLETED
    run_id: str | None = None
    created_at: datetime | None = None
    updated_at: datetime | None = None
    metadata: dict[str, Any] = field(default_factory=dict)
