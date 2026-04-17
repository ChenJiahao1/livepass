from __future__ import annotations

from typing import Any, Mapping

from app.runs.tool_call_models import ToolCallRecord

TOOL_CALL_ALLOWED_ACTIONS = ("approve", "reject", "edit")


def normalize_allowed_actions(request: Mapping[str, Any]) -> list[str]:
    raw_actions = request.get("allowedActions")
    if raw_actions is None:
        raw_actions = request.get("allowed_actions")
    if not isinstance(raw_actions, (list, tuple)):
        return list(TOOL_CALL_ALLOWED_ACTIONS)
    normalized: list[str] = []
    for item in raw_actions:
        action = str(item or "").strip()
        if action in TOOL_CALL_ALLOWED_ACTIONS and action not in normalized:
            normalized.append(action)
    return normalized or list(TOOL_CALL_ALLOWED_ACTIONS)


def build_human_request(*, tool_name: str, request: Mapping[str, Any]) -> dict[str, Any]:
    return {
        "kind": "approval" if tool_name == "human_approval" else "input",
        "title": str(request.get("title") or "人工确认").strip(),
        "description": request.get("description"),
        "allowedActions": normalize_allowed_actions(request),
    }


def serialize_tool_call(
    tool_call: ToolCallRecord,
    *,
    include_context_fields: bool = True,
    include_result_fields: bool = True,
    include_metadata: bool = True,
    include_timestamps: bool = True,
) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "id": tool_call.id,
        "messageId": tool_call.message_id,
        "name": tool_call.name,
        "status": tool_call.status,
        "input": dict(tool_call.input),
        "humanRequest": dict(tool_call.human_request) if tool_call.human_request else None,
    }
    if include_context_fields:
        payload["runId"] = tool_call.run_id
        payload["threadId"] = tool_call.thread_id
    if include_result_fields:
        payload["output"] = dict(tool_call.output) if tool_call.output is not None else None
        payload["error"] = dict(tool_call.error) if tool_call.error is not None else None
    if include_metadata:
        payload["metadata"] = dict(tool_call.metadata)
    if include_timestamps:
        payload["createdAt"] = tool_call.created_at
        payload["updatedAt"] = tool_call.updated_at
        payload["completedAt"] = tool_call.completed_at
    return {key: value for key, value in payload.items() if value is not None}


def serialize_tool_call_event(tool_call: ToolCallRecord) -> dict[str, Any]:
    return serialize_tool_call(
        tool_call,
        include_context_fields=False,
        include_result_fields=True,
        include_metadata=False,
        include_timestamps=False,
    )
