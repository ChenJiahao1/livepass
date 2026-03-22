"""Handoff specialist."""

from app.orchestrator.agents.base import BaseAgent


class HandoffAgent(BaseAgent):
    async def handle(self, *, message: str):
        await self.tool("request_handoff").ainvoke({"reason": message})
        return self.result(
            reply="已为你转接人工客服，请稍候。",
            current_agent="handoff",
            need_handoff=True,
            status="handoff",
        )
