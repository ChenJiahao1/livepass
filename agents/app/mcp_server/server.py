"""FastMCP server bootstrap."""

from __future__ import annotations

import argparse

from mcp.server.fastmcp import FastMCP

from app.mcp_server.toolsets import build_toolset


def build_server(toolset: str, *, clients: dict | None = None) -> FastMCP:
    server = FastMCP(name=f"damai-{toolset}-mcp")
    for tool in build_toolset(toolset, clients=clients):
        server.tool(name=tool.name, description=tool.description)(tool.coroutine)
    return server


def main() -> None:
    parser = argparse.ArgumentParser(description="Run damai MCP server")
    parser.add_argument("--toolset", required=True, choices=["activity", "order", "refund", "handoff"])
    args = parser.parse_args()
    build_server(args.toolset).run(transport="stdio")
