from __future__ import annotations

from uuid import uuid4


def new_thread_id() -> str:
    return f"thr_{uuid4().hex}"


def new_message_id() -> str:
    return f"msg_{uuid4().hex}"


def new_run_id() -> str:
    return f"run_{uuid4().hex}"


def new_run_event_id() -> str:
    return f"evt_{uuid4().hex}"


def new_tool_call_id() -> str:
    return f"tool_{uuid4().hex}"
