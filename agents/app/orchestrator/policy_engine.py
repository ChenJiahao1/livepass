"""Policy controls for parent-generated tasks."""

from __future__ import annotations

from app.tasking.task_card import TaskCard


class PolicyEngine:
    def __init__(self, *, max_steps_limit: int = 3) -> None:
        self.max_steps_limit = max_steps_limit

    def apply(self, task: TaskCard) -> TaskCard:
        payload = task.model_dump()
        payload["max_steps"] = min(task.max_steps, self.max_steps_limit)
        return TaskCard.model_validate(payload)

