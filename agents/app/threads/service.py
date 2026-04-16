from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone

from app.common.errors import ApiError, ApiErrorCode
from app.config import Settings, get_settings
from app.session.store import ThreadOwnershipError, ThreadOwnershipStore
from app.threads.models import THREAD_STATUS_ACTIVE, ThreadRecord
from app.threads.repository import ThreadRepository


@dataclass(slots=True)
class ThreadListResult:
    threads: list[ThreadRecord]
    next_cursor: str | None


class ThreadService:
    def __init__(
        self,
        *,
        thread_repository: ThreadRepository,
        ownership_store: ThreadOwnershipStore,
        settings: Settings | None = None,
    ) -> None:
        self.thread_repository = thread_repository
        self.ownership_store = ownership_store
        self.settings = settings or get_settings()

    def create_thread(self, *, user_id: int, title: str | None) -> ThreadRecord:
        now = datetime.now(timezone.utc)
        thread = self.thread_repository.create(
            user_id=user_id,
            title=title or self.settings.agents_thread_default_title,
            now=now,
        )
        self.ownership_store.save(thread_id=thread.id, user_id=user_id)
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
                code=ApiErrorCode.FORBIDDEN,
                message="无权访问该线程",
                http_status=403,
                details={"threadId": thread_id},
            )

        try:
            self.ownership_store.assert_owner(thread_id=thread_id, user_id=user_id)
        except ThreadOwnershipError as exc:
            raise ApiError(
                code=ApiErrorCode.FORBIDDEN,
                message="无权访问该线程",
                http_status=403,
                details={"threadId": thread_id},
            ) from exc

        return thread

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
        thread = self.get_thread(user_id=user_id, thread_id=thread_id)
        if title is not None:
            thread = self.thread_repository.update_title(thread_id=thread_id, title=title) or thread
        if status is not None:
            thread = self.thread_repository.update_status(
                thread_id=thread_id,
                status=status,
                now=datetime.now(timezone.utc),
            ) or thread
        return thread
