"""Shared orchestrator state types."""

from pydantic import BaseModel


class OrchestratorResult(BaseModel):
    reply: str
    status: str
    current_agent: str | None = None
    need_handoff: bool = False
