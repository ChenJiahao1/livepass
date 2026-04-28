from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass
from typing import Any, Literal


@dataclass(frozen=True)
class HITLToolPolicy:
    tool_name: str
    mode: Literal["approve_only", "preview_then_approve"]
    preview_tool_name: str | None = None
    title: str = "操作前确认"
    risk_level: str = "medium"
    allowed_actions: tuple[str, ...] = ("approve", "reject", "edit")
    description_builder: Callable[[dict[str, Any], dict[str, Any] | None], str] | None = None


def build_refund_approval_description(payload: dict[str, Any], preview: dict[str, Any] | None) -> str:
    order_id = str(payload.get("order_id") or payload.get("orderId") or "").strip()
    if not preview:
        return f"订单 {order_id} 将提交退款，请确认后继续。" if order_id else "将提交退款，请确认后继续。"
    amount = preview.get("refund_amount") or preview.get("refundAmount") or ""
    percent = preview.get("refund_percent") or preview.get("refundPercent") or ""
    if order_id and amount and percent:
        return f"订单 {order_id} 预计退款 {amount} 元，退款比例 {percent}%。确认后将提交退款。"
    if order_id and amount:
        return f"订单 {order_id} 预计退款 {amount} 元。确认后将提交退款。"
    return f"订单 {order_id} 将提交退款，请确认后继续。" if order_id else "将提交退款，请确认后继续。"


HITL_TOOL_POLICIES: dict[str, HITLToolPolicy] = {
    "refund_order": HITLToolPolicy(
        tool_name="refund_order",
        mode="preview_then_approve",
        preview_tool_name="preview_refund_order",
        title="退款前确认",
        description_builder=build_refund_approval_description,
    ),
}
