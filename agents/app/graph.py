"""LangGraph runtime assembly for the customer agent flow."""

from __future__ import annotations

import re
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
from app.router import route_intent
from app.state import ConversationState, GraphContext


ORDER_ID_PATTERN = re.compile(r"ORD-\d+", re.IGNORECASE)
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
    builder.add_edge("knowledge", "supervisor")
    builder.add_edge("handoff", END)

    return builder.compile(checkpointer=checkpointer)


def _prepare_turn_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    state_with_context = _state_with_runtime_context(state, runtime)
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
    if state_with_context.get("current_user_id"):
        payload["current_user_id"] = state_with_context["current_user_id"]
    return payload


def _coordinator_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    state_with_context = _state_with_runtime_context(state, runtime)
    llm = runtime.context.get("llm")
    current_user_id = state_with_context.get("current_user_id")

    if llm is None:
        intent = route_intent(state_with_context)
        selected_order_id = _extract_order_id(state_with_context)
        if intent == "unknown":
            reply = "请补充一下你想咨询的节目、订单或退款问题。"
            return {
                "coordinator_action": "clarify",
                "business_ready": False,
                "delegated": False,
                "selected_order_id": selected_order_id,
                "current_user_id": current_user_id,
                "trace": ["coordinator:clarify"],
                "current_agent": "coordinator",
                "reply": reply,
                "final_reply": reply,
                "messages": [AIMessage(content=reply)],
            }
        return {
            "coordinator_action": "delegate",
            "business_ready": True,
            "delegated": True,
            "selected_order_id": selected_order_id,
            "current_user_id": current_user_id,
            "trace": ["coordinator:delegate"],
        }

    coordinator = CoordinatorAgent(llm=llm)
    result = coordinator.handle(state_with_context)
    selected_order_id = result.get("selected_order_id") or _extract_order_id(state_with_context)
    base_state: dict[str, Any] = {
        "coordinator_action": result["action"],
        "business_ready": result["business_ready"],
        "delegated": result["action"] == "delegate",
        "selected_order_id": selected_order_id,
        "current_user_id": current_user_id,
        "trace": result["trace"],
        "specialist_result": None,
        "next_agent": None,
        "need_handoff": False,
    }
    if result["action"] == "delegate":
        return base_state

    reply = result["reply"]
    return {
        **base_state,
        "current_agent": "coordinator",
        "reply": reply,
        "final_reply": reply,
        "messages": [AIMessage(content=reply)],
    }


def _supervisor_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    state_with_context = _state_with_runtime_context(state, runtime)
    current_user_id = state_with_context.get("current_user_id")

    if _should_finish_after_knowledge(state_with_context):
        return {
            "next_agent": "finish",
            "route": "knowledge",
            "last_intent": "knowledge",
            "trace": state_with_context.get("trace", []),
            "selected_order_id": None,
            "need_handoff": False,
            "current_user_id": current_user_id,
        }
    if _should_finish_after_order_listing(state_with_context):
        route = state_with_context.get("route") or "order"
        return {
            "next_agent": "finish",
            "route": route,
            "last_intent": route,
            "trace": state_with_context.get("trace", []),
            "selected_order_id": None,
            "need_handoff": False,
            "current_user_id": current_user_id,
        }

    llm = runtime.context.get("llm")
    if llm is None:
        route = state_with_context.get("route") or state_with_context.get("last_intent") or route_intent(state_with_context)
        if (
            route == "refund"
            and not state_with_context.get("selected_order_id")
            and state_with_context.get("current_user_id")
        ):
            route = "order"
        if route == "unknown":
            route = "handoff"
        return {
            "next_agent": route,
            "route": route,
            "last_intent": route,
            "trace": [*state_with_context.get("trace", []), f"route:{route}"],
            "selected_order_id": _extract_order_id(state_with_context),
            "need_handoff": route == "handoff",
            "current_user_id": current_user_id,
        }

    supervisor = SupervisorAgent(llm=llm)
    result = supervisor.handle(state_with_context)
    return {
        "next_agent": result["next_agent"],
        "route": result["route"],
        "last_intent": result["route"],
        "trace": [*state_with_context.get("trace", []), *result["trace"]],
        "selected_order_id": result.get("selected_order_id") or _extract_order_id(state_with_context),
        "need_handoff": result["need_handoff"],
        "current_user_id": current_user_id,
    }


async def _activity_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    state_with_context = _state_with_runtime_context(state, runtime)
    agent = ActivityAgent(registry=runtime.context.get("registry"), llm=runtime.context.get("llm"))
    result = await agent.handle(state_with_context)
    reply = result["reply"]
    return {
        "current_agent": "activity",
        "reply": reply,
        "final_reply": reply,
        "messages": [AIMessage(content=reply)],
        "trace": [*state_with_context.get("trace", []), *result.get("trace", [])],
        "selected_program_id": result.get("selected_program_id") or state_with_context.get("selected_program_id"),
        "selected_order_id": result.get("selected_order_id") or _extract_order_id(state_with_context),
        "current_user_id": state_with_context.get("current_user_id"),
        "need_handoff": result["need_handoff"],
        "specialist_result": {
            "agent": "activity",
            "completed": result.get("completed", False),
            "need_handoff": result["need_handoff"],
            "result_summary": result.get("result_summary", reply),
        },
    }


