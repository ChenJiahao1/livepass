"""Common helpers for deterministic specialist agents."""

from __future__ import annotations

import re

from app.orchestrator.state import OrchestratorResult

ORDER_NUMBER_PATTERN = re.compile(r"(?<!\d)(\d{4,})(?!\d)")


class BaseAgent:
    def __init__(self, *, tools: list) -> None:
        self.tools = {tool.name: tool for tool in tools}

    def tool(self, name: str):
        return self.tools[name]

    def extract_order_number(self, message: str) -> int | None:
        match = ORDER_NUMBER_PATTERN.search(message)
        if not match:
            return None
        return int(match.group(1))

    def result(
        self,
        *,
        reply: str,
        current_agent: str,
        need_handoff: bool = False,
        status: str = "completed",
    ) -> OrchestratorResult:
        return OrchestratorResult(
            reply=reply,
            status=status,
            current_agent=current_agent,
            need_handoff=need_handoff,
        )
