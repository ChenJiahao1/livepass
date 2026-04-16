from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Callable, Mapping

from langgraph.types import Command

from app.agent_runtime.callbacks import RuntimeCallbacks
from app.runs.interrupt_bridge import InterruptBridge


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
                "current_user_id": str(user_id),
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

    async def _invoke_runtime(self, *, payload: Any, thread_id: str, user_id: int) -> dict[str, Any]:
        config = {"configurable": {"thread_id": thread_id}}
        context = {
            "registry": self.registry,
            "llm": self.llm,
            "knowledge_service": self.knowledge_service,
            "current_user_id": str(user_id),
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
            context = {
                "registry": self.registry,
                "llm": self.llm,
                "knowledge_service": self.knowledge_service,
                "current_user_id": str(run.user_id),
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
                    message_id=run.assistant_message_id,
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
                    message_id=run.assistant_message_id,
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
                message_id=run.assistant_message_id,
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
