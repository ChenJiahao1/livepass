"""Per-task execution context."""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field


class ExecutionContext(BaseModel):
    task_id: str
    session_id: str
    current_skill: str
    step: int = 0
    tool_calls: list[str] = Field(default_factory=list)
    last_observation: dict[str, Any] | None = None
    structured_output: dict[str, Any] | None = None

