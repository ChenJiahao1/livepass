from __future__ import annotations

from typing import Any, Mapping

from app.agent_runtime.service import AgentRuntimeService
from app.runs.interrupt_bridge import InterruptBridge
from app.runs.models import RunRecord
from app.runs.tool_call_models import ToolCallRecord


class ResumeCommandExecutor:
    def __init__(
        self,
        *,
        runtime_service: AgentRuntimeService,
        interrupt_bridge: InterruptBridge | None = None,
    ) -> None:
        self.runtime_service = runtime_service
        self.interrupt_bridge = interrupt_bridge or InterruptBridge()

    async def resume(
        self,
        *,
        run: RunRecord,
        tool_call: ToolCallRecord,
        action_payload: Mapping[str, Any],
        callbacks,
    ) -> dict[str, Any]:
        resume_payload = self.interrupt_bridge.build_command_resume_payload(
            tool_call=tool_call,
            action_payload=action_payload,
        )
        return await self.runtime_service.invoke_resume(
            run=run,
            resume_payload=resume_payload,
            callbacks=callbacks,
        )
