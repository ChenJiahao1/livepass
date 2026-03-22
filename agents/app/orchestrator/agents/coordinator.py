"""Coordinator that derives the coarse user intent."""


class CoordinatorAgent:
    def detect_intent(self, message: str) -> str:
        normalized = message.lower()
        if "人工" in message:
            return "handoff"
        if "退款" in message or "退票" in message:
            return "refund"
        if "订单" in message:
            return "order"
        if "演出" in message or "活动" in message:
            return "activity"
        return "handoff"
