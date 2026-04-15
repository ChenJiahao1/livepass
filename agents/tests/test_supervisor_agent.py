from langchain_core.messages import HumanMessage

from app.agents.supervisor import SupervisorAgent
from tests.fakes import ScriptedChatModel


def test_supervisor_routes_refund_request():
    agent = SupervisorAgent(
        llm=ScriptedChatModel(
            structured_responses=[
                {
                    "next_agent": "refund",
                    "selected_order_id": None,
                    "need_handoff": False,
                    "reason": "refund request",
                }
            ]
        )
    )

    result = agent.handle({"messages": [HumanMessage(content="我要退款")]})

    assert result["next_agent"] == "refund"
    assert result["route"] == "refund"
