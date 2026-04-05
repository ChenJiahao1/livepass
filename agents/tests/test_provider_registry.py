from pathlib import Path

import pytest

from app.registry.provider_registry import ProviderRegistry
from app.registry.skill_registry import SkillRegistry


def test_provider_registry_maps_refund_preview_to_go_order_provider():
    registry = ProviderRegistry.from_config("app/skills/registry.yaml")

    provider = registry.get_provider_for_skill("refund.preview")

    assert provider.provider_type == "go_mcp"
    assert provider.server_name == "order"
    assert provider.toolset == "order"


def test_provider_registry_maps_handoff_to_python_provider():
    registry = ProviderRegistry.from_config("app/skills/registry.yaml")

    provider = registry.get_provider_for_skill("handoff.create_ticket")

    assert provider.provider_type == "python_mcp"
    assert provider.server_name == "handoff"


def test_skill_registry_loads_required_skill_metadata():
    registry = SkillRegistry.from_config(Path("app/skills/registry.yaml"))

    skill = registry.get_skill("refund.preview")

    assert skill.version == "v1"
    assert skill.domain == "refund"
    assert skill.agent_type == "refund"
    assert skill.skill_path == "skills/refund-preview/SKILL.md"
    assert skill.name == "refund-preview"
    assert "预览订单退款资格" in skill.description
    assert skill.allowed_tools == ["preview_refund_order"]
    assert skill.tools == ["preview_refund_order"]
    assert skill.required_slots == ["order_id"]
    assert skill.max_steps == 3
    assert skill.termination_rule == "got_preview_or_fail"
    assert skill.fallback_policy == "handoff"
    assert skill.audit_tags == ["refund", "preview"]


def test_skill_registry_loads_skill_markdown_content():
    registry = SkillRegistry.from_config(Path("app/skills/registry.yaml"))

    prompt = registry.load_prompt("refund.preview")

    assert not prompt.startswith("---")
    assert "用于预览订单退款资格" in prompt


def test_provider_registry_raises_for_unknown_skill():
    registry = ProviderRegistry.from_config("app/skills/registry.yaml")

    with pytest.raises(KeyError):
        registry.get_provider_for_skill("refund.unknown")
