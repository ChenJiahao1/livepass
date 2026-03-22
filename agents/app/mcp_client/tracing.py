"""Tracing helpers for MCP tool calls."""

from __future__ import annotations

from typing import Any, Awaitable, Callable


async def trace_tool_calls(request, handler: Callable[[Any], Awaitable[Any]]) -> Any:
    trace = _resolve_trace_container(request.runtime)
    if trace is not None:
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
