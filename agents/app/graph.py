"""LangGraph runtime assembly for the customer agent flow."""

from __future__ import annotations

from typing import Any

from langchain_core.messages import AIMessage
from langgraph.graph import END, START, StateGraph
from langgraph.runtime import Runtime

from app.agents.activity import ActivityAgent
from app.agents.coordinator import CoordinatorAgent
from app.agents.handoff import HandoffAgent
from app.agents.knowledge import KnowledgeAgent
from app.agents.order import OrderAgent
from app.agents.refund import RefundAgent
from app.agents.supervisor import SupervisorAgent
from app.state import ConversationState, GraphContext


SPECIALIST_AGENTS = {"activity", "order", "refund", "handoff", "knowledge"}


def build_graph_app(*, checkpointer=None):
    builder = StateGraph(ConversationState, context_schema=GraphContext)
    builder.add_node("prepare_turn", _prepare_turn_node)
    builder.add_node("coordinator", _coordinator_node)
    builder.add_node("supervisor", _supervisor_node)
    builder.add_node("activity", _activity_node)
    builder.add_node("order", _order_node)
    builder.add_node("refund", _refund_node)
    builder.add_node("handoff", _handoff_node)
    builder.add_node("knowledge", _knowledge_node)

    builder.add_edge(START, "prepare_turn")
    builder.add_edge("prepare_turn", "coordinator")
    builder.add_conditional_edges(
        "coordinator",
        _next_from_coordinator,
        {
            "supervisor": "supervisor",
            END: END,
        },
    )
    builder.add_conditional_edges(
        "supervisor",
        _next_from_supervisor,
        {
            "activity": "activity",
            "order": "order",
            "refund": "refund",
            "handoff": "handoff",
            "knowledge": "knowledge",
            END: END,
        },
    )
    builder.add_edge("activity", "supervisor")
    builder.add_edge("order", "supervisor")
    builder.add_edge("refund", "supervisor")
    builder.add_edge("handoff", END)
    builder.add_edge("knowledge", "supervisor")
    return builder.compile(checkpointer=checkpointer)


def _prepare_turn_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    _require_context(runtime, "llm")
    payload: dict[str, Any] = {
        "route": None,
        "coordinator_action": None,
        "next_agent": None,
        "business_ready": False,
        "delegated": False,
        "specialist_result": None,
        "need_handoff": False,
        "trace": [],
        "current_agent": None,
        "final_reply": "",
        "reply": "",
    }
    if not state.get("current_user_id") and runtime.context.get("current_user_id"):
        payload["current_user_id"] = runtime.context["current_user_id"]
    return payload


def _coordinator_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    hydrated = _hydrate_state(state, runtime)
    llm = _require_context(runtime, "llm")
    current_user_id = hydrated.get("current_user_id")

    result = CoordinatorAgent(llm=llm).handle(hydrated)
    base_state: dict[str, Any] = {
        "coordinator_action": result["action"],
        "business_ready": result["business_ready"],
        "delegated": result["action"] == "delegate",
        "selected_order_id": result.get("selected_order_id"),
        "current_user_id": current_user_id,
        "trace": result["trace"],
        "specialist_result": None,
        "next_agent": None,
        "need_handoff": False,
    }
    if result["action"] == "delegate":
        return base_state

    reply = str(result["reply"])
    return {
        **base_state,
        "current_agent": "coordinator",
        "reply": reply,
        "final_reply": reply,
        "messages": [AIMessage(content=reply)],
    }


