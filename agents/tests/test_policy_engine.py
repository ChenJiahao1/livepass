from app.orchestrator.policy_engine import PolicyEngine
from app.tasking.task_card import TaskCard


def test_policy_engine_caps_task_max_steps():
    engine = PolicyEngine(max_steps_limit=3)
    task = TaskCard(
        task_id="task-001",
        session_id="sess-001",
        domain="refund",
        task_type="refund_preview",
        skill_id="refund.preview",
        goal="确认订单是否可退款",
        input_slots={"order_id": "ORD-10001"},
        required_slots=["order_id"],
        max_steps=5,
        risk_level="medium",
        fallback_policy="handoff",
        expected_output_schema="refund_preview_v1",
    )

    applied = engine.apply(task)

    assert applied.max_steps == 3
