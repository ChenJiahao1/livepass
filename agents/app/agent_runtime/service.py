from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

from langgraph.types import Command

from app.agent_runtime.callbacks import RuntimeCallbacks
from app.runs.interrupt_bridge import InterruptBridge


@dataclass(slots=True)
class AgentRuntimeResult:
    reply: str
    metadata: dict[str, Any] = field(default_factory=dict)
    raw_result: dict[str, Any] = field(default_factory=dict)


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
    ) -> dict[str, Any]:
        await callbacks.on_run_started(run=run)
        result = await self.invoke_start(run=run, user_text=user_text, callbacks=callbacks)
        return result

    async def invoke_start(
        self,
        *,
        run,
        user_text: str,
        callbacks: RuntimeCallbacks,
    ) -> dict[str, Any]:
        result = await self._invoke_runtime(
            payload={"messages": [{"role": "user", "content": user_text}]},
            thread_id=run.thread_id,
            user_id=run.user_id,
        )
        await self._project_result(run=run, callbacks=callbacks, result=result)
        return result

    async def invoke_resume(
        self,
        *,
        run,
        resume_payload: dict[str, Any],
        callbacks: RuntimeCallbacks,
    ) -> dict[str, Any]:
        result = await self._invoke_runtime(
            payload=Command(resume=resume_payload),
            thread_id=run.thread_id,
            user_id=run.user_id,
        )
        await self._project_result(run=run, callbacks=callbacks, result=result)
        return result

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

    async def _project_result(self, *, run, callbacks: RuntimeCallbacks, result: dict[str, Any]) -> None:
        reply = str(result.get("final_reply") or result.get("reply") or "")
        if reply:
            await callbacks.on_message_delta(
                run=run,
                message_id=str(run.metadata.get("assistantMessageId", "")),
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
