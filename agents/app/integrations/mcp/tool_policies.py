from __future__ import annotations

from dataclasses import dataclass
from typing import Literal

from app.shared.runtime_constants import AGENT_ACTIVITY, AGENT_ORDER

TOOLSET_ACTIVITY = AGENT_ACTIVITY
TOOLSET_ORDER = AGENT_ORDER

SUPPORTED_TOOLSETS = (TOOLSET_ACTIVITY, TOOLSET_ORDER)
TOOLSET_TOOL_NAMES = {
    TOOLSET_ORDER: {
        "list_user_orders",
        "get_order_detail_for_service",
        "preview_refund_order",
        "refund_order",
    },
}


@dataclass(frozen=True)
class ToolAccessPolicy:
    mode: Literal["read", "write"]


TOOLSET_ACCESS_POLICIES = {
    TOOLSET_ORDER: {
        "list_user_orders": ToolAccessPolicy(mode="read"),
        "get_order_detail_for_service": ToolAccessPolicy(mode="read"),
        "preview_refund_order": ToolAccessPolicy(mode="read"),
        "refund_order": ToolAccessPolicy(mode="write"),
    }
}


def get_tool_access_policy(toolset: str, tool_name: str) -> ToolAccessPolicy | None:
    return TOOLSET_ACCESS_POLICIES.get(toolset, {}).get(tool_name)
