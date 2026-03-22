"""Keyword-based intent routing."""

from app.state import ConversationState, Intent

ACTIVITY_KEYWORDS = ("活动", "节目", "演出", "场次", "票档", "票价")
ORDER_KEYWORDS = ("订单", "查单", "购票记录", "我的票")
REFUND_KEYWORDS = ("退款", "退票", "退单", "退钱")
HANDOFF_KEYWORDS = ("人工", "客服", "转人工", "投诉")
KNOWLEDGE_KEYWORDS = ("是谁", "简介", "代表作", "奖项", "获奖", "经历")


def route_intent(state: ConversationState) -> Intent:
    message = _latest_user_message(state)

    if not message:
        return "unknown"
    if _contains_keyword(message, REFUND_KEYWORDS):
        return "refund"
    if _contains_keyword(message, HANDOFF_KEYWORDS):
        return "handoff"
    if _contains_keyword(message, ORDER_KEYWORDS):
        return "order"
    if _contains_keyword(message, ACTIVITY_KEYWORDS):
        return "activity"
    if _contains_keyword(message, KNOWLEDGE_KEYWORDS):
        return "knowledge"
    return "unknown"


def _latest_user_message(state: ConversationState) -> str:
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


def _contains_keyword(message: str, keywords: tuple[str, ...]) -> bool:
    return any(keyword in message for keyword in keywords)
