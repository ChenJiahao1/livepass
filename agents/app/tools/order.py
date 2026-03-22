"""Order tools backed by order-rpc."""

from langchain_core.tools import StructuredTool


def build_order_tools(order_client):
    async def list_orders(user_id: int, page_number: int = 1, page_size: int = 10):
        return await order_client.list_orders(
            user_id=user_id,
            page_number=page_number,
            page_size=page_size,
        )

    async def get_order_service_view(user_id: int, order_number: int):
        return await order_client.get_order_service_view(
            user_id=user_id,
            order_number=order_number,
        )

    return [
        StructuredTool.from_function(
            coroutine=list_orders,
            name="list_orders",
            description="查询当前用户订单列表",
        ),
        StructuredTool.from_function(
            coroutine=get_order_service_view,
            name="get_order_service_view",
            description="查询订单客服视图",
        ),
    ]
