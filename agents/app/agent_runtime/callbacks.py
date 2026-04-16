from __future__ import annotations

from typing import Any, Protocol

from app.runs.models import RunRecord


class RuntimeCallbacks(Protocol):
    async def on_run_started(self, *, run: RunRecord) -> None: ...

    async def on_message_delta(
        self,
        *,
        run: RunRecord,
        message_id: str,
        delta: str,
        metadata: dict[str, Any] | None = None,
    ) -> None: ...

    async def on_tool_call_started(
        self,
        *,
        run: RunRecord,
        tool_name: str,
        arguments: dict[str, Any],
        metadata: dict[str, Any] | None = None,
    ) -> None: ...

    async def on_tool_call_requires_human(
        self,
        *,
        run: RunRecord,
        tool_name: str,
        arguments: dict[str, Any],
        metadata: dict[str, Any] | None = None,
    ) -> None: ...

    async def on_tool_call_completed(
        self,
        *,
        run: RunRecord,
        tool_call_id: str,
        output: dict[str, Any],
        metadata: dict[str, Any] | None = None,
    ) -> None: ...

    async def on_tool_call_failed(
        self,
        *,
        run: RunRecord,
        tool_call_id: str,
        error: dict[str, Any],
        metadata: dict[str, Any] | None = None,
    ) -> None: ...
