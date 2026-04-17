from __future__ import annotations

import pytest

from app.common.errors import ApiError, ApiErrorCode
from app.conversations.messages.models import MESSAGE_ROLE_ASSISTANT, MESSAGE_ROLE_USER, MESSAGE_STATUS_IN_PROGRESS
from app.conversations.messages.repository import InMemoryMessageRepository
from app.conversations.messages.service import MessageService
from app.runs.models import (
    RUN_STATUS_COMPLETED,
    RUN_STATUS_QUEUED,
    RUN_STATUS_REQUIRES_ACTION,
    RUN_STATUS_RUNNING,
)
from app.runs.repository import InMemoryRunRepository
from app.runs.execution.runtime import RunService
from app.conversations.threads.repository import InMemoryThreadRepository
from app.conversations.threads.service import ThreadService


class ServiceBundle:
    def __init__(self, *, threads: ThreadService, messages: MessageService, runs: RunService) -> None:
        self.threads = threads
        self.messages = messages
        self.runs = runs


def build_services() -> ServiceBundle:
    thread_repo = InMemoryThreadRepository()
    message_repo = InMemoryMessageRepository()
    run_repo = InMemoryRunRepository()

    message_service = MessageService(
        thread_repository=thread_repo,
        message_repository=message_repo,
    )
    run_service = RunService(
        run_repository=run_repo,
        message_service=message_service,
    )
    thread_service = ThreadService(
        thread_repository=thread_repo,
        run_repository=run_repo,
    )
    return ServiceBundle(threads=thread_service, messages=message_service, runs=run_service)


def test_create_run_persists_user_message_output_message_and_output_message_id():
    services = build_services()
    thread = services.threads.create_thread(user_id=3001, title=None)

    run, accepted_message, output_message = services.runs.create_run(
        user_id=3001,
        thread_id=thread.id,
        content=[{"type": "text", "text": "帮我查订单"}],
    )

    messages, next_cursor = services.messages.list_messages(
        user_id=3001,
        thread_id=thread.id,
        limit=20,
        before=None,
    )

    assert run.status == RUN_STATUS_QUEUED
    assert run.output_message_id == output_message.id
    assert run.metadata.get("outputMessageId") is None
    assert accepted_message.id == run.trigger_message_id
    assert getattr(accepted_message, "updated_at", None) == accepted_message.created_at
    assert [message.role for message in messages] == [MESSAGE_ROLE_USER, MESSAGE_ROLE_ASSISTANT]
    assert messages[0].content == [{"type": "text", "text": "帮我查订单"}]
    assert messages[1].status == MESSAGE_STATUS_IN_PROGRESS
    assert messages[1].content == []
    assert messages[1].run_id == run.id
    assert getattr(output_message, "updated_at", None) == output_message.created_at
    assert next_cursor is None


def test_create_user_message_uses_content_and_extract_text_rejects_non_text():
    services = build_services()
    thread = services.threads.create_thread(user_id=3001, title=None)

    message = services.messages.create_user_message(
        user_id=3001,
        thread_id=thread.id,
        content=[{"type": "text", "text": "你好"}],
    )

    assert message.content == [{"type": "text", "text": "你好"}]
    assert services.messages.extract_text([{"type": "text", "text": "  A  "}, {"type": "text", "text": "B"}]) == "A\nB"

    with pytest.raises(ApiError) as exc_info:
        services.messages.extract_text([{"type": "image", "imageUrl": "https://example.com/a.png"}])

    assert exc_info.value.code == ApiErrorCode.UNSUPPORTED_CONTENT_TYPE


def test_get_thread_returns_active_run_id_for_running_run():
    services = build_services()
    thread = services.threads.create_thread(user_id=3001, title=None)
    run, _, _ = services.runs.create_run(
        user_id=3001,
        thread_id=thread.id,
        content=[{"type": "text", "text": "帮我查订单"}],
    )

    services.runs.mark_running(run_id=run.id)
    loaded = services.threads.get_thread(user_id=3001, thread_id=thread.id)

    assert loaded.active_run_id == run.id
    assert services.runs.get_run(user_id=3001, run_id=run.id).status == RUN_STATUS_RUNNING


