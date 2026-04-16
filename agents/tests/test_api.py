from datetime import datetime, timezone

import pytest
from fastapi.testclient import TestClient
from pydantic import ValidationError

from app.api.schemas import CreateRunRequest, CreateThreadResponse, ThreadDTO
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


def test_create_thread_response_uses_thread_resource_shape():
    thread = ThreadDTO(
        id="thr_01",
        title="新会话",
        status="active",
        created_at=datetime(2026, 4, 16, 10, 0, tzinfo=timezone.utc),
        updated_at=datetime(2026, 4, 16, 10, 0, tzinfo=timezone.utc),
        last_message_at=None,
        active_run_id=None,
        metadata={},
    )

    body = CreateThreadResponse(thread=thread).model_dump(by_alias=True)

    assert set(body.keys()) == {"thread"}
    assert body["thread"]["id"] == "thr_01"
    assert body["thread"]["lastMessageAt"] is None
    assert body["thread"]["activeRunId"] is None


def test_create_run_request_rejects_non_user_role():
    with pytest.raises(ValidationError):
        CreateRunRequest.model_validate(
            {"messages": [{"role": "assistant", "parts": [{"type": "text", "text": "x"}]}]}
        )


class FakeAgentRuntime:
    async def ainvoke(self, state_payload, config, context):
        message = state_payload["messages"][-1]["content"]
        return {
            **state_payload,
            "final_reply": f"已处理：{message}",
            "current_agent": "order",
            "need_handoff": False,
            "route_source": "rule",
        }


def build_thread_test_client() -> TestClient:
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
    app.dependency_overrides[get_agent_runtime] = lambda: FakeAgentRuntime()
    app.dependency_overrides[get_thread_repository] = lambda: thread_repository
    app.dependency_overrides[get_message_repository] = lambda: message_repository
    app.dependency_overrides[get_run_repository] = lambda: run_repository
    app.dependency_overrides[get_event_store] = lambda: event_store
    app.dependency_overrides[get_tool_call_repository] = lambda: tool_call_repository
    app.dependency_overrides[get_thread_ownership_store] = lambda: ownership_store
    app.dependency_overrides[get_tool_registry] = lambda: object()
    app.dependency_overrides[get_llm] = lambda: object()
    return TestClient(app)


def create_run(client: TestClient, *, user_id: str, thread_id: str | None, text: str) -> dict:
    body = {"messages": [{"role": "user", "parts": [{"type": "text", "text": text}]}]}
    if thread_id is not None:
        body["threadId"] = thread_id
    response = client.post("/agent/runs", headers={"X-User-Id": user_id}, json=body)
    assert response.status_code == 200
    return response.json()


def test_create_thread_allows_empty_thread_but_list_hides_it():
    client = build_thread_test_client()

    created = client.post("/agent/threads", headers={"X-User-Id": "3001"}, json={})
    listed = client.get("/agent/threads", headers={"X-User-Id": "3001"})

    assert created.status_code == 200
    assert created.json()["thread"]["id"].startswith("thr_")
    assert created.json()["thread"]["lastMessageAt"] is None
    assert listed.status_code == 200
    assert listed.json() == {"threads": [], "nextCursor": None}


def test_get_missing_thread_returns_error_shape():
    client = build_thread_test_client()

    response = client.get("/agent/threads/thr_missing", headers={"X-User-Id": "3001"})

    assert response.status_code == 404
    assert response.json()["detail"]["error"]["code"] == "THREAD_NOT_FOUND"


def test_get_thread_for_other_user_returns_forbidden():
    client = build_thread_test_client()
    thread_id = client.post("/agent/threads", headers={"X-User-Id": "3001"}, json={}).json()["thread"]["id"]

    response = client.get(f"/agent/threads/{thread_id}", headers={"X-User-Id": "3002"})

    assert response.status_code == 403
    assert response.json()["detail"]["error"]["code"] == "FORBIDDEN"


def test_message_list_returns_recent_limit_ascending():
    client = build_thread_test_client()
    first = create_run(client, user_id="3001", thread_id=None, text="第0条")
    thread_id = first["threadId"]
    create_run(client, user_id="3001", thread_id=thread_id, text="第1条")
    create_run(client, user_id="3001", thread_id=thread_id, text="第2条")

    response = client.get(f"/agent/threads/{thread_id}/messages?limit=2", headers={"X-User-Id": "3001"})

    assert response.status_code == 200
    assert len(response.json()["messages"]) == 2
    assert response.json()["messages"][0]["createdAt"] <= response.json()["messages"][1]["createdAt"]


def test_thread_list_supports_cursor_paging():
    client = build_thread_test_client()

    first_thread_id = create_run(client, user_id="3001", thread_id=None, text="第一条")["threadId"]
    second_thread_id = create_run(client, user_id="3001", thread_id=None, text="第二条")["threadId"]

    first_page = client.get("/agent/threads?limit=1", headers={"X-User-Id": "3001"})

    assert first_page.status_code == 200
    assert len(first_page.json()["threads"]) == 1
    assert first_page.json()["nextCursor"] is not None

    second_page = client.get(
        "/agent/threads",
        headers={"X-User-Id": "3001"},
        params={"limit": 1, "cursor": first_page.json()["nextCursor"]},
    )

    assert second_page.status_code == 200
    assert len(second_page.json()["threads"]) == 1
    assert {first_thread_id, second_thread_id} == {
        first_page.json()["threads"][0]["id"],
        second_page.json()["threads"][0]["id"],
    }


def test_message_list_supports_before_cursor():
    client = build_thread_test_client()
    first = create_run(client, user_id="3001", thread_id=None, text="第0条")
    thread_id = first["threadId"]
    create_run(client, user_id="3001", thread_id=thread_id, text="第1条")
    create_run(client, user_id="3001", thread_id=thread_id, text="第2条")

    first_page = client.get(f"/agent/threads/{thread_id}/messages?limit=2", headers={"X-User-Id": "3001"})

    assert first_page.status_code == 200
    assert len(first_page.json()["messages"]) == 2
    assert first_page.json()["nextCursor"] is not None

    second_page = client.get(
        f"/agent/threads/{thread_id}/messages",
        headers={"X-User-Id": "3001"},
        params={"limit": 2, "before": first_page.json()["nextCursor"]},
    )

    assert second_page.status_code == 200
    assert len(second_page.json()["messages"]) >= 1
    assert second_page.json()["messages"][-1]["id"] != first_page.json()["messages"][-1]["id"]
