"""Skill and provider resolution for TaskCards."""

from __future__ import annotations

from pydantic import BaseModel

from app.registry.provider_registry import ProviderRegistry, ProviderSpec
from app.registry.skill_registry import SkillRegistry, SkillSpec
from app.tasking.task_card import TaskCard


class SkillResolution(BaseModel):
    skills: list[SkillSpec]
    providers: list[ProviderSpec]


class SkillResolver:
    def __init__(self, *, skill_registry: SkillRegistry, provider_registry: ProviderRegistry) -> None:
        self.skill_registry = skill_registry
        self.provider_registry = provider_registry

    def resolve(self, task: TaskCard) -> SkillResolution:
        skills = [self.skill_registry.get_skill(skill_id) for skill_id in task.allowed_skills]
        providers: list[ProviderSpec] = []
        seen_provider_names: set[str] = set()
        for skill in skills:
            provider = self.provider_registry.get_provider_for_skill(skill.skill_id)
            if provider.name in seen_provider_names:
                continue
            seen_provider_names.add(provider.name)
            providers.append(provider)
        return SkillResolution(skills=skills, providers=providers)
