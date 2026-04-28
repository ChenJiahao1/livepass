import asyncio
from pathlib import Path

from langgraph.checkpoint.base import empty_checkpoint

from app.graph.builder import build_graph_app
from app.integrations.storage.redis import RedisCheckpointSaver
from app.graph.state import ConversationState
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
    }

    assert state["selected_order_id"] == "ORD-10001"
    assert state["current_user_id"] == 3001


def test_conversation_state_protocol_removes_derived_business_flags():
    annotations = ConversationState.__annotations__

    assert "business_ready" not in annotations
    assert "delegated" not in annotations
    assert "coordinator_action" in annotations


def test_runtime_constants_are_exported_for_agent_and_route_names():
    from app.shared.runtime_constants import AGENT_ACTIVITY, AGENT_ORDER, INTENT_UNKNOWN, NEXT_AGENT_FINISH

    assert AGENT_ACTIVITY == "activity"
    assert AGENT_ORDER == "order"
    assert NEXT_AGENT_FINISH == "finish"
    assert INTENT_UNKNOWN == "unknown"


def test_graph_orchestration_is_split_into_stable_modules():
    graph_dir = Path("app/graph")

    assert (graph_dir / "builder.py").is_file()
    assert (graph_dir / "routing.py").is_file()
    assert (graph_dir / "nodes.py").is_file()
    assert (graph_dir / "state.py").is_file()
    assert not (graph_dir / "subgraphs" / "refund.py").exists()
    assert not Path("app/graph.py").exists()
    assert not Path("app/state.py").exists()


def test_graph_can_finish_after_coordinator_and_supervisor():
    app = build_graph_app()
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "action": "delegate",
                "reply": "",
                "route": "order",
                "selected_order_id": None,
                "selected_program_id": None,
                "reason": "business request",
            },
            {
                "next_agent": "finish",
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


def test_graph_delegates_business_request_without_required_slots():
    app = build_graph_app()
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "action": "delegate",
                "reply": "",
                "route": "order",
                "selected_order_id": None,
                "selected_program_id": None,
                "reason": "refund request without order id",
            },
            {"next_agent": "finish", "reason": "no specialist in this unit test"},
        ]
    )

    result = asyncio.run(
        app.ainvoke(
            {"messages": [{"role": "user", "content": "我要退款"}]},
            config={"configurable": {"thread_id": "conv-no-slots"}},
            context={"llm": llm, "registry": StubRegistry(), "current_user_id": 3001},
        )
    )

    assert result["coordinator_action"] == "delegate"
    assert "business_ready" not in result
    assert "delegated" not in result
