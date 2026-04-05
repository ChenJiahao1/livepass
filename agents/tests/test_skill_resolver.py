from app.orchestrator.skill_resolver import SkillResolver
from app.registry.provider_registry import ProviderRegistry
from app.registry.skill_registry import SkillRegistry
from app.tasking.task_card import TaskCard


def test_skill_resolver_resolves_refund_preview_to_order_provider():
    resolver = SkillResolver(
        skill_registry=SkillRegistry.from_config("app/skills/registry.yaml"),
        provider_registry=ProviderRegistry.from_config("app/skills/registry.yaml"),
    )
    task = TaskCard(
        task_id="task-001",
        session_id="sess-001",
        domain="refund",
        task_type="refund_preview",
        skill_id="refund.preview",
        goal="确认订单是否可退款",
        input_slots={"order_id": "ORD-10001"},
        required_slots=["order_id"],
        risk_level="medium",
        fallback_policy="handoff",
        expected_output_schema="refund_preview_v1",
    )

    resolution = resolver.resolve(task)

    assert resolution.skill.skill_id == "refund.preview"
    assert resolution.provider.server_name == "order"
    assert resolution.skill.tools == ["preview_refund_order"]
