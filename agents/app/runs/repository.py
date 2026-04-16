from __future__ import annotations

import json
from dataclasses import replace
from datetime import datetime
from typing import Protocol

from app.runs.models import (
    RUN_STATUS_CANCELLED,
    RUN_STATUS_COMPLETED,
    RUN_STATUS_FAILED,
    RUN_STATUS_QUEUED,
    RUN_STATUS_REQUIRES_ACTION,
    RUN_STATUS_RUNNING,
    RunRecord,
)
from app.threads.repository import MySQLConnectionFactory

ACTIVE_RUN_STATUSES = {RUN_STATUS_QUEUED, RUN_STATUS_RUNNING, RUN_STATUS_REQUIRES_ACTION}


class RunRepository(Protocol):
    def create(self, record: RunRecord) -> RunRecord: ...

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

    def create_running(self, record: RunRecord) -> RunRecord: ...

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

    def create_running(self, record: RunRecord) -> RunRecord:
        return self.create(record)

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
            metadata={"outputMessageIds": list(output_message_ids)},
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
                      id, thread_id, user_id, trigger_message_id, status, started_at, completed_at, error_json, metadata_json
                    ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        record.id,
                        record.thread_id,
                        record.user_id,
                        record.trigger_message_id,
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
                    SELECT id, thread_id, user_id, trigger_message_id, status, started_at, completed_at, error_json, metadata_json
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
                    SELECT id, thread_id, user_id, trigger_message_id, status, started_at, completed_at, error_json, metadata_json
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

    def create_running(self, record: RunRecord) -> RunRecord:
        return self.create(record)

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
        metadata = {**current.metadata, "outputMessageIds": list(output_message_ids)}
        return self.update_status(
            run_id=run_id,
            status=RUN_STATUS_COMPLETED,
            completed_at=completed_at,
            metadata=metadata,
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
            status=row["status"],
            started_at=row["started_at"],
            completed_at=row.get("completed_at"),
            error=json.loads(error) if error else None,
            metadata=json.loads(metadata) if metadata else {},
        )
