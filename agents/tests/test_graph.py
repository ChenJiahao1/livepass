import asyncio
from types import SimpleNamespace

from langchain_core.messages import HumanMessage
from langgraph.checkpoint.memory import InMemorySaver

import app.graph as graph_module
from app.graph import _supervisor_node, build_graph_app
from tests.fakes import ScriptedChatModel


class _StubAsyncAgent:
    def __init__(self, handler):
        self._handler = handler

    async def handle(self, state):
        return self._handler(state)


def test_graph_passes_runtime_current_user_id_into_first_coordinator_turn():
    app = build_graph_app(checkpointer=InMemorySaver())
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "action": "clarify",
                "reply": "请描述一下你的问题。",
                "selected_order_id": None,
                "business_ready": False,
                "reason": "check prompt context on first turn",
            }
        ]
    )

    asyncio.run(
        app.ainvoke(
            {"messages": [HumanMessage(content="你看看我有没有订单详情")]},
            config={"configurable": {"thread_id": "conv-current-user"}},
            context={"llm": llm, "registry": None, "current_user_id": "U-3001"},
        )
    )

    assert "current_user_id" in llm.structured_calls[0][0].content
    assert "U-3001" in llm.structured_calls[0][0].content


def test_graph_returns_specialist_result_back_to_supervisor(monkeypatch):
    app = build_graph_app(checkpointer=InMemorySaver())

    monkeypatch.setattr(
        graph_module,
        "OrderAgent",
        lambda *args, **kwargs: _StubAsyncAgent(
            lambda state: {
                "reply": "订单 ORD-1001 当前状态为 paid。",
                "trace": ["order:ORD-1001"],
                "need_handoff": False,
                "selected_order_id": "ORD-1001",
                "completed": True,
                "result_summary": "订单已成功查询",
            }
        ),
    )

    llm = ScriptedChatModel(
        structured_responses=[
            {
                "action": "delegate",
                "reply": "",
                "selected_order_id": "ORD-1001",
                "business_ready": True,
                "reason": "order query is ready",
            },
            {
                "next_agent": "order",
                "selected_order_id": "ORD-1001",
                "need_handoff": False,
                "reason": "route to order",
            },
            {
                "next_agent": "finish",
                "selected_order_id": "ORD-1001",
                "need_handoff": False,
                "reason": "order flow is complete",
            },
        ]
    )

    result = asyncio.run(
        app.ainvoke(
            {"messages": [HumanMessage(content="查一下 ORD-1001")]},
            config={"configurable": {"thread_id": "conv-order-roundtrip"}},
            context={"llm": llm, "registry": None, "current_user_id": "U-3001"},
        )
    )

    assert result["current_agent"] == "order"
    assert "订单 ORD-1001 当前状态为 paid" in result["final_reply"]
    assert len(llm.structured_calls) == 3
    assert "specialist_result" in llm.structured_calls[2][0].content
    assert "订单已成功查询" in llm.structured_calls[2][0].content


def test_supervisor_node_finishes_after_order_list_is_shown():
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "next_agent": "order",
                "selected_order_id": None,
                "need_handoff": False,
                "reason": "llm would otherwise keep routing to order",
            }
        ]
    )
    state = {
        "route": "order",
        "selected_order_id": None,
        "current_user_id": "U-3001",
        "trace": ["coordinator:delegate", "route:order", "tool:list_user_orders"],
        "specialist_result": {
            "agent": "order",
            "completed": True,
            "need_handoff": False,
            "result_summary": "已向用户展示订单列表",
        },
    }

    result = _supervisor_node(
        state,
        SimpleNamespace(context={"llm": llm, "current_user_id": "U-3001"}),
    )

    assert result["next_agent"] == "finish"
    assert result["route"] == "order"
    assert result["selected_order_id"] is None
    assert result["trace"] == state["trace"]


def test_supervisor_node_finishes_after_knowledge_result():
    llm = ScriptedChatModel(
        structured_responses=[
            {
                "next_agent": "handoff",
                "selected_order_id": None,
                "need_handoff": True,
                "reason": "llm should not override completed knowledge result",
            }
        ]
    )
    state = {
        "route": "knowledge",
        "selected_order_id": None,
        "trace": ["coordinator:delegate", "route:knowledge", "knowledge:lightrag"],
        "specialist_result": {
            "agent": "knowledge",
            "completed": True,
            "need_handoff": False,
            "result_summary": "周杰伦基础百科已返回",
        },
    }

    result = _supervisor_node(
        state,
        SimpleNamespace(context={"llm": llm, "current_user_id": "U-3001"}),
    )

    assert result["next_agent"] == "finish"
    assert result["route"] == "knowledge"
    assert result["selected_order_id"] is None
    assert result["trace"] == state["trace"]


def test_extract_order_id_ignores_ai_messages():
    extract_order_id = getattr(graph_module, "_extract_order_id", None)
    state = {
        "messages": [
            HumanMessage(content="你看看我有没有订单详情"),
            {"role": "assistant", "content": "你当前有 ORD-1001 和 ORD-1002 两笔订单。"},
            HumanMessage(content="帮我退一下"),
        ],
        "selected_order_id": None,
    }

    assert callable(extract_order_id)
    assert extract_order_id(state) is None


