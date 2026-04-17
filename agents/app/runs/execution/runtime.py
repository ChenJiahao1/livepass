from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Callable, Mapping

from langgraph.types import Command

from app.runs.execution.callbacks import RuntimeCallbacks
from app.runs.execution.interrupt_bridge import InterruptBridge


@dataclass(slots=True)
class AgentRuntimeResult:
    reply: str
    metadata: dict[str, Any] = field(default_factory=dict)
    raw_result: dict[str, Any] = field(default_factory=dict)


@dataclass(slots=True)
class AgentRuntimeRunResult:
    requires_action: bool = False
    cancelled: bool = False


class AgentRuntimeService:
    def __init__(
        self,
        *,
        agent_runtime,
        registry,
        llm,
        knowledge_service=None,
        interrupt_bridge: InterruptBridge | None = None,
    ) -> None:
        self.agent_runtime = agent_runtime
        self.registry = registry
        self.llm = llm
        self.knowledge_service = knowledge_service
        self.interrupt_bridge = interrupt_bridge or InterruptBridge()

    async def invoke(self, *, user_id: int, thread_id: str, user_text: str) -> AgentRuntimeResult:
        result = await self.agent_runtime.ainvoke(
            {"messages": [{"role": "user", "content": user_text}]},
            config={"configurable": {"thread_id": thread_id}},
            context={
                "registry": self.registry,
                "llm": self.llm,
                "knowledge_service": self.knowledge_service,
                "current_user_id": user_id,
            },
        )
        reply = result.get("final_reply") or result.get("reply") or ""
        metadata = {
            "routeSource": result.get("route_source", "rule"),
            "specialist": result.get("current_agent"),
            "needHandoff": bool(result.get("need_handoff")),
        }
        return AgentRuntimeResult(reply=reply, metadata=metadata, raw_result=result)

    async def invoke_run(
        self,
        *,
        run,
        user_text: str,
        callbacks: RuntimeCallbacks,
        should_stop: Callable[[], bool] | None = None,
    ) -> AgentRuntimeRunResult:
        await callbacks.on_run_started(run=run)
        result = await self.invoke_start(run=run, user_text=user_text, callbacks=callbacks, should_stop=should_stop)
        return result

    async def invoke_start(
        self,
        *,
        run,
        user_text: str,
        callbacks: RuntimeCallbacks,
        should_stop: Callable[[], bool] | None = None,
    ) -> AgentRuntimeRunResult:
        return await self._stream_runtime(
            run=run,
            payload={"messages": [{"role": "user", "content": user_text}]},
            callbacks=callbacks,
            should_stop=should_stop,
        )

    async def invoke_resume(
        self,
        *,
        run,
        resume_payload: dict[str, Any],
        callbacks: RuntimeCallbacks,
        should_stop: Callable[[], bool] | None = None,
    ) -> AgentRuntimeRunResult:
        return await self._stream_runtime(
            run=run,
            payload=Command(resume=resume_payload),
            callbacks=callbacks,
            should_stop=should_stop,
        )

    async def _invoke_runtime(
        self,
        *,
        payload: Any,
        thread_id: str,
        user_id: int,
        run_id: str | None = None,
    ) -> dict[str, Any]:
        config = {"configurable": {"thread_id": thread_id}}
        registry = self.registry
        if run_id and hasattr(registry, "bind_context"):
            registry = registry.bind_context(
                user_id=user_id,
                thread_id=thread_id,
                run_id=run_id,
            )
        context = {
            "registry": registry,
            "llm": self.llm,
            "knowledge_service": self.knowledge_service,
            "current_user_id": user_id,
        }
        if hasattr(self.agent_runtime, "invoke"):
            import asyncio

            return await asyncio.to_thread(
                self.agent_runtime.invoke,
                payload,
                config=config,
                context=context,
            )
        return await self.agent_runtime.ainvoke(payload, config=config, context=context)

    async def _stream_runtime(
        self,
        *,
        run,
        payload: Any,
        callbacks: RuntimeCallbacks,
        should_stop: Callable[[], bool] | None = None,
    ) -> AgentRuntimeRunResult:
        result = AgentRuntimeRunResult()
        if hasattr(self.agent_runtime, "astream"):
            config = {"configurable": {"thread_id": run.thread_id}}
            registry = self.registry
            if hasattr(registry, "bind_context"):
                registry = registry.bind_context(
                    user_id=run.user_id,
                    thread_id=run.thread_id,
                    run_id=run.id,
                )
            context = {
                "registry": registry,
                "llm": self.llm,
                "knowledge_service": self.knowledge_service,
                "current_user_id": run.user_id,
            }
            async for mode, chunk in self.agent_runtime.astream(
                payload,
                config=config,
                context=context,
                stream_mode=["messages", "updates", "custom"],
            ):
                if should_stop is not None and should_stop():
                    result.cancelled = True
                    break
                if await self._project_chunk(run=run, callbacks=callbacks, mode=mode, chunk=chunk):
                    result.requires_action = True
            return result

        legacy_result = await self._invoke_runtime(payload=payload, thread_id=run.thread_id, user_id=run.user_id)
        await self._project_result(run=run, callbacks=callbacks, result=legacy_result)
        result.requires_action = bool(legacy_result.get("tool_call") or legacy_result.get("__interrupt__"))
        return result

    async def _project_chunk(self, *, run, callbacks: RuntimeCallbacks, mode: str, chunk: Any) -> bool:
        if mode == "messages":
            delta = self._extract_message_delta(chunk)
            if delta:
                await callbacks.on_message_delta(
                    run=run,
                    message_id=run.output_message_id,
                    delta=delta,
                    metadata=None,
                )
            return False

        if mode == "updates":
            interrupt_payload = self._extract_interrupt_payload(chunk)
            if interrupt_payload is not None:
                interrupt = self.interrupt_bridge.parse_interrupt(interrupt_payload)
                await callbacks.on_tool_call_started(
                    run=run,
                    tool_name=interrupt.tool_name,
                    args=dict(interrupt.args),
                    request=dict(interrupt.request),
                    metadata=None,
                )
                await callbacks.on_tool_call_requires_human(
                    run=run,
                    tool_name=interrupt.tool_name,
                    args=dict(interrupt.args),
                    request=dict(interrupt.request),
                    metadata=None,
                )
                return True

            run_payload = self._extract_named_payload(chunk, "run")
            if run_payload and str(run_payload.get("status") or "") not in {"", "running"}:
                await callbacks.on_run_updated(
                    run=run,
                    status=str(run_payload.get("status")),
                    payload=run_payload,
                    metadata=None,
                )
            message_payload = self._extract_named_payload(chunk, "message")
            if message_payload and str(message_payload.get("status") or ""):
                await callbacks.on_message_updated(
                    run=run,
                    message_id=run.output_message_id,
                    status=str(message_payload.get("status")),
                    payload=message_payload,
                    metadata=None,
                )
            return False

        if mode == "custom":
            tool_payload = self._extract_named_payload(chunk, "tool")
            if tool_payload:
                tool_name = str(tool_payload.get("name") or tool_payload.get("toolName") or "tool")
                await callbacks.on_tool_call_progress(
                    run=run,
                    tool_name=tool_name,
                    payload=tool_payload,
                    metadata=None,
                )
                return False
            progress_payload = self._extract_named_payload(chunk, "progress")
            if progress_payload:
                await callbacks.on_run_progress(
                    run=run,
                    payload=progress_payload,
                    metadata=None,
                )
            return False

        return False

    async def _project_result(self, *, run, callbacks: RuntimeCallbacks, result: dict[str, Any]) -> None:
        reply = str(result.get("final_reply") or result.get("reply") or "")
        if reply:
            await callbacks.on_message_delta(
                run=run,
                message_id=run.output_message_id,
                delta=reply,
                metadata={
                    "routeSource": result.get("route_source", "rule"),
                    "specialist": result.get("current_agent"),
                    "needHandoff": bool(result.get("need_handoff")),
                },
            )
        tool_call = result.get("tool_call")
        interrupt_payload = tool_call
        if interrupt_payload is None:
            interrupts = result.get("__interrupt__") or []
            if interrupts:
                interrupt_payload = getattr(interrupts[0], "value", None)
        if interrupt_payload:
            interrupt = self.interrupt_bridge.parse_interrupt(interrupt_payload)
            await callbacks.on_tool_call_started(
                run=run,
                tool_name=interrupt.tool_name,
                args=dict(interrupt.args),
                request=dict(interrupt.request),
                metadata={"specialist": result.get("current_agent")},
            )
            await callbacks.on_tool_call_requires_human(
                run=run,
                tool_name=interrupt.tool_name,
                args=dict(interrupt.args),
                request=dict(interrupt.request),
                metadata={"specialist": result.get("current_agent")},
            )

    def _extract_message_delta(self, chunk: Any) -> str:
        if isinstance(chunk, Mapping):
            delta = chunk.get("delta")
            if isinstance(delta, str):
                return delta
        if isinstance(chunk, tuple) and chunk:
            message = chunk[0]
            content = getattr(message, "content", None)
            if isinstance(content, str):
                return content
            if isinstance(content, list):
                texts: list[str] = []
                for item in content:
                    if isinstance(item, str):
                        texts.append(item)
                    elif isinstance(item, Mapping) and item.get("type") == "text":
                        texts.append(str(item.get("text") or ""))
                return "".join(texts)
        return ""

    def _extract_interrupt_payload(self, chunk: Any) -> dict[str, Any] | None:
        if not isinstance(chunk, Mapping):
            return None
        interrupts = chunk.get("__interrupt__")
        if not isinstance(interrupts, (list, tuple)) or not interrupts:
            return None
        payload = getattr(interrupts[0], "value", None)
        if isinstance(payload, Mapping):
            return dict(payload)
        return None

    def _extract_named_payload(self, chunk: Any, key: str) -> dict[str, Any] | None:
        if not isinstance(chunk, Mapping):
            return None
        payload = chunk.get(key)
        if not isinstance(payload, Mapping):
            return None
        return dict(payload)

