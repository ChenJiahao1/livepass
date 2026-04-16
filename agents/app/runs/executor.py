from __future__ import annotations

from datetime import datetime, timezone

from app.agent_runtime.service import AgentRuntimeService
from app.common.errors import ApiError, ApiErrorCode
from app.messages.service import MessageService
from app.runs.event_projector import RunEventProjector
from app.runs.event_models import RUN_EVENT_TYPE_RUN_CANCELLED, RUN_EVENT_TYPE_RUN_RESUMED
from app.runs.event_store import RunEventStore
from app.runs.resume_command_executor import ResumeCommandExecutor
from app.runs.models import RUN_STATUS_CANCELLED, RUN_STATUS_QUEUED, RUN_STATUS_REQUIRES_ACTION, RUN_STATUS_RUNNING
from app.runs.repository import RunRepository
from app.runs.service import RunService
from app.runs.tool_call_models import TOOL_CALL_STATUS_COMPLETED, TOOL_CALL_STATUS_WAITING_HUMAN
from app.runs.tool_call_repository import ToolCallRepository


class RunExecutor:
    def __init__(
        self,
        *,
        run_repository: RunRepository,
        run_service: RunService,
        message_service: MessageService,
        event_store: RunEventStore,
        tool_call_repository: ToolCallRepository,
        runtime_service: AgentRuntimeService,
    ) -> None:
        self.run_repository = run_repository
        self.run_service = run_service
        self.message_service = message_service
        self.event_store = event_store
        self.tool_call_repository = tool_call_repository
        self.runtime_service = runtime_service
        self.resume_executor = ResumeCommandExecutor(runtime_service=runtime_service)

    async def start(self, run_id: str) -> None:
        run = self._get_run(run_id)
        projector = RunEventProjector(
            event_store=self.event_store,
            tool_call_repository=self.tool_call_repository,
            run_service=self.run_service,
            message_service=self.message_service,
        )
        try:
            result = await self.runtime_service.invoke_run(
                run=run,
                user_text=str(run.metadata.get("userText", "")),
                callbacks=projector,
            )
            if result.get("tool_call") or result.get("__interrupt__"):
                return
            await projector.finalize_run(
                run=run,
                output_message_ids=[str(run.metadata.get("assistantMessageId", ""))],
            )
        except Exception as exc:
            await projector.fail_run(run=run, message=str(exc) or "运行失败")

    async def resume(self, run_id: str, tool_call_id: str, action_payload: dict) -> None:
        run = self._get_run(run_id)
        tool_call = self._get_tool_call(run=run, tool_call_id=tool_call_id)
        if run.status != RUN_STATUS_REQUIRES_ACTION:
            if self._is_idempotent_resume(run=run, tool_call=tool_call, action_payload=action_payload):
                return
            raise ApiError(
                code=ApiErrorCode.RUN_STATE_INVALID,
                message="当前运行状态不可恢复",
                http_status=409,
                details={"runId": run.id, "status": run.status},
            )
        if tool_call.status != TOOL_CALL_STATUS_WAITING_HUMAN:
            if self._is_idempotent_resume(run=run, tool_call=tool_call, action_payload=action_payload):
                return
            raise ApiError(
                code=ApiErrorCode.RUN_STATE_INVALID,
                message="当前工具调用状态不可恢复",
                http_status=409,
                details={"runId": run.id, "toolCallId": tool_call_id, "status": tool_call.status},
            )

        self.run_service.resume_run(user_id=run.user_id, run_id=run_id)
        self.event_store.append(
            run_id=run.id,
            thread_id=run.thread_id,
            user_id=run.user_id,
            event_type=RUN_EVENT_TYPE_RUN_RESUMED,
            payload={"toolCallId": tool_call_id, "action": action_payload.get("action")},
            now=datetime.now(timezone.utc),
        )

        projector = RunEventProjector(
            event_store=self.event_store,
            tool_call_repository=self.tool_call_repository,
            run_service=self.run_service,
            message_service=self.message_service,
        )
        try:
            result = await self.resume_executor.resume(
                run=run,
                tool_call=tool_call,
                action_payload=action_payload,
                callbacks=projector,
            )
            await projector.on_tool_call_completed(
                run=run,
                tool_call_id=tool_call_id,
                output={"action": action_payload.get("action")},
            )
            if result.get("tool_call") or result.get("__interrupt__"):
                return
            await projector.finalize_run(
                run=run,
                output_message_ids=[str(run.metadata.get("assistantMessageId", ""))],
            )
        except Exception as exc:
            await projector.on_tool_call_failed(
                run=run,
                tool_call_id=tool_call_id,
                error={"message": str(exc) or "运行失败"},
            )
            await projector.fail_run(run=run, message=str(exc) or "运行失败")

    async def cancel(self, run_id: str) -> None:
        run = self._get_run(run_id)
        if run.status == RUN_STATUS_CANCELLED:
            return
        if run.status not in {RUN_STATUS_QUEUED, RUN_STATUS_RUNNING, RUN_STATUS_REQUIRES_ACTION}:
            raise ApiError(
                code=ApiErrorCode.RUN_STATE_INVALID,
                message="当前运行状态不可取消",
                http_status=409,
                details={"runId": run.id, "status": run.status},
            )
        self.run_service.mark_cancelled(run_id=run_id)
        waiting_tool_call = self.tool_call_repository.find_waiting_by_run(run_id=run.id)
        if waiting_tool_call is not None:
            self.tool_call_repository.mark_cancelled(
                tool_call_id=waiting_tool_call.id,
                now=datetime.now(timezone.utc),
            )
        self.event_store.append(
            run_id=run.id,
            thread_id=run.thread_id,
            user_id=run.user_id,
            event_type=RUN_EVENT_TYPE_RUN_CANCELLED,
            payload={"status": "cancelled"},
            now=datetime.now(timezone.utc),
        )

    def _get_run(self, run_id: str):
        run = self.run_repository.find_by_id(run_id=run_id)
        if run is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"runId": run_id},
            )
        return run

    def _get_tool_call(self, *, run, tool_call_id: str):
        tool_call = self.tool_call_repository.find_by_id(tool_call_id=tool_call_id)
        if tool_call is None or tool_call.run_id != run.id or tool_call.thread_id != run.thread_id or tool_call.user_id != run.user_id:
            raise ApiError(
                code=ApiErrorCode.TOOL_CALL_NOT_FOUND,
                message="工具调用不存在",
                http_status=404,
                details={"runId": run.id, "toolCallId": tool_call_id},
            )
        return tool_call

    def _is_idempotent_resume(self, *, run, tool_call, action_payload: dict) -> bool:
        if tool_call.status != TOOL_CALL_STATUS_COMPLETED:
            return False
        completed_action = ""
        if isinstance(tool_call.output, dict):
            completed_action = str(tool_call.output.get("action") or "")
        requested_action = str(action_payload.get("action") or "")
        if not requested_action or completed_action != requested_action:
            return False
        return run.status != RUN_STATUS_REQUIRES_ACTION
