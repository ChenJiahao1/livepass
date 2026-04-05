"""Skill-level tool allowlist policies."""

from __future__ import annotations

from app.registry.skill_registry import SkillRegistry


class ToolPolicy:
    def __init__(self, allowed_tools_by_skill: dict[str, set[str]]) -> None:
        self._allowed_tools_by_skill = allowed_tools_by_skill

    @classmethod
    def from_skill_registry(cls, registry: SkillRegistry) -> "ToolPolicy":
        allowed_tools_by_skill = {
            skill_id: set(registry.get_skill(skill_id).tools)
            for skill_id in registry.list_skills()
        }
        return cls(allowed_tools_by_skill)

    def assert_allowed(self, skill_id: str, tool_name: str) -> None:
        allowed_tools = self._allowed_tools_by_skill.get(skill_id, set())
        if tool_name not in allowed_tools:
            raise PermissionError(f"tool {tool_name} is not allowed for skill {skill_id}")

