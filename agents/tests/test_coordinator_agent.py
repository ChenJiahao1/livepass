from app.agents.coordinator import CoordinatorAgent
from tests.fakes import ScriptedChatModel


def test_coordinator_delegates_business_request():
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "action": "delegate",
                "reply": "",
                "selected_order_id": "ORD-1001",
                "business_ready": True,
                "reason": "business request is ready",
            }
        ]
    )
    agent = CoordinatorAgent(llm=llm)

    result = agent.handle({"messages": [{"role": "user", "content": "帮我查 ORD-1001"}]})

    assert result["action"] == "delegate"
    assert result["selected_order_id"] == "ORD-1001"
