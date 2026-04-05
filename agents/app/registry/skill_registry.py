"""Skill registry for orchestrator routing metadata."""

from __future__ import annotations

import re
from pathlib import Path
from typing import Any

import yaml
from pydantic import BaseModel, Field

from app.tasking.task_card import FallbackPolicy

DEFAULT_REGISTRY_YAML = Path(__file__).resolve().parents[1] / "skills" / "registry.yaml"
SKILL_FRONTMATTER_PATTERN = re.compile(r"^---\s*\n(.*?)\n---\s*\n?(.*)$", re.DOTALL)
SKILL_NAME_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
SUPPORTED_FRONTMATTER_KEYS = {
    "name",
    "description",
    "license",
    "compatibility",
    "metadata",
    "allowed-tools",
    # DeerFlow 对外部技能安装也接受这些可选扩展键，这里一并兼容。
    "version",
    "author",
}


class SkillDocument(BaseModel):
    name: str
    description: str
    raw_markdown: str
    body: str
    license: str | None = None
    compatibility: str | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)
    allowed_tools: list[str] = Field(default_factory=list)


class SkillSpec(BaseModel):
    """Declarative skill metadata loaded from the registry config."""

    skill_id: str
    version: str = "v1"
    domain: str
    agent_type: str
    skill_path: str
    name: str
    description: str
    license: str | None = None
    compatibility: str | None = None
    metadata: dict[str, Any] = Field(default_factory=dict)
    allowed_tools: list[str] = Field(default_factory=list)
    provider: str
    tools: list[str] = Field(default_factory=list)
    required_slots: list[str] = Field(default_factory=list)
    max_steps: int = Field(default=3, ge=1)
    termination_rule: str
    fallback_policy: FallbackPolicy
    audit_tags: list[str] = Field(default_factory=list)


