"""Refund specialist."""

from app.orchestrator.agents.base import BaseAgent


class RefundAgent(BaseAgent):
    async def handle(self, *, user_id: int, message: str):
        order_number = self.extract_order_number(message)
        if not order_number:
            return self.result(reply="请先提供需要处理的订单号。", current_agent="refund")

        wants_submit = "帮我退款" in message or "申请退款" in message or "发起退款" in message
        if wants_submit:
            result = await self.tool("refund_order").ainvoke(
                {
                    "user_id": user_id,
                    "order_number": order_number,
                    "reason": "用户发起退款",
                }
            )
            return self.result(
                reply=f"订单 {result['orderNumber']} 已提交退款，退款金额 {result['refundAmount']}。",
                current_agent="refund",
            )

        preview = await self.tool("preview_refund_order").ainvoke(
            {"user_id": user_id, "order_number": order_number}
        )
        if preview["allowRefund"]:
            return self.result(
                reply=(
                    f"订单 {preview['orderNumber']} 当前可退款，预计退款 {preview['refundAmount']}，"
                    f"退款比例 {preview['refundPercent']}%。"
                ),
                current_agent="refund",
            )

        return self.result(
            reply=preview["rejectReason"] or "当前订单不可退。",
            current_agent="refund",
        )
