from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any

THREAD_STATUS_ACTIVE = "active"
THREAD_STATUS_ARCHIVED = "archived"


@dataclass(slots=True)
class ThreadRecord:
    id: str
    user_id: int
    title: str
    status: str
    created_at: datetime
    updated_at: datetime
    last_message_at: datetime | None = None
    active_run_id: str | None = None
    metadata: dict[str, Any] = field(default_factory=dict)
