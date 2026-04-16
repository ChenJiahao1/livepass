from __future__ import annotations

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
from tests.fakes import FakeRedis


class FakeAgentRuntime:
    def __init__(self) -> None:
        self.calls: list[dict] = []

    async def ainvoke(self, state_payload, config, context):
        self.calls.append({"state": state_payload, "config": config, "context": context})
        message = state_payload["messages"][-1]["content"]
        return {
            **state_payload,
            "final_reply": f"已处理：{message}",
            "current_agent": "order",
            "need_handoff": False,
            "route_source": "rule",
        }


def build_thread_test_client() -> tuple[TestClient, FakeAgentRuntime]:
    agent_runtime = FakeAgentRuntime()
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

    app = create_app()
    app.dependency_overrides[get_agent_runtime] = lambda: agent_runtime
    app.dependency_overrides[get_thread_repository] = lambda: thread_repository
    app.dependency_overrides[get_message_repository] = lambda: message_repository
    app.dependency_overrides[get_run_repository] = lambda: run_repository
    app.dependency_overrides[get_event_store] = lambda: event_store
    app.dependency_overrides[get_tool_call_repository] = lambda: tool_call_repository
    app.dependency_overrides[get_thread_ownership_store] = lambda: ownership_store
    app.dependency_overrides[get_tool_registry] = lambda: object()
    app.dependency_overrides[get_llm] = lambda: object()
    return TestClient(app), agent_runtime


def test_create_run_then_fetch_history_and_run_resource():
    client, runtime = build_thread_test_client()
    response = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={"messages": [{"role": "user", "parts": [{"type": "text", "text": "帮我查订单"}]}]},
    )

    body = response.json()
    assert response.status_code == 200
    assert set(body.keys()) == {"runId", "threadId"}
    run = client.get(f"/agent/runs/{body['runId']}", headers={"X-User-Id": "3001"}).json()["run"]
    messages = client.get(
        f"/agent/threads/{body['threadId']}/messages",
        headers={"X-User-Id": "3001"},
    ).json()["messages"]
    thread = client.get(f"/agent/threads/{body['threadId']}", headers={"X-User-Id": "3001"}).json()["thread"]

    assert run["threadId"] == body["threadId"]
    assert run["status"] == "completed"
    assert [message["role"] for message in messages] == ["user", "assistant"]
    assert thread["activeRunId"] is None
    assert runtime.calls[0]["config"]["configurable"]["thread_id"] == body["threadId"]


def test_old_chat_routes_are_removed():
    client, _runtime = build_thread_test_client()

    assert client.post("/agent/chat", headers={"X-User-Id": "3001"}, json={"message": "hi"}).status_code == 404
    assert client.post("/agent/chat/stream", headers={"X-User-Id": "3001"}, json={"message": "hi"}).status_code == 404
    legacy_status = client.post(
        "/agent/threads/thr_01/messages",
        headers={"X-User-Id": "3001"},
        json={"message": {"role": "user", "parts": [{"type": "text", "text": "hi"}]}},
    ).status_code
    assert legacy_status in {404, 405}
