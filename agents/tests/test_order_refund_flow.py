import pytest
from langgraph.checkpoint.memory import InMemorySaver

from app.graph import build_graph_app
from tests.fakes import ScriptedChatModel, StubRegistry, build_async_tool


async def run_graph_turns(*, messages: list[str], registry, llm) -> dict:
    app = build_graph_app(checkpointer=InMemorySaver())
    config = {"configurable": {"thread_id": "conv-graph-flow"}}
    result = {}
    for message in messages:
        result = await app.ainvoke(
            {"messages": [{"role": "user", "content": message}]},
            config=config,
            context={"llm": llm, "registry": registry, "current_user_id": "1001"},
        )
    return result


@pytest.mark.anyio
async def test_graph_lists_orders_before_refund_submit():
    calls: list[str] = []

    async def _list_user_orders(identifier: str):
        calls.append("list_user_orders")
        return {"orders": [{"order_id": "ORD-1", "status": "PAID"}]}

    registry = StubRegistry(
        tools_by_toolset={
            "order": [
                build_async_tool(
                    name="list_user_orders",
                    description="list user orders",
                    coroutine=_list_user_orders,
                )
            ]
        }
    )
    llm = ScriptedChatModel(
        structured_responses=[
            {"action": "delegate", "reply": "", "selected_order_id": None, "business_ready": True, "reason": "refund"},
            {"next_agent": "order", "selected_order_id": None, "need_handoff": False, "reason": "list first"},
        ]
    )

    result = await run_graph_turns(messages=["我要退款"], registry=registry, llm=llm)

    assert "订单" in result["final_reply"]
    assert calls == ["list_user_orders"]


@pytest.mark.anyio
async def test_graph_refund_requires_preview_before_submit():
    calls: list[str] = []

    async def _preview_refund_order(order_id: str, user_id: str | None = None):
        calls.append("preview_refund_order")
        return {"order_id": order_id, "allow_refund": True, "refund_amount": "100", "refund_percent": 100}

    async def _refund_order(order_id: str, reason: str | None = None, user_id: str | None = None):
        calls.append("refund_order")
        return {"order_id": order_id, "accepted": True, "refund_amount": "100"}

    registry = StubRegistry(
        tools_by_toolset={
            "refund": [
                build_async_tool(
                    name="preview_refund_order",
                    description="preview refund",
                    coroutine=_preview_refund_order,
                ),
                build_async_tool(
                    name="refund_order",
                    description="submit refund",
                    coroutine=_refund_order,
                ),
            ]
        }
    )
    llm = ScriptedChatModel(
        structured_responses=[
            {"action": "delegate", "reply": "", "selected_order_id": "ORD-1", "business_ready": True, "reason": "preview"},
            {"next_agent": "refund", "selected_order_id": "ORD-1", "need_handoff": False, "reason": "preview"},
            {"action": "delegate", "reply": "", "selected_order_id": None, "business_ready": True, "reason": "submit"},
            {"next_agent": "refund", "selected_order_id": None, "need_handoff": False, "reason": "submit"},
        ]
    )

    await run_graph_turns(messages=["ORD-1 可以退款吗", "确认退款"], registry=registry, llm=llm)

    assert calls == ["preview_refund_order", "refund_order"]
