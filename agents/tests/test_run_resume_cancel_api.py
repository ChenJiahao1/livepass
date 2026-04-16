from fastapi.testclient import TestClient

from app.api.routes import (
    get_agent_runtime,
    get_event_store,
    get_llm,
    get_message_repository,
    get_run_repository,
    get_thread_ownership_store,
    get_thread_repository,
    get_tool_call_repository,
    get_tool_registry,
)
from app.main import create_app
from app.messages.repository import InMemoryMessageRepository
from app.runs.event_store import InMemoryRunEventStore
from app.runs.repository import InMemoryRunRepository
from app.runs.tool_call_repository import InMemoryToolCallRepository
from app.session.store import ThreadOwnershipStore
from app.threads.repository import InMemoryThreadRepository
from tests.fakes import FakeRedis, StubRegistry, build_async_tool


class FakePauseRuntime:
    async def ainvoke(self, state_payload, config, context):
        return {
            "final_reply": "订单预览完成",
            "current_agent": "refund",
            "need_handoff": False,
            "route_source": "rule",
            "tool_call": {
                "toolName": "human_approval",
                "arguments": {
                    "action": "refund_order",
                    "orderId": "ORD-1",
                    "values": {"order_id": "ORD-1", "reason": "用户发起退款", "user_id": "3001"},
                },
            },
        }


def build_client() -> tuple[TestClient, InMemoryToolCallRepository]:
    thread_repository = InMemoryThreadRepository()
    message_repository = InMemoryMessageRepository()
    run_repository = InMemoryRunRepository()
    event_store = InMemoryRunEventStore()
    tool_call_repository = InMemoryToolCallRepository()
    ownership_store = ThreadOwnershipStore(
        redis_client=FakeRedis(),
        ttl_seconds=600,
        key_prefix="agents:thread",
    )

    async def _refund_order(order_id: str, reason: str | None = None, user_id: str | None = None):
        return {"order_id": order_id, "accepted": True, "refund_amount": "100"}

    app = create_app()
    app.dependency_overrides[get_agent_runtime] = lambda: FakePauseRuntime()
    app.dependency_overrides[get_thread_repository] = lambda: thread_repository
    app.dependency_overrides[get_message_repository] = lambda: message_repository
    app.dependency_overrides[get_run_repository] = lambda: run_repository
    app.dependency_overrides[get_event_store] = lambda: event_store
    app.dependency_overrides[get_tool_call_repository] = lambda: tool_call_repository
    app.dependency_overrides[get_thread_ownership_store] = lambda: ownership_store
    app.dependency_overrides[get_tool_registry] = lambda: StubRegistry(
        tools_by_toolset={
            "refund": [
                build_async_tool(name="refund_order", description="refund order", coroutine=_refund_order),
            ]
        }
    )
    app.dependency_overrides[get_llm] = lambda: object()
    return TestClient(app), tool_call_repository


def test_resume_waiting_human_tool_call_restarts_same_run():
    client, tool_call_repository = build_client()
    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={"messages": [{"role": "user", "parts": [{"type": "text", "text": "我要退款"}]}]},
    ).json()
    tool_call_id = next(iter(tool_call_repository._tool_calls.keys()))

    response = client.post(
        f"/agent/runs/{created['runId']}/tool-calls/{tool_call_id}/resume",
        headers={"X-User-Id": "3001"},
        json={"action": "approve", "reason": "同意退款", "values": {}},
    )

    assert response.status_code == 200
    assert response.json()["run"]["id"] == created["runId"]
    assert response.json()["run"]["status"] == "completed"


def test_cancel_requires_action_run_returns_cancelled():
    client, _tool_call_repository = build_client()
    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={"messages": [{"role": "user", "parts": [{"type": "text", "text": "我要退款"}]}]},
    ).json()

    response = client.post(
        f"/agent/runs/{created['runId']}/cancel",
        headers={"X-User-Id": "3001"},
    )

    assert response.status_code == 200
    assert response.json()["run"]["status"] == "cancelled"


def test_resume_with_foreign_tool_call_returns_client_error():
    client, tool_call_repository = build_client()
    first = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={"messages": [{"role": "user", "parts": [{"type": "text", "text": "我要退款 A"}]}]},
    ).json()
    second = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={"messages": [{"role": "user", "parts": [{"type": "text", "text": "我要退款 B"}]}]},
    ).json()
    tool_call_ids = list(tool_call_repository._tool_calls.keys())

    response = client.post(
        f"/agent/runs/{first['runId']}/tool-calls/{tool_call_ids[-1]}/resume",
        headers={"X-User-Id": "3001"},
        json={"action": "approve", "reason": "错单恢复", "values": {}},
    )

    assert response.status_code in {400, 404, 409}


def test_resume_completed_run_returns_conflict_not_500():
    client, tool_call_repository = build_client()
    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={"messages": [{"role": "user", "parts": [{"type": "text", "text": "我要退款"}]}]},
    ).json()
    tool_call_id = next(iter(tool_call_repository._tool_calls.keys()))

    first_response = client.post(
        f"/agent/runs/{created['runId']}/tool-calls/{tool_call_id}/resume",
        headers={"X-User-Id": "3001"},
        json={"action": "approve", "reason": "同意退款", "values": {}},
    )
    assert first_response.status_code == 200

    second_response = client.post(
        f"/agent/runs/{created['runId']}/tool-calls/{tool_call_id}/resume",
        headers={"X-User-Id": "3001"},
        json={"action": "approve", "reason": "重复恢复", "values": {}},
    )

    assert second_response.status_code == 409
