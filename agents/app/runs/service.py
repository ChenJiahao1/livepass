from __future__ import annotations

from datetime import datetime, timezone

from app.common.errors import ApiError, ApiErrorCode
from app.common.ids import new_run_id
from app.messages.service import MessageService
from app.runs.models import (
    RUN_STATUS_CANCELLED,
    RUN_STATUS_COMPLETED,
    RUN_STATUS_FAILED,
    RUN_STATUS_QUEUED,
    RUN_STATUS_REQUIRES_ACTION,
    RUN_STATUS_RUNNING,
    RunRecord,
)
from app.runs.repository import RunRepository
from app.session.store import ThreadOwnershipError, ThreadOwnershipStore


class RunService:
    def __init__(
        self,
        *,
        run_repository: RunRepository,
        message_service: MessageService,
        ownership_store: ThreadOwnershipStore,
    ) -> None:
        self.run_repository = run_repository
        self.message_service = message_service
        self.ownership_store = ownership_store

    def create_run(self, *, user_id: int, thread_id: str, parts: list[dict]) -> RunRecord:
        run_id = new_run_id()
        user_text = self.message_service.extract_text(parts)
        active_run = self.run_repository.find_active_by_thread(thread_id=thread_id)
        if active_run is not None:
            raise ApiError(
                code=ApiErrorCode.RUN_STATE_INVALID,
                message="当前线程已有进行中的运行",
                http_status=409,
                details={"threadId": thread_id, "activeRunId": active_run.id, "status": active_run.status},
            )
        user_message = self.message_service.create_user_message(
            user_id=user_id,
            thread_id=thread_id,
            parts=parts,
            run_id=run_id,
        )
        assistant_message = self.message_service.create_assistant_message(
            user_id=user_id,
            thread_id=thread_id,
            run_id=run_id,
            parts=[],
        )
        record = RunRecord(
            id=run_id,
            thread_id=thread_id,
            user_id=user_id,
            trigger_message_id=user_message.id,
            status=RUN_STATUS_QUEUED,
            started_at=datetime.now(timezone.utc),
            completed_at=None,
            error=None,
            metadata={"assistantMessageId": assistant_message.id, "userText": user_text},
        )
        return self.run_repository.create(record)

    def create_running(self, *, user_id: int, thread_id: str, trigger_message_id: str) -> RunRecord:
        record = RunRecord(
            id=new_run_id(),
            thread_id=thread_id,
            user_id=user_id,
            trigger_message_id=trigger_message_id,
            status=RUN_STATUS_RUNNING,
            started_at=datetime.now(timezone.utc),
            completed_at=None,
            error=None,
            metadata={},
        )
        return self.run_repository.create(record)

    def mark_running(self, *, run_id: str) -> RunRecord:
        return self._update_status(run_id=run_id, status=RUN_STATUS_RUNNING)

    def mark_requires_action(self, *, run_id: str) -> RunRecord:
        return self._update_status(run_id=run_id, status=RUN_STATUS_REQUIRES_ACTION)

    def mark_completed(self, *, run_id: str, output_message_ids: list[str]) -> RunRecord:
        run = self.run_repository.find_by_id(run_id=run_id)
        if run is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"runId": run_id},
            )
        completed = self.run_repository.mark_completed(
            thread_id=run.thread_id,
            run_id=run_id,
            completed_at=datetime.now(timezone.utc),
            output_message_ids=output_message_ids,
        )
        if completed is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"runId": run_id},
            )
        return completed

    def mark_failed(self, *, run_id: str, message: str) -> RunRecord:
        run = self.run_repository.find_by_id(run_id=run_id)
        if run is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"runId": run_id},
            )
        failed = self.run_repository.mark_failed(
            thread_id=run.thread_id,
            run_id=run_id,
            completed_at=datetime.now(timezone.utc),
            error={"code": ApiErrorCode.AGENT_RUN_FAILED, "message": message},
        )
        if failed is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"runId": run_id},
            )
        return failed

    def mark_cancelled(self, *, run_id: str) -> RunRecord:
        return self._update_status(
            run_id=run_id,
            status=RUN_STATUS_CANCELLED,
            completed_at=datetime.now(timezone.utc),
        )

    def get_run(self, *, user_id: int, run_id: str) -> RunRecord:
        run = self.run_repository.find_by_id(run_id=run_id)
        if run is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"runId": run_id},
            )
        try:
            self.ownership_store.assert_owner(thread_id=run.thread_id, user_id=user_id)
        except ThreadOwnershipError as exc:
            raise ApiError(
                code=ApiErrorCode.FORBIDDEN,
                message="无权访问该线程",
                http_status=403,
                details={"threadId": run.thread_id},
            ) from exc

        if run.user_id != user_id:
            raise ApiError(
                code=ApiErrorCode.FORBIDDEN,
                message="无权访问该线程",
                http_status=403,
                details={"threadId": run.thread_id},
            )
        return run

    def get_active_run(self, *, user_id: int, thread_id: str) -> RunRecord | None:
        try:
            self.ownership_store.assert_owner(thread_id=thread_id, user_id=user_id)
        except ThreadOwnershipError as exc:
            raise ApiError(
                code=ApiErrorCode.FORBIDDEN,
                message="无权访问该线程",
                http_status=403,
                details={"threadId": thread_id},
            ) from exc
        run = self.run_repository.find_active_by_thread(thread_id=thread_id)
        if run is not None and run.user_id != user_id:
            raise ApiError(
                code=ApiErrorCode.FORBIDDEN,
                message="无权访问该线程",
                http_status=403,
                details={"threadId": thread_id},
            )
        return run

    def resume_run(self, *, user_id: int, run_id: str) -> RunRecord:
        run = self.get_run(user_id=user_id, run_id=run_id)
        if run.status != RUN_STATUS_REQUIRES_ACTION:
            raise ApiError(
                code=ApiErrorCode.RUN_STATE_INVALID,
                message="当前运行状态不可恢复",
                http_status=409,
                details={"runId": run_id, "status": run.status},
            )
        return self.mark_running(run_id=run_id)

    def _update_status(
        self,
        *,
        run_id: str,
        status: str,
        completed_at: datetime | None = None,
        error: dict | None = None,
        metadata: dict | None = None,
    ) -> RunRecord:
        run = self.run_repository.update_status(
            run_id=run_id,
            status=status,
            completed_at=completed_at,
            error=error,
            metadata=metadata,
        )
        if run is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"runId": run_id},
            )
        return run
