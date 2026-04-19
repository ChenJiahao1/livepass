import pytest
from langchain_core.messages import HumanMessage

from app.agents.specialists.order_specialist import OrderAgent
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
        {"messages": [HumanMessage(content="帮我查订单")], "current_user_id": 1001}
    )

    assert "订单" in result["reply"]
    assert received_payloads == [{"user_id": 1001}]


@pytest.mark.anyio
async def test_order_agent_handles_refund_preview_and_submit_tools():
    calls: list[str] = []

    async def _preview_refund_order(order_id: str, user_id: int | None = None):
        calls.append("preview_refund_order")
        return {"order_id": order_id, "allow_refund": True, "refund_amount": "100", "refund_percent": 100}

    async def _refund_order(order_id: str, reason: str, user_id: int | None = None):
        calls.append("refund_order")
        return {"order_id": order_id, "accepted": True, "refund_amount": "100"}

    registry = StubRegistry(
        tools_by_toolset={
            "order": [
                build_async_tool(
                    name="preview_refund_order",
                    description="preview refund",
                    coroutine=_preview_refund_order,
                ),
                build_async_tool(
                    name="refund_order",
                    description="submit refund",
                    coroutine=_refund_order,
                )
            ]
        }
    )
    result = await OrderAgent(registry=registry, llm=object()).handle(
        {"messages": [HumanMessage(content="ORD-1 可以退款吗")], "selected_order_id": "ORD-1", "current_user_id": 1001}
    )

    assert calls == ["preview_refund_order", "refund_order"]
    assert "已提交退款" in result["reply"]
