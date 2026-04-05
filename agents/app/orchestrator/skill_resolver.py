"""Skill and provider resolution for TaskCards."""

from __future__ import annotations

from pydantic import BaseModel

from app.registry.provider_registry import ProviderRegistry, ProviderSpec
from app.registry.skill_registry import SkillRegistry, SkillSpec
from app.tasking.task_card import TaskCard


class SkillResolution(BaseModel):
    skill: SkillSpec
    provider: ProviderSpec


class SkillResolver:
    def __init__(self, *, skill_registry: SkillRegistry, provider_registry: ProviderRegistry) -> None:
        self.skill_registry = skill_registry
        self.provider_registry = provider_registry

    def resolve(self, task: TaskCard) -> SkillResolution:
        skill = self.skill_registry.get_skill(task.skill_id)
        provider = self.provider_registry.get_provider_for_skill(task.skill_id)
        return SkillResolution(skill=skill, provider=provider)
