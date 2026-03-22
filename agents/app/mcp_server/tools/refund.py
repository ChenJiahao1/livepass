"""MCP tools for refund flows."""

from langchain_core.tools import StructuredTool

from app.mcp_server.tools.order import normalize_order_status


def build_preview_refund_order_tool(order_client):
    async def preview_refund_order(order_id: str, user_id: str | int | None = None):
        payload = await order_client.preview_refund_order(order_id=order_id, user_id=user_id)
        return {
            "order_id": str(payload["orderNumber"]),
            "allow_refund": payload["allowRefund"],
            "refund_amount": payload["refundAmount"],
            "refund_percent": payload["refundPercent"],
            "reject_reason": payload["rejectReason"],
        }

    return StructuredTool.from_function(
        coroutine=preview_refund_order,
        name="preview_refund_order",
        description="预览订单退款结果",
    )


def build_refund_order_tool(order_client):
    async def refund_order(order_id: str, user_id: str | int | None = None, reason: str = "用户发起退款"):
        payload = await order_client.refund_order(order_id=order_id, user_id=user_id, reason=reason)
        return {
            "order_id": str(payload["orderNumber"]),
            "status": normalize_order_status(payload["orderStatus"]),
            "refund_amount": payload["refundAmount"],
            "refund_percent": payload["refundPercent"],
            "refund_bill_no": payload.get("refundBillNo", ""),
            "refund_time": payload.get("refundTime", ""),
        }

    return StructuredTool.from_function(
        coroutine=refund_order,
        name="refund_order",
        description="提交订单退款申请",
    )


def build_refund_tools(order_client):
    return [
        build_preview_refund_order_tool(order_client),
        build_refund_order_tool(order_client),
    ]
