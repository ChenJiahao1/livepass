from __future__ import annotations

import asyncio
import json
from collections.abc import Callable
from dataclasses import dataclass
from typing import Any, Mapping

from langgraph.types import interrupt

from app.agents.tools.human_tools import build_human_approval_interrupt
from app.shared.errors import ApiError, ApiErrorCode
from app.integrations.mcp.execution_context import ToolExecutionContext
from app.integrations.mcp.tool_policies import get_tool_access_policy


@dataclass(frozen=True)
class ToolExecutionPolicy:
    requires_hitl: bool = False
    title: str | None = None
    risk_level: str = "medium"
    description_builder: Callable[[Mapping[str, Any]], str] | None = None


def build_refund_description(payload: Mapping[str, Any]) -> str:
    order_id = str(payload.get("order_id") or payload.get("orderId") or "").strip()
    if order_id:
        return f"订单 {order_id} 将提交退款，请确认后继续。"
    return "将提交退款，请确认后继续。"


DEFAULT_TOOL_EXECUTION_POLICY = ToolExecutionPolicy()
DEFAULT_WRITE_TOOL_EXECUTION_POLICY = ToolExecutionPolicy(
    requires_hitl=True,
    title="写操作前确认",
    risk_level="medium",
)
TOOL_EXECUTION_POLICIES = {
    "refund_order": ToolExecutionPolicy(
        requires_hitl=True,
        title="退款前确认",
        risk_level="medium",
        description_builder=build_refund_description,
    ),
}


def get_tool_execution_policy(toolset: str, tool_name: str) -> ToolExecutionPolicy:
    policy = TOOL_EXECUTION_POLICIES.get(tool_name)
    if policy is not None:
        return policy

    access_policy = get_tool_access_policy(toolset, tool_name)
    if access_policy is not None and access_policy.mode == "write":
        return DEFAULT_WRITE_TOOL_EXECUTION_POLICY
    return DEFAULT_TOOL_EXECUTION_POLICY


