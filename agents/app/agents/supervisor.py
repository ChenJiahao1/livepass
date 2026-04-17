"""Supervisor agent entrypoint."""

from langchain_core.messages import SystemMessage
from langchain_core.messages.utils import convert_to_messages

from app.llm.schemas import SupervisorDecision
from app.shared.prompt_loader import PromptLoader
from app.state import ConversationState


class SupervisorAgent:
    def __init__(self, *, llm, prompt_loader: PromptLoader | None = None) -> None:
        self.llm = llm
        self.prompt_loader = prompt_loader or PromptLoader()

    def handle(self, state: ConversationState) -> dict[str, object]:
        system_prompt = self.prompt_loader.render(
            "supervisor",
            selected_order_id=state.get("selected_order_id"),
            route=state.get("route"),
            specialist_result=state.get("specialist_result"),
            current_user_id=state.get("current_user_id"),
        )
        decision = self.llm.with_structured_output(SupervisorDecision).invoke(
            [SystemMessage(content=system_prompt), *convert_to_messages(state.get("messages", []))]
        )
        route = state.get("route")
        if decision.next_agent in {"activity", "order", "refund", "handoff", "knowledge"}:
            route = decision.next_agent
        elif route is None:
            specialist_result = state.get("specialist_result") or {}
            route = specialist_result.get("agent") or state.get("last_intent", "unknown")

        return {
            "agent": "supervisor",
            "next_agent": decision.next_agent,
            "route": route,
            "reply": "",
            "trace": [f"route:{route}"] if decision.next_agent != "finish" else [],
            "need_handoff": decision.need_handoff,
            "selected_order_id": decision.selected_order_id,
        }
