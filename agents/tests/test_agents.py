import pytest
from langchain_core.messages import HumanMessage

from app.agents.order import OrderAgent
from app.agents.refund import RefundAgent
from tests.fakes import StubRegistry, build_async_tool


@pytest.mark.anyio
async def test_order_agent_lists_user_orders_before_detail_lookup():
    received_payloads: list[dict] = []

    async def _list_user_orders(*, user_id: int):
        received_payloads.append({"user_id": user_id})
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
    result = await OrderAgent(registry=registry, llm=object()).handle(
        {"messages": [HumanMessage(content="帮我查订单")], "current_user_id": "1001"}
    )

    assert "订单" in result["reply"]
    assert received_payloads == [{"user_id": 1001}]


@pytest.mark.anyio
async def test_refund_agent_previews_order_before_submit():
    async def _preview_refund_order(order_id: str, user_id: int | None = None):
        return {"order_id": order_id, "allow_refund": True, "refund_amount": "100", "refund_percent": 100}

    registry = StubRegistry(
        tools_by_toolset={
            "refund": [
                build_async_tool(
                    name="preview_refund_order",
                    description="preview refund",
                    coroutine=_preview_refund_order,
                )
            ]
        }
    )
    result = await RefundAgent(registry=registry, llm=object()).handle(
        {"messages": [HumanMessage(content="ORD-1 可以退款吗")], "selected_order_id": "ORD-1"}
    )

    assert "可退款" in result["reply"]
