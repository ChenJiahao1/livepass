from langchain_core.messages import HumanMessage

from app.agents.supervisor import SupervisorAgent
from tests.fakes import ScriptedChatModel


def test_supervisor_routes_order_request_with_trace():
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "next_agent": "order",
                "selected_order_id": "ORD-1001",
                "need_handoff": False,
                "reason": "order request",
            }
        ]
    )
    agent = SupervisorAgent(llm=llm)

    result = agent.handle({"messages": [HumanMessage(content="查一下 ORD-1001")]})

    assert result["agent"] == "supervisor"
    assert result["next_agent"] == "order"
    assert result["route"] == "order"
    assert result["selected_order_id"] == "ORD-1001"
    assert result["trace"] == ["route:order"]


def test_supervisor_can_finish_after_specialist_result():
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "next_agent": "finish",
                "selected_order_id": "ORD-1001",
                "need_handoff": False,
                "reason": "order flow is complete",
            }
        ]
    )
    agent = SupervisorAgent(llm=llm)

    result = agent.handle(
        {
            "messages": [HumanMessage(content="查一下 ORD-1001")],
            "route": "order",
            "specialist_result": {
                "agent": "order",
                "completed": True,
                "result_summary": "订单已成功查询",
            },
        }
    )

    assert result["next_agent"] == "finish"
    assert result["route"] == "order"
    assert result["selected_order_id"] == "ORD-1001"
    assert result["trace"] == []


def test_supervisor_routes_knowledge_request():
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "next_agent": "knowledge",
                "selected_order_id": None,
                "need_handoff": False,
                "reason": "celebrity biography route",
            }
        ]
    )
    agent = SupervisorAgent(llm=llm)

    result = agent.handle({"messages": [HumanMessage(content="周杰伦是谁")]})

    assert result["next_agent"] == "knowledge"
    assert result["route"] == "knowledge"


def test_supervisor_includes_current_user_id_in_prompt_for_refund_lookup():
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "next_agent": "order",
                "selected_order_id": None,
                "need_handoff": False,
                "reason": "must list orders before refund",
            }
        ]
    )
    agent = SupervisorAgent(llm=llm)

    result = agent.handle(
        {
            "messages": [HumanMessage(content="我不知道订单号，你看看我有没有订单详情")],
            "last_intent": "refund",
            "current_user_id": "U-3001",
        }
    )

    assert result["next_agent"] == "order"
    assert "current_user_id" in llm.structured_calls[0][0].content
    assert "U-3001" in llm.structured_calls[0][0].content
