"""Coordinator agent entrypoint."""

from langchain_core.messages import SystemMessage
from langchain_core.messages.utils import convert_to_messages

from app.agents.llm import CoordinatorDecision
from app.shared.prompt_loader import PromptLoader
from app.graph.state import ConversationState


class CoordinatorAgent:
    def __init__(self, *, llm, prompt_loader: PromptLoader | None = None) -> None:
        self.llm = llm
        self.prompt_loader = prompt_loader or PromptLoader()

    def handle(self, state: ConversationState) -> dict[str, object]:
        system_prompt = self.prompt_loader.render(
            "coordinator",
            selected_order_id=state.get("selected_order_id"),
            last_intent=state.get("last_intent", "unknown"),
            current_user_id=state.get("current_user_id"),
        )
        decision = self.llm.with_structured_output(CoordinatorDecision).invoke(
            [SystemMessage(content=system_prompt), *convert_to_messages(state.get("messages", []))]
        )
        return {
            "agent": "coordinator",
            "action": decision.action,
            "reply": decision.reply,
            "trace": [f"coordinator:{decision.action}"],
            "selected_order_id": decision.selected_order_id,
            "business_ready": decision.business_ready,
            "reason": decision.reason,
        }
