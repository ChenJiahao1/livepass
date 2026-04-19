"""LangGraph runtime assembly for the customer agent flow."""

from langgraph.graph import END, START, StateGraph

from app.graph.nodes import (
    activity_node,
    coordinator_node,
    order_node,
    prepare_turn_node,
    supervisor_node,
)
from app.graph.routing import next_from_coordinator, next_from_supervisor
from app.graph.state import ConversationState, GraphContext
from app.shared.runtime_constants import AGENT_ACTIVITY, AGENT_ORDER


def build_graph_app(*, checkpointer=None):
    builder = StateGraph(ConversationState, context_schema=GraphContext)
    builder.add_node("prepare_turn", prepare_turn_node)
    builder.add_node("coordinator", coordinator_node)
    builder.add_node("supervisor", supervisor_node)
    builder.add_node(AGENT_ACTIVITY, activity_node)
    builder.add_node(AGENT_ORDER, order_node)

    builder.add_edge(START, "prepare_turn")
    builder.add_edge("prepare_turn", "coordinator")
    builder.add_conditional_edges(
        "coordinator",
        next_from_coordinator,
        {
            "supervisor": "supervisor",
            END: END,
        },
    )
    builder.add_conditional_edges(
        "supervisor",
        next_from_supervisor,
        {
            AGENT_ACTIVITY: AGENT_ACTIVITY,
            AGENT_ORDER: AGENT_ORDER,
            END: END,
        },
    )
    builder.add_edge(AGENT_ACTIVITY, "supervisor")
    builder.add_edge(AGENT_ORDER, "supervisor")
    return builder.compile(checkpointer=checkpointer)
