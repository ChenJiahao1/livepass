from __future__ import annotations

from dataclasses import replace
from datetime import datetime, timezone
from typing import Any, Mapping

from app.runs.interrupt_models import HumanInterruptPayload
from app.common.errors import ApiErrorCode
from app.common.ids import new_tool_call_id
from app.conversations.messages.models import MESSAGE_STATUS_CANCELLED, MESSAGE_STATUS_COMPLETED, MESSAGE_STATUS_ERROR
from app.conversations.messages.service import MessageService
from app.runs.execution.event_bus import RunEventBus
from app.runs.event_models import (
    RUN_EVENT_TYPE_MESSAGE_CANCELLED,
    RUN_EVENT_TYPE_MESSAGE_COMPLETED,
    RUN_EVENT_TYPE_MESSAGE_CREATED,
    RUN_EVENT_TYPE_MESSAGE_DELTA,
    RUN_EVENT_TYPE_MESSAGE_FAILED,
    RUN_EVENT_TYPE_MESSAGE_UPDATED,
    RUN_EVENT_TYPE_RUN_CANCELLED,
    RUN_EVENT_TYPE_RUN_COMPLETED,
    RUN_EVENT_TYPE_RUN_CREATED,
    RUN_EVENT_TYPE_RUN_FAILED,
    RUN_EVENT_TYPE_RUN_PROGRESS,
    RUN_EVENT_TYPE_RUN_UPDATED,
    RUN_EVENT_TYPE_TOOL_CALL_COMPLETED,
    RUN_EVENT_TYPE_TOOL_CALL_CREATED,
    RUN_EVENT_TYPE_TOOL_CALL_FAILED,
    RUN_EVENT_TYPE_TOOL_CALL_PROGRESS,
    RUN_EVENT_TYPE_TOOL_CALL_UPDATED,
    RUN_EVENT_TYPE_TOOL_CALL_WAITING_HUMAN,
)
from app.runs.event_store import RunEventStore
from app.runs.execution.interrupt_bridge import InterruptBridge
from app.runs.models import RunRecord
from app.runs.execution.runtime import RunService
from app.runs.tool_call_contract import serialize_tool_call_event
from app.runs.tool_call_models import TOOL_CALL_STATUS_WAITING_HUMAN, ToolCallRecord
from app.runs.tool_call_repository import ToolCallRepository


