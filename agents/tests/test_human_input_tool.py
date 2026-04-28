import pytest
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.graph import END, START, StateGraph
from langgraph.types import Command

from app.agents.tools.human_tools import build_human_input_tool


@pytest.mark.anyio
async def test_human_input_interrupt_resumes_with_selected_values():
    tool = build_human_input_tool()

    async def _node(_state: dict):
        result = await tool.ainvoke(
            {
                "action": "select_refund_order",
                "title": "选择要退款的订单",
                "description": "请选择一个订单继续退款流程。",
                "values": {"orders": [{"order_id": "ORD-1", "label": "A / 已支付 / 299 元"}]},
                "allowed_actions": ["edit", "reject"],
            }
        )
        return {"result": result}

    builder = StateGraph(dict)
    builder.add_node("ask", _node)
    builder.add_edge(START, "ask")
    builder.add_edge("ask", END)
    graph = builder.compile(checkpointer=InMemorySaver())
    config = {"configurable": {"thread_id": "human-input-select-order"}}

    interrupted = await graph.ainvoke({}, config=config)
    payload = interrupted["__interrupt__"][0].value

    assert payload["toolName"] == "human_input"
    assert payload["args"]["action"] == "select_refund_order"
    assert payload["request"]["allowedActions"] == ["edit", "reject"]

    resumed = await graph.ainvoke(
        Command(resume={"action": "edit", "values": {"order_id": "ORD-1"}}),
        config=config,
    )

    assert resumed["result"] == {"action": "edit", "values": {"order_id": "ORD-1"}}
