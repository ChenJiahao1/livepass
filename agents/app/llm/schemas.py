"""Structured LLM outputs used by the graph runtime."""

from typing import Literal

from pydantic import BaseModel


class CoordinatorDecision(BaseModel):
    action: Literal["respond", "clarify", "delegate"]
    reply: str = ""
    selected_order_id: str | None = None
    business_ready: bool = False
    reason: str = ""


class SupervisorDecision(BaseModel):
    next_agent: Literal["activity", "order", "refund", "handoff", "knowledge", "finish"]
    selected_order_id: str | None = None
    need_handoff: bool = False
    reason: str = ""
