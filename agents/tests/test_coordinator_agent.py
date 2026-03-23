from langchain_core.messages import HumanMessage

from app.agents.coordinator import CoordinatorAgent
from tests.fakes import ScriptedChatModel


def test_coordinator_replies_to_smalltalk_with_trace():
    agent = CoordinatorAgent(
        llm=ScriptedChatModel(
            structured_responses=[
                {
                    "action": "respond",
                    "reply": "你好，这里是票务客服，我可以帮你查节目、订单、退款或转人工。",
                    "selected_order_id": None,
                    "business_ready": False,
                    "reason": "smalltalk",
                }
            ]
        )
    )

    result = agent.handle({"messages": [HumanMessage(content="你好")]})

    assert result["agent"] == "coordinator"
    assert result["action"] == "respond"
    assert "票务客服" in result["reply"]
    assert result["business_ready"] is False
    assert result["trace"] == ["coordinator:respond"]


def test_coordinator_delegates_business_request():
    agent = CoordinatorAgent(
        llm=ScriptedChatModel(
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
    )

    result = agent.handle({"messages": [HumanMessage(content="帮我查 ORD-1001")]})

    assert result["agent"] == "coordinator"
    assert result["action"] == "delegate"
    assert result["selected_order_id"] == "ORD-1001"
    assert result["trace"] == ["coordinator:delegate"]


def test_coordinator_includes_current_user_id_in_prompt_for_order_lookup():
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "action": "delegate",
                "reply": "",
                "selected_order_id": None,
                "business_ready": True,
                "reason": "current user can query own orders",
            }
        ]
    )
    agent = CoordinatorAgent(llm=llm)

    result = agent.handle(
        {
            "messages": [HumanMessage(content="你看看我有没有订单详情")],
            "current_user_id": "U-3001",
        }
    )

    assert result["action"] == "delegate"
    assert "current_user_id" in llm.structured_calls[0][0].content
    assert "U-3001" in llm.structured_calls[0][0].content
