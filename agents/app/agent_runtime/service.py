from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any

from app.observability.audit import build_audit_record
from app.observability.tracing import build_trace_record


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
    ) -> None:
        self.agent_runtime = agent_runtime
        self.registry = registry
        self.llm = llm
        self.knowledge_service = knowledge_service

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
        _ = build_trace_record(
            route_source=metadata["routeSource"],
            result=result,
            thread_id=thread_id,
            user_id=user_id,
        )
        _ = build_audit_record(result=result)
        return AgentRuntimeResult(reply=reply, metadata=metadata, raw_result=result)
