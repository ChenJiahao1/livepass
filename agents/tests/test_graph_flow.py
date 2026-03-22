import asyncio

from app.orchestrator.graph import GraphOrchestrator
from app.session.models import ConversationSession


class FakeProgramClient:
    async def page_programs(self, *, page_number: int = 1, page_size: int = 10):
        return {
            "list": [
                {"id": 2001, "title": "银河剧场", "showTime": "2026-04-01 19:30:00"},
            ]
        }

    async def get_program_detail(self, *, program_id: int):
        return {
            "id": program_id,
            "title": "银河剧场",
            "showTime": "2026-04-01 19:30:00",
        }


class FakeOrderClient:
    async def list_orders(self, *, user_id: int, page_number: int = 1, page_size: int = 10):
        return {
            "list": [
                {"orderNumber": 93001, "orderStatus": 3, "payStatus": 2, "ticketStatus": 3},
            ]
        }

    async def get_order_service_view(self, *, user_id: int, order_number: int):
        return {
            "orderNumber": order_number,
            "orderStatus": 3,
            "payStatus": 2,
            "ticketStatus": 3,
            "canRefund": True,
            "refundBlockedReason": "",
        }

    async def preview_refund_order(self, *, user_id: int, order_number: int):
        return {
            "orderNumber": order_number,
            "allowRefund": True,
            "refundAmount": 478,
            "refundPercent": 80,
            "rejectReason": "",
        }

    async def refund_order(self, *, user_id: int, order_number: int, reason: str):
        return {
            "orderNumber": order_number,
            "orderStatus": 4,
            "refundAmount": 478,
            "refundPercent": 80,
        }


class FakeUserClient:
    async def get_user_by_id(self, *, user_id: int):
        return {"id": user_id, "name": "测试用户"}


def _new_session() -> ConversationSession:
    return ConversationSession(conversationId="conv-graph-1", userId=3001)


def test_graph_routes_activity_order_refund_and_handoff():
    orchestrator = GraphOrchestrator(
        program_client=FakeProgramClient(),
        order_client=FakeOrderClient(),
        user_client=FakeUserClient(),
    )

    activity_result = asyncio.run(orchestrator.reply(_new_session(), message="最近有什么演出"))
    order_result = asyncio.run(orchestrator.reply(_new_session(), message="帮我查一下订单"))
    refund_preview_result = asyncio.run(
        orchestrator.reply(_new_session(), message="订单 93001 可以退款吗")
    )
    refund_submit_result = asyncio.run(
        orchestrator.reply(_new_session(), message="帮我退款订单 93001")
    )
    handoff_result = asyncio.run(orchestrator.reply(_new_session(), message="我要人工客服"))

    assert activity_result.current_agent == "activity"
    assert "银河剧场" in activity_result.reply

    assert order_result.current_agent == "order"
    assert "93001" in order_result.reply

    assert refund_preview_result.current_agent == "refund"
    assert "478" in refund_preview_result.reply

    assert refund_submit_result.current_agent == "refund"
    assert "已提交退款" in refund_submit_result.reply

    assert handoff_result.current_agent == "handoff"
    assert handoff_result.need_handoff is True
