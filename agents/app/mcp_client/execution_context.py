from __future__ import annotations

from dataclasses import dataclass


@dataclass(slots=True)
class ToolExecutionContext:
    user_id: int
    thread_id: str
    run_id: str
    tool_call_id: str
    channel_code: str | None = None
    request_id: str | None = None

    def to_meta(self) -> dict[str, int | str]:
        payload = {
            "userId": self.user_id,
            "threadId": self.thread_id,
            "runId": self.run_id,
            "toolCallId": self.tool_call_id,
        }
        if self.channel_code:
            payload["channelCode"] = self.channel_code
        if self.request_id:
            payload["requestId"] = self.request_id
        return payload
