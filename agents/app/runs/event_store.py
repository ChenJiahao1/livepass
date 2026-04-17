from __future__ import annotations

import json
from dataclasses import replace
from datetime import datetime
from typing import Protocol

from app.common.ids import new_run_event_id
from app.runs.event_models import RunEventRecord
from app.conversations.threads.repository import MySQLConnectionFactory


class RunEventStore(Protocol):
    def append(
        self,
        *,
        run_id: str,
        thread_id: str,
        user_id: int,
        event_type: str,
        payload: dict,
        message_id: str | None = None,
        tool_call_id: str | None = None,
        now: datetime,
    ) -> RunEventRecord: ...

    def list_after(self, *, run_id: str, after_sequence_no: int) -> list[RunEventRecord]: ...

    def latest(self, *, run_id: str) -> RunEventRecord | None: ...


class InMemoryRunEventStore:
    def __init__(self) -> None:
        self._events_by_run: dict[str, list[RunEventRecord]] = {}

    def append(
        self,
        *,
        run_id: str,
        thread_id: str,
        user_id: int,
        event_type: str,
        payload: dict,
        message_id: str | None = None,
        tool_call_id: str | None = None,
        now: datetime,
    ) -> RunEventRecord:
        events = self._events_by_run.setdefault(run_id, [])
        payload_data = dict(payload)
        resolved_message_id = message_id or payload_data.pop("messageId", None)
        resolved_tool_call_id = tool_call_id or payload_data.pop("toolCallId", None)
        record = RunEventRecord(
            id=new_run_event_id(),
            run_id=run_id,
            thread_id=thread_id,
            user_id=user_id,
            sequence_no=len(events) + 1,
            event_type=event_type,
            message_id=resolved_message_id,
            tool_call_id=resolved_tool_call_id,
            payload=payload_data,
            created_at=now,
        )
        events.append(record)
        return replace(record)

    def list_after(self, *, run_id: str, after_sequence_no: int) -> list[RunEventRecord]:
        events = self._events_by_run.get(run_id, [])
        return [replace(record) for record in events if record.sequence_no > after_sequence_no]

    def latest(self, *, run_id: str) -> RunEventRecord | None:
        events = self._events_by_run.get(run_id, [])
        return replace(events[-1]) if events else None


class MySQLRunEventStore:
    def __init__(self, connection_factory: MySQLConnectionFactory | None = None) -> None:
        self.connection_factory = connection_factory or MySQLConnectionFactory()

    def append(
        self,
        *,
        run_id: str,
        thread_id: str,
        user_id: int,
        event_type: str,
        payload: dict,
        message_id: str | None = None,
        tool_call_id: str | None = None,
        now: datetime,
    ) -> RunEventRecord:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    SELECT COALESCE(MAX(sequence_no), 0) AS last_sequence_no
                    FROM agent_run_events
                    WHERE run_id = %s
                    FOR UPDATE
                    """,
                    (run_id,),
                )
                row = cursor.fetchone()
                sequence_no = int(row["last_sequence_no"]) + 1
                payload_data = dict(payload)
                resolved_message_id = message_id or payload_data.pop("messageId", None)
                resolved_tool_call_id = tool_call_id or payload_data.pop("toolCallId", None)
                record = RunEventRecord(
                    id=new_run_event_id(),
                    run_id=run_id,
                    thread_id=thread_id,
                    user_id=user_id,
                    sequence_no=sequence_no,
                    event_type=event_type,
                    message_id=resolved_message_id,
                    tool_call_id=resolved_tool_call_id,
                    payload=payload_data,
                    created_at=now,
                )
                cursor.execute(
                    """
                    INSERT INTO agent_run_events (
                      id, run_id, thread_id, user_id, sequence_no, event_type, message_id, tool_call_id, payload_json, created_at
                    ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        record.id,
                        record.run_id,
                        record.thread_id,
                        record.user_id,
                        record.sequence_no,
                        record.event_type,
                        record.message_id,
                        record.tool_call_id,
                        json.dumps(record.payload),
                        record.created_at,
                    ),
                )
            connection.commit()
            return record
        except Exception:
            connection.rollback()
            raise
        finally:
            connection.close()

    def list_after(self, *, run_id: str, after_sequence_no: int) -> list[RunEventRecord]:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    SELECT id, run_id, thread_id, user_id, sequence_no, event_type, message_id, tool_call_id, payload_json, created_at
                    FROM agent_run_events
                    WHERE run_id = %s AND sequence_no > %s
                    ORDER BY sequence_no ASC
                    """,
                    (run_id, after_sequence_no),
                )
                rows = cursor.fetchall()
            return [self._map_row(row) for row in rows]
        finally:
            connection.close()

    def latest(self, *, run_id: str) -> RunEventRecord | None:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    SELECT id, run_id, thread_id, user_id, sequence_no, event_type, message_id, tool_call_id, payload_json, created_at
                    FROM agent_run_events
                    WHERE run_id = %s
                    ORDER BY sequence_no DESC
                    LIMIT 1
                    """,
                    (run_id,),
                )
                row = cursor.fetchone()
            return self._map_row(row) if row else None
        finally:
            connection.close()

    def _map_row(self, row: dict) -> RunEventRecord:
        return RunEventRecord(
            id=row["id"],
            run_id=row["run_id"],
            thread_id=row["thread_id"],
            user_id=int(row["user_id"]),
            sequence_no=int(row["sequence_no"]),
            event_type=row["event_type"],
            message_id=row.get("message_id"),
            tool_call_id=row.get("tool_call_id"),
            payload=json.loads(row["payload_json"]),
            created_at=row["created_at"],
        )
