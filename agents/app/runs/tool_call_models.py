from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Any

TOOL_CALL_STATUS_IN_PROGRESS = "in_progress"
TOOL_CALL_STATUS_WAITING_HUMAN = "waiting_human"
TOOL_CALL_STATUS_COMPLETED = "completed"
TOOL_CALL_STATUS_FAILED = "failed"
TOOL_CALL_STATUS_CANCELLED = "cancelled"


@dataclass(slots=True, init=False)
class ToolCallRecord:
    id: str
    run_id: str
    message_id: str
    thread_id: str
    user_id: int
    message_id: str | None
    tool_name: str
    status: str
    arguments: dict[str, Any] = field(default_factory=dict)
    request: dict[str, Any] = field(default_factory=dict)
    output: dict[str, Any] | None = None
    error: dict[str, Any] | None = None
    created_at: datetime | None = None
    updated_at: datetime | None = None
    completed_at: datetime | None = None
    metadata: dict[str, Any] = field(default_factory=dict)

    def __init__(
        self,
        *,
        id: str,
        run_id: str,
        thread_id: str,
        user_id: int,
        tool_name: str,
        status: str,
        message_id: str | None = None,
        arguments: dict[str, Any] | None = None,
        request: dict[str, Any] | None = None,
        human_request: dict[str, Any] | None = None,
        output: dict[str, Any] | None = None,
        result: dict[str, Any] | None = None,
        error: dict[str, Any] | None = None,
        created_at: datetime | None = None,
        updated_at: datetime | None = None,
        completed_at: datetime | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> None:
        self.id = id
        self.run_id = run_id
        self.thread_id = thread_id
        self.user_id = user_id
        self.message_id = message_id
        self.tool_name = tool_name
        self.status = status
        self.arguments = dict(arguments or {})
        self.request = dict(human_request if human_request is not None else request or {})
        resolved_result = result if result is not None else output
        self.output = dict(resolved_result) if isinstance(resolved_result, dict) else resolved_result
        self.error = dict(error) if isinstance(error, dict) else error
        self.created_at = created_at
        self.updated_at = updated_at
        self.completed_at = completed_at
        self.metadata = dict(metadata or {})

    @property
    def human_request(self) -> dict[str, Any]:
        return self.request

    @human_request.setter
    def human_request(self, value: dict[str, Any]) -> None:
        self.request = dict(value)

    @property
    def result(self) -> dict[str, Any] | None:
        return self.output

    @result.setter
    def result(self, value: dict[str, Any] | None) -> None:
        self.output = dict(value) if isinstance(value, dict) else value