async def _order_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    state_with_context = _state_with_runtime_context(state, runtime)
    agent = OrderAgent(registry=runtime.context.get("registry"), llm=runtime.context.get("llm"))
    result = await agent.handle(state_with_context)
    reply = result["reply"]
    return {
        "current_agent": "order",
        "reply": reply,
        "final_reply": reply,
        "messages": [AIMessage(content=reply)],
        "trace": [*state_with_context.get("trace", []), *result.get("trace", [])],
        "selected_order_id": (
            result.get("selected_order_id")
            or state_with_context.get("selected_order_id")
            or _extract_order_id(state_with_context)
        ),
        "current_user_id": state_with_context.get("current_user_id"),
        "need_handoff": result["need_handoff"],
        "specialist_result": {
            "agent": "order",
            "completed": result.get("completed", False),
            "need_handoff": result["need_handoff"],
            "result_summary": result.get("result_summary", reply),
        },
    }


async def _refund_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    state_with_context = _state_with_runtime_context(state, runtime)
    agent = RefundAgent(registry=runtime.context.get("registry"), llm=runtime.context.get("llm"))
    result = await agent.handle(state_with_context)
    reply = result["reply"]
    return {
        "current_agent": "refund",
        "reply": reply,
        "final_reply": reply,
        "messages": [AIMessage(content=reply)],
        "trace": [*state_with_context.get("trace", []), *result.get("trace", [])],
        "selected_order_id": (
            result.get("selected_order_id")
            or state_with_context.get("selected_order_id")
            or _extract_order_id(state_with_context)
        ),
        "current_user_id": state_with_context.get("current_user_id"),
        "need_handoff": result["need_handoff"],
        "specialist_result": {
            "agent": "refund",
            "completed": result.get("completed", False),
            "need_handoff": result["need_handoff"],
            "result_summary": result.get("result_summary", reply),
        },
    }


async def _handoff_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    state_with_context = _state_with_runtime_context(state, runtime)
    agent = HandoffAgent(registry=runtime.context.get("registry"), llm=runtime.context.get("llm"))
    result = await agent.handle(state_with_context)
    reply = result["reply"]
    return {
        "current_agent": "handoff",
        "reply": reply,
        "final_reply": reply,
        "messages": [AIMessage(content=reply)],
        "trace": [*state_with_context.get("trace", []), *result.get("trace", [])],
        "selected_order_id": (
            result.get("selected_order_id")
            or state_with_context.get("selected_order_id")
            or _extract_order_id(state_with_context)
        ),
        "current_user_id": state_with_context.get("current_user_id"),
        "need_handoff": result["need_handoff"],
        "status": result.get("status", "handoff"),
        "specialist_result": {
            "agent": "handoff",
            "completed": result.get("completed", False),
            "need_handoff": result["need_handoff"],
            "result_summary": result.get("result_summary", reply),
        },
    }


async def _knowledge_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    state_with_context = _state_with_runtime_context(state, runtime)
    agent = KnowledgeAgent(http_client=runtime.context.get("knowledge_http_client"))
    result = await agent.handle(state_with_context)
    reply = result["reply"]
    return {
        "current_agent": "knowledge",
        "reply": reply,
        "final_reply": reply,
        "messages": [AIMessage(content=reply)],
        "trace": [*state_with_context.get("trace", []), *result.get("trace", [])],
        "selected_order_id": (
            result.get("selected_order_id")
            or state_with_context.get("selected_order_id")
            or _extract_order_id(state_with_context)
        ),
        "current_user_id": state_with_context.get("current_user_id"),
        "need_handoff": result["need_handoff"],
        "specialist_result": {
            "agent": "knowledge",
            "completed": result.get("completed", False),
            "need_handoff": result["need_handoff"],
            "result_summary": result.get("result_summary", reply),
        },
    }


def _next_from_coordinator(state: ConversationState) -> str:
    if state.get("coordinator_action") == "delegate":
        return "supervisor"
    return END


def _next_from_supervisor(state: ConversationState) -> str:
    next_agent = state.get("next_agent", "handoff")
    if next_agent in SPECIALIST_AGENTS:
        return next_agent
    return END


def _message_content(message: object) -> str:
    if hasattr(message, "content"):
        return str(message.content)
    if isinstance(message, dict):
        return str(message.get("content", ""))
    return ""


def _extract_order_id(state: ConversationState) -> str | None:
    if state.get("selected_order_id"):
        return state["selected_order_id"]

    messages = state.get("messages", [])
    for message in reversed(messages):
        message_type = getattr(message, "type", None)
        message_role = message.get("role") if isinstance(message, dict) else None
        if message_type not in {"human"} and message_role != "user":
            continue
        match = ORDER_ID_PATTERN.search(_message_content(message))
        if match:
            return match.group(0).upper()
    return None


def _state_with_runtime_context(
    state: ConversationState,
    runtime: Runtime[GraphContext],
) -> ConversationState:
    payload = dict(state)
    current_user_id = runtime.context.get("current_user_id")
    if current_user_id is not None:
        payload["current_user_id"] = current_user_id
    return payload


def _should_finish_after_order_listing(state: ConversationState) -> bool:
    specialist_result = state.get("specialist_result") or {}
    return (
        specialist_result.get("agent") == "order"
        and specialist_result.get("completed") is True
        and specialist_result.get("need_handoff") is False
        and specialist_result.get("result_summary") == "已向用户展示订单列表"
        and state.get("selected_order_id") is None
    )


def _should_finish_after_knowledge(state: ConversationState) -> bool:
    specialist_result = state.get("specialist_result") or {}
    return (
        specialist_result.get("agent") == "knowledge"
        and specialist_result.get("completed") is True
        and specialist_result.get("need_handoff") is False
    )
