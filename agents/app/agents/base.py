"""Shared helpers for customer specialist agents."""

from __future__ import annotations

import re
from typing import Any

from langchain.agents import create_agent
from langchain_core.messages import AIMessage, BaseMessage
from langchain_core.messages.utils import convert_to_messages

from app.config import get_settings
from app.mcp_client.tracing import reset_trace_buffer, set_trace_buffer
from app.prompts import PromptRenderer
from app.state import ConversationState

ORDER_ID_PATTERN = re.compile(r"(ORD-\d{4,}|\d{4,})")
_UNSET = object()


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

    async def handle(self, state: ConversationState) -> dict[str, Any]:
        return await self.run_tool_agent(state)

    async def run_tool_agent(self, state: ConversationState) -> dict[str, Any]:
        trace = list(self.initial_trace(state))
        tools = await self.get_tools()
        system_prompt = self.prompt_renderer.render(
            self.prompt_template,
            **self.build_prompt_context(state),
        )
        agent = create_agent(
            model=self.llm,
            tools=tools,
            system_prompt=system_prompt,
            name=self.agent_name,
        )
        token = set_trace_buffer(trace)
        try:
            result = await agent.ainvoke(
                {"messages": convert_to_messages(state.get("messages", []))},
                config={"recursion_limit": max(4, get_settings().max_tool_steps * 2 + 2)},
            )
        except Exception:
            return self.result(
                state,
                reply="当前处理失败，已转人工继续处理。",
                trace=trace,
                need_handoff=True,
                completed=True,
                result_summary="当前处理失败，已转人工继续处理。",
            )
        finally:
            reset_trace_buffer(token)

        reply = self.extract_reply(result.get("messages", [])) or self.empty_reply()
        return self.result(
            state,
            reply=reply,
            trace=trace,
            need_handoff=self.default_need_handoff(state),
            completed=True,
            result_summary=reply,
        )

    def build_prompt_context(self, state: ConversationState) -> dict[str, Any]:
        return {}

    def initial_trace(self, state: ConversationState) -> list[str]:
        return []

    def default_need_handoff(self, state: ConversationState) -> bool:
        return False

    def empty_reply(self) -> str:
        return "当前未获取到足够信息，请稍后重试。"

    def result(
        self,
        state: ConversationState,
        *,
        reply: str,
        trace: list[str] | None = None,
        need_handoff: bool = False,
        completed: bool = True,
        result_summary: str | None = None,
        selected_order_id: str | None | object = _UNSET,
        selected_program_id: str | None | object = _UNSET,
        status: str | None = None,
    ) -> dict[str, Any]:
        payload: dict[str, Any] = {
            "agent": self.agent_name,
            "reply": reply,
            "trace": list(trace or self.initial_trace(state)),
            "need_handoff": need_handoff,
            "completed": completed,
            "result_summary": result_summary or reply,
        }
        if selected_order_id is _UNSET:
            payload["selected_order_id"] = state.get("selected_order_id")
        else:
            payload["selected_order_id"] = selected_order_id
        if selected_program_id is _UNSET:
            payload["selected_program_id"] = state.get("selected_program_id")
        else:
            payload["selected_program_id"] = selected_program_id
        if status is not None:
            payload["status"] = status
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
            return state["selected_order_id"]
        message = self.latest_user_message(state)
        match = ORDER_ID_PATTERN.search(message)
        if not match:
            return None
        return match.group(1)

    def extract_reply(self, messages: list[BaseMessage]) -> str:
        for message in reversed(messages):
            if isinstance(message, AIMessage):
                text = self._message_text(message)
                if text:
                    return text
        return ""

    def _message_text(self, message: AIMessage) -> str:
        content = message.content
        if isinstance(content, str):
            return content.strip()
        if isinstance(content, list):
            parts = []
            for block in content:
                if isinstance(block, str):
                    parts.append(block)
                elif isinstance(block, dict) and isinstance(block.get("text"), str):
                    parts.append(block["text"])
            return "\n".join(part.strip() for part in parts if part.strip()).strip()
        return ""
