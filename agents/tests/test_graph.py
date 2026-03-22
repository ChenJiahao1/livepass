import asyncio

from langgraph.checkpoint.memory import InMemorySaver

from app.graph import build_graph_app
from tests.fakes import ScriptedChatModel, StubRegistry


def test_graph_routes_refund_request_and_finishes():
    app = build_graph_app(checkpointer=InMemorySaver())
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
            config={"configurable": {"thread_id": "conv-refund"}},
            context={"llm": llm, "registry": StubRegistry(), "current_user_id": "3001"},
        )
    )

    assert result["current_agent"] == "refund"
    assert "请先提供需要处理的订单号" in result["final_reply"]


def test_graph_finishes_without_llm_and_persists_assistant_reply_in_messages():
    app = build_graph_app(checkpointer=InMemorySaver())
    config = {"configurable": {"thread_id": "conv-no-llm"}}

    result = asyncio.run(
        app.ainvoke(
            {"messages": [{"role": "user", "content": "我想退款"}]},
            config=config,
            context={"llm": None, "registry": StubRegistry(), "current_user_id": "3001"},
        )
    )
    snapshot = asyncio.run(app.aget_state(config))

    assert result["current_agent"] == "refund"
    assert "请先提供需要处理的订单号" in result["final_reply"]
    assert [message.content for message in snapshot.values["messages"]] == [
        "我想退款",
        "请先提供需要处理的订单号。",
    ]


def test_graph_uses_thread_id_memory_for_follow_up_messages():
    app = build_graph_app(checkpointer=InMemorySaver())
    config = {"configurable": {"thread_id": "conv-follow-up"}}

    asyncio.run(
        app.ainvoke(
            {"messages": [{"role": "user", "content": "你好"}]},
            config=config,
            context={"llm": None, "registry": StubRegistry(), "current_user_id": "3001"},
        )
    )
    asyncio.run(
        app.ainvoke(
            {"messages": [{"role": "user", "content": "我想退款"}]},
            config=config,
            context={"llm": None, "registry": StubRegistry(), "current_user_id": "3001"},
        )
    )
    snapshot = asyncio.run(app.aget_state(config))

    assert [message.content for message in snapshot.values["messages"]] == [
        "你好",
        "请补充一下你想咨询的节目、订单或退款问题。",
        "我想退款",
        "请先提供需要处理的订单号。",
    ]
