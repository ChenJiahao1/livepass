"""Order specialist agent."""

import json

from app.agents.base import ToolCallingAgent
from app.state import ConversationState


class OrderAgent(ToolCallingAgent):
    agent_name = "order"
    toolset = "order"
    prompt_template = "order/system.md"

    def build_prompt_context(self, state: ConversationState) -> dict[str, object]:
        context = super().build_prompt_context(state)
        context["selected_order_id"] = state.get("selected_order_id")
        context["current_user_id"] = state.get("current_user_id")
        return context

    def initial_trace(self, state: ConversationState) -> list[str]:
        order_id = state.get("selected_order_id")
        return [f"order:{order_id}"] if order_id else []

    async def handle(self, state: ConversationState) -> dict[str, object]:
        tools = await self.get_tools()
        order_id = self.extract_order_id(state)
        current_user_id = state.get("current_user_id")

        if not order_id and current_user_id:
            list_orders_tool = self.find_tool(tools, "list_user_orders", "list_orders")
            if list_orders_tool is not None:
                orders = await list_orders_tool.ainvoke({"identifier": current_user_id})
                items = self._normalize_orders(orders)
                trace = ["tool:list_user_orders"]
                if not items:
                    return self.result(
                        state,
                        reply="当前账号下没有可查询订单。",
                        trace=trace,
                        selected_order_id=None,
                        result_summary="当前账号无订单",
                    )

                if len(items) == 1:
                    only_order = items[0]
                    reply = self._format_single_order(only_order)
                    return self.result(
                        state,
                        reply=reply,
                        trace=trace,
                        selected_order_id=only_order.get("order_id"),
                        result_summary=reply,
                    )

                reply = self._format_order_list(items)
                return self.result(
                    state,
                    reply=reply,
                    trace=trace,
                    selected_order_id=None,
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
                    trace=[*self.initial_trace(state), "tool:get_order_detail_for_service"],
                    selected_order_id=resolved_order_id,
                )

        return await super().handle(state)

    def _format_single_order(self, order: dict[str, object]) -> str:
        order_id = order.get("order_id", "未知订单")
        status = order.get("status", "unknown")
        payment_status = order.get("payment_status", "unknown")
        ticket_status = order.get("ticket_status", "unknown")
        return (
            f"订单 {order_id} 当前状态为 {status}，"
            f"支付状态为 {payment_status}，票券状态为 {ticket_status}。"
        )

    def _format_order_list(self, orders: list[dict[str, object]]) -> str:
        lines = ["当前账号下有以下订单，请选择订单号："]
        for order in orders:
            lines.append(
                f"- {order.get('order_id', '未知订单')}：状态 {order.get('status', 'unknown')}，"
                f"支付 {order.get('payment_status', 'unknown')}，票券 {order.get('ticket_status', 'unknown')}"
            )
        return "\n".join(lines)

    def _normalize_orders(self, orders: object) -> list[dict[str, object]]:
        if isinstance(orders, dict):
            raw_orders = orders.get("orders") or orders.get("list") or []
        else:
            raw_orders = orders
        if not isinstance(raw_orders, list):
            return []

        normalized_orders: list[dict[str, object]] = []
        for order in raw_orders:
            if isinstance(order, dict) and "order_id" in order:
                normalized_orders.append(order)
                continue

            text = None
            if isinstance(order, dict) and isinstance(order.get("text"), str):
                text = order["text"]
            elif isinstance(order, str):
                text = order

            if text is None:
                continue

            try:
                parsed = json.loads(text)
            except json.JSONDecodeError:
                continue
            if isinstance(parsed, dict):
                normalized_orders.append(parsed)

        return normalized_orders
