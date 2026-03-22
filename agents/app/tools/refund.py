"""Refund tools backed by order-rpc."""

from langchain_core.tools import StructuredTool


def build_refund_tools(order_client):
    async def preview_refund_order(user_id: int, order_number: int):
        return await order_client.preview_refund_order(
            user_id=user_id,
            order_number=order_number,
        )

    async def refund_order(user_id: int, order_number: int, reason: str):
        return await order_client.refund_order(
            user_id=user_id,
            order_number=order_number,
            reason=reason,
        )

    return [
        StructuredTool.from_function(
            coroutine=preview_refund_order,
            name="preview_refund_order",
            description="退款预检",
        ),
        StructuredTool.from_function(
            coroutine=refund_order,
            name="refund_order",
            description="发起退款",
        ),
    ]
