from __future__ import annotations

from typing import Any, Mapping

from app.agent_runtime.interrupt_models import HumanResumePayload
from app.common.errors import ApiError, ApiErrorCode
from app.agent_runtime.interrupt_models import HumanInterruptPayload
from app.runs.tool_call_repository import ToolCallRepository
from app.runs.tool_call_models import TOOL_CALL_STATUS_WAITING_HUMAN, ToolCallRecord


class InterruptBridge:
    def parse_interrupt(self, payload: Mapping[str, Any]) -> HumanInterruptPayload:
        return HumanInterruptPayload.from_raw(payload)

    def project_interrupt(self, *, tool_call_id: str, interrupt: HumanInterruptPayload) -> dict[str, Any]:
        return {
            "toolCallId": tool_call_id,
            "toolName": interrupt.tool_name,
            "args": dict(interrupt.args),
            "request": dict(interrupt.request),
        }

    def parse_resume_payload(self, action_payload: Mapping[str, Any]) -> HumanResumePayload:
        values = action_payload.get("values")
        return HumanResumePayload(
            action=str(action_payload.get("action") or "reject"),  # type: ignore[arg-type]
            reason=action_payload.get("reason"),
            values=dict(values) if isinstance(values, Mapping) else {},
        )

    def build_command_resume_payload(
        self,
        *,
        tool_call: ToolCallRecord,
        action_payload: Mapping[str, Any],
    ) -> dict[str, Any]:
        resume_payload = self.parse_resume_payload(action_payload)
        base_values = tool_call.arguments.get("values")
        merged_values = dict(base_values) if isinstance(base_values, Mapping) else {}
        merged_values.update(resume_payload.values)
        return {
            "action": resume_payload.action,
            "reason": resume_payload.reason,
            "values": merged_values,
        }

    def assert_no_waiting_human_tool_call(
        self,
        *,
        tool_call_repository: ToolCallRepository,
        run_id: str,
    ) -> None:
        waiting = tool_call_repository.find_waiting_by_run(run_id=run_id)
        if waiting is None:
            return
        raise ApiError(
            code=ApiErrorCode.RUN_STATE_INVALID,
            message="当前运行已存在待处理人工操作",
            http_status=409,
            details={"runId": run_id, "toolCallId": waiting.id, "status": TOOL_CALL_STATUS_WAITING_HUMAN},
        )
