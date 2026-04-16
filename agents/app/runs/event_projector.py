from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

from app.common.errors import ApiErrorCode
from app.common.ids import new_tool_call_id
from app.messages.models import MESSAGE_STATUS_COMPLETED
from app.messages.service import MessageService
from app.runs.event_models import (
    RUN_EVENT_TYPE_MESSAGE_DELTA,
    RUN_EVENT_TYPE_RUN_COMPLETED,
    RUN_EVENT_TYPE_RUN_FAILED,
    RUN_EVENT_TYPE_RUN_STARTED,
    RUN_EVENT_TYPE_TOOL_CALL_REQUIRES_HUMAN,
    RUN_EVENT_TYPE_TOOL_CALL_STARTED,
)
from app.runs.event_store import RunEventStore
from app.runs.models import RunRecord
from app.runs.service import RunService
from app.runs.tool_call_models import TOOL_CALL_STATUS_WAITING_HUMAN, ToolCallRecord
from app.runs.tool_call_repository import ToolCallRepository


class RunEventProjector:
    def __init__(
        self,
        *,
        event_store: RunEventStore,
        tool_call_repository: ToolCallRepository,
        run_service: RunService,
        message_service: MessageService,
    ) -> None:
        self.event_store = event_store
        self.tool_call_repository = tool_call_repository
        self.run_service = run_service
        self.message_service = message_service
        self._message_buffers: dict[str, str] = {}

    async def on_run_started(self, *, run: RunRecord) -> None:
        self.run_service.mark_running(run_id=run.id)
        self.event_store.append(
            run_id=run.id,
            thread_id=run.thread_id,
            user_id=run.user_id,
            event_type=RUN_EVENT_TYPE_RUN_STARTED,
            payload={"status": "running"},
            now=run.started_at,
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
        self.event_store.append(
            run_id=run.id,
            thread_id=run.thread_id,
            user_id=run.user_id,
            event_type=RUN_EVENT_TYPE_MESSAGE_DELTA,
            payload={"messageId": message_id, "delta": delta, "metadata": metadata or {}},
            now=run.started_at,
        )

    async def on_tool_call_started(
        self,
        *,
        run: RunRecord,
        tool_name: str,
        arguments: dict[str, Any],
        metadata: dict[str, Any] | None = None,
    ) -> None:
        tool_call_id = new_tool_call_id()
        record = ToolCallRecord(
            id=tool_call_id,
            run_id=run.id,
            thread_id=run.thread_id,
            user_id=run.user_id,
            tool_name=tool_name,
            status=TOOL_CALL_STATUS_WAITING_HUMAN,
            arguments=dict(arguments),
            output=None,
            error=None,
            created_at=run.started_at,
            updated_at=run.started_at,
            completed_at=None,
            metadata=dict(metadata or {}),
        )
        self.tool_call_repository.create(record)
        self.event_store.append(
            run_id=run.id,
            thread_id=run.thread_id,
            user_id=run.user_id,
            event_type=RUN_EVENT_TYPE_TOOL_CALL_STARTED,
            payload={"toolCallId": tool_call_id, "toolName": tool_name, "arguments": arguments},
            now=run.started_at,
        )

    async def on_tool_call_requires_human(
        self,
        *,
        run: RunRecord,
        tool_name: str,
        arguments: dict[str, Any],
        metadata: dict[str, Any] | None = None,
    ) -> None:
        self.run_service.mark_requires_action(run_id=run.id)
        self.event_store.append(
            run_id=run.id,
            thread_id=run.thread_id,
            user_id=run.user_id,
            event_type=RUN_EVENT_TYPE_TOOL_CALL_REQUIRES_HUMAN,
            payload={"toolName": tool_name, "arguments": arguments, "metadata": metadata or {}},
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
        return None

    async def on_tool_call_failed(
        self,
        *,
        run: RunRecord,
        tool_call_id: str,
        error: dict[str, Any],
        metadata: dict[str, Any] | None = None,
    ) -> None:
        return None

    async def finalize_run(self, *, run: RunRecord, output_message_ids: list[str]) -> None:
        assistant_message_id = str(run.metadata.get("assistantMessageId", ""))
        reply = self._message_buffers.get(run.id, "")
        if assistant_message_id:
            self.message_service.update_message_status(
                user_id=run.user_id,
                thread_id=run.thread_id,
                message_id=assistant_message_id,
                status=MESSAGE_STATUS_COMPLETED,
                parts=[{"type": "text", "text": reply}],
                metadata={},
            )
        self.run_service.mark_completed(run_id=run.id, output_message_ids=output_message_ids)
        self.event_store.append(
            run_id=run.id,
            thread_id=run.thread_id,
            user_id=run.user_id,
            event_type=RUN_EVENT_TYPE_RUN_COMPLETED,
            payload={"status": "completed", "outputMessageIds": output_message_ids},
            now=run.started_at,
        )

    async def fail_run(self, *, run: RunRecord, message: str) -> None:
        assistant_message_id = str(run.metadata.get("assistantMessageId", ""))
        now = datetime.now(timezone.utc)
        if assistant_message_id:
            self.message_service.update_message_status(
                user_id=run.user_id,
                thread_id=run.thread_id,
                message_id=assistant_message_id,
                status=MESSAGE_STATUS_COMPLETED,
                parts=[{"type": "text", "text": message}],
                metadata={},
            )
        self.run_service.mark_failed(run_id=run.id, message=message)
        self.event_store.append(
            run_id=run.id,
            thread_id=run.thread_id,
            user_id=run.user_id,
            event_type=RUN_EVENT_TYPE_RUN_FAILED,
            payload={
                "status": "failed",
                "error": {"code": ApiErrorCode.AGENT_RUN_FAILED, "message": message},
            },
            now=now,
        )
