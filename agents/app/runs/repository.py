from __future__ import annotations

import json
from dataclasses import replace
from datetime import datetime
from typing import Protocol

from app.runs.models import RUN_STATUS_COMPLETED, RUN_STATUS_FAILED, RunRecord
from app.threads.repository import MySQLConnectionFactory


class RunRepository(Protocol):
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
        self._runs: dict[tuple[str, str], RunRecord] = {}

    def create_running(self, record: RunRecord) -> RunRecord:
        self._runs[(record.thread_id, record.id)] = record
        return replace(record)

    def mark_completed(
        self,
        *,
        thread_id: str,
        run_id: str,
        completed_at: datetime,
        output_message_ids: list[str],
    ) -> RunRecord | None:
        record = self._runs.get((thread_id, run_id))
        if record is None:
            return None
        record.status = RUN_STATUS_COMPLETED
        record.completed_at = completed_at
        record.metadata = {**record.metadata, "outputMessageIds": list(output_message_ids)}
        return replace(record)

    def mark_failed(
        self,
        *,
        thread_id: str,
        run_id: str,
        completed_at: datetime,
        error: dict,
    ) -> RunRecord | None:
        record = self._runs.get((thread_id, run_id))
        if record is None:
            return None
        record.status = RUN_STATUS_FAILED
        record.completed_at = completed_at
        record.error = dict(error)
        return replace(record)

    def find_by_thread_and_id(self, *, thread_id: str, run_id: str) -> RunRecord | None:
        record = self._runs.get((thread_id, run_id))
        return replace(record) if record else None


class MySQLRunRepository:
    def __init__(self, connection_factory: MySQLConnectionFactory | None = None) -> None:
        self.connection_factory = connection_factory or MySQLConnectionFactory()

    def create_running(self, record: RunRecord) -> RunRecord:
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

    def mark_completed(
        self,
        *,
        thread_id: str,
        run_id: str,
        completed_at: datetime,
        output_message_ids: list[str],
    ) -> RunRecord | None:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    UPDATE agent_runs
                    SET status = %s, completed_at = %s, metadata_json = JSON_SET(COALESCE(metadata_json, JSON_OBJECT()), '$.outputMessageIds', %s)
                    WHERE thread_id = %s AND id = %s
                    """,
                    (RUN_STATUS_COMPLETED, completed_at, json.dumps(output_message_ids), thread_id, run_id),
                )
            connection.commit()
            return self.find_by_thread_and_id(thread_id=thread_id, run_id=run_id)
        except Exception:
            connection.rollback()
            raise
        finally:
            connection.close()

    def mark_failed(
        self,
        *,
        thread_id: str,
        run_id: str,
        completed_at: datetime,
        error: dict,
    ) -> RunRecord | None:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    UPDATE agent_runs
                    SET status = %s, completed_at = %s, error_json = %s
                    WHERE thread_id = %s AND id = %s
                    """,
                    (RUN_STATUS_FAILED, completed_at, json.dumps(error), thread_id, run_id),
                )
            connection.commit()
            return self.find_by_thread_and_id(thread_id=thread_id, run_id=run_id)
        except Exception:
            connection.rollback()
            raise
        finally:
            connection.close()

    def find_by_thread_and_id(self, *, thread_id: str, run_id: str) -> RunRecord | None:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    SELECT id, thread_id, user_id, trigger_message_id, status, started_at, completed_at, error_json, metadata_json
                    FROM agent_runs
                    WHERE thread_id = %s AND id = %s
                    """,
                    (thread_id, run_id),
                )
                row = cursor.fetchone()
            return self._map_row(row) if row else None
        finally:
            connection.close()

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
