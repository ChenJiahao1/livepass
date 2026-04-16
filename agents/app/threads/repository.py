from __future__ import annotations

import json
from dataclasses import replace
from datetime import datetime, timezone
from typing import Protocol

import pymysql

from app.common.cursor import decode_cursor, encode_cursor
from app.common.ids import new_thread_id
from app.config import Settings, get_settings
from app.threads.models import THREAD_STATUS_ACTIVE, THREAD_STATUS_ARCHIVED, ThreadRecord


class ThreadRepository(Protocol):
    def create(self, *, user_id: int, title: str, now: datetime) -> ThreadRecord: ...

    def find_by_id(self, *, thread_id: str) -> ThreadRecord | None: ...

    def list_by_user(
        self,
        *,
        user_id: int,
        status: str,
        limit: int,
        cursor: str | None,
        include_empty: bool,
    ) -> tuple[list[ThreadRecord], str | None]: ...

    def update_metadata(self, *, thread_id: str, metadata: dict) -> ThreadRecord | None: ...

    def update_title(self, *, thread_id: str, title: str) -> ThreadRecord | None: ...

    def update_last_message_at(self, *, thread_id: str, last_message_at: datetime) -> ThreadRecord | None: ...

    def update_status(self, *, thread_id: str, status: str, now: datetime) -> ThreadRecord | None: ...

    def archive(self, *, thread_id: str, now: datetime) -> ThreadRecord | None: ...


class InMemoryThreadRepository:
    def __init__(self) -> None:
        self._threads: dict[str, ThreadRecord] = {}

    def create(self, *, user_id: int, title: str, now: datetime) -> ThreadRecord:
        record = ThreadRecord(
            id=new_thread_id(),
            user_id=user_id,
            title=title,
            status=THREAD_STATUS_ACTIVE,
            created_at=now,
            updated_at=now,
            last_message_at=None,
            metadata={},
        )
        self._threads[record.id] = record
        return replace(record)

    def find_by_id(self, *, thread_id: str) -> ThreadRecord | None:
        record = self._threads.get(thread_id)
        return replace(record) if record else None

    def list_by_user(
        self,
        *,
        user_id: int,
        status: str,
        limit: int,
        cursor: str | None,
        include_empty: bool,
    ) -> tuple[list[ThreadRecord], str | None]:
        threads = [
            replace(record)
            for record in self._threads.values()
            if record.user_id == user_id and record.status == status
        ]
        if not include_empty:
            threads = [record for record in threads if record.last_message_at is not None]

        threads.sort(
            key=lambda record: (
                record.last_message_at or record.created_at,
                record.created_at,
                record.id,
            ),
            reverse=True,
        )
        if cursor is not None:
            cursor_key = _thread_key_from_cursor(cursor)
            threads = [record for record in threads if _thread_sort_key(record) < cursor_key]

        has_more = len(threads) > limit
        page = threads[:limit]
        next_cursor = _build_cursor(page[-1]) if has_more and page else None
        return page, next_cursor

    def update_metadata(self, *, thread_id: str, metadata: dict) -> ThreadRecord | None:
        record = self._threads.get(thread_id)
        if record is None:
            return None
        record.metadata = dict(metadata)
        return replace(record)

    def update_title(self, *, thread_id: str, title: str) -> ThreadRecord | None:
        record = self._threads.get(thread_id)
        if record is None:
            return None
        record.title = title
        return replace(record)

    def update_last_message_at(self, *, thread_id: str, last_message_at: datetime) -> ThreadRecord | None:
        record = self._threads.get(thread_id)
        if record is None:
            return None
        record.last_message_at = last_message_at
        record.updated_at = last_message_at
        return replace(record)

    def update_status(self, *, thread_id: str, status: str, now: datetime) -> ThreadRecord | None:
        record = self._threads.get(thread_id)
        if record is None:
            return None
        record.status = status
        record.updated_at = now
        return replace(record)

    def archive(self, *, thread_id: str, now: datetime) -> ThreadRecord | None:
        return self.update_status(thread_id=thread_id, status=THREAD_STATUS_ARCHIVED, now=now)


class MySQLConnectionFactory:
    def __init__(self, settings: Settings | None = None) -> None:
        self.settings = settings or get_settings()

    def connect(self):
        return pymysql.connect(
            host=self.settings.agents_mysql_host,
            port=self.settings.agents_mysql_port,
            user=self.settings.agents_mysql_user,
            password=self.settings.agents_mysql_password,
            database=self.settings.agents_mysql_database,
            charset=self.settings.agents_mysql_charset,
            cursorclass=pymysql.cursors.DictCursor,
            autocommit=False,
        )


