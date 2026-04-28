"""Graph node adapters between state and agent roles."""

from __future__ import annotations

from typing import Any

from langchain_core.messages import AIMessage
from langgraph.runtime import Runtime

from app.agents.coordinator import CoordinatorAgent
from app.agents.specialists.activity_specialist import ActivityAgent
from app.agents.specialists.order_specialist import OrderAgent
from app.agents.supervisor import SupervisorAgent
from app.graph.state import ConversationState, GraphContext
from app.shared.runtime_constants import AGENT_ACTIVITY, AGENT_ORDER


def prepare_turn_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    require_context(runtime, "llm")
    payload: dict[str, Any] = {
        "route": None,
        "coordinator_action": None,
        "next_agent": None,
        "specialist_result": None,
        "trace": [],
        "current_agent": None,
        "final_reply": "",
        "reply": "",
    }
    if not state.get("current_user_id") and runtime.context.get("current_user_id"):
        payload["current_user_id"] = runtime.context["current_user_id"]
    return payload


def coordinator_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    hydrated = hydrate_state(state, runtime)
    llm = require_context(runtime, "llm")
    current_user_id = hydrated.get("current_user_id")

    result = CoordinatorAgent(llm=llm).handle(hydrated)
    base_state: dict[str, Any] = {
        "coordinator_action": result["action"],
        "route": result.get("route"),
        "selected_order_id": result.get("selected_order_id"),
        "selected_program_id": result.get("selected_program_id"),
        "current_user_id": current_user_id,
        "trace": result["trace"],
        "specialist_result": None,
        "next_agent": None,
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


def supervisor_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    hydrated = hydrate_state(state, runtime)
    llm = require_context(runtime, "llm")
    current_user_id = hydrated.get("current_user_id")

    specialist_result = hydrated.get("specialist_result") or {}
    if specialist_result.get("completed"):
        route = hydrated.get("route") or specialist_result.get("agent") or hydrated.get("last_intent", "unknown")
        return {
            "current_agent": "supervisor",
            "next_agent": "finish",
            "route": route,
            "last_intent": route,
            "trace": hydrated.get("trace", []),
            "selected_order_id": hydrated.get("selected_order_id"),
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
        "current_user_id": current_user_id,
    }
    if result["next_agent"] == "finish":
        payload["reply"] = ""
        payload["final_reply"] = hydrated.get("final_reply", "")
    return payload


async def activity_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    agent = ActivityAgent(registry=runtime.context.get("registry"), llm=require_context(runtime, "llm"))
    result = await agent.handle(hydrate_state(state, runtime))
    return map_specialist_result(state, result, AGENT_ACTIVITY)


async def order_node(state: ConversationState, runtime: Runtime[GraphContext]) -> dict[str, Any]:
    agent = OrderAgent(registry=runtime.context.get("registry"), llm=require_context(runtime, "llm"))
    result = await agent.handle(hydrate_state(state, runtime))
    return map_specialist_result(state, result, AGENT_ORDER)


def hydrate_state(state: ConversationState, runtime: Runtime[GraphContext]) -> ConversationState:
    payload = dict(state)
    if not payload.get("current_user_id") and runtime.context.get("current_user_id"):
        payload["current_user_id"] = runtime.context["current_user_id"]
    return payload


def require_context(runtime: Runtime[GraphContext], key: str) -> Any:
    value = runtime.context.get(key)
    if value is None:
        raise ValueError(f"{key} is required")
    return value


def map_specialist_result(state: ConversationState, result: dict[str, Any], agent_name: str) -> dict[str, Any]:
    reply = str(result["reply"])
    payload: dict[str, Any] = {
        "current_agent": agent_name,
        "reply": reply,
        "final_reply": reply,
        "messages": result.get("messages", [AIMessage(content=reply)]),
        "trace": [*state.get("trace", []), *result.get("trace", [])],
        "selected_order_id": result.get("selected_order_id") or state.get("selected_order_id"),
        "selected_program_id": result.get("selected_program_id") or state.get("selected_program_id"),
        "specialist_result": {
            "agent": agent_name,
            "completed": result.get("completed", False),
            "result_summary": result.get("result_summary", reply),
        },
    }
    return payload
