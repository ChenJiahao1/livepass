import pytest
from pydantic import ValidationError

from app.tasking.task_card import TaskCard


def test_task_card_uses_spec_defaults_and_schema_id():
    card = TaskCard(
        task_id="task-001",
        session_id="sess-001",
        domain="refund",
        task_type="refund_read",
        goal="处理退款咨询并确认退款资格",
        input_slots={"order_id": "ORD-10001"},
        required_slots=["order_id"],
        allowed_skills=["refund.read"],
        risk_level="medium",
        requires_confirmation=False,
        fallback_policy="handoff",
        expected_output_schema="refund_read_result_v1",
    )

    assert card.max_steps == 3
    assert card.risk_level == "medium"
    assert card.fallback_policy == "handoff"
    assert card.expected_output_schema == "refund_read_result_v1"
    assert card.allowed_skills == ["refund.read"]
    assert card.requires_confirmation is False


@pytest.mark.parametrize("risk_level", ["urgent", ""])
def test_task_card_rejects_invalid_risk_level(risk_level: str):
    with pytest.raises(ValidationError):
        TaskCard(
            task_id="task-001",
            session_id="sess-001",
            domain="refund",
            task_type="refund_read",
            goal="处理退款咨询并确认退款资格",
            input_slots={"order_id": "ORD-10001"},
            required_slots=["order_id"],
            allowed_skills=["refund.read"],
            risk_level=risk_level,
            fallback_policy="handoff",
            expected_output_schema="refund_read_result_v1",
        )


@pytest.mark.parametrize("fallback_policy", ["handoff.create_ticket", "unknown"])
def test_task_card_rejects_invalid_fallback_policy(fallback_policy: str):
    with pytest.raises(ValidationError):
        TaskCard(
            task_id="task-001",
            session_id="sess-001",
            domain="refund",
            task_type="refund_read",
            goal="处理退款咨询并确认退款资格",
            input_slots={"order_id": "ORD-10001"},
            required_slots=["order_id"],
            allowed_skills=["refund.read"],
            risk_level="medium",
            fallback_policy=fallback_policy,
            expected_output_schema="refund_read_result_v1",
        )


def test_task_card_rejects_empty_allowed_skills():
    with pytest.raises(ValidationError):
        TaskCard(
            task_id="task-001",
            session_id="sess-001",
            domain="refund",
            task_type="refund_read",
            goal="处理退款咨询并确认退款资格",
            input_slots={"order_id": "ORD-10001"},
            required_slots=["order_id"],
            allowed_skills=[],
            risk_level="medium",
            fallback_policy="handoff",
            expected_output_schema="refund_read_result_v1",
        )
