from __future__ import annotations

from datetime import datetime, timezone

from app.common.errors import ApiError, ApiErrorCode
from app.common.ids import new_run_id
from app.runs.models import RUN_STATUS_FAILED, RUN_STATUS_RUNNING, RunRecord
from app.runs.repository import RunRepository
from app.session.store import ThreadOwnershipError, ThreadOwnershipStore


class RunService:
    def __init__(
        self,
        *,
        run_repository: RunRepository,
        ownership_store: ThreadOwnershipStore,
    ) -> None:
        self.run_repository = run_repository
        self.ownership_store = ownership_store

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
        return self.run_repository.create_running(record)

    def mark_completed(self, *, thread_id: str, run_id: str, output_message_ids: list[str]) -> RunRecord:
        run = self.run_repository.mark_completed(
            thread_id=thread_id,
            run_id=run_id,
            completed_at=datetime.now(timezone.utc),
            output_message_ids=output_message_ids,
        )
        if run is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"threadId": thread_id, "runId": run_id},
            )
        return run

    def mark_failed(self, *, thread_id: str, run_id: str, message: str) -> RunRecord:
        run = self.run_repository.mark_failed(
            thread_id=thread_id,
            run_id=run_id,
            completed_at=datetime.now(timezone.utc),
            error={"code": ApiErrorCode.AGENT_RUN_FAILED, "message": message},
        )
        if run is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"threadId": thread_id, "runId": run_id},
            )
        return run

    def get_run(self, *, user_id: int, thread_id: str, run_id: str) -> RunRecord:
        try:
            self.ownership_store.assert_owner(thread_id=thread_id, user_id=user_id)
        except ThreadOwnershipError as exc:
            raise ApiError(
                code=ApiErrorCode.FORBIDDEN,
                message="无权访问该线程",
                http_status=403,
                details={"threadId": thread_id},
            ) from exc

        run = self.run_repository.find_by_thread_and_id(thread_id=thread_id, run_id=run_id)
        if run is None:
            raise ApiError(
                code=ApiErrorCode.RUN_NOT_FOUND,
                message="运行不存在",
                http_status=404,
                details={"threadId": thread_id, "runId": run_id},
            )
        if run.user_id != user_id:
            raise ApiError(
                code=ApiErrorCode.FORBIDDEN,
                message="无权访问该线程",
                http_status=403,
                details={"threadId": thread_id},
            )
        return run
