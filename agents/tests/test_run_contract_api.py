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
from app.main import create_app
from app.messages.repository import InMemoryMessageRepository
from app.runs.event_store import InMemoryRunEventStore
from app.runs.repository import InMemoryRunRepository
from app.runs.tool_call_repository import InMemoryToolCallRepository
from app.threads.repository import InMemoryThreadRepository


class FakeAgentRuntime:
    def __init__(self, *, requires_action: bool = False) -> None:
        self.requires_action = requires_action

    async def ainvoke(self, state_payload, config, context):
        message = state_payload["messages"][-1]["content"]
        if self.requires_action:
            return {
                "final_reply": f"订单预览完成：{message}",
                "current_agent": "refund",
                "need_handoff": False,
                "route_source": "rule",
                "tool_call": {
                    "toolName": "human_approval",
                    "args": {
                        "action": "refund_order",
                        "orderId": "ORD-1",
                        "values": {"order_id": "ORD-1", "reason": "用户发起退款", "user_id": "3001"},
                    },
                    "request": {
                        "title": "退款前确认",
                        "description": "订单 ORD-1 预计退款 100",
                    },
                },
            }
        return {
            "final_reply": f"已处理：{message}",
            "current_agent": "order",
            "need_handoff": False,
            "route_source": "rule",
        }


class FailingAgentRuntime:
    async def ainvoke(self, state_payload, config, context):
        raise RuntimeError("runtime exploded")


def build_client(*, requires_action: bool = False, runtime=None) -> TestClient:
    thread_repository = InMemoryThreadRepository()
    message_repository = InMemoryMessageRepository()
    run_repository = InMemoryRunRepository()
    event_store = InMemoryRunEventStore()
    tool_call_repository = InMemoryToolCallRepository()
    app = create_app()
    app.dependency_overrides[get_agent_runtime] = lambda: runtime or FakeAgentRuntime(requires_action=requires_action)
    app.dependency_overrides[get_thread_repository] = lambda: thread_repository
    app.dependency_overrides[get_message_repository] = lambda: message_repository
    app.dependency_overrides[get_run_repository] = lambda: run_repository
    app.dependency_overrides[get_event_store] = lambda: event_store
    app.dependency_overrides[get_tool_call_repository] = lambda: tool_call_repository
    app.dependency_overrides[get_tool_registry] = lambda: object()
    app.dependency_overrides[get_llm] = lambda: object()
    return TestClient(app)


def create_thread(client: TestClient) -> str:
    response = client.post("/agent/threads", headers={"X-User-Id": "3001"}, json={"title": "订单咨询"})
    assert response.status_code == 200
    return response.json()["thread"]["id"]


def test_post_agent_runs_uses_thread_input_and_returns_nested_resources():
    client = build_client()
    thread_id = create_thread(client)

    response = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"parts": [{"type": "text", "text": "帮我查订单"}]},
            "metadata": {},
        },
    )

    assert response.status_code == 200
    body = response.json()
    assert body["thread"]["id"] == thread_id
    assert body["run"]["threadId"] == thread_id
    assert body["run"]["assistantMessageId"].startswith("msg_")
    assert body["acceptedMessage"]["metadata"] == {}
    assert body["assistantMessage"]["role"] == "assistant"
    assert body["assistantMessage"]["status"] == "in_progress"
    assert body["assistantMessage"]["parts"] == []


def test_create_run_request_schema_does_not_expose_client_message_id():
    client = build_client()

    response = client.get("/openapi.json")

    assert response.status_code == 200
    run_input_schema = response.json()["components"]["schemas"]["RunInputDTO"]
    assert "clientMessageId" not in run_input_schema["properties"]


def test_get_agent_runs_by_run_id_returns_new_run_resource_shape():
    client = build_client()
    thread_id = create_thread(client)
    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"parts": [{"type": "text", "text": "帮我查订单"}]},
            "metadata": {},
        },
    ).json()

    response = client.get(f"/agent/runs/{created['run']['id']}", headers={"X-User-Id": "3001"})

    assert response.status_code == 200
    assert response.json()["run"]["id"] == created["run"]["id"]
    assert response.json()["run"]["threadId"] == thread_id
    assert response.json()["run"]["assistantMessageId"] == created["assistantMessage"]["id"]


def test_get_agent_run_events_uses_new_sse_envelope_and_after_cursor():
    client = build_client(requires_action=True)
    thread_id = create_thread(client)
    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"parts": [{"type": "text", "text": "帮我退款"}]},
            "metadata": {},
        },
    ).json()

    with client.stream(
        "GET",
        f"/agent/runs/{created['run']['id']}/events",
        headers={"X-User-Id": "3001"},
        params={"after": 0},
    ) as response:
        body = "".join(response.iter_text())

    assert response.status_code == 200
    assert "event: agent.run.event" in body
    assert "schemaVersion" in body
    assert "message.delta" in body
    assert "tool_call.updated" in body
    assert "run.updated" in body


def test_get_agent_run_events_after_cursor_is_strictly_greater_than_after():
    client = build_client()
    thread_id = create_thread(client)
    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"parts": [{"type": "text", "text": "帮我查订单"}]},
            "metadata": {},
        },
    ).json()

    with client.stream(
        "GET",
        f"/agent/runs/{created['run']['id']}/events",
        headers={"X-User-Id": "3001"},
        params={"after": 12},
    ) as response:
        body = "".join(response.iter_text())

    assert response.status_code == 200
    assert "id: 12\n" not in body


def test_get_agent_run_returns_failed_after_runtime_error():
    client = build_client(runtime=FailingAgentRuntime())
    thread_id = create_thread(client)

    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"parts": [{"type": "text", "text": "帮我查订单"}]},
            "metadata": {},
        },
    ).json()

    response = client.get(f"/agent/runs/{created['run']['id']}", headers={"X-User-Id": "3001"})

    assert response.status_code == 200
    assert response.json()["run"]["status"] == "failed"
