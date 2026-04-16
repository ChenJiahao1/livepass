"""Internal trace logging for orchestrator requests."""

from __future__ import annotations

from typing import Any

def build_trace_record(*, route_source: str, result: dict[str, Any], thread_id: str, user_id: int) -> dict[str, Any]:
    return {
        "route_source": route_source,
        "thread_id": thread_id,
        "user_id": user_id,
        "task_trace": result.get("task_trace", []),
        "selected_order_id": result.get("selected_order_id"),
        "need_handoff": result.get("need_handoff", False),
    }
