"""MCP tools for order queries."""

from langchain_core.tools import StructuredTool

ORDER_STATUS_LABELS = {
    1: "unpaid",
    2: "cancelled",
    3: "paid",
    4: "refunded",
}
PAY_STATUS_LABELS = {
    1: "unpaid",
    2: "paid",
    3: "refunded",
}
TICKET_STATUS_LABELS = {
    1: "unpaid",
    2: "cancelled",
    3: "issued",
    4: "refunded",
}


def normalize_order_status(status: int) -> str:
    return ORDER_STATUS_LABELS.get(status, f"unknown:{status}")


def normalize_pay_status(status: int) -> str:
    return PAY_STATUS_LABELS.get(status, f"unknown:{status}")


def normalize_ticket_status(status: int) -> str:
    return TICKET_STATUS_LABELS.get(status, f"unknown:{status}")


def build_list_user_orders_tool(order_client):
    async def list_user_orders(identifier: str, page_number: int = 1, page_size: int = 10):
        payload = await order_client.list_user_orders(
            identifier=identifier,
            page_number=page_number,
            page_size=page_size,
        )
        return {
            "orders": [
                {
                    "order_id": str(item["orderNumber"]),
                    "status": normalize_order_status(item["orderStatus"]),
                    "program_title": item.get("programTitle", ""),
                }
                for item in payload.get("list", [])
            ]
        }

    return StructuredTool.from_function(
        coroutine=list_user_orders,
        name="list_user_orders",
        description="按当前用户标识查询订单列表",
    )


def build_get_order_detail_for_service_tool(order_client):
    async def get_order_detail_for_service(order_id: str, user_id: str | int | None = None):
        payload = await order_client.get_order_detail_for_service(order_id=order_id, user_id=user_id)
        return {
            "order_id": str(payload["orderNumber"]),
            "status": normalize_order_status(payload["orderStatus"]),
            "payment_status": normalize_pay_status(payload["payStatus"]),
            "ticket_status": normalize_ticket_status(payload["ticketStatus"]),
            "program_title": payload.get("programTitle", ""),
            "program_show_time": payload.get("programShowTime", ""),
            "ticket_count": payload.get("ticketCount", 0),
            "order_price": payload.get("orderPrice", 0),
            "can_refund": payload.get("canRefund", False),
            "refund_blocked_reason": payload.get("refundBlockedReason", ""),
        }

    return StructuredTool.from_function(
        coroutine=get_order_detail_for_service,
        name="get_order_detail_for_service",
        description="查询客服视角的订单详情",
    )


def build_order_tools(order_client):
    return [
        build_list_user_orders_tool(order_client),
        build_get_order_detail_for_service_tool(order_client),
    ]