class MCPToolInterceptor:
    def __init__(self, *, timeout_seconds: float | None = None) -> None:
        self.timeout_seconds = timeout_seconds

    async def invoke(
        self,
        *,
        server_name: str,
        tool_name: str,
        payload: dict[str, Any],
        context: ToolExecutionContext,
        tool: Any | None,
    ) -> Any:
        if tool is None:
            raise ApiError(
                code=ApiErrorCode.MCP_TOOL_NOT_FOUND,
                message="MCP 工具不存在",
                http_status=502,
                details={"serverName": server_name, "toolName": tool_name},
            )
        request_payload = self._inject_context(payload=payload, context=context)
        request_payload = self._apply_human_interrupt_if_required(
            toolset=server_name,
            tool_name=tool_name,
            payload=request_payload,
        )
        if request_payload.pop("__cancelled_by_human__", False):
            return request_payload
        try:
            coroutine = tool.ainvoke(request_payload)
            if self.timeout_seconds is not None:
                result = await asyncio.wait_for(coroutine, timeout=self.timeout_seconds)
            else:
                result = await coroutine
        except asyncio.TimeoutError as exc:
            raise ApiError(
                code=ApiErrorCode.MCP_TIMEOUT,
                message="MCP 工具执行超时",
                http_status=504,
                details={"serverName": server_name, "toolName": tool_name},
            ) from exc
        except ApiError:
            raise
        except Exception as exc:
            if self._is_unavailable_error(exc):
                raise ApiError(
                    code=ApiErrorCode.MCP_UNAVAILABLE,
                    message="MCP 服务不可用",
                    http_status=503,
                    details={"serverName": server_name, "toolName": tool_name},
                ) from exc
            raise ApiError(
                code=ApiErrorCode.MCP_EXECUTION_ERROR,
                message="MCP 工具执行失败",
                http_status=502,
                details={
                    "serverName": server_name,
                    "toolName": tool_name,
                    "reason": str(exc),
                },
            ) from exc
        return self._normalize_result(result=result, server_name=server_name, tool_name=tool_name)

    def _inject_context(self, *, payload: dict[str, Any], context: ToolExecutionContext) -> dict[str, Any]:
        del context
        return dict(payload)

    def _apply_human_interrupt_if_required(
        self,
        *,
        toolset: str,
        tool_name: str,
        payload: dict[str, Any],
    ) -> dict[str, Any]:
        policy = get_tool_execution_policy(toolset, tool_name)
        if not policy.requires_hitl:
            return payload

        description = (
            policy.description_builder(payload)
            if policy.description_builder
            else "该工具会执行写操作，请确认后继续。"
        )
        interrupt_payload = build_human_approval_interrupt(
            action=tool_name,
            args={
                "orderId": str(payload.get("order_id") or payload.get("orderId") or ""),
                "values": dict(payload),
            },
            request={
                "title": policy.title or "操作前确认",
                "description": description,
                "riskLevel": policy.risk_level,
                "allowedActions": ["approve", "reject", "edit"],
            },
        )
        decision = interrupt(
            {
                "toolName": interrupt_payload.tool_name,
                "args": dict(interrupt_payload.args),
                "request": dict(interrupt_payload.request),
            }
        )
        return self._payload_after_human_decision(original=payload, decision=decision)

    def _payload_after_human_decision(self, *, original: dict[str, Any], decision: Any) -> dict[str, Any]:
        if not isinstance(decision, Mapping):
            return self._cancelled_payload(reason="invalid_resume_payload")
        if isinstance(decision.get("decisions"), list) and decision["decisions"]:
            first = decision["decisions"][0]
            if not isinstance(first, Mapping):
                return self._cancelled_payload(reason="invalid_resume_payload")
            decision_type = str(first.get("type") or "").strip()
            if decision_type == "approve":
                return original
            if decision_type == "edit":
                edited_action = first.get("edited_action")
                if not isinstance(edited_action, Mapping):
                    return self._cancelled_payload(reason="invalid_resume_payload")
                args = edited_action.get("args")
                return dict(args) if isinstance(args, Mapping) else {}
            return self._cancelled_payload(reason=str(first.get("message") or "human_rejected"))

        action = str(decision.get("action") or "").strip()
        if action == "approve":
            return original
        if action == "edit":
            values = decision.get("values")
            return dict(values) if isinstance(values, Mapping) else {}
        return self._cancelled_payload(reason=str(decision.get("reason") or "human_rejected"))

    def _cancelled_payload(self, *, reason: str) -> dict[str, Any]:
        return {
            "__cancelled_by_human__": True,
            "cancelled": True,
            "reason": reason,
        }

    def _normalize_result(self, *, result: Any, server_name: str, tool_name: str) -> Any:
        normalized = self._coerce_result(result)
        if normalized is None:
            raise ApiError(
                code=ApiErrorCode.MCP_BAD_RESPONSE,
                message="MCP 返回了无法识别的响应",
                http_status=502,
                details={"serverName": server_name, "toolName": tool_name},
            )
        return normalized

    def _coerce_result(self, result: Any) -> Any | None:
        if isinstance(result, dict):
            return result
        if isinstance(result, str):
            try:
                return json.loads(result)
            except json.JSONDecodeError:
                return result
        if isinstance(result, list):
            text_parts: list[str] = []
            for item in result:
                if isinstance(item, dict) and item.get("type") == "text":
                    text_parts.append(str(item.get("text", "")))
            if text_parts:
                joined = "\n".join(text_parts)
                try:
                    return json.loads(joined)
                except json.JSONDecodeError:
                    return joined
            return result
        return None

    def _is_unavailable_error(self, exc: Exception) -> bool:
        exc_type = type(exc).__name__.lower()
        if "connect" in exc_type or "unavailable" in exc_type:
            return True
        message = str(exc).lower()
        return "connection refused" in message or "name or service not known" in message
