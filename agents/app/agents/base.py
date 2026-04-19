"""Shared helpers for customer specialist agents."""

import re
from typing import Any

from langchain.agents import create_agent
from langchain_core.messages import AIMessage

from app.shared.prompt_loader import PromptLoader
from app.graph.state import ConversationState

ORDER_ID_PATTERN = re.compile(r"(ORD-\d{1,}|\d{4,})", re.IGNORECASE)
REFUND_KEYWORDS = ("退款", "退票", "退单", "refund")


class BaseSpecialistAgent:
    agent_name = ""
    toolset = ""
    prompt_template = ""

    def __init__(self, *, registry, llm, prompt_loader: PromptLoader | None = None) -> None:
        self.registry = registry
        self.llm = llm
        self.prompt_loader = prompt_loader or PromptLoader()

    async def get_tools(self) -> list:
        if self.registry is None:
            return []
        return await self.registry.get_tools(self.toolset)

    async def run_tool_agent(self, state: ConversationState) -> dict[str, Any]:
        tools = await self.get_tools()
        system_prompt = self.prompt_loader.render(self.prompt_template, **self.prompt_context(state))
        agent = create_agent(
            model=self.llm,
            tools=tools,
            system_prompt=system_prompt,
            name=self.agent_name,
        )
        result = await agent.ainvoke({"messages": state.get("messages", [])})
        return self.result(
            state,
            reply=self.extract_reply(result),
            messages=self.extract_new_messages(state, result),
        )

    def prompt_context(self, state: ConversationState) -> dict[str, Any]:
        return dict(state)

    def result(
        self,
        state: ConversationState,
        *,
        reply: str,
        status: str = "completed",
        completed: bool = True,
        result_summary: str | None = None,
        specialist_result: dict[str, Any] | None = None,
        messages: list[Any] | None = None,
        selected_order_id: str | None = None,
        selected_program_id: str | None = None,
    ) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "reply": reply,
            "final_reply": reply,
            "current_agent": self.agent_name,
            "status": status,
            "completed": completed,
            "result_summary": result_summary or reply,
            "selected_order_id": selected_order_id if selected_order_id is not None else state.get("selected_order_id"),
            "selected_program_id": (
                selected_program_id if selected_program_id is not None else state.get("selected_program_id")
            ),
            "messages": messages or [AIMessage(content=reply)],
        }
        if specialist_result is not None:
            payload["specialist_result"] = specialist_result
        return payload

    def find_tool(self, tools: list, *names: str):
        tools_by_name = {tool.name: tool for tool in tools}
        for name in names:
            if name in tools_by_name:
                return tools_by_name[name]
        return None

    def latest_user_message(self, state: ConversationState) -> str:
        messages = state.get("messages", [])
        for message in reversed(messages):
            role = getattr(message, "type", None)
            if role is None and hasattr(message, "get"):
                role = message.get("role")
            if role in {"human", "user"}:
                if hasattr(message, "content"):
                    return str(message.content)
                return str(message.get("content", ""))
        return ""

    def extract_order_id(self, state: ConversationState) -> str | None:
        if state.get("selected_order_id"):
            return str(state["selected_order_id"])
        message = self.latest_user_message(state)
        match = ORDER_ID_PATTERN.search(message)
        if not match:
            return None
        return match.group(1).upper()

    def is_refund_request(self, state: ConversationState) -> bool:
        message = self.latest_user_message(state)
        return any(keyword in message.lower() for keyword in REFUND_KEYWORDS)

    def extract_reply(self, result: dict[str, Any]) -> str:
        messages = result.get("messages", [])
        for message in reversed(messages):
            if isinstance(message, AIMessage):
                return str(message.content)
        return ""

    def extract_new_messages(self, state: ConversationState, result: dict[str, Any]) -> list[Any]:
        messages = list(result.get("messages", []))
        existing_messages = state.get("messages", [])
        if len(messages) > len(existing_messages):
            return messages[len(existing_messages) :]
        reply = self.extract_reply(result)
        if reply:
            return [AIMessage(content=reply)]
        return []
