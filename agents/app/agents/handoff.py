"""Handoff specialist agent."""

from app.agents.base import ToolCallingAgent
from app.state import ConversationState


class HandoffAgent(ToolCallingAgent):
    agent_name = "handoff"
    toolset = "handoff"

    async def handle(self, state: ConversationState) -> dict[str, object]:
        if self.llm is not None:
            result = await self.execute_skill(
                state,
                skill_id="handoff.create_ticket",
                task_type="handoff_create_ticket",
                goal="创建人工工单",
                required_slots=[],
                fallback_policy="handoff",
                expected_output_schema="handoff_ticket_v1",
                input_slots={"user_id": state.get("current_user_id")},
            )
            return self.result(
                state,
                reply="已为你转接人工客服，请稍候。",
                trace=[f"tool:{name}" for name in result.get("tool_calls", [])],
                need_handoff=True,
                status="handoff",
                result_summary="人工工单已创建",
            )

        tools = await self.get_tools()
        request_handoff_tool = self.find_tool(tools, "create_handoff_ticket", "request_handoff")
        reason = self.latest_user_message(state)
        trace: list[str] = []
        if request_handoff_tool is not None:
            await request_handoff_tool.ainvoke({"reason": reason})
            trace.append(f"tool:{request_handoff_tool.name}")
        return self.result(
            state,
            reply="已为你转接人工客服，请稍候。",
            trace=trace,
            need_handoff=True,
            status="handoff",
            result_summary="人工工单已创建",
        )
