"""Handoff specialist agent."""

from app.agents.base import ToolCallingAgent
from app.state import ConversationState


class HandoffAgent(ToolCallingAgent):
    agent_name = "handoff"
    toolset = "handoff"
    prompt_template = "handoff/system.md"

    def build_prompt_context(self, state: ConversationState) -> dict[str, object]:
        context = super().build_prompt_context(state)
        context["last_intent"] = state.get("last_intent", "unknown")
        context["selected_order_id"] = state.get("selected_order_id")
        return context

    def default_need_handoff(self, state: ConversationState) -> bool:
        return True

    async def handle(self, state: ConversationState) -> dict[str, object]:
        if self.llm is not None:
            result = await super().handle(state)
            result["status"] = "handoff"
            return result

        tools = await self.get_tools()
        request_handoff_tool = self.find_tool(tools, "request_handoff")
        reason = self.latest_user_message(state)
        trace: list[str] = []
        if request_handoff_tool is not None:
            await request_handoff_tool.ainvoke({"reason": reason})
            trace.append("tool:request_handoff")
        return self.result(
            state,
            reply="已为你转接人工客服，请稍候。",
            trace=trace,
            need_handoff=True,
            status="handoff",
            result_summary="人工工单已创建",
        )
