"""Toolset registry for MCP server bootstrapping."""

from app.config import Settings, get_settings
from app.mcp_server.tools.activity import build_activity_tools
from app.mcp_server.tools.handoff import build_handoff_tools
from app.mcp_server.tools.order import build_order_tools
from app.mcp_server.tools.refund import build_refund_tools
from app.rpc.order_client import OrderRpcClient
from app.rpc.program_client import ProgramRpcClient

TOOLSETS = {
    "activity": build_activity_tools,
    "order": build_order_tools,
    "refund": build_refund_tools,
    "handoff": build_handoff_tools,
}


def build_toolset(toolset: str, *, settings: Settings | None = None, clients: dict | None = None):
    settings = settings or get_settings()
    clients = clients or {}

    if toolset == "activity":
        program_client = clients.get("program") or ProgramRpcClient(target=settings.program_rpc_target)
        return build_activity_tools(program_client)
    if toolset == "order":
        order_client = clients.get("order") or OrderRpcClient(target=settings.order_rpc_target)
        return build_order_tools(order_client)
    if toolset == "refund":
        order_client = clients.get("order") or OrderRpcClient(target=settings.order_rpc_target)
        return build_refund_tools(order_client)
    if toolset == "handoff":
        return build_handoff_tools()
    raise ValueError(f"unsupported toolset: {toolset}")
