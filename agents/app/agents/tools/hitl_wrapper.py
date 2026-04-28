from __future__ import annotations

from collections.abc import Awaitable, Callable, Mapping
from typing import Any

from langgraph.types import interrupt

from app.agents.tools.hitl_policies import HITL_TOOL_POLICIES, HITLToolPolicy
from app.agents.tools.human_tools import build_human_approval_interrupt


class HITLWrappedTool:
    def __init__(
        self,
        *,
        original_tool: Any,
        policy: HITLToolPolicy,
        invoke_tool: Callable[[str, dict[str, Any]], Awaitable[Any]],
    ) -> None:
        self._original_tool = original_tool
        self._policy = policy
        self._invoke_tool = invoke_tool
        self.name = policy.tool_name
        self.description = getattr(original_tool, "description", self.name)
        self.args_schema = getattr(original_tool, "args_schema", None)
        if hasattr(original_tool, "__signature__"):
            self.__signature__ = getattr(original_tool, "__signature__")

    async def ainvoke(self, payload: dict[str, Any]) -> Any:
        if self._policy.mode == "preview_then_approve":
            return await self._preview_then_approve(dict(payload))
        return await self._approve_only(dict(payload))

    async def _preview_then_approve(self, payload: dict[str, Any]) -> Any:
        if not self._policy.preview_tool_name:
            raise ValueError("preview_tool_name is required")
        preview = await self._invoke_tool(self._policy.preview_tool_name, payload)
        if isinstance(preview, Mapping) and preview.get("allow_refund") is False:
            return dict(preview)
        normalized_preview = dict(preview) if isinstance(preview, Mapping) else None
        decision = self._interrupt_for_approval(payload=payload, preview=normalized_preview)
        action, next_payload = normalize_hitl_decision(original=payload, decision=decision)
        if action == "approve":
            return await self._invoke_tool(self._policy.tool_name, next_payload)
        if action == "edit":
            return await self._preview_then_approve(next_payload)
        return {"cancelled": True, "reason": next_payload.get("reason", "human_rejected")}

    async def _approve_only(self, payload: dict[str, Any]) -> Any:
        decision = self._interrupt_for_approval(payload=payload, preview=None)
        action, next_payload = normalize_hitl_decision(original=payload, decision=decision)
        if action == "approve":
            return await self._invoke_tool(self._policy.tool_name, next_payload)
        if action == "edit":
            return await self._approve_only(next_payload)
        return {"cancelled": True, "reason": next_payload.get("reason", "human_rejected")}

    def _interrupt_for_approval(self, *, payload: dict[str, Any], preview: dict[str, Any] | None) -> Any:
        if self._policy.description_builder:
            description = self._policy.description_builder(payload, preview)
        else:
            description = "该工具会执行写操作，请确认后继续。"
        interrupt_payload = build_human_approval_interrupt(
            action=self._policy.tool_name,
            args={"values": dict(payload)},
            request={
                "title": self._policy.title,
                "description": description,
                "riskLevel": self._policy.risk_level,
                "allowedActions": list(self._policy.allowed_actions),
            },
        )
        return interrupt(
            {
                "toolName": interrupt_payload.tool_name,
                "args": dict(interrupt_payload.args),
                "request": dict(interrupt_payload.request),
            }
        )


def normalize_hitl_decision(*, original: dict[str, Any], decision: Any) -> tuple[str, dict[str, Any]]:
    if not isinstance(decision, Mapping):
        return "reject", {"reason": "invalid_resume_payload"}
    if isinstance(decision.get("decisions"), list) and decision["decisions"]:
        return _normalize_legacy_decision(original=original, decision=decision["decisions"][0])

    action = str(decision.get("action") or "").strip()
    if action == "approve":
        return "approve", dict(original)
    if action == "edit":
        values = decision.get("values")
        if isinstance(values, Mapping):
            return "edit", dict(values)
        return "reject", {"reason": "invalid_resume_payload"}
    return "reject", {"reason": str(decision.get("reason") or "human_rejected")}


def _normalize_legacy_decision(*, original: dict[str, Any], decision: Any) -> tuple[str, dict[str, Any]]:
    if not isinstance(decision, Mapping):
        return "reject", {"reason": "invalid_resume_payload"}
    decision_type = str(decision.get("type") or "").strip()
    if decision_type == "approve":
        return "approve", dict(original)
    if decision_type == "edit":
        edited_action = decision.get("edited_action")
        if not isinstance(edited_action, Mapping):
            return "reject", {"reason": "invalid_resume_payload"}
        args = edited_action.get("args")
        if isinstance(args, Mapping):
            return "edit", dict(args)
        return "reject", {"reason": "invalid_resume_payload"}
    return "reject", {"reason": str(decision.get("message") or "human_rejected")}


def wrap_tool_with_hitl(
    *,
    tool: Any,
    policy: HITLToolPolicy,
    invoke_tool: Callable[[str, dict[str, Any]], Awaitable[Any]],
) -> HITLWrappedTool:
    return HITLWrappedTool(original_tool=tool, policy=policy, invoke_tool=invoke_tool)


def wrap_tools_with_hitl_policies(
    *,
    tools: list[Any],
    invoke_tool: Callable[[str, dict[str, Any]], Awaitable[Any]],
) -> list[Any]:
    wrapped: list[Any] = []
    for tool in tools:
        policy = HITL_TOOL_POLICIES.get(getattr(tool, "name", ""))
        if policy is None:
            wrapped.append(tool)
            continue
        wrapped.append(wrap_tool_with_hitl(tool=tool, policy=policy, invoke_tool=invoke_tool))
    return wrapped
