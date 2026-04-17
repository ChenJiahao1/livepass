"""Graph routing decisions."""

from langgraph.graph import END

from app.graph.state import ConversationState

SPECIALIST_AGENTS = {"activity", "order", "refund", "handoff", "knowledge"}


def next_from_coordinator(state: ConversationState) -> str:
    if state.get("coordinator_action") == "delegate":
        return "supervisor"
    return END


def next_from_supervisor(state: ConversationState) -> str:
    next_agent = state.get("next_agent", "finish")
    if next_agent in SPECIALIST_AGENTS:
        return str(next_agent)
    return END