def test_terminal_run_cannot_resume():
    services = build_services()
    thread = services.threads.create_thread(user_id=3001, title=None)
    run, _, _ = services.runs.create_run(
        user_id=3001,
        thread_id=thread.id,
        content=[{"type": "text", "text": "帮我查订单"}],
    )
    services.runs.mark_completed(run_id=run.id, output_message_ids=[])

    with pytest.raises(ApiError) as exc_info:
        services.runs.resume_run(user_id=3001, run_id=run.id)

    assert exc_info.value.code == ApiErrorCode.RUN_NOT_ACTIVE
    assert services.runs.get_run(user_id=3001, run_id=run.id).status == RUN_STATUS_COMPLETED


def test_thread_with_active_run_cannot_create_second_run():
    services = build_services()
    thread = services.threads.create_thread(user_id=3001, title=None)
    first_run, _, _ = services.runs.create_run(
        user_id=3001,
        thread_id=thread.id,
        content=[{"type": "text", "text": "第一条"}],
    )

    with pytest.raises(ApiError) as exc_info:
        services.runs.create_run(
            user_id=3001,
            thread_id=thread.id,
            content=[{"type": "text", "text": "第二条"}],
        )

    assert exc_info.value.code == ApiErrorCode.ACTIVE_RUN_EXISTS
    assert exc_info.value.details["activeRunId"] == first_run.id


def test_requires_action_run_blocks_second_run_and_terminal_run_clears_active_run_id():
    services = build_services()
    thread = services.threads.create_thread(user_id=3001, title=None)
    run, _, _ = services.runs.create_run(
        user_id=3001,
        thread_id=thread.id,
        content=[{"type": "text", "text": "第一条"}],
    )

    services.runs.mark_requires_action(run_id=run.id)
    loaded = services.threads.get_thread(user_id=3001, thread_id=thread.id)
    assert loaded.active_run_id == run.id
    assert services.runs.get_run(user_id=3001, run_id=run.id).status == RUN_STATUS_REQUIRES_ACTION

    with pytest.raises(ApiError) as exc_info:
        services.runs.create_run(
            user_id=3001,
            thread_id=thread.id,
            content=[{"type": "text", "text": "第二条"}],
        )

    assert exc_info.value.code == ApiErrorCode.ACTIVE_RUN_EXISTS
    assert exc_info.value.details["activeRunId"] == run.id

    services.runs.mark_completed(run_id=run.id, output_message_ids=[])

    reloaded = services.threads.get_thread(user_id=3001, thread_id=thread.id)
    assert reloaded.active_run_id is None

    next_run, _, _ = services.runs.create_run(
        user_id=3001,
        thread_id=thread.id,
        content=[{"type": "text", "text": "第三条"}],
    )
    assert next_run.status == RUN_STATUS_QUEUED


def test_get_thread_hides_other_users_thread():
    services = build_services()
    thread = services.threads.create_thread(user_id=3001, title=None)

    with pytest.raises(ApiError) as exc_info:
        services.threads.get_thread(user_id=3002, thread_id=thread.id)

    assert exc_info.value.code == ApiErrorCode.THREAD_NOT_FOUND


def test_list_messages_hides_other_users_thread():
    services = build_services()
    thread = services.threads.create_thread(user_id=3001, title=None)
    services.runs.create_run(
        user_id=3001,
        thread_id=thread.id,
        content=[{"type": "text", "text": "帮我查订单"}],
    )

    with pytest.raises(ApiError) as exc_info:
        services.messages.list_messages(
            user_id=3002,
            thread_id=thread.id,
            limit=20,
            before=None,
        )

    assert exc_info.value.code == ApiErrorCode.THREAD_NOT_FOUND
