from __future__ import annotations

import pytest

from app.common.errors import ApiError, ApiErrorCode
from app.messages.models import MESSAGE_ROLE_ASSISTANT, MESSAGE_ROLE_USER, MESSAGE_STATUS_IN_PROGRESS
from app.messages.repository import InMemoryMessageRepository
from app.messages.service import MessageService
from app.runs.models import RUN_STATUS_COMPLETED, RUN_STATUS_QUEUED, RUN_STATUS_RUNNING
from app.runs.repository import InMemoryRunRepository
from app.runs.service import RunService
from app.session.store import ThreadOwnershipStore
from app.threads.repository import InMemoryThreadRepository
from app.threads.service import ThreadService
from tests.fakes import FakeRedis


class ServiceBundle:
    def __init__(self, *, threads: ThreadService, messages: MessageService, runs: RunService) -> None:
        self.threads = threads
        self.messages = messages
        self.runs = runs


def build_services() -> ServiceBundle:
    thread_repo = InMemoryThreadRepository()
    message_repo = InMemoryMessageRepository()
    run_repo = InMemoryRunRepository()
    ownership_store = ThreadOwnershipStore(redis_client=FakeRedis(), ttl_seconds=600, key_prefix="agents:thread")

    message_service = MessageService(
        thread_repository=thread_repo,
        message_repository=message_repo,
        ownership_store=ownership_store,
    )
    run_service = RunService(
        run_repository=run_repo,
        message_service=message_service,
        ownership_store=ownership_store,
    )
    thread_service = ThreadService(
        thread_repository=thread_repo,
        ownership_store=ownership_store,
        run_repository=run_repo,
    )
    return ServiceBundle(threads=thread_service, messages=message_service, runs=run_service)


def test_create_run_persists_user_message_and_queued_run():
    services = build_services()
    thread = services.threads.create_thread(user_id=3001, title=None)

    run = services.runs.create_run(
        user_id=3001,
        thread_id=thread.id,
        parts=[{"type": "text", "text": "帮我查订单"}],
    )

    messages, next_cursor = services.messages.list_messages(
        user_id=3001,
        thread_id=thread.id,
        limit=20,
        before=None,
    )

    assert run.status == RUN_STATUS_QUEUED
    assert run.metadata["assistantMessageId"] == messages[1].id
    assert [message.role for message in messages] == [MESSAGE_ROLE_USER, MESSAGE_ROLE_ASSISTANT]
    assert messages[1].status == MESSAGE_STATUS_IN_PROGRESS
    assert messages[1].parts == []
    assert messages[1].run_id == run.id
    assert next_cursor is None


def test_get_thread_returns_active_run_id_for_running_run():
    services = build_services()
    thread = services.threads.create_thread(user_id=3001, title=None)
    run = services.runs.create_run(
        user_id=3001,
        thread_id=thread.id,
        parts=[{"type": "text", "text": "帮我查订单"}],
    )

    services.runs.mark_running(run_id=run.id)
    loaded = services.threads.get_thread(user_id=3001, thread_id=thread.id)

    assert loaded.active_run_id == run.id
    assert services.runs.get_run(user_id=3001, run_id=run.id).status == RUN_STATUS_RUNNING


def test_terminal_run_cannot_resume():
    services = build_services()
    thread = services.threads.create_thread(user_id=3001, title=None)
    run = services.runs.create_run(
        user_id=3001,
        thread_id=thread.id,
        parts=[{"type": "text", "text": "帮我查订单"}],
    )
    services.runs.mark_completed(run_id=run.id, output_message_ids=[])

    with pytest.raises(ApiError) as exc_info:
        services.runs.resume_run(user_id=3001, run_id=run.id)

    assert exc_info.value.code == ApiErrorCode.RUN_STATE_INVALID
    assert services.runs.get_run(user_id=3001, run_id=run.id).status == RUN_STATUS_COMPLETED


def test_thread_with_active_run_cannot_create_second_run():
    services = build_services()
    thread = services.threads.create_thread(user_id=3001, title=None)
    services.runs.create_run(
        user_id=3001,
        thread_id=thread.id,
        parts=[{"type": "text", "text": "第一条"}],
    )

    with pytest.raises(ApiError) as exc_info:
        services.runs.create_run(
            user_id=3001,
            thread_id=thread.id,
            parts=[{"type": "text", "text": "第二条"}],
        )

    assert exc_info.value.code == ApiErrorCode.RUN_STATE_INVALID
