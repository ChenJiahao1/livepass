"""Handoff specialist agent."""

from app.agents.base import ToolCallingAgent
from app.state import ConversationState


class HandoffAgent(ToolCallingAgent):
    agent_name = "handoff"
    toolset = "handoff"
    prompt_template = "handoff/system.md"

    async def handle(self, state: ConversationState) -> dict[str, object]:
        tools = await self.get_tools()
        reason = self.latest_user_message(state)
        request_handoff_tool = self.find_tool(tools, "request_handoff", "create_handoff_ticket")
        ticket_id = None
        if request_handoff_tool is not None:
            result = await request_handoff_tool.ainvoke({"reason": reason})
            ticket_id = result.get("ticket_id") or result.get("ticketId")
        reply = "已为你转接人工客服，请稍候。"
        if ticket_id:
            reply = f"{reply} 工单号 {ticket_id}。"
        return self.result(
            state,
            reply=reply,
            need_handoff=True,
            status="handoff",
            result_summary="已转人工",
        )
