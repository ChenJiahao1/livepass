"""LangGraph runtime assembly for the customer agent flow."""

from __future__ import annotations

from typing import Any

from langgraph.graph import END, START, StateGraph
from langgraph.runtime import Runtime

from app.agents.activity import ActivityAgent
from app.agents.coordinator import CoordinatorAgent
from app.agents.handoff import HandoffAgent
from app.agents.knowledge import KnowledgeAgent
from app.agents.order import OrderAgent
from app.agents.refund import RefundAgent
from app.agents.supervisor import SupervisorAgent
from app.router import route_intent
from app.state import ConversationState, GraphContext


def build_graph_app():
    builder = StateGraph(ConversationState, context_schema=GraphContext)
    builder.add_node("coordinator", _coordinator_node)
    builder.add_node("supervisor", _supervisor_node)
    builder.add_node("activity", _activity_node)
    builder.add_node("order", _order_node)
    builder.add_node("refund", _refund_node)
    builder.add_node("handoff", _handoff_node)
    builder.add_node("knowledge", _knowledge_node)

    builder.add_edge(START, "coordinator")
    builder.add_conditional_edges(
        "coordinator",
        _after_coordinator,
        {
            "supervisor": "supervisor",
            "finish": END,
        },
    )
    builder.add_conditional_edges(
        "supervisor",
        _after_supervisor,
        {
            "activity": "activity",
            "order": "order",
            "refund": "refund",
            "handoff": "handoff",
            "knowledge": "knowledge",
            "finish": END,
        },
    )
    builder.add_edge("activity", "supervisor")
    builder.add_edge("order", "supervisor")
    builder.add_edge("refund", "supervisor")
    builder.add_edge("handoff", "supervisor")
    builder.add_edge("knowledge", "supervisor")
    return builder.compile()


def _coordinator_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    hydrated = _hydrate_state(state, runtime)
    llm = runtime.context.get("llm")
    if llm is None:
        intent = route_intent(hydrated)
        if intent == "unknown":
            reply = "请补充一下你想咨询的节目、订单或退款问题。"
            return {
                "coordinator_action": "clarify",
                "current_agent": "coordinator",
                "reply": reply,
                "final_reply": reply,
                "trace": _append_trace(hydrated, "coordinator:clarify"),
            }
        return {
            "coordinator_action": "delegate",
            "route": intent,
            "trace": _append_trace(hydrated, "coordinator:delegate"),
        }

    decision = CoordinatorAgent(llm=llm).handle(hydrated)
    payload: dict[str, Any] = {
        "coordinator_action": decision["action"],
        "trace": _append_trace(hydrated, f"coordinator:{decision['action']}"),
    }
    if decision.get("selected_order_id"):
        payload["selected_order_id"] = decision["selected_order_id"]
    if decision["action"] in {"respond", "clarify"}:
        payload["current_agent"] = "coordinator"
        payload["reply"] = decision["reply"]
        payload["final_reply"] = decision["reply"]
    return payload


def _after_coordinator(state: ConversationState) -> str:
    if state.get("coordinator_action") == "delegate":
        return "supervisor"
    return "finish"


def _supervisor_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    hydrated = _hydrate_state(state, runtime)
    llm = runtime.context.get("llm")
    if llm is None:
        next_agent = hydrated.get("route") or route_intent(hydrated)
        if next_agent == "unknown":
            next_agent = "handoff"
        return {
            "next_agent": next_agent,
            "need_handoff": next_agent == "handoff",
            "trace": _append_trace(hydrated, f"supervisor:{next_agent}"),
        }

    decision = SupervisorAgent(llm=llm).handle(hydrated)
    payload: dict[str, Any] = {
        "next_agent": decision["next_agent"],
        "need_handoff": decision["need_handoff"],
        "trace": _append_trace(hydrated, f"supervisor:{decision['next_agent']}"),
    }
    if decision.get("selected_order_id"):
        payload["selected_order_id"] = decision["selected_order_id"]
    return payload


def _after_supervisor(state: ConversationState) -> str:
    return state.get("next_agent", "finish")


async def _activity_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    hydrated = _hydrate_state(state, runtime)
    agent = ActivityAgent(registry=runtime.context.get("registry"), llm=runtime.context.get("llm"))
    return await agent.handle(hydrated)


async def _order_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    hydrated = _hydrate_state(state, runtime)
    agent = OrderAgent(registry=runtime.context.get("registry"), llm=runtime.context.get("llm"))
    return await agent.handle(hydrated)


async def _refund_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    hydrated = _hydrate_state(state, runtime)
    agent = RefundAgent(registry=runtime.context.get("registry"), llm=runtime.context.get("llm"))
    return await agent.handle(hydrated)


async def _handoff_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    hydrated = _hydrate_state(state, runtime)
    agent = HandoffAgent(registry=runtime.context.get("registry"), llm=runtime.context.get("llm"))
    return await agent.handle(hydrated)


async def _knowledge_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    hydrated = _hydrate_state(state, runtime)
    agent = KnowledgeAgent(http_client=runtime.context.get("knowledge_http_client"))
    return await agent.handle(hydrated)


def _hydrate_state(state: ConversationState, runtime: Runtime[GraphContext]) -> ConversationState:
    payload = dict(state)
    if not payload.get("current_user_id") and runtime.context.get("current_user_id"):
        payload["current_user_id"] = runtime.context["current_user_id"]
    return payload


def _append_trace(state: ConversationState, step: str) -> list[str]:
    return [*state.get("trace", []), step]
