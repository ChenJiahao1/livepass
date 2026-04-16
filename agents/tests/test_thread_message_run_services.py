from __future__ import annotations

import pytest

from app.agent_runtime.service import AgentRuntimeService
from app.messages.repository import InMemoryMessageRepository
from app.messages.service import MessageService
from app.runs.repository import InMemoryRunRepository
from app.runs.service import RunService
from app.session.store import ThreadOwnershipStore
from app.threads.repository import InMemoryThreadRepository
from app.threads.service import ThreadService
from tests.fakes import FakeRedis


class FakeAgentRuntime:
    def __init__(self) -> None:
        self.calls: list[dict] = []

    async def ainvoke(self, state_payload, config, context):
        self.calls.append({"state": state_payload, "config": config, "context": context})
        user_text = state_payload["messages"][-1]["content"]
        return {
            **state_payload,
            "final_reply": f"已处理：{user_text}",
            "current_agent": "order",
            "need_handoff": False,
            "route_source": "rule",
        }


class FailingAgentRuntime:
    def __init__(self, error: Exception) -> None:
        self.error = error
        self.calls: list[dict] = []

    async def ainvoke(self, state_payload, config, context):
        self.calls.append({"state": state_payload, "config": config, "context": context})
        raise self.error


class ServiceBundle:
    def __init__(self, *, threads: ThreadService, messages: MessageService, runs: RunService) -> None:
        self.threads = threads
        self.messages = messages
        self.runs = runs


def build_services(runtime) -> ServiceBundle:
    thread_repo = InMemoryThreadRepository()
    message_repo = InMemoryMessageRepository()
    run_repo = InMemoryRunRepository()
    ownership_store = ThreadOwnershipStore(redis_client=FakeRedis(), ttl_seconds=600, key_prefix="agents:thread")

    thread_service = ThreadService(
        thread_repository=thread_repo,
        ownership_store=ownership_store,
    )
    run_service = RunService(
        run_repository=run_repo,
        ownership_store=ownership_store,
    )
    runtime_service = AgentRuntimeService(
        agent_runtime=runtime,
        registry=object(),
        llm=object(),
    )
    message_service = MessageService(
        thread_repository=thread_repo,
        message_repository=message_repo,
        run_service=run_service,
        runtime_service=runtime_service,
        ownership_store=ownership_store,
        settings=thread_service.settings,
    )
    return ServiceBundle(threads=thread_service, messages=message_service, runs=run_service)


@pytest.mark.anyio
async def test_send_message_creates_run_calls_graph_and_projects_reply():
    runtime = FakeAgentRuntime()
    services = build_services(runtime=runtime)
    thread = services.threads.create_thread(user_id=3001, title=None)

    result = await services.messages.send_user_message(
        user_id=3001,
        thread_id=thread.id,
        parts=[{"type": "text", "text": "帮我查订单"}],
    )

    assert result.run.status == "completed"
    assert [message.role for message in result.messages] == ["user", "assistant"]
    assert result.messages[1].parts == [{"type": "text", "text": "已处理：帮我查订单"}]
    assert runtime.calls[0]["config"]["configurable"]["thread_id"] == thread.id
    assert runtime.calls[0]["context"]["current_user_id"] == "3001"


@pytest.mark.anyio
async def test_send_message_keeps_user_message_and_failed_run_when_graph_fails():
    runtime = FailingAgentRuntime(RuntimeError("模型服务暂时不可用"))
    services = build_services(runtime=runtime)
    thread = services.threads.create_thread(user_id=3001, title=None)

    result = await services.messages.send_user_message(
        user_id=3001,
        thread_id=thread.id,
        parts=[{"type": "text", "text": "帮我查订单"}],
    )

    assert result.run.status == "failed"
    assert result.run.error == {"code": "AGENT_RUN_FAILED", "message": "模型服务暂时不可用"}
    assert [message.role for message in result.messages] == ["user"]
