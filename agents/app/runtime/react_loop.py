"""Minimal bounded tool loop for orchestrator skills."""

from __future__ import annotations

from typing import Any

from app.orchestrator.skill_resolver import SkillResolution
from app.runtime.execution_context import ExecutionContext
from app.tasking.task_card import TaskCard


class ReactLoop:
    def __init__(self, *, broker) -> None:
        self.broker = broker

    async def run(
        self,
        *,
        task: TaskCard,
        resolution: SkillResolution,
    ) -> ExecutionContext:
        ctx = ExecutionContext(
            task_id=task.task_id,
            session_id=task.session_id,
            current_skill=resolution.skill.skill_id,
        )
        output: dict[str, Any] = {}
        for step, tool_name in enumerate(resolution.skill.tools[: task.max_steps], start=1):
            ctx.step = step
            ctx.tool_calls.append(tool_name)
            output = await self.broker.call(
                task=task,
                skill_id=resolution.skill.skill_id,
                tool_name=tool_name,
                payload=dict(task.input_slots),
            )
            if isinstance(output, dict):
                ctx.last_observation = output
                ctx.structured_output = output
            break
        return ctx

