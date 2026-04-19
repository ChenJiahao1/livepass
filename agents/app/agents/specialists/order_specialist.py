"""Order specialist agent."""

from app.agents.base import BaseSpecialistAgent
from app.graph.state import ConversationState


class OrderAgent(BaseSpecialistAgent):
    agent_name = "order"
    toolset = "order"
    prompt_template = "order_specialist"

    async def handle(self, state: ConversationState) -> dict[str, object]:
        return await self.run_tool_agent(state)
