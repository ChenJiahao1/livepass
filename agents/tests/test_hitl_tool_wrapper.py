import pytest
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.graph import END, START, StateGraph
from langgraph.types import Command

from app.agents.tools.hitl_policies import HITL_TOOL_POLICIES
from app.agents.tools.hitl_wrapper import wrap_tool_with_hitl


class _FakeTool:
    description = "submit refund"

    def __init__(self, name: str) -> None:
        self.name = name


def _compile_single_node_graph(node):
    builder = StateGraph(dict)
    builder.add_node("call_tool", node)
    builder.add_edge(START, "call_tool")
    builder.add_edge("call_tool", END)
    return builder.compile(checkpointer=InMemorySaver())


@pytest.mark.anyio
async def test_refund_wrapper_previews_before_approval_and_executes_after_approve():
    calls: list[tuple[str, dict]] = []

    async def invoke(tool_name: str, payload: dict):
        calls.append((tool_name, dict(payload)))
        if tool_name == "preview_refund_order":
            return {"order_id": payload["order_id"], "allow_refund": True, "refund_amount": "199", "refund_percent": 80}
        return {"accepted": True, "order_id": payload["order_id"]}

    tool = wrap_tool_with_hitl(
        tool=_FakeTool("refund_order"),
        policy=HITL_TOOL_POLICIES["refund_order"],
        invoke_tool=invoke,
    )

    async def _node(_state: dict):
        return {"result": await tool.ainvoke({"order_id": "ORD-1", "reason": "用户发起退款"})}

    graph = _compile_single_node_graph(_node)
    config = {"configurable": {"thread_id": "refund-wrapper-approve"}}

    interrupted = await graph.ainvoke({}, config=config)
    payload = interrupted["__interrupt__"][0].value

    assert calls == [("preview_refund_order", {"order_id": "ORD-1", "reason": "用户发起退款"})]
    assert payload["toolName"] == "human_approval"
    assert "预计退款 199 元" in payload["request"]["description"]

    resumed = await graph.ainvoke(Command(resume={"action": "approve"}), config=config)

    assert resumed["result"]["accepted"] is True
    assert calls == [
        ("preview_refund_order", {"order_id": "ORD-1", "reason": "用户发起退款"}),
        ("preview_refund_order", {"order_id": "ORD-1", "reason": "用户发起退款"}),
        ("refund_order", {"order_id": "ORD-1", "reason": "用户发起退款"}),
    ]
    assert calls[-1] == ("refund_order", {"order_id": "ORD-1", "reason": "用户发起退款"})


@pytest.mark.anyio
async def test_refund_wrapper_returns_preview_reject_reason_without_interrupt():
    calls: list[tuple[str, dict]] = []

    async def invoke(tool_name: str, payload: dict):
        calls.append((tool_name, dict(payload)))
        return {"order_id": payload["order_id"], "allow_refund": False, "reason": "订单不可退款"}

    tool = wrap_tool_with_hitl(
        tool=_FakeTool("refund_order"),
        policy=HITL_TOOL_POLICIES["refund_order"],
        invoke_tool=invoke,
    )

    result = await tool.ainvoke({"order_id": "ORD-1", "reason": "用户发起退款"})

    assert result == {"order_id": "ORD-1", "allow_refund": False, "reason": "订单不可退款"}
    assert calls == [("preview_refund_order", {"order_id": "ORD-1", "reason": "用户发起退款"})]


@pytest.mark.anyio
async def test_refund_wrapper_edit_repreviews_and_then_executes_edited_payload():
    calls: list[tuple[str, dict]] = []

    async def invoke(tool_name: str, payload: dict):
        calls.append((tool_name, dict(payload)))
        if tool_name == "preview_refund_order":
            return {"order_id": payload["order_id"], "allow_refund": True, "refund_amount": "199"}
        return {"accepted": True, "payload": dict(payload)}

    tool = wrap_tool_with_hitl(
        tool=_FakeTool("refund_order"),
        policy=HITL_TOOL_POLICIES["refund_order"],
        invoke_tool=invoke,
    )

    async def _node(_state: dict):
        return {"result": await tool.ainvoke({"order_id": "ORD-1", "reason": "用户发起退款"})}

    graph = _compile_single_node_graph(_node)
    config = {"configurable": {"thread_id": "refund-wrapper-edit"}}

    await graph.ainvoke({}, config=config)
    edited = await graph.ainvoke(
        Command(resume={"action": "edit", "values": {"order_id": "ORD-2", "reason": "人工修改原因"}}),
        config=config,
    )

    assert "__interrupt__" in edited
    assert calls == [
        ("preview_refund_order", {"order_id": "ORD-1", "reason": "用户发起退款"}),
        ("preview_refund_order", {"order_id": "ORD-1", "reason": "用户发起退款"}),
        ("preview_refund_order", {"order_id": "ORD-2", "reason": "人工修改原因"}),
    ]

    resumed = await graph.ainvoke(Command(resume={"action": "approve"}), config=config)

    assert resumed["result"]["payload"] == {"order_id": "ORD-2", "reason": "人工修改原因"}
    assert calls[-1] == ("refund_order", {"order_id": "ORD-2", "reason": "人工修改原因"})


@pytest.mark.anyio
async def test_refund_wrapper_reject_returns_cancelled_without_real_write():
    calls: list[tuple[str, dict]] = []

    async def invoke(tool_name: str, payload: dict):
        calls.append((tool_name, dict(payload)))
        if tool_name == "preview_refund_order":
            return {"order_id": payload["order_id"], "allow_refund": True, "refund_amount": "199"}
        return {"accepted": True, "payload": dict(payload)}

    tool = wrap_tool_with_hitl(
        tool=_FakeTool("refund_order"),
        policy=HITL_TOOL_POLICIES["refund_order"],
        invoke_tool=invoke,
    )

    async def _node(_state: dict):
        return {"result": await tool.ainvoke({"order_id": "ORD-1", "reason": "用户发起退款"})}

    graph = _compile_single_node_graph(_node)
    config = {"configurable": {"thread_id": "refund-wrapper-reject"}}

    await graph.ainvoke({}, config=config)
    resumed = await graph.ainvoke(Command(resume={"action": "reject", "reason": "用户取消"}), config=config)

    assert resumed["result"] == {"cancelled": True, "reason": "用户取消"}
    assert calls == [
        ("preview_refund_order", {"order_id": "ORD-1", "reason": "用户发起退款"}),
        ("preview_refund_order", {"order_id": "ORD-1", "reason": "用户发起退款"}),
    ]
