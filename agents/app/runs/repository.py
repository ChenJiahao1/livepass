from __future__ import annotations

import json
from dataclasses import replace
from datetime import datetime
from typing import Protocol

from app.shared.errors import ApiError, ApiErrorCode
from app.conversations.messages.models import MessageRecord
from app.conversations.messages.repository import MessageRepository
from app.runs.models import (
    RUN_STATUS_CANCELLED,
    RUN_STATUS_COMPLETED,
    RUN_STATUS_FAILED,
    RUN_STATUS_QUEUED,
    RUN_STATUS_REQUIRES_ACTION,
    RUN_STATUS_RUNNING,
    RunRecord,
)
from app.integrations.storage.mysql import MySQLConnectionFactory
from app.conversations.threads.repository import ThreadRepository

ACTIVE_RUN_STATUSES = {RUN_STATUS_QUEUED, RUN_STATUS_RUNNING, RUN_STATUS_REQUIRES_ACTION}


class RunRepository(Protocol):
    def create(self, record: RunRecord) -> RunRecord: ...

    def create_with_messages(
        self,
        *,
        run: RunRecord,
        user_message: MessageRecord,
        assistant_message: MessageRecord,
        title_if_first_message: str | None,
        thread_repository: ThreadRepository | None = None,
        message_repository: MessageRepository | None = None,
    ) -> tuple[RunRecord, MessageRecord, MessageRecord]: ...

    def update_status(
        self,
        *,
        run_id: str,
        status: str,
        completed_at: datetime | None = None,
        error: dict | None = None,
        metadata: dict | None = None,
    ) -> RunRecord | None: ...

    def find_by_id(self, *, run_id: str) -> RunRecord | None: ...

    def find_active_by_thread(self, *, thread_id: str) -> RunRecord | None: ...

    def find_by_thread_and_id(self, *, thread_id: str, run_id: str) -> RunRecord | None: ...

    def mark_completed(
        self,
        *,
        thread_id: str,
        run_id: str,
        completed_at: datetime,
        output_message_ids: list[str],
    ) -> RunRecord | None: ...

    def mark_failed(
        self,
        *,
        thread_id: str,
        run_id: str,
        completed_at: datetime,
        error: dict,
    ) -> RunRecord | None: ...

    def find_by_thread_and_id(self, *, thread_id: str, run_id: str) -> RunRecord | None: ...


class InMemoryRunRepository:
    def __init__(self) -> None:
        self._runs: dict[str, RunRecord] = {}

    def create(self, record: RunRecord) -> RunRecord:
        self._runs[record.id] = record
        return replace(record)

    def create_with_messages(
        self,
        *,
        run: RunRecord,
        user_message: MessageRecord,
        assistant_message: MessageRecord,
        title_if_first_message: str | None,
        thread_repository: ThreadRepository | None = None,
        message_repository: MessageRepository | None = None,
    ) -> tuple[RunRecord, MessageRecord, MessageRecord]:
        if thread_repository is None or message_repository is None:
            raise RuntimeError("create_with_messages requires thread_repository and message_repository")

        thread = thread_repository.find_by_id(thread_id=run.thread_id)
        if thread is None or thread.user_id != run.user_id:
            raise ApiError(
                code=ApiErrorCode.THREAD_NOT_FOUND,
                message="线程不存在",
                http_status=404,
                details={"threadId": run.thread_id},
            )

        active_run = self.find_active_by_thread(thread_id=run.thread_id)
        if active_run is not None:
            raise ApiError(
                code=ApiErrorCode.ACTIVE_RUN_EXISTS,
                message="当前线程已有进行中的运行",
                http_status=409,
                details={"threadId": run.thread_id, "activeRunId": active_run.id, "status": active_run.status},
            )

        created_user_message = message_repository.create(user_message)
        if message_repository.count_by_thread(thread_id=run.thread_id, user_id=run.user_id) == 1 and title_if_first_message:
            thread_repository.update_title(thread_id=run.thread_id, title=title_if_first_message)
        created_assistant_message = message_repository.create(assistant_message)
        created_run = self.create(run)
        thread_repository.update_last_message_at(
            thread_id=run.thread_id,
            last_message_at=created_user_message.created_at,
        )
        return created_run, created_user_message, created_assistant_message

    def update_status(
        self,
        *,
        run_id: str,
        status: str,
        completed_at: datetime | None = None,
        error: dict | None = None,
        metadata: dict | None = None,
    ) -> RunRecord | None:
        record = self._runs.get(run_id)
        if record is None:
            return None
        record.status = status
        record.completed_at = completed_at
        record.error = dict(error) if error is not None else None
        if metadata:
            record.metadata = {**record.metadata, **metadata}
        return replace(record)

    def find_by_id(self, *, run_id: str) -> RunRecord | None:
        record = self._runs.get(run_id)
        return replace(record) if record else None

    def find_active_by_thread(self, *, thread_id: str) -> RunRecord | None:
        active = [
            record
            for record in self._runs.values()
            if record.thread_id == thread_id and record.status in ACTIVE_RUN_STATUSES
        ]
        if not active:
            return None
        active.sort(key=lambda record: (record.started_at, record.id), reverse=True)
        return replace(active[0])

    def mark_completed(
        self,
        *,
        thread_id: str,
        run_id: str,
        completed_at: datetime,
        output_message_ids: list[str],
    ) -> RunRecord | None:
        return self.update_status(
            run_id=run_id,
            status=RUN_STATUS_COMPLETED,
            completed_at=completed_at,
        )

    def mark_failed(
        self,
        *,
        thread_id: str,
        run_id: str,
        completed_at: datetime,
        error: dict,
    ) -> RunRecord | None:
        return self.update_status(
            run_id=run_id,
            status=RUN_STATUS_FAILED,
            completed_at=completed_at,
            error=error,
        )

    def find_by_thread_and_id(self, *, thread_id: str, run_id: str) -> RunRecord | None:
        record = self._runs.get(run_id)
        if record is None or record.thread_id != thread_id:
            return None
        return replace(record) if record else None


