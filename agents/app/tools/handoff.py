"""Handoff tools."""

from langchain_core.tools import StructuredTool


def build_handoff_tools():
    async def request_handoff(reason: str):
        return {"accepted": True, "reason": reason}

    return [
        StructuredTool.from_function(
            coroutine=request_handoff,
            name="request_handoff",
            description="创建人工客服接入请求",
        )
    ]
