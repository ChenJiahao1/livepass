from __future__ import annotations

import asyncio
import json
from typing import Any

from app.common.errors import ApiError, ApiErrorCode
from app.mcp_client.execution_context import ToolExecutionContext


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
        next_payload = dict(payload)
        raw_meta = next_payload.get("_meta")
        meta = dict(raw_meta) if isinstance(raw_meta, dict) else {}
        meta.update(context.to_meta())
        next_payload["_meta"] = meta
        return next_payload

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
