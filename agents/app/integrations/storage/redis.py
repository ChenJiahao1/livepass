"""Redis-backed LangGraph checkpointer keyed by resource thread."""

from __future__ import annotations

import asyncio
import base64
import json
from collections.abc import AsyncIterator, Iterator, Sequence
from typing import Any

import redis
from langchain_core.runnables import RunnableConfig
from langgraph.checkpoint.base import (
    WRITES_IDX_MAP,
    BaseCheckpointSaver,
    ChannelVersions,
    Checkpoint,
    CheckpointMetadata,
    CheckpointTuple,
    get_checkpoint_id,
    get_checkpoint_metadata,
)


def _pack_typed(value: tuple[str, bytes]) -> str:
    return json.dumps({"type": value[0], "data": base64.b64encode(value[1]).decode("ascii")})


def _unpack_typed(value: str) -> tuple[str, bytes]:
    payload = json.loads(value)
    return str(payload["type"]), base64.b64decode(payload["data"])


class RedisCheckpointSaver(BaseCheckpointSaver[str]):
    """Persist LangGraph checkpoints in Redis using the resource `thread_id`."""

    def __init__(
        self,
        *,
        redis_client,
        ttl_seconds: int,
        key_prefix: str = "agents:langgraph",
    ) -> None:
        super().__init__()
        self.redis_client = redis_client
        self.ttl_seconds = ttl_seconds
        self.key_prefix = key_prefix

    @classmethod
    def from_url(
        cls,
        redis_url: str,
        *,
        ttl_seconds: int,
        key_prefix: str = "agents:langgraph",
    ) -> "RedisCheckpointSaver":
        client = redis.Redis.from_url(redis_url, decode_responses=True)
        return cls(redis_client=client, ttl_seconds=ttl_seconds, key_prefix=key_prefix)

    def get_tuple(self, config: RunnableConfig) -> CheckpointTuple | None:
        thread_id = config["configurable"]["thread_id"]
        checkpoint_ns = config["configurable"].get("checkpoint_ns", "")
        checkpoint_id = get_checkpoint_id(config) or self._latest_checkpoint_id(thread_id, checkpoint_ns)
        if checkpoint_id is None:
            return None

        payload = self.redis_client.hget(self._checkpoints_key(thread_id, checkpoint_ns), checkpoint_id)
        if payload is None:
            return None

        record = json.loads(payload)
        checkpoint = self.serde.loads_typed(_unpack_typed(record["checkpoint"]))
        metadata = self.serde.loads_typed(_unpack_typed(record["metadata"]))
        parent_checkpoint_id = record.get("parent_checkpoint_id") or None
        pending_writes = self._load_pending_writes(thread_id, checkpoint_ns, checkpoint_id)

        return CheckpointTuple(
            config={
                "configurable": {
                    "thread_id": thread_id,
                    "checkpoint_ns": checkpoint_ns,
                    "checkpoint_id": checkpoint_id,
                }
            },
            checkpoint=checkpoint,
            metadata=metadata,
            parent_config=(
                {
                    "configurable": {
                        "thread_id": thread_id,
                        "checkpoint_ns": checkpoint_ns,
                        "checkpoint_id": parent_checkpoint_id,
                    }
                }
                if parent_checkpoint_id
                else None
            ),
            pending_writes=pending_writes,
        )

    def list(
        self,
        config: RunnableConfig | None,
        *,
        filter: dict[str, Any] | None = None,
        before: RunnableConfig | None = None,
        limit: int | None = None,
    ) -> Iterator[CheckpointTuple]:
        candidates: list[CheckpointTuple] = []
        checkpoint_keys = self._iter_checkpoint_keys(config)
        before_id = get_checkpoint_id(before) if before else None

        for key in checkpoint_keys:
            thread_id, checkpoint_ns = self._parse_checkpoint_key(key)
            checkpoint_ids = sorted(self.redis_client.hkeys(key), reverse=True)
            for checkpoint_id in checkpoint_ids:
                if before_id is not None and checkpoint_id >= before_id:
                    continue
                item = self.get_tuple(
                    {
                        "configurable": {
                            "thread_id": thread_id,
                            "checkpoint_ns": checkpoint_ns,
                            "checkpoint_id": checkpoint_id,
                        }
                    }
                )
                if item is None:
                    continue
                if filter and any(item.metadata.get(name) != value for name, value in filter.items()):
                    continue
                candidates.append(item)
                if limit is not None and len(candidates) >= limit:
                    return iter(candidates)
        return iter(candidates)

    def put(
        self,
        config: RunnableConfig,
        checkpoint: Checkpoint,
        metadata: CheckpointMetadata,
        new_versions: ChannelVersions,
    ) -> RunnableConfig:
        del new_versions

        thread_id = config["configurable"]["thread_id"]
        checkpoint_ns = config["configurable"].get("checkpoint_ns", "")
        checkpoint_id = checkpoint["id"]
        payload = json.dumps(
            {
                "checkpoint": _pack_typed(self.serde.dumps_typed(checkpoint)),
                "metadata": _pack_typed(self.serde.dumps_typed(get_checkpoint_metadata(config, metadata))),
                "parent_checkpoint_id": config["configurable"].get("checkpoint_id"),
            }
        )

        checkpoints_key = self._checkpoints_key(thread_id, checkpoint_ns)
        self.redis_client.hset(checkpoints_key, checkpoint_id, payload)
        self.redis_client.expire(checkpoints_key, self.ttl_seconds)
        return {
            "configurable": {
                "thread_id": thread_id,
                "checkpoint_ns": checkpoint_ns,
                "checkpoint_id": checkpoint_id,
            }
        }

    def put_writes(
        self,
        config: RunnableConfig,
        writes: Sequence[tuple[str, Any]],
        task_id: str,
        task_path: str = "",
    ) -> None:
        thread_id = config["configurable"]["thread_id"]
        checkpoint_ns = config["configurable"].get("checkpoint_ns", "")
        checkpoint_id = config["configurable"]["checkpoint_id"]
        writes_key = self._writes_key(thread_id, checkpoint_ns, checkpoint_id)

        existing = self.redis_client.hgetall(writes_key)
        for idx, (channel, value) in enumerate(writes):
            sort_index = WRITES_IDX_MAP.get(channel, idx)
            field = f"{task_id}:{sort_index}"
            if sort_index >= 0 and field in existing:
                continue
            payload = json.dumps(
                {
                    "task_id": task_id,
                    "channel": channel,
                    "value": _pack_typed(self.serde.dumps_typed(value)),
                    "task_path": task_path,
                    "sort_index": sort_index,
                }
            )
            self.redis_client.hset(writes_key, field, payload)

        self.redis_client.expire(writes_key, self.ttl_seconds)

    def delete_thread(self, thread_id: str) -> None:
        keys = list(self.redis_client.scan_iter(match=self._thread_pattern(thread_id)))
        if keys:
            self.redis_client.delete(*keys)

    async def aget_tuple(self, config: RunnableConfig) -> CheckpointTuple | None:
        return await asyncio.to_thread(self.get_tuple, config)

    async def alist(
        self,
        config: RunnableConfig | None,
        *,
        filter: dict[str, Any] | None = None,
        before: RunnableConfig | None = None,
        limit: int | None = None,
    ) -> AsyncIterator[CheckpointTuple]:
        items = await asyncio.to_thread(lambda: list(self.list(config, filter=filter, before=before, limit=limit)))
        for item in items:
            yield item

    async def aput(
        self,
        config: RunnableConfig,
        checkpoint: Checkpoint,
        metadata: CheckpointMetadata,
        new_versions: ChannelVersions,
    ) -> RunnableConfig:
        return await asyncio.to_thread(self.put, config, checkpoint, metadata, new_versions)

    async def aput_writes(
        self,
        config: RunnableConfig,
        writes: Sequence[tuple[str, Any]],
        task_id: str,
        task_path: str = "",
    ) -> None:
        await asyncio.to_thread(self.put_writes, config, writes, task_id, task_path)

    async def adelete_thread(self, thread_id: str) -> None:
        await asyncio.to_thread(self.delete_thread, thread_id)

    def _iter_checkpoint_keys(self, config: RunnableConfig | None) -> list[str]:
        if config is not None:
            thread_id = config["configurable"]["thread_id"]
            checkpoint_ns = config["configurable"].get("checkpoint_ns", "")
            return [self._checkpoints_key(thread_id, checkpoint_ns)]
        return list(self.redis_client.scan_iter(match=f"{self.key_prefix}:checkpoints:*"))

    def _latest_checkpoint_id(self, thread_id: str, checkpoint_ns: str) -> str | None:
        checkpoint_ids = self.redis_client.hkeys(self._checkpoints_key(thread_id, checkpoint_ns))
        if not checkpoint_ids:
            return None
        return max(checkpoint_ids)

    def _load_pending_writes(
        self,
        thread_id: str,
        checkpoint_ns: str,
        checkpoint_id: str,
    ) -> list[tuple[str, str, Any]]:
        payloads = self.redis_client.hgetall(self._writes_key(thread_id, checkpoint_ns, checkpoint_id)).values()
        items: list[tuple[int, tuple[str, str, Any]]] = []
        for payload in payloads:
            record = json.loads(payload)
            items.append(
                (
                    int(record["sort_index"]),
                    (
                        str(record["task_id"]),
                        str(record["channel"]),
                        self.serde.loads_typed(_unpack_typed(record["value"])),
                    ),
                )
            )
        items.sort(key=lambda item: item[0])
        return [item[1] for item in items]

    def _parse_checkpoint_key(self, key: str) -> tuple[str, str]:
        prefix = f"{self.key_prefix}:checkpoints:"
        suffix = key.removeprefix(prefix)
        thread_id, checkpoint_ns = suffix.split(":", 1)
        return thread_id, self._decode_namespace(checkpoint_ns)

    def _checkpoints_key(self, thread_id: str, checkpoint_ns: str) -> str:
        return f"{self.key_prefix}:checkpoints:{thread_id}:{self._encode_namespace(checkpoint_ns)}"

    def _writes_key(self, thread_id: str, checkpoint_ns: str, checkpoint_id: str) -> str:
        return f"{self.key_prefix}:writes:{thread_id}:{self._encode_namespace(checkpoint_ns)}:{checkpoint_id}"

    def _thread_pattern(self, thread_id: str) -> str:
        return f"{self.key_prefix}:*:{thread_id}:*"

    @staticmethod
    def _encode_namespace(checkpoint_ns: str) -> str:
        return checkpoint_ns or "__root__"

    @staticmethod
    def _decode_namespace(checkpoint_ns: str) -> str:
        if checkpoint_ns == "__root__":
            return ""
        return checkpoint_ns
