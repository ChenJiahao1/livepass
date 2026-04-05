"""Order specialist agent."""

import json

from app.agents.base import ToolCallingAgent
from app.state import ConversationState


class OrderAgent(ToolCallingAgent):
    agent_name = "order"
    toolset = "order"

    def initial_trace(self, state: ConversationState) -> list[str]:
        order_id = state.get("selected_order_id")
        return [f"order:{order_id}"] if order_id else []

    async def handle(self, state: ConversationState) -> dict[str, object]:
        if self.llm is not None:
            return await self._handle_with_skill_runtime(state)

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

        return self.result(
            state,
            reply="当前未获取到足够订单信息，请稍后重试。",
            trace=self.initial_trace(state),
            selected_order_id=order_id,
            result_summary="订单处理失败",
        )

    async def _handle_with_skill_runtime(self, state: ConversationState) -> dict[str, object]:
        order_id = self.extract_order_id(state)
        current_user_id = state.get("current_user_id")
        if order_id:
            result = await self.execute_skill(
                state,
                skill_id="order.get_detail",
                task_type="order_get_detail",
                goal="查询订单详情",
                required_slots=["order_id"],
                fallback_policy="return_parent",
                expected_output_schema="order_detail_v1",
                input_slots={"order_id": order_id, "user_id": current_user_id},
            )
            output = result.get("output", {})
            resolved_order_id = output.get("order_id") or output.get("orderNumber") or order_id
            status = output.get("status") or output.get("orderStatus", "未知")
            payment_status = output.get("payment_status") or output.get("payStatus", "未知")
            ticket_status = output.get("ticket_status") or output.get("ticketStatus", "未知")
            return self.result(
                state,
                reply=(
                    f"订单 {resolved_order_id} 当前状态 {status}，"
                    f"支付状态 {payment_status}，票券状态 {ticket_status}。"
                ),
                trace=[*self.initial_trace(state), *[f"tool:{name}" for name in result.get("tool_calls", [])]],
                selected_order_id=resolved_order_id,
            )

        if current_user_id:
            result = await self.execute_skill(
                state,
                skill_id="order.list_recent",
                task_type="order_list_recent",
                goal="查询最近订单",
                required_slots=[],
                fallback_policy="return_parent",
                expected_output_schema="order_list_recent_v1",
                input_slots={"user_id": current_user_id},
            )
            items = self._normalize_orders(result.get("output", {}).get("orders", []))
            trace = [f"tool:{name}" for name in result.get("tool_calls", [])]
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

        return self.result(
            state,
            reply="请先提供需要查询的订单号。",
            selected_order_id=None,
            result_summary="缺少订单号",
        )

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
