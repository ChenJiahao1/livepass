"""Activity specialist agent."""

from app.agents.base import ToolCallingAgent
from app.state import ConversationState


class ActivityAgent(ToolCallingAgent):
    agent_name = "activity"
    toolset = "activity"
    prompt_template = "activity/system.md"

    async def handle(self, state: ConversationState) -> dict[str, object]:
        tools = await self.get_tools()
        program_id = state.get("selected_program_id")

        if program_id:
            detail_tool = self.find_tool(tools, "get_program_detail")
            if detail_tool is not None:
                detail = await detail_tool.ainvoke({"program_id": program_id})
                title = detail.get("title", str(program_id))
                show_time = detail.get("show_time") or detail.get("showTime", "待定")
                return self.result(
                    state,
                    reply=f"节目《{title}》的演出时间是 {show_time}。",
                    specialist_result=detail,
                    result_summary="节目详情已返回",
                )

        page_tool = self.find_tool(tools, "page_programs")
        if page_tool is not None:
            programs = await page_tool.ainvoke({})
            items = programs.get("list") or programs.get("programs") or []
            if items:
                first = items[0]
                title = first.get("title", "待定节目")
                show_time = first.get("show_time") or first.get("showTime", "待定")
                return self.result(
                    state,
                    reply=f"当前可关注节目有《{title}》，演出时间 {show_time}。",
                    specialist_result={"programs": items},
                    result_summary="节目列表已返回",
                )

        return await self.run_tool_agent(state)
