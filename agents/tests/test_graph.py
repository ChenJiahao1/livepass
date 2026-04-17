import asyncio

from langgraph.checkpoint.base import empty_checkpoint

from app.graph import build_graph_app
from app.session.checkpointer import RedisCheckpointSaver
from app.state import ConversationState
from tests.fakes import FakeRedis, ScriptedChatModel, StubRegistry


def test_checkpointer_round_trip():
    saver = RedisCheckpointSaver(redis_client=FakeRedis(), ttl_seconds=60)
    checkpoint = empty_checkpoint()
    metadata = {"source": "input", "step": 1}
    config = {"configurable": {"thread_id": "conv-1"}}

    next_config = saver.put(config, checkpoint, metadata, {})
    loaded = saver.get_tuple(next_config)

    assert loaded is not None
    assert loaded.config["configurable"]["thread_id"] == "conv-1"
    assert loaded.checkpoint["id"] == checkpoint["id"]


def test_conversation_state_accepts_cross_turn_fields():
    state: ConversationState = {
        "messages": [],
        "last_intent": "unknown",
        "selected_order_id": "ORD-10001",
        "current_user_id": 3001,
        "need_handoff": False,
    }

    assert state["selected_order_id"] == "ORD-10001"
    assert state["current_user_id"] == 3001


def test_graph_can_finish_after_coordinator_and_supervisor():
    app = build_graph_app()
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "action": "delegate",
                "reply": "",
                "selected_order_id": None,
                "business_ready": True,
                "reason": "business request",
            },
            {
                "next_agent": "finish",
                "selected_order_id": None,
                "need_handoff": False,
                "reason": "finish immediately",
            },
        ]
    )

    result = asyncio.run(
        app.ainvoke(
            {"messages": [{"role": "user", "content": "帮我查订单"}]},
            config={"configurable": {"thread_id": "conv-finish"}},
            context={"llm": llm, "registry": StubRegistry(), "current_user_id": 3001},
        )
    )

    assert result["current_agent"] == "supervisor"
    assert result["final_reply"] == ""
