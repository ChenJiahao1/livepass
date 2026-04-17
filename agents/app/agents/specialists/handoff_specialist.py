"""Handoff specialist agent."""

from langchain_core.messages import AIMessage

from app.agents.base import ToolCallingAgent
from app.graph.state import ConversationState


class HandoffAgent(ToolCallingAgent):
    agent_name = "handoff"
    prompt_template = "handoff_specialist"

    async def handle(self, state: ConversationState) -> dict[str, object]:
        reason = self.latest_user_message(state)
        reply = "已记录人工协助诉求，当前转人工链路尚未接入（TODO）。"
        if reason:
            reply = f"{reply} 诉求：{reason}"
        return self.result(
            state,
            reply=reply,
            need_handoff=True,
            status="handoff_todo",
            result_summary="人工协助待接入",
            messages=[AIMessage(content=reply)],
        )
