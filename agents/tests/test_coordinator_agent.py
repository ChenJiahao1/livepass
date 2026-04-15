from langchain_core.messages import HumanMessage

from app.agents.coordinator import CoordinatorAgent
from tests.fakes import ScriptedChatModel


def test_coordinator_responds_to_smalltalk():
    agent = CoordinatorAgent(
        llm=ScriptedChatModel(
            structured_responses=[
                {
                    "action": "respond",
                    "reply": "你好，这里是票务客服。",
                    "selected_order_id": None,
                    "business_ready": False,
                    "reason": "smalltalk",
                }
            ]
        )
    )

    result = agent.handle({"messages": [HumanMessage(content="你好")]})

    assert result["action"] == "respond"
    assert result["trace"] == ["coordinator:respond"]


def test_coordinator_delegates_business_request():
    agent = CoordinatorAgent(
        llm=ScriptedChatModel(
            structured_responses=[
                {
                    "action": "delegate",
                    "reply": "",
                    "selected_order_id": None,
                    "business_ready": True,
                    "reason": "business request is ready",
                }
            ]
        )
    )

    result = agent.handle({"messages": [HumanMessage(content="帮我查订单")]})

    assert result["action"] == "delegate"
    assert result["business_ready"] is True
