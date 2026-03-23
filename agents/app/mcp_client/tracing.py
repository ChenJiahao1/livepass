"""Tracing helpers for MCP tool calls."""

from __future__ import annotations

from contextvars import ContextVar, Token
from typing import Any, Awaitable, Callable


_trace_buffer: ContextVar[list[str] | None] = ContextVar("mcp_trace_buffer", default=None)


def set_trace_buffer(buffer: list[str]) -> Token[list[str] | None]:
    return _trace_buffer.set(buffer)


def reset_trace_buffer(token: Token[list[str] | None]) -> None:
    _trace_buffer.reset(token)


def append_tool_trace(name: str) -> None:
    trace = _trace_buffer.get()
    if trace is not None:
        trace.append(f"tool:{name}")


async def trace_tool_calls(request, handler: Callable[[Any], Awaitable[Any]]) -> Any:
    append_tool_trace(request.name)
    trace = _resolve_trace_container(request.runtime)
    if trace is not None and trace is not _trace_buffer.get():
        trace.append(f"tool:{request.name}")
    return await handler(request)


def _resolve_trace_container(runtime: Any) -> list[str] | None:
    if runtime is None:
        return None
    if isinstance(runtime, dict):
        trace = runtime.get("trace")
        return trace if isinstance(trace, list) else None

    context = getattr(runtime, "context", None)
    if isinstance(context, dict):
        trace = context.get("trace")
        if isinstance(trace, list):
            return trace

    trace = getattr(runtime, "trace", None)
    return trace if isinstance(trace, list) else None
