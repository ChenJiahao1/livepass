"""Order specialist."""

from app.orchestrator.agents.base import BaseAgent


class OrderAgent(BaseAgent):
    async def handle(self, *, user_id: int, message: str):
        order_number = self.extract_order_number(message)
        if order_number:
            detail = await self.tool("get_order_service_view").ainvoke(
                {"user_id": user_id, "order_number": order_number}
            )
            return self.result(
                reply=(
                    f"订单 {detail['orderNumber']} 当前状态 {detail['orderStatus']}，"
                    f"支付状态 {detail['payStatus']}，票券状态 {detail['ticketStatus']}。"
                ),
                current_agent="order",
            )

        orders = await self.tool("list_orders").ainvoke({"user_id": user_id})
        if not orders["list"]:
            return self.result(reply="当前账号下没有可查询订单。", current_agent="order")

        first = orders["list"][0]
        return self.result(
            reply=f"当前可查询订单包含 {first['orderNumber']}，状态 {first['orderStatus']}。",
            current_agent="order",
        )
