import pytest

from tests.test_order_refund_flow import build_flow_agent


@pytest.mark.anyio
async def test_runtime_returns_handoff_when_business_flow_fails():
    agent = build_flow_agent(force_tool_failure=True)

    result = await agent.ainvoke(
        {"messages": [{"role": "user", "content": "帮我退最近那单"}]},
        config={"configurable": {"thread_id": "sess-001"}},
        context={"current_user_id": "3001", "llm": None, "session_state": {"user_id": 3001}},
    )

    assert result["need_handoff"] is True