class MySQLThreadRepository:
    def __init__(self, connection_factory: MySQLConnectionFactory | None = None) -> None:
        self.connection_factory = connection_factory or MySQLConnectionFactory()

    def create(self, *, user_id: int, title: str, now: datetime) -> ThreadRecord:
        record = ThreadRecord(
            id=new_thread_id(),
            user_id=user_id,
            title=title,
            status=THREAD_STATUS_ACTIVE,
            created_at=now,
            updated_at=now,
            last_message_at=None,
            metadata={},
        )
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    INSERT INTO agent_threads (
                      id, user_id, title, status, created_at, updated_at, last_message_at, metadata_json
                    ) VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        record.id,
                        record.user_id,
                        record.title,
                        record.status,
                        record.created_at,
                        record.updated_at,
                        record.last_message_at,
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

    def find_by_id(self, *, thread_id: str) -> ThreadRecord | None:
        connection = self.connection_factory.connect()
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """
                    SELECT id, user_id, title, status, created_at, updated_at, last_message_at, metadata_json
                    FROM agent_threads
                    WHERE id = %s
                    """,
                    (thread_id,),
                )
                row = cursor.fetchone()
            return self._map_row(row) if row else None
        finally:
            connection.close()

    def list_by_user(
        self,
        *,
        user_id: int,
        status: str,
        limit: int,
        cursor: str | None,
        include_empty: bool,
    ) -> tuple[list[ThreadRecord], str | None]:
        connection = self.connection_factory.connect()
        try:
            sql = """
                SELECT id, user_id, title, status, created_at, updated_at, last_message_at, metadata_json
                FROM agent_threads
                WHERE user_id = %s AND status = %s
            """
            args: list[object] = [user_id, status]
            if not include_empty:
                sql += " AND last_message_at IS NOT NULL"
            if cursor is not None:
                sort_at, created_at, cursor_id = _thread_key_from_cursor(cursor)
                sql += """
                    AND (
                        COALESCE(last_message_at, created_at) < %s
                        OR (COALESCE(last_message_at, created_at) = %s AND created_at < %s)
                        OR (COALESCE(last_message_at, created_at) = %s AND created_at = %s AND id < %s)
                    )
                """
                args.extend([sort_at, sort_at, created_at, sort_at, created_at, cursor_id])
            sql += """
                ORDER BY COALESCE(last_message_at, created_at) DESC, created_at DESC, id DESC
                LIMIT %s
            """
            args.append(limit + 1)
            with connection.cursor() as cursor_obj:
                cursor_obj.execute(sql, tuple(args))
                rows = cursor_obj.fetchall()
            threads = [self._map_row(row) for row in rows]
            has_more = len(threads) > limit
            page = threads[:limit]
            next_cursor = _build_cursor(page[-1]) if has_more and page else None
            return page, next_cursor
        finally:
            connection.close()

    def update_metadata(self, *, thread_id: str, metadata: dict) -> ThreadRecord | None:
        return self._update_fields(thread_id=thread_id, updates={"metadata_json": json.dumps(metadata)})

    def update_title(self, *, thread_id: str, title: str) -> ThreadRecord | None:
        return self._update_fields(thread_id=thread_id, updates={"title": title})

    def update_last_message_at(self, *, thread_id: str, last_message_at: datetime) -> ThreadRecord | None:
        return self._update_fields(
            thread_id=thread_id,
            updates={"last_message_at": last_message_at, "updated_at": last_message_at},
        )

    def update_status(self, *, thread_id: str, status: str, now: datetime) -> ThreadRecord | None:
        return self._update_fields(thread_id=thread_id, updates={"status": status, "updated_at": now})

    def archive(self, *, thread_id: str, now: datetime) -> ThreadRecord | None:
        return self.update_status(thread_id=thread_id, status=THREAD_STATUS_ARCHIVED, now=now)

    def _update_fields(self, *, thread_id: str, updates: dict[str, object]) -> ThreadRecord | None:
        connection = self.connection_factory.connect()
        try:
            assignments = ", ".join(f"{field} = %s" for field in updates.keys())
            values = list(updates.values())
            with connection.cursor() as cursor:
                cursor.execute(
                    f"UPDATE agent_threads SET {assignments} WHERE id = %s",
                    (*values, thread_id),
                )
            connection.commit()
            return self.find_by_id(thread_id=thread_id)
        except Exception:
            connection.rollback()
            raise
        finally:
            connection.close()

    def _map_row(self, row: dict) -> ThreadRecord:
        metadata = row.get("metadata_json")
        return ThreadRecord(
            id=row["id"],
            user_id=int(row["user_id"]),
            title=row["title"],
            status=row["status"],
            created_at=row["created_at"],
            updated_at=row["updated_at"],
            last_message_at=row.get("last_message_at"),
            metadata=json.loads(metadata) if metadata else {},
        )


def _build_cursor(record: ThreadRecord) -> str:
    sort_at = record.last_message_at or record.created_at
    return encode_cursor(
        {
            "sortAt": _isoformat(sort_at),
            "createdAt": _isoformat(record.created_at),
            "id": record.id,
        }
    )


def _thread_key_from_cursor(cursor: str) -> tuple[datetime, datetime, str]:
    payload = decode_cursor(cursor)
    return (
        _parse_datetime(payload["sortAt"]),
        _parse_datetime(payload["createdAt"]),
        payload["id"],
    )


def _thread_sort_key(record: ThreadRecord) -> tuple[datetime, datetime, str]:
    return (record.last_message_at or record.created_at, record.created_at, record.id)


def _isoformat(value: datetime) -> str:
    normalized = value.astimezone(timezone.utc)
    return normalized.isoformat().replace("+00:00", "Z")


def _parse_datetime(value: str) -> datetime:
    return datetime.fromisoformat(value.replace("Z", "+00:00"))
