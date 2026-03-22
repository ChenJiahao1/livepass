"""Deterministic graph orchestrator for the first delivery."""

from app.clients.rpc.order_client import OrderRpcClient
from app.clients.rpc.program_client import ProgramRpcClient
from app.clients.rpc.user_client import UserRpcClient
from app.config import get_settings
from app.orchestrator.agents.activity import ActivityAgent
from app.orchestrator.agents.coordinator import CoordinatorAgent
from app.orchestrator.agents.handoff import HandoffAgent
from app.orchestrator.agents.order import OrderAgent
from app.orchestrator.agents.refund import RefundAgent
from app.orchestrator.agents.supervisor import SupervisorAgent
from app.tools.activity import build_activity_tools
from app.tools.handoff import build_handoff_tools
from app.tools.order import build_order_tools
from app.tools.refund import build_refund_tools


class GraphOrchestrator:
    def __init__(self, *, program_client=None, order_client=None, user_client=None) -> None:
        self.settings = get_settings()
        self.program_client = program_client
        self.order_client = order_client
        self.user_client = user_client
        self.coordinator = CoordinatorAgent()
        self.supervisor = SupervisorAgent()
        self.activity_agent = None
        self.order_agent = None
        self.refund_agent = None
        self.handoff_agent = None

    def _ensure_agents(self) -> None:
        if self.program_client is None:
            self.program_client = ProgramRpcClient(target=self.settings.program_rpc_target)
        if self.order_client is None:
            self.order_client = OrderRpcClient(target=self.settings.order_rpc_target)
        if self.user_client is None:
            self.user_client = UserRpcClient(target=self.settings.user_rpc_target)
        if self.activity_agent is None:
            self.activity_agent = ActivityAgent(tools=build_activity_tools(self.program_client))
        if self.order_agent is None:
            self.order_agent = OrderAgent(tools=build_order_tools(self.order_client))
        if self.refund_agent is None:
            self.refund_agent = RefundAgent(tools=build_refund_tools(self.order_client))
        if self.handoff_agent is None:
            self.handoff_agent = HandoffAgent(tools=build_handoff_tools())

    async def reply(self, session, *, message: str):
        self._ensure_agents()
        intent = self.coordinator.detect_intent(message)
        next_agent = self.supervisor.next_agent(intent)

        if next_agent == "activity":
            return await self.activity_agent.handle(message=message)
        if next_agent == "order":
            return await self.order_agent.handle(user_id=session.user_id, message=message)
        if next_agent == "refund":
            return await self.refund_agent.handle(user_id=session.user_id, message=message)
        return await self.handoff_agent.handle(message=message)
