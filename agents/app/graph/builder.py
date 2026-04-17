"""LangGraph runtime assembly for the customer agent flow."""

from langgraph.graph import END, START, StateGraph

from app.graph.nodes import (
    activity_node,
    coordinator_node,
    handoff_node,
    knowledge_node,
    order_node,
    prepare_turn_node,
    supervisor_node,
)
from app.graph.routing import next_from_coordinator, next_from_supervisor
from app.graph.state import ConversationState, GraphContext
from app.graph.subgraphs.refund import (
    apply_human_decision,
    finish_refund,
    prepare_refund,
    preview_refund,
    request_refund_approval,
    should_preview_refund,
    should_request_refund_approval,
    submit_refund,
)


def build_graph_app(*, checkpointer=None):
    builder = StateGraph(ConversationState, context_schema=GraphContext)
    builder.add_node("prepare_turn", prepare_turn_node)
    builder.add_node("coordinator", coordinator_node)
    builder.add_node("supervisor", supervisor_node)
    builder.add_node("activity", activity_node)
    builder.add_node("order", order_node)
    builder.add_node("refund_prepare", prepare_refund)
    builder.add_node("refund_preview", preview_refund)
    builder.add_node("refund_request_approval", request_refund_approval)
    builder.add_node("refund_apply_human_decision", apply_human_decision)
    builder.add_node("refund_submit", submit_refund)
    builder.add_node("refund_finish", finish_refund)
    builder.add_node("handoff", handoff_node)
    builder.add_node("knowledge", knowledge_node)

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
            "refund": "refund_prepare",
            "handoff": "handoff",
            "knowledge": "knowledge",
            END: END,
        },
    )
    builder.add_edge("activity", "supervisor")
    builder.add_edge("order", "supervisor")
    builder.add_conditional_edges(
        "refund_prepare",
        should_preview_refund,
        {
            "preview": "refund_preview",
            "finish": "refund_finish",
        },
    )
    builder.add_conditional_edges(
        "refund_preview",
        should_request_refund_approval,
        {
            "request_approval": "refund_request_approval",
            "finish": "refund_finish",
        },
    )
    builder.add_edge("refund_request_approval", "refund_apply_human_decision")
    builder.add_edge("refund_apply_human_decision", "refund_submit")
    builder.add_edge("refund_submit", "refund_finish")
    builder.add_edge("refund_finish", "supervisor")
    builder.add_edge("handoff", END)
    builder.add_edge("knowledge", "supervisor")
    return builder.compile(checkpointer=checkpointer)