class SkillRegistry:
    def __init__(self, *, skills: dict[str, SkillSpec], base_dir: Path) -> None:
        self._skills = skills
        self._base_dir = base_dir
        self._document_cache: dict[str, SkillDocument] = {}
        self._markdown_cache: dict[str, str] = {}
        self._prompt_cache: dict[str, str] = {}

    @classmethod
    def from_config(cls, path: str | Path) -> "SkillRegistry":
        config_path = Path(path)
        raw_config = yaml.safe_load(config_path.read_text(encoding="utf-8")) or {}
        base_dir = config_path.parent.parent
        skills: dict[str, SkillSpec] = {}
        for skill_id, skill_config in (raw_config.get("skills", {}) or {}).items():
            cfg = skill_config or {}
            skill_path = str(cfg.get("skill_path") or "")
            document = cls._parse_skill_document(base_dir=base_dir, skill_path=skill_path)
            tools = list(cfg.get("tools", []) or [])
            if document.allowed_tools and set(document.allowed_tools) != set(tools):
                raise ValueError(
                    f"skill {skill_id} allowed-tools {document.allowed_tools} do not match registry tools {tools}"
                )
            skills[skill_id] = SkillSpec(
                skill_id=skill_id,
                version=str(cfg.get("version", "v1")),
                domain=str(cfg.get("domain", "")),
                agent_type=str(cfg.get("agent_type", "")),
                skill_path=skill_path,
                name=document.name,
                description=document.description,
                license=document.license,
                compatibility=document.compatibility,
                metadata=document.metadata,
                allowed_tools=document.allowed_tools,
                provider=str(cfg.get("provider", "")),
                tools=tools,
                required_slots=list(cfg.get("required_slots", []) or []),
                max_steps=int(cfg.get("max_steps", 3)),
                termination_rule=str(cfg.get("termination_rule", "")),
                fallback_policy=cfg.get("fallback_policy", "handoff"),
                audit_tags=list(cfg.get("audit_tags", []) or []),
            )
        return cls(skills=skills, base_dir=base_dir)

    @classmethod
    def from_default(cls) -> "SkillRegistry":
        return cls.from_config(DEFAULT_REGISTRY_YAML)

    def get_skill(self, skill_id: str) -> SkillSpec:
        skill = self._skills.get(skill_id)
        if skill is None:
            raise KeyError(f"skill not registered: {skill_id}")
        return skill

    def list_skills(self) -> list[str]:
        return sorted(self._skills.keys())

    def load_prompt(self, skill_id: str) -> str:
        if skill_id not in self._prompt_cache:
            self._prompt_cache[skill_id] = self._load_document(skill_id).body
        return self._prompt_cache[skill_id]

    def load_skill_markdown(self, skill_id: str) -> str:
        if skill_id not in self._markdown_cache:
            self._markdown_cache[skill_id] = self._load_document(skill_id).raw_markdown
        return self._markdown_cache[skill_id]

    def _load_document(self, skill_id: str) -> SkillDocument:
        if skill_id not in self._document_cache:
            skill = self.get_skill(skill_id)
            self._document_cache[skill_id] = self._parse_skill_document(
                base_dir=self._base_dir,
                skill_path=skill.skill_path,
            )
        return self._document_cache[skill_id]

    @staticmethod
    def _parse_skill_document(*, base_dir: Path, skill_path: str) -> SkillDocument:
        if not skill_path:
            raise ValueError("skill_path is required")

        resolved_path = Path(skill_path)
        if not resolved_path.is_absolute():
            resolved_path = base_dir / resolved_path
        content = resolved_path.read_text(encoding="utf-8").strip()
        match = SKILL_FRONTMATTER_PATTERN.match(content)
        if match is None:
            raise ValueError(f"skill file missing frontmatter: {resolved_path}")

        raw_frontmatter = yaml.safe_load(match.group(1)) or {}
        if not isinstance(raw_frontmatter, dict):
            raise ValueError(f"skill frontmatter must be a mapping: {resolved_path}")
        unexpected_keys = sorted(set(raw_frontmatter) - SUPPORTED_FRONTMATTER_KEYS)
        if unexpected_keys:
            raise ValueError(f"skill file has unsupported frontmatter keys {unexpected_keys}: {resolved_path}")
        body = match.group(2).strip()
        allowed_tools = SkillRegistry._parse_allowed_tools(raw_frontmatter.get("allowed-tools"))

        name = str(raw_frontmatter.get("name", "")).strip()
        description = str(raw_frontmatter.get("description", "")).strip()
        if not name or not description:
            raise ValueError(f"skill file missing name/description: {resolved_path}")
        if not SKILL_NAME_PATTERN.match(name):
            raise ValueError(f"skill name must be lowercase hyphen-case: {resolved_path}")
        if len(name) > 64:
            raise ValueError(f"skill name too long (>64 chars): {resolved_path}")
        if len(description) > 1024:
            raise ValueError(f"skill description too long (>1024 chars): {resolved_path}")

        metadata = raw_frontmatter.get("metadata") or {}
        if not isinstance(metadata, dict):
            raise ValueError(f"skill metadata must be a mapping: {resolved_path}")
        normalized_metadata: dict[str, str] = {}
        for key, value in metadata.items():
            if not isinstance(key, str) or not isinstance(value, str):
                raise ValueError(f"skill metadata must be a string-to-string mapping: {resolved_path}")
            normalized_metadata[key] = value

        compatibility = raw_frontmatter.get("compatibility")
        license_text = raw_frontmatter.get("license")
        if compatibility is not None and not isinstance(compatibility, str):
            raise ValueError(f"skill compatibility must be a string: {resolved_path}")
        if isinstance(compatibility, str) and len(compatibility) > 500:
            raise ValueError(f"skill compatibility too long (>500 chars): {resolved_path}")
        if license_text is not None and not isinstance(license_text, str):
            raise ValueError(f"skill license must be a string: {resolved_path}")
        for optional_key in ("version", "author"):
            optional_value = raw_frontmatter.get(optional_key)
            if optional_value is not None and not isinstance(optional_value, str):
                raise ValueError(f"skill {optional_key} must be a string: {resolved_path}")
        return SkillDocument(
            name=name,
            description=description,
            raw_markdown=content,
            body=body,
            license=str(license_text) if license_text is not None else None,
            compatibility=str(compatibility) if compatibility is not None else None,
            metadata=normalized_metadata,
            allowed_tools=allowed_tools,
        )

    @staticmethod
    def _parse_allowed_tools(raw_value: Any) -> list[str]:
        if raw_value is None:
            return []
        if not isinstance(raw_value, str):
            raise ValueError("allowed-tools must be a space-delimited string")
        return [item for item in raw_value.split() if item]
