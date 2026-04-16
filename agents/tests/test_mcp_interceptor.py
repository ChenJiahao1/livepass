from __future__ import annotations

import asyncio

import pytest

from app.common.errors import ApiError
from app.mcp_client.execution_context import ToolExecutionContext
from app.mcp_client.interceptor import MCPToolInterceptor


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


def _context() -> ToolExecutionContext:
    return ToolExecutionContext(
        user_id="3001",
        thread_id="thr_001",
        run_id="run_001",
        tool_call_id="tool_001",
    )


@pytest.mark.anyio
async def test_interceptor_injects_runtime_meta_into_payload():
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
            "_meta": {
                "userId": "3001",
                "threadId": "thr_001",
                "runId": "run_001",
                "toolCallId": "tool_001",
            },
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
