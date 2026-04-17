import json

from fastapi.testclient import TestClient
from langgraph.types import Command

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


class FakePauseRuntime:
    def __init__(self, *, allowed_actions: list[str] | None = None) -> None:
        self.resume_payloads: list[dict] = []
        self.allowed_actions = allowed_actions or ["approve", "reject", "edit"]

    async def astream(self, state_payload, config, context, stream_mode):
        del config
        del context
        del stream_mode
        if isinstance(state_payload, Command):
            payload = state_payload.resume if isinstance(state_payload.resume, dict) else {}
            self.resume_payloads.append(payload)
            if payload == {"decisions": [{"type": "approve"}]}:
                yield ("messages", {"delta": "订单 ORD-1 已提交退款，退款金额 100。"})
                return
            if payload == {
                "decisions": [
                    {
                        "type": "edit",
                        "edited_action": {
                            "name": "refund_order",
                            "args": {
                                "order_id": "ORD-1",
                                "reason": "客服改成退全款",
                                "user_id": "3001",
                            },
                        },
                    }
                ]
            }:
                yield ("messages", {"delta": "订单 ORD-1 已提交退款，退款金额 100。"})
                return
            yield ("messages", {"delta": "已取消本次退款操作。"})
            return

        yield ("messages", {"delta": "订单预览完成"})
        yield (
            "updates",
            {
                "__interrupt__": (
                    _FakeInterrupt(
                        {
                            "toolName": "human_approval",
                            "args": {
                                "action": "refund_order",
                                "orderId": "ORD-1",
                                "values": {
                                    "order_id": "ORD-1",
                                    "reason": "用户发起退款",
                                    "user_id": "3001",
                                },
                            },
                            "request": {
                                "title": "退款前确认",
                                "description": "订单 ORD-1 预计退款 100",
                                "allowedActions": list(self.allowed_actions),
                            },
                        }
                    ),
                )
            },
        )

    async def ainvoke(self, state_payload, config, context):
        del state_payload
        del config
        del context
        raise AssertionError("run runtime should use astream")


class _FakeInterrupt:
    def __init__(self, value: dict) -> None:
        self.value = value


def build_client(*, allowed_actions: list[str] | None = None) -> tuple[TestClient, InMemoryToolCallRepository, FakePauseRuntime]:
    runtime = FakePauseRuntime(allowed_actions=allowed_actions)
    thread_repository = InMemoryThreadRepository()
    message_repository = InMemoryMessageRepository()
    run_repository = InMemoryRunRepository()
    event_store = InMemoryRunEventStore()
    tool_call_repository = InMemoryToolCallRepository()
    app = create_app()
    app.dependency_overrides[get_agent_runtime] = lambda: runtime
    app.dependency_overrides[get_thread_repository] = lambda: thread_repository
    app.dependency_overrides[get_message_repository] = lambda: message_repository
    app.dependency_overrides[get_run_repository] = lambda: run_repository
    app.dependency_overrides[get_event_store] = lambda: event_store
    app.dependency_overrides[get_tool_call_repository] = lambda: tool_call_repository
    app.dependency_overrides[get_tool_registry] = lambda: object()
    app.dependency_overrides[get_llm] = lambda: object()
    return TestClient(app), tool_call_repository, runtime


def create_thread(client: TestClient) -> str:
    response = client.post("/agent/threads", headers={"X-User-Id": "3001"}, json={"title": "退款咨询"})
    assert response.status_code == 200
    return response.json()["thread"]["id"]


def test_resume_waiting_human_tool_call_restarts_same_run_and_thread():
    client, tool_call_repository, runtime = build_client()
    thread_id = create_thread(client)
    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"content": [{"type": "text", "text": "我要退款"}]},
            "metadata": {},
        },
    ).json()
    tool_call_id = next(iter(tool_call_repository._tool_calls.keys()))

    response = client.post(
        f"/agent/runs/{created['run']['id']}/tool-calls/{tool_call_id}/resume",
        headers={"X-User-Id": "3001"},
        json={"action": "approve", "reason": "同意退款", "values": {}},
    )

    assert response.status_code == 200
    assert response.json()["run"]["id"] == created["run"]["id"]
    assert response.json()["run"]["threadId"] == thread_id
    assert response.json()["outputMessage"]["id"] == created["outputMessage"]["id"]
    assert runtime.resume_payloads == [{"decisions": [{"type": "approve"}]}]


def test_get_run_snapshot_returns_output_message_and_active_tool_call():
    client, tool_call_repository, _runtime = build_client()
    thread_id = create_thread(client)
    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"content": [{"type": "text", "text": "我要退款"}]},
            "metadata": {},
        },
    ).json()
    tool_call_id = next(iter(tool_call_repository._tool_calls.keys()))

    response = client.get(f"/agent/runs/{created['run']['id']}", headers={"X-User-Id": "3001"})

    assert response.status_code == 200
    body = response.json()
    assert set(body.keys()) == {"run", "outputMessage", "activeToolCall"}
    assert body["run"]["id"] == created["run"]["id"]
    assert body["outputMessage"]["id"] == created["outputMessage"]["id"]
    assert body["activeToolCall"]["id"] == tool_call_id
    assert body["activeToolCall"]["status"] == "waiting_human"
    assert body["activeToolCall"]["humanRequest"] == {
        "kind": "approval",
        "title": "退款前确认",
        "description": "订单 ORD-1 预计退款 100",
        "allowedActions": ["approve", "reject", "edit"],
    }


