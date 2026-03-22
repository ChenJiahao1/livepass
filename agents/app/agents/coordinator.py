"""Coordinator agent entrypoint."""

from langchain_core.messages import SystemMessage
from langchain_core.messages.utils import convert_to_messages

from app.llm.schemas import CoordinatorDecision
from app.prompts import PromptRenderer
from app.state import ConversationState


class CoordinatorAgent:
    def __init__(self, *, llm, prompt_renderer: PromptRenderer | None = None) -> None:
        self.llm = llm
        self.prompt_renderer = prompt_renderer or PromptRenderer()

    def handle(self, state: ConversationState) -> dict[str, object]:
        system_prompt = self.prompt_renderer.render("coordinator/system.md")
        decision = self.llm.with_structured_output(CoordinatorDecision).invoke(
            [SystemMessage(content=system_prompt), *convert_to_messages(state.get("messages", []))]
        )
        return decision.model_dump()
