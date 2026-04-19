from langchain_core.messages import HumanMessage

from app.agents.supervisor import SupervisorAgent
from tests.fakes import ScriptedChatModel


def test_supervisor_routes_refund_request():
    agent = SupervisorAgent(
        llm=ScriptedChatModel(
            structured_responses=[
                {
                    "next_agent": "order",
                    "selected_order_id": None,
                    "reason": "refund request",
                }
            ]
        )
    )

    result = agent.handle({"messages": [HumanMessage(content="我要退款")]})

    assert result["next_agent"] == "order"
    assert result["route"] == "order"
