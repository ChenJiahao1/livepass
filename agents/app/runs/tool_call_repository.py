from __future__ import annotations

import json
from dataclasses import replace
from datetime import datetime
from typing import Protocol

from app.runs.tool_call_models import TOOL_CALL_STATUS_CANCELLED, TOOL_CALL_STATUS_WAITING_HUMAN
from app.runs.tool_call_models import ToolCallRecord
from app.threads.repository import MySQLConnectionFactory


class ToolCallRepository(Protocol):
    def create(self, record: ToolCallRecord) -> ToolCallRecord: ...

    def find_waiting_by_run(self, *, run_id: str) -> ToolCallRecord | None: ...

    def update_status(
        self,
        *,
        tool_call_id: str,
        status: str,
        error: dict | None,
        now: datetime,
        result: dict | None = None,
        output: dict | None = None,
    ) -> ToolCallRecord | None: ...

    def find_by_id(self, *, tool_call_id: str) -> ToolCallRecord | None: ...

    def mark_cancelled(self, *, tool_call_id: str, now: datetime) -> ToolCallRecord | None: ...


class InMemoryToolCallRepository:
    def __init__(self) -> None:
        self._tool_calls: dict[str, ToolCallRecord] = {}

    def create(self, record: ToolCallRecord) -> ToolCallRecord:
        self._tool_calls[record.id] = record
        return replace(record)

    def find_waiting_by_run(self, *, run_id: str) -> ToolCallRecord | None:
        for record in self._tool_calls.values():
            if record.run_id == run_id and record.status == TOOL_CALL_STATUS_WAITING_HUMAN:
                return replace(record)
        return None

    def update_status(
        self,
        *,
        tool_call_id: str,
        status: str,
        error: dict | None,
        now: datetime,
        result: dict | None = None,
        output: dict | None = None,
    ) -> ToolCallRecord | None:
        record = self._tool_calls.get(tool_call_id)
        if record is None:
            return None
        resolved_result = result if result is not None else output
        record.status = status
        record.result = dict(resolved_result) if resolved_result is not None else None
        record.error = dict(error) if error is not None else None
        record.updated_at = now
        record.completed_at = now if status in {"completed", "failed", "cancelled"} else None
        return replace(record)

    def find_by_id(self, *, tool_call_id: str) -> ToolCallRecord | None:
        record = self._tool_calls.get(tool_call_id)
        return replace(record) if record else None

    def mark_cancelled(self, *, tool_call_id: str, now: datetime) -> ToolCallRecord | None:
        return self.update_status(
            tool_call_id=tool_call_id,
            status=TOOL_CALL_STATUS_CANCELLED,
            result=None,
            error=None,
            now=now,
        )


class MySQLToolCallRepository:
    def __init__(self, connection_factory: MySQLConnectionFactory | None = None) -> None:
        self.connection_factory = connection_factory or MySQLConnectionFactory()

    def create(self, record: ToolCallRecord) -> ToolCallRecord:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    INSERT INTO agent_tool_calls (
                      id, run_id, thread_id, user_id, message_id, tool_name, status, arguments_json, request_json,
                      output_json, error_json, created_at, updated_at, completed_at, metadata_json
                    ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        record.id,
                        record.run_id,
                        record.thread_id,
                        record.user_id,
                        record.message_id,
                        record.tool_name,
                        record.status,
                        json.dumps(record.arguments),
                        json.dumps(record.human_request),
                        json.dumps(record.result) if record.result is not None else None,
                        json.dumps(record.error) if record.error is not None else None,
                        record.created_at,
                        record.updated_at,
                        record.completed_at,
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

    def find_waiting_by_run(self, *, run_id: str) -> ToolCallRecord | None:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    SELECT id, run_id, thread_id, user_id, message_id, tool_name, status, arguments_json, request_json,
                           output_json, error_json, created_at, updated_at, completed_at, metadata_json
                    FROM agent_tool_calls
                    WHERE run_id = %s AND status = %s
                    ORDER BY created_at DESC, id DESC
                    LIMIT 1
                    """,
                    (run_id, TOOL_CALL_STATUS_WAITING_HUMAN),
                )
                row = cursor.fetchone()
            return self._map_row(row) if row else None
        finally:
            connection.close()

    def update_status(
        self,
        *,
        tool_call_id: str,
        status: str,
        error: dict | None,
        now: datetime,
        result: dict | None = None,
        output: dict | None = None,
    ) -> ToolCallRecord | None:
        completed_at = now if status in {"completed", "failed", "cancelled"} else None
        resolved_result = result if result is not None else output
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    UPDATE agent_tool_calls
                    SET status = %s, output_json = %s, error_json = %s, updated_at = %s, completed_at = %s
                    WHERE id = %s
                    """,
                    (
                        status,
                        json.dumps(resolved_result) if resolved_result is not None else None,
                        json.dumps(error) if error is not None else None,
                        now,
                        completed_at,
                        tool_call_id,
                    ),
                )
            connection.commit()
            return self.find_by_id(tool_call_id=tool_call_id)
        except Exception:
            connection.rollback()
            raise
        finally:
            connection.close()

    def find_by_id(self, *, tool_call_id: str) -> ToolCallRecord | None:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    SELECT id, run_id, thread_id, user_id, message_id, tool_name, status, arguments_json, request_json,
                           output_json, error_json, created_at, updated_at, completed_at, metadata_json
                    FROM agent_tool_calls
                    WHERE id = %s
                    """,
                    (tool_call_id,),
                )
                row = cursor.fetchone()
            return self._map_row(row) if row else None
        finally:
            connection.close()

    def mark_cancelled(self, *, tool_call_id: str, now: datetime) -> ToolCallRecord | None:
        return self.update_status(
            tool_call_id=tool_call_id,
            status=TOOL_CALL_STATUS_CANCELLED,
            result=None,
            error=None,
            now=now,
        )

    def _map_row(self, row: dict) -> ToolCallRecord:
        arguments = row.get("arguments_json")
        request = row.get("request_json")
        output = row.get("output_json")
        error = row.get("error_json")
        metadata = row.get("metadata_json")
        return ToolCallRecord(
            id=row["id"],
            run_id=row["run_id"],
            thread_id=row["thread_id"],
            user_id=int(row["user_id"]),
            message_id=row.get("message_id"),
            tool_name=row["tool_name"],
            status=row["status"],
            arguments=_parse_json_object(arguments),
            human_request=_parse_json_object(request),
            result=_parse_json_object(output) if output else None,
            error=_parse_json_object(error) if error else None,
            created_at=row["created_at"],
            updated_at=row["updated_at"],
            completed_at=row["completed_at"],
            metadata=_parse_json_object(metadata),
        )


def _parse_json_object(value: object) -> dict:
    if value is None:
        return {}
    if isinstance(value, dict):
        return dict(value)
    if isinstance(value, str):
        parsed = json.loads(value)
        return dict(parsed) if isinstance(parsed, dict) else {}
    return {}
