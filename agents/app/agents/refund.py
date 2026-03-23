"""Refund specialist agent."""

from app.agents.base import ToolCallingAgent
from app.state import ConversationState


class RefundAgent(ToolCallingAgent):
    agent_name = "refund"
    toolset = "refund"
    prompt_template = "refund/system.md"

    def build_prompt_context(self, state: ConversationState) -> dict[str, object]:
        context = super().build_prompt_context(state)
        context["selected_order_id"] = state.get("selected_order_id")
        return context

    def initial_trace(self, state: ConversationState) -> list[str]:
        order_id = state.get("selected_order_id")
        return [f"order:{order_id}"] if order_id else []

    async def handle(self, state: ConversationState) -> dict[str, object]:
        order_id = state.get("selected_order_id") or self.extract_order_id(state)
        if not order_id:
            return self.result(
                state,
                reply="请先提供需要处理的订单号。",
                selected_order_id=None,
                result_summary="缺少订单号",
            )

        tools = await self.get_tools()
        current_user_id = state.get("current_user_id")
        message = self.latest_user_message(state)
        wants_submit = any(keyword in message for keyword in ("帮我退款", "申请退款", "发起退款"))

        if wants_submit:
            refund_tool = self.find_tool(tools, "refund_order")
            if refund_tool is not None:
                payload = {"order_id": order_id, "reason": "用户发起退款"}
                if current_user_id is not None:
                    payload["user_id"] = current_user_id
                result = await refund_tool.ainvoke(payload)
                refund_amount = result.get("refund_amount") or result.get("refundAmount", "待确认")
                return self.result(
                    state,
                    reply=f"订单 {order_id} 已提交退款，退款金额 {refund_amount}。",
                    trace=[*self.initial_trace(state), "tool:refund_order"],
                    selected_order_id=order_id,
                    result_summary="退款申请已提交",
                )

        preview_tool = self.find_tool(tools, "preview_refund_order")
        if preview_tool is not None:
            payload = {"order_id": order_id}
            if current_user_id is not None:
                payload["user_id"] = current_user_id
            preview = await preview_tool.ainvoke(payload)
            allow_refund = preview.get("allow_refund")
            if allow_refund is None:
                allow_refund = preview.get("allowRefund")
            if allow_refund:
                refund_amount = preview.get("refund_amount") or preview.get("refundAmount", "待确认")
                refund_percent = preview.get("refund_percent") or preview.get("refundPercent", "待确认")
                return self.result(
                    state,
                    reply=f"订单 {order_id} 当前可退款，预计退款 {refund_amount}，退款比例 {refund_percent}%。",
                    trace=[*self.initial_trace(state), "tool:preview_refund_order"],
                    selected_order_id=order_id,
                    result_summary="退款资格已确认",
                )
            reject_reason = preview.get("reject_reason") or preview.get("rejectReason") or "当前订单不可退。"
            return self.result(
                state,
                reply=reject_reason,
                trace=[*self.initial_trace(state), "tool:preview_refund_order"],
                selected_order_id=order_id,
                result_summary="退款被拒绝",
            )

        return await super().handle(state)
