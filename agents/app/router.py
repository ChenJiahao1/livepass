"""Keyword-based intent routing."""

from langchain_core.messages import AnyMessage

from app.state import ConversationState, Intent

ACTIVITY_KEYWORDS = ("活动", "节目", "演出", "场次", "票档", "票价", "时间", "地点")
ORDER_KEYWORDS = ("订单", "查单", "购票记录", "我的票", "ord-", "支付", "票码", "出票")
REFUND_KEYWORDS = ("退款", "退票", "退单", "退钱")
HANDOFF_KEYWORDS = ("人工", "客服", "转人工", "投诉")
KNOWLEDGE_KEYWORDS = ("是谁", "简介", "代表作", "奖项", "获奖", "经历")


def route_intent(state: ConversationState) -> Intent:
    message = _latest_user_message(state)

    if not message:
        return "handoff" if state.get("need_handoff") else "unknown"
    if _contains_keyword(message, HANDOFF_KEYWORDS):
        return "handoff"
    if "退" in message and ("ord-" in message or "订单" in message or "票" in message):
        return "refund"
    if _contains_keyword(message, REFUND_KEYWORDS):
        return "refund"
    if _contains_keyword(message, ORDER_KEYWORDS):
        return "order"
    if _contains_keyword(message, ACTIVITY_KEYWORDS):
        return "activity"
    if _contains_keyword(message, KNOWLEDGE_KEYWORDS):
        return "knowledge"
    return "handoff" if state.get("need_handoff") else "unknown"


def _latest_user_message(state: ConversationState) -> str:
    messages = state.get("messages", [])
    for message in reversed(messages):
        if _message_role(message) == "user":
            return _message_content(message).strip().lower()
    return ""


def _message_role(message: AnyMessage | dict) -> str | None:
    if hasattr(message, "type"):
        if message.type == "human":
            return "user"
        if message.type == "ai":
            return "assistant"
    if isinstance(message, dict):
        return message.get("role") or message.get("type")
    return None


def _message_content(message: AnyMessage | dict) -> str:
    if hasattr(message, "content"):
        return str(message.content)
    if isinstance(message, dict):
        return str(message.get("content", ""))
    return ""


def _contains_keyword(message: str, keywords: tuple[str, ...]) -> bool:
    return any(keyword in message for keyword in keywords)
