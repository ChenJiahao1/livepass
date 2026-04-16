from datetime import datetime, timezone

import pytest
from fastapi.testclient import TestClient
from pydantic import ValidationError

from app.api.schemas import CreateThreadResponse, SendMessageRequest, ThreadDTO
from app.api.routes import (
    get_agent_runtime,
    get_llm,
    get_message_repository,
    get_run_repository,
    get_thread_ownership_store,
    get_thread_repository,
    get_tool_registry,
)
from app.main import create_app
from app.messages.repository import InMemoryMessageRepository
from app.runs.repository import InMemoryRunRepository
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
        metadata={},
    )

    body = CreateThreadResponse(thread=thread).model_dump(by_alias=True)

    assert set(body.keys()) == {"thread"}
    assert body["thread"]["id"] == "thr_01"
    assert body["thread"]["lastMessageAt"] is None


def test_send_message_request_rejects_non_user_role():
    with pytest.raises(ValidationError):
        SendMessageRequest.model_validate(
            {"message": {"role": "assistant", "parts": [{"type": "text", "text": "x"}]}}
        )


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
    app.dependency_overrides[get_thread_ownership_store] = lambda: ownership_store
    app.dependency_overrides[get_tool_registry] = lambda: object()
    app.dependency_overrides[get_llm] = lambda: object()
    return TestClient(app), agent_runtime


def test_create_thread_allows_empty_thread_but_list_hides_it():
    client, _runtime = build_thread_test_client()

    created = client.post("/agent/threads", headers={"X-User-Id": "3001"}, json={})
    listed = client.get("/agent/threads", headers={"X-User-Id": "3001"})

    assert created.status_code == 200
    assert created.json()["thread"]["id"].startswith("thr_")
    assert created.json()["thread"]["lastMessageAt"] is None
    assert listed.status_code == 200
    assert listed.json() == {"threads": [], "nextCursor": None}


def test_get_missing_thread_returns_error_shape():
    client, _runtime = build_thread_test_client()

    response = client.get("/agent/threads/thr_missing", headers={"X-User-Id": "3001"})

    assert response.status_code == 404
    assert response.json()["detail"]["error"]["code"] == "THREAD_NOT_FOUND"


def test_get_thread_for_other_user_returns_forbidden():
    client, _runtime = build_thread_test_client()
    thread_id = client.post("/agent/threads", headers={"X-User-Id": "3001"}, json={}).json()["thread"]["id"]

    response = client.get(f"/agent/threads/{thread_id}", headers={"X-User-Id": "3002"})

    assert response.status_code == 403
    assert response.json()["detail"]["error"]["code"] == "FORBIDDEN"


def test_message_list_returns_recent_limit_ascending():
    client, _runtime = build_thread_test_client()
    thread_id = client.post("/agent/threads", headers={"X-User-Id": "3001"}, json={}).json()["thread"]["id"]
    for index in range(3):
        client.post(
            f"/agent/threads/{thread_id}/messages",
            headers={"X-User-Id": "3001"},
            json={"message": {"role": "user", "parts": [{"type": "text", "text": f"第{index}条"}]}},
        )

    response = client.get(f"/agent/threads/{thread_id}/messages?limit=2", headers={"X-User-Id": "3001"})

    assert response.status_code == 200
    assert len(response.json()["messages"]) == 2
    assert response.json()["messages"][0]["createdAt"] <= response.json()["messages"][1]["createdAt"]


def test_thread_list_supports_cursor_paging():
    client, _runtime = build_thread_test_client()

    first_thread_id = client.post("/agent/threads", headers={"X-User-Id": "3001"}, json={}).json()["thread"]["id"]
    client.post(
        f"/agent/threads/{first_thread_id}/messages",
        headers={"X-User-Id": "3001"},
        json={"message": {"role": "user", "parts": [{"type": "text", "text": "第一条"}]}},
    )

    second_thread_id = client.post("/agent/threads", headers={"X-User-Id": "3001"}, json={}).json()["thread"]["id"]
    client.post(
        f"/agent/threads/{second_thread_id}/messages",
        headers={"X-User-Id": "3001"},
        json={"message": {"role": "user", "parts": [{"type": "text", "text": "第二条"}]}},
    )

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
    assert second_page.json()["threads"][0]["id"] != first_page.json()["threads"][0]["id"]


def test_message_list_supports_before_cursor():
    client, _runtime = build_thread_test_client()
    thread_id = client.post("/agent/threads", headers={"X-User-Id": "3001"}, json={}).json()["thread"]["id"]
    for index in range(3):
        client.post(
            f"/agent/threads/{thread_id}/messages",
            headers={"X-User-Id": "3001"},
            json={"message": {"role": "user", "parts": [{"type": "text", "text": f"第{index}条"}]}},
        )

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
