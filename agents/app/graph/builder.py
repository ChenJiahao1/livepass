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


def build_graph_app(*, checkpointer=None):
    builder = StateGraph(ConversationState, context_schema=GraphContext)
    builder.add_node("prepare_turn", prepare_turn_node)
    builder.add_node("coordinator", coordinator_node)
    builder.add_node("supervisor", supervisor_node)
    builder.add_node("activity", activity_node)
    builder.add_node("order", order_node)

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
            "activity": "activity",
            "order": "order",
            END: END,
        },
    )
    builder.add_edge("activity", "supervisor")
    builder.add_edge("order", "supervisor")
    return builder.compile(checkpointer=checkpointer)