def test_graph_reuses_selected_order_id_across_turns(monkeypatch):
    app = build_graph_app(checkpointer=InMemorySaver())

    monkeypatch.setattr(
        graph_module,
        "OrderAgent",
        lambda *args, **kwargs: _StubAsyncAgent(
            lambda state: {
                "reply": "订单 ORD-1001 当前状态为 paid。",
                "trace": ["order:ORD-1001"],
                "need_handoff": False,
                "selected_order_id": "ORD-1001",
                "completed": True,
                "result_summary": "订单已成功查询",
            }
        ),
    )

    seen_order_ids: list[str | None] = []

    monkeypatch.setattr(
        graph_module,
        "RefundAgent",
        lambda *args, **kwargs: _StubAsyncAgent(
            lambda state: {
                "reply": f"订单 {state.get('selected_order_id')} 可以申请退款。",
                "trace": [f"refund:{state.get('selected_order_id')}"],
                "need_handoff": False,
                "selected_order_id": state.get("selected_order_id"),
                "completed": True,
                "result_summary": "退款资格已确认",
            }
        ),
    )

    def _capture_selected_order_id(state):
        seen_order_ids.append(state.get("selected_order_id"))
        return {
            "reply": f"订单 {state.get('selected_order_id')} 可以申请退款。",
            "trace": [f"refund:{state.get('selected_order_id')}"],
            "need_handoff": False,
            "selected_order_id": state.get("selected_order_id"),
            "completed": True,
            "result_summary": "退款资格已确认",
        }

    monkeypatch.setattr(
        graph_module,
        "RefundAgent",
        lambda *args, **kwargs: _StubAsyncAgent(_capture_selected_order_id),
    )

    llm = ScriptedChatModel(
        structured_responses=[
            {
                "action": "delegate",
                "reply": "",
                "selected_order_id": "ORD-1001",
                "business_ready": True,
                "reason": "order id is present",
            },
            {
                "next_agent": "order",
                "selected_order_id": "ORD-1001",
                "need_handoff": False,
                "reason": "route to order",
            },
            {
                "next_agent": "finish",
                "selected_order_id": "ORD-1001",
                "need_handoff": False,
                "reason": "order flow is complete",
            },
            {
                "action": "delegate",
                "reply": "",
                "selected_order_id": None,
                "business_ready": True,
                "reason": "refund can reuse remembered order id",
            },
            {
                "next_agent": "refund",
                "selected_order_id": None,
                "need_handoff": False,
                "reason": "route to refund",
            },
            {
                "next_agent": "finish",
                "selected_order_id": None,
                "need_handoff": False,
                "reason": "refund flow is complete",
            },
        ]
    )
    config = {"configurable": {"thread_id": "conv-memory-reuse"}}
    context = {"llm": llm, "registry": None, "current_user_id": "U-3001"}

    asyncio.run(
        app.ainvoke(
            {"messages": [HumanMessage(content="查一下 ORD-1001")]},
            config=config,
            context=context,
        )
    )
    result = asyncio.run(
        app.ainvoke(
            {"messages": [HumanMessage(content="帮我退款")]},
            config=config,
            context=context,
        )
    )

    assert result["current_agent"] == "refund"
    assert result["selected_order_id"] == "ORD-1001"
    assert result["trace"] == ["coordinator:delegate", "route:refund", "refund:ORD-1001"]
    assert seen_order_ids == ["ORD-1001"]


def test_graph_lists_current_user_orders_before_refund(monkeypatch):
    app = build_graph_app(checkpointer=InMemorySaver())

    monkeypatch.setattr(
        graph_module,
        "OrderAgent",
        lambda *args, **kwargs: _StubAsyncAgent(
            lambda state: {
                "reply": "当前账号下有以下订单：ORD-1001（PAID），ORD-1002（COMPLETED）",
                "trace": ["tool:list_user_orders"],
                "need_handoff": False,
                "selected_order_id": None,
                "completed": True,
                "result_summary": "已向用户展示订单列表",
            }
        ),
    )

    llm = ScriptedChatModel(
        structured_responses=[
            {
                "action": "clarify",
                "reply": "请提供订单号，例如 ORD-1001。",
                "selected_order_id": None,
                "business_ready": False,
                "reason": "refund requires order id",
            },
            {
                "action": "delegate",
                "reply": "",
                "selected_order_id": None,
                "business_ready": True,
                "reason": "current user can query own orders",
            },
            {
                "next_agent": "order",
                "selected_order_id": None,
                "need_handoff": False,
                "reason": "must list orders before refund",
            },
        ]
    )
    config = {"configurable": {"thread_id": "conv-current-user-refund"}}
    context = {"llm": llm, "registry": None, "current_user_id": "U-3001"}

    asyncio.run(
        app.ainvoke(
            {"messages": [HumanMessage(content="帮我退单")]},
            config=config,
            context=context,
        )
    )
    result = asyncio.run(
        app.ainvoke(
            {"messages": [HumanMessage(content="我不知道，你看看我有没有订单详情")]},
            config=config,
            context=context,
        )
    )

    assert result["current_agent"] == "order"
    assert "ORD-1001" in result["final_reply"]
    assert "ORD-1002" in result["final_reply"]
    assert result["selected_order_id"] is None
    assert result["current_user_id"] == "U-3001"
    assert result["trace"] == ["coordinator:delegate", "route:order", "tool:list_user_orders"]
