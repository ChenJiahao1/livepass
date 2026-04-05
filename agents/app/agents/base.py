"""Shared helpers for customer specialist agents."""

from __future__ import annotations

import re
from functools import lru_cache
from typing import Any

from app.orchestrator.skill_resolver import SkillResolver
from app.registry.provider_registry import ProviderRegistry
from app.registry.skill_registry import SkillRegistry
from app.runtime.subagent_runtime import SubagentRuntime
from app.state import ConversationState
from app.tasking.task_card import TaskCard
from app.tools.broker import ToolBroker
from app.tools.policies import ToolPolicy

ORDER_ID_PATTERN = re.compile(r"(ORD-\d{4,}|\d{4,})")
_UNSET = object()


@lru_cache(maxsize=1)
def _default_skill_registry() -> SkillRegistry:
    return SkillRegistry.from_default()


@lru_cache(maxsize=1)
def _default_provider_registry() -> ProviderRegistry:
    return ProviderRegistry.from_default()


class ToolCallingAgent:
    agent_name = ""
    toolset = ""

    def __init__(self, *, registry, llm) -> None:
        self.registry = registry
        self.llm = llm

    async def get_tools(self) -> list:
        if self.registry is None:
            return []
        return await self.registry.get_tools(self.toolset)

    async def handle(self, state: ConversationState) -> dict[str, Any]:
        raise NotImplementedError

    def initial_trace(self, state: ConversationState) -> list[str]:
        return []

    def result(
        self,
        state: ConversationState,
        *,
        reply: str,
        trace: list[str] | None = None,
        need_handoff: bool = False,
        completed: bool = True,
        result_summary: str | None = None,
        selected_order_id: str | None | object = _UNSET,
        selected_program_id: str | None | object = _UNSET,
        status: str | None = None,
    ) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "agent": self.agent_name,
            "reply": reply,
            "trace": list(trace or self.initial_trace(state)),
            "need_handoff": need_handoff,
            "completed": completed,
            "result_summary": result_summary or reply,
        }
        if selected_order_id is _UNSET:
            payload["selected_order_id"] = state.get("selected_order_id")
        else:
            payload["selected_order_id"] = selected_order_id
        if selected_program_id is _UNSET:
            payload["selected_program_id"] = state.get("selected_program_id")
        else:
            payload["selected_program_id"] = selected_program_id
        if status is not None:
            payload["status"] = status
        return payload

    def find_tool(self, tools: list, *names: str):
        tools_by_name = {tool.name: tool for tool in tools}
        for name in names:
            if name in tools_by_name:
                return tools_by_name[name]
        return None

    def latest_user_message(self, state: ConversationState) -> str:
        messages = state.get("messages", [])
        for message in reversed(messages):
            role = getattr(message, "type", None)
            if role is None and hasattr(message, "get"):
                role = message.get("role")
            if role in {"human", "user"}:
                if hasattr(message, "content"):
                    return str(message.content)
                return str(message.get("content", ""))
        return ""

    def extract_order_id(self, state: ConversationState) -> str | None:
        if state.get("selected_order_id"):
            return state["selected_order_id"]
        message = self.latest_user_message(state)
        match = ORDER_ID_PATTERN.search(message)
        if not match:
            return None
        return match.group(1)

    def _build_skill_runtime(self) -> SubagentRuntime:
        skill_registry = _default_skill_registry()
        provider_registry = _default_provider_registry()
        return SubagentRuntime(
            broker=ToolBroker(
                registry=self.registry,
                skill_registry=skill_registry,
                provider_registry=provider_registry,
                policy=ToolPolicy.from_skill_registry(skill_registry),
            ),
            skill_registry=skill_registry,
        )

    async def execute_skill(
        self,
        state: ConversationState,
        *,
        skill_id: str,
        task_type: str,
        goal: str,
        required_slots: list[str],
        fallback_policy: str,
        expected_output_schema: str,
        risk_level: str = "low",
        input_slots: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        session_id = str(state.get("conversation_id") or state.get("thread_id") or self.agent_name or "session")
        task = TaskCard(
            task_id=f"{self.agent_name}-task",
            session_id=session_id,
            domain=skill_id.split(".")[0],
            task_type=task_type,
            skill_id=skill_id,
            goal=goal,
            source_message=self.latest_user_message(state),
            input_slots=input_slots or {},
            required_slots=required_slots,
            risk_level=risk_level,
            fallback_policy=fallback_policy,
            expected_output_schema=expected_output_schema,
        )
        resolver = SkillResolver(
            skill_registry=_default_skill_registry(),
            provider_registry=_default_provider_registry(),
        )
        runtime = self._build_skill_runtime()
        return await runtime.execute(
            task=task,
            resolution=resolver.resolve(task),
            session_state={
                "selected_order_id": state.get("selected_order_id"),
                "recent_order_candidates": state.get("recent_order_candidates", []),
                "last_task_summary": state.get("last_task_summary"),
                "last_handoff_ticket_id": state.get("last_handoff_ticket_id"),
            },
            llm=self.llm,
        )