class MySQLRunRepository:
    def __init__(self, connection_factory: MySQLConnectionFactory | None = None) -> None:
        self.connection_factory = connection_factory or MySQLConnectionFactory()

    def create(self, record: RunRecord) -> RunRecord:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    INSERT INTO agent_runs (
                      id, thread_id, user_id, trigger_message_id, output_message_id, status, started_at, completed_at, error_json, metadata_json
                    ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        record.id,
                        record.thread_id,
                        record.user_id,
                        record.trigger_message_id,
                        record.output_message_id,
                        record.status,
                        record.started_at,
                        record.completed_at,
                        json.dumps(record.error) if record.error else None,
                        json.dumps(record.metadata),
                    ),
                )
            connection.commit()
            return record
        except Exception:
            connection.rollback()
            raise
        finally:
            connection.close()

    def create_with_messages(
        self,
        *,
        run: RunRecord,
        user_message: MessageRecord,
        assistant_message: MessageRecord,
        title_if_first_message: str | None,
        thread_repository: ThreadRepository | None = None,
        message_repository: MessageRepository | None = None,
    ) -> tuple[RunRecord, MessageRecord, MessageRecord]:
        del thread_repository
        del message_repository
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    SELECT id, user_id
                    FROM agent_threads
                    WHERE id = %s
                    FOR UPDATE
                    """,
                    (run.thread_id,),
                )
                thread_row = cursor.fetchone()
                if thread_row is None or int(thread_row["user_id"]) != run.user_id:
                    raise ApiError(
                        code=ApiErrorCode.THREAD_NOT_FOUND,
                        message="线程不存在",
                        http_status=404,
                        details={"threadId": run.thread_id},
                    )

                cursor.execute(
                    """
                    SELECT id, status
                    FROM agent_runs
                    WHERE thread_id = %s AND status IN (%s, %s, %s)
                    ORDER BY started_at DESC, id DESC
                    LIMIT 1
                    FOR UPDATE
                    """,
                    (run.thread_id, RUN_STATUS_QUEUED, RUN_STATUS_RUNNING, RUN_STATUS_REQUIRES_ACTION),
                )
                active_run_row = cursor.fetchone()
                if active_run_row is not None:
                    raise ApiError(
                        code=ApiErrorCode.ACTIVE_RUN_EXISTS,
                        message="当前线程已有进行中的运行",
                        http_status=409,
                        details={
                            "threadId": run.thread_id,
                            "activeRunId": active_run_row["id"],
                            "status": active_run_row["status"],
                        },
                    )

                cursor.execute(
                    """
                    SELECT COUNT(1) AS total
                    FROM agent_messages
                    WHERE thread_id = %s AND user_id = %s
                    """,
                    (run.thread_id, run.user_id),
                )
                message_count_row = cursor.fetchone()
                existing_message_count = int(message_count_row["total"]) if message_count_row else 0

                cursor.execute(
                    """
                    INSERT INTO agent_messages (
                      id, thread_id, user_id, role, content_json, status, run_id, created_at, updated_at, metadata_json
                    ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        user_message.id,
                        user_message.thread_id,
                        user_message.user_id,
                        user_message.role,
                        json.dumps(user_message.content),
                        user_message.status,
                        user_message.run_id,
                        user_message.created_at,
                        user_message.updated_at,
                        json.dumps(user_message.metadata),
                    ),
                )
                cursor.execute(
                    """
                    INSERT INTO agent_messages (
                      id, thread_id, user_id, role, content_json, status, run_id, created_at, updated_at, metadata_json
                    ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        assistant_message.id,
                        assistant_message.thread_id,
                        assistant_message.user_id,
                        assistant_message.role,
                        json.dumps(assistant_message.content),
                        assistant_message.status,
                        assistant_message.run_id,
                        assistant_message.created_at,
                        assistant_message.updated_at,
                        json.dumps(assistant_message.metadata),
                    ),
                )
                cursor.execute(
                    """
                    INSERT INTO agent_runs (
                      id, thread_id, user_id, trigger_message_id, output_message_id, status, started_at, completed_at, error_json, metadata_json
                    ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        run.id,
                        run.thread_id,
                        run.user_id,
                        run.trigger_message_id,
                        run.output_message_id,
                        run.status,
                        run.started_at,
                        run.completed_at,
                        json.dumps(run.error) if run.error else None,
                        json.dumps(run.metadata),
                    ),
                )
                if existing_message_count == 0 and title_if_first_message:
                    cursor.execute(
                        "UPDATE agent_threads SET title = %s WHERE id = %s",
                        (title_if_first_message, run.thread_id),
                    )
                cursor.execute(
                    """
                    UPDATE agent_threads
                    SET last_message_at = %s, updated_at = %s
                    WHERE id = %s
                    """,
                    (user_message.created_at, user_message.created_at, run.thread_id),
                )
            connection.commit()
            return run, user_message, assistant_message
        except Exception:
            connection.rollback()
            raise
        finally:
            connection.close()

    def update_status(
        self,
        *,
        run_id: str,
        status: str,
        completed_at: datetime | None = None,
        error: dict | None = None,
        metadata: dict | None = None,
    ) -> RunRecord | None:
        connection = self.connection_factory.connect()
        try:
            updates = ["status = %s", "completed_at = %s", "error_json = %s"]
            values: list[object] = [status, completed_at, json.dumps(error) if error is not None else None]
            if metadata is not None:
                updates.append("metadata_json = %s")
                values.append(json.dumps(metadata))
            with connection.cursor() as cursor:
                cursor.execute(
                    f"UPDATE agent_runs SET {', '.join(updates)} WHERE id = %s",
                    (*values, run_id),
                )
            connection.commit()
            return self.find_by_id(run_id=run_id)
        except Exception:
            connection.rollback()
            raise
        finally:
            connection.close()

    def find_by_id(self, *, run_id: str) -> RunRecord | None:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    SELECT id, thread_id, user_id, trigger_message_id, output_message_id, status, started_at, completed_at, error_json, metadata_json
                    FROM agent_runs
                    WHERE id = %s
                    """,
                    (run_id,),
                )
                row = cursor.fetchone()
            return self._map_row(row) if row else None
        finally:
            connection.close()

    def find_active_by_thread(self, *, thread_id: str) -> RunRecord | None:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    SELECT id, thread_id, user_id, trigger_message_id, output_message_id, status, started_at, completed_at, error_json, metadata_json
                    FROM agent_runs
                    WHERE thread_id = %s AND status IN (%s, %s, %s)
                    ORDER BY started_at DESC, id DESC
                    LIMIT 1
                    """,
                    (thread_id, RUN_STATUS_QUEUED, RUN_STATUS_RUNNING, RUN_STATUS_REQUIRES_ACTION),
                )
                row = cursor.fetchone()
            return self._map_row(row) if row else None
        finally:
            connection.close()

    def mark_completed(
        self,
        *,
        thread_id: str,
        run_id: str,
        completed_at: datetime,
        output_message_ids: list[str],
    ) -> RunRecord | None:
        current = self.find_by_thread_and_id(thread_id=thread_id, run_id=run_id)
        if current is None:
            return None
        return self.update_status(
            run_id=run_id,
            status=RUN_STATUS_COMPLETED,
            completed_at=completed_at,
            metadata=current.metadata,
        )

    def mark_failed(
        self,
        *,
        thread_id: str,
        run_id: str,
        completed_at: datetime,
        error: dict,
    ) -> RunRecord | None:
        current = self.find_by_thread_and_id(thread_id=thread_id, run_id=run_id)
        if current is None:
            return None
        return self.update_status(
            run_id=run_id,
            status=RUN_STATUS_FAILED,
            completed_at=completed_at,
            error=error,
            metadata=current.metadata,
        )

    def find_by_thread_and_id(self, *, thread_id: str, run_id: str) -> RunRecord | None:
        run = self.find_by_id(run_id=run_id)
        if run is None or run.thread_id != thread_id:
            return None
        return run

    def _map_row(self, row: dict) -> RunRecord:
        error = row.get("error_json")
        metadata = row.get("metadata_json")
        return RunRecord(
            id=row["id"],
            thread_id=row["thread_id"],
            user_id=int(row["user_id"]),
            trigger_message_id=row["trigger_message_id"],
            output_message_id=row["output_message_id"],
            status=row["status"],
            started_at=row["started_at"],
            completed_at=row.get("completed_at"),
            error=json.loads(error) if error else None,
            metadata=json.loads(metadata) if metadata else {},
        )
