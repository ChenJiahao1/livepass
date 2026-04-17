from __future__ import annotations

import asyncio


class RunEventBus:
    def __init__(self) -> None:
        self._subscribers: dict[str, list[asyncio.Queue[int]]] = {}

    def subscribe(self, *, run_id: str) -> asyncio.Queue[int]:
        queue: asyncio.Queue[int] = asyncio.Queue()
        self._subscribers.setdefault(run_id, []).append(queue)
        return queue

    def unsubscribe(self, *, run_id: str, queue: asyncio.Queue[int]) -> None:
        subscribers = self._subscribers.get(run_id, [])
        self._subscribers[run_id] = [item for item in subscribers if item is not queue]
        if not self._subscribers[run_id]:
            self._subscribers.pop(run_id, None)

    def publish(self, *, run_id: str, sequence_no: int) -> None:
        for queue in self._subscribers.get(run_id, []):
            queue.put_nowait(sequence_no)
