import asyncio

from langchain_core.messages import AIMessage, HumanMessage

from app.agents.activity import ActivityAgent
from app.agents.handoff import HandoffAgent
from app.agents.order import OrderAgent
from app.agents.refund import RefundAgent
from tests.fakes import (
    ScriptedChatModel,
    StubRegistry,
    build_async_tool,
    make_tool_call_message,
)


async def _search_programs(program_id: str):
    return {"program_id": program_id, "title": "周杰伦嘉年华", "show_time": "2026-03-28 19:30"}


async def _list_empty_orders(identifier: str):
    return {"orders": []}


async def _list_single_order(identifier: str):
    return {
        "orders": [
            {
                "order_id": "ORD-2001",
                "status": "paid",
                "payment_status": "paid",
                "ticket_status": "issued",
            }
        ]
    }


async def _list_many_orders(identifier: str):
    return {
        "orders": [
            {
                "order_id": "ORD-2001",
                "status": "paid",
                "payment_status": "paid",
                "ticket_status": "issued",
            },
            {
                "order_id": "ORD-2002",
                "status": "completed",
                "payment_status": "paid",
                "ticket_status": "verified",
            },
        ]
    }


async def _preview_refund_ok(order_id: str, user_id: str | None = None):
    return {
        "allow_refund": True,
        "refund_amount": "99.00",
        "refund_percent": 100,
    }


async def _preview_refund_blocked(order_id: str, user_id: str | None = None):
    return {
        "allow_refund": False,
        "reject_reason": "该订单当前不可退。",
    }


async def _request_handoff(reason: str):
    return {"accepted": True, "ticket_id": "HOF-1001", "reason": reason}


search_programs_tool = build_async_tool(
    name="search_programs",
    description="根据节目 ID 查询节目信息",
    coroutine=_search_programs,
)
list_empty_orders_tool = build_async_tool(
    name="list_user_orders",
    description="列出当前用户订单",
    coroutine=_list_empty_orders,
)
list_single_order_tool = build_async_tool(
    name="list_user_orders",
    description="列出当前用户订单",
    coroutine=_list_single_order,
)
list_many_orders_tool = build_async_tool(
    name="list_user_orders",
    description="列出当前用户订单",
    coroutine=_list_many_orders,
)
preview_refund_ok_tool = build_async_tool(
    name="preview_refund_order",
    description="预览退款资格",
    coroutine=_preview_refund_ok,
)
preview_refund_blocked_tool = build_async_tool(
    name="preview_refund_order",
    description="预览退款资格",
    coroutine=_preview_refund_blocked,
)
request_handoff_tool = build_async_tool(
    name="request_handoff",
    description="转接人工客服",
    coroutine=_request_handoff,
)


def test_activity_agent_injects_selected_program_id_into_prompt_and_trace():
    registry = StubRegistry(tools_by_toolset={"activity": [search_programs_tool]})
    llm = ScriptedChatModel(
        responses=[
            make_tool_call_message("search_programs", {"program_id": "PGM-2001"}),
            AIMessage(content="节目《周杰伦嘉年华》将于 2026-03-28 19:30 开演。"),
        ]
    )
    agent = ActivityAgent(registry=registry, llm=llm)

    result = asyncio.run(
        agent.handle(
            {
                "messages": [HumanMessage(content="看看这个节目")],
                "selected_program_id": "PGM-2001",
            }
        )
    )

    assert result["trace"] == ["program:PGM-2001", "tool:search_programs"]
    assert result["completed"] is True
    assert "PGM-2001" in llm.calls[0][0].content


def test_order_agent_reports_when_current_user_has_no_orders():
    registry = StubRegistry(tools_by_toolset={"order": [list_empty_orders_tool]})
    agent = OrderAgent(registry=registry, llm=ScriptedChatModel(responses=[]))

    result = asyncio.run(
        agent.handle(
            {
                "messages": [HumanMessage(content="你看看我有没有订单详情")],
                "current_user_id": "U-3001",
                "selected_order_id": None,
            }
        )
    )

    assert result["result_summary"] == "当前账号无订单"
    assert result["selected_order_id"] is None
    assert result["completed"] is True


def test_order_agent_auto_selects_single_order_for_current_user():
    registry = StubRegistry(tools_by_toolset={"order": [list_single_order_tool]})
    agent = OrderAgent(registry=registry, llm=ScriptedChatModel(responses=[]))

    result = asyncio.run(
        agent.handle(
            {
                "messages": [HumanMessage(content="你看看我有没有订单详情")],
                "current_user_id": "U-3001",
                "selected_order_id": None,
            }
        )
    )

    assert "ORD-2001" in result["reply"]
    assert result["selected_order_id"] == "ORD-2001"
    assert result["completed"] is True


def test_order_agent_lists_multiple_orders_without_auto_selecting():
    registry = StubRegistry(tools_by_toolset={"order": [list_many_orders_tool]})
    agent = OrderAgent(registry=registry, llm=ScriptedChatModel(responses=[]))

    result = asyncio.run(
        agent.handle(
            {
                "messages": [HumanMessage(content="你看看我有没有订单详情")],
                "current_user_id": "U-3001",
                "selected_order_id": None,
            }
        )
    )

    assert result["result_summary"] == "已向用户展示订单列表"
    assert result["selected_order_id"] is None
    assert result["completed"] is True


def test_refund_agent_returns_preview_for_refundable_order():
    registry = StubRegistry(tools_by_toolset={"refund": [preview_refund_ok_tool]})
    agent = RefundAgent(registry=registry, llm=ScriptedChatModel(responses=[]))

    result = asyncio.run(
        agent.handle(
            {
                "messages": [HumanMessage(content="订单 ORD-1001 可以退款吗")],
                "selected_order_id": "ORD-1001",
            }
        )
    )

    assert result["selected_order_id"] == "ORD-1001"
    assert result["completed"] is True
    assert "预计退款" in result["reply"]
    assert result["result_summary"] == "退款资格已确认"


def test_refund_agent_returns_reject_reason_for_blocked_order():
    registry = StubRegistry(tools_by_toolset={"refund": [preview_refund_blocked_tool]})
    agent = RefundAgent(registry=registry, llm=ScriptedChatModel(responses=[]))

    result = asyncio.run(
        agent.handle(
            {
                "messages": [HumanMessage(content="订单 ORD-1002 可以退款吗")],
                "selected_order_id": "ORD-1002",
            }
        )
    )

    assert result["reply"] == "该订单当前不可退。"
    assert result["completed"] is True
    assert result["result_summary"] == "退款被拒绝"


def test_handoff_agent_sets_need_handoff_and_tracks_tool_trace():
    registry = StubRegistry(tools_by_toolset={"handoff": [request_handoff_tool]})
    llm = ScriptedChatModel(
        responses=[
            make_tool_call_message("request_handoff", {"reason": "转人工"}),
            AIMessage(content="已为你转人工，接管单号 HOF-1001。"),
        ]
    )
    agent = HandoffAgent(registry=registry, llm=llm)

    result = asyncio.run(
        agent.handle(
            {
                "messages": [HumanMessage(content="转人工")],
                "last_intent": "refund",
                "selected_order_id": "ORD-1001",
            }
        )
    )

    assert result["need_handoff"] is True
    assert result["completed"] is True
    assert result["trace"] == ["tool:request_handoff"]
    assert "selected_order_id" in llm.calls[0][0].content
