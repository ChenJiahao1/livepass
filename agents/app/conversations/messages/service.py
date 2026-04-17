from __future__ import annotations

from datetime import datetime, timezone

from app.shared.errors import ApiError, ApiErrorCode
from app.shared.ids import new_message_id
from app.shared.config import Settings, get_settings
from app.conversations.messages.models import (
    MESSAGE_ROLE_ASSISTANT,
    MESSAGE_ROLE_USER,
    MESSAGE_STATUS_COMPLETED,
    MESSAGE_STATUS_IN_PROGRESS,
    MessageRecord,
)
from app.conversations.messages.repository import MessageRepository
from app.conversations.threads.models import ThreadRecord
from app.conversations.threads.repository import ThreadRepository


class MessageService:
    def __init__(
        self,
        *,
        thread_repository: ThreadRepository,
        message_repository: MessageRepository,
        settings: Settings | None = None,
    ) -> None:
        self.thread_repository = thread_repository
        self.message_repository = message_repository
        self.settings = settings or get_settings()

    def list_messages(self, *, user_id: int, thread_id: str, limit: int, before: str | None):
        return self.list_thread_messages(
            user_id=user_id,
            thread_id=thread_id,
            limit=limit,
            before=before,
        )

    def list_thread_messages(self, *, user_id: int, thread_id: str, limit: int, before: str | None):
        self._ensure_thread_access(user_id=user_id, thread_id=thread_id)
        return self.message_repository.list_by_thread(
            thread_id=thread_id,
            user_id=user_id,
            limit=limit,
            before=before,
        )

    def create_user_message(
        self,
        *,
        user_id: int,
        thread_id: str,
        content: list[dict],
        run_id: str | None = None,
    ) -> MessageRecord:
        thread = self._ensure_thread_access(user_id=user_id, thread_id=thread_id)
        user_text = self.extract_text(content)
        now = datetime.now(timezone.utc)

        user_message = self.message_repository.create(
            MessageRecord(
                id=new_message_id(),
                thread_id=thread_id,
                user_id=user_id,
                role=MESSAGE_ROLE_USER,
                content=content,
                status=MESSAGE_STATUS_COMPLETED,
                run_id=run_id,
                created_at=now,
                updated_at=now,
                metadata={},
            )
        )
        if self.message_repository.count_by_thread(thread_id=thread_id, user_id=user_id) == 1:
            self.thread_repository.update_title(
                thread_id=thread_id,
                title=user_text[: self.settings.agents_thread_title_max_length] or self.settings.agents_thread_default_title,
            )
        self.thread_repository.update_last_message_at(
            thread_id=thread_id,
            last_message_at=user_message.created_at,
        )
        return user_message

    def create_assistant_message(
        self,
        *,
        user_id: int,
        thread_id: str,
        run_id: str,
        content: list[dict] | None = None,
        status: str = MESSAGE_STATUS_IN_PROGRESS,
        metadata: dict | None = None,
    ) -> MessageRecord:
        self._ensure_thread_access(user_id=user_id, thread_id=thread_id)
        now = datetime.now(timezone.utc)
        return self.message_repository.create(
            MessageRecord(
                id=new_message_id(),
                thread_id=thread_id,
                user_id=user_id,
                role=MESSAGE_ROLE_ASSISTANT,
                content=list(content or []),
                status=status,
                run_id=run_id,
                created_at=now,
                updated_at=now,
                metadata=dict(metadata or {}),
            )
        )

    def update_message_status(
        self,
        *,
        user_id: int,
        thread_id: str,
        message_id: str,
        status: str,
        content: list[dict] | None = None,
        metadata: dict | None = None,
    ) -> MessageRecord:
        self._ensure_thread_access(user_id=user_id, thread_id=thread_id)
        message = self.message_repository.update_status(
            message_id=message_id,
            status=status,
            content=content,
            metadata=metadata,
        )
        if message is None:
            raise ApiError(
                code=ApiErrorCode.MESSAGE_NOT_FOUND,
                message="消息不存在",
                http_status=404,
                details={"threadId": thread_id, "messageId": message_id},
            )
        return message

    def _ensure_thread_access(self, *, user_id: int, thread_id: str) -> ThreadRecord:
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
        return thread

    def extract_text(self, content: list[dict]) -> str:
        if not content:
            raise ApiError(
                code=ApiErrorCode.VALIDATION_ERROR,
                message="消息内容不能为空",
                http_status=400,
            )
        unsupported_types = [str(part.get("type", "")) for part in content if part.get("type") != "text"]
        if unsupported_types:
            raise ApiError(
                code=ApiErrorCode.UNSUPPORTED_CONTENT_TYPE,
                message="当前仅支持文本消息内容",
                http_status=400,
                details={"types": unsupported_types},
            )
        texts = [str(part.get("text", "")).strip() for part in content]
        text = "\n".join([value for value in texts if value])
        if not text:
            raise ApiError(
                code=ApiErrorCode.VALIDATION_ERROR,
                message="消息内容不能为空",
                http_status=400,
            )
        return text
