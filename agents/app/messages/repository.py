from __future__ import annotations

import json
from dataclasses import replace
from datetime import datetime, timezone
from typing import Protocol

from app.common.cursor import decode_cursor, encode_cursor
from app.messages.models import MessageRecord
from app.threads.repository import MySQLConnectionFactory


class MessageRepository(Protocol):
    def create(self, record: MessageRecord) -> MessageRecord: ...

    def find_by_id(self, *, message_id: str) -> MessageRecord | None: ...

    def list_by_thread(
        self,
        *,
        thread_id: str,
        user_id: int,
        limit: int,
        before: str | None,
    ) -> tuple[list[MessageRecord], str | None]: ...

    def count_by_thread(self, *, thread_id: str, user_id: int) -> int: ...

    def update_status(
        self,
        *,
        message_id: str,
        status: str,
        parts: list[dict] | None,
        metadata: dict | None,
    ) -> MessageRecord | None: ...


class InMemoryMessageRepository:
    def __init__(self) -> None:
        self._messages: list[MessageRecord] = []

    def create(self, record: MessageRecord) -> MessageRecord:
        self._messages.append(record)
        return replace(record)

    def find_by_id(self, *, message_id: str) -> MessageRecord | None:
        for record in self._messages:
            if record.id == message_id:
                return replace(record)
        return None

    def list_by_thread(
        self,
        *,
        thread_id: str,
        user_id: int,
        limit: int,
        before: str | None,
    ) -> tuple[list[MessageRecord], str | None]:
        before_key = _message_key_from_cursor(before) if before is not None else None
        messages = [
            replace(record)
            for record in self._messages
            if record.thread_id == thread_id
            and record.user_id == user_id
            and (before_key is None or _message_sort_key(record) < before_key)
        ]
        messages.sort(key=lambda record: (record.created_at or datetime.min, record.id))
        has_more = len(messages) > limit
        page = messages[-limit:]
        next_cursor = _build_message_cursor(page[0]) if has_more and page else None
        return page, next_cursor

    def count_by_thread(self, *, thread_id: str, user_id: int) -> int:
        return len(
            [
                record
                for record in self._messages
                if record.thread_id == thread_id and record.user_id == user_id
            ]
        )

    def update_status(
        self,
        *,
        message_id: str,
        status: str,
        parts: list[dict] | None,
        metadata: dict | None,
    ) -> MessageRecord | None:
        for record in self._messages:
            if record.id != message_id:
                continue
            record.status = status
            if parts is not None:
                record.parts = list(parts)
            if metadata is not None:
                record.metadata = dict(metadata)
            record.updated_at = datetime.now(timezone.utc)
            return replace(record)
        return None


class MySQLMessageRepository:
    def __init__(self, connection_factory: MySQLConnectionFactory | None = None) -> None:
        self.connection_factory = connection_factory or MySQLConnectionFactory()

    def create(self, record: MessageRecord) -> MessageRecord:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    INSERT INTO agent_messages (
                      id, thread_id, user_id, role, parts_json, status, run_id, created_at, updated_at, metadata_json
                    ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        record.id,
                        record.thread_id,
                        record.user_id,
                        record.role,
                        json.dumps(record.parts),
                        record.status,
                        record.run_id,
                        record.created_at,
                        record.updated_at,
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

    def find_by_id(self, *, message_id: str) -> MessageRecord | None:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    SELECT id, thread_id, user_id, role, parts_json, status, run_id, created_at, updated_at, metadata_json
                    FROM agent_messages
                    WHERE id = %s
                    """,
                    (message_id,),
                )
                row = cursor.fetchone()
            return self._map_row(row) if row else None
        finally:
            connection.close()

    def list_by_thread(
        self,
        *,
        thread_id: str,
        user_id: int,
        limit: int,
        before: str | None,
    ) -> tuple[list[MessageRecord], str | None]:
        connection = self.connection_factory.connect()
        try:
            sql = """
                SELECT id, thread_id, user_id, role, parts_json, status, run_id, created_at, updated_at, metadata_json
                FROM agent_messages
                WHERE thread_id = %s AND user_id = %s
            """
            args: list[object] = [thread_id, user_id]
            if before is not None:
                before_created_at, before_id = _message_key_from_cursor(before)
                sql += " AND (created_at < %s OR (created_at = %s AND id < %s))"
                args.extend([before_created_at, before_created_at, before_id])
            sql += " ORDER BY created_at DESC, id DESC LIMIT %s"
            args.append(limit + 1)
            with connection.cursor() as cursor:
                cursor.execute(sql, tuple(args))
                rows = cursor.fetchall()
            has_more = len(rows) > limit
            page_rows = list(rows[:limit])
            page_rows.reverse()
            messages = [self._map_row(row) for row in page_rows]
            next_cursor = _build_message_cursor(messages[0]) if has_more and messages else None
            return messages, next_cursor
        finally:
            connection.close()

    def count_by_thread(self, *, thread_id: str, user_id: int) -> int:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    "SELECT COUNT(1) AS total FROM agent_messages WHERE thread_id = %s AND user_id = %s",
                    (thread_id, user_id),
                )
                row = cursor.fetchone()
            return int(row["total"])
        finally:
            connection.close()

    def update_status(
        self,
        *,
        message_id: str,
        status: str,
        parts: list[dict] | None,
        metadata: dict | None,
    ) -> MessageRecord | None:
        connection = self.connection_factory.connect()
        try:
            updates = {"status": status}
            if parts is not None:
                updates["parts_json"] = json.dumps(parts)
            if metadata is not None:
                updates["metadata_json"] = json.dumps(metadata)
            updates["updated_at"] = datetime.now(timezone.utc)
            assignments = ", ".join(f"{field} = %s" for field in updates.keys())
            with connection.cursor() as cursor:
                cursor.execute(
                    f"UPDATE agent_messages SET {assignments} WHERE id = %s",
                    (*updates.values(), message_id),
                )
            connection.commit()
            return self.find_by_id(message_id=message_id)
        except Exception:
            connection.rollback()
            raise
        finally:
            connection.close()

    def _map_row(self, row: dict) -> MessageRecord:
        metadata = row.get("metadata_json")
        return MessageRecord(
            id=row["id"],
            thread_id=row["thread_id"],
            user_id=int(row["user_id"]),
            role=row["role"],
            parts=json.loads(row["parts_json"]),
            status=row["status"],
            run_id=row.get("run_id"),
            created_at=row["created_at"],
            updated_at=row.get("updated_at"),
            metadata=json.loads(metadata) if metadata else {},
        )


def _build_message_cursor(record: MessageRecord) -> str:
    return encode_cursor({"createdAt": record.created_at.isoformat().replace("+00:00", "Z"), "id": record.id})


def _message_key_from_cursor(cursor: str) -> tuple[datetime, str]:
    payload = decode_cursor(cursor)
    return (datetime.fromisoformat(payload["createdAt"].replace("Z", "+00:00")), payload["id"])


def _message_sort_key(record: MessageRecord) -> tuple[datetime, str]:
    return (record.created_at, record.id)
