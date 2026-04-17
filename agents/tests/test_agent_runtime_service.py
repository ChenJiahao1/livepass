from __future__ import annotations

from datetime import datetime, timezone
from pathlib import Path

import pytest

from app.runs.execution.runtime import AgentRuntimeService
from app.runs.models import RunRecord


def test_run_execution_modules_live_under_runs_execution():
    assert Path("app/runs/execution/runtime.py").is_file()
    assert Path("app/runs/execution/callbacks.py").is_file()
    assert Path("app/runs/execution/executor.py").is_file()
    assert Path("app/runs/execution/stream.py").is_file()
    assert Path("app/runs/execution/projector.py").is_file()
    assert Path("app/runs/execution/resume.py").is_file()
    assert Path("app/runs/execution/interrupt_bridge.py").is_file()
    assert Path("app/runs/execution/event_bus.py").is_file()
    assert Path("app/runs/interrupt_models.py").is_file()
    assert Path("app/agents/tools/human_tools.py").is_file()
    assert not Path("app/agent_runtime").exists()
    assert not Path("app/runs/service.py").exists()


class _CapturingRuntime:
    def __init__(self) -> None:
        self.contexts: list[dict] = []

    async def ainvoke(self, payload, config, context):
        del payload
        del config
        self.contexts.append(dict(context))
        return {"reply": "ok", "final_reply": "ok"}


class _CapturingStreamRuntime:
    def __init__(self) -> None:
        self.contexts: list[dict] = []

    async def astream(self, payload, config, context, stream_mode):
        del payload
        del config
        del stream_mode
        self.contexts.append(dict(context))
        if False:
            yield None


class _CapturingRegistry:
    def __init__(self) -> None:
        self.bound_user_ids: list[int] = []

    def bind_context(self, *, user_id: int, thread_id: str, run_id: str):
        del thread_id
        del run_id
        self.bound_user_ids.append(user_id)
        return self


class _NoopCallbacks:
    async def on_run_started(self, *, run):
        del run

    async def on_run_updated(self, *, run, status, payload=None, metadata=None):
        del run, status, payload, metadata

    async def on_message_delta(self, *, run, message_id, delta, metadata=None):
        del run, message_id, delta, metadata

    async def on_message_updated(self, *, run, message_id, status, payload=None, metadata=None):
        del run, message_id, status, payload, metadata

    async def on_tool_call_started(self, *, run, tool_name, args, request, metadata=None):
        del run, tool_name, args, request, metadata

    async def on_tool_call_requires_human(self, *, run, tool_name, args, request, metadata=None):
        del run, tool_name, args, request, metadata

    async def on_tool_call_completed(self, *, run, tool_call_id, output, metadata=None):
        del run, tool_call_id, output, metadata

    async def on_tool_call_progress(self, *, run, tool_name, payload, metadata=None):
        del run, tool_name, payload, metadata

    async def on_tool_call_failed(self, *, run, tool_call_id, error, metadata=None):
        del run, tool_call_id, error, metadata

    async def on_run_progress(self, *, run, payload, metadata=None):
        del run, payload, metadata


@pytest.mark.anyio
async def test_invoke_passes_integer_current_user_id_into_runtime_context():
    runtime = _CapturingRuntime()
    service = AgentRuntimeService(
        agent_runtime=runtime,
        registry=object(),
        llm=object(),
    )

    result = await service.invoke(user_id=3001, thread_id="thr_001", user_text="帮我查订单")

    assert result.reply == "ok"
    assert runtime.contexts == [
        {
            "registry": service.registry,
            "llm": service.llm,
            "knowledge_service": None,
            "current_user_id": 3001,
        }
    ]


@pytest.mark.anyio
async def test_invoke_run_passes_integer_user_id_to_registry_and_runtime_context():
    runtime = _CapturingStreamRuntime()
    registry = _CapturingRegistry()
    service = AgentRuntimeService(
        agent_runtime=runtime,
        registry=registry,
        llm=object(),
    )
    run = RunRecord(
        id="run_001",
        thread_id="thr_001",
        user_id=3001,
        trigger_message_id="msg_in",
        output_message_id="msg_out",
        status="running",
        started_at=datetime.now(timezone.utc),
    )

    result = await service.invoke_run(
        run=run,
        user_text="帮我查订单",
        callbacks=_NoopCallbacks(),
    )

    assert result.requires_action is False
    assert registry.bound_user_ids == [3001]
    assert runtime.contexts == [
        {
            "registry": registry,
            "llm": service.llm,
            "knowledge_service": None,
            "current_user_id": 3001,
        }
    ]
