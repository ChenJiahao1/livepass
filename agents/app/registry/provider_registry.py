"""Provider registry for skill -> provider resolution."""

from __future__ import annotations

from pathlib import Path

import yaml
from pydantic import BaseModel

from app.registry.skill_registry import SkillRegistry

DEFAULT_REGISTRY_YAML = Path(__file__).resolve().parents[1] / "skills" / "registry.yaml"


class ProviderSpec(BaseModel):
    """Runtime provider target for a skill."""

    name: str
    provider_type: str
    server_name: str
    toolset: str
    transport: str


class ProviderRegistry:
    def __init__(self, *, providers: dict[str, ProviderSpec], skill_registry: SkillRegistry) -> None:
        self._providers = providers
        self._skill_registry = skill_registry

    @classmethod
    def from_config(cls, path: str | Path) -> "ProviderRegistry":
        config_path = Path(path)
        raw_config = yaml.safe_load(config_path.read_text(encoding="utf-8")) or {}
        providers: dict[str, ProviderSpec] = {}
        for provider_name, provider_config in (raw_config.get("providers", {}) or {}).items():
            cfg = provider_config or {}
            providers[provider_name] = ProviderSpec(
                name=provider_name,
                provider_type=str(cfg.get("provider_type", "")),
                server_name=str(cfg.get("server_name", provider_name)),
                toolset=str(cfg.get("toolset", provider_name)),
                transport=str(cfg.get("transport", "")),
            )
        return cls(providers=providers, skill_registry=SkillRegistry.from_config(config_path))

    @classmethod
    def from_default(cls) -> "ProviderRegistry":
        return cls.from_config(DEFAULT_REGISTRY_YAML)

    def get_provider_for_skill(self, skill_id: str) -> ProviderSpec:
        skill = self._skill_registry.get_skill(skill_id)
        provider = self._providers.get(skill.provider)
        if provider is None:
            raise KeyError(f"provider not registered for skill {skill_id}: {skill.provider}")
        return provider
