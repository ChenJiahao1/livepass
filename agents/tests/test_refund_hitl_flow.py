import pytest
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.types import Command

from app.graph.builder import build_graph_app
from tests.fakes import ScriptedChatModel, StubRegistry, build_async_tool


def _build_refund_context(*, calls: list[str]):
    async def _preview_refund_order(order_id: str, user_id: int | None = None):
        calls.append("preview_refund_order")
        return {"order_id": order_id, "allow_refund": True, "refund_amount": "100", "refund_percent": 100}

    async def _refund_order(order_id: str, reason: str | None = None, user_id: int | None = None):
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
            {"next_agent": "finish", "selected_order_id": "ORD-1", "need_handoff": False, "reason": "old pseudo pause"},
        ]
    )
    return {"llm": llm, "registry": registry, "current_user_id": 1001}


async def _start_refund_flow(*, calls: list[str], thread_id: str = "conv-refund-hitl"):
    app = build_graph_app(checkpointer=InMemorySaver())
    config = {"configurable": {"thread_id": thread_id}}
    context = _build_refund_context(calls=calls)
    result = await app.ainvoke(
        {"messages": [{"role": "user", "content": "ORD-1 可以退款吗"}]},
        config=config,
        context=context,
    )
    return app, config, context, result


def _interrupt_payload(result: dict):
    interrupts = result.get("__interrupt__")
    assert interrupts
    return interrupts[0].value


@pytest.mark.anyio
async def test_refund_flow_interrupts_after_preview_before_side_effect():
    calls: list[str] = []

    _app, _config, _context, result = await _start_refund_flow(calls=calls)
    payload = _interrupt_payload(result)

    assert calls == ["preview_refund_order"]
    assert payload["toolName"] == "human_approval"
    assert payload["args"]["action"] == "refund_order"
    assert payload["request"]["title"] == "退款前确认"


@pytest.mark.anyio
async def test_refund_flow_approve_resumes_then_calls_refund_order():
    calls: list[str] = []
    app, config, context, result = await _start_refund_flow(calls=calls, thread_id="conv-refund-approve")
    _interrupt_payload(result)

    resumed = await app.ainvoke(
        Command(resume={"action": "approve", "reason": "同意退款", "values": {}}),
        config=config,
        context=context,
    )

    assert calls == ["preview_refund_order", "refund_order"]
    assert resumed["refund_result"]["accepted"] is True
    assert "已提交退款" in resumed["final_reply"]


@pytest.mark.anyio
async def test_refund_flow_reject_resumes_without_calling_refund_order():
    calls: list[str] = []
    app, config, context, result = await _start_refund_flow(calls=calls, thread_id="conv-refund-reject")
    _interrupt_payload(result)

    resumed = await app.ainvoke(
        Command(resume={"action": "reject", "reason": "暂不退款", "values": {}}),
        config=config,
        context=context,
    )

    assert calls == ["preview_refund_order"]
    assert resumed["refund_result"] is None
    assert resumed["final_reply"] == "已取消本次退款操作。"
