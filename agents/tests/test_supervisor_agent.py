from app.agents.supervisor import SupervisorAgent
from tests.fakes import ScriptedChatModel


def test_supervisor_routes_refund_request():
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "next_agent": "refund",
                "selected_order_id": "ORD-1001",
                "need_handoff": False,
                "reason": "refund request",
            }
        ]
    )
    agent = SupervisorAgent(llm=llm)

    result = agent.handle({"messages": [{"role": "user", "content": "订单 ORD-1001 可以退款吗"}]})

    assert result["next_agent"] == "refund"
    assert result["selected_order_id"] == "ORD-1001"
