"""Graph routing decisions."""

from langgraph.graph import END

from app.graph.state import ConversationState
from app.shared.runtime_constants import NEXT_AGENT_FINISH, SPECIALIST_AGENT_NAMES

SPECIALIST_AGENTS = set(SPECIALIST_AGENT_NAMES)


def next_from_coordinator(state: ConversationState) -> str:
    if state.get("coordinator_action") == "delegate":
        return "supervisor"
    return END


def next_from_supervisor(state: ConversationState) -> str:
    next_agent = state.get("next_agent", NEXT_AGENT_FINISH)
    if next_agent in SPECIALIST_AGENTS:
        return str(next_agent)
    return END
