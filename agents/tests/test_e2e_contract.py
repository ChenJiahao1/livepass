from __future__ import annotations

from fastapi.testclient import TestClient

from app.api.routes import (
    get_agent_runtime,
    get_event_store,
    get_llm,
    get_message_repository,
    get_run_repository,
    get_thread_repository,
    get_tool_call_repository,
    get_tool_registry,
)
from app.api.app import create_app
from app.conversations.messages.repository import InMemoryMessageRepository
from app.runs.event_store import InMemoryRunEventStore
from app.runs.repository import InMemoryRunRepository
from app.runs.tool_call_repository import InMemoryToolCallRepository
from app.conversations.threads.repository import InMemoryThreadRepository


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
    app = create_app()
    app.dependency_overrides[get_agent_runtime] = lambda: agent_runtime
    app.dependency_overrides[get_thread_repository] = lambda: thread_repository
    app.dependency_overrides[get_message_repository] = lambda: message_repository
    app.dependency_overrides[get_run_repository] = lambda: run_repository
    app.dependency_overrides[get_event_store] = lambda: event_store
    app.dependency_overrides[get_tool_call_repository] = lambda: tool_call_repository
    app.dependency_overrides[get_tool_registry] = lambda: object()
    app.dependency_overrides[get_llm] = lambda: object()
    return TestClient(app), agent_runtime


def test_create_thread_run_fetch_history_and_run_resource():
    client, runtime = build_thread_test_client()
    thread = client.post(
        "/agent/threads",
        headers={"X-User-Id": "3001"},
        json={"title": "订单咨询"},
    ).json()["thread"]

    response = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread["id"],
            "input": {"content": [{"type": "text", "text": "帮我查订单"}]},
            "metadata": {},
        },
    )

    body = response.json()
    assert response.status_code == 200
    assert set(body.keys()) == {"thread", "run", "inputMessage", "outputMessage"}
    run_body = client.get(f"/agent/runs/{body['run']['id']}", headers={"X-User-Id": "3001"}).json()
    run = run_body["run"]
    messages = client.get(
        f"/agent/threads/{thread['id']}/messages",
        headers={"X-User-Id": "3001"},
    ).json()["messages"]
    thread_view = client.get(f"/agent/threads/{thread['id']}", headers={"X-User-Id": "3001"}).json()["thread"]

    assert run["threadId"] == thread["id"]
    assert run["outputMessageId"] == body["outputMessage"]["id"]
    assert run_body["outputMessage"]["id"] == body["outputMessage"]["id"]
    assert [message["role"] for message in messages] == ["user", "assistant"]
    assert messages[0]["content"] == [{"type": "text", "text": "帮我查订单"}]
    assert thread_view["activeRunId"] is None
    assert runtime.calls[0]["config"]["configurable"]["thread_id"] == thread["id"]


def test_old_chat_routes_are_removed():
    client, _runtime = build_thread_test_client()

    assert client.post("/agent/chat", headers={"X-User-Id": "3001"}, json={"message": "hi"}).status_code == 404
    assert client.post("/agent/chat/stream", headers={"X-User-Id": "3001"}, json={"message": "hi"}).status_code == 404
    legacy_status = client.post(
        "/agent/threads/thr_01/messages",
        headers={"X-User-Id": "3001"},
        json={"message": {"role": "user", "content": [{"type": "text", "text": "hi"}]}},
    ).status_code
    assert legacy_status in {404, 405}
