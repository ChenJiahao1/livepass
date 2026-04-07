import pytest
from langchain_core.messages import AIMessage

from tests.test_order_refund_flow import build_flow_agent
from tests.fakes import ScriptedChatModel, make_tool_call_message


@pytest.mark.anyio
async def test_runtime_returns_handoff_when_business_flow_fails():
    agent = build_flow_agent(force_tool_failure=True)
    llm = ScriptedChatModel(
        structured_responses=[
            {"action": "delegate", "task_type": "order_list_recent"},
        ],
        responses=[
            make_tool_call_message("list_user_orders", {"user_id": 3001}),
            make_tool_call_message("create_handoff_ticket", {"reason": "帮我退最近那单"}),
            AIMessage(content="已创建人工工单。"),
        ],
    )

    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": "帮我退最近那单"}]},
        config={"configurable": {"thread_id": "sess-001"}},
        context={"current_user_id": "3001", "llm": llm, "session_state": {"user_id": 3001}},
    )

    assert result["need_handoff"] is True
    assert result["reply"] == "已为你转接人工客服，请稍候。"
