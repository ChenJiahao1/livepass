"""Shared conversation state for the customer runtime."""

from typing import Any, Literal

from langgraph.graph import MessagesState
from typing_extensions import NotRequired, TypedDict

Intent = Literal["activity", "order", "refund", "handoff", "knowledge", "unknown"]


class ConversationState(MessagesState):
    last_intent: NotRequired[Intent]
    selected_program_id: NotRequired[str | None]
    selected_order_id: NotRequired[str | None]
    current_user_id: NotRequired[str | None]
    last_refund_preview: NotRequired[dict[str, Any] | None]
    tool_call: NotRequired[dict[str, Any] | None]
    need_handoff: NotRequired[bool]

    route: NotRequired[Intent | None]
    coordinator_action: NotRequired[Literal["respond", "clarify", "delegate"]]
    next_agent: NotRequired[Literal["activity", "order", "refund", "handoff", "knowledge", "finish"]]
    business_ready: NotRequired[bool]
    delegated: NotRequired[bool]
    reply: NotRequired[str]
    specialist_result: NotRequired[dict[str, Any] | None]
    status: NotRequired[str]
    trace: NotRequired[list[str]]
    current_agent: NotRequired[str]
    final_reply: NotRequired[str]


class GraphContext(TypedDict, total=False):
    llm: Any
    registry: Any
    current_user_id: str
