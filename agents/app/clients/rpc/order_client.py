"""Thin async wrapper around order-rpc stubs."""

from app.clients.rpc import generated
from app.clients.rpc.channel import create_rpc_channel


class OrderRpcClient:
    def __init__(self, *, target: str, timeout_seconds: float = 5.0) -> None:
        self.timeout_seconds = timeout_seconds
        self.channel = create_rpc_channel(target)
        self.stub = generated.order_pb2_grpc.OrderRpcStub(self.channel)

    async def list_orders(self, *, user_id: int, page_number: int = 1, page_size: int = 10):
        request = generated.order_pb2.ListOrdersReq(
            userId=user_id,
            pageNumber=page_number,
            pageSize=page_size,
        )
        response = await self.stub.ListOrders(request, timeout=self.timeout_seconds)
        return {
            "pageNum": response.pageNum,
            "pageSize": response.pageSize,
            "totalSize": response.totalSize,
            "list": [
                {
                    "orderNumber": item.orderNumber,
                    "orderStatus": item.orderStatus,
                    "programTitle": item.programTitle,
                }
                for item in response.list
            ],
        }

    async def get_order_service_view(self, *, user_id: int, order_number: int):
        request = generated.order_pb2.GetOrderServiceViewReq(
            userId=user_id,
            orderNumber=order_number,
        )
        response = await self.stub.GetOrderServiceView(request, timeout=self.timeout_seconds)
        return {
            "orderNumber": response.orderNumber,
            "orderStatus": response.orderStatus,
            "payStatus": response.payStatus,
            "ticketStatus": response.ticketStatus,
            "programTitle": response.programTitle,
            "programShowTime": response.programShowTime,
            "ticketCount": response.ticketCount,
            "orderPrice": response.orderPrice,
            "canRefund": response.canRefund,
            "refundBlockedReason": response.refundBlockedReason,
        }

    async def preview_refund_order(self, *, user_id: int, order_number: int):
        request = generated.order_pb2.PreviewRefundOrderReq(
            userId=user_id,
            orderNumber=order_number,
        )
        response = await self.stub.PreviewRefundOrder(request, timeout=self.timeout_seconds)
        return {
            "orderNumber": response.orderNumber,
            "allowRefund": response.allowRefund,
            "refundAmount": response.refundAmount,
            "refundPercent": response.refundPercent,
            "rejectReason": response.rejectReason,
        }

    async def refund_order(self, *, user_id: int, order_number: int, reason: str):
        request = generated.order_pb2.RefundOrderReq(
            userId=user_id,
            orderNumber=order_number,
            reason=reason,
        )
        response = await self.stub.RefundOrder(request, timeout=self.timeout_seconds)
        return {
            "orderNumber": response.orderNumber,
            "orderStatus": response.orderStatus,
            "refundAmount": response.refundAmount,
            "refundPercent": response.refundPercent,
            "refundBillNo": response.refundBillNo,
            "refundTime": response.refundTime,
        }
