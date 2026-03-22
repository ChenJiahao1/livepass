import asyncio

from app.agents.handoff import HandoffAgent
from app.agents.order import OrderAgent
from app.agents.refund import RefundAgent
from tests.fakes import ScriptedChatModel, StubRegistry, build_async_tool


async def _list_user_orders(identifier: str):
    return {
        "orders": [
            {"order_id": "ORD-1001", "status": "PAID"},
            {"order_id": "ORD-1002", "status": "REFUNDING"},
        ]
    }


async def _request_handoff(reason: str):
    return {"accepted": True, "reason": reason}


list_user_orders_tool = build_async_tool(
    name="list_user_orders",
    description="列出当前用户的订单",
    coroutine=_list_user_orders,
)
request_handoff_tool = build_async_tool(
    name="request_handoff",
    description="转接人工客服",
    coroutine=_request_handoff,
)


def test_order_agent_lists_current_user_orders_when_order_id_missing():
    registry = StubRegistry(tools_by_toolset={"order": [list_user_orders_tool]})
    agent = OrderAgent(registry=registry, llm=ScriptedChatModel(responses=[]))

    result = asyncio.run(agent.handle({"messages": [], "current_user_id": "U-3001"}))

    assert "当前账号下有以下订单" in result["reply"]


def test_refund_agent_requires_order_id_when_missing():
    agent = RefundAgent(registry=StubRegistry(), llm=ScriptedChatModel(responses=[]))

    result = asyncio.run(agent.handle({"messages": [{"role": "user", "content": "我想退款"}]}))

    assert "请先提供需要处理的订单号" in result["reply"]


def test_handoff_agent_marks_need_handoff():
    registry = StubRegistry(tools_by_toolset={"handoff": [request_handoff_tool]})
    agent = HandoffAgent(registry=registry, llm=ScriptedChatModel(responses=[]))

    result = asyncio.run(agent.handle({"messages": [{"role": "user", "content": "转人工"}]}))

    assert result["need_handoff"] is True
    assert result["status"] == "handoff"
