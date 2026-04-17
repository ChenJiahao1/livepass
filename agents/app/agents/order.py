"""Order specialist agent."""

from app.agents.base import ToolCallingAgent
from app.state import ConversationState


class OrderAgent(ToolCallingAgent):
    agent_name = "order"
    toolset = "order"
    prompt_template = "order/system.md"

    async def handle(self, state: ConversationState) -> dict[str, object]:
        tools = await self.get_tools()
        order_id = self.extract_order_id(state)
        current_user_id = self.normalize_user_id(state.get("current_user_id"))

        if not order_id and current_user_id:
            list_orders_tool = self.find_tool(tools, "list_user_orders", "list_orders")
            if list_orders_tool is not None:
                orders = await list_orders_tool.ainvoke({"user_id": current_user_id})
                items = orders.get("orders") or orders.get("list") or []
                if not items:
                    return self.result(state, reply="当前账号下没有可查询订单。", result_summary="当前账号下没有可查询订单")
                summary = "，".join(
                    f"{item.get('order_id') or item.get('orderNumber')}（{item.get('status') or item.get('orderStatus')}）"
                    for item in items
                )
                return self.result(
                    state,
                    reply=f"当前账号下有以下订单：{summary}",
                    specialist_result={"orders": items},
                    result_summary="已向用户展示订单列表",
                )

        if order_id:
            detail_tool = self.find_tool(tools, "get_order_detail_for_service", "get_order_service_view")
            if detail_tool is not None:
                payload = {"order_id": order_id}
                if current_user_id is not None:
                    payload["user_id"] = current_user_id
                detail = await detail_tool.ainvoke(payload)
                status = detail.get("status") or detail.get("orderStatus", "未知")
                payment_status = detail.get("payment_status") or detail.get("payStatus", "未知")
                ticket_status = detail.get("ticket_status") or detail.get("ticketStatus", "未知")
                resolved_order_id = detail.get("order_id") or detail.get("orderNumber") or order_id
                return self.result(
                    state,
                    reply=(
                        f"订单 {resolved_order_id} 当前状态 {status}，"
                        f"支付状态 {payment_status}，票券状态 {ticket_status}。"
                    ),
                    specialist_result=detail,
                    selected_order_id=str(resolved_order_id),
                    result_summary="订单已成功查询",
                )

        return await self.run_tool_agent(state)
