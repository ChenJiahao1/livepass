import pytest

from app.common.errors import ApiError
from app.runs.repository import InMemoryRunRepository
from app.conversations.threads.repository import InMemoryThreadRepository
from app.conversations.threads.service import ThreadService


def test_thread_ownership_uses_thread_repository_user_id():
    repository = InMemoryThreadRepository()
    service = ThreadService(thread_repository=repository, run_repository=InMemoryRunRepository())

    thread = service.create_thread(user_id=3001, title="订单咨询")

    assert service.get_thread(user_id=3001, thread_id=thread.id).id == thread.id


def test_thread_ownership_rejects_other_user_from_repository_record():
    repository = InMemoryThreadRepository()
    service = ThreadService(thread_repository=repository, run_repository=InMemoryRunRepository())
    thread = service.create_thread(user_id=3001, title="订单咨询")

    with pytest.raises(ApiError) as exc_info:
        service.get_thread(user_id=3002, thread_id=thread.id)

    assert exc_info.value.code == "THREAD_NOT_FOUND"
