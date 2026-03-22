"""Activity specialist."""

from app.orchestrator.agents.base import BaseAgent


class ActivityAgent(BaseAgent):
    async def handle(self, *, message: str):
        program_id = self.extract_order_number(message)
        if program_id:
            detail = await self.tool("get_program_detail").ainvoke({"program_id": program_id})
            return self.result(
                reply=f"活动《{detail['title']}》演出时间是 {detail['showTime']}。",
                current_agent="activity",
            )

        programs = await self.tool("page_programs").ainvoke({})
        first = programs["list"][0]
        return self.result(
            reply=f"当前可关注活动有《{first['title']}》，演出时间 {first['showTime']}。",
            current_agent="activity",
        )
