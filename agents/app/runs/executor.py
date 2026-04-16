from __future__ import annotations

from datetime import datetime, timezone

from app.agent_runtime.service import AgentRuntimeService
from app.common.errors import ApiError, ApiErrorCode
from app.messages.service import MessageService
from app.runs.event_projector import RunEventProjector
from app.runs.event_models import RUN_EVENT_TYPE_RUN_CANCELLED, RUN_EVENT_TYPE_RUN_RESUMED
from app.runs.event_store import RunEventStore
from app.runs.models import RUN_STATUS_QUEUED, RUN_STATUS_REQUIRES_ACTION, RUN_STATUS_RUNNING
from app.runs.repository import RunRepository
from app.runs.service import RunService
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
            if result.get("tool_call"):
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
            raise ApiError(
                code=ApiErrorCode.RUN_STATE_INVALID,
                message="当前运行状态不可恢复",
                http_status=409,
                details={"runId": run.id, "status": run.status},
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
            action = action_payload.get("action")
            if action == "approve" and tool_call.arguments.get("action") == "refund_order":
                values = dict(tool_call.arguments.get("values", {}))
                values.update(action_payload.get("values", {}))
                result = await self.runtime_service.registry.invoke(
                    server_name="refund",
                    tool_name="refund_order",
                    payload=values,
                )
                refund_amount = result.get("refund_amount") or result.get("refundAmount", "待确认")
                reply = f"订单 {values.get('order_id')} 已提交退款，退款金额 {refund_amount}。"
            else:
                reply = "已取消本次人工确认操作。"

            self.tool_call_repository.update_status(
                tool_call_id=tool_call_id,
                status="completed",
                output={"action": action},
                error=None,
                now=datetime.now(timezone.utc),
            )
            assistant_message_id = str(run.metadata.get("assistantMessageId", ""))
            self.event_store.append(
                run_id=run.id,
                thread_id=run.thread_id,
                user_id=run.user_id,
                event_type="message_delta",
                payload={"messageId": assistant_message_id, "delta": reply},
                now=datetime.now(timezone.utc),
            )
            self.message_service.update_message_status(
                user_id=run.user_id,
                thread_id=run.thread_id,
                message_id=assistant_message_id,
                status="completed",
                parts=[{"type": "text", "text": reply}],
                metadata={},
            )
            self.run_service.mark_completed(run_id=run.id, output_message_ids=[assistant_message_id])
            self.event_store.append(
                run_id=run.id,
                thread_id=run.thread_id,
                user_id=run.user_id,
                event_type="run_completed",
                payload={"status": "completed", "outputMessageIds": [assistant_message_id]},
                now=datetime.now(timezone.utc),
            )
        except Exception as exc:
            self.tool_call_repository.update_status(
                tool_call_id=tool_call_id,
                status="failed",
                output=None,
                error={"message": str(exc) or "运行失败"},
                now=datetime.now(timezone.utc),
            )
            await projector.fail_run(run=run, message=str(exc) or "运行失败")

    async def cancel(self, run_id: str) -> None:
        run = self._get_run(run_id)
        if run.status not in {RUN_STATUS_QUEUED, RUN_STATUS_RUNNING, RUN_STATUS_REQUIRES_ACTION}:
            raise ApiError(
                code=ApiErrorCode.RUN_STATE_INVALID,
                message="当前运行状态不可取消",
                http_status=409,
                details={"runId": run.id, "status": run.status},
            )
        self.run_service.mark_cancelled(run_id=run_id)
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
