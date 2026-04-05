"""TaskCard v1 protocol for orchestrator task decomposition."""

from __future__ import annotations

from typing import Literal, TypeAlias

from pydantic import BaseModel, Field

SlotValue: TypeAlias = str | int | float | bool | None
RiskLevel = Literal["low", "medium", "high"]
FallbackPolicy = Literal["clarify", "return_parent", "handoff"]


class TaskCard(BaseModel):
    """Stable v1 task contract shared across orchestrator components."""

    task_id: str
    session_id: str
    domain: str
    task_type: str
    skill_id: str
    goal: str
    source_message: str | None = None
    input_slots: dict[str, SlotValue] = Field(default_factory=dict)
    required_slots: list[str] = Field(default_factory=list)
    max_steps: int = Field(default=3, ge=1)
    risk_level: RiskLevel = "low"
    fallback_policy: FallbackPolicy
    expected_output_schema: str
