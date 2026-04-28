from __future__ import annotations

from collections.abc import Mapping
from typing import Any

from langchain_core.tools import StructuredTool
from langgraph.types import interrupt

from app.runs.interrupt_models import HumanInterruptPayload


def build_human_approval_interrupt(
    *,
    action: str,
    args: dict[str, Any],
    request: dict[str, Any],
) -> HumanInterruptPayload:
    return HumanInterruptPayload(
        tool_name="human_approval",
        action=action,
        args={**args, "action": action},
        request=dict(request),
    )


def build_human_input_interrupt(
    *,
    action: str,
    args: dict[str, Any],
    request: dict[str, Any],
) -> HumanInterruptPayload:
    return HumanInterruptPayload(
        tool_name="human_input",
        action=action,
        args={**args, "action": action},
        request=dict(request),
    )


async def _human_input(
    action: str,
    title: str,
    description: str,
    values: dict[str, Any] | None = None,
    allowed_actions: list[str] | None = None,
) -> dict[str, Any]:
    payload = build_human_input_interrupt(
        action=action,
        args={"values": values or {}},
        request={
            "title": title,
            "description": description,
            "allowedActions": allowed_actions or ["edit", "reject"],
        },
    )
    decision = interrupt(
        {
            "toolName": payload.tool_name,
            "args": dict(payload.args),
            "request": dict(payload.request),
        }
    )
    return normalize_human_tool_decision(decision)


def normalize_human_tool_decision(decision: Any) -> dict[str, Any]:
    if not isinstance(decision, Mapping):
        return {"action": "reject", "reason": "invalid_resume_payload", "values": {}}
    action = str(decision.get("action") or "reject").strip()
    if action not in {"approve", "reject", "edit"}:
        action = "reject"
    values = decision.get("values")
    result = {
        "action": action,
        "values": dict(values) if isinstance(values, Mapping) else {},
    }
    reason = decision.get("reason")
    if reason is not None:
        result["reason"] = reason
    return result


def build_human_input_tool() -> StructuredTool:
    return StructuredTool.from_function(
        coroutine=_human_input,
        name="human_input",
        description="Ask the user for structured business input such as selecting an order.",
    )
