from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timezone

from app.agent_runtime.service import AgentRuntimeService
from app.common.errors import ApiError, ApiErrorCode
from app.common.ids import new_message_id
from app.config import Settings, get_settings
from app.messages.models import MESSAGE_ROLE_ASSISTANT, MESSAGE_ROLE_USER, MESSAGE_STATUS_COMPLETED, MessageRecord
from app.messages.repository import MessageRepository
from app.runs.models import RunRecord
from app.runs.service import RunService
from app.session.store import ThreadOwnershipError, ThreadOwnershipStore
from app.threads.models import ThreadRecord
from app.threads.repository import ThreadRepository


@dataclass(slots=True)
class SendMessageResult:
    run: RunRecord
    messages: list[MessageRecord]
    thread: ThreadRecord


class MessageService:
    def __init__(
        self,
        *,
        thread_repository: ThreadRepository,
        message_repository: MessageRepository,
        run_service: RunService,
        runtime_service: AgentRuntimeService,
        ownership_store: ThreadOwnershipStore,
        settings: Settings | None = None,
    ) -> None:
        self.thread_repository = thread_repository
        self.message_repository = message_repository
        self.run_service = run_service
        self.runtime_service = runtime_service
        self.ownership_store = ownership_store
        self.settings = settings or get_settings()

    def list_messages(self, *, user_id: int, thread_id: str, limit: int, before: str | None):
        self._ensure_thread_access(user_id=user_id, thread_id=thread_id)
        return self.message_repository.list_by_thread(
            thread_id=thread_id,
            user_id=user_id,
            limit=limit,
            before=before,
        )

    async def send_user_message(
        self,
        *,
        user_id: int,
        thread_id: str,
        parts: list[dict],
    ) -> SendMessageResult:
        thread = self._ensure_thread_access(user_id=user_id, thread_id=thread_id)
        user_text = self._extract_text(parts)
        now = datetime.now(timezone.utc)

        user_message = self.message_repository.create(
            MessageRecord(
                id=new_message_id(),
                thread_id=thread_id,
                user_id=user_id,
                role=MESSAGE_ROLE_USER,
                parts=parts,
                status=MESSAGE_STATUS_COMPLETED,
                run_id=None,
                created_at=now,
                metadata={},
            )
        )
        run = self.run_service.create_running(
            user_id=user_id,
            thread_id=thread_id,
            trigger_message_id=user_message.id,
        )

        if self.message_repository.count_by_thread(thread_id=thread_id, user_id=user_id) == 1:
            thread = self.thread_repository.update_title(
                thread_id=thread_id,
                title=user_text[: self.settings.agents_thread_title_max_length] or self.settings.agents_thread_default_title,
            ) or thread

        try:
            runtime_result = await self.runtime_service.invoke(
                user_id=user_id,
                thread_id=thread_id,
                user_text=user_text,
            )
        except Exception as exc:
            failed_run = self.run_service.mark_failed(
                thread_id=thread_id,
                run_id=run.id,
                message=str(exc),
            )
            thread = self.thread_repository.update_last_message_at(
                thread_id=thread_id,
                last_message_at=user_message.created_at,
            ) or thread
            return SendMessageResult(run=failed_run, messages=[user_message], thread=thread)

        assistant_message = self.message_repository.create(
            MessageRecord(
                id=new_message_id(),
                thread_id=thread_id,
                user_id=user_id,
                role=MESSAGE_ROLE_ASSISTANT,
                parts=[{"type": "text", "text": runtime_result.reply}],
                status=MESSAGE_STATUS_COMPLETED,
                run_id=run.id,
                created_at=datetime.now(timezone.utc),
                metadata=runtime_result.metadata,
            )
        )
        completed_run = self.run_service.mark_completed(
            thread_id=thread_id,
            run_id=run.id,
            output_message_ids=[assistant_message.id],
        )
        thread = self.thread_repository.update_last_message_at(
            thread_id=thread_id,
            last_message_at=assistant_message.created_at,
        ) or thread
        return SendMessageResult(
            run=completed_run,
            messages=[user_message, assistant_message],
            thread=thread,
        )

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

    def _extract_text(self, parts: list[dict]) -> str:
        if not parts:
            raise ApiError(
                code=ApiErrorCode.VALIDATION_ERROR,
                message="消息内容不能为空",
                http_status=400,
            )
        texts = [str(part.get("text", "")).strip() for part in parts if part.get("type") == "text"]
        text = "\n".join([value for value in texts if value])
        if not text:
            raise ApiError(
                code=ApiErrorCode.VALIDATION_ERROR,
                message="消息内容不能为空",
                http_status=400,
            )
        return text
