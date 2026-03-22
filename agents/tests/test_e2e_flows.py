import asyncio

from langchain_core.messages import HumanMessage
from langgraph.checkpoint.memory import InMemorySaver

import app.graph as graph_module
from app.graph import build_graph_app
from tests.fakes import ScriptedChatModel


class _StubAsyncAgent:
    def __init__(self, result):
        self._result = result

    async def handle(self, state):
        result = dict(self._result)
        if callable(self._result):
            result = self._result(state)
        return result


def _scripted_llm_for_flow(route: str, selected_order_id: str | None = None) -> ScriptedChatModel:
    structured_responses = [
        {
            "action": "delegate",
            "reply": "",
            "selected_order_id": selected_order_id,
            "business_ready": True,
            "reason": "business intent is ready",
        },
        {
            "next_agent": route,
            "selected_order_id": selected_order_id,
            "need_handoff": route == "handoff",
            "reason": f"route to {route}",
        },
    ]
    if route != "handoff":
        structured_responses.append(
            {
                "next_agent": "finish",
                "selected_order_id": selected_order_id,
                "need_handoff": False,
                "reason": f"{route} flow is complete",
            }
        )
    return ScriptedChatModel(structured_responses=structured_responses)


def test_smalltalk_flow_returns_coordinator_reply():
    app = build_graph_app(checkpointer=InMemorySaver())
    llm = ScriptedChatModel(
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

    result = asyncio.run(
        app.ainvoke(
            {"messages": [HumanMessage(content="你好")]},
            config={"configurable": {"thread_id": "flow-smalltalk"}},
            context={"llm": llm, "registry": None},
        )
    )

    assert result["current_agent"] == "coordinator"


def test_activity_flow_routes_to_activity(monkeypatch):
    app = build_graph_app(checkpointer=InMemorySaver())
    monkeypatch.setattr(
        graph_module,
        "ActivityAgent",
        lambda *args, **kwargs: _StubAsyncAgent(
            {
                "reply": "上海夏夜音乐节今晚开演。",
                "trace": ["activity:program"],
                "need_handoff": False,
                "selected_program_id": "PGM-2001",
                "completed": True,
                "result_summary": "节目详情已返回",
            }
        ),
    )

    result = asyncio.run(
        app.ainvoke(
            {"messages": [HumanMessage(content="上海有什么演出")]},
            config={"configurable": {"thread_id": "flow-activity"}},
            context={"llm": _scripted_llm_for_flow("activity"), "registry": None},
        )
    )

    assert result["current_agent"] == "activity"


def test_order_flow_routes_to_order(monkeypatch):
    app = build_graph_app(checkpointer=InMemorySaver())
    monkeypatch.setattr(
        graph_module,
        "OrderAgent",
        lambda *args, **kwargs: _StubAsyncAgent(
            {
                "reply": "订单 ORD-1001 当前状态为 paid。",
                "trace": ["order:ORD-1001"],
                "need_handoff": False,
                "selected_order_id": "ORD-1001",
                "completed": True,
                "result_summary": "订单已成功查询",
            }
        ),
    )

    result = asyncio.run(
        app.ainvoke(
            {"messages": [HumanMessage(content="查一下 ORD-1001")]},
            config={"configurable": {"thread_id": "flow-order"}},
            context={"llm": _scripted_llm_for_flow("order", "ORD-1001"), "registry": None},
        )
    )

    assert result["current_agent"] == "order"


def test_refund_flow_routes_to_refund(monkeypatch):
    app = build_graph_app(checkpointer=InMemorySaver())
    monkeypatch.setattr(
        graph_module,
        "RefundAgent",
        lambda *args, **kwargs: _StubAsyncAgent(
            {
                "reply": "订单 ORD-1001 可以申请退款。",
                "trace": ["refund:ORD-1001"],
                "need_handoff": False,
                "selected_order_id": "ORD-1001",
                "completed": True,
                "result_summary": "退款资格已确认",
            }
        ),
    )

    result = asyncio.run(
        app.ainvoke(
            {"messages": [HumanMessage(content="订单 ORD-1001 可以退款吗")]},
            config={"configurable": {"thread_id": "flow-refund"}},
            context={"llm": _scripted_llm_for_flow("refund", "ORD-1001"), "registry": None},
        )
    )

    assert result["current_agent"] == "refund"


def test_handoff_flow_routes_to_handoff(monkeypatch):
    app = build_graph_app(checkpointer=InMemorySaver())
    monkeypatch.setattr(
        graph_module,
        "HandoffAgent",
        lambda *args, **kwargs: _StubAsyncAgent(
            {
                "reply": "已为你转人工。",
                "trace": ["handoff:ticket"],
                "need_handoff": True,
                "selected_order_id": None,
                "completed": True,
                "result_summary": "人工工单已创建",
                "status": "handoff",
            }
        ),
    )

    result = asyncio.run(
        app.ainvoke(
            {"messages": [HumanMessage(content="我要转人工")]},
            config={"configurable": {"thread_id": "flow-handoff"}},
            context={"llm": _scripted_llm_for_flow("handoff"), "registry": None},
        )
    )

    assert result["current_agent"] == "handoff"
