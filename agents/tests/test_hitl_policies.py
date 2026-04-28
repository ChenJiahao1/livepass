from app.agents.tools.hitl_policies import HITL_TOOL_POLICIES


def test_refund_order_policy_is_preview_then_approve():
    policy = HITL_TOOL_POLICIES["refund_order"]

    assert policy.mode == "preview_then_approve"
    assert policy.preview_tool_name == "preview_refund_order"
    assert policy.allowed_actions == ("approve", "reject", "edit")