class RunEventProjector:
    def __init__(
        self,
        *,
        event_store: RunEventStore,
        event_bus: RunEventBus,
        tool_call_repository: ToolCallRepository,
        run_service: RunService,
        message_service: MessageService,
        interrupt_bridge: InterruptBridge | None = None,
    ) -> None:
        self.event_store = event_store
        self.event_bus = event_bus
        self.tool_call_repository = tool_call_repository
        self.run_service = run_service
        self.message_service = message_service
        self.interrupt_bridge = interrupt_bridge or InterruptBridge()
        self._message_buffers: dict[str, str] = {}
        self._active_tool_call_ids: dict[str, str] = {}
        self._progress_tool_call_ids: dict[tuple[str, str], str] = {}

    async def on_run_created(self, *, run: RunRecord) -> None:
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_RUN_CREATED,
            payload={"run": {"id": run.id, "status": run.status}},
            now=run.started_at,
        )
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_MESSAGE_CREATED,
            message_id=run.output_message_id,
            payload={"message": {"id": run.output_message_id, "role": "assistant", "status": "streaming", "content": []}},
            now=run.started_at,
        )

    async def on_run_started(self, *, run: RunRecord) -> None:
        self.run_service.mark_running(run_id=run.id)
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_RUN_UPDATED,
            payload={"run": {"id": run.id, "status": "running"}},
            now=run.started_at,
        )

    async def on_run_updated(
        self,
        *,
        run: RunRecord,
        status: str,
        payload: dict[str, Any] | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> None:
        event_payload = {"run": {"id": run.id, "status": status}}
        if payload:
            event_payload["run"].update(dict(payload))
        if metadata:
            event_payload["metadata"] = dict(metadata)
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_RUN_UPDATED,
            payload=event_payload,
            now=datetime.now(timezone.utc),
        )

    async def on_message_delta(
        self,
        *,
        run: RunRecord,
        message_id: str,
        delta: str,
        metadata: dict[str, Any] | None = None,
    ) -> None:
        self._message_buffers[run.id] = f"{self._message_buffers.get(run.id, '')}{delta}"
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_MESSAGE_DELTA,
            message_id=message_id,
            payload={"delta": {"type": "text", "text": delta}, **({"metadata": metadata} if metadata else {})},
            now=run.started_at,
        )

    async def on_message_updated(
        self,
        *,
        run: RunRecord,
        message_id: str,
        status: str,
        payload: dict[str, Any] | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> None:
        event_payload = {"message": {"id": message_id, "status": status}}
        if payload:
            event_payload["message"].update(dict(payload))
        if metadata:
            event_payload["metadata"] = dict(metadata)
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_MESSAGE_UPDATED,
            message_id=message_id,
            payload=event_payload,
            now=datetime.now(timezone.utc),
        )

    async def on_tool_call_started(
        self,
        *,
        run: RunRecord,
        tool_name: str,
        args: dict[str, Any],
        request: dict[str, Any],
        metadata: dict[str, Any] | None = None,
    ) -> None:
        self.interrupt_bridge.assert_no_waiting_human_tool_call(
            tool_call_repository=self.tool_call_repository,
            run_id=run.id,
        )
        tool_call_id = new_tool_call_id()
        message_id = str(run.metadata.get("outputMessageId", "")) or None
        projected_payload = self.interrupt_bridge.project_interrupt(
            tool_call_id=tool_call_id,
            interrupt=HumanInterruptPayload(
                tool_name=tool_name,  # type: ignore[arg-type]
                action=str(args.get("action") or ""),
                args=dict(args),
                request=dict(request),
            ),
        )
        record = ToolCallRecord(
            id=tool_call_id,
            run_id=run.id,
            thread_id=run.thread_id,
            user_id=run.user_id,
            message_id=message_id or run.output_message_id,
            name=tool_name,
            status=TOOL_CALL_STATUS_WAITING_HUMAN,
            input=dict(args),
            human_request=dict(projected_payload["humanRequest"]),
            output=None,
            error=None,
            created_at=run.started_at,
            updated_at=run.started_at,
            completed_at=None,
            metadata=dict(metadata or {}),
        )
        self.tool_call_repository.create(record)
        self._active_tool_call_ids[run.id] = tool_call_id
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_TOOL_CALL_CREATED,
            payload={
                "toolCall": serialize_tool_call_event(
                    replace(record, status="running"),
                )
            },
            message_id=record.message_id,
            tool_call_id=tool_call_id,
            now=run.started_at,
        )

    async def on_tool_call_requires_human(
        self,
        *,
        run: RunRecord,
        tool_name: str,
        args: dict[str, Any],
        request: dict[str, Any],
        metadata: dict[str, Any] | None = None,
    ) -> None:
        self.run_service.mark_requires_action(run_id=run.id)
        tool_call_id = self._active_tool_call_ids.get(run.id, "")
        projected_payload = self.interrupt_bridge.project_interrupt(
            tool_call_id=tool_call_id,
            interrupt=HumanInterruptPayload(
                tool_name=tool_name,  # type: ignore[arg-type]
                action=str(args.get("action") or ""),
                args=dict(args),
                request=dict(request),
            ),
        )
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_RUN_UPDATED,
            payload={"run": {"id": run.id, "status": "requires_action"}},
            tool_call_id=tool_call_id,
            now=run.started_at,
        )
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_TOOL_CALL_WAITING_HUMAN,
            payload={
                "toolCall": serialize_tool_call_event(
                    ToolCallRecord(
                        id=tool_call_id,
                        run_id=run.id,
                        thread_id=run.thread_id,
                        user_id=run.user_id,
                        message_id=self._message_id_for_tool_call(tool_call_id) or run.output_message_id,
                        name=tool_name,
                        status=TOOL_CALL_STATUS_WAITING_HUMAN,
                        input=dict(args),
                        human_request=dict(projected_payload["humanRequest"]),
                        metadata=dict(metadata or {}),
                    )
                ),
                **({"metadata": metadata} if metadata else {}),
            },
            message_id=self._message_id_for_tool_call(tool_call_id) or run.output_message_id,
            tool_call_id=tool_call_id,
            now=run.started_at,
        )

    async def on_tool_call_completed(
        self,
        *,
        run: RunRecord,
        tool_call_id: str,
        output: dict[str, Any],
        metadata: dict[str, Any] | None = None,
    ) -> None:
        now = datetime.now(timezone.utc)
        updated_tool_call = self.tool_call_repository.update_status(
            tool_call_id=tool_call_id,
            status="completed",
            error=None,
            now=now,
            output=output,
        )
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_TOOL_CALL_COMPLETED,
            payload={
                "toolCall": serialize_tool_call_event(
                    updated_tool_call
                    or self._fallback_tool_call(run=run, tool_call_id=tool_call_id, status="completed", output=output)
                ),
                **({"metadata": metadata} if metadata else {}),
            },
            message_id=updated_tool_call.message_id if updated_tool_call else run.output_message_id,
            tool_call_id=tool_call_id,
            now=now,
        )

    async def on_tool_call_progress(
        self,
        *,
        run: RunRecord,
        tool_name: str,
        payload: dict[str, Any],
        metadata: dict[str, Any] | None = None,
    ) -> None:
        progress_key = (run.id, tool_name)
        tool_call_id = self._progress_tool_call_ids.get(progress_key)
        if tool_call_id is None:
            tool_call_id = new_tool_call_id()
            self._progress_tool_call_ids[progress_key] = tool_call_id
        event_payload = {
            "toolCall": serialize_tool_call_event(
                ToolCallRecord(
                    id=tool_call_id,
                    run_id=run.id,
                    thread_id=run.thread_id,
                    user_id=run.user_id,
                    message_id=run.output_message_id,
                    name=tool_name,
                    status=str(payload.get("status") or "running"),
                    input=self._mapping_field(payload.get("input")),
                    output=self._optional_mapping_field(payload.get("output")),
                    error=self._optional_mapping_field(payload.get("error")),
                    metadata=dict(metadata or {}),
                )
            )
        }
        if metadata:
            event_payload["metadata"] = dict(metadata)
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_TOOL_CALL_PROGRESS,
            message_id=run.output_message_id,
            tool_call_id=tool_call_id,
            payload=event_payload,
            now=datetime.now(timezone.utc),
        )

    async def on_tool_call_failed(
        self,
        *,
        run: RunRecord,
        tool_call_id: str,
        error: dict[str, Any],
        metadata: dict[str, Any] | None = None,
    ) -> None:
        now = datetime.now(timezone.utc)
        updated_tool_call = self.tool_call_repository.update_status(
            tool_call_id=tool_call_id,
            status="failed",
            error=error,
            now=now,
            output=None,
        )
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_TOOL_CALL_FAILED,
            payload={
                "toolCall": serialize_tool_call_event(
                    updated_tool_call or self._fallback_tool_call(run=run, tool_call_id=tool_call_id, status="failed", error=error)
                ),
                **({"metadata": metadata} if metadata else {}),
            },
            message_id=updated_tool_call.message_id if updated_tool_call else run.output_message_id,
            tool_call_id=tool_call_id,
            now=now,
        )

    async def on_run_progress(
        self,
        *,
        run: RunRecord,
        payload: dict[str, Any],
        metadata: dict[str, Any] | None = None,
    ) -> None:
        event_payload = {"progress": dict(payload)}
        if metadata:
            event_payload["metadata"] = dict(metadata)
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_RUN_PROGRESS,
            payload=event_payload,
            now=datetime.now(timezone.utc),
        )

    async def finalize_run(self, *, run: RunRecord, output_message_ids: list[str]) -> None:
        del output_message_ids
        output_message_id = run.output_message_id
        reply = self._message_buffers.get(run.id, "")
        if output_message_id:
            self.message_service.update_message_status(
                user_id=run.user_id,
                thread_id=run.thread_id,
                message_id=output_message_id,
                status=MESSAGE_STATUS_COMPLETED,
                content=[{"type": "text", "text": reply}],
                metadata={},
            )
            self._append_event(
                run=run,
                event_type=RUN_EVENT_TYPE_MESSAGE_COMPLETED,
                message_id=output_message_id,
                payload={"message": {"id": output_message_id, "status": "completed", "content": [{"type": "text", "text": reply}]}},
                now=datetime.now(timezone.utc),
            )
        self.run_service.mark_completed(run_id=run.id, output_message_ids=[])
        now = datetime.now(timezone.utc)
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_RUN_COMPLETED,
            payload={"run": {"id": run.id, "status": "completed"}},
            now=now,
        )

    async def fail_run(self, *, run: RunRecord, message: str, details: dict | None = None) -> None:
        output_message_id = run.output_message_id
        now = datetime.now(timezone.utc)
        if output_message_id:
            self.message_service.update_message_status(
                user_id=run.user_id,
                thread_id=run.thread_id,
                message_id=output_message_id,
                status=MESSAGE_STATUS_ERROR,
                content=[{"type": "text", "text": message}],
                metadata={},
            )
            self._append_event(
                run=run,
                event_type=RUN_EVENT_TYPE_MESSAGE_FAILED,
                message_id=output_message_id,
                payload={"message": {"id": output_message_id, "status": "failed", "content": [{"type": "text", "text": message}]}},
                now=now,
            )
        error = {"code": ApiErrorCode.LANGGRAPH_RUNTIME_ERROR, "message": message, "details": dict(details or {})}
        self.run_service.mark_failed(run_id=run.id, message=message, details=details)
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_RUN_FAILED,
            payload={
                "run": {
                    "id": run.id,
                    "status": "failed",
                    "error": error,
                },
            },
            now=now,
        )

    async def cancel_run(self, *, run: RunRecord) -> None:
        now = datetime.now(timezone.utc)
        waiting_tool_call = self.tool_call_repository.find_waiting_by_run(run_id=run.id)
        if waiting_tool_call is not None:
            cancelled_tool_call = self.tool_call_repository.mark_cancelled(tool_call_id=waiting_tool_call.id, now=now)
            self._append_event(
                run=run,
                event_type=RUN_EVENT_TYPE_TOOL_CALL_UPDATED,
                message_id=waiting_tool_call.message_id,
                tool_call_id=waiting_tool_call.id,
                payload={"toolCall": serialize_tool_call_event(cancelled_tool_call or replace(waiting_tool_call, status="cancelled"))},
                now=now,
            )
        self.run_service.mark_cancelled(run_id=run.id)
        self.message_service.update_message_status(
            user_id=run.user_id,
            thread_id=run.thread_id,
            message_id=run.output_message_id,
            status=MESSAGE_STATUS_CANCELLED,
            content=None,
            metadata=None,
        )
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_MESSAGE_CANCELLED,
            message_id=run.output_message_id,
            payload={"message": {"id": run.output_message_id, "status": "cancelled"}},
            now=now,
        )
        self._append_event(
            run=run,
            event_type=RUN_EVENT_TYPE_RUN_CANCELLED,
            payload={"run": {"id": run.id, "status": "cancelled"}},
            now=now,
        )

    def _append_event(
        self,
        *,
        run: RunRecord,
        event_type: str,
        payload: dict[str, Any],
        now: datetime,
        message_id: str | None = None,
        tool_call_id: str | None = None,
    ) -> None:
        record = self.event_store.append(
            run_id=run.id,
            thread_id=run.thread_id,
            user_id=run.user_id,
            event_type=event_type,
            message_id=message_id,
            tool_call_id=tool_call_id,
            payload=payload,
            now=now,
        )
        self.event_bus.publish(run_id=record.run_id, sequence_no=record.sequence_no)

    def _message_id_for_tool_call(self, tool_call_id: str) -> str | None:
        if not tool_call_id:
            return None
        tool_call = self.tool_call_repository.find_by_id(tool_call_id=tool_call_id)
        return tool_call.message_id if tool_call is not None else None

    def _fallback_tool_call(
        self,
        *,
        run: RunRecord,
        tool_call_id: str,
        status: str,
        output: dict[str, Any] | None = None,
        error: dict[str, Any] | None = None,
    ) -> ToolCallRecord:
        return ToolCallRecord(
            id=tool_call_id,
            run_id=run.id,
            thread_id=run.thread_id,
            user_id=run.user_id,
            message_id=run.output_message_id,
            name="tool",
            status=status,
            input={},
            output=output,
            error=error,
        )

    def _mapping_field(self, value: Any) -> dict[str, Any]:
        return dict(value) if isinstance(value, Mapping) else {}

    def _optional_mapping_field(self, value: Any) -> dict[str, Any] | None:
        return dict(value) if isinstance(value, Mapping) else None
