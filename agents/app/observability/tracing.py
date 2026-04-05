"""Internal trace logging for orchestrator requests."""

from __future__ import annotations

from typing import Any

from app.session.store import StoredConversation


def build_trace_record(*, route_source: str, result: dict[str, Any], session: StoredConversation) -> dict[str, Any]:
    return {
        "route_source": route_source,
        "conversation_id": session.conversation_id,
        "user_id": session.user_id,
        "task_trace": result.get("task_trace", []),
        "selected_order_id": result.get("selected_order_id"),
        "need_handoff": result.get("need_handoff", False),
    }