def test_get_run_snapshot_active_tool_call_human_request_matches_sse_waiting_human_shape():
    client, tool_call_repository, _runtime = build_client()
    thread_id = create_thread(client)
    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"content": [{"type": "text", "text": "我要退款"}]},
            "metadata": {},
        },
    ).json()
    tool_call_id = next(iter(tool_call_repository._tool_calls.keys()))

    run_snapshot = client.get(f"/agent/runs/{created['run']['id']}", headers={"X-User-Id": "3001"}).json()

    with client.stream(
        "GET",
        f"/agent/runs/{created['run']['id']}/events",
        headers={"X-User-Id": "3001"},
        params={"after": 0},
    ) as response:
        body = "".join(response.iter_text())

    assert response.status_code == 200
    waiting_human_payload = None
    for line in body.splitlines():
        if not line.startswith("data: "):
            continue
        payload = json.loads(line.removeprefix("data: "))
        if payload["type"] == "tool_call.waiting_human":
            waiting_human_payload = payload
            break

    assert waiting_human_payload is not None
    assert run_snapshot["activeToolCall"]["id"] == tool_call_id
    assert run_snapshot["activeToolCall"]["humanRequest"] == waiting_human_payload["toolCall"]["humanRequest"]


def test_cancel_completed_run_returns_run_not_active():
    client, tool_call_repository, _runtime = build_client()
    thread_id = create_thread(client)
    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"content": [{"type": "text", "text": "我要退款"}]},
            "metadata": {},
        },
    ).json()
    tool_call_id = next(iter(tool_call_repository._tool_calls.keys()))

    resume_response = client.post(
        f"/agent/runs/{created['run']['id']}/tool-calls/{tool_call_id}/resume",
        headers={"X-User-Id": "3001"},
        json={"action": "approve", "reason": "同意退款", "values": {}},
    )
    assert resume_response.status_code == 200

    cancel_response = client.post(
        f"/agent/runs/{created['run']['id']}/cancel",
        headers={"X-User-Id": "3001"},
    )

    assert cancel_response.status_code == 409
    assert cancel_response.json()["detail"]["error"]["code"] == "RUN_NOT_ACTIVE"


def test_resume_completed_run_is_idempotent_for_same_action():
    client, tool_call_repository, runtime = build_client()
    thread_id = create_thread(client)
    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"content": [{"type": "text", "text": "我要退款"}]},
            "metadata": {},
        },
    ).json()
    tool_call_id = next(iter(tool_call_repository._tool_calls.keys()))

    first_response = client.post(
        f"/agent/runs/{created['run']['id']}/tool-calls/{tool_call_id}/resume",
        headers={"X-User-Id": "3001"},
        json={"action": "approve", "reason": "同意退款", "values": {}},
    )
    assert first_response.status_code == 200

    second_response = client.post(
        f"/agent/runs/{created['run']['id']}/tool-calls/{tool_call_id}/resume",
        headers={"X-User-Id": "3001"},
        json={"action": "approve", "reason": "重复恢复", "values": {}},
    )

    assert second_response.status_code == 200
    assert second_response.json()["run"]["status"] == "completed"
    assert len(runtime.resume_payloads) == 1


def test_resume_edit_maps_to_langgraph_edited_action_payload():
    client, tool_call_repository, runtime = build_client()
    thread_id = create_thread(client)
    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"content": [{"type": "text", "text": "我要退款"}]},
            "metadata": {},
        },
    ).json()
    tool_call_id = next(iter(tool_call_repository._tool_calls.keys()))

    response = client.post(
        f"/agent/runs/{created['run']['id']}/tool-calls/{tool_call_id}/resume",
        headers={"X-User-Id": "3001"},
        json={"action": "edit", "reason": "改参数", "values": {"reason": "客服改成退全款"}},
    )

    assert response.status_code == 200
    assert runtime.resume_payloads[-1] == {
        "decisions": [
            {
                "type": "edit",
                "edited_action": {
                    "name": "refund_order",
                    "args": {
                        "order_id": "ORD-1",
                        "reason": "客服改成退全款",
                        "user_id": "3001",
                    },
                },
            }
        ]
    }


def test_resume_rejects_action_not_in_allowed_actions():
    client, tool_call_repository, _runtime = build_client(allowed_actions=["approve", "reject"])
    thread_id = create_thread(client)
    created = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"content": [{"type": "text", "text": "我要退款"}]},
            "metadata": {},
        },
    ).json()
    tool_call_id = next(iter(tool_call_repository._tool_calls.keys()))

    response = client.post(
        f"/agent/runs/{created['run']['id']}/tool-calls/{tool_call_id}/resume",
        headers={"X-User-Id": "3001"},
        json={"action": "edit", "reason": "不允许编辑", "values": {"reason": "x"}},
    )

    assert response.status_code == 409
    assert response.json()["detail"]["error"]["code"] == "TOOL_CALL_DECISION_NOT_ALLOWED"


def test_create_run_rejects_second_active_run_with_conflict_details():
    client, _tool_call_repository, _runtime = build_client()
    thread_id = create_thread(client)

    first_response = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"content": [{"type": "text", "text": "第一次发起"}]},
            "metadata": {},
        },
    )
    assert first_response.status_code == 200
    first_run_id = first_response.json()["run"]["id"]

    second_response = client.post(
        "/agent/runs",
        headers={"X-User-Id": "3001"},
        json={
            "threadId": thread_id,
            "input": {"content": [{"type": "text", "text": "第二次发起"}]},
            "metadata": {},
        },
    )

    assert second_response.status_code == 409
    assert second_response.json()["detail"]["error"]["code"] == "ACTIVE_RUN_EXISTS"
    assert second_response.json()["detail"]["error"]["details"]["threadId"] == thread_id
    assert second_response.json()["detail"]["error"]["details"]["activeRunId"] == first_run_id
