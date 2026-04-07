from pathlib import Path

import pytest

from app.registry.provider_registry import ProviderRegistry
from app.registry.skill_registry import SkillRegistry


def test_provider_registry_maps_refund_read_to_go_order_provider():
    registry = ProviderRegistry.from_config("app/skills/registry.yaml")

    provider = registry.get_provider_for_skill("refund.read")

    assert provider.provider_type == "go_mcp"
    assert provider.server_name == "order"
    assert provider.toolset == "order"


def test_provider_registry_maps_handoff_write_to_python_provider():
    registry = ProviderRegistry.from_config("app/skills/registry.yaml")

    provider = registry.get_provider_for_skill("handoff.write")

    assert provider.provider_type == "python_mcp"
    assert provider.server_name == "handoff"


def test_skill_registry_loads_required_skill_metadata():
    registry = SkillRegistry.from_config(Path("app/skills/registry.yaml"))

    skill = registry.get_skill("refund.read")

    assert skill.version == "v1"
    assert skill.domain == "refund"
    assert skill.agent_type == "refund"
    assert skill.skill_path == "skills/refund-read/SKILL.md"
    assert skill.name == "refund-read"
    assert "退款相关只读查询" in skill.description
    assert skill.allowed_tools == ["list_user_orders", "get_order_detail_for_service", "preview_refund_order"]
    assert skill.tools == ["list_user_orders", "get_order_detail_for_service", "preview_refund_order"]
    assert skill.required_slots == []
    assert skill.max_steps == 4
    assert skill.termination_rule == "got_refund_read_result_or_fail"
    assert skill.fallback_policy == "return_parent"
    assert skill.audit_tags == ["refund", "read"]


def test_skill_registry_loads_skill_markdown_content():
    registry = SkillRegistry.from_config(Path("app/skills/registry.yaml"))

    prompt = registry.load_prompt("refund.read")

    assert not prompt.startswith("---")
    assert "退款相关查询" in prompt


def test_old_single_step_skills_are_not_registered():
    registry = ProviderRegistry.from_config("app/skills/registry.yaml")

    with pytest.raises(KeyError):
        registry.get_provider_for_skill("refund.preview")
    with pytest.raises(KeyError):
        registry.get_provider_for_skill("refund.submit")
    with pytest.raises(KeyError):
        registry.get_provider_for_skill("handoff.create_ticket")


def test_provider_registry_raises_for_unknown_skill():
    registry = ProviderRegistry.from_config("app/skills/registry.yaml")

    with pytest.raises(KeyError):
        registry.get_provider_for_skill("refund.unknown")
