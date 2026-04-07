"""Governed tool broker for MCP tool execution."""

from __future__ import annotations

from typing import Any

from langchain_core.tools import StructuredTool

from app.registry.provider_registry import ProviderRegistry
from app.registry.skill_registry import SkillRegistry
from app.tasking.task_card import TaskCard
from app.tools.policies import ToolPolicy


class ToolBroker:
    def __init__(
        self,
        *,
        registry,
        skill_registry: SkillRegistry,
        provider_registry: ProviderRegistry,
        policy: ToolPolicy | None = None,
    ) -> None:
        self.registry = registry
        self.skill_registry = skill_registry
        self.provider_registry = provider_registry
        self.policy = policy or ToolPolicy.from_skill_registry(skill_registry)

    def assert_allowed(self, skill_id: str, tool_name: str) -> None:
        self.policy.assert_allowed(skill_id, tool_name)

    async def call(
        self,
        *,
        task: TaskCard,
        skill_id: str,
        tool_name: str,
        payload: dict[str, Any],
    ) -> Any:
        if skill_id not in task.allowed_skills:
            raise PermissionError(f"skill {skill_id} is not allowed for task {task.task_id}")
        self.assert_allowed(skill_id, tool_name)
        self._assert_write_allowed(task=task, skill_id=skill_id)
        provider = self.provider_registry.get_provider_for_skill(skill_id)
        merged_payload = self.inject_context(task, payload)
        return await self.registry.invoke(
            server_name=provider.server_name,
            tool_name=tool_name,
            payload=merged_payload,
        )

    async def get_skill_tools(self, skill_id: str, *, task: TaskCard | None = None) -> list[Any]:
        provider = self.provider_registry.get_provider_for_skill(skill_id)
        tools = await self.registry.get_provider_tools(provider.server_name)
        allowed_tool_names = set(self.skill_registry.get_skill(skill_id).tools)
        visible_tools = [tool for tool in tools if getattr(tool, "name", None) in allowed_tool_names]
        if task is None:
            return visible_tools
        return [self._bind_tool(task=task, skill_id=skill_id, tool=tool) for tool in visible_tools]

    async def get_task_tools(self, task: TaskCard) -> list[Any]:
        bound_tools: list[Any] = []
        seen_tool_names: set[str] = set()
        provider_tool_cache: dict[str, list[Any]] = {}
        for skill_id in task.allowed_skills:
            provider = self.provider_registry.get_provider_for_skill(skill_id)
            cached_tools = provider_tool_cache.get(provider.server_name)
            if cached_tools is None:
                provider_tools = await self.registry.get_provider_tools(provider.server_name)
                provider_tool_cache[provider.server_name] = provider_tools
            else:
                provider_tools = cached_tools

            allowed_tool_names = set(self.skill_registry.get_skill(skill_id).tools)
            visible_tools = [
                self._bind_tool(task=task, skill_id=skill_id, tool=tool)
                for tool in provider_tools
                if getattr(tool, "name", None) in allowed_tool_names
            ]
            for tool in visible_tools:
                tool_name = str(getattr(tool, "name", ""))
                if tool_name in seen_tool_names:
                    continue
                seen_tool_names.add(tool_name)
                bound_tools.append(tool)
        return bound_tools

    def _bind_tool(self, *, task: TaskCard, skill_id: str, tool: Any) -> StructuredTool:
        async def _invoke_bound_tool(**kwargs: Any) -> Any:
            return await self.call(
                task=task,
                skill_id=skill_id,
                tool_name=tool.name,
                payload=kwargs,
            )

        return StructuredTool.from_function(
            coroutine=_invoke_bound_tool,
            name=str(tool.name),
            description=str(getattr(tool, "description", "") or ""),
            args_schema=getattr(tool, "args_schema", None),
        )

    def inject_context(self, task: TaskCard, payload: dict[str, Any]) -> dict[str, Any]:
        merged_payload = dict(payload)
        for key in ("user_id", "session_id", "task_id"):
            if key in merged_payload:
                continue
            if key == "session_id":
                merged_payload[key] = task.session_id
                continue
            if key == "task_id":
                merged_payload[key] = task.task_id
                continue
            value = task.input_slots.get(key)
            if value is not None:
                merged_payload[key] = value
        return merged_payload

    def _assert_write_allowed(self, *, task: TaskCard, skill_id: str) -> None:
        skill = self.skill_registry.get_skill(skill_id)
        if skill.access_mode != "write":
            return
        if not task.requires_confirmation:
            raise PermissionError(f"write tool requires confirmation for task {task.task_id}")
