from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone

from app.shared.errors import ApiError, ApiErrorCode
from app.shared.config import Settings, get_settings
from app.runs.repository import RunRepository
from app.conversations.threads.models import THREAD_STATUS_ACTIVE, ThreadRecord
from app.conversations.threads.repository import ThreadRepository


@dataclass(slots=True)
class ThreadListResult:
    threads: list[ThreadRecord]
    next_cursor: str | None


class ThreadService:
    def __init__(
        self,
        *,
        thread_repository: ThreadRepository,
        run_repository: RunRepository | None = None,
        settings: Settings | None = None,
    ) -> None:
        self.thread_repository = thread_repository
        self.run_repository = run_repository
        self.settings = settings or get_settings()

    def create_thread(self, *, user_id: int, title: str | None) -> ThreadRecord:
        now = datetime.now(timezone.utc)
        thread = self.thread_repository.create(
            user_id=user_id,
            title=title or self.settings.agents_thread_default_title,
            now=now,
        )
        return thread

    def list_threads(
        self,
        *,
        user_id: int,
        status: str = THREAD_STATUS_ACTIVE,
        limit: int = 20,
        cursor: str | None = None,
        include_empty: bool = False,
    ) -> ThreadListResult:
        threads, next_cursor = self.thread_repository.list_by_user(
            user_id=user_id,
            status=status,
            limit=limit,
            cursor=cursor,
            include_empty=include_empty,
        )
        threads = [self._with_active_run(thread) for thread in threads]
        return ThreadListResult(threads=threads, next_cursor=next_cursor)

    def get_thread(self, *, user_id: int, thread_id: str) -> ThreadRecord:
        thread = self.thread_repository.find_by_id(thread_id=thread_id)
        if thread is None:
            raise ApiError(
                code=ApiErrorCode.THREAD_NOT_FOUND,
                message="线程不存在",
                http_status=404,
                details={"threadId": thread_id},
            )

        if thread.user_id != user_id:
            raise ApiError(
                code=ApiErrorCode.THREAD_NOT_FOUND,
                message="线程不存在",
                http_status=404,
                details={"threadId": thread_id},
            )

        return self._with_active_run(thread)

    def update_title_from_first_message(self, *, thread: ThreadRecord, text: str) -> ThreadRecord:
        title = text.strip()[: self.settings.agents_thread_title_max_length] or self.settings.agents_thread_default_title
        updated = self.thread_repository.update_title(thread_id=thread.id, title=title)
        return updated or thread

    def patch_thread(
        self,
        *,
        user_id: int,
        thread_id: str,
        title: str | None,
        status: str | None,
    ) -> ThreadRecord:
        return self.update_thread(
            user_id=user_id,
            thread_id=thread_id,
            title=title,
            status=status,
        )

    def update_thread(
        self,
        *,
        user_id: int,
        thread_id: str,
        title: str | None,
        status: str | None,
    ) -> ThreadRecord:
        thread = self.get_thread(user_id=user_id, thread_id=thread_id)
        if title is not None:
            thread = self.thread_repository.update_title(thread_id=thread_id, title=title) or thread
        if status is not None:
            thread = self.thread_repository.update_status(
                thread_id=thread_id,
                status=status,
                now=datetime.now(timezone.utc),
            ) or thread
        return self._with_active_run(thread)

    def _with_active_run(self, thread: ThreadRecord) -> ThreadRecord:
        if self.run_repository is None:
            return thread
        active_run = self.run_repository.find_active_by_thread(thread_id=thread.id)
        thread.active_run_id = active_run.id if active_run else None
        return thread
