from datetime import datetime, timezone

import pytest

from app.runs.interrupt_models import HumanInterruptPayload
from app.common.errors import ApiError, ApiErrorCode
from app.runs.execution.interrupt_bridge import InterruptBridge
from app.runs.tool_call_models import TOOL_CALL_STATUS_WAITING_HUMAN, ToolCallRecord
from app.runs.tool_call_repository import InMemoryToolCallRepository


def test_interrupt_bridge_projects_approval_payload_to_human_tool_contract():
    bridge = InterruptBridge()
    payload = HumanInterruptPayload(
        tool_name="human_approval",
        action="refund_order",
        args={"orderId": "ORD-1", "refundAmount": "100"},
        request={
            "title": "退款前确认",
            "description": "订单 ORD-1 预计退款 100",
            "riskLevel": "medium",
        },
    )

    projected = bridge.project_interrupt(
        tool_call_id="tool_01",
        interrupt=payload,
    )

    assert projected == {
        "toolCallId": "tool_01",
        "toolName": "human_approval",
        "args": {"orderId": "ORD-1", "refundAmount": "100"},
        "request": {
            "title": "退款前确认",
            "description": "订单 ORD-1 预计退款 100",
            "riskLevel": "medium",
            "allowedActions": ["approve", "reject", "edit"],
        },
        "humanRequest": {
            "kind": "approval",
            "title": "退款前确认",
            "description": "订单 ORD-1 预计退款 100",
            "allowedActions": ["approve", "reject", "edit"],
        },
    }


def test_bridge_rejects_second_waiting_human_tool_call_in_same_run():
    repo = InMemoryToolCallRepository()
    repo.create(
        ToolCallRecord(
            id="tool_01",
            run_id="run_01",
            message_id="msg_01",
            thread_id="thr_01",
            user_id=3001,
            name="human_approval",
            status=TOOL_CALL_STATUS_WAITING_HUMAN,
            input={"action": "refund_order"},
            human_request={"title": "退款前确认"},
            created_at=datetime.now(timezone.utc),
            updated_at=datetime.now(timezone.utc),
        )
    )

    with pytest.raises(ApiError) as exc_info:
        InterruptBridge().assert_no_waiting_human_tool_call(
            tool_call_repository=repo,
            run_id="run_01",
        )

    assert exc_info.value.http_status == 409


def test_bridge_maps_approve_to_langgraph_decision_payload():
    record = ToolCallRecord(
        id="tool_01",
        run_id="run_01",
        message_id="msg_01",
        thread_id="thr_01",
        user_id=3001,
        name="human_approval",
        status=TOOL_CALL_STATUS_WAITING_HUMAN,
        input={
            "action": "refund_order",
            "values": {"order_id": "ORD-1", "reason": "用户发起退款", "user_id": "3001"},
        },
        human_request={"title": "退款前确认", "allowedActions": ["approve", "reject", "edit"]},
    )

    payload = InterruptBridge().build_command_resume_payload(
        tool_call=record,
        action_payload={"action": "approve", "reason": "同意退款", "values": {"operator": "agent"}},
    )

    assert payload == {
        "decisions": [{"type": "approve"}],
    }


def test_bridge_maps_edit_to_edited_action_payload():
    record = ToolCallRecord(
        id="tool_01",
        run_id="run_01",
        message_id="msg_01",
        thread_id="thr_01",
        user_id=3001,
        name="human_approval",
        status=TOOL_CALL_STATUS_WAITING_HUMAN,
        input={
            "action": "refund_order",
            "values": {"order_id": "ORD-1", "reason": "用户发起退款", "user_id": "3001"},
        },
        human_request={"title": "退款前确认", "allowedActions": ["approve", "reject", "edit"]},
    )

    payload = InterruptBridge().build_command_resume_payload(
        tool_call=record,
        action_payload={"action": "edit", "reason": "改成备注退款", "values": {"reason": "客服修改后退款"}},
    )

    assert payload == {
        "decisions": [
            {
                "type": "edit",
                "edited_action": {
                    "name": "refund_order",
                    "args": {
                        "order_id": "ORD-1",
                        "reason": "客服修改后退款",
                        "user_id": "3001",
                    },
                },
            }
        ]
    }


def test_bridge_rejects_action_outside_allowed_actions():
    record = ToolCallRecord(
        id="tool_01",
        run_id="run_01",
        message_id="msg_01",
        thread_id="thr_01",
        user_id=3001,
        name="human_approval",
        status=TOOL_CALL_STATUS_WAITING_HUMAN,
        input={"action": "refund_order", "values": {"order_id": "ORD-1"}},
        human_request={"title": "退款前确认", "allowedActions": ["approve", "reject"]},
    )

    with pytest.raises(ApiError) as exc_info:
        InterruptBridge().build_command_resume_payload(
            tool_call=record,
            action_payload={"action": "edit", "reason": "不允许编辑", "values": {"reason": "x"}},
        )

    assert exc_info.value.code == ApiErrorCode.TOOL_CALL_DECISION_NOT_ALLOWED
