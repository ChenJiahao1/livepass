"""Shared conversation state for the customer runtime."""

from typing import Literal

from langgraph.graph import MessagesState
from typing_extensions import NotRequired

Intent = Literal["activity", "order", "refund", "handoff", "knowledge", "unknown"]


class ConversationState(MessagesState):
    # Cross-turn state
    last_intent: NotRequired[Intent]
    selected_program_id: NotRequired[str | None]
    selected_order_id: NotRequired[str | None]
    current_user_id: NotRequired[str | None]

    # Turn-local state
    reply: NotRequired[str]
    need_handoff: NotRequired[bool]
    status: NotRequired[str]
    trace: NotRequired[list[str]]
    final_reply: NotRequired[str]
