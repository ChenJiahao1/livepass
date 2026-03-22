"""Handoff specialist agent."""

from app.agents.base import ToolCallingAgent
from app.state import ConversationState


class HandoffAgent(ToolCallingAgent):
    agent_name = "handoff"
    toolset = "handoff"
    prompt_template = "handoff/system.md"

    async def handle(self, state: ConversationState) -> dict[str, object]:
        tools = await self.get_tools()
        request_handoff_tool = self.find_tool(tools, "request_handoff")
        reason = self.latest_user_message(state)
        if request_handoff_tool is not None:
            await request_handoff_tool.ainvoke({"reason": reason})
        return self.result(
            state,
            reply="已为你转接人工客服，请稍候。",
            need_handoff=True,
            status="handoff",
        )
