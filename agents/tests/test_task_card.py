import pytest
from pydantic import ValidationError

from app.tasking.task_card import TaskCard


def test_task_card_uses_spec_defaults_and_schema_id():
    card = TaskCard(
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

    assert card.max_steps == 3
    assert card.risk_level == "medium"
    assert card.fallback_policy == "handoff"
    assert card.expected_output_schema == "refund_preview_v1"


@pytest.mark.parametrize("risk_level", ["urgent", ""])
def test_task_card_rejects_invalid_risk_level(risk_level: str):
    with pytest.raises(ValidationError):
        TaskCard(
            task_id="task-001",
            session_id="sess-001",
            domain="refund",
            task_type="refund_preview",
            skill_id="refund.preview",
            goal="确认订单是否可退款",
            input_slots={"order_id": "ORD-10001"},
            required_slots=["order_id"],
            risk_level=risk_level,
            fallback_policy="handoff",
            expected_output_schema="refund_preview_v1",
        )


@pytest.mark.parametrize("fallback_policy", ["handoff.create_ticket", "unknown"])
def test_task_card_rejects_invalid_fallback_policy(fallback_policy: str):
    with pytest.raises(ValidationError):
        TaskCard(
            task_id="task-001",
            session_id="sess-001",
            domain="refund",
            task_type="refund_preview",
            skill_id="refund.preview",
            goal="确认订单是否可退款",
            input_slots={"order_id": "ORD-10001"},
            required_slots=["order_id"],
            risk_level="medium",
            fallback_policy=fallback_policy,
            expected_output_schema="refund_preview_v1",
        )