def _supervisor_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    hydrated = _hydrate_state(state, runtime)
    llm = _require_context(runtime, "llm")
    current_user_id = hydrated.get("current_user_id")

    specialist_result = hydrated.get("specialist_result") or {}
    if specialist_result.get("completed"):
        route = hydrated.get("route") or specialist_result.get("agent") or hydrated.get("last_intent", "unknown")
        if hydrated.get("need_handoff") or specialist_result.get("need_handoff"):
            return {
                "current_agent": "supervisor",
                "next_agent": "handoff",
                "route": "handoff",
                "last_intent": "handoff",
                "trace": [*hydrated.get("trace", []), "route:handoff"],
                "selected_order_id": hydrated.get("selected_order_id"),
                "need_handoff": True,
                "current_user_id": current_user_id,
            }
        return {
            "current_agent": "supervisor",
            "next_agent": "finish",
            "route": route,
            "last_intent": route,
            "trace": hydrated.get("trace", []),
            "selected_order_id": hydrated.get("selected_order_id"),
            "need_handoff": False,
            "current_user_id": current_user_id,
            "reply": hydrated.get("reply", ""),
            "final_reply": hydrated.get("final_reply", ""),
        }

    result = SupervisorAgent(llm=llm).handle(hydrated)
    payload = {
        "current_agent": "supervisor",
        "next_agent": result["next_agent"],
        "route": result["route"],
        "last_intent": result["route"],
        "trace": [*hydrated.get("trace", []), *result["trace"]],
        "selected_order_id": result.get("selected_order_id") or hydrated.get("selected_order_id"),
        "need_handoff": result["need_handoff"],
        "current_user_id": current_user_id,
    }
    if result["next_agent"] == "finish":
        payload["reply"] = ""
        payload["final_reply"] = hydrated.get("final_reply", "")
    return payload


async def _activity_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    agent = ActivityAgent(registry=runtime.context.get("registry"), llm=_require_context(runtime, "llm"))
    result = await agent.handle(_hydrate_state(state, runtime))
    return _map_specialist_result(state, result, "activity")


async def _order_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    agent = OrderAgent(registry=runtime.context.get("registry"), llm=_require_context(runtime, "llm"))
    result = await agent.handle(_hydrate_state(state, runtime))
    return _map_specialist_result(state, result, "order")


async def _refund_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    agent = RefundAgent(registry=runtime.context.get("registry"), llm=_require_context(runtime, "llm"))
    result = await agent.handle(_hydrate_state(state, runtime))
    return _map_specialist_result(state, result, "refund")


async def _handoff_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    agent = HandoffAgent(registry=None, llm=runtime.context.get("llm"))
    result = await agent.handle(_hydrate_state(state, runtime))
    return _map_specialist_result(state, result, "handoff")


async def _knowledge_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    agent = KnowledgeAgent(service=runtime.context.get("knowledge_service"))
    result = await agent.handle(_hydrate_state(state, runtime))
    return _map_specialist_result(state, result, "knowledge")


def _next_from_coordinator(state: ConversationState) -> str:
    if state.get("coordinator_action") == "delegate":
        return "supervisor"
    return END


def _next_from_supervisor(state: ConversationState) -> str:
    next_agent = state.get("next_agent", "finish")
    if next_agent in SPECIALIST_AGENTS:
        return next_agent
    return END


def _hydrate_state(state: ConversationState, runtime: Runtime[GraphContext]) -> ConversationState:
    payload = dict(state)
    if not payload.get("current_user_id") and runtime.context.get("current_user_id"):
        payload["current_user_id"] = runtime.context["current_user_id"]
    return payload


def _require_context(runtime: Runtime[GraphContext], key: str) -> Any:
    value = runtime.context.get(key)
    if value is None:
        raise ValueError(f"{key} is required")
    return value


def _map_specialist_result(state: ConversationState, result: dict[str, Any], agent_name: str) -> dict[str, Any]:
    reply = str(result["reply"])
    payload: dict[str, Any] = {
        "current_agent": agent_name,
        "reply": reply,
        "final_reply": reply,
        "messages": result.get("messages", [AIMessage(content=reply)]),
        "trace": [*state.get("trace", []), *result.get("trace", [])],
        "selected_order_id": result.get("selected_order_id") or state.get("selected_order_id"),
        "selected_program_id": result.get("selected_program_id") or state.get("selected_program_id"),
        "need_handoff": result.get("need_handoff", False),
        "specialist_result": {
            "agent": agent_name,
            "completed": result.get("completed", False),
            "need_handoff": result.get("need_handoff", False),
            "result_summary": result.get("result_summary", reply),
        },
    }
    if result.get("last_refund_preview") is not None:
        payload["last_refund_preview"] = result["last_refund_preview"]
    if result.get("pending_confirmation") is not None:
        payload["pending_confirmation"] = result["pending_confirmation"]
    if result.get("pending_action") is not None:
        payload["pending_action"] = result["pending_action"]
    return payload
