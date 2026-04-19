from __future__ import annotations

import asyncio

import pytest
from langgraph.checkpoint.memory import InMemorySaver
from langgraph.graph import END, START, StateGraph
from langgraph.types import Command

from app.shared.errors import ApiError
from app.integrations.mcp.execution_context import ToolExecutionContext
from app.integrations.mcp.interceptor import MCPToolInterceptor


class _TimeoutTool:
    name = "preview_refund_order"
    description = "preview refund"

    async def ainvoke(self, payload: dict):
        raise asyncio.TimeoutError


class _ExecutionErrorTool:
    name = "preview_refund_order"
    description = "preview refund"

    async def ainvoke(self, payload: dict):
        raise RuntimeError("downstream exploded")


class _BadResponseTool:
    name = "preview_refund_order"
    description = "preview refund"

    async def ainvoke(self, payload: dict):
        return object()


class _SuccessTool:
    name = "preview_refund_order"
    description = "preview refund"

    def __init__(self) -> None:
        self.calls: list[dict] = []

    async def ainvoke(self, payload: dict):
        self.calls.append(dict(payload))
        return {"ok": True, "payload": payload}


class _RefundTool:
    name = "refund_order"
    description = "submit refund"

    def __init__(self) -> None:
        self.calls: list[dict] = []

    async def ainvoke(self, payload: dict):
        self.calls.append(dict(payload))
        return {"accepted": True, "payload": payload}


def _context() -> ToolExecutionContext:
    return ToolExecutionContext(
        user_id=3001,
        thread_id="thr_001",
        run_id="run_001",
        tool_call_id="tool_001",
    )


@pytest.mark.anyio
async def test_interceptor_passes_payload_without_injecting_runtime_meta():
    tool = _SuccessTool()
    interceptor = MCPToolInterceptor()

    result = await interceptor.invoke(
        server_name="order",
        tool_name="preview_refund_order",
        payload={"order_id": "ORD-10001"},
        context=_context(),
        tool=tool,
    )

    assert result == {
        "ok": True,
        "payload": {
            "order_id": "ORD-10001",
        },
    }
    assert tool.calls == [result["payload"]]


@pytest.mark.anyio
async def test_interceptor_maps_timeout_to_stable_api_error():
    interceptor = MCPToolInterceptor()

    with pytest.raises(ApiError) as exc_info:
        await interceptor.invoke(
            server_name="order",
            tool_name="preview_refund_order",
            payload={"order_id": "ORD-10001"},
            context=_context(),
            tool=_TimeoutTool(),
        )

    assert exc_info.value.code == "MCP_TIMEOUT"


@pytest.mark.anyio
async def test_interceptor_maps_missing_tool_to_stable_api_error():
    interceptor = MCPToolInterceptor()

    with pytest.raises(ApiError) as exc_info:
        await interceptor.invoke(
            server_name="order",
            tool_name="preview_refund_order",
            payload={"order_id": "ORD-10001"},
            context=_context(),
            tool=None,
        )

    assert exc_info.value.code == "MCP_TOOL_NOT_FOUND"


@pytest.mark.anyio
async def test_interceptor_maps_bad_response_to_stable_api_error():
    interceptor = MCPToolInterceptor()

    with pytest.raises(ApiError) as exc_info:
        await interceptor.invoke(
            server_name="order",
            tool_name="preview_refund_order",
            payload={"order_id": "ORD-10001"},
            context=_context(),
            tool=_BadResponseTool(),
        )

    assert exc_info.value.code == "MCP_BAD_RESPONSE"


@pytest.mark.anyio
async def test_interceptor_maps_execution_error_to_stable_api_error():
    interceptor = MCPToolInterceptor()

    with pytest.raises(ApiError) as exc_info:
        await interceptor.invoke(
            server_name="order",
            tool_name="preview_refund_order",
            payload={"order_id": "ORD-10001"},
            context=_context(),
            tool=_ExecutionErrorTool(),
        )

    assert exc_info.value.code == "MCP_EXECUTION_ERROR"


@pytest.mark.anyio
async def test_interceptor_interrupts_refund_order_before_side_effect_and_resumes_on_approve():
    tool = _RefundTool()
    interceptor = MCPToolInterceptor()

    async def _node(_state: dict):
        result = await interceptor.invoke(
            server_name="order",
            tool_name="refund_order",
            payload={"order_id": "ORD-10001", "reason": "用户发起退款"},
            context=_context(),
            tool=tool,
        )
        return {"result": result}

    builder = StateGraph(dict)
    builder.add_node("submit_refund", _node)
    builder.add_edge(START, "submit_refund")
    builder.add_edge("submit_refund", END)
    graph = builder.compile(checkpointer=InMemorySaver())
    config = {"configurable": {"thread_id": "hitl-refund-order"}}

    interrupted = await graph.ainvoke({}, config=config)
    payload = interrupted["__interrupt__"][0].value

    assert tool.calls == []
    assert payload["toolName"] == "human_approval"
    assert payload["args"]["action"] == "refund_order"
    assert payload["args"]["values"] == {"order_id": "ORD-10001", "reason": "用户发起退款"}
    assert payload["request"]["allowedActions"] == ["approve", "reject", "edit"]

    resumed = await graph.ainvoke(Command(resume={"decisions": [{"type": "approve"}]}), config=config)

    assert tool.calls == [{"order_id": "ORD-10001", "reason": "用户发起退款"}]
    assert resumed["result"]["accepted"] is True
