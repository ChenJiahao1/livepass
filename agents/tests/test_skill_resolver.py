from app.orchestrator.skill_resolver import SkillResolver
from app.registry.provider_registry import ProviderRegistry
from app.registry.skill_registry import SkillRegistry
from app.tasking.task_card import TaskCard


def test_skill_resolver_resolves_refund_read_to_order_provider():
    resolver = SkillResolver(
        skill_registry=SkillRegistry.from_config("app/skills/registry.yaml"),
        provider_registry=ProviderRegistry.from_config("app/skills/registry.yaml"),
    )
    task = TaskCard(
        task_id="task-001",
        session_id="sess-001",
        domain="refund",
        task_type="refund_read",
        goal="处理退款咨询并确认退款资格",
        input_slots={"order_id": "ORD-10001"},
        required_slots=["order_id"],
        allowed_skills=["refund.read"],
        risk_level="medium",
        fallback_policy="handoff",
        expected_output_schema="refund_read_result_v1",
    )

    resolution = resolver.resolve(task)

    assert [item.skill_id for item in resolution.skills] == ["refund.read"]
    assert [item.server_name for item in resolution.providers] == ["order"]
    assert resolution.skills[0].tools == [
        "list_user_orders",
        "get_order_detail_for_service",
        "preview_refund_order",
    ]
