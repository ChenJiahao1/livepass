from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Literal, Mapping


class InterruptPayloadError(ValueError):
    """Raised when a human interrupt payload cannot be normalized."""


@dataclass(slots=True)
class HumanInterruptPayload:
    tool_name: Literal["human_approval", "human_input"]
    action: str
    args: dict[str, Any] = field(default_factory=dict)
    request: dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_raw(cls, payload: Mapping[str, Any]) -> "HumanInterruptPayload":
        tool_name = str(payload.get("toolName") or payload.get("tool_name") or "").strip()
        if tool_name not in {"human_approval", "human_input"}:
            raise InterruptPayloadError("unsupported human tool name")

        raw_args = payload.get("args")
        if raw_args is None:
            raw_args = payload.get("arguments")
        if raw_args is None:
            raw_args = {}
        if not isinstance(raw_args, Mapping):
            raise InterruptPayloadError("interrupt args must be an object")

        raw_request = payload.get("request")
        if raw_request is None:
            raw_request = _build_default_request(raw_args)
        if not isinstance(raw_request, Mapping):
            raise InterruptPayloadError("interrupt request must be an object")

        args = dict(raw_args)
        action = str(payload.get("action") or args.get("action") or "").strip()
        if not action:
            raise InterruptPayloadError("interrupt action is required")

        return cls(
            tool_name=tool_name,
            action=action,
            args=args,
            request=dict(raw_request),
        )


@dataclass(slots=True)
class HumanResumePayload:
    action: Literal["approve", "reject", "respond"]
    reason: str | None = None
    values: dict[str, Any] = field(default_factory=dict)


def _build_default_request(args: Mapping[str, Any]) -> dict[str, Any]:
    return {
        "title": str(args.get("title") or "人工确认").strip(),
        "description": str(args.get("description") or "请确认后继续执行。").strip(),
        "riskLevel": str(args.get("riskLevel") or args.get("risk_level") or "medium").strip(),
    }
