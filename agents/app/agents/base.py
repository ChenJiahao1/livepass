"""Shared helpers for customer specialist agents."""

from __future__ import annotations

import re
from typing import Any

from langchain.agents import create_agent
from langchain_core.messages import AIMessage

from app.prompts import PromptRenderer
from app.state import ConversationState

ORDER_ID_PATTERN = re.compile(r"(ORD-\d{4,}|\d{4,})")


class ToolCallingAgent:
    agent_name = ""
    toolset = ""
    prompt_template = ""

    def __init__(self, *, registry, llm, prompt_renderer: PromptRenderer | None = None) -> None:
        self.registry = registry
        self.llm = llm
        self.prompt_renderer = prompt_renderer or PromptRenderer()

    async def get_tools(self) -> list:
        if self.registry is None:
            return []
        return await self.registry.get_tools(self.toolset)

    async def run_tool_agent(self, state: ConversationState) -> dict[str, Any]:
        tools = await self.get_tools()
        system_prompt = self.prompt_renderer.render(self.prompt_template)
        agent = create_agent(
            model=self.llm,
            tools=tools,
            system_prompt=system_prompt,
            name=self.agent_name,
        )
        result = await agent.ainvoke({"messages": state.get("messages", [])})
        return self.result(state, reply=self.extract_reply(result))

    def result(
        self,
        state: ConversationState,
        *,
        reply: str,
        need_handoff: bool = False,
        status: str = "completed",
        specialist_result: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "reply": reply,
            "final_reply": reply,
            "current_agent": self.agent_name,
            "status": status,
            "need_handoff": need_handoff,
            "selected_order_id": state.get("selected_order_id"),
            "selected_program_id": state.get("selected_program_id"),
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
            if message.get("role") == "user":
                return str(message.get("content", ""))
        return ""

    def extract_order_id(self, state: ConversationState) -> str | None:
        if state.get("selected_order_id"):
            return state["selected_order_id"]
        message = self.latest_user_message(state)
        match = ORDER_ID_PATTERN.search(message)
        if not match:
            return None
        return match.group(1)

    def extract_reply(self, result: dict[str, Any]) -> str:
        messages = result.get("messages", [])
        for message in reversed(messages):
            if isinstance(message, AIMessage):
                return str(message.content)
        return ""
