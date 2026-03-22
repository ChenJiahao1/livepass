"""MCP tools for manual handoff."""

from langchain_core.tools import StructuredTool


def build_request_handoff_tool():
    async def request_handoff(reason: str):
        return {"queued": True, "reason": reason}

    return StructuredTool.from_function(
        coroutine=request_handoff,
        name="request_handoff",
        description="发起人工客服转接",
    )


def build_handoff_tools():
    return [build_request_handoff_tool()]