from datetime import datetime, timedelta, timezone

from app.shared.errors import ApiError, ApiErrorCode
from app.shared.ids import new_message_id, new_run_id
from app.conversations.messages.models import (
    MESSAGE_ROLE_ASSISTANT,
    MESSAGE_ROLE_USER,
    MESSAGE_STATUS_COMPLETED,
    MESSAGE_STATUS_IN_PROGRESS,
    MessageRecord,
)
from app.conversations.messages.service import MessageService
from app.runs.models import (
    RUN_STATUS_CANCELLED,
    RUN_STATUS_COMPLETED,
    RUN_STATUS_FAILED,
    RUN_STATUS_QUEUED,
    RUN_STATUS_REQUIRES_ACTION,
    RUN_STATUS_RUNNING,
    RunRecord,
)
from app.runs.repository import RunRepository


class RunService:
    def __init__(
        self,
        *,
        run_repository: RunRepository,
        message_service: MessageService,
    ) -> None:
        self.run_repository = run_repository
        self.message_service = message_service

    def create_run(
        self,
        *,
        user_id: int,
        thread_id: str,
        content: list[dict],
    ) -> tuple[RunRecord, MessageRecord, MessageRecord]:
        run_id = new_run_id()
        user_text = self.message_service.extract_text(content)
        now = datetime.now(timezone.utc)
        assistant_now = now + timedelta(microseconds=1)
        user_message = MessageRecord(
            id=new_message_id(),
            thread_id=thread_id,
            user_id=user_id,
            role=MESSAGE_ROLE_USER,
            content=list(content),
            status=MESSAGE_STATUS_COMPLETED,
            run_id=run_id,
            created_at=now,
            updated_at=now,
            metadata={},
        )
        assistant_message = MessageRecord(
            id=new_message_id(),
            thread_id=thread_id,
            user_id=user_id,
            role=MESSAGE_ROLE_ASSISTANT,
            content=[],
            status=MESSAGE_STATUS_IN_PROGRESS,
            run_id=run_id,
            created_at=assistant_now,
            updated_at=assistant_now,
            metadata={},
        )
        record = RunRecord(
            id=run_id,
            thread_id=thread_id,
            user_id=user_id,
            trigger_message_id=user_message.id,
            output_message_id=assistant_message.id,
            status=RUN_STATUS_QUEUED,
            started_at=now,
            completed_at=None,
            error=None,
            metadata={"userText": user_text},
        )
        title = user_text[: self.message_service.settings.agents_thread_title_max_length] or self.message_service.settings.agents_thread_default_title
        return self.run_repository.create_with_messages(
            run=record,
            user_message=user_message,
            assistant_message=assistant_message,
            title_if_first_message=title,
            thread_repository=self.message_service.thread_repository,
            message_repository=self.message_service.message_repository,
        )

    def mark_running(self, *, run_id: str) -> RunRecord:
        return self._update_status(run_id=run_id, status=RUN_STATUS_RUNNING)

    def mark_requires_action(self, *, run_id: str) -> RunRecord:
        return self._update_status(run_id=run_id, status=RUN_STATUS_REQUIRES_ACTION)

    def mark_completed(self, *, run_id: str, output_message_ids: list[str]) -> RunRecord:
        run = self.run_repository.find_by_id(run_id=run_id)
        if run is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"runId": run_id},
            )
        completed = self.run_repository.mark_completed(
            thread_id=run.thread_id,
            run_id=run_id,
            completed_at=datetime.now(timezone.utc),
            output_message_ids=output_message_ids,
        )
        if completed is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"runId": run_id},
            )
        return completed

    def mark_failed(self, *, run_id: str, message: str, details: dict | None = None) -> RunRecord:
        run = self.run_repository.find_by_id(run_id=run_id)
        if run is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"runId": run_id},
            )
        failed = self.run_repository.mark_failed(
            thread_id=run.thread_id,
            run_id=run_id,
            completed_at=datetime.now(timezone.utc),
            error={
                "code": ApiErrorCode.LANGGRAPH_RUNTIME_ERROR,
                "message": message,
                "details": dict(details or {}),
            },
        )
        if failed is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"runId": run_id},
            )
        return failed

    def mark_cancelled(self, *, run_id: str) -> RunRecord:
        return self._update_status(
            run_id=run_id,
            status=RUN_STATUS_CANCELLED,
            completed_at=datetime.now(timezone.utc),
        )

    def get_run(self, *, user_id: int, run_id: str) -> RunRecord:
        run = self.run_repository.find_by_id(run_id=run_id)
        if run is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"runId": run_id},
            )

        if run.user_id != user_id:
            raise ApiError(
                code=ApiErrorCode.FORBIDDEN,
                message="无权访问该线程",
                http_status=403,
                details={"threadId": run.thread_id},
            )
        return run

    def get_active_run(self, *, user_id: int, thread_id: str) -> RunRecord | None:
        run = self.run_repository.find_active_by_thread(thread_id=thread_id)
        if run is not None and run.user_id != user_id:
            raise ApiError(
                code=ApiErrorCode.FORBIDDEN,
                message="无权访问该线程",
                http_status=403,
                details={"threadId": thread_id},
            )
        return run

    def resume_run(self, *, user_id: int, run_id: str) -> RunRecord:
        run = self.get_run(user_id=user_id, run_id=run_id)
        if run.status != RUN_STATUS_REQUIRES_ACTION:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_ACTIVE,
                message="当前运行状态不可恢复",
                http_status=409,
                details={"runId": run_id, "status": run.status},
            )
        return self.mark_running(run_id=run_id)

    def _update_status(
        self,
        *,
        run_id: str,
        status: str,
        completed_at: datetime | None = None,
        error: dict | None = None,
        metadata: dict | None = None,
    ) -> RunRecord:
        run = self.run_repository.update_status(
            run_id=run_id,
            status=status,
            completed_at=completed_at,
            error=error,
            metadata=metadata,
        )
        if run is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"runId": run_id},
            )
        return run
