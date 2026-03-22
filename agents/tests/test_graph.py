import asyncio

from app.graph import build_graph_app
from tests.fakes import ScriptedChatModel, StubRegistry


def test_graph_routes_refund_request_and_finishes():
    app = build_graph_app()
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "action": "delegate",
                "reply": "",
                "selected_order_id": None,
                "business_ready": True,
                "reason": "refund request",
            },
            {
                "next_agent": "refund",
                "selected_order_id": None,
                "need_handoff": False,
                "reason": "refund request",
            },
            {
                "next_agent": "finish",
                "selected_order_id": None,
                "need_handoff": False,
                "reason": "done",
            },
        ]
    )

    result = asyncio.run(
        app.ainvoke(
            {"messages": [{"role": "user", "content": "我想退款"}]},
            context={"llm": llm, "registry": StubRegistry(), "current_user_id": "3001"},
        )
    )

    assert result["current_agent"] == "refund"
    assert "请先提供需要处理的订单号" in result["final_reply"]
