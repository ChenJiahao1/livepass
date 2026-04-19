"""Order specialist agent."""

from app.agents.base import BaseSpecialistAgent
from app.graph.state import ConversationState
from app.shared.runtime_constants import AGENT_ORDER


class OrderAgent(BaseSpecialistAgent):
    agent_name = AGENT_ORDER
    toolset = AGENT_ORDER
    prompt_template = "order_specialist"

    async def handle(self, state: ConversationState) -> dict[str, object]:
        return await self.run_tool_agent(state)
